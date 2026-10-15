// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/flow/editor"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/trigger"
)

// EditorStore is every port the storage adapters implement for the visual flow
// editor, on top of the shared [Store].
type EditorStore interface {
	Store
	editor.Store
}

// EditorFactory returns a store on a fresh, migrated database and registers its
// cleanup with t.
type EditorFactory func(t *testing.T) EditorStore

// editorAtomic is the transaction that purges a flow for good; both adapters
// implement it (the trigger store owns the flow lifecycle).
type editorAtomic interface {
	Atomic(ctx context.Context, org identity.OrgID, fn func(ctx context.Context, tx trigger.Tx) error) error
}

// RunEditorSuite runs the editor storage battery (SPEC-014 section 6, ADR-0082): pins, append-only node
// outputs and test runs, one preview per node execution, presence expiry, retention of unreferenced editor
// data and the cascade of a flow purge. Every case also reads through the other organization.
func RunEditorSuite(t *testing.T, newStore EditorFactory) {
	t.Helper()
	cases := []struct {
		name string
		fn   func(t *testing.T, f *editorFixture)
	}{
		{"pins", editorPinsCase},
		{"node test data", editorTestDataCase},
		{"test runs and seeds", editorTestRunCase},
		{"effect previews", editorPreviewCase},
		{"editing sessions", editorSessionsCase},
		{"retention and flow purge", editorRetentionCase},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Helper()
			c.fn(t, newEditorFixture(t, newStore))
		})
	}
}

// editorFixture is an execution fixture (one flow version and an execution in org A) plus the editor store.
type editorFixture struct {
	*execFixture
	ed EditorStore
}

func newEditorFixture(t *testing.T, newStore EditorFactory) *editorFixture {
	t.Helper()
	var ed EditorStore
	ef := newExecFixture(t, func(t *testing.T) Store {
		ed = newStore(t)
		return ed
	})
	return &editorFixture{execFixture: ef, ed: ed}
}

// output inserts one node output of org A's flow, created at at.
func (f *editorFixture) output(node string, at time.Time) editor.NodeTestData {
	f.t.Helper()
	d := editor.NodeTestData{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, FlowID: f.flow.ID, NodeID: node, SourceKind: editor.SourceManual,
		OutputDocument: []byte(`{"ports":{}}`), ItemCount: 0, SizeBytes: 12, ContentSHA256: sha32(0xab),
		CreatedAt: at.Truncate(time.Microsecond),
	}
	mustTx(f.t, f.ed.InsertNodeTestData(f.ctx(), d), "insert node test data")
	return d
}

func sha32(b byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = b
	}
	return out
}

// pinOf builds the pin of a node to one output, owned by org A.
func (f *editorFixture) pinOf(node string, data editor.NodeTestData, at time.Time) editor.NodePin {
	return editor.NodePin{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, FlowID: f.flow.ID, NodeID: node, NodeTestDataID: data.ID,
		CreatedAt: at.Truncate(time.Microsecond), UpdatedAt: at.Truncate(time.Microsecond),
	}
}

