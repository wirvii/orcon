// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package limit is the arithmetic of the shared rate buckets of external
// services (ADR-0014 section 2.4, SPEC-003 R-DECL-04). A bucket is shared by
// every flow that runs under one credential, so its state lives in the store;
// this package decides one take or block on a state the store has read, and the
// store writes the result in the same transaction.
//
// Capacity is the number of permits the bucket holds; the bucket refills at
// RefillPerSecond permits per second up to its capacity. A provider's
// Retry-After or 429 blocks the bucket until an instant, and nothing is granted
// before it.
package limit

import (
	"errors"
	"time"
)

// Spec is the configuration of a bucket.
type Spec struct {
	Capacity        int64
	RefillPerSecond int64
}

// State is the stored state of a bucket.
type State struct {
	Tokens       int64
	RefilledAt   time.Time
	BlockedUntil time.Time
}

// ErrInvalid is returned for a spec or request the arithmetic cannot serve.
var ErrInvalid = errors.New("limit: invalid bucket request")

// Validate reports a spec that cannot describe a bucket.
func (s Spec) Validate() error {
	if s.Capacity <= 0 || s.RefillPerSecond < 0 {
		return ErrInvalid
	}
	return nil
}

// NewState is the state of a bucket that has never been used: full.
func NewState(spec Spec, now time.Time) State {
	return State{Tokens: spec.Capacity, RefilledAt: now}
}

// Take tries to take permits from the bucket at instant now. It returns the new
// state, whether the permits were granted, and, when they were not, the instant
// at which a retry can succeed. A refused take changes only the refill
// bookkeeping.
func Take(st State, spec Spec, permits int64, now time.Time) (State, bool, time.Time, error) {
	if err := spec.Validate(); err != nil {
		return st, false, time.Time{}, err
	}
	if permits <= 0 || permits > spec.Capacity {
		return st, false, time.Time{}, ErrInvalid
	}
	st = refill(st, spec, now)
	if now.Before(st.BlockedUntil) {
		return st, false, st.BlockedUntil, nil
	}
	if st.Tokens >= permits {
		st.Tokens -= permits
		return st, true, time.Time{}, nil
	}
	return st, false, retryAt(st, spec, permits, now), nil
}

// Block refuses every take until the instant until (a provider's Retry-After or
// a 429). A later block replaces an earlier one only when it is later.
func Block(st State, until time.Time) State {
	if until.After(st.BlockedUntil) {
		st.BlockedUntil = until
	}
	return st
}

// refill adds the permits earned since the last refill, rounded down to whole
// permits, and moves the refill instant only by the time those permits account
// for, so no fraction is lost.
func refill(st State, spec Spec, now time.Time) State {
	if now.Before(st.RefilledAt) || spec.RefillPerSecond == 0 {
		if now.After(st.RefilledAt) {
			st.RefilledAt = now
		}
		return st
	}
	elapsed := now.Sub(st.RefilledAt)
	earned := int64(elapsed.Seconds() * float64(spec.RefillPerSecond))
	if earned <= 0 {
		return st
	}
	st.Tokens = min(spec.Capacity, st.Tokens+earned)
	// Advance the refill instant by the time the whole permits took.
	used := time.Duration(float64(earned) / float64(spec.RefillPerSecond) * float64(time.Second))
	st.RefilledAt = st.RefilledAt.Add(used)
	if st.Tokens == spec.Capacity {
		st.RefilledAt = now
	}
	return st
}

// retryAt is the earliest instant at which the bucket holds permits.
func retryAt(st State, spec Spec, permits int64, now time.Time) time.Time {
	if spec.RefillPerSecond == 0 {
		return time.Time{}
	}
	missing := permits - st.Tokens
	wait := time.Duration(float64(missing) / float64(spec.RefillPerSecond) * float64(time.Second))
	return now.Add(wait)
}
