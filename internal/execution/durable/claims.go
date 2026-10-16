// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
)

// claimState is what the workers of a process know about each other so that a
// token one abandonment short of the poison threshold runs alone (SPEC-001
// Enmienda 12 section 12-1): an abandonment is counted against every token a
// crashed process held, and the one that reaches the threshold must be one that
// ran by itself, or healthy neighbors of a poison item would be suspended with it.
type claimState struct {
	mu sync.Mutex
	// busy counts the tokens this process has claimed and not finished.
	busy int
	// alone is set while the process holds a last chance token: it claims nothing else.
	alone bool
	// starving is set when a last chance token has waited longer than IsolationWait:
	// the process claims nothing new until it is empty, and then takes that token.
	starving bool
	checked  time.Time
}

// claimKind says how a claim was made.
type claimKind int

const (
	claimNormal claimKind = iota
	claimLastChance
)

// claimNext claims the next token for one worker goroutine of the process. It
// takes a last chance token first when the process has nothing else in hand, claims
// normally otherwise, and claims nothing while the process holds a last chance
// token or has to empty itself for one.
func (e *Engine) claimNext(ctx context.Context) ([]store.Claimed, time.Time, claimKind, error) {
	cs := e.claims
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.alone {
		return nil, time.Time{}, claimNormal, nil
	}
	if cs.busy == 0 {
		start := time.Now()
		claimed, err := e.store.ClaimLastChanceToken(ctx, e.claimRequest(1))
		if err != nil {
			return nil, start, claimLastChance, fmt.Errorf("claim last chance token: %w", err)
		}
		if len(claimed) > 0 {
			cs.busy, cs.alone, cs.starving = len(claimed), true, false
			return claimed, start, claimLastChance, nil
		}
	}
	if err := e.checkStarvation(ctx); err != nil {
		return nil, time.Time{}, claimNormal, err
	}
	if cs.starving {
		return nil, time.Time{}, claimNormal, nil // empty out: the last chance token goes to the first worker with nothing in hand
	}
	claimed, start, err := e.Claim(ctx, 1)
	if err != nil {
		return nil, start, claimNormal, err
	}
	cs.busy += len(claimed)
	return claimed, start, claimNormal, nil
}

// checkStarvation refreshes, at most once per poll interval, whether a last
// chance token has waited longer than IsolationWait. The caller holds the lock.
func (e *Engine) checkStarvation(ctx context.Context) error {
	cs := e.claims
	if time.Since(cs.checked) < e.cfg.PollEvery {
		return nil
	}
	cs.checked = time.Now()
	since, found, err := e.store.LastChanceWaiting(ctx, e.cfg.PoisonThreshold)
	if err != nil {
		return fmt.Errorf("look for a last chance token: %w", err)
	}
	cs.starving = found && e.cfg.Clock.Now().Sub(since) > e.cfg.IsolationWait
	return nil
}

// claimDone records that a worker finished a token claimed as kind.
func (e *Engine) claimDone(kind claimKind) {
	cs := e.claims
	cs.mu.Lock()
	cs.busy--
	if kind == claimLastChance {
		cs.alone = false
	}
	cs.mu.Unlock()
}

// claimRequest is the claim of this worker for limit tokens.
func (e *Engine) claimRequest(limit int) store.ClaimRequest {
	return store.ClaimRequest{
		Worker: e.cfg.Worker, Limit: limit, LeaseTTL: e.cfg.LeaseTTL, Now: e.cfg.Clock.Now(), PoisonThreshold: e.cfg.PoisonThreshold,
		ExecutionLimit: int64(e.cfg.ExecutionLimit),
	}
}
