// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
)

// Causes a lease context is canceled with.
var (
	// ErrLeaseLost means the renewal found the lease taken: it expired and the
	// reaper or another worker has the token.
	ErrLeaseLost = errors.New("durable: the lease was lost")
	// ErrLeaseStale means the worker could not renew its leases for so long that
	// it must assume they expired (local self-fencing, ADR-0013 section 4).
	ErrLeaseStale = errors.New("durable: the lease may have expired; the worker fenced itself")
)

// heldLease is a lease this process holds.
type heldLease struct {
	fence store.Fence
	// registered is when the lease was entered in the table, so that a renewal
	// that started before does not count it lost.
	registered time.Time
	// validUntil is the instant, on the monotonic clock, after which the worker
	// treats the lease as expired whatever the database says: the start of the
	// last successful renewal plus the TTL, less a margin.
	validUntil time.Time
	cancel     context.CancelCauseFunc
}

// leaseTable keeps the leases a process holds. The lifetime of a lease is
// judged on the monotonic clock of the process: wall-clock skew on a worker has
// no effect (ADR-0013 section 4), and a process that was stopped and continued
// finds its leases stale before it commits anything.
type leaseTable struct {
	ttl    time.Duration
	margin time.Duration

	mu   sync.Mutex
	held map[store.TokenID]*heldLease
}

func newLeaseTable(ttl time.Duration) *leaseTable {
	margin := 2 * time.Second
	if q := ttl / 4; q < margin {
		margin = q
	}
	return &leaseTable{ttl: ttl, margin: margin, held: make(map[store.TokenID]*heldLease)}
}

// add registers a lease granted by a claim or created by a commit that started
// at start.
func (t *leaseTable) add(parent context.Context, f store.Fence, start time.Time) (context.Context, *heldLease) {
	ctx, cancel := context.WithCancelCause(parent)
	l := &heldLease{fence: f, registered: time.Now(), validUntil: start.Add(t.ttl - t.margin), cancel: cancel}
	t.mu.Lock()
	t.held[f.Token] = l
	t.mu.Unlock()
	return ctx, l
}

// drop forgets a lease.
func (t *leaseTable) drop(l *heldLease) {
	t.mu.Lock()
	if t.held[l.fence.Token] == l {
		delete(t.held, l.fence.Token)
	}
	t.mu.Unlock()
	l.cancel(nil)
}

// extend re-registers the lease under a new fence after a commit that kept a
// token: the same token keeps its epoch, a child starts at epoch 1.
func (t *leaseTable) extend(l *heldLease, f store.Fence, start time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.held[l.fence.Token] == l {
		delete(t.held, l.fence.Token)
	}
	l.fence = f
	l.registered = time.Now()
	l.validUntil = start.Add(t.ttl - t.margin)
	t.held[f.Token] = l
}

// valid reports whether the worker may still act on the lease.
func (l *heldLease) valid(t *leaseTable) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return time.Now().Before(l.validUntil)
}

// renewed applies the answer of a renewal that started at start: leases it
// returned are valid until start + TTL; leases it did not return, and that were
// registered before it started, are lost.
func (t *leaseTable) renewed(start time.Time, got []store.Renewed) (lost int) {
	have := make(map[store.TokenID]int64, len(got))
	for _, r := range got {
		have[r.TokenID] = r.LeaseEpoch
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for tok, l := range t.held {
		epoch, ok := have[tok]
		switch {
		case ok && epoch == l.fence.Epoch:
			l.validUntil = start.Add(t.ttl - t.margin)
		case l.registered.Before(start):
			l.cancel(ErrLeaseLost)
			lost++
		}
	}
	return lost
}

// stale cancels the leases whose validity ran out because renewals failed.
func (t *leaseTable) stale() (n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for _, l := range t.held {
		if !now.Before(l.validUntil) {
			l.cancel(ErrLeaseStale)
			n++
		}
	}
	return n
}

// count returns how many leases the process holds.
func (t *leaseTable) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.held)
}
