// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migrationSQL = `
CREATE TABLE org (id uuid PRIMARY KEY);
CREATE TABLE principal (id uuid PRIMARY KEY);
CREATE TABLE project (org_id uuid NOT NULL, id uuid NOT NULL, name text, PRIMARY KEY (org_id, id));
CREATE TABLE flow (org_id uuid NOT NULL, id uuid NOT NULL, project_id uuid NOT NULL, name text, PRIMARY KEY (org_id, id));
CREATE TABLE flow_version (org_id uuid NOT NULL, id uuid NOT NULL, flow_id uuid NOT NULL, PRIMARY KEY (org_id, id));
CREATE TABLE scratch (id uuid);
DROP TABLE scratch;
`

const instanceTables = `# tables without org_id
org  # tenancy root
principal  # actor identity
`

// repo builds a repository tree below a temporary directory and returns its root.
type repo struct {
	t    *testing.T
	root string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	r := &repo{t: t, root: t.TempDir()}
	r.write("internal/adapter/postgres/migrations/0001_init.sql", migrationSQL)
	r.write("internal/adapter/storetest/dbrules/policy/instance_tables.txt", instanceTables)
	return r
}

func (r *repo) write(rel, content string) {
	r.t.Helper()
	path := filepath.Join(r.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		r.t.Fatal(err)
	}
}

// run executes sqlcheck on the postgres adapter and returns the exit status and the findings text.
func (r *repo) run() (int, string) {
	r.t.Helper()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-root", r.root, filepath.Join(r.root, "internal/adapter/postgres")}, &stdout, &stderr)
	return code, stdout.String() + stderr.String()
}

func (r *repo) queries(text string) { r.write("internal/adapter/postgres/queries/q.sql", text) }

func TestRejectsUnscopedQueries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		query string
		want  string // empty: the query is accepted; otherwise a fragment of the finding
	}{
		// Accepted shapes: the check must not raise false positives.
		{"select filtered", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT id, name FROM flow WHERE org_id = $1 AND id = $2;", ""},
		{"select alias and reversed", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT f.id FROM flow AS f WHERE $1 = f.org_id AND f.id = $2;", ""},
		{"sqlc named parameter", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id);", ""},
		{"sqlite placeholder", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = ? AND id = ?;", ""},
		{"cast parameter", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = $1::uuid AND id = $2;", ""},
		{"join restricted in ON", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT f.id, p.name FROM flow f JOIN project p ON p.org_id = f.org_id AND p.id = f.project_id WHERE f.org_id = $1 ORDER BY f.id LIMIT 10;", ""},
		{"join both restricted", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT f.id FROM flow f JOIN project p ON p.org_id = $1 AND p.id = f.project_id WHERE f.org_id = $1 LIMIT 10;", ""},
		{"join with instance table", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT f.id FROM flow f JOIN principal pr ON pr.id = f.project_id WHERE f.org_id = $1;", ""},
		{"subquery restricted", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = $1 AND project_id IN (SELECT id FROM project WHERE org_id = $1) LIMIT 10;", ""},
		{"exists restricted", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT f.id FROM flow f WHERE f.org_id = $1 AND EXISTS (SELECT 1 FROM flow_version v WHERE v.org_id = f.org_id AND v.flow_id = f.id) LIMIT 10;", ""},
		{"parenthesized conjunction", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT id FROM flow WHERE (org_id = $1 AND id > $2) LIMIT 10;", ""},
		{"cte restricted", "-- name: ListFlows :many\n-- orcon:scope org\nWITH mine AS (SELECT id FROM flow WHERE org_id = $1) SELECT id FROM mine LIMIT 10;", ""},
		{"union restricted", "-- name: ListThings :many\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = $1 UNION ALL SELECT id FROM project WHERE org_id = $1;", ""},
		{"update restricted", "-- name: RenameFlow :exec\n-- orcon:scope org\nUPDATE flow SET name = $3 WHERE org_id = $1 AND id = $2;", ""},
		{"update with from", "-- name: RenameFlow :exec\n-- orcon:scope org\nUPDATE flow SET name = p.name FROM project p WHERE flow.org_id = $1 AND p.org_id = flow.org_id AND p.id = flow.project_id;", ""},
		{"delete restricted", "-- name: DeleteFlow :exec\n-- orcon:scope org\n-- orcon:delete audit:flow.deleted\nDELETE FROM flow WHERE org_id = $1 AND id = $2;", ""},
		{"insert values", "-- name: CreateFlow :exec\n-- orcon:scope org\nINSERT INTO flow (org_id, id, project_id, name) VALUES ($1, $2, $3, $4);", ""},
		{"insert multiple tuples", "-- name: CreateFlows :exec\n-- orcon:scope org\nINSERT INTO flow (id, org_id, project_id, name) VALUES ($1, $2, $3, now()), ($4, $2, $3, now());", ""},
		{"insert on conflict", "-- name: UpsertFlow :exec\n-- orcon:scope org\nINSERT INTO flow (org_id, id, project_id, name) VALUES ($1, $2, $3, $4) ON CONFLICT (org_id, id) DO UPDATE SET name = excluded.name;", ""},
		{"insert select", "-- name: CopyFlow :exec\n-- orcon:scope org\nINSERT INTO flow (org_id, id, project_id, name) SELECT org_id, $2, project_id, name FROM flow WHERE org_id = $1 AND id = $3;", ""},
		{"select for update", "-- name: LockFlow :one\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = $1 AND id = $2 FOR UPDATE;", ""},
		{"instance table only", "-- name: GetPrincipal :one\n-- orcon:scope instance\nSELECT id FROM principal WHERE id = $1;", ""},
		{"comment mentions table", "-- name: GetPrincipal :one\n-- orcon:scope instance\n-- flow is read elsewhere\nSELECT id FROM principal WHERE id = $1;", ""},
		{"string mentions org_id", "-- name: GetPrincipal :one\n-- orcon:scope instance\nSELECT 'flow WHERE org_id = $1' FROM principal;", ""},
		{"function in from", "-- name: Series :many\n-- orcon:scope org\nSELECT n FROM generate_series(1, 3) AS n;", ""},
		{"table dropped in migrations", "-- name: Scratch :many\n-- orcon:scope org\nSELECT id FROM scratch;", ""},

		// Rejected shapes.
		{"select without org_id", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT id FROM flow WHERE id = $1;", "flow is not restricted"},
		{"select without where", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT id FROM flow LIMIT 10;", "flow is not restricted"},
		{"org_id compared to literal", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = 'x' AND id = $1;", "flow is not restricted"},
		{"org_id compared to a column of another table", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT f.id FROM flow f JOIN project p ON p.org_id = f.org_id WHERE p.id = $1;", "is not restricted"},
		{"org_id not equal", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id <> $1;", "flow is not restricted"},
		{"restriction under OR", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = $1 OR id = $2;", "flow is not restricted"},
		{"restriction under nested OR", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT id FROM flow WHERE id = $2 AND (org_id = $1 OR name = $3);", "flow is not restricted"},
		{"restriction under NOT", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT id FROM flow WHERE NOT (org_id = $1);", "flow is not restricted"},
		{"org_id only in select list", "-- name: GetFlow :one\n-- orcon:scope org\nSELECT org_id = $1 FROM flow WHERE id = $2;", "flow is not restricted"},
		{"join table unrestricted", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT f.id FROM flow f JOIN project p ON p.id = f.project_id WHERE f.org_id = $1;", "project is not restricted"},
		{"comma join unrestricted", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT f.id FROM flow f, project p WHERE f.org_id = $1 AND p.id = f.project_id;", "project is not restricted"},
		{"subquery unrestricted", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = $1 AND project_id IN (SELECT id FROM project WHERE name = $2);", "project is not restricted"},
		{"subquery restriction does not vouch for the outer table", "-- name: ListFlows :many\n-- orcon:scope org\nSELECT id FROM flow WHERE id IN (SELECT flow_id FROM flow_version WHERE org_id = $1);", "flow is not restricted"},
		{"cte unrestricted", "-- name: ListFlows :many\n-- orcon:scope org\nWITH mine AS (SELECT id FROM flow) SELECT id FROM mine;", "flow is not restricted"},
		{"union second branch unrestricted", "-- name: ListThings :many\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = $1 UNION ALL SELECT id FROM project;", "project is not restricted"},
		{"update without org_id", "-- name: RenameFlow :exec\n-- orcon:scope org\nUPDATE flow SET name = $2 WHERE id = $1;", "flow is not restricted"},
		{"update sets org_id only", "-- name: MoveFlow :exec\n-- orcon:scope org\nUPDATE flow SET org_id = $1 WHERE id = $2;", "flow is not restricted"},
		{"delete without org_id", "-- name: DeleteFlow :exec\n-- orcon:scope org\n-- orcon:delete audit:flow.deleted\nDELETE FROM flow WHERE id = $1;", "flow is not restricted"},
		{"insert without org_id column", "-- name: CreateFlow :exec\n-- orcon:scope org\nINSERT INTO flow (id, project_id, name) VALUES ($1, $2, $3);", "does not set org_id"},
		{"insert without column list", "-- name: CreateFlow :exec\n-- orcon:scope org\nINSERT INTO flow VALUES ($1, $2, $3, $4);", "explicit column list"},
		{"insert org_id from a literal", "-- name: CreateFlow :exec\n-- orcon:scope org\nINSERT INTO flow (org_id, id, project_id, name) VALUES ('x', $2, $3, $4);", "org_id value must be a parameter"},
		{"insert second tuple from a literal", "-- name: CreateFlows :exec\n-- orcon:scope org\nINSERT INTO flow (id, org_id, project_id, name) VALUES ($1, $2, $3, $4), ($5, 'x', $3, $4);", "org_id value must be a parameter"},
		{"insert select with a literal", "-- name: CopyFlow :exec\n-- orcon:scope org\nINSERT INTO flow (org_id, id, project_id, name) SELECT 'x', $2, project_id, name FROM flow WHERE org_id = $1;", "org_id value must be a parameter or an org_id column"},
		{"insert select unrestricted source", "-- name: CopyFlow :exec\n-- orcon:scope org\nINSERT INTO flow (org_id, id, project_id, name) SELECT $1, $2, project_id, name FROM flow WHERE id = $3;", "flow is not restricted"},
		{"instance scope touching an org table", "-- name: ListFlows :many\n-- orcon:scope instance\nSELECT id FROM flow WHERE id = $1;", "scope instance references organization-scoped table flow"},
		{"missing scope", "-- name: GetFlow :one\nSELECT id FROM flow WHERE org_id = $1;", "missing -- orcon:scope"},
		{"unknown scope", "-- name: GetFlow :one\n-- orcon:scope tenant\nSELECT id FROM flow WHERE org_id = $1;", "must be org, instance or engine"},
		{"engine not listed", "-- name: ClaimTimers :many\n-- orcon:scope engine\nSELECT id FROM flow WHERE id = $1;", "not listed in engine_allowlist.yaml"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			r.queries(tc.query)
			code, out := r.run()
			if tc.want == "" {
				if code != 0 {
					t.Fatalf("exit %d, want 0; output:\n%s", code, out)
				}
				return
			}
			if code != 1 {
				t.Fatalf("exit %d, want 1; output:\n%s", code, out)
			}
			if !strings.Contains(out, tc.want) {
				t.Fatalf("output lacks %q:\n%s", tc.want, out)
			}
		})
	}
}

func TestFindingNamesFileAndLine(t *testing.T) {
	t.Parallel()
	r := newRepo(t)
	r.queries("-- name: Good :one\n-- orcon:scope org\nSELECT id FROM flow WHERE org_id = $1;\n\n-- name: Bad :one\n-- orcon:scope org\nSELECT id\nFROM flow\nWHERE id = $1;\n")
	code, out := r.run()
	if code != 1 {
		t.Fatalf("exit %d; %s", code, out)
	}
	if !strings.Contains(out, "queries/q.sql:8: TI-3 query Bad: organization-scoped table flow") {
		t.Fatalf("finding does not name the file and the line of the table:\n%s", out)
	}
	if strings.Contains(out, "query Good") {
		t.Fatalf("the accepted query was reported:\n%s", out)
	}
}

func TestEngineAllowlist(t *testing.T) {
	t.Parallel()
	engineQuery := "-- name: ClaimTimers :many\n-- orcon:scope engine\nSELECT id FROM flow WHERE id = $1;"
	tests := []struct {
		name      string
		allowlist string
		want      string
	}{
		{"listed", "# reviewed\n- dialect: postgres\n  query: ClaimTimers\n  reason: cross-organization scan\n", ""},
		{"other dialect", "- dialect: sqlite\n  query: ClaimTimers\n  reason: x\n", "not listed"},
		{"other query", "- dialect: postgres\n  query: Other\n  reason: x\n", "not listed"},
		{"missing reason", "- dialect: postgres\n  query: ClaimTimers\n", "needs a reason"},
		{"bad dialect", "- dialect: mysql\n  query: ClaimTimers\n  reason: x\n", "must be sqlite or postgres"},
		{"stale entry", "- dialect: postgres\n  query: ClaimTimers\n  reason: x\n- dialect: postgres\n  query: Gone\n  reason: x\n", "Gone matches no query"},
		{"unreadable", "everything\n", "cannot read"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			r.queries(engineQuery)
			r.write("internal/adapter/storetest/dbrules/policy/engine_allowlist.yaml", tc.allowlist)
			code, out := r.run()
			if tc.want == "" {
				if code != 0 {
					t.Fatalf("exit %d; %s", code, out)
				}
				return
			}
			if code != 1 || !strings.Contains(out, tc.want) {
				t.Fatalf("exit %d, want 1 with %q:\n%s", code, tc.want, out)
			}
		})
	}
}

func TestRejectsSQLOutsideTheDataAccessLayer(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"internal/flow/queries.sql", "cmd/orcon/schema.sql", "internal/adapter/httpapi/q.sql", "packages/go/sdk/x.SQL"} {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			r.write(rel, "SELECT 1;")
			code, out := r.run()
			if code != 1 || !strings.Contains(out, "SQL-1") || !strings.Contains(out, filepath.Base(rel)) {
				t.Fatalf("exit %d, want a SQL-1 finding for %s:\n%s", code, rel, out)
			}
		})
	}
	for _, rel := range []string{
		"internal/adapter/sqlite/migrations/0001_x.sql", "internal/adapter/postgres/queries/a.sql",
		"internal/adapter/storetest/dbrules/testdata/bad/x/schema.sql", "tools/dbcheck/testdata/x.sql",
		".agents/skills/orcon-db/assets/template.sql", "node_modules/pkg/dump.sql",
	} {
		t.Run("allowed "+rel, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			r.write(rel, "-- a file that belongs here\n")
			if code, out := r.run(); code != 0 {
				t.Fatalf("exit %d for %s:\n%s", code, rel, out)
			}
		})
	}
}

func TestUsageErrors(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("no arguments: exit %d, want 2", code)
	}
	r := newRepo(t)
	if err := os.Remove(filepath.Join(r.root, "internal/adapter/storetest/dbrules/policy/instance_tables.txt")); err != nil {
		t.Fatal(err)
	}
	if code, _ := r.run(); code != 2 {
		t.Fatalf("missing policy list: exit %d, want 2", code)
	}
}

// TestRepositoryTree runs the check on the real adapters: the tree on main must
// pass, so a change that breaks the rule or the checker shows up here.
func TestRepositoryTree(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"-root", root, filepath.Join(root, "internal/adapter/sqlite"), filepath.Join(root, "internal/adapter/postgres")}
	if code := run(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d:\n%s%s", code, stdout.String(), stderr.String())
	}
}
