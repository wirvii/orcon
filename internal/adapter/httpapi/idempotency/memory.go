// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package idempotency

import (
	"context"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/idempotencyclaim"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// MemoryStore is an [identity.IdempotencyStore] that keeps its records in the memory of the
// process. It is the store of tests and of single-process development runs: it survives
// neither a restart nor a second api process, so the composition root never wires it.
//
// Claim stands in for the claim of a transaction: a test handler calls it where a store
// would write the record in the transaction of its change.
type MemoryStore struct {
	clk     clock.Clock
	mu      sync.Mutex
	records map[memoryKey]identity.IdempotencyRecord
}

type memoryKey struct {
	org       identity.OrgID
	requester string
	value     string
}

// NewMemoryStore returns an empty store.
func NewMemoryStore(clk clock.Clock) *MemoryStore {
	return &MemoryStore{clk: clk, records: map[memoryKey]identity.IdempotencyRecord{}}
}

var _ identity.IdempotencyStore = (*MemoryStore)(nil)

// Claim writes the record of the claim carried by ctx, in the organization, as a transactional
// store would. Without a claim it does nothing; a live record of the key fails with
// [idempotencyclaim.ErrTaken].
func (s *MemoryStore) Claim(ctx context.Context, org orgscope.Org, resourceRef string) error {
	claim, ok := idempotencyclaim.From(ctx)
	if !ok {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clk.Now()
	key := memoryKey{org: identity.OrgIDOf(org), requester: claim.Requester, value: claim.Value}
	if old, ok := s.records[key]; ok && old.CreatedAt.Add(identity.IdempotencyTTL).After(now) {
		return idempotencyclaim.ErrTaken
	}
	s.records[key] = identity.IdempotencyRecord{
		Fingerprint: claim.Fingerprint, ResourceRef: resourceRef, CreatedAt: now,
	}
	return nil
}

// LookupIdempotency implements [identity.IdempotencyStore]. A record past its expiry is absent.
func (s *MemoryStore) LookupIdempotency(_ context.Context, org orgscope.Org, key identity.IdempotencyKey, now time.Time) (identity.IdempotencyRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[memoryKey{org: identity.OrgIDOf(org), requester: key.Requester, value: key.Value}]
	if !ok || !rec.CreatedAt.Add(identity.IdempotencyTTL).After(now) {
		return identity.IdempotencyRecord{}, false, nil
	}
	return rec, true, nil
}

// CompleteIdempotency implements [identity.IdempotencyStore]. It completes the record that
// [MemoryStore.Claim] wrote, or stores a completed one when there is none.
func (s *MemoryStore) CompleteIdempotency(_ context.Context, org orgscope.Org, key identity.IdempotencyKey, c identity.IdempotencyCompletion, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mk := memoryKey{org: identity.OrgIDOf(org), requester: key.Requester, value: key.Value}
	rec, ok := s.records[mk]
	if ok && rec.Completed {
		return nil
	}
	if !ok {
		rec = identity.IdempotencyRecord{Fingerprint: c.Fingerprint, CreatedAt: now}
	}
	rec.Completed = true
	rec.Response = c.Response
	s.records[mk] = rec
	return nil
}
