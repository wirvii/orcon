// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/flow/editor"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// EditorBackend is what the editor cases need from a store: the editor port, the flow and
// execution rows its tables point at, and the transaction that writes execution rows.
type EditorBackend interface {
	editor.Store
	flow.InstanceStore
	flow.ProjectStore
	flow.Store
	flow.VersionStore
	flow.SnapshotStore
	store.Transactor
}

// EditorOpener returns a fresh, migrated backend for one case.
type EditorOpener func(tb testing.TB) EditorBackend

// EditorSurface lists the organization-scoped store methods of the editor port. The retention
// methods (PurgeEditorTestData, PurgeEditorPresence) cross organizations by design: they are the
// engine's, not a request of a person, so they have no organization case.
type EditorSurface struct{}

// Routes returns nothing: the editor has no HTTP route of its own in this cut.
func (EditorSurface) Routes() []string { return nil }

// StoreMethods returns the organization-scoped methods of the editor port.
func (EditorSurface) StoreMethods() []string {
	return []string{
		"EditorStore.InsertNodeTestData",
		"EditorStore.GetNodeTestData",
		"EditorStore.ListNodeTestDataForNode",
		"EditorStore.PinNode",
		"EditorStore.GetNodePin",
		"EditorStore.ListNodePins",
		"EditorStore.UnpinNode",
		"EditorStore.InsertTestRun",
		"EditorStore.GetTestRun",
		"EditorStore.ListTestRuns",
		"EditorStore.InsertTestRunSeed",
		"EditorStore.ListTestRunSeeds",
		"EditorStore.InsertEffectPreview",
		"EditorStore.GetEffectPreview",
		"EditorStore.DecideEffectPreview",
		"EditorStore.ListEffectPreviews",
		"EditorStore.UpsertEditingSession",
		"EditorStore.ListEditingSessions",
	}
}

// editorPlant is what organization B owns in a case: one flow with an output, a pin, a test
// run with a seed, a preview and an editing session. Organization A is the attacker.
type editorPlant struct {
	ctx       context.Context
	tb        testing.TB
	b         EditorBackend
	env       *Env
	ids       *id.Sequence
	orgA      identity.OrgID
	orgB      identity.OrgID
	flowA     flow.FlowID
	flowB     flow.FlowID
	version   flow.FlowVersionID
	snapshot  flow.FlowSnapshotID
	execB     store.ExecutionID
	nodeExecB store.NodeExecutionID
	dataB     editor.NodeTestData
	pinB      editor.NodePin
	runB      editor.TestRun
	seedB     editor.TestRunSeed
	previewB  editor.EffectPreview
	sessionB  editor.EditingSession
	nodeB     string
}

var editorBase = time.Date(2026, 10, 13, 9, 30, 15, 0, time.UTC)

func parseOrg(tb testing.TB, s string) identity.OrgID {
	tb.Helper()
	u, err := id.Parse(s)
	if err != nil {
		tb.Fatalf("organization identifier %q: %v", s, err)
	}
	return identity.OrgID(u)
}

