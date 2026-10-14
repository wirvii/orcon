// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/platform/clock"
)

// TestClockSkew (FI-10): workers whose wall clocks are an hour apart from each other
// and from the database agree on what has expired and what is claimable, because
// the database clock decides both (ADR-0013 section 4).
func TestClockSkew(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		const lease = 300 * time.Millisecond
		ahead := clock.NewFake(time.Now().Add(time.Hour))
		behind := clock.NewFake(time.Now().Add(-time.Hour))
		engAhead := env.Engine(durable.Config{Concurrency: 1, Clock: ahead, LeaseTTL: lease}, runner())
		engBehind := env.Engine(durable.Config{Concurrency: 1, Clock: behind, LeaseTTL: lease}, runner())
		pub := env.Publish(docOf(forkJoin(3)))
		execID := env.Start(engAhead, pub, input())

		// A token made by the worker that is ahead is claimable at once by the worker that is behind.
		claimed, start, err := engBehind.Claim(ctx, 1)
		if err != nil || len(claimed) != 1 {
			t.Fatalf("the worker behind claimed %d tokens, %v; want the token of the execution at once", len(claimed), err)
		}
		// Its lease has not expired early for the worker that is ahead...
		if n, err := engAhead.Reap(ctx); err != nil || n != 0 {
			t.Fatalf("the worker ahead reaped %d, %v right after the claim: the lease expired early", n, err)
		}
		// ...and the worker behind commits: its fork makes tokens that the other claims at once.
		if err := engBehind.Work(ctx, claimed[0], start); err != nil {
			t.Fatal(err)
		}
		forked, _, err := engAhead.Claim(ctx, 1)
		if err != nil || len(forked) != 1 {
			t.Fatalf("the worker ahead claimed %d tokens after the fork of the other, %v; want one at once", len(forked), err)
		}
		// A lease that is never renewed expires after its TTL for everyone, however skewed.
		time.Sleep(3 * lease)
		if n, err := engBehind.Reap(ctx); err != nil || n < 1 {
			t.Fatalf("the worker behind reaped %d, %v after the TTL: the lease expired late", n, err)
		}
		env.Drain(engBehind)
		if got := statusOf(env, execID); got != execution.ExecSucceeded {
			t.Errorf("execution is %s, want SUCCEEDED", got)
		}
	})
}
