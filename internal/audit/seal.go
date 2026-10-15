// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

// Sealing limits (R-AUD-03): one batch holds at most this many pending entries.
const (
	// MaxBatch is the most entries one block seals.
	MaxBatch = 1000
	// Each pass visits at most this many chains.
	maxChainsPerPass = 100
)

// Sealer turns pending entries into sealed events. It is safe to run in one process
// per chain; a second sealer that reads a stale tail fails its commit with
// [ErrTailMoved] and retries on its next pass, so no event is sealed twice.
type Sealer struct {
	store Store
	keys  KeySource
	ids   id.Generator
	clock clock.Clock
}

// NewSealer returns a sealer over its ports.
func NewSealer(store Store, keys KeySource, ids id.Generator, clk clock.Clock) *Sealer {
	return &Sealer{store: store, keys: keys, ids: ids, clock: clk}
}

// SealOnce seals at most one batch of every chain that has pending entries and
// returns how many events it sealed. A chain that another sealer moved meanwhile is
// skipped, not an error. A chain that fails does not stop the others: the pass goes on
// and returns the failures joined, so one broken organization cannot hold back the
// audit entries of every organization after it.
func (s *Sealer) SealOnce(ctx context.Context) (int, error) {
	chains, err := s.store.PendingChains(ctx, maxChainsPerPass)
	if err != nil {
		return 0, fmt.Errorf("seal audit: %w", err)
	}
	total := 0
	var failures []error
	for _, chain := range chains {
		n, err := s.sealChain(ctx, chain)
		if errors.Is(err, ErrTailMoved) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("seal audit of %s: %w", chain.Label(), err))
			if ctx.Err() != nil {
				break
			}
			continue
		}
		total += n
	}
	return total, errors.Join(failures...)
}

// sealChain seals one batch of one chain.
func (s *Sealer) sealChain(ctx context.Context, chain Chain) (int, error) {
	pending, err := s.store.ListPending(ctx, chain, MaxBatch)
	if err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}
	tail, err := s.store.Tail(ctx, chain)
	if err != nil {
		return 0, err
	}
	version, key, err := s.keys.ActiveMACKey(ctx, chain)
	if err != nil {
		return 0, fmt.Errorf("mac key: %w", err)
	}
	defer clear(key)
	if len(key) != 32 {
		return 0, fmt.Errorf("audit: the MAC key must be 32 bytes, got %d", len(key))
	}
	events := make([]Sealed, 0, len(pending))
	prev := tail.Hash
	seq := tail.Sequence
	pendingIDs := make([]id.UUID, 0, len(pending))
	for i := range pending {
		seq++
		h := chainHash(prev, seq, pending[i].Body)
		events = append(events, Sealed{Sequence: seq, PreviousHash: prev, Hash: h, Entry: pending[i]})
		pendingIDs = append(pendingIDs, pending[i].ID)
		prev = h
	}
	last := events[len(events)-1]
	block := Block{
		ID: s.ids.New(), Chain: chain, FirstSequenceNumber: tail.Sequence + 1,
		LastSequenceNumber: last.Sequence, HeadHash: last.Hash,
		MAC:        blockMAC(key, chain, last.Sequence, last.Hash),
		KeyVersion: version, SealedAt: s.clock.Now().UTC().Truncate(time.Microsecond),
	}
	if err := s.store.CommitBlock(ctx, chain, tail, Batch{Block: block, Events: events, PendingIDs: pendingIDs}); err != nil {
		return 0, err
	}
	return len(events), nil
}
