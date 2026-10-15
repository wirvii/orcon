// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"container/list"
	"context"
	"fmt"
	"sync"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// Plans loads the pinned revision of an execution and compiles it, keeping the
// upgraded document in memory by content hash (ADR-0007 section 7: stored
// documents are never rewritten; the upgraded form is cached by contentHash).
//
// The cache is keyed by the content hash and the version of the reader, because
// a document upgraded by one reader is not the document another produces. It is
// bounded by entries and by the bytes of the stored documents, evicting the
// least recently used. Loading the same revision again reads, parses, upgrades
// and compiles nothing. A revision is immutable, so the mapping from a version or
// snapshot id to its content hash is kept as well, and a second load does not
// even read the stored document.
type Plans struct {
	revisions Revisions
	scopes    store.Scopes
	reader    DocumentReader
	maxEntry  int
	maxBytes  int

	mu     sync.Mutex
	byHash map[planKey]*list.Element
	lru    *list.List
	bytes  int
	// ids maps a revision id to its content hash: revisions never change.
	ids map[revisionKey]string
	// loading makes concurrent loads of one revision share the work.
	loading map[planKey]*pendingLoad
}

type planKey struct {
	hash   string
	reader int
}

type revisionKey struct {
	snapshot bool
	id       [16]byte
}

type planEntry struct {
	key  planKey
	plan *execution.Plan
	size int
}

type pendingLoad struct {
	done chan struct{}
	plan *execution.Plan
	err  error
}

// PlanLimits bound the cache of upgraded documents.
type PlanLimits struct {
	// Entries is the largest number of documents kept; 256 when zero.
	Entries int
	// Bytes is the largest sum of the stored sizes of the documents kept; 64 MiB
	// when zero.
	Bytes int
}

// NewPlans returns a loader over a revision store, the scopes of its organizations
// and a document reader.
func NewPlans(revisions Revisions, scopes store.Scopes, reader DocumentReader, limits PlanLimits) *Plans {
	if limits.Entries <= 0 {
		limits.Entries = 256
	}
	if limits.Bytes <= 0 {
		limits.Bytes = 64 << 20
	}
	return &Plans{
		scopes:    scopes,
		revisions: revisions, reader: reader, maxEntry: limits.Entries, maxBytes: limits.Bytes,
		byHash: make(map[planKey]*list.Element), lru: list.New(), ids: make(map[revisionKey]string),
		loading: make(map[planKey]*pendingLoad),
	}
}

// Len reports the documents in the cache and the bytes they account for.
func (p *Plans) Len() (entries, bytes int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lru.Len(), p.bytes
}

// For returns the plan of the revision an execution is pinned to.
func (p *Plans) For(ctx context.Context, e *store.Execution) (*execution.Plan, error) {
	if !e.FlowSnapshotID.IsZero() {
		return p.load(ctx, e.OrgID, revisionKey{snapshot: true, id: [16]byte(e.FlowSnapshotID)})
	}
	return p.load(ctx, e.OrgID, revisionKey{id: [16]byte(e.FlowVersionID)})
}

// Version returns the plan of a published version.
func (p *Plans) Version(ctx context.Context, org identity.OrgID, version flow.FlowVersionID) (*execution.Plan, error) {
	return p.load(ctx, org, revisionKey{id: [16]byte(version)})
}

func (p *Plans) load(ctx context.Context, org identity.OrgID, rk revisionKey) (*execution.Plan, error) {
	p.mu.Lock()
	if hash, ok := p.ids[rk]; ok {
		if plan, ok := p.touch(planKey{hash, p.reader.Version()}); ok {
			p.mu.Unlock()
			return plan, nil
		}
	}
	p.mu.Unlock()
	hash, document, err := p.fetch(ctx, org, rk)
	if err != nil {
		return nil, err
	}
	key := planKey{hash, p.reader.Version()}
	p.mu.Lock()
	p.ids[rk] = hash
	if plan, ok := p.touch(key); ok {
		p.mu.Unlock()
		return plan, nil
	}
	if pending, ok := p.loading[key]; ok {
		p.mu.Unlock()
		select {
		case <-pending.done:
			return pending.plan, pending.err
		case <-ctx.Done():
			return nil, fmt.Errorf("load revision %s: %w", hash, ctx.Err())
		}
	}
	pending := &pendingLoad{done: make(chan struct{})}
	p.loading[key] = pending
	p.mu.Unlock()

	plan, err := p.compile(document)
	p.mu.Lock()
	delete(p.loading, key)
	if err == nil {
		p.insert(key, plan, len(document))
	}
	p.mu.Unlock()
	pending.plan, pending.err = plan, err
	close(pending.done)
	return plan, err
}

// fetch reads the stored revision.
func (p *Plans) fetch(ctx context.Context, org identity.OrgID, rk revisionKey) (hash string, document []byte, err error) {
	if rk.snapshot {
		scope, err := engineScope(p.scopes, org)
		if err != nil {
			return "", nil, err
		}
		rec, err := p.revisions.GetSnapshot(ctx, scope, flow.FlowSnapshotID(rk.id))
		if err != nil {
			return "", nil, fmt.Errorf("load snapshot %s: %w", flow.FlowSnapshotID(rk.id), err)
		}
		return rec.ContentHash, rec.Document, nil
	}
	scope, err := engineScope(p.scopes, org)
	if err != nil {
		return "", nil, err
	}
	rec, err := p.revisions.GetVersion(ctx, scope, flow.FlowVersionID(rk.id))
	if err != nil {
		return "", nil, fmt.Errorf("load version %s: %w", flow.FlowVersionID(rk.id), err)
	}
	return rec.ContentHash, rec.Document, nil
}

func (p *Plans) compile(document []byte) (*execution.Plan, error) {
	doc, err := p.reader.Read(document)
	if err != nil {
		return nil, fmt.Errorf("read flow document: %w", err)
	}
	plan, err := execution.Compile(doc)
	if err != nil {
		return nil, fmt.Errorf("compile flow document: %w", err)
	}
	return plan, nil
}

// touch returns the cached plan of key and marks it recently used. The caller
// holds the mutex.
func (p *Plans) touch(key planKey) (*execution.Plan, bool) {
	el, ok := p.byHash[key]
	if !ok {
		return nil, false
	}
	p.lru.MoveToFront(el)
	entry, _ := el.Value.(*planEntry)
	return entry.plan, true
}

// insert adds a plan and evicts what the bounds no longer allow. The caller
// holds the mutex. A document larger than the byte bound is not kept at all.
func (p *Plans) insert(key planKey, plan *execution.Plan, size int) {
	if size > p.maxBytes {
		return
	}
	p.byHash[key] = p.lru.PushFront(&planEntry{key: key, plan: plan, size: size})
	p.bytes += size
	for p.lru.Len() > p.maxEntry || p.bytes > p.maxBytes {
		oldest := p.lru.Back()
		if oldest == nil {
			return
		}
		entry, _ := oldest.Value.(*planEntry)
		p.lru.Remove(oldest)
		delete(p.byHash, entry.key)
		p.bytes -= entry.size
	}
}
