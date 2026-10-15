// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package idempotency

import (
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/platform/clock"
)

// TTL is how long a record is kept (ADR-0053 section 4).
const TTL = 24 * time.Hour

// defaultMaxRecords bounds the memory of a [MemoryStore].
const defaultMaxRecords = 100_000

// ErrStoreFull means the store holds as many live records as it may.
var ErrStoreFull = errors.New("the idempotency store is full")

// ErrNotClaimed means a key was completed or abandoned without being claimed.
var ErrNotClaimed = errors.New("the idempotency key is not claimed")

// MemoryStore is a [Store] that keeps its records in the memory of the process.
//
// It survives neither a restart nor a second api process, so it is the store of
// tests and of a single-process development run. The durable store writes the
// record in the same transaction as the state change (ADR-0053 section 4) and
// belongs to the storage adapters.
type MemoryStore struct {
	clk        clock.Clock
	maxRecords int
	mu         sync.Mutex
	records    map[Key]*entry
}

type entry struct {
	record    Record
	expiresAt time.Time
}

// NewMemoryStore returns an empty store that expires records after [TTL].
func NewMemoryStore(clk clock.Clock) *MemoryStore {
	return &MemoryStore{clk: clk, maxRecords: defaultMaxRecords, records: map[Key]*entry{}}
}

// Begin implements [Store].
func (s *MemoryStore) Begin(_ context.Context, key Key, fingerprint [sha256.Size]byte) (Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clk.Now()
	if e, ok := s.records[key]; ok {
		if now.Before(e.expiresAt) {
			return e.record, false, nil
		}
		delete(s.records, key)
	}
	if len(s.records) >= s.maxRecords {
		s.sweep(now)
		if len(s.records) >= s.maxRecords {
			return Record{}, false, ErrStoreFull
		}
	}
	s.records[key] = &entry{record: Record{Fingerprint: fingerprint}, expiresAt: now.Add(TTL)}
	return Record{}, true, nil
}

// Complete implements [Store].
func (s *MemoryStore) Complete(_ context.Context, key Key, resp *Response) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.records[key]
	if !ok || e.record.Completed {
		return ErrNotClaimed
	}
	e.record.Completed = true
	e.record.Response = *resp
	return nil
}

// Abandon implements [Store].
func (s *MemoryStore) Abandon(_ context.Context, key Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.records[key]; ok && !e.record.Completed {
		delete(s.records, key)
	}
	return nil
}

// Peek returns the record of key, for tests that inspect what is stored.
func (s *MemoryStore) Peek(key Key) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.records[key]
	if !ok {
		return Record{}, false
	}
	return e.record, true
}

// sweep drops expired records.
func (s *MemoryStore) sweep(now time.Time) {
	for k, e := range s.records {
		if !now.Before(e.expiresAt) {
			delete(s.records, k)
		}
	}
}
