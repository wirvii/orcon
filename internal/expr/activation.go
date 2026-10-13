// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"math/rand/v2"
	"sync"
	"time"
)

// SeedSize is the size in bytes of an activation seed.
const SeedSize = 32

// Activation holds the recorded non-determinism of one node activation
// (ADR-0013 section 7, ADR-0018 section 5): the activation time, constant
// during the activation, and the stream of random values seeded by the
// recorded seed. Evaluating the same expressions in the same order against
// an activation rebuilt from the same time and seed gives identical results,
// which is what recomputation after a crash relies on.
//
// An Activation is safe for concurrent use; the order of draws is the order
// of the calls.
type Activation struct {
	at time.Time

	mu  sync.Mutex
	rng *rand.ChaCha8
}

// NewActivation returns the activation recorded with time at and the given
// seed. The same pair always yields the same stream.
func NewActivation(at time.Time, seed [SeedSize]byte) *Activation {
	return &Activation{at: at.UTC(), rng: rand.NewChaCha8(seed)}
}

// Time returns the activation time. $now, $today and Date.now() resolve to it.
func (a *Activation) Time() time.Time { return a.at }

// Read fills p with the next random bytes of the stream.
func (a *Activation) Read(p []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	// ChaCha8.Read never returns an error.
	_, _ = a.rng.Read(p)
}

// Float64 returns the next random number in [0, 1), the value Math.random()
// takes. It uses the 53 high bits of the next 64.
func (a *Activation) Float64() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return float64(a.rng.Uint64()>>11) / (1 << 53)
}