func editorPinsCase(t *testing.T, f *editorFixture) {
	t.Helper()
	ctx := f.ctx()
	first := f.output("n1", base)
	mustTx(t, f.ed.PinNode(ctx, f.pinOf("n1", first, base)), "pin node")
	got, err := f.ed.GetNodePin(ctx, f.orgA, f.flow.ID, "n1")
	if err != nil || got.Revision != 1 || got.NodeTestDataID != first.ID {
		t.Fatalf("first pin: want revision 1 on the first output, got %+v, %v", got, err)
	}

	second := f.output("n1", base.Add(time.Minute))
	mustTx(t, f.ed.PinNode(ctx, f.pinOf("n1", second, base.Add(time.Minute))), "repin node")
	got, err = f.ed.GetNodePin(ctx, f.orgA, f.flow.ID, "n1")
	if err != nil || got.Revision != 2 || got.NodeTestDataID != second.ID {
		t.Fatalf("repin: want one pin per node at revision 2 on the second output, got %+v, %v", got, err)
	}

	pins, err := f.ed.ListNodePins(ctx, f.orgA, f.flow.ID, "", 10)
	if err != nil || len(pins) != 1 {
		t.Fatalf("list pins: want one pin for the one node, got %d, %v", len(pins), err)
	}
	if _, err := f.ed.GetNodePin(ctx, f.orgB, f.flow.ID, "n1"); !errors.Is(err, flow.ErrNotFound) {
		t.Fatalf("pin read through another organization: want ErrNotFound, got %v", err)
	}
	if pins, err := f.ed.ListNodePins(ctx, f.orgB, f.flow.ID, "", 10); err != nil || len(pins) != 0 {
		t.Fatalf("pin list through another organization: want none, got %d, %v", len(pins), err)
	}

	mustTx(t, f.ed.UnpinNode(ctx, f.orgA, f.flow.ID, "n1"), "unpin node")
	if _, err := f.ed.GetNodePin(ctx, f.orgA, f.flow.ID, "n1"); !errors.Is(err, flow.ErrNotFound) {
		t.Fatalf("after unpin: want ErrNotFound, got %v", err)
	}
	if err := f.ed.UnpinNode(ctx, f.orgA, f.flow.ID, "n1"); !errors.Is(err, flow.ErrNotFound) {
		t.Fatalf("second unpin: want ErrNotFound, got %v", err)
	}
}

func editorTestDataCase(t *testing.T, f *editorFixture) {
	t.Helper()
	ctx := f.ctx()
	older := f.output("n1", base)
	newer := f.output("n1", base.Add(time.Second))
	other := f.output("n2", base.Add(2*time.Second))

	latest, err := f.ed.ListNodeTestDataForNode(ctx, f.orgA, f.flow.ID, "n1", 1)
	if err != nil || len(latest) != 1 || latest[0].ID != newer.ID {
		t.Fatalf("latest output of n1 with limit 1: want the newest, got %d rows, %v", len(latest), err)
	}
	all, err := f.ed.ListNodeTestDataForNode(ctx, f.orgA, f.flow.ID, "n1", 10)
	if err != nil || len(all) != 2 || all[0].ID != newer.ID || all[1].ID != older.ID {
		t.Fatalf("outputs of n1: want newest first, got %d rows, %v", len(all), err)
	}
	if got, err := f.ed.GetNodeTestData(ctx, f.orgA, other.ID); err != nil || got.NodeID != "n2" || !got.CreatedAt.Equal(other.CreatedAt) {
		t.Fatalf("get output: got %+v, %v", got, err)
	}
	if _, err := f.ed.GetNodeTestData(ctx, f.orgB, other.ID); !errors.Is(err, flow.ErrNotFound) {
		t.Fatalf("get output through another organization: want ErrNotFound, got %v", err)
	}
	if rows, err := f.ed.ListNodeTestDataForNode(ctx, f.orgB, f.flow.ID, "n1", 10); err != nil || len(rows) != 0 {
		t.Fatalf("list outputs through another organization: want none, got %d, %v", len(rows), err)
	}

	// A write names its own organization: an output of org B cannot point at a flow of org A.
	bad := editor.NodeTestData{
		ID: next[id.UUID](f.ids), OrgID: f.orgB, FlowID: f.flow.ID, NodeID: "n1", SourceKind: editor.SourceManual,
		OutputDocument: []byte(`{}`), ContentSHA256: sha32(0xcd), CreatedAt: base,
	}
	if err := f.ed.InsertNodeTestData(ctx, bad); err == nil {
		t.Fatal("output of org B for a flow of org A: want a reference error, got none")
	}
}

