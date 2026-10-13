// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// newDraftFlow returns a flow whose draft is the named conformance flow.
func newDraftFlow(t *testing.T, name string) *flow.Flow {
	t.Helper()
	f := flow.NewFlow("linear")
	if _, err := f.SaveDraft(0, conformanceValue(t, name)); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	return f
}

// withName returns the linear flow with a different description, so two drafts
// differ in content.
func withDescription(t *testing.T, desc string) jsonvalue.Value {
	t.Helper()
	return conformanceValue(t, "linear").With("description", jsonvalue.String(desc))
}

func TestContentHash(t *testing.T) {
	t.Parallel()
	t.Run("is SHA-256 over the canonical JSON without editor", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "linear")
		got, err := flow.ContentHash(&doc)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := jsonvalue.Canonical(doc.Value().Without("editor"))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(canonical)
		if got != flow.Hash(sum) || got.String() != hex.EncodeToString(sum[:]) {
			t.Errorf("ContentHash = %s, want %x", got, sum)
		}
		if strings.Contains(string(canonical), `"editor"`) {
			t.Error("the hashed text still holds the editor member")
		}
	})
	t.Run("ignores editor data", func(t *testing.T) {
		t.Parallel()
		a := load(t, "linear")
		b := load(t, "linear")
		b.Editor = obj(jsonvalue.Member{Key: "zoom", Value: jsonvalue.Int(3)})
		ha, errA := flow.ContentHash(&a)
		hb, errB := flow.ContentHash(&b)
		if errA != nil || errB != nil {
			t.Fatal(errA, errB)
		}
		if ha != hb {
			t.Errorf("editor-only change moved the hash: %s vs %s", ha, hb)
		}
	})
	t.Run("changes with semantic content", func(t *testing.T) {
		t.Parallel()
		a := load(t, "linear")
		b := load(t, "linear")
		b.Description = "changed"
		ha, _ := flow.ContentHash(&a)
		hb, _ := flow.ContentHash(&b)
		if ha == hb {
			t.Error("a semantic change left the hash unchanged")
		}
	})
	t.Run("does not depend on member order or number spelling", func(t *testing.T) {
		t.Parallel()
		one, err := jsonvalue.Parse([]byte(`{"b":1.0,"a":{"y":[1,2],"x":true},"editor":{"q":1}}`))
		if err != nil {
			t.Fatal(err)
		}
		two, err := jsonvalue.Parse([]byte(`{"editor":{},"a":{"x":true,"y":[1,2]},"b":1}`))
		if err != nil {
			t.Fatal(err)
		}
		h1, _ := flow.ContentHashOfValue(one)
		h2, _ := flow.ContentHashOfValue(two)
		if h1 != h2 {
			t.Errorf("hashes differ for equal data: %s vs %s", h1, h2)
		}
	})
	t.Run("hex round trip", func(t *testing.T) {
		t.Parallel()
		linear := load(t, "linear")
		h, _ := flow.ContentHash(&linear)
		back, err := flow.ParseHash(h.String())
		if err != nil || back != h {
			t.Errorf("ParseHash(%s) = %v, %v", h, back, err)
		}
		for _, bad := range []string{"", "zz", strings.Repeat("a", 63), strings.Repeat("a", 66)} {
			if _, err := flow.ParseHash(bad); err == nil {
				t.Errorf("ParseHash(%q) succeeded, want error", bad)
			}
		}
	})
	t.Run("rejects a number outside the double range", func(t *testing.T) {
		t.Parallel()
		v := conformanceValue(t, "linear").With("description", mustNumber(t, "1e999"))
		if _, err := flow.ContentHashOfValue(v); err == nil {
			t.Error("ContentHashOfValue succeeded for 1e999, want error")
		}
	})
}

