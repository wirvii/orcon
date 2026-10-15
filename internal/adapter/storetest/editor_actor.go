// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/flow/editor"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// editorActorColumns are the actor columns that the editor tables add (migrations 0015 and 0016 of both
// adapters), as table and column. The PostgreSQL function that clears them is replaced by migration 0017.
var editorActorColumns = []struct{ table, column string }{
	{"node_test_data", "created_by_principal_id"},
	{"flow_node_pin", "pinned_by_principal_id"},
	{"test_run", "started_by_principal_id"},
	{"effect_preview", "decided_by_principal_id"},
	{"flow_editing_session", "opened_by_principal_id"},
	{"flow", "draft_updated_by_principal_id"},
}

// editorActorCase (E-PRIN-1, SPEC-014 ED2): deleting a principal sets each of the six editor actor columns
// that name it to NULL, in bounded batches, and deletes no row of history. Five of the columns are written
// through the editor port. A flow draft names the principal that last changed it, and no port writes that
// column in this cut, so the case sets it with SQL.
func editorActorCase(t *testing.T, f *editorFixture) {
	t.Helper()
	ctx := f.ctx()
	actor := identity.PrincipalRecord{ID: next[identity.PrincipalID](f.ids), Kind: identity.PrincipalUser, CreatedAt: base, UpdatedAt: base}
	mustTx(t, f.s.CreatePrincipal(ctx, actor), "create principal")
	by := actor.ID

	exec := f.execution()
	nodeExec := f.newNodeExecution(exec, "send")
	data := editor.NodeTestData{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, FlowID: f.flow.ID, NodeID: "n1", SourceKind: editor.SourceManual,
		OutputDocument: []byte(`{}`), ContentSHA256: sha32(0x01), CreatedByPrincipal: by, CreatedAt: base,
	}
	mustTx(t, f.ed.InsertNodeTestData(ctx, f.scopeA, data), "insert node test data")
	pin := editor.NodePin{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, FlowID: f.flow.ID, NodeID: "n1", NodeTestDataID: data.ID,
		PinnedByPrincipal: by, CreatedAt: base, UpdatedAt: base,
	}
	mustTx(t, f.ed.PinNode(ctx, f.scopeA, pin), "pin node")
	f.newRunStartedBy(exec, by)
	preview := editor.EffectPreview{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, ExecutionID: exec.ID, NodeExecutionID: nodeExec, NodeID: "send",
		Status: editor.PreviewOpen, Effect: editor.EffectWrite, SafetyLevel: editor.SafetyIdempotencyKey, ItemCount: 1,
		IsPreviewAvailable: true, PreviewDocument: []byte(`{}`), PreviewSHA256: sha32(0x02),
		ExpiresAt: base.Add(time.Hour), Revision: 1, CreatedAt: base, UpdatedAt: base,
	}
	mustTx(t, f.ed.InsertEffectPreview(ctx, f.scopeA, &preview), "insert effect preview")
	mustTx(t, f.ed.DecideEffectPreview(ctx, f.scopeA, preview.ID, 1, editor.PreviewCanceled, []byte(`{}`), by, base.Add(time.Minute)), "decide effect preview")
	session := editor.EditingSession{
		ID: next[id.UUID](f.ids), OrgID: f.orgA, FlowID: f.flow.ID, OpenedByPrincipal: by, ClientLabel: "editor-actor",
		OpenedAt: base, LastSeenAt: base, ExpiresAt: base.Add(time.Hour), Revision: 1, CreatedAt: base, UpdatedAt: base,
	}
	mustTx(t, f.ed.UpsertEditingSession(ctx, f.scopeA, session), "open editing session")
	setFlowDraftActor(t, f, by)

	for _, c := range editorActorColumns {
		if n := countRows(t, f, "SELECT count(*) FROM "+c.table+" WHERE "+c.column+" IS NOT NULL"); n != 1 {
			t.Fatalf("%s.%s names the principal in %d rows before the delete, want 1", c.table, c.column, n)
		}
	}

	if err := f.s.DeletePrincipal(ctx, by, 1); err != nil {
		t.Fatalf("delete principal: %v", err)
	}
	for _, c := range editorActorColumns {
		if n := countRows(t, f, "SELECT count(*) FROM "+c.table+" WHERE "+c.column+" IS NOT NULL"); n != 0 {
			t.Errorf("%s.%s still names the deleted principal in %d rows", c.table, c.column, n)
		}
		// A check that found no rows at all would pass on an empty table; the row must still be there.
		if n := countRows(t, f, "SELECT count(*) FROM "+c.table); n == 0 {
			t.Errorf("%s has no rows after the delete: the history must be kept", c.table)
		}
	}
	if got, err := f.ed.GetEffectPreview(ctx, f.scopeA, preview.ID); err != nil || !got.DecidedByPrincipal.IsZero() {
		t.Errorf("effect preview after its decider was deleted: decided by %v, %v", got.DecidedByPrincipal, err)
	}
}

// inOrgRaw runs fn on the store's own connection for organization org. SQLite has no row-level security.
// PostgreSQL applies row-level security to the owner of the tables too, so the transaction sets the
// organization the way the application does (orcon.org_id) and sees the rows of that organization only.
func inOrgRaw(t *testing.T, f *editorFixture, org identity.OrgID, fn func(ctx context.Context, tx pgx.Tx) error) error {
	t.Helper()
	ctx := f.ctx()
	pg, ok := f.ed.(*postgres.Store)
	if !ok {
		t.Fatalf("no raw access for %T", f.ed)
	}
	tx, err := pg.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('orcon.org_id', $1, true)", org.String()); err != nil {
		return err
	}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// countRows returns the result of a count query for the case's organization A.
func countRows(t *testing.T, f *editorFixture, query string) int64 {
	t.Helper()
	var n int64
	var err error
	switch v := f.ed.(type) {
	case *sqlite.Store:
		err = v.Reader().QueryRowContext(f.ctx(), query).Scan(&n)
	case *postgres.Store:
		err = inOrgRaw(t, f, f.orgA, func(ctx context.Context, tx pgx.Tx) error {
			return tx.QueryRow(ctx, query).Scan(&n)
		})
	default:
		t.Fatalf("no raw access for %T", f.ed)
	}
	if err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

// setFlowDraftActor records that the principal last changed the draft of the case's flow.
func setFlowDraftActor(t *testing.T, f *editorFixture, by identity.PrincipalID) {
	t.Helper()
	var err error
	switch v := f.ed.(type) {
	case *sqlite.Store:
		_, err = v.Writer().ExecContext(f.ctx(), "UPDATE flow SET draft_updated_by_principal_id = ? WHERE id = ?", by[:], f.flow.ID[:])
	case *postgres.Store:
		err = inOrgRaw(t, f, f.orgA, func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "UPDATE flow SET draft_updated_by_principal_id = $1 WHERE id = $2", [16]byte(by), [16]byte(f.flow.ID))
			return err
		})
	default:
		t.Fatalf("no raw access for %T", f.ed)
	}
	mustTx(t, err, "set the draft actor of the flow")
}
