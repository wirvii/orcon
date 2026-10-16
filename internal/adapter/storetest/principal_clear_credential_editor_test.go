// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/flow"
)

// TestPrincipalDeleteClearsEditorAndCredentialColumns (E-PRIN-1, orcon-db IN-12): the clearing of a
// principal (the PostgreSQL function replaced by migrations 0030 and 0031, the SQLite clearers) reaches
// the actor columns of the editor tables and of the credential tables in the same call, so neither
// migration drops the columns of the other.
func TestPrincipalDeleteClearsEditorAndCredentialColumns(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		r := rawOf(t, f.s)
		ctx := f.ctx()
		actor := f.user("editor-and-credential-actor@example.test")
		who := r.id(actor.PrincipalID)
		at := r.instant(base)

		// Editor side: the flow draft save actor and an editing session opened by the principal.
		draftFlow := f.flow(f.orgA, f.projA, "editor-and-credential-draft")
		if err := r.exec(ctx, f.orgA, "UPDATE flow SET draft_updated_at = ?, draft_updated_by_principal_id = ? WHERE id = ?", at, who, r.id(draftFlow.ID)); err != nil {
			t.Fatalf("draft actor: %v", err)
		}
		sessionFlow := f.flow(f.orgA, f.projA, "editor-and-credential-session")
		if err := r.exec(ctx, f.orgA, "INSERT INTO flow_editing_session (id, org_id, flow_id, opened_by_principal_id, client_label, last_seen_draft_revision, opened_at, last_seen_at, expires_at, revision, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			r.id(next[flow.FlowID](f)), r.id(f.orgA), r.id(sessionFlow.ID), who, "browser", 0, at, at, at, 1, at, at); err != nil {
			t.Fatalf("editing session: %v", err)
		}

		// Credential side: a credential, its version and a share, each naming the principal.
		credID := next[flow.FlowID](f)
		if err := r.exec(ctx, f.orgA, "INSERT INTO credential (id, org_id, project_id, name, connector_reference, status, allowed_host_document, allowed_host_document_format, current_version_number, created_by_principal_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			r.id(credID), r.id(f.orgA), r.id(f.projA), "cleared", "http", "OK", []byte(`["hooks.example.test"]`), "JSON", 1, who, at, at); err != nil {
			t.Fatalf("credential: %v", err)
		}
		if err := r.exec(ctx, f.orgA, "INSERT INTO credential_version (id, org_id, credential_id, version_number, public_field_document, public_field_document_format, secret_field_ciphertext, data_key_wrapped, org_key_version_number, created_by_principal_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			r.id(next[flow.FlowID](f)), r.id(f.orgA), r.id(credID), 1, []byte(`{}`), "JSON", make([]byte, 40), []byte{1}, 1, who, at, at); err != nil {
			t.Fatalf("credential version: %v", err)
		}
		if err := r.exec(ctx, f.orgA, "INSERT INTO credential_share (id, org_id, credential_id, project_id, granted_by_principal_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			r.id(next[flow.FlowID](f)), r.id(f.orgA), r.id(credID), r.id(f.projA), who, at, at); err != nil {
			t.Fatalf("credential share: %v", err)
		}

		if err := f.s.DeletePrincipal(ctx, actor.PrincipalID, 10); err != nil {
			t.Fatal(err)
		}

		for _, c := range []struct {
			what, query string
			arg         any
		}{
			{"flow.draft_updated_by_principal_id", "SELECT count(*) FROM flow WHERE id = ? AND draft_updated_at IS NOT NULL AND draft_updated_by_principal_id IS NULL", r.id(draftFlow.ID)},
			{"flow_editing_session.opened_by_principal_id", "SELECT count(*) FROM flow_editing_session WHERE flow_id = ? AND opened_by_principal_id IS NULL", r.id(sessionFlow.ID)},
			{"credential.created_by_principal_id", "SELECT count(*) FROM credential WHERE id = ? AND created_by_principal_id IS NULL", r.id(credID)},
			{"credential_version.created_by_principal_id", "SELECT count(*) FROM credential_version WHERE credential_id = ? AND created_by_principal_id IS NULL", r.id(credID)},
			{"credential_share.granted_by_principal_id", "SELECT count(*) FROM credential_share WHERE credential_id = ? AND granted_by_principal_id IS NULL", r.id(credID)},
		} {
			t.Run(c.what, func(t *testing.T) { mustCount(t, r, f.orgA, 1, c.query, c.arg) })
		}
	})
}