func mustNumber(t *testing.T, s string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Number(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestPublishCreatesLinearImmutableVersions(t *testing.T) {
	t.Parallel()
	f := newDraftFlow(t, "linear")
	if _, ok := f.Active(); ok {
		t.Fatal("a flow with nothing published has an active version")
	}
	v1, err := f.Publish()
	if err != nil {
		t.Fatalf("first Publish: %v", err)
	}
	if v1.Number != 1 {
		t.Errorf("first version number = %d, want 1", v1.Number)
	}
	linear := load(t, "linear")
	wantHash, _ := flow.ContentHash(&linear)
	if v1.ContentHash != wantHash {
		t.Errorf("version hash = %s, want %s", v1.ContentHash, wantHash)
	}

	// Publishing identical content again still creates version N+1.
	v2, err := f.Publish()
	if err != nil {
		t.Fatalf("second Publish: %v", err)
	}
	if v2.Number != 2 || v2.ContentHash != v1.ContentHash {
		t.Errorf("second publish = version %d hash %s, want version 2 with the same hash", v2.Number, v2.ContentHash)
	}

	// A different draft makes version 3 and leaves 1 and 2 untouched.
	_, rev := f.Draft()
	if _, err := f.SaveDraft(rev, withDescription(t, "edited")); err != nil {
		t.Fatal(err)
	}
	v3, err := f.Publish()
	if err != nil {
		t.Fatalf("third Publish: %v", err)
	}
	if v3.Number != 3 || v3.ContentHash == v1.ContentHash {
		t.Errorf("third publish = version %d hash %s, want version 3 with a new hash", v3.Number, v3.ContentHash)
	}
	if got, _ := f.Version(1); got.ContentHash != v1.ContentHash || got.Document().Description == "edited" {
		t.Error("version 1 changed after later publications")
	}
	if active, _ := f.Active(); active.Number != 3 {
		t.Errorf("active version = %d, want 3", active.Number)
	}
	if got := len(f.Versions()); got != 3 {
		t.Errorf("Versions() has %d entries, want 3", got)
	}
}

func TestPublishRejectsInvalidDrafts(t *testing.T) {
	t.Parallel()
	t.Run("validation errors block and create nothing", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "linear")
		node(&doc, "done").Name = "Fetch" // V12
		f := flow.NewFlow("linear")
		if _, err := f.SaveDraft(0, doc.Value()); err != nil {
			t.Fatal(err)
		}
		_, err := f.Publish()
		var ve *flow.ValidationError
		if !errors.As(err, &ve) || !ve.Report.Has(flow.CodeV12, flow.SeverityError) {
			t.Fatalf("Publish error = %v, want a ValidationError with V12", err)
		}
		if len(f.Versions()) != 0 {
			t.Error("a blocked publication created a version")
		}
	})
	t.Run("a draft of the wrong shape is rejected", func(t *testing.T) {
		t.Parallel()
		f := flow.NewFlow("x")
		if _, err := f.Publish(); err == nil { // the empty initial draft
			t.Error("Publish of the empty draft succeeded, want error")
		}
	})
	t.Run("warnings are recorded with the version", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "exclusive-and-inclusive")
		edge(&doc, "e3").Default = nil
		f := flow.NewFlow("w")
		if _, err := f.SaveDraft(0, doc.Value()); err != nil {
			t.Fatal(err)
		}
		v, err := f.Publish()
		if err != nil {
			t.Fatal(err)
		}
		if len(v.Warnings) == 0 || v.Warnings[0].Code != flow.CodeV5 {
			t.Errorf("Warnings = %v, want a V5 warning", v.Warnings)
		}
	})
}

