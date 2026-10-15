// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package dispatch_test runs the shared rate buckets of external services
// against the real stores (SPEC-003 CN3-dispatch, ADR-0014 section 2.4).
package dispatch_test

import (
	"flag"
	"os"
	"slices"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
)

func TestMain(m *testing.M) {
	flag.Parse()
	kinds, _ := storetest.KindsFor(executiontest.StoreFlag("auto"))
	if slices.Contains(kinds, storetest.PostgreSQL) {
		storetest.EnablePostgres()
		os.Exit(runWithPostgres(m)) //nolint:forbidigo // AL-8: TestMain ends the test process with the code of the run
	}
	goleak.VerifyTestMain(m)
}

func eachStore(t *testing.T, fn func(t *testing.T, env *storetest.EngineEnv)) {
	t.Helper()
	kinds, err := storetest.KindsFor(executiontest.StoreFlag("auto"))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Helper()
			fn(t, storetest.NewEngineEnv(t, kind, storetest.EngineOptions{Weak: true}))
		})
	}
}

// TestBucketsAreSharedAndRefillAndBlock: the bucket of one credential and host
// grants its capacity, refuses with the instant of the next permit, refills with
// time, and holds a block until its instant.
func TestBucketsAreSharedAndRefillAndBlock(t *testing.T) {
	eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		var buckets store.LimitBuckets = env.Store
		now := time.Now().UTC().Truncate(time.Microsecond)
		req := store.BucketRequest{Key: "http:cred-1:api.example.com", Capacity: 2, RefillPerSecond: 1, Permits: 1, Now: now}

		for i := range 2 {
			v, err := buckets.TakeBucketPermits(t.Context(), env.Org, req)
			if err != nil || !v.Granted {
				t.Fatalf("take %d: %+v, %v", i, v, err)
			}
		}
		v, err := buckets.TakeBucketPermits(t.Context(), env.Org, req)
		if err != nil || v.Granted {
			t.Fatalf("third take at the same instant: %+v, %v", v, err)
		}
		if !v.RetryAt.After(now) {
			t.Fatalf("retry instant %s is not after %s", v.RetryAt, now)
		}

		later := req
		later.Now = now.Add(2 * time.Second)
		if v, err := buckets.TakeBucketPermits(t.Context(), env.Org, later); err != nil || !v.Granted {
			t.Fatalf("take after refill: %+v, %v", v, err)
		}

		if err := buckets.BlockBucket(t.Context(), env.Org, req.Key, now.Add(time.Hour), now); err != nil {
			t.Fatalf("BlockBucket: %v", err)
		}
		blocked := later
		blocked.Now = now.Add(3 * time.Second)
		if v, err := buckets.TakeBucketPermits(t.Context(), env.Org, blocked); err != nil || v.Granted {
			t.Fatalf("take during a block: %+v, %v", v, err)
		}
	})
}
