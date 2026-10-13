// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"context"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// wantSchemaCases is the number of planted schema violations of SPEC-009 4.4.6.
const wantSchemaCases = 23

func TestSchemaLintGoodSQLite(t *testing.T) {
	db := openSQLite(t)
	execSQLite(t, db, readFile(t, "testdata/good/sqlite/schema.sql"))
	if got := LintSQLite(t.Context(), db, mustPolicy(t)); len(got) != 0 {
		t.Fatalf("the good fixture has findings:\n%s", describe(got))
	}
}

func TestSchemaLintPlantedSQLite(t *testing.T) {
	dirs := caseDirs(t, "bad")
	if len(dirs) != wantSchemaCases {
		t.Fatalf("found %d planted schema cases, SPEC-009 4.4.6 requires %d", len(dirs), wantSchemaCases)
	}
	ran := 0
	for _, dir := range dirs {
		if !hasDir(filepath.Join(dir, "sqlite")) {
			continue
		}
		ran++
		t.Run(filepath.Base(dir), func(t *testing.T) {
			db := openSQLite(t)
			execSQLite(t, db, readFile(t, filepath.Join(dir, "sqlite", "schema.sql")))
			got := LintSQLite(t.Context(), db, mustPolicy(t))
			if want := expectedRules(t, dir); !reflect.DeepEqual(rules(got), want) {
				t.Fatalf("rules = %v, want exactly %v\n%s", rules(got), want, describe(got))
			}
		})
	}
	if ran == 0 {
		t.Fatal("no SQLite case ran")
	}
}

func TestSchemaLintGoodPostgres(t *testing.T) {
	conn := connectPostgres(t, newPostgresDatabase(t))
	execPostgres(t, conn, readFile(t, "testdata/good/postgres/schema.sql"))
	if got := LintPostgres(t.Context(), conn, mustPolicy(t)); len(got) != 0 {
		t.Fatalf("the good fixture has findings:\n%s", describe(got))
	}
}

func TestSchemaLintPlantedPostgres(t *testing.T) {
	requirePostgres(t)
	dirs := caseDirs(t, "bad")
	if len(dirs) != wantSchemaCases {
		t.Fatalf("found %d planted schema cases, SPEC-009 4.4.6 requires %d", len(dirs), wantSchemaCases)
	}
	for _, dir := range dirs {
		if !hasDir(filepath.Join(dir, "postgres")) {
			continue
		}
		t.Run(filepath.Base(dir), func(t *testing.T) {
			conn := connectPostgres(t, newPostgresDatabase(t))
			execPostgres(t, conn, readFile(t, filepath.Join(dir, "postgres", "schema.sql")))
			got := LintPostgres(t.Context(), conn, mustPolicy(t))
			if want := expectedRules(t, dir); !reflect.DeepEqual(rules(got), want) {
				t.Fatalf("rules = %v, want exactly %v\n%s", rules(got), want, describe(got))
			}
		})
	}
}