func TestPublishRollbackCreatesNextVersion(t *testing.T) {
	t.Parallel()
	f := newDraftFlow(t, "linear")
	v1, err := f.Publish()
	if err != nil {
		t.Fatal(err)
	}
	_, rev := f.Draft()
	if _, err := f.SaveDraft(rev, withDescription(t, "second")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Publish(); err != nil {
		t.Fatal(err)
	}
	rolled, err := f.Rollback(1)
	if err != nil {
		t.Fatalf("Rollback(1): %v", err)
	}
	if rolled.Number != 3 || rolled.ContentHash != v1.ContentHash {
		t.Errorf("rollback = version %d hash %s, want version 3 with the hash of version 1", rolled.Number, rolled.ContentHash)
	}
	if len(f.Versions()) != 3 {
		t.Errorf("history has %d versions, want 3 (linear, nothing removed)", len(f.Versions()))
	}
	draft, _ := f.Draft()
	if h, _ := flow.ContentHashOfValue(draft); h != v1.ContentHash {
		t.Error("the draft does not show the rolled-back content")
	}
	if _, err := f.Rollback(9); !errors.Is(err, flow.ErrVersionNotFound) {
		t.Errorf("Rollback(9) error = %v, want ErrVersionNotFound", err)
	}
}

func TestSnapshotIsDeduplicatedByHash(t *testing.T) {
	t.Parallel()
	f := newDraftFlow(t, "linear")
	s1, err := f.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	s2, err := f.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if s1.ContentHash != s2.ContentHash || f.SnapshotCount() != 1 {
		t.Errorf("same content gave %d snapshots, want 1", f.SnapshotCount())
	}

	// An editor-only change keeps the hash, so it shares the snapshot.
	doc := load(t, "linear")
	doc.Editor = obj(jsonvalue.Member{Key: "moved", Value: jsonvalue.Bool(true)})
	_, rev := f.Draft()
	if _, err := f.SaveDraft(rev, doc.Value()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Snapshot(); err != nil || f.SnapshotCount() != 1 {
		t.Errorf("editor-only change: err=%v snapshots=%d, want 1", err, f.SnapshotCount())
	}

	// Different content makes a second snapshot; snapshots carry no number.
	_, rev = f.Draft()
	if _, err := f.SaveDraft(rev, withDescription(t, "other")); err != nil {
		t.Fatal(err)
	}
	s3, err := f.Snapshot()
	if err != nil || f.SnapshotCount() != 2 || s3.ContentHash == s1.ContentHash {
		t.Errorf("different content: err=%v snapshots=%d", err, f.SnapshotCount())
	}
	if len(f.Versions()) != 0 {
		t.Error("a test snapshot created a numbered version")
	}
}

func TestSnapshotRejectsInvalidDraft(t *testing.T) {
	t.Parallel()
	doc := load(t, "linear")
	node(&doc, "done").Name = "Fetch"
	f := flow.NewFlow("x")
	if _, err := f.SaveDraft(0, doc.Value()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Snapshot(); err == nil {
		t.Error("Snapshot of an invalid draft succeeded, want error")
	}
	if f.SnapshotCount() != 0 {
		t.Error("an invalid draft left a snapshot behind")
	}
}

func TestPublishKeepsRevisionOfRunningExecution(t *testing.T) {
	t.Parallel()
	f := newDraftFlow(t, "linear")
	v1, err := f.Publish()
	if err != nil {
		t.Fatal(err)
	}
	pinnedToVersion := flow.VersionRevision(v1.Number)
	snap, err := f.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	pinnedToSnapshot := flow.SnapshotRevision(snap.ContentHash)

	// Publish another version and change the draft; the pinned executions
	// must keep seeing the content they started with.
	_, rev := f.Draft()
	if _, err := f.SaveDraft(rev, withDescription(t, "newer")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Publish(); err != nil {
		t.Fatal(err)
	}
	for name, pin := range map[string]flow.Revision{"version": pinnedToVersion, "snapshot": pinnedToSnapshot} {
		doc, err := f.Resolve(pin)
		if err != nil {
			t.Fatalf("Resolve(%s): %v", name, err)
		}
		if doc.Description == "newer" || doc.Description != load(t, "linear").Description {
			t.Errorf("Resolve(%s) description = %q, want the original", name, doc.Description)
		}
	}
	if n, ok := pinnedToVersion.Version(); !ok || n != 1 {
		t.Errorf("Version() = %d, %v", n, ok)
	}
	if _, ok := pinnedToVersion.Snapshot(); ok {
		t.Error("a version revision reports a snapshot")
	}
	if h, ok := pinnedToSnapshot.Snapshot(); !ok || h != snap.ContentHash {
		t.Error("snapshot revision lost its hash")
	}
	if _, ok := pinnedToSnapshot.Version(); ok {
		t.Error("a snapshot revision reports a version")
	}
}

func TestResolveErrors(t *testing.T) {
	t.Parallel()
	f := newDraftFlow(t, "linear")
	if _, err := f.Resolve(flow.Revision{}); !errors.Is(err, flow.ErrInvalidRevision) {
		t.Errorf("Resolve(zero) error = %v, want ErrInvalidRevision", err)
	}
	if _, err := f.Resolve(flow.VersionRevision(4)); !errors.Is(err, flow.ErrVersionNotFound) {
		t.Errorf("Resolve(v4) error = %v, want ErrVersionNotFound", err)
	}
	if _, err := f.Resolve(flow.SnapshotRevision(flow.Hash{1})); !errors.Is(err, flow.ErrSnapshotNotFound) {
		t.Errorf("Resolve(unknown snapshot) error = %v, want ErrSnapshotNotFound", err)
	}
}

func TestSaveDraftOptimisticConcurrency(t *testing.T) {
	t.Parallel()
	f := flow.NewFlow("x")
	rev, err := f.SaveDraft(0, conformanceValue(t, "linear"))
	if err != nil || rev != 1 {
		t.Fatalf("first SaveDraft = %d, %v, want 1, nil", rev, err)
	}
	if _, err := f.SaveDraft(0, withDescription(t, "stale")); !errors.Is(err, flow.ErrDraftConflict) {
		t.Errorf("stale SaveDraft error = %v, want ErrDraftConflict", err)
	}
	draft, current := f.Draft()
	if current != 1 {
		t.Errorf("a rejected save changed the draft revision to %d", current)
	}
	if got, _ := draft.Get("description"); !jsonvalue.Equal(got, jsonvalue.String("A start event, one HTTP request and an end event.")) {
		t.Error("a rejected save replaced the draft content")
	}
	if f.ID() != "x" {
		t.Errorf("ID() = %q", f.ID())
	}
}

func TestVersionDocumentReadsBack(t *testing.T) {
	t.Parallel()
	f := newDraftFlow(t, "effects-calls-and-settings")
	v, err := f.Publish()
	if err != nil {
		t.Fatal(err)
	}
	want := load(t, "effects-calls-and-settings")
	a, _ := jsonvalue.Canonical(want.Value())
	b, _ := jsonvalue.Canonical(v.Value())
	published := v.Document()
	c, _ := jsonvalue.Canonical(published.Value())
	if diff := cmp.Diff(string(a), string(b)); diff != "" {
		t.Errorf("the stored value differs from the published document (-want +got):\n%s", diff)
	}
	if !bytes.Equal(a, c) {
		t.Error("Document() does not read back to the published content")
	}
}
