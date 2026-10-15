// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// The shared command of criterion E-PRIN-1 passes the flag -store to the test binary of every package it
// names, this one included. The store-backed cases it runs live in internal/adapter/storetest, so only the
// flag is declared here.
var _ = flag.String("store", "", "ignored here: the store-backed cases are in internal/adapter/storetest")

// fakeStore is an in-memory ProjectStore and OrgStore with the uniqueness rules of the real tables:
// a slug is unique per organization and a handle is unique in the instance.
type fakeStore struct {
	orgs     map[identity.OrgID]identity.OrgRecord
	projects []identity.ProjectRecord
	// failWith, when set, is returned by CreateProject instead of writing.
	failWith error
}

func newFakeStore() *fakeStore { return &fakeStore{orgs: map[identity.OrgID]identity.OrgRecord{}} }

func (f *fakeStore) CreateOrganization(_ context.Context, o identity.OrgRecord) error {
	f.orgs[o.ID] = o
	return nil
}

func (f *fakeStore) GetOrganization(_ context.Context, org orgscope.Org) (identity.OrgRecord, error) {
	o, ok := f.orgs[identity.OrgIDOf(org)]
	if !ok {
		return identity.OrgRecord{}, identity.ErrNotFound
	}
	return o, nil
}

func (f *fakeStore) CreateProject(_ context.Context, org orgscope.Org, p identity.ProjectRecord, _ audit.Entry) error {
	if f.failWith != nil {
		return f.failWith
	}
	if !p.OrgID.Matches(org) {
		return identity.ErrScopeMismatch
	}
	for i := range f.projects {
		other := &f.projects[i]
		if other.OrgID == p.OrgID && other.Slug == p.Slug {
			return fmt.Errorf("slug %q: %w", p.Slug, identity.ErrConflict)
		}
		if other.Handle == p.Handle {
			// A driver error quotes the colliding value; the service must not pass it on.
			return fmt.Errorf("handle %q held by project %q of organization %q: %w", other.Handle, other.Name, f.orgs[other.OrgID].Name, identity.ErrHandleTaken)
		}
	}
	f.projects = append(f.projects, p)
	return nil
}

func (f *fakeStore) GetProject(context.Context, orgscope.Org, identity.ProjectID) (identity.ProjectRecord, error) {
	return identity.ProjectRecord{}, identity.ErrNotFound
}

func (f *fakeStore) ListProjects(context.Context, orgscope.Org, identity.ProjectID, int) ([]identity.ProjectRecord, error) {
	return nil, nil
}

type world struct {
	t        *testing.T
	store    *fakeStore
	projects *identity.Projects
	ids      *id.Sequence
}

func newWorld(t *testing.T) *world {
	t.Helper()
	store := newFakeStore()
	ids := id.NewSequence()
	clk := clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))
	return &world{t: t, store: store, projects: identity.NewProjects(store, store, clk, ids), ids: ids}
}

// org creates an organization with a name and a slug and returns its scope.
func (w *world) org(name, slug string) orgscope.Org {
	w.t.Helper()
	rec := identity.OrgRecord{ID: identity.OrgID(w.ids.New()), Name: name, Slug: slug, Status: identity.OrgActive}
	if err := w.store.CreateOrganization(context.Background(), rec); err != nil {
		w.t.Fatal(err)
	}
	scope, err := issue.New(id.UUID(rec.ID))
	if err != nil {
		w.t.Fatal(err)
	}
	return scope
}

func (w *world) create(org orgscope.Org, name, slug string) (identity.ProjectRecord, error) {
	w.t.Helper()
	return w.projects.Create(context.Background(), org, identity.NewProject{Name: name, Slug: slug})
}

func TestProjectHandleUniquePerInstance(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	acme, globex := w.org("Acme Corporation", "acme"), w.org("Globex Industries", "globex")

	first, err := w.create(acme, "Payments", "payments")
	if err != nil || first.Handle != "payments" {
		t.Fatalf("first project: handle %q, err %v; want the slug", first.Handle, err)
	}
	other, err := w.create(globex, "Ledger", "ledger")
	if err != nil || other.Handle != "ledger" {
		t.Fatalf("a slug nobody holds: handle %q, err %v; want the slug", other.Handle, err)
	}
	second, err := w.create(globex, "Payments", "payments")
	if err != nil {
		t.Fatal(err)
	}
	if second.Handle == first.Handle {
		t.Fatalf("two projects share the handle %q", second.Handle)
	}
	seen := map[string]bool{}
	for i := range w.store.projects {
		handle := w.store.projects[i].Handle
		if seen[handle] {
			t.Errorf("handle %q is held twice", handle)
		}
		seen[handle] = true
	}
	// The slug stays unique inside one organization: that is a conflict, not a handle problem.
	if _, err := w.create(acme, "Again", "payments"); !errors.Is(err, identity.ErrConflict) || errors.Is(err, identity.ErrHandleUnavailable) {
		t.Errorf("a repeated slug in the organization: %v, want ErrConflict", err)
	}
	if !first.IsProtected {
		t.Error("a new project is protected")
	}
}