func TestExternalIDWildcardMustNotShadowATable(t *testing.T) {
	db := openSQLite(t)
	execSQLite(t, db, readFile(t, "testdata/good/sqlite/schema.sql"))
	policy := func(line string) *Policy {
		t.Helper()
		fsys := policyWith(t, fileExternalIDs, "*.node_id  # inside a document\n"+line+"\n")
		p, err := LoadPolicyFS(fsys, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if got := LintSQLite(t.Context(), db, policy("")); len(got) != 0 {
		t.Fatalf("findings:\n%s", describe(got))
	}
	got := LintSQLite(t.Context(), db, policy("*.flow_id  # wrong: table flow exists"))
	if !strings.Contains(describe(got), "IN-1 policy external_ids.txt: entry *.flow_id") {
		t.Fatalf("the wildcard over an existing table was not flagged:\n%s", describe(got))
	}
}

// policyWith returns the embedded policy with one file replaced.
func policyWith(t *testing.T, name, content string) fs.FS {
	t.Helper()
	sub, err := fs.Sub(embeddedPolicy, "policy")
	if err != nil {
		t.Fatal(err)
	}
	return overlayFS{base: sub, name: name, content: content}
}

type overlayFS struct {
	base    fs.FS
	name    string
	content string
}

func (o overlayFS) Open(name string) (fs.File, error) {
	if name == o.name {
		return fstest.MapFS{name: {Data: []byte(o.content)}}.Open(name)
	}
	return o.base.Open(name)
}

func TestEnumValueRule(t *testing.T) {
	db := openSQLite(t)
	execSQLite(t, db, readFile(t, "testdata/bad/enum-check-lowercase/sqlite/schema.sql"))
	if got := LintSQLite(t.Context(), db, mustPolicy(t)); !strings.Contains(describe(got), `enumeration value "revoked" in CHECK ck_credential_status of credential is not UPPER_SNAKE (fix: 'REVOKED')`) {
		t.Fatalf("findings:\n%s", describe(got))
	}
	// An owner-approved NM-16 line for the constraint silences it.
	fsys := policyWith(t, fileExceptions, "NM-16  sqlite:credential.ck_credential_status  external standard  2099-01-01  approved-by:owner\n")
	p, err := LoadPolicyFS(fsys, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := LintSQLite(t.Context(), db, p); len(got) != 0 {
		t.Fatalf("the exception was not honored:\n%s", describe(got))
	}
}

func TestEnumValueCases(t *testing.T) {
	words := []string{"status", "kind", "format"}
	tests := []struct {
		expr string
		bad  []string
	}{
		{"status IN ('ACTIVE', 'revoked')", []string{"revoked"}},
		{"status IN ('ACTIVE', 'NOT_FOUND', 'OAUTH2')", nil},
		{"status NOT IN ('a')", []string{"a"}},
		{"status = ANY (ARRAY['ACTIVE'::text, 'revoked'::text])", []string{"revoked"}},
		{"((status)::text = ANY ((ARRAY['A'::character varying, 'b'::character varying])::text[]))", []string{"b"}},
		{"status <> ALL (ARRAY['x'])", []string{"x"}},
		{"status = 'Active'", []string{"Active"}},
		{"'Active' = status", []string{"Active"}},
		{"status != 'draft'", []string{"draft"}},
		{"name <> ''", nil},
		{"status IN ('')", []string{""}},
		{"body_document_format IN ('orcon/flow@1')", nil},
		{"kind IN ('OAUTH2')", nil},
		{"status IN ('it''s')", []string{"it's"}},
		{"is_active IN (0, 1)", nil},
		{"status <= 'a'", nil},
	}
	for _, tt := range tests {
		toks, err := tokenizePostgres(tt.expr)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, l := range badEnumLiterals(toEnumToks(toks), words) {
			got = append(got, l.value)
		}
		if !reflect.DeepEqual(got, tt.bad) {
			t.Errorf("%s: bad values %q, want %q", tt.expr, got, tt.bad)
		}
	}
	if got := upperSnakeFix("notification-link"); got != "NOTIFICATION_LINK" {
		t.Errorf("fix = %q", got)
	}
}

// TestRowsAreVisibleThroughTheOrganizationContext proves the TI-4 policy with
// the SETOF uuid form of current_org_ids(): no context shows nothing, one
// organization shows its rows, and orcon.org_ids with two organizations shows both.
func TestRowsAreVisibleThroughTheOrganizationContext(t *testing.T) {
	conn := connectPostgres(t, newPostgresDatabase(t))
	execPostgres(t, conn, readFile(t, "testdata/good/postgres/schema.sql"))
	const orgA, orgB = "00000000-0000-0000-0000-00000000000a", "00000000-0000-0000-0000-00000000000b"
	execPostgres(t, conn, `
INSERT INTO org (id, name, status, created_at, updated_at) VALUES ('`+orgA+`', 'a', 'ACTIVE', now(), now()), ('`+orgB+`', 'b', 'ACTIVE', now(), now());
INSERT INTO project (id, org_id, name, created_at, updated_at) VALUES
  ('00000000-0000-0000-0000-000000000001', '`+orgA+`', 'one', now(), now()),
  ('00000000-0000-0000-0000-000000000002', '`+orgB+`', 'two', now(), now())`)
	count := func(settings ...string) int {
		t.Helper()
		tx, err := conn.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(context.WithoutCancel(t.Context())) }()
		if _, err := tx.Exec(t.Context(), "SET LOCAL ROLE orcon_application"); err != nil {
			t.Fatal(err)
		}
		for i := 0; i+1 < len(settings); i += 2 {
			if _, err := tx.Exec(t.Context(), "SELECT set_config($1, $2, true)", settings[i], settings[i+1]); err != nil {
				t.Fatal(err)
			}
		}
		var n int
		if err := tx.QueryRow(t.Context(), "SELECT count(*) FROM project").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(); n != 0 {
		t.Errorf("no context: %d rows, want 0", n)
	}
	if n := count("orcon.org_id", orgA); n != 1 {
		t.Errorf("one organization: %d rows, want 1", n)
	}
	if n := count("orcon.org_ids", orgA+","+orgB); n != 2 {
		t.Errorf("two organizations in orcon.org_ids: %d rows, want 2", n)
	}
	if n := count("orcon.org_ids", orgB); n != 1 {
		t.Errorf("one organization in orcon.org_ids: %d rows, want 1", n)
	}
}
