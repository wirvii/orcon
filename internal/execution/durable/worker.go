// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/fairness"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
)

// Claim leases up to limit runnable tokens for this worker, whatever their
// organization (ADR-0013 section 3), except the one a token one abandonment short
// of the poison threshold, which only the worker loop claims (it must run alone).
// The caller must call [Engine.Work] for each, or the leases expire and the reaper
// returns the tokens.
func (e *Engine) Claim(ctx context.Context, limit int) ([]store.Claimed, time.Time, error) {
	start := time.Now()
	now := e.cfg.Clock.Now()
	counts, err := e.store.RunnableCounts(ctx, now)
	if err != nil {
		return nil, start, err
	}
	entries := make([]fairness.Entry, 0, len(counts))
	for _, c := range counts {
		entries = append(entries, fairness.Entry{
			Key: c.FairnessKey, Class: fairnessClass(c.Priority), Runnable: c.Runnable, Oldest: c.Oldest,
		})
	}
	e.fairMu.Lock()
	grants := e.fair.Plan(entries, now, fairness.DefaultAgingInterval, int64(limit))
	e.fairMu.Unlock()

	var out []store.Claimed
	for _, g := range grants {
		req := e.claimRequest(int(g.Count))
		req.FairnessKeys = []string{g.Key}
		claimed, err := e.store.ClaimTokens(ctx, req)
		out = append(out, claimed...)
		if err != nil {
			return out, start, err
		}
	}
	// The snapshot is not a reservation: whatever the grants did not take is
	// claimed unscoped, so that no capacity is left idle by a stale count.
	if rest := limit - len(out); rest > 0 {
		claimed, err := e.store.ClaimTokens(ctx, e.claimRequest(rest))
		out = append(out, claimed...)
		return out, start, err
	}
	return out, start, nil
}

// fairnessClass maps the stored priority of a token to its class (SPEC-001
// R-FAIR-01). Stored values outside the three classes are read as LOW.
func fairnessClass(p int64) fairness.Class {
	switch p {
	case store.PriorityHigh:
		return fairness.High
	case store.PriorityNormal:
		return fairness.Normal
	default:
		return fairness.Low
	}
}

// Work runs one claimed token until it ends, waits or is released, committing at
// each confirmation point. It returns nil when the token was handled, including
// when its lease was lost (the work is then done by whoever holds the token
// now), and an error only when the engine could not process the token: the lease
// is then given back at once and counts as an abandonment (ADR-0013 sections 4
// and 9), because renewing by worker would otherwise keep it alive for as long
// as the worker holds any other token.
func (e *Engine) Work(ctx context.Context, c store.Claimed, claimedAt time.Time) error {
	f := store.Fence{OrgID: c.OrgID, Token: c.TokenID, Epoch: c.LeaseEpoch, Owner: e.cfg.Worker}
	lctx, l := e.leases.add(ctx, f, claimedAt)
	defer e.leases.drop(l)
	err := e.drive(ctx, lctx, l, c)
	if err != nil && ctx.Err() == nil {
		e.abandonLease(ctx, l.fence)
	}
	return err
}

// drive works the token and reports why it stopped.
func (e *Engine) drive(ctx, lctx context.Context, l *heldLease, c store.Claimed) error {
	if e.cfg.Faults != nil {
		if err := e.cfg.Faults.Hit(lctx, PointClaimed, c.TokenID); err != nil {
			return err
		}
	}
	for {
		more, err := e.segment(lctx, l, c.ExecutionID)
		switch {
		case err == nil && !more:
			return nil
		case err == nil:
			continue
		case errors.Is(err, store.ErrFenced), errors.Is(err, errLostToken), errors.Is(err, ErrLeaseLost), errors.Is(err, ErrLeaseStale):
			e.cfg.Logger.InfoContext(ctx, "token dropped", slog.String("token", l.fence.Token.String()), slog.String("reason", err.Error()))
			return nil
		default:
			return err
		}
	}
}

