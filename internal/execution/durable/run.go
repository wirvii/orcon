// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
)

// Run starts the loops the configuration asks for and blocks until ctx ends,
// then shuts down gracefully (ADR-0013 section 9): it stops claiming, lets the
// tokens it holds checkpoint and release their lease within the grace period,
// abandons what is left and exits. It returns once everything it started has
// stopped.
func (e *Engine) Run(ctx context.Context) error {
	register := execution.WorkerRecord{
		ID: e.cfg.Worker, Role: e.cfg.Role, Host: e.cfg.Host, Version: e.cfg.Version,
		StartedAt: e.cfg.Clock.Now(), HeartbeatAt: e.cfg.Clock.Now(),
	}
	if err := e.store.RegisterWorker(ctx, register); err != nil {
		return fmt.Errorf("register worker %s: %w", e.cfg.Worker, err)
	}
	// Whatever a previous incarnation of this worker left leased was lost without
	// being returned, so it counts as an abandonment (Enmienda 12 section 12-1).
	if lost, err := e.store.ReleaseLostLeases(ctx, e.cfg.Worker, e.cfg.PoisonThreshold, e.cfg.Clock.Now()); err != nil {
		return fmt.Errorf("release the leases a previous run of worker %s left: %w", e.cfg.Worker, err)
	} else if len(lost) > 0 {
		e.cfg.Logger.WarnContext(ctx, "leases of a previous run released", slog.Int("tokens", len(lost)))
	}
	// Three lifetimes: claiming ends with ctx; the tokens in flight get the grace
	// period after that; the background loops end last, after the leases are given back.
	workCtx, abandon := context.WithCancel(context.WithoutCancel(ctx))
	defer abandon()
	loopCtx, stopLoops := context.WithCancel(context.WithoutCancel(ctx))
	defer stopLoops()

	var loops, workers sync.WaitGroup
	loops.Add(1)
	go func() {
		defer loops.Done()
		e.renewLoop(loopCtx, register)
	}()
	if e.cfg.Reaper {
		loops.Add(1)
		go func() {
			defer loops.Done()
			e.reapLoop(loopCtx)
		}()
	}
	if e.cfg.Dispatcher && e.cfg.Deliver != nil {
		loops.Add(1)
		go func() {
			defer loops.Done()
			e.dispatchLoop(loopCtx)
		}()
	}
	if e.cfg.Workers {
		for range e.cfg.Concurrency {
			workers.Add(1)
			go func() {
				defer workers.Done()
				e.workerLoop(ctx, workCtx)
			}()
		}
	}

	<-ctx.Done()
	e.draining.Store(true)
	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()
	grace := time.NewTimer(e.cfg.ShutdownGrace)
	defer grace.Stop()
	select {
	case <-done:
	case <-grace.C:
		e.cfg.Logger.WarnContext(ctx, "shutdown grace elapsed; abandoning the tokens in flight")
		abandon()
		<-done
	}
	err := e.releaseAll(ctx)
	stopLoops()
	loops.Wait()
	return err
}

// releaseAll gives up whatever lease is still held, with a context of its own so
// that a canceled Run can still do it.
func (e *Engine) releaseAll(parent context.Context) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
	defer cancel()
	released, err := e.store.ReleaseWorkerLeases(ctx, e.cfg.Worker, e.cfg.PoisonThreshold, e.cfg.Clock.Now())
	if err != nil {
		return fmt.Errorf("release the leases of worker %s: %w", e.cfg.Worker, err)
	}
	if len(released) > 0 {
		e.cfg.Logger.InfoContext(ctx, "leases released at shutdown", slog.Int("tokens", len(released)))
	}
	return nil
}

// jitter spreads d by up to 20 % either way, so that idle workers do not poll in step.
func jitter(d time.Duration) time.Duration {
	//nolint:gosec // G404: the jitter spreads polls, it protects nothing
	return time.Duration(float64(d) * (0.8 + 0.4*rand.Float64()))
}

// pause waits for d, a wake-up or the end of ctx.
func (e *Engine) pause(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-e.wake:
	case <-t.C:
	}
}

// workerLoop claims one token at a time and works it. The claiming ends with
// claimCtx and the work in flight with workCtx.
func (e *Engine) workerLoop(claimCtx, workCtx context.Context) {
	for claimCtx.Err() == nil {
		claimed, start, kind, err := e.claimNext(claimCtx)
		if err != nil {
			if claimCtx.Err() == nil {
				e.cfg.Logger.WarnContext(claimCtx, "claim failed", slog.Any("error", err))
				e.pause(claimCtx, jitter(e.cfg.PollEvery))
			}
			continue
		}
		if len(claimed) == 0 {
			e.pause(claimCtx, jitter(e.cfg.PollEvery))
			continue
		}
		for i := range claimed {
			if err := e.Work(workCtx, claimed[i], start); err != nil && workCtx.Err() == nil {
				e.cfg.Logger.ErrorContext(workCtx, "token abandoned after an error", slog.String("token", claimed[i].TokenID.String()), slog.Any("error", err))
			}
			e.claimDone(kind)
		}
	}
}