// newRun inserts a test run of the fixture's execution against a fresh snapshot.
func (f *editorFixture) newRun(exec *store.Execution) editor.TestRun {
	f.t.Helper()
	snap, err := f.s.CreateSnapshot(f.ctx(), flow.SnapshotRecord{
		ID: next[flow.FlowSnapshotID](f.ids), OrgID: f.orgA, FlowID: f.flow.ID,
		ContentHash: "hash-" + f.ids.New().String(), Document: []byte(`{}`), CreatedAt: base,
	})
	mustTx(f.t, err, "create snapshot")
	run := editor.TestRun{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, FlowID: f.flow.ID, ExecutionID: exec.ID, FlowSnapshotID: snap.ID,
		TargetKind: editor.TargetFlow, EffectMode: editor.EffectConfirm, PlanDocument: []byte(`{}`), CreatedAt: base,
	}
	mustTx(f.t, f.ed.InsertTestRun(f.ctx(), run), "insert test run")
	return run
}

func editorTestRunCase(t *testing.T, f *editorFixture) {
	t.Helper()
	ctx := f.ctx()
	exec := f.execution()
	run := f.newRun(exec)

	got, err := f.ed.GetTestRun(ctx, f.orgA, run.ID)
	if err != nil || got.ExecutionID != exec.ID || got.TargetKind != editor.TargetFlow {
		t.Fatalf("get test run: got %+v, %v", got, err)
	}
	if _, err := f.ed.GetTestRun(ctx, f.orgB, run.ID); !errors.Is(err, flow.ErrNotFound) {
		t.Fatalf("get test run through another organization: want ErrNotFound, got %v", err)
	}
	history, err := f.ed.ListTestRuns(ctx, f.orgA, f.flow.ID, 50)
	if err != nil || len(history) != 1 {
		t.Fatalf("test history: want one run, got %d, %v", len(history), err)
	}

	// One test run per TEST execution.
	dup := run
	dup.ID = next[id.UUID](f.ids)
	if err := f.ed.InsertTestRun(ctx, dup); !errors.Is(err, flow.ErrConflict) {
		t.Fatalf("second run of one execution: want ErrConflict, got %v", err)
	}

	data := f.output("n1", base)
	seed := editor.TestRunSeed{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, TestRunID: run.ID, NodeID: "n1", SeedKind: editor.SeedPin,
		NodeTestDataID: data.ID, CreatedAt: base,
	}
	mustTx(t, f.ed.InsertTestRunSeed(ctx, seed), "insert seed")
	seeds, err := f.ed.ListTestRunSeeds(ctx, f.orgA, run.ID, 100)
	if err != nil || len(seeds) != 1 || seeds[0].NodeTestDataID != data.ID {
		t.Fatalf("seeds of run: want the pin, got %+v, %v", seeds, err)
	}
	if seeds, err := f.ed.ListTestRunSeeds(ctx, f.orgB, run.ID, 100); err != nil || len(seeds) != 0 {
		t.Fatalf("seeds through another organization: want none, got %d, %v", len(seeds), err)
	}
}

// newNodeExecution inserts a node execution of the execution through a token, and returns its identifier.
func (f *editorFixture) newNodeExecution(exec *store.Execution, node string) store.NodeExecutionID {
	f.t.Helper()
	tok := f.token(exec, node, 0, base)
	nodeExec := store.NodeExecution{
		ID: next[store.NodeExecutionID](f.ids), OrgID: exec.OrgID, ExecutionID: exec.ID, TokenID: tok.ID,
		NodeID: node, Status: "RUNNING", StartedAt: base, CreatedAt: base, UpdatedAt: base,
	}
	mustTx(f.t, f.tx(exec.OrgID, func(ctx context.Context, tx store.Tx) error { return tx.InsertNodeExecution(ctx, nodeExec) }), "insert node execution")
	return nodeExec.ID
}