// abandonLease gives the lease of a failed token back. When the database cannot
// be reached the lease is remembered and given back by the renewal loop as soon
// as it can: until then renewals keep it alive.
func (e *Engine) abandonLease(ctx context.Context, f store.Fence) {
	if f.Epoch == 0 {
		return // the token had left the worker when the failure came
	}
	if !e.releaseLease(ctx, f) {
		e.leases.orphan(f)
	}
}

// releaseLease tries once to give a lease back. It reports whether the lease is
// settled: released, or no longer the worker's.
func (e *Engine) releaseLease(ctx context.Context, f store.Fence) bool {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	reaped, ok, err := e.store.ReleaseTokenLease(rctx, f, e.cfg.PoisonThreshold, e.cfg.Clock.Now())
	if err != nil {
		e.cfg.Logger.WarnContext(ctx, "lease of a failed token not released; it is retried", slog.Any("error", err))
		return false
	}
	if ok {
		e.cfg.Logger.WarnContext(ctx, "lease of a failed token released", slog.String("execution", reaped.ExecutionID.String()),
			slog.Int64("abandoned", reaped.AbandonedCount), slog.Bool("poisoned", reaped.Poisoned))
		e.signal(1)
	}
	return true
}

// ClaimLastChance leases the one token that is one abandonment short of the
// poison threshold, if there is one. The worker loop takes it only with nothing
// else in hand and claims nothing else while it holds it; a caller that uses
// this directly takes on that duty.
func (e *Engine) ClaimLastChance(ctx context.Context) ([]store.Claimed, time.Time, error) {
	start := time.Now()
	claimed, err := e.store.ClaimLastChanceToken(ctx, e.claimRequest(1))
	return claimed, start, err
}

// segment runs the held token from where it is to the next confirmation point
// and commits. It reports whether the worker still holds a token to go on with.
func (e *Engine) segment(ctx context.Context, l *heldLease, execID store.ExecutionID) (bool, error) {
	if context.Cause(ctx) != nil {
		return false, fmt.Errorf("segment of execution %s: %w", execID, context.Cause(ctx))
	}
	org := l.fence.OrgID
	started := time.Now()
	if !l.valid(e.leases) {
		return false, ErrLeaseStale
	}
	ctx = withOrg(ctx, org)
	ex, err := e.metaOf(ctx, org, execID)
	if err != nil {
		return false, err
	}
	plan, err := e.plans.For(ctx, ex)
	if err != nil {
		return false, err
	}
	snap, err := e.store.LoadSnapshot(ctx, org, execID)
	if err != nil {
		return false, fmt.Errorf("load snapshot of %s: %w", execID, err)
	}
	mine, ok := findToken(&snap, l.fence)
	if !ok {
		return false, errLostToken
	}
	esnap, err := snapshotOf(execID, &snap)
	if err != nil {
		return false, err
	}
	known, err := e.knownEnvs(ctx, org, mine)
	if err != nil {
		return false, err
	}
	sess, err := e.interp.OpenSession(plan, esnap, execution.SessionOptions{Known: known})
	if err != nil {
		return false, fmt.Errorf("open execution %s: %w", execID, err)
	}
	work := execution.WorkOptions{
		Budget: e.cfg.CheckpointActivations, StopAtFork: true, Release: e.draining.Load(),
		Checkpoint: e.checkpointOf(org, execID, l.fence),
		// A worker that shuts down pauses at the next activation boundary, commits, and gives the token back.
		Pause: func() bool {
			return time.Since(started) >= e.cfg.CheckpointInterval || !l.valid(e.leases) || e.draining.Load()
		},
	}
	result, err := sess.Work(ctx, execution.TokenID(l.fence.Token.String()), work)
	if err != nil {
		if cause := context.Cause(ctx); errors.Is(cause, ErrLeaseLost) || errors.Is(cause, ErrLeaseStale) {
			return false, cause // the lease was lost while the token ran: the work is dropped, not failed
		}
		return false, fmt.Errorf("work on execution %s: %w", execID, err)
	}
	delta, err := sess.Delta()
	if err != nil {
		return false, fmt.Errorf("collect step of %s: %w", execID, err)
	}
	if e.cfg.Faults != nil {
		if err := e.cfg.Faults.Hit(ctx, PointWorked, l.fence.Token); err != nil {
			return false, err
		}
	}
	if cause := context.Cause(ctx); cause != nil {
		return false, cause
	}
	if !l.valid(e.leases) {
		return false, ErrLeaseStale
	}
	commitStart := time.Now()
	out, err := e.commit(ctx, &step{
		exec: ex, plan: plan, fence: l.fence, baseRevision: snap.Revision, work: work, known: known, delta: delta, result: result,
	})
	if err != nil {
		return false, err
	}
	e.afterCommit(ctx, &out)
	// The lease table follows the commit at once: a failure from here on gives back
	// the lease the worker holds now, not the one it held before.
	e.leases.extend(l, out.held, commitStart)
	if e.cfg.Faults != nil {
		if err := e.cfg.Faults.Hit(ctx, PointCommitted, l.fence.Token); err != nil {
			return false, err
		}
		if out.result.Forked {
			if err := e.cfg.Faults.Hit(ctx, PointForked, l.fence.Token); err != nil {
				return false, err
			}
		}
	}
	e.cfg.Logger.DebugContext(ctx, "step committed", slog.String("execution", execID.String()), slog.Bool("replayed", out.replayed),
		slog.Int("steps", out.result.Steps), slog.Duration("took", time.Since(started)))
	if out.held.Epoch == 0 {
		return false, nil
	}
	return true, nil
}

