// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Version is an immutable published revision of a flow (ADR-0007 section 6).
// It holds the document as an immutable JSON value, so neither the caller's
// draft nor anything obtained from the version can change it.
type Version struct {
	// Number is 1, 2, 3, ... in publication order; history is linear.
	Number int
	// ContentHash is the hash of the semantic document.
	ContentHash Hash
	// Warnings are the validation warnings recorded at publication.
	Warnings []Issue
	doc      jsonvalue.Value
}

// Value returns the published document in JSON form.
func (v Version) Value() jsonvalue.Value { return v.doc }

// Document returns the published document.
func (v Version) Document() Document { return documentOf(v.doc) }

// Snapshot is the immutable copy of a draft frozen by a manual test run. It is
// keyed by content hash, so running the same content twice shares one
// snapshot, and it carries no version number.
type Snapshot struct {
	ContentHash Hash
	doc         jsonvalue.Value
}

// Value returns the frozen document in JSON form.
func (s Snapshot) Value() jsonvalue.Value { return s.doc }

// Document returns the frozen document.
func (s Snapshot) Document() Document { return documentOf(s.doc) }

// documentOf maps a value that already passed validation back to a Document.
func documentOf(v jsonvalue.Value) Document {
	doc, err := FromValue(v)
	if err != nil {
		// Versions and snapshots are built only from documents that read cleanly.
		panic("flow: stored revision does not read back: " + err.Error())
	}
	return doc
}

// Revision pins an execution to exactly one immutable revision of a flow:
// a published version or a snapshot. The zero Revision is invalid.
type Revision struct {
	version  int
	snapshot Hash
	isSnap   bool
}

// VersionRevision pins a published version.
func VersionRevision(number int) Revision { return Revision{version: number} }

// SnapshotRevision pins a snapshot.
func SnapshotRevision(h Hash) Revision { return Revision{snapshot: h, isSnap: true} }

// Version returns the pinned version number, or false for a snapshot revision.
func (r Revision) Version() (int, bool) { return r.version, !r.isSnap && r.version > 0 }

// Snapshot returns the pinned snapshot hash, or false for a version revision.
func (r Revision) Snapshot() (Hash, bool) { return r.snapshot, r.isSnap }

// Flow is the in-memory model of one flow: a single mutable draft, its
// published versions and its test snapshots. The storage adapters persist
// this shape; the rules live here so every backend applies the same ones.
// A Flow is not safe for concurrent use.
type Flow struct {
	id            ID
	draft         jsonvalue.Value
	draftRevision int
	versions      []Version
	snapshots     map[Hash]Snapshot
}

// NewFlow starts a flow with an empty draft at draft revision 0.
func NewFlow(id ID) *Flow {
	return &Flow{id: id, draft: (&builder{}).build(), snapshots: map[Hash]Snapshot{}}
}

// ID returns the flow id.
func (f *Flow) ID() ID { return f.id }

// Draft returns the draft and its revision. Callers pass the revision back to
// [Flow.SaveDraft] so concurrent edits are detected.
func (f *Flow) Draft() (jsonvalue.Value, int) { return f.draft, f.draftRevision }

// SaveDraft replaces the draft if expectedRevision is the current draft
// revision (optimistic concurrency) and returns the new revision. A stale
// revision fails with [ErrDraftConflict] and changes nothing. The draft may be
// incomplete or invalid: validation happens when it is published or frozen
// for a test run.
func (f *Flow) SaveDraft(expectedRevision int, draft jsonvalue.Value) (int, error) {
	if expectedRevision != f.draftRevision {
		return f.draftRevision, fmt.Errorf("save draft of flow %s at revision %d: %w", f.id, expectedRevision, ErrDraftConflict)
	}
	f.draft = draft
	f.draftRevision++
	return f.draftRevision, nil
}

// Publish validates the draft and, if it has no errors, appends it as the
// next immutable version (maximum number plus one), recording the warnings.
// Publishing identical content again still creates a new version.
func (f *Flow) Publish() (Version, error) {
	return f.publish(f.draft, nil)
}

// PublishWith is [Flow.Publish] with the node-type rules of validation level 5
// applied: a node type the catalog does not know, an invalid safety contract
// and a connector pin that is not installed, not granted or blocked by policy
// all stop the publication (ADR-0007 section 9).
func (f *Flow) PublishWith(ctx context.Context, types NodeTypes) (Version, error) {
	return f.publish(f.draft, &typeCheck{ctx: ctx, types: types})
}

// Rollback publishes the content of an earlier version as a new version;
// version history is linear and nothing is ever edited or deleted. The draft
// becomes that content, so the editor shows what is live.
func (f *Flow) Rollback(number int) (Version, error) {
	return f.rollback(number, nil)
}

// RollbackWith is [Flow.Rollback] with the node-type rules of validation
// level 5 applied to the content being republished, so a version that pins a
// connector that is no longer installed or allowed cannot go live again.
func (f *Flow) RollbackWith(ctx context.Context, number int, types NodeTypes) (Version, error) {
	return f.rollback(number, &typeCheck{ctx: ctx, types: types})
}

