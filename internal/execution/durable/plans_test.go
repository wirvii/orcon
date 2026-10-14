// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// revisionStore is a fake of the pinned revisions: it counts reads.
type revisionStore struct {
	mu       sync.Mutex
	ids      *id.Sequence
	versions map[flow.FlowVersionID]flow.VersionRecord
	reads    atomic.Int64
}

func newRevisionStore() *revisionStore {
	return &revisionStore{ids: id.NewSequence(), versions: make(map[flow.FlowVersionID]flow.VersionRecord)}
}

func (r *revisionStore) add(hash string, document []byte) flow.FlowVersionID {
	r.mu.Lock()
	defer r.mu.Unlock()
	version := flow.FlowVersionID(r.ids.New())
	r.versions[version] = flow.VersionRecord{ID: version, ContentHash: hash, Document: bytes.Clone(document)}
	return version
}

func (r *revisionStore) GetVersion(_ context.Context, _ identity.OrgID, v flow.FlowVersionID) (flow.VersionRecord, error) {
	r.reads.Add(1)
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.versions[v]
	if !ok {
		return flow.VersionRecord{}, flow.ErrVersionNotFound
	}
	return rec, nil
}

func (r *revisionStore) GetSnapshot(context.Context, identity.OrgID, flow.FlowSnapshotID) (flow.SnapshotRecord, error) {
	return flow.SnapshotRecord{}, flow.ErrSnapshotNotFound
}

// countingReader parses by looking the text up in a table, and counts.
type countingReader struct {
	docs    map[string]flow.Document
	version int
	parses  atomic.Int64
	fail    atomic.Bool
}

func (c *countingReader) Read(data []byte) (*flow.Document, error) {
	c.parses.Add(1)
	if c.fail.Load() {
		return nil, errors.New("unreadable document")
	}
	doc, ok := c.docs[string(data)]
	if !ok {
		return nil, fmt.Errorf("no document %q", data)
	}
	return &doc, nil
}

func (c *countingReader) Version() int { return c.version }

func tinyFlow(name string) flow.Document {
	return executiontest.NewFlow(name).Start("s").End("e").Edge("s", "e").Doc()
}

func execOf(v flow.FlowVersionID) *store.Execution {
	return &store.Execution{OrgID: identity.OrgID(id.NewSequence().New()), FlowVersionID: v}
}