func editorPreviewCase(t *testing.T, f *editorFixture) {
	t.Helper()
	ctx := f.ctx()
	exec := f.execution()
	nodeExec := f.newNodeExecution(exec, "send")
	preview := editor.EffectPreview{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, ExecutionID: exec.ID, NodeExecutionID: nodeExec, NodeID: "send",
		Status: editor.PreviewOpen, Effect: editor.EffectWrite, SafetyLevel: editor.SafetyIdempotencyKey, ItemCount: 1,
		IsPreviewAvailable: true, PreviewDocument: []byte(`{}`), PreviewSHA256: sha32(0xef),
		ExpiresAt: base.Add(time.Hour), Revision: 1, CreatedAt: base, UpdatedAt: base,
	}
	mustTx(t, f.ed.InsertEffectPreview(ctx, &preview), "insert preview")

	// One preview per gated node activation.
	again := preview
	again.ID = next[id.UUID](f.ids)
	if err := f.ed.InsertEffectPreview(ctx, &again); !errors.Is(err, flow.ErrConflict) {
		t.Fatalf("second preview of one node execution: want ErrConflict, got %v", err)
	}

	if err := f.ed.DecideEffectPreview(ctx, f.orgB, preview.ID, 1, editor.PreviewDispatchConfirmed, []byte(`{}`), identity.PrincipalID{}, base); !errors.Is(err, flow.ErrConflict) {
		t.Fatalf("decision through another organization: want no change, got %v", err)
	}
	mustTx(t, f.ed.DecideEffectPreview(ctx, f.orgA, preview.ID, 1, editor.PreviewDispatchConfirmed, []byte(`{}`), identity.PrincipalID{}, base.Add(time.Minute)), "decide preview")
	got, err := f.ed.GetEffectPreview(ctx, f.orgA, preview.ID)
	if err != nil || got.Status != editor.PreviewDispatchConfirmed || got.Revision != 2 || got.DecidedAt.IsZero() {
		t.Fatalf("decided preview: got %+v, %v", got, err)
	}
	// A decision made on a revision the person no longer saw is refused.
	if err := f.ed.DecideEffectPreview(ctx, f.orgA, preview.ID, 1, editor.PreviewCanceled, []byte(`{}`), identity.PrincipalID{}, base); !errors.Is(err, flow.ErrConflict) {
		t.Fatalf("stale decision: want ErrConflict, got %v", err)
	}
	list, err := f.ed.ListEffectPreviews(ctx, f.orgA, exec.ID, 10)
	if err != nil || len(list) != 1 {
		t.Fatalf("previews of the execution: want one, got %d, %v", len(list), err)
	}
}

func editorSessionsCase(t *testing.T, f *editorFixture) {
	t.Helper()
	ctx := f.ctx()
	session := editor.EditingSession{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, FlowID: f.flow.ID, ClientLabel: "Firefox on macOS",
		LastSeenDraftRevision: 0, OpenedAt: base, LastSeenAt: base, ExpiresAt: base.Add(10 * time.Minute),
		CreatedAt: base, UpdatedAt: base,
	}
	mustTx(t, f.ed.UpsertEditingSession(ctx, session), "open session")
	renewed := session
	renewed.LastSeenAt = base.Add(5 * time.Minute)
	renewed.ExpiresAt = base.Add(20 * time.Minute)
	renewed.UpdatedAt = base.Add(5 * time.Minute)
	mustTx(t, f.ed.UpsertEditingSession(ctx, renewed), "renew session")

	live, err := f.ed.ListEditingSessions(ctx, f.orgA, f.flow.ID, base.Add(6*time.Minute), 10)
	if err != nil || len(live) != 1 || !live[0].ExpiresAt.Equal(renewed.ExpiresAt.Truncate(time.Microsecond)) || live[0].Revision != 2 {
		t.Fatalf("renewed session: want one live session at revision 2, got %+v, %v", live, err)
	}
	if rows, err := f.ed.ListEditingSessions(ctx, f.orgB, f.flow.ID, base, 10); err != nil || len(rows) != 0 {
		t.Fatalf("sessions through another organization: want none, got %d, %v", len(rows), err)
	}

	// Expiry removes the session once its expires_at is before the purge instant; a live one stays.
	if n, err := f.ed.PurgeEditorPresence(ctx, base.Add(15*time.Minute), 100); err != nil || n != 0 {
		t.Fatalf("purge before expiry: want 0 removed, got %d, %v", n, err)
	}
	if n, err := f.ed.PurgeEditorPresence(ctx, base.Add(30*time.Minute), 100); err != nil || n != 1 {
		t.Fatalf("purge after expiry: want 1 removed, got %d, %v", n, err)
	}
	if n, err := f.ed.PurgeEditorPresence(ctx, base.Add(30*time.Minute), 100); err != nil || n != 0 {
		t.Fatalf("second purge: want 0 removed, got %d, %v", n, err)
	}
}