// plantEditor creates both organizations, their projects and flows, and what B owns.
func plantEditor(ctx context.Context, tb testing.TB, b EditorBackend, env *Env) *editorPlant {
	tb.Helper()
	p := &editorPlant{ctx: ctx, tb: tb, b: b, env: env, ids: id.NewSequence(), nodeB: env.B.Sentinel("node")}
	p.orgA, p.orgB = parseOrg(tb, env.A.ID), parseOrg(tb, env.B.ID)
	for _, o := range []struct {
		org  identity.OrgID
		name Org
	}{{p.orgA, env.A}, {p.orgB, env.B}} {
		must(tb, b.CreateOrganization(ctx, flow.OrgRecord{ID: o.org, Name: o.name.Name, Slug: o.name.slug, Status: flow.OrgActive, CreatedAt: editorBase, UpdatedAt: editorBase}), "create organization")
	}
	projA, projB := identity.ProjectID(p.ids.New()), identity.ProjectID(p.ids.New())
	must(tb, b.CreateProject(ctx, flow.ProjectRecord{ID: projA, OrgID: p.orgA, Name: "Project a", Slug: "alpha", Environment: "development", IsProtected: true, CreatedAt: editorBase, UpdatedAt: editorBase}), "create project")
	must(tb, b.CreateProject(ctx, flow.ProjectRecord{ID: projB, OrgID: p.orgB, Name: "Project b", Slug: "alpha", Environment: "development", IsProtected: true, CreatedAt: editorBase, UpdatedAt: editorBase}), "create project")
	p.flowA, p.flowB = flow.FlowID(p.ids.New()), flow.FlowID(p.ids.New())
	must(tb, b.CreateFlow(ctx, flow.Record{ID: p.flowA, OrgID: p.orgA, ProjectID: projA, Name: "flow-a", Draft: []byte(`{"format":"orcon/flow@1"}`), CreatedAt: editorBase, UpdatedAt: editorBase}), "create flow")
	must(tb, b.CreateFlow(ctx, flow.Record{ID: p.flowB, OrgID: p.orgB, ProjectID: projB, Name: "flow-b", Draft: []byte(`{"format":"orcon/flow@1"}`), CreatedAt: editorBase, UpdatedAt: editorBase}), "create flow")

	p.version = flow.FlowVersionID(p.ids.New())
	must(tb, b.PublishVersion(ctx, flow.VersionRecord{ID: p.version, OrgID: p.orgB, FlowID: p.flowB, Number: 1, Document: []byte(`{"version":1}`), ContentHash: fmt.Sprintf("%064d", 1), PublishedAt: editorBase, CreatedAt: editorBase}), "publish version")
	snap, err := b.CreateSnapshot(ctx, flow.SnapshotRecord{ID: flow.FlowSnapshotID(p.ids.New()), OrgID: p.orgB, FlowID: p.flowB, ContentHash: "snapshot-b", Document: []byte(`{}`), CreatedAt: editorBase})
	must(tb, err, "create snapshot")
	p.snapshot = snap.ID

	ex := store.Execution{
		ID: store.ExecutionID(p.ids.New()), OrgID: p.orgB, ProjectID: projB, FlowID: p.flowB, FlowVersionID: p.version,
		Status: "QUEUED", Mode: "TEST", RunnableCount: 1, CreatedAt: editorBase, UpdatedAt: editorBase,
	}
	must(tb, b.InTx(ctx, p.orgB, func(ctx context.Context, tx store.Tx) error { return tx.InsertExecution(ctx, &ex) }), "insert execution")
	p.execB = ex.ID
	tok := store.Token{
		ID: store.TokenID(p.ids.New()), OrgID: p.orgB, ExecutionID: ex.ID, Status: "RUNNABLE", NodeID: p.nodeB, ScopePath: "",
		ForkLineage: []byte(`[]`), FairnessKey: "flow/" + id.UUID(p.flowB).String(), EffectivePriority: 0,
		AvailableAt: editorBase, CreatedAt: editorBase, UpdatedAt: editorBase,
	}
	must(tb, b.InTx(ctx, p.orgB, func(ctx context.Context, tx store.Tx) error { return tx.InsertToken(ctx, &tok) }), "insert token")
	p.nodeExecB = store.NodeExecutionID(p.ids.New())
	nodeExec := store.NodeExecution{
		ID: p.nodeExecB, OrgID: p.orgB, ExecutionID: ex.ID, TokenID: tok.ID, NodeID: p.nodeB, Status: "RUNNING",
		StartedAt: editorBase, CreatedAt: editorBase, UpdatedAt: editorBase,
	}
	must(tb, b.InTx(ctx, p.orgB, func(ctx context.Context, tx store.Tx) error { return tx.InsertNodeExecution(ctx, nodeExec) }), "insert node execution")

	p.dataB = editor.NodeTestData{
		ID: id.UUID(p.ids.New()), OrgID: p.orgB, FlowID: p.flowB, NodeID: p.nodeB, SourceKind: editor.SourceManual,
		OutputDocument: []byte(`{"ports":{}}`), SizeBytes: 12, ContentSHA256: make([]byte, 32), CreatedAt: editorBase,
	}
	must(tb, b.InsertNodeTestData(ctx, p.dataB), "insert output")
	p.pinB = editor.NodePin{ID: id.UUID(p.ids.New()), OrgID: p.orgB, FlowID: p.flowB, NodeID: p.nodeB, NodeTestDataID: p.dataB.ID, CreatedAt: editorBase, UpdatedAt: editorBase}
	must(tb, b.PinNode(ctx, p.pinB), "pin node")
	p.runB = editor.TestRun{
		ID: id.UUID(p.ids.New()), OrgID: p.orgB, FlowID: p.flowB, ExecutionID: ex.ID, FlowSnapshotID: p.snapshot,
		TargetKind: editor.TargetFlow, EffectMode: editor.EffectConfirm, PlanDocument: []byte(`{}`), CreatedAt: editorBase,
	}
	must(tb, b.InsertTestRun(ctx, p.runB), "insert test run")
	p.seedB = editor.TestRunSeed{ID: id.UUID(p.ids.New()), OrgID: p.orgB, TestRunID: p.runB.ID, NodeID: p.nodeB, SeedKind: editor.SeedPin, NodeTestDataID: p.dataB.ID, CreatedAt: editorBase}
	must(tb, b.InsertTestRunSeed(ctx, p.seedB), "insert seed")
	p.previewB = editor.EffectPreview{
		ID: id.UUID(p.ids.New()), OrgID: p.orgB, ExecutionID: ex.ID, NodeExecutionID: p.nodeExecB, NodeID: p.nodeB,
		Status: editor.PreviewOpen, Effect: editor.EffectWrite, SafetyLevel: editor.SafetyIdempotencyKey, ItemCount: 1,
		IsPreviewAvailable: true, PreviewDocument: []byte(`{}`), PreviewSHA256: make([]byte, 32), ExpiresAt: editorBase.Add(time.Hour),
		Revision: 1, CreatedAt: editorBase, UpdatedAt: editorBase,
	}
	must(tb, b.InsertEffectPreview(ctx, p.previewB), "insert preview")
	p.sessionB = editor.EditingSession{
		ID: id.UUID(p.ids.New()), OrgID: p.orgB, FlowID: p.flowB, ClientLabel: "Firefox on macOS", OpenedAt: editorBase,
		LastSeenAt: editorBase, ExpiresAt: editorBase.Add(time.Hour), Revision: 1, CreatedAt: editorBase, UpdatedAt: editorBase,
	}
	must(tb, b.UpsertEditingSession(ctx, p.sessionB), "open session")
	return p
}

