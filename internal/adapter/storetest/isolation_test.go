// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/adapter/storetest/dbrules"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// TestEveryTableOrgScopedOrInstance is wall 2(d) of organization isolation
// (SPEC-004 R-ISO-02, orcon-db TI-1): every table of the migrated database has
// org_id NOT NULL or is listed in policy/instance_tables.txt, and none has an
// optional org_id.
func TestEveryTableOrgScopedOrInstance(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		policy, err := dbrules.LoadPolicy()
		if err != nil {
			t.Fatal(err)
		}
		cat := readCatalog(t.Context(), t, newStore(t))
		for _, tbl := range cat.tables {
			switch {
			case tbl.hasOrgID && policy.IsInstanceTable(tbl.name):
				t.Errorf("table %s has an org_id column and is listed as an instance table", tbl.name)
			case tbl.hasOrgID && !tbl.orgIDRequired:
				t.Errorf("table %s has an optional org_id", tbl.name)
			case !tbl.hasOrgID && !policy.IsInstanceTable(tbl.name):
				t.Errorf("table %s has no org_id and is not listed in instance_tables.txt", tbl.name)
			}
		}
		// A vacuous pass would hide an empty catalog: the tables this cut and its predecessors created must be there.
		for _, name := range []string{"project", "flow", "flow_version", "flow_snapshot", "membership"} {
			if tbl, ok := cat.table(name); !ok || !tbl.hasOrgID || !tbl.orgIDRequired {
				t.Errorf("table %s must exist with org_id NOT NULL: %+v", name, tbl)
			}
		}
		for _, name := range []string{"org", "principal", "application_user", "worker", "schema_migration"} {
			if tbl, ok := cat.table(name); !ok || tbl.hasOrgID || !policy.IsInstanceTable(name) {
				t.Errorf("table %s must exist as an instance table: %+v", name, tbl)
			}
		}
	})
}

// TestCompositeOrgForeignKeys checks wall 1's schema half (TI-7): a foreign key
// between organization tables is composite, (org_id, x) to (org_id, id), and never
// SET NULL; a foreign key from an organization table to an instance table is
// single-column. Then it proves the database itself rejects a child row whose
// organization differs from its parent's, without going through any store query.
func TestCompositeOrgForeignKeys(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		s := newStore(t)
		ctx := t.Context()
		cat := readCatalog(ctx, t, s)
		orgScoped := map[string]bool{}
		for _, name := range cat.orgTables() {
			orgScoped[name] = true
		}
		composite := 0
		for _, fk := range cat.fks {
			switch {
			case orgScoped[fk.table] && orgScoped[fk.refTable]:
				composite++
				if len(fk.columns) != 2 || fk.columns[0] != "org_id" || !sameStrings(fk.refs, []string{"org_id", "id"}) {
					t.Errorf("foreign key between organization tables is not (org_id, x) -> (org_id, id): %s", fk)
				}
				if fk.onDelete == "SET NULL" {
					t.Errorf("a composite foreign key never uses SET NULL (IN-6): %s", fk)
				}
			case orgScoped[fk.table] && !orgScoped[fk.refTable]:
				if len(fk.columns) != 1 {
					t.Errorf("foreign key to an instance table is single-column: %s", fk)
				}
			}
		}
		if composite == 0 {
			t.Fatal("no composite foreign key found: the catalog read is wrong")
		}
	})
}

// TestCrossOrgChildRowRejectedByDatabase inserts a child whose org_id differs from its
// parent's directly with SQL, which no store query does, and expects the composite
// foreign key to refuse it. A control insert with matching organizations must succeed,
// so the refusal is the organization and nothing else.
func TestCrossOrgChildRowRejectedByDatabase(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		r := rawOf(t, f.s)
		ctx := f.ctx()
		const insertFlow = "INSERT INTO flow (id, org_id, project_id, name, draft_document, draft_document_format, draft_revision, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'orcon/flow@1', 0, ?, ?)"
		at := r.instant(base)
		draft := []byte("{}")
		if err := r.exec(ctx, f.orgA, insertFlow, r.id(f.ids.New()), r.id(f.orgA), r.id(f.projA), "control", draft, at, at); err != nil {
			t.Fatalf("control: a flow of organization A in its own project: %v", err)
		}
		if err := r.exec(ctx, f.orgB, insertFlow, r.id(f.ids.New()), r.id(f.orgB), r.id(f.projA), "cross", draft, at, at); err == nil {
			t.Error("a flow of organization B was stored in the project of organization A")
		}
		rec := f.flow(f.orgA, f.projA, "parent")
		const insertVersion = "INSERT INTO flow_version (id, org_id, flow_id, version_number, document, document_format, content_hash, published_at, created_at) VALUES (?, ?, ?, 1, ?, 'orcon/flow@1', 'h', ?, ?)"
		if err := r.exec(ctx, f.orgA, insertVersion, r.id(f.ids.New()), r.id(f.orgA), r.id(rec.ID), draft, at, at); err != nil {
			t.Fatalf("control: a version of organization A for its own flow: %v", err)
		}
		other := f.flow(f.orgA, f.projA, "parent-2")
		if err := r.exec(ctx, f.orgB, insertVersion, r.id(f.ids.New()), r.id(f.orgB), r.id(other.ID), draft, at, at); err == nil {
			t.Error("a version of organization B was stored for the flow of organization A")
		}
	})
}