// findToken returns the held token of a snapshot if this worker still holds it
// at this epoch.
func findToken(snap *store.EngineSnapshot, f store.Fence) (*store.Token, bool) {
	for i := range snap.Tokens {
		t := &snap.Tokens[i]
		if t.ID == f.Token {
			return t, t.Status == "LEASED" && t.LeaseOwner == f.Owner && t.LeaseEpoch == f.Epoch
		}
	}
	return nil, false
}

// knownEnvs reads the environments recorded for a token that was worked before:
// a token claimed for the first time has none, so the read is skipped.
func (e *Engine) knownEnvs(ctx context.Context, org identity.OrgID, t *store.Token) (map[execution.NodeExecutionID]execution.NodeEnv, error) {
	if t.AbandonedCount == 0 && t.ClaimCount <= 1 {
		return nil, nil //nolint:nilnil // an empty map and no map mean the same to the session
	}
	recs, err := e.store.ListTokenNondeterminism(ctx, org, t.ID, 1000)
	if err != nil {
		return nil, fmt.Errorf("read recorded environments of token %s: %w", t.ID, err)
	}
	known := make(map[execution.NodeExecutionID]execution.NodeEnv, len(recs))
	for i := range recs {
		env, err := parseEnv(recs[i].Document)
		if err != nil {
			return nil, err
		}
		known[execution.NodeExecutionID(recs[i].NodeExecutionID.String())] = env
	}
	return known, nil
}

// afterCommit tells the world about a committed step: the observer, the live
// stream, and idle workers when it made tokens claimable.
func (e *Engine) afterCommit(ctx context.Context, c *committed) {
	if e.cfg.Committed != nil {
		e.cfg.Committed(&c.delta, c.replayed)
	}
	if e.cfg.Live != nil {
		for i := range c.delta.Events {
			ev := c.delta.Events[i]
			if err := e.cfg.Live.Emit(ctx, ev); err != nil {
				e.cfg.Logger.WarnContext(ctx, "live event not delivered", slog.String("type", string(ev.Type)), slog.Any("error", err))
			}
		}
	}
	if c.runnable > 0 {
		e.signal(c.runnable)
	}
}