func must(tb testing.TB, err error, what string) {
	tb.Helper()
	if err != nil {
		tb.Fatalf("%s: %v", what, err)
	}
}

// open returns the backend of the case and the plant of organization B in it.
func openEditor(ctx context.Context, tb testing.TB, env *Env) *editorPlant {
	tb.Helper()
	if env.Editor == nil {
		tb.Fatal("isolation: the run has no editor store opener (Env.Editor)")
	}
	return plantEditor(ctx, tb, env.Editor(tb), env)
}

// expectNotFound fails unless err is "not found": the attacker sees nothing of organization B.
func expectNotFound(tb testing.TB, what string, err error) {
	tb.Helper()
	if !errors.Is(err, flow.ErrNotFound) {
		tb.Errorf("%s through another organization: want not found, got %v", what, err)
	}
}

// expectRefused fails unless the write was refused. Any refusal is acceptable: a reference, a
// conflict or a validation error.
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

// editorCases is every case of the editor surface, in the order of [EditorSurface.StoreMethods].
func editorCases() []Case {
	return []Case{
		{Subject: StoreMethod("EditorStore.InsertNodeTestData"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			d := p.dataB
			d.ID, d.OrgID = id.UUID(p.ids.New()), p.orgA // the attacker names its own organization and B's flow
			expectRefused(tb, "insert node test data", p.b.InsertNodeTestData(ctx, d))
		}},
		{Subject: StoreMethod("EditorStore.GetNodeTestData"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			_, err := p.b.GetNodeTestData(ctx, p.orgA, p.dataB.ID)
			expectNotFound(tb, "get node test data", err)
		}},
		{Subject: StoreMethod("EditorStore.ListNodeTestDataForNode"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			rows, err := p.b.ListNodeTestDataForNode(ctx, p.orgA, p.flowB, p.nodeB, 50)
			expectNoRows(tb, "list node test data", len(rows), err)
		}},
		{Subject: StoreMethod("EditorStore.PinNode"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			pin := editor.NodePin{ID: id.UUID(p.ids.New()), OrgID: p.orgA, FlowID: p.flowB, NodeID: p.nodeB, NodeTestDataID: p.dataB.ID, CreatedAt: editorBase, UpdatedAt: editorBase}
			expectRefused(tb, "pin node", p.b.PinNode(ctx, pin))
		}},
		{Subject: StoreMethod("EditorStore.GetNodePin"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			_, err := p.b.GetNodePin(ctx, p.orgA, p.flowB, p.nodeB)
			expectNotFound(tb, "get node pin", err)
		}},
		{Subject: StoreMethod("EditorStore.ListNodePins"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			rows, err := p.b.ListNodePins(ctx, p.orgA, p.flowB, "", 50)
			expectNoRows(tb, "list node pins", len(rows), err)
		}},
		{Subject: StoreMethod("EditorStore.UnpinNode"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			expectNotFound(tb, "unpin node", p.b.UnpinNode(ctx, p.orgA, p.flowB, p.nodeB))
			if _, err := p.b.GetNodePin(ctx, p.orgB, p.flowB, p.nodeB); err != nil {
				tb.Errorf("the pin of organization B was changed by another organization: %v", err)
			}
		}},
		{Subject: StoreMethod("EditorStore.InsertTestRun"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			run := p.runB
			run.ID, run.OrgID = id.UUID(p.ids.New()), p.orgA
			expectRefused(tb, "insert test run", p.b.InsertTestRun(ctx, run))
		}},
		{Subject: StoreMethod("EditorStore.GetTestRun"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			_, err := p.b.GetTestRun(ctx, p.orgA, p.runB.ID)
			expectNotFound(tb, "get test run", err)
		}},
		{Subject: StoreMethod("EditorStore.ListTestRuns"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			rows, err := p.b.ListTestRuns(ctx, p.orgA, p.flowB, 50)
			expectNoRows(tb, "list test runs", len(rows), err)
		}},
		{Subject: StoreMethod("EditorStore.InsertTestRunSeed"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			seed := p.seedB
			seed.ID, seed.OrgID = id.UUID(p.ids.New()), p.orgA
			expectRefused(tb, "insert test run seed", p.b.InsertTestRunSeed(ctx, seed))
		}},
		{Subject: StoreMethod("EditorStore.ListTestRunSeeds"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			rows, err := p.b.ListTestRunSeeds(ctx, p.orgA, p.runB.ID, 50)
			expectNoRows(tb, "list test run seeds", len(rows), err)
		}},
		{Subject: StoreMethod("EditorStore.InsertEffectPreview"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			pv := p.previewB
			pv.ID, pv.OrgID = id.UUID(p.ids.New()), p.orgA
			expectRefused(tb, "insert effect preview", p.b.InsertEffectPreview(ctx, pv))
		}},
		{Subject: StoreMethod("EditorStore.GetEffectPreview"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			_, err := p.b.GetEffectPreview(ctx, p.orgA, p.previewB.ID)
			expectNotFound(tb, "get effect preview", err)
		}},
		{Subject: StoreMethod("EditorStore.DecideEffectPreview"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			if err := p.b.DecideEffectPreview(ctx, p.orgA, p.previewB.ID, 1, editor.PreviewDispatchConfirmed, []byte(`{}`), identity.PrincipalID{}, editorBase); err == nil {
				tb.Error("a decision of organization A on a preview of organization B was accepted")
			}
			got, err := p.b.GetEffectPreview(ctx, p.orgB, p.previewB.ID)
			if err != nil || got.Status != editor.PreviewOpen {
				tb.Errorf("the preview of organization B was decided by another organization: %+v, %v", got, err)
			}
		}},
		{Subject: StoreMethod("EditorStore.ListEffectPreviews"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			rows, err := p.b.ListEffectPreviews(ctx, p.orgA, p.execB, 50)
			expectNoRows(tb, "list effect previews", len(rows), err)
		}},
		{Subject: StoreMethod("EditorStore.UpsertEditingSession"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			s := p.sessionB
			s.ID, s.OrgID = id.UUID(p.ids.New()), p.orgA
			expectRefused(tb, "open editing session", p.b.UpsertEditingSession(ctx, s))
		}},
		{Subject: StoreMethod("EditorStore.ListEditingSessions"), Run: func(ctx context.Context, tb testing.TB, env *Env) {
			p := openEditor(ctx, tb, env)
			rows, err := p.b.ListEditingSessions(ctx, p.orgA, p.flowB, editorBase, 50)
			expectNoRows(tb, "list editing sessions", len(rows), err)
		}},
	}
}

// addEditorCases registers the editor cases of the battery.
func addEditorCases(reg *Registry) error {
	for _, c := range editorCases() {
		if err := reg.Add(c); err != nil {
			return err
		}
	}
	return nil
}