// TestUpgradedDocumentCache (E-FMT-6): loading the same pinned revision twice
// parses and upgrades it once, the stored document does not change, and the
// cache keeps to its bounds of entries and bytes (ADR-0007 section 7).
func TestUpgradedDocumentCache(t *testing.T) {
	t.Run("a revision loaded twice is read once", func(t *testing.T) {
		revisions := newRevisionStore()
		stored := []byte("document-a")
		v := revisions.add("hash-a", stored)
		reader := &countingReader{docs: map[string]flow.Document{"document-a": tinyFlow("a")}, version: 1}
		plans := durable.NewPlans(revisions, reader, durable.PlanLimits{})
		first, err := plans.For(t.Context(), execOf(v))
		if err != nil {
			t.Fatal(err)
		}
		second, err := plans.For(t.Context(), execOf(v))
		if err != nil {
			t.Fatal(err)
		}
		if first != second {
			t.Error("the second load compiled the document again")
		}
		if got := reader.parses.Load(); got != 1 {
			t.Errorf("document parsed and upgraded %d times, want 1", got)
		}
		if got := revisions.reads.Load(); got != 1 {
			t.Errorf("stored revision read %d times, want 1 (the id is known to map to its hash)", got)
		}
		if !bytes.Equal(revisions.versions[v].Document, []byte("document-a")) || !bytes.Equal(stored, []byte("document-a")) {
			t.Error("the stored document changed")
		}
	})

	t.Run("two versions with the same content share the entry", func(t *testing.T) {
		revisions := newRevisionStore()
		v1, v2 := revisions.add("same-hash", []byte("document-a")), revisions.add("same-hash", []byte("document-a"))
		reader := &countingReader{docs: map[string]flow.Document{"document-a": tinyFlow("a")}, version: 1}
		plans := durable.NewPlans(revisions, reader, durable.PlanLimits{})
		for _, v := range []flow.FlowVersionID{v1, v2, v1} {
			if _, err := plans.For(t.Context(), execOf(v)); err != nil {
				t.Fatal(err)
			}
		}
		if got := reader.parses.Load(); got != 1 {
			t.Errorf("one content hash parsed %d times, want 1", got)
		}
	})

	t.Run("the cache keeps to its bound of entries and evicts the least recently used", func(t *testing.T) {
		revisions := newRevisionStore()
		docs := map[string]flow.Document{}
		var versions []flow.FlowVersionID
		for i := range 4 {
			name := fmt.Sprintf("document-%d", i)
			docs[name] = tinyFlow(name)
			versions = append(versions, revisions.add(fmt.Sprintf("hash-%d", i), []byte(name)))
		}
		reader := &countingReader{docs: docs, version: 1}
		plans := durable.NewPlans(revisions, reader, durable.PlanLimits{Entries: 2})
		load := func(i int) {
			t.Helper()
			if _, err := plans.For(t.Context(), execOf(versions[i])); err != nil {
				t.Fatal(err)
			}
		}
		load(0)
		load(1)
		load(0) // 0 is now the most recent, 1 the oldest
		load(2) // evicts 1
		if entries, _ := plans.Len(); entries != 2 {
			t.Fatalf("%d entries, want 2", entries)
		}
		before := reader.parses.Load()
		load(0)
		load(2)
		if reader.parses.Load() != before {
			t.Error("a revision that should be cached was parsed again")
		}
		load(1)
		if reader.parses.Load() != before+1 {
			t.Error("the evicted revision was not parsed again")
		}
		if entries, _ := plans.Len(); entries != 2 {
			t.Errorf("%d entries after reloading, want 2", entries)
		}
	})

	t.Run("the cache keeps to its bound of bytes", func(t *testing.T) {
		revisions := newRevisionStore()
		docs := map[string]flow.Document{}
		var versions []flow.FlowVersionID
		for i := range 5 {
			name := fmt.Sprintf("%02d-%s", i, string(bytes.Repeat([]byte("x"), 20)))
			docs[name] = tinyFlow(name)
			versions = append(versions, revisions.add(fmt.Sprintf("hash-%d", i), []byte(name)))
		}
		huge := string(bytes.Repeat([]byte("h"), 500))
		docs[huge] = tinyFlow("huge")
		hugeVersion := revisions.add("hash-huge", []byte(huge))
		reader := &countingReader{docs: docs, version: 1}
		plans := durable.NewPlans(revisions, reader, durable.PlanLimits{Bytes: 100})
		for _, v := range versions {
			if _, err := plans.For(t.Context(), execOf(v)); err != nil {
				t.Fatal(err)
			}
			if _, size := plans.Len(); size > 100 {
				t.Fatalf("cache holds %d bytes, bound is 100", size)
			}
		}
		if _, err := plans.For(t.Context(), execOf(hugeVersion)); err != nil {
			t.Fatal(err)
		}
		if _, size := plans.Len(); size > 100 {
			t.Fatalf("cache holds %d bytes after a document larger than the bound, bound is 100", size)
		}
		parses := reader.parses.Load()
		if _, err := plans.For(t.Context(), execOf(hugeVersion)); err != nil {
			t.Fatal(err)
		}
		if reader.parses.Load() != parses+1 {
			t.Error("a document larger than the bound must be read every time, and it was served from the cache")
		}
	})

	t.Run("concurrent loads of one revision read it once", func(t *testing.T) {
		revisions := newRevisionStore()
		v := revisions.add("hash-a", []byte("document-a"))
		reader := &countingReader{docs: map[string]flow.Document{"document-a": tinyFlow("a")}, version: 1}
		plans := durable.NewPlans(revisions, reader, durable.PlanLimits{})
		var wg sync.WaitGroup
		for range 32 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := plans.For(t.Context(), execOf(v)); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		if got := reader.parses.Load(); got != 1 {
			t.Errorf("32 concurrent loads parsed the document %d times, want 1", got)
		}
	})

	t.Run("a failed read is not cached", func(t *testing.T) {
		revisions := newRevisionStore()
		v := revisions.add("hash-a", []byte("document-a"))
		reader := &countingReader{docs: map[string]flow.Document{"document-a": tinyFlow("a")}, version: 1}
		reader.fail.Store(true)
		plans := durable.NewPlans(revisions, reader, durable.PlanLimits{})
		if _, err := plans.For(t.Context(), execOf(v)); err == nil {
			t.Fatal("a document that cannot be read loaded")
		}
		reader.fail.Store(false)
		if _, err := plans.For(t.Context(), execOf(v)); err != nil {
			t.Fatalf("the document did not load once it could be read: %v", err)
		}
		if entries, _ := plans.Len(); entries != 1 {
			t.Errorf("%d entries, want 1", entries)
		}
	})

	t.Run("an unknown revision is an error", func(t *testing.T) {
		plans := durable.NewPlans(newRevisionStore(), &countingReader{version: 1}, durable.PlanLimits{})
		_, err := plans.For(t.Context(), execOf(flow.FlowVersionID(id.NewSequence().New())))
		if !errors.Is(err, flow.ErrVersionNotFound) {
			t.Errorf("error %v, want flow.ErrVersionNotFound", err)
		}
	})
}