// TestZeroScopeIsRefused: the zero orgscope.Org scopes to nothing, so every store
// method of organization data refuses it before touching the database.
func TestZeroScopeIsRefused(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		ctx := f.ctx()
		var zero orgscope.Org
		if _, err := f.s.GetOrganization(ctx, zero); !errors.Is(err, identity.ErrNoScope) {
			t.Errorf("GetOrganization: %v, want ErrNoScope", err)
		}
		if _, err := f.s.GetProject(ctx, zero, f.projA); !errors.Is(err, identity.ErrNoScope) {
			t.Errorf("GetProject: %v, want ErrNoScope", err)
		}
		if _, err := f.s.ListMemberships(ctx, zero, identity.MembershipID{}, 10); !errors.Is(err, identity.ErrNoScope) {
			t.Errorf("ListMemberships: %v, want ErrNoScope", err)
		}
		rec := flow.Record{ID: flow.FlowID(f.ids.New()), OrgID: f.orgA, ProjectID: f.projA, Name: "zero", Draft: []byte(`{}`), CreatedAt: base, UpdatedAt: base}
		if err := f.s.CreateFlow(ctx, zero, rec); !errors.Is(err, identity.ErrNoScope) {
			t.Errorf("CreateFlow: %v, want ErrNoScope", err)
		}
		// A record that names another organization than its scope is refused too.
		if err := f.s.CreateFlow(ctx, scopeOf(f.orgB), rec); !errors.Is(err, identity.ErrScopeMismatch) {
			t.Errorf("CreateFlow under another organization's scope: %v, want ErrScopeMismatch", err)
		}
	})
}

// allowedIssueImporters are the paths whose non-test files may import the issuing
// package of orgscope.Org (ADR-0046 section 2): the authorization context, the
// adapter files that turn the rows of engine functions into scopes (engine_functions*.go
// of each database adapter, the only files of the adapters that may do so), the
// contract suite and the isolation battery. Test files may import it anywhere.
var allowedIssueImporters = []string{
	"internal/authz/",
	"internal/adapter/sqlite/engine_functions",
	"internal/adapter/postgres/engine_functions",
	"internal/adapter/storetest/",
	"internal/platform/orgscope/",
	"test/isolation/",
}

const (
	orgscopeDir   = "internal/platform/orgscope"
	issuePackage  = "github.com/wirvii/orcon/internal/platform/orgscope/issue"
	moduleRootDir = "go.mod"
)

// TestOrgScopeConstructorUnexported (E-ISO-1): an organization scope cannot be
// built outside the authorization context and the engine. Three facts make that
// true and this test checks each: the type's fields are unexported, so no composite
// literal fills it; package orgscope, which every store port names, exports no
// function or method that returns one; and the one package that does is imported,
// outside tests, only from the places above.
func TestOrgScopeConstructorUnexported(t *testing.T) {
	root := moduleRoot(t)

	typ := reflect.TypeOf(orgscope.Org{})
	if typ.Kind() != reflect.Struct || typ.NumField() == 0 {
		t.Fatalf("orgscope.Org is %v with %d fields; want a struct with fields", typ.Kind(), typ.NumField())
	}
	for i := range typ.NumField() {
		if f := typ.Field(i); f.IsExported() {
			t.Errorf("orgscope.Org has the exported field %s: a composite literal could fill it", f.Name)
		}
	}

	entries, err := os.ReadDir(filepath.Join(root, orgscopeDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, orgscopeDir, entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !fn.Name.IsExported() || fn.Type.Results == nil {
				continue
			}
			for _, res := range fn.Type.Results.List {
				if mentionsOrg(res.Type) {
					t.Errorf("package orgscope exports %s, which returns an Org: the constructor belongs in the issue package", fn.Name.Name)
				}
			}
		}
	}

	for _, dir := range []string{"internal", "cmd", "test", "packages/go"} {
		if _, err := os.Stat(filepath.Join(root, dir)); err != nil {
			continue // a tree this checkout does not have
		}
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if name := d.Name(); name == "node_modules" || name == "testdata" || name == "dist" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if perr != nil {
				return perr
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			for _, imp := range file.Imports {
				if p, _ := strconv.Unquote(imp.Path.Value); p == issuePackage && !hasAnyPrefix(rel, allowedIssueImporters) {
					t.Errorf("%s imports %s: an organization scope is created only by the authorization context and the engine", rel, issuePackage)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func mentionsOrg(e ast.Expr) bool {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name == "Org"
	case *ast.SelectorExpr:
		return v.Sel.Name == "Org"
	case *ast.StarExpr:
		return mentionsOrg(v.X)
	case *ast.ArrayType:
		return mentionsOrg(v.Elt)
	case *ast.MapType:
		return mentionsOrg(v.Value)
	}
	return false
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, moduleRootDir)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}
