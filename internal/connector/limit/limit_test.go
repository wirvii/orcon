// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package limit

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// TestRateLimitBuckets: capacity is shared, refill is by time, a block holds
// until its instant, and a refusal says when to retry.
func TestRateLimitBuckets(t *testing.T) {
	spec := Spec{Capacity: 3, RefillPerSecond: 1}
	st := NewState(spec, t0)

	for i := range 3 {
		var ok bool
		var err error
		st, ok, _, err = Take(st, spec, 1, t0)
		if err != nil || !ok {
			t.Fatalf("take %d: ok=%v err=%v", i, ok, err)
		}
	}
	st, ok, retry, err := Take(st, spec, 1, t0)
	if err != nil || ok {
		t.Fatalf("fourth take at the same instant granted: ok=%v err=%v", ok, err)
	}
	if want := t0.Add(time.Second); !retry.Equal(want) {
		t.Fatalf("retry = %s, want %s", retry, want)
	}
	// One second later one permit has been refilled.
	if _, ok, _, _ := Take(st, spec, 1, t0.Add(time.Second)); !ok {
		t.Fatal("no permit after one refill second")
	}

	// A block refuses everything until its instant, whatever the tokens.
	blocked := Block(NewState(spec, t0), t0.Add(10*time.Second))
	if _, ok, until, _ := Take(blocked, spec, 1, t0.Add(5*time.Second)); ok || !until.Equal(t0.Add(10*time.Second)) {
		t.Fatalf("blocked take: ok=%v until=%s", ok, until)
	}
	if _, ok, _, _ := Take(blocked, spec, 1, t0.Add(10*time.Second)); !ok {
		t.Fatal("take refused at the block instant")
	}
}

// TestRefillNeverExceedsCapacity: a long idle time refills to capacity, not more.
func TestRefillNeverExceedsCapacity(t *testing.T) {
	spec := Spec{Capacity: 2, RefillPerSecond: 10}
	st := NewState(spec, t0)
	st, _, _, _ = Take(st, spec, 2, t0)
	st, ok, _, _ := Take(st, spec, 2, t0.Add(time.Hour))
	if !ok {
		t.Fatal("full bucket refused")
	}
	if st.Tokens != 0 {
		t.Fatalf("tokens after taking the full capacity = %d", st.Tokens)
	}
}

func TestInvalidRequests(t *testing.T) {
	spec := Spec{Capacity: 1, RefillPerSecond: 1}
	if _, _, _, err := Take(NewState(spec, t0), spec, 2, t0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("more permits than capacity: %v", err)
	}
	if _, _, _, err := Take(NewState(spec, t0), Spec{}, 1, t0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("zero spec: %v", err)
	}
}