// renewLoop renews the leases of the worker and keeps its record alive.
func (e *Engine) renewLoop(ctx context.Context, record execution.WorkerRecord) {
	tick := time.NewTicker(e.cfg.RenewEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		for _, f := range e.leases.orphaned() {
			if e.releaseLease(ctx, f) {
				e.leases.settle(f)
			}
		}
		if e.leases.count() > 0 {
			start := time.Now()
			renewed, err := e.store.RenewLeases(ctx, e.cfg.Worker, e.cfg.LeaseTTL, e.cfg.Clock.Now())
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				e.cfg.Logger.WarnContext(ctx, "lease renewal failed", slog.Any("error", err))
				if n := e.leases.stale(); n > 0 {
					e.cfg.Logger.WarnContext(ctx, "leases stale; the worker fenced itself", slog.Int("tokens", n))
				}
			} else if lost := e.leases.renewed(start, renewed); lost > 0 {
				e.cfg.Logger.WarnContext(ctx, "leases lost", slog.Int("tokens", lost))
			}
		}
		record.HeartbeatAt = e.cfg.Clock.Now()
		if err := e.store.RegisterWorker(ctx, record); err != nil && ctx.Err() == nil {
			e.cfg.Logger.WarnContext(ctx, "worker heartbeat failed", slog.Any("error", err))
		}
	}
}

// Reap runs one pass of the recovery: expired leases return to RUNNABLE or are
// suspended as poison, and due timers fire and wake their tokens (ADR-0013
// section 4, ADR-0010 section 8). It reports how many tokens it recovered.
func (e *Engine) Reap(ctx context.Context) (int, error) {
	reaped, err := e.store.ReleaseExpiredLeases(ctx, 100, e.cfg.PoisonThreshold, e.cfg.Clock.Now())
	if err != nil {
		return 0, fmt.Errorf("release expired leases: %w", err)
	}
	runnable := 0
	for _, r := range reaped {
		attrs := []any{slog.String("token", r.TokenID.String()), slog.String("execution", r.ExecutionID.String()), slog.Int64("abandoned", r.AbandonedCount)}
		if r.Poisoned {
			e.cfg.Logger.WarnContext(ctx, "token suspended as poison", attrs...)
			continue
		}
		e.cfg.Logger.InfoContext(ctx, "expired lease released", attrs...)
		runnable++
	}
	fired, err := e.store.FireDueTimers(ctx, 100, e.cfg.Clock.Now())
	if err != nil {
		return len(reaped), fmt.Errorf("fire due timers: %w", err)
	}
	e.signal(runnable + len(fired))
	return len(reaped) + len(fired), nil
}

func (e *Engine) reapLoop(ctx context.Context) {
	tick := time.NewTicker(e.cfg.ReapEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if _, err := e.Reap(ctx); err != nil && ctx.Err() == nil {
			e.cfg.Logger.WarnContext(ctx, "reaper pass failed", slog.Any("error", err))
		}
	}
}

// Dispatch delivers one batch of outbox rows: it leases the rows, hands each to
// the deliverer and marks the delivered ones, so that a row is delivered at
// least once and only the holder of its lease can mark it (ADR-0013 section 8).
// It reports how many rows it delivered.
func (e *Engine) Dispatch(ctx context.Context, limit int) (int, error) {
	if e.cfg.Deliver == nil {
		return 0, errors.New("dispatch outbox: no deliverer is configured")
	}
	rows, err := e.store.ClaimOutbox(ctx, e.cfg.Worker, limit, e.cfg.LeaseTTL, e.cfg.Clock.Now())
	if err != nil {
		return 0, fmt.Errorf("claim outbox: %w", err)
	}
	delivered := 0
	for i := range rows {
		row := &rows[i]
		d := Delivery{OrgID: row.OrgID, ID: row.OutboxID, Topic: row.Topic, Payload: row.Payload, Attempt: row.AttemptCount}
		if err := e.cfg.Deliver.Deliver(ctx, d); err != nil {
			e.cfg.Logger.WarnContext(ctx, "outbox delivery failed; the row is retried when its lease expires",
				slog.String("outbox", row.OutboxID.String()), slog.String("topic", row.Topic), slog.Int64("attempt", row.AttemptCount), slog.Any("error", err))
			continue
		}
		err := e.inTx(ctx, row.OrgID, func(ctx context.Context, tx store.Tx) error {
			ok, err := tx.MarkOutboxDelivered(ctx, row.OutboxID, e.cfg.Worker, e.cfg.Clock.Now())
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("outbox row %s: the lease was lost before the delivery was recorded", row.OutboxID)
			}
			return nil
		})
		if err != nil {
			e.cfg.Logger.WarnContext(ctx, "outbox delivery not recorded; the row will be delivered again", slog.String("outbox", row.OutboxID.String()), slog.Any("error", err))
			continue
		}
		delivered++
	}
	return delivered, nil
}

func (e *Engine) dispatchLoop(ctx context.Context) {
	tick := time.NewTicker(e.cfg.DispatchEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if _, err := e.Dispatch(ctx, 100); err != nil && ctx.Err() == nil {
			e.cfg.Logger.WarnContext(ctx, "outbox dispatch failed", slog.Any("error", err))
		}
	}
}