func TestProjectHandleFallbackOrgPrefix(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	acme, globex := w.org("Acme Corporation", "acme"), w.org("Globex Industries", "globex")
	if _, err := w.create(acme, "Payments", "payments"); err != nil {
		t.Fatal(err)
	}
	got, err := w.create(globex, "Payments", "payments")
	if err != nil {
		t.Fatal(err)
	}
	if want := "globex-payments"; got.Handle != want {
		t.Errorf("handle = %q, want %q (<orgSlug>-<projectSlug>)", got.Handle, want)
	}
	if got.Slug != "payments" {
		t.Errorf("the slug stays %q whatever the handle is", got.Slug)
	}
}

func TestHandleCollisionMessageGeneric(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	acme, globex, initech := w.org("Acme Corporation", "acme"), w.org("Globex Industries", "globex"), w.org("Initech", "initech")
	// Acme holds "payments" and Initech holds "globex-payments": both candidates of Globex are taken.
	if _, err := w.create(acme, "Secret Payments Project", "payments"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.create(initech, "Initech Billing", "globex-payments"); err != nil {
		t.Fatal(err)
	}
	_, err := w.create(globex, "Payments", "payments")
	if !errors.Is(err, identity.ErrHandleUnavailable) {
		t.Fatalf("error = %v, want ErrHandleUnavailable", err)
	}
	if errors.Is(err, identity.ErrHandleTaken) || errors.Is(err, identity.ErrConflict) {
		t.Error("the caller must not see the store's own collision error")
	}
	for _, leak := range []string{"Acme", "acme", "Secret", "Initech", "initech", "Billing", "globex-payments"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("the collision message %q reveals %q", err.Error(), leak)
		}
	}
	if got, want := err.Error(), identity.ErrHandleUnavailable.Error(); got != want {
		t.Errorf("message = %q, want exactly %q", got, want)
	}
}

func TestCreateProjectValidation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	org := w.org("Acme", "acme")
	long := strings.Repeat("x", 101)
	for name, in := range map[string]identity.NewProject{
		"no name":          {Slug: "a"},
		"no slug":          {Name: "A"},
		"slug too long":    {Name: "A", Slug: long},
		"name too long":    {Name: strings.Repeat("n", 201), Slug: "a"},
		"label too long":   {Name: "A", Slug: "a", Environment: long + long},
		"all fields empty": {},
	} {
		if _, err := w.projects.Create(t.Context(), org, in); !errors.Is(err, identity.ErrInvalidInput) {
			t.Errorf("%s: error %v, want ErrInvalidInput", name, err)
		}
	}
	if len(w.store.projects) != 0 {
		t.Errorf("%d projects were stored by invalid requests", len(w.store.projects))
	}
}

func TestCreateProjectPassesOtherFailuresThrough(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	org := w.org("Acme", "acme")
	boom := errors.New("disk on fire")
	w.store.failWith = boom
	if _, err := w.create(org, "A", "a"); !errors.Is(err, boom) {
		t.Errorf("error = %v, want the store's error", err)
	}
	// An unknown organization fails before any project is written.
	w.store.failWith = nil
	ghost, err := issue.New(w.ids.New())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.create(ghost, "A", "a"); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("unknown organization: %v, want ErrNotFound", err)
	}
}

func TestOrgIDMatchesScope(t *testing.T) {
	t.Parallel()
	ids := id.NewSequence()
	u := ids.New()
	scope, err := issue.New(u)
	if err != nil {
		t.Fatal(err)
	}
	if !identity.OrgID(u).Matches(scope) {
		t.Error("an organization matches its own scope")
	}
	if identity.OrgID(ids.New()).Matches(scope) {
		t.Error("another organization does not match")
	}
	if (identity.OrgID{}).Matches(orgscope.Org{}) {
		t.Error("the zero scope matches nothing, not even the nil organization")
	}
	if identity.OrgIDOf(scope) != identity.OrgID(u) {
		t.Error("OrgIDOf returns the organization of the scope")
	}
}