func editorRetentionCase(t *testing.T, f *editorFixture) {
	t.Helper()
	ctx := f.ctx()
	old := base.Add(-25 * time.Hour)
	unreferenced := f.output("n1", old)
	pinned := f.output("n2", old)
	fresh := f.output("n3", base.Add(-time.Hour))
	mustTx(t, f.ed.PinNode(ctx, f.pinOf("n2", pinned, old)), "pin n2")
	mustTx(t, f.ed.UpsertEditingSession(ctx, editor.EditingSession{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, FlowID: f.flow.ID, ClientLabel: "Chrome",
		OpenedAt: base, LastSeenAt: base, ExpiresAt: base.Add(time.Minute), CreatedAt: base, UpdatedAt: base,
	}), "open session")

	// Only the output that no pin and no seed refers to, and that is older than 24 hours, is removed.
	if n, err := f.ed.PurgeEditorTestData(ctx, base, 100); err != nil || n != 1 {
		t.Fatalf("purge of editor test data: want 1 removed, got %d, %v", n, err)
	}
	if _, err := f.ed.GetNodeTestData(ctx, f.orgA, unreferenced.ID); !errors.Is(err, flow.ErrNotFound) {
		t.Fatalf("unreferenced output after purge: want ErrNotFound, got %v", err)
	}
	kept := []editor.NodeTestData{pinned, fresh}
	for i := range kept {
		if _, err := f.ed.GetNodeTestData(ctx, f.orgA, kept[i].ID); err != nil {
			t.Fatalf("output %s must survive the purge: %v", kept[i].NodeID, err)
		}
	}

	// A flow purge cascades to its pins, outputs and editing sessions (the flow row is the parent).
	atomic, ok := f.ed.(editorAtomic)
	if !ok {
		t.Fatal("the store does not run the flow purge transaction")
	}
	flowID := f.flow.ID
	err := atomic.Atomic(ctx, f.orgA, func(ctx context.Context, tx trigger.Tx) error {
		if err := tx.SoftDeleteFlow(ctx, flowID, identity.PrincipalID{}, base); err != nil {
			return err
		}
		return tx.PurgeFlow(ctx, flowID)
	})
	mustTx(t, err, "purge flow")
	if _, err := f.ed.GetNodePin(ctx, f.orgA, flowID, "n2"); !errors.Is(err, flow.ErrNotFound) {
		t.Fatalf("pin after flow purge: want ErrNotFound, got %v", err)
	}
	if rows, err := f.ed.ListNodeTestDataForNode(ctx, f.orgA, flowID, "n2", 10); err != nil || len(rows) != 0 {
		t.Fatalf("outputs after flow purge: want none, got %d, %v", len(rows), err)
	}
	if rows, err := f.ed.ListEditingSessions(ctx, f.orgA, flowID, base, 10); err != nil || len(rows) != 0 {
		t.Fatalf("sessions after flow purge: want none, got %d, %v", len(rows), err)
	}
}
