// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// accessCatalog adds project:read-all, an organization permission, to the test catalog.
func accessCatalog(t *testing.T) *authz.Catalog {
	t.Helper()
	cat, err := authz.NewCatalog(map[authz.Scope][]authz.Permission{
		authz.ScopeOrganization: {"org:read", "org:update", "org:delete", "member:read", "role:read", "project:create", "project:read-all", "secret-store:reference"},
		authz.ScopeProject:      {"project:read", "project:update", "project:manage-members", "flow:read", "flow:create"},
	})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	return cat
}

func testScope(t *testing.T) orgscope.Org {
	t.Helper()
	scope, err := issue.New(id.UUID(testOrg))
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	return scope
}

// TestProjectAccessFollowsTheBindings (R-SH-PRJ-01, R-SH-PRJ-03): a Member reads no project and
// does not read every one; a project Viewer reads its project only; an organization Viewer reads
// every project but does not hold project:read-all; Owner and Admin hold project:read-all.
func TestProjectAccessFollowsTheBindings(t *testing.T) {
	cases := []struct {
		name         string
		bindings     []authz.Binding
		readsOne     bool
		readsTwo     bool
		readsEveryOf bool
	}{
		{"member", []authz.Binding{{Role: authz.RoleMember}}, false, false, false},
		{"project viewer", []authz.Binding{{Role: authz.RoleViewer, Project: projectOne}}, true, false, false},
		{"organization viewer", []authz.Binding{{Role: authz.RoleViewer}}, true, true, false},
		{"admin", []authz.Binding{{Role: authz.RoleAdmin}}, true, true, true},
		{"owner", []authz.Binding{{Role: authz.RoleOwner}}, true, true, true},
		{"no binding", nil, false, false, false},
	}
	for _, c := range cases {
		st := baseStore()
		st.bindings = c.bindings
		ev := authz.NewEvaluator(st, accessCatalog(t))
		access, err := ev.ProjectAccess(t.Context(), testUser, testScope(t))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := access.CanRead(projectOne); got != c.readsOne {
			t.Errorf("%s reads the first project = %v, want %v", c.name, got, c.readsOne)
		}
		if got := access.CanRead(projectTwo); got != c.readsTwo {
			t.Errorf("%s reads the second project = %v, want %v", c.name, got, c.readsTwo)
		}
		if got := access.ReadsEveryProject(); got != c.readsEveryOf {
			t.Errorf("%s holds project:read-all = %v, want %v", c.name, got, c.readsEveryOf)
		}
	}
	// A user the store does not know reads nothing and costs no lookup.
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleOwner}}
	none, err := authz.NewEvaluator(st, accessCatalog(t)).ProjectAccess(t.Context(), identity.UserID{}, testScope(t))
	if err != nil || none.CanRead(projectOne) || st.bindCalls != 0 {
		t.Errorf("zero user: reads %v, %d binding lookups, err %v; want nothing and no lookup", none.CanRead(projectOne), st.bindCalls, err)
	}
}

// TestProjectAccessUsesOneGrantLookup: the access is computed once from the cached grant, whatever
// the number of projects asked about.
func TestProjectAccessUsesOneGrantLookup(t *testing.T) {
	st := baseStore()
	st.bindings = []authz.Binding{{Role: authz.RoleViewer, Project: projectOne}}
	ev := authz.NewEvaluator(st, accessCatalog(t))
	for range 5 {
		if _, err := ev.ProjectAccess(t.Context(), testUser, testScope(t)); err != nil {
			t.Fatal(err)
		}
	}
	if st.bindCalls != 1 {
		t.Errorf("binding lookups = %d, want 1 while the epoch does not move", st.bindCalls)
	}
}

// fakeLister is a project store of a fixed list that records how many reads it served.
type fakeLister struct {
	rows  []identity.ProjectRecord
	reads int
	err   error
}

func (f *fakeLister) ListProjects(_ context.Context, _ orgscope.Org, after identity.ProjectID, limit int) ([]identity.ProjectRecord, error) {
	f.reads++
	if f.err != nil {
		return nil, f.err
	}
	var out []identity.ProjectRecord
	for i := range f.rows {
		if string(f.rows[i].ID[:]) > string(after[:]) {
			out = append(out, f.rows[i])
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

func listerOf(n int) (*fakeLister, []string) {
	seq := id.NewSequence()
	f := &fakeLister{}
	var ids []string
	for range n {
		p := identity.ProjectRecord{ID: identity.ProjectID(seq.New()), OrgID: testOrg}
		f.rows = append(f.rows, p)
		ids = append(ids, p.ID.String())
	}
	return f, ids
}

// TestReadableProjectsFiltersInBatches: the readable projects come back in order across batches,
// bounded by the limit; a user with no access reads nothing from the store; a user with named
// projects stops reading once each was found.
func TestReadableProjectsFiltersInBatches(t *testing.T) {
	store, ids := listerOf(7)
	everyone := authz.ProjectAccessOf(authz.Grant{Everywhere: []authz.Permission{"project:read"}})
	rows, err := authz.ReadableProjects(t.Context(), store, testScope(t), everyone, 3, 100)
	if err != nil || len(rows) != 7 || store.reads != 3 {
		t.Fatalf("everyone: %d rows in %d reads, err %v; want 7 rows in 3 reads", len(rows), store.reads, err)
	}
	limited, err := authz.ReadableProjects(t.Context(), store, testScope(t), everyone, 3, 4)
	if err != nil || len(limited) != 4 {
		t.Fatalf("limited: %d rows, err %v; want 4", len(limited), err)
	}

	some := authz.ProjectAccessOf(authz.Grant{PerProject: map[string][]authz.Permission{ids[1]: {"project:read"}, ids[2]: {"project:read"}, ids[6]: {"flow:read"}}})
	store.reads = 0
	rows, err = authz.ReadableProjects(t.Context(), store, testScope(t), some, 3, 100)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.ID.String())
	}
	if !slices.Equal(got, []string{ids[1], ids[2]}) || store.reads != 1 {
		t.Errorf("named projects: %v in %d reads, want the two with project:read found in the first read", got, store.reads)
	}

	none := authz.ProjectAccessOf(authz.Grant{})
	store.reads = 0
	if rows, err := authz.ReadableProjects(t.Context(), store, testScope(t), none, 3, 100); err != nil || len(rows) != 0 || store.reads != 0 {
		t.Errorf("no access: %d rows, %d reads, err %v; want nothing and no read", len(rows), store.reads, err)
	}

	boom := errors.New("store down")
	if _, err := authz.ReadableProjects(t.Context(), &fakeLister{rows: store.rows, err: boom}, testScope(t), everyone, 3, 100); !errors.Is(err, boom) {
		t.Errorf("a failing store = %v, want its error", err)
	}
}
