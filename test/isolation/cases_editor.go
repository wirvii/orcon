// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/flow/editor"
	"github.com/wirvii/orcon/internal/identity"
)

// The cases of the editor store methods (SPEC-014 ED2). They follow the rules of the store cases: each acts
// as organization A, with A's scope and B's identities, and expects that a row of B is not found, a list holds
// none of B's rows, a write that names B is refused and B's rows are unchanged afterwards. The retention
// methods (PurgeEditorTestData, PurgeEditorPresence) cross organizations by design: they are the engine's,
// not a request of a person, so they are in [UncoveredStoreMethods] and have no case.

// editorPlant is what organization B owns in an editor case, on top of what [Seed] stored: an execution with
// a node run, and one output, pin, test run, seed, preview and editing session that point at B's flow.
type editorPlant struct {
	execB     store.ExecutionID
	nodeExecB store.NodeExecutionID
	nodeB     string
	dataB     editor.NodeTestData
	pinB      editor.NodePin
	runB      editor.TestRun
	seedB     editor.TestRunSeed
	previewB  editor.EffectPreview
	sessionB  editor.EditingSession
}

// plantEditor writes the editor rows of organization B through the store under test.
func plantEditor(ctx context.Context, tb testing.TB, env *Env) *editorPlant {
	tb.Helper()
	s, b := env.Stores, &env.B
	must := func(what string, err error) {
		tb.Helper()
		if err != nil {
			tb.Fatalf("plant %s of organization B: %v", what, err)
		}
	}
	p := &editorPlant{nodeB: b.Sentinel("node")}
	ex := store.Execution{
		ID: store.ExecutionID(env.IDs.New()), OrgID: b.Record.ID, ProjectID: b.Project.ID, FlowID: b.Flow.ID, FlowVersionID: b.Version.ID,
		Status: "QUEUED", Mode: "TEST", RunnableCount: 1, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	must("execution", s.InTx(ctx, b.Scope, func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &ex) }))
	p.execB = ex.ID
	tok := store.Token{
		ID: store.TokenID(env.IDs.New()), OrgID: b.Record.ID, ExecutionID: ex.ID, Status: "RUNNABLE", NodeID: p.nodeB, ScopePath: "",
		ForkLineage: []byte(`[]`), FairnessKey: "flow/" + b.UUID.String(), EffectivePriority: 0,
		AvailableAt: seededAt, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	must("token", s.InTx(ctx, b.Scope, func(ctx context.Context, tx store.Tx) error { return tx.InsertToken(ctx, &tok) }))
	p.nodeExecB = store.NodeExecutionID(env.IDs.New())
	nodeExec := store.NodeExecution{
		ID: p.nodeExecB, OrgID: b.Record.ID, ExecutionID: ex.ID, TokenID: tok.ID, NodeID: p.nodeB, Status: "RUNNING",
		StartedAt: seededAt, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	must("node execution", s.InTx(ctx, b.Scope, func(ctx context.Context, tx store.Tx) error { return tx.InsertNodeExecution(ctx, nodeExec) }))

	p.dataB = editor.NodeTestData{
		ID: env.IDs.New(), OrgID: b.Record.ID, FlowID: b.Flow.ID, NodeID: p.nodeB, SourceKind: editor.SourceManual,
		OutputDocument: []byte(`{"ports":{}}`), SizeBytes: 12, ContentSHA256: make([]byte, 32), CreatedAt: seededAt,
	}
	must("output", s.InsertNodeTestData(ctx, b.Scope, p.dataB))
	p.pinB = editor.NodePin{ID: env.IDs.New(), OrgID: b.Record.ID, FlowID: b.Flow.ID, NodeID: p.nodeB, NodeTestDataID: p.dataB.ID, CreatedAt: seededAt, UpdatedAt: seededAt}
	must("pin", s.PinNode(ctx, b.Scope, p.pinB))
	p.runB = editor.TestRun{
		ID: env.IDs.New(), OrgID: b.Record.ID, FlowID: b.Flow.ID, ExecutionID: ex.ID, FlowSnapshotID: b.Snapshot.ID,
		TargetKind: editor.TargetFlow, EffectMode: editor.EffectConfirm, PlanDocument: []byte(`{}`), CreatedAt: seededAt,
	}
	must("test run", s.InsertTestRun(ctx, b.Scope, p.runB))
	p.seedB = editor.TestRunSeed{ID: env.IDs.New(), OrgID: b.Record.ID, TestRunID: p.runB.ID, NodeID: p.nodeB, SeedKind: editor.SeedPin, NodeTestDataID: p.dataB.ID, CreatedAt: seededAt}
	must("seed", s.InsertTestRunSeed(ctx, b.Scope, p.seedB))
	p.previewB = editor.EffectPreview{
		ID: env.IDs.New(), OrgID: b.Record.ID, ExecutionID: ex.ID, NodeExecutionID: p.nodeExecB, NodeID: p.nodeB,
		Status: editor.PreviewOpen, Effect: editor.EffectWrite, SafetyLevel: editor.SafetyIdempotencyKey, ItemCount: 1,
		IsPreviewAvailable: true, PreviewDocument: []byte(`{}`), PreviewSHA256: make([]byte, 32), ExpiresAt: seededAt.Add(time.Hour),
		Revision: 1, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	must("preview", s.InsertEffectPreview(ctx, b.Scope, &p.previewB))
	p.sessionB = editor.EditingSession{
		ID: env.IDs.New(), OrgID: b.Record.ID, FlowID: b.Flow.ID, ClientLabel: b.Sentinel("client"), OpenedAt: seededAt,
		LastSeenAt: seededAt, ExpiresAt: seededAt.Add(time.Hour), Revision: 1, CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	must("session", s.UpsertEditingSession(ctx, b.Scope, p.sessionB))
	return p
}

// editorCases is the editor port methods that have a case; the retention methods are not here.
var editorCases = map[string]func(ctx context.Context, tb testing.TB, env *Env){
	"InsertNodeTestData":      caseInsertNodeTestData,
	"GetNodeTestData":         caseGetNodeTestData,
	"ListNodeTestDataForNode": caseListNodeTestDataForNode,
	"PinNode":                 casePinNode,
	"GetNodePin":              caseGetNodePin,
	"ListNodePins":            caseListNodePins,
	"UnpinNode":               caseUnpinNode,
	"InsertTestRun":           caseInsertTestRun,
	"GetTestRun":              caseGetTestRun,
	"ListTestRuns":            caseListTestRuns,
	"InsertTestRunSeed":       caseInsertTestRunSeed,
	"ListTestRunSeeds":        caseListTestRunSeeds,
	"InsertEffectPreview":     caseInsertEffectPreview,
	"GetEffectPreview":        caseGetEffectPreview,
	"DecideEffectPreview":     caseDecideEffectPreview,
	"ListEffectPreviews":      caseListEffectPreviews,
	"UpsertEditingSession":    caseUpsertEditingSession,
	"ListEditingSessions":     caseListEditingSessions,
}

// addEditorCases registers the editor cases of the battery.
func addEditorCases(r *Registry) error {
	for name, run := range editorCases {
		if err := r.Add(Case{Subject: StoreMethod(name), Run: run}); err != nil {
			return err
		}
	}
	return nil
}

// expectNotFound fails unless err is "not found": the attacker sees nothing of organization B.
func expectNotFound(tb testing.TB, what string, err error) {
	tb.Helper()
	if !errors.Is(err, flow.ErrNotFound) {
		tb.Errorf("%s through another organization: want not found, got %v", what, err)
	}
}

// expectRefused fails unless the write was refused. Any refusal is acceptable: a scope mismatch, a
// reference, a conflict or a validation error.
func expectRefused(tb testing.TB, what string, err error) {
	tb.Helper()
	if err == nil {
		tb.Errorf("%s: a write of organization A that points at organization B was accepted", what)
	}
}

// expectNoRows fails when a list returns anything; the attacker must see an empty result.
func expectNoRows(tb testing.TB, what string, n int, err error) {
	tb.Helper()
	if err != nil {
		tb.Errorf("%s: %v", what, err)
	}
	if n != 0 {
		tb.Errorf("%s through another organization returned %d rows", what, n)
	}
}

func caseInsertNodeTestData(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	d := p.dataB
	d.ID = env.IDs.New()
	// The record names B, the scope is A's: refused. The record names A and B's flow: refused as well.
	expectRefused(tb, "insert node test data under another organization's record", env.Stores.InsertNodeTestData(ctx, env.A.Scope, d))
	d.OrgID = env.A.Record.ID
	expectRefused(tb, "insert node test data for another organization's flow", env.Stores.InsertNodeTestData(ctx, env.A.Scope, d))
}

func caseGetNodeTestData(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	got, err := env.Stores.GetNodeTestData(ctx, env.A.Scope, p.dataB.ID)
	expectNotFound(tb, "get node test data", err)
	noLeak(tb, "the lookup", got, err, &env.B)
}

func caseListNodeTestDataForNode(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	rows, err := env.Stores.ListNodeTestDataForNode(ctx, env.A.Scope, env.B.Flow.ID, p.nodeB, 50)
	expectNoRows(tb, "list node test data", len(rows), err)
}

func casePinNode(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	pin := editor.NodePin{ID: env.IDs.New(), OrgID: env.A.Record.ID, FlowID: env.B.Flow.ID, NodeID: p.nodeB, NodeTestDataID: p.dataB.ID, CreatedAt: seededAt, UpdatedAt: seededAt}
	expectRefused(tb, "pin node", env.Stores.PinNode(ctx, env.A.Scope, pin))
	got, err := env.Stores.GetNodePin(ctx, env.B.Scope, env.B.Flow.ID, p.nodeB)
	if err != nil || got.Revision != 1 || got.ID != p.pinB.ID {
		tb.Errorf("the pin of organization B was changed by another organization: %+v, %v", got, err)
	}
}

func caseGetNodePin(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	got, err := env.Stores.GetNodePin(ctx, env.A.Scope, env.B.Flow.ID, p.nodeB)
	expectNotFound(tb, "get node pin", err)
	noLeak(tb, "the lookup", got, err, &env.B)
}

func caseListNodePins(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	plantEditor(ctx, tb, env)
	rows, err := env.Stores.ListNodePins(ctx, env.A.Scope, env.B.Flow.ID, "", 50)
	expectNoRows(tb, "list node pins", len(rows), err)
}

func caseUnpinNode(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	expectNotFound(tb, "unpin node", env.Stores.UnpinNode(ctx, env.A.Scope, env.B.Flow.ID, p.nodeB))
	if _, err := env.Stores.GetNodePin(ctx, env.B.Scope, env.B.Flow.ID, p.nodeB); err != nil {
		tb.Errorf("the pin of organization B was removed by another organization: %v", err)
	}
}

func caseInsertTestRun(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	run := p.runB
	run.ID = env.IDs.New()
	expectRefused(tb, "insert test run under another organization's record", env.Stores.InsertTestRun(ctx, env.A.Scope, run))
	run.OrgID = env.A.Record.ID
	expectRefused(tb, "insert test run for another organization's flow", env.Stores.InsertTestRun(ctx, env.A.Scope, run))
}

func caseGetTestRun(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	got, err := env.Stores.GetTestRun(ctx, env.A.Scope, p.runB.ID)
	expectNotFound(tb, "get test run", err)
	noLeak(tb, "the lookup", got, err, &env.B)
}

func caseListTestRuns(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	plantEditor(ctx, tb, env)
	rows, err := env.Stores.ListTestRuns(ctx, env.A.Scope, env.B.Flow.ID, 50)
	expectNoRows(tb, "list test runs", len(rows), err)
}

func caseInsertTestRunSeed(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	seed := p.seedB
	seed.ID = env.IDs.New()
	expectRefused(tb, "insert test run seed under another organization's record", env.Stores.InsertTestRunSeed(ctx, env.A.Scope, seed))
	seed.OrgID = env.A.Record.ID
	expectRefused(tb, "insert test run seed for another organization's run", env.Stores.InsertTestRunSeed(ctx, env.A.Scope, seed))
}

func caseListTestRunSeeds(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	rows, err := env.Stores.ListTestRunSeeds(ctx, env.A.Scope, p.runB.ID, 50)
	expectNoRows(tb, "list test run seeds", len(rows), err)
}

func caseInsertEffectPreview(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	pv := p.previewB
	pv.ID = env.IDs.New()
	expectRefused(tb, "insert effect preview under another organization's record", env.Stores.InsertEffectPreview(ctx, env.A.Scope, &pv))
	pv.OrgID = env.A.Record.ID
	expectRefused(tb, "insert effect preview for another organization's execution", env.Stores.InsertEffectPreview(ctx, env.A.Scope, &pv))
}

func caseGetEffectPreview(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	got, err := env.Stores.GetEffectPreview(ctx, env.A.Scope, p.previewB.ID)
	expectNotFound(tb, "get effect preview", err)
	noLeak(tb, "the lookup", got, err, &env.B)
}

func caseDecideEffectPreview(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	if err := env.Stores.DecideEffectPreview(ctx, env.A.Scope, p.previewB.ID, 1, editor.PreviewDispatchConfirmed, []byte(`{}`), identity.PrincipalID{}, seededAt); err == nil {
		tb.Error("a decision of organization A on a preview of organization B was accepted")
	}
	got, err := env.Stores.GetEffectPreview(ctx, env.B.Scope, p.previewB.ID)
	if err != nil || got.Status != editor.PreviewOpen || got.Revision != 1 {
		tb.Errorf("the preview of organization B was decided by another organization: %+v, %v", got, err)
	}
}

func caseListEffectPreviews(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	rows, err := env.Stores.ListEffectPreviews(ctx, env.A.Scope, p.execB, 50)
	expectNoRows(tb, "list effect previews", len(rows), err)
}

func caseUpsertEditingSession(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	p := plantEditor(ctx, tb, env)
	sess := p.sessionB
	sess.ID = env.IDs.New()
	expectRefused(tb, "open editing session under another organization's record", env.Stores.UpsertEditingSession(ctx, env.A.Scope, sess))
	sess.OrgID = env.A.Record.ID
	expectRefused(tb, "open editing session on another organization's flow", env.Stores.UpsertEditingSession(ctx, env.A.Scope, sess))
	renewed := p.sessionB
	renewed.OrgID = env.A.Record.ID
	expectRefused(tb, "renew another organization's editing session", env.Stores.UpsertEditingSession(ctx, env.A.Scope, renewed))
	rows, err := env.Stores.ListEditingSessions(ctx, env.B.Scope, env.B.Flow.ID, seededAt, 50)
	if err != nil || len(rows) != 1 || rows[0].Revision != 1 || rows[0].ClientLabel != p.sessionB.ClientLabel {
		tb.Errorf("the editing session of organization B was changed by another organization: %+v, %v", rows, err)
	}
}

func caseListEditingSessions(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	plantEditor(ctx, tb, env)
	rows, err := env.Stores.ListEditingSessions(ctx, env.A.Scope, env.B.Flow.ID, seededAt, 50)
	expectNoRows(tb, "list editing sessions", len(rows), err)
	noLeak(tb, "the list", rows, err, &env.B)
}