func (f *Flow) rollback(number int, tc *typeCheck) (Version, error) {
	old, err := f.Version(number)
	if err != nil {
		return Version{}, fmt.Errorf("roll back flow %s: %w", f.id, err)
	}
	published, err := f.publish(old.doc, tc)
	if err != nil {
		return Version{}, fmt.Errorf("roll back flow %s to version %d: %w", f.id, number, err)
	}
	f.draft = old.doc
	f.draftRevision++
	return published, nil
}

// typeCheck carries the node-type catalog and the context its lookups use.
type typeCheck struct {
	ctx   context.Context //nolint:containedctx // scoped to one validation call, never stored
	types NodeTypes
}

// validated reads and validates a draft value, returning the normalized
// document value and the report. A draft that does not read cleanly (wrong
// shape) fails with its structure errors. A nil tc skips the node-type rules.
func validated(v jsonvalue.Value, tc *typeCheck) (jsonvalue.Value, Report, error) {
	doc, err := FromValue(v)
	if err != nil {
		return jsonvalue.Value{}, Report{}, err
	}
	var report Report
	if tc == nil {
		report = Validate(&doc)
	} else {
		report = ValidateWithTypes(tc.ctx, &doc, tc.types)
	}
	if err := report.Err(); err != nil {
		return jsonvalue.Value{}, report, err
	}
	return doc.Value(), report, nil
}

func (f *Flow) publish(draft jsonvalue.Value, tc *typeCheck) (Version, error) {
	normalized, report, err := validated(draft, tc)
	if err != nil {
		return Version{}, fmt.Errorf("publish flow %s: %w", f.id, err)
	}
	hash, err := ContentHashOfValue(normalized)
	if err != nil {
		return Version{}, fmt.Errorf("publish flow %s: %w", f.id, err)
	}
	v := Version{
		Number:      len(f.versions) + 1,
		ContentHash: hash,
		Warnings:    report.Warnings(),
		doc:         normalized,
	}
	f.versions = append(f.versions, v)
	return v, nil
}

// Version returns the published version with the given number.
func (f *Flow) Version(number int) (Version, error) {
	if number < 1 || number > len(f.versions) {
		return Version{}, fmt.Errorf("flow %s version %d: %w", f.id, number, ErrVersionNotFound)
	}
	return f.versions[number-1], nil
}

// Versions returns the published versions in publication order.
func (f *Flow) Versions() []Version {
	out := make([]Version, len(f.versions))
	copy(out, f.versions)
	return out
}

// Active returns the newest published version, the only one automatic
// triggers start. It reports false while nothing is published.
func (f *Flow) Active() (Version, bool) {
	if len(f.versions) == 0 {
		return Version{}, false
	}
	return f.versions[len(f.versions)-1], true
}

// Snapshot freezes the draft for a manual test run. It needs a draft without
// validation errors. The snapshot is keyed by content hash: freezing the same
// content again returns the existing snapshot instead of creating another.
func (f *Flow) Snapshot() (Snapshot, error) {
	return f.snapshot(nil)
}

// SnapshotWith is [Flow.Snapshot] with the node-type rules of validation
// level 5 applied.
func (f *Flow) SnapshotWith(ctx context.Context, types NodeTypes) (Snapshot, error) {
	return f.snapshot(&typeCheck{ctx: ctx, types: types})
}

func (f *Flow) snapshot(tc *typeCheck) (Snapshot, error) {
	normalized, _, err := validated(f.draft, tc)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot flow %s: %w", f.id, err)
	}
	hash, err := ContentHashOfValue(normalized)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot flow %s: %w", f.id, err)
	}
	if existing, ok := f.snapshots[hash]; ok {
		return existing, nil
	}
	s := Snapshot{ContentHash: hash, doc: normalized}
	f.snapshots[hash] = s
	return s, nil
}

// SnapshotCount returns how many distinct snapshots exist.
func (f *Flow) SnapshotCount() int { return len(f.snapshots) }

// Resolve returns the document an execution pinned to rev must run. Whatever
// is published or frozen later never changes the answer for an existing
// revision (ADR-0007 section 6: an execution keeps its revision until it ends).
func (f *Flow) Resolve(rev Revision) (Document, error) {
	if n, ok := rev.Version(); ok {
		v, err := f.Version(n)
		if err != nil {
			return Document{}, err
		}
		return v.Document(), nil
	}
	if h, ok := rev.Snapshot(); ok {
		s, found := f.snapshots[h]
		if !found {
			return Document{}, fmt.Errorf("flow %s snapshot %s: %w", f.id, h, ErrSnapshotNotFound)
		}
		return s.Document(), nil
	}
	return Document{}, fmt.Errorf("flow %s: %w", f.id, ErrInvalidRevision)
}
