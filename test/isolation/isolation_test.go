// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation_test

import (
	"context"
	"errors"
	"flag"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
	"github.com/wirvii/orcon/test/isolation"
)

// storeFlag returns the value of -store, set after -args: go test ./test/isolation/... -args -store=postgres.
//
//	auto      SQLite, and PostgreSQL when ORCON_TEST_POSTGRES=1 (default)
//	sqlite    SQLite only
//	postgres  PostgreSQL only; the test starts a server and fails if it cannot
//	both      SQLite and PostgreSQL; PostgreSQL must be available
//
// The flag is defined once, in executiontest, because this package links
// storetest, which links executiontest: a second flag.String("store") panics
// at start-up with "flag redefined: store".
func storeFlag() string { return executiontest.StoreFlag("auto") }

func TestMain(m *testing.M) {
	flag.Parse()
	switch storeFlag() {
	case "postgres", "both":
		storetest.EnablePostgres()
	}
	if os.Getenv("ORCON_TEST_POSTGRES") == "1" || storetest.PostgresEnabled() {
		// The container client keeps goroutines of its own for the whole run.
		_ = runWithPostgres(m) // the test wrapper takes the exit code from m.Run
		return
	}
	goleak.VerifyTestMain(m)
}

// selectedStores returns the stores this run covers.
func selectedStores(t *testing.T) []isolation.Store {
	t.Helper()
	stores, err := isolation.ParseStores(storeFlag(), storetest.PostgresEnabled() || os.Getenv("ORCON_TEST_POSTGRES") == "1")
	if err != nil {
		t.Fatal(err)
	}
	return stores
}

func factoryOf(t *testing.T, st isolation.Store) storetest.Factory {
	t.Helper()
	if st == isolation.StorePostgres {
		return storetest.PostgresFactory()
	}
	return storetest.SQLiteFactory()
}

// fakeSurface is a surface with fixed contents.
type fakeSurface struct{ routes, methods []string }

func (f fakeSurface) Routes() []string       { return f.routes }
func (f fakeSurface) StoreMethods() []string { return f.methods }

// TestRegistryComplete fails when a route or store method that handles
// organization data has no isolation case, or a case outlived its subject.
func TestRegistryComplete(t *testing.T) {
	reg := isolation.BuildRegistry()
	for _, s := range reg.Missing(isolation.DefaultSurface) {
		t.Errorf("%s has no isolation case; add one to test/isolation", s)
	}
	for _, s := range reg.Orphans(isolation.DefaultSurface) {
		t.Errorf("the isolation case for %s has no route or store method; remove it", s)
	}
	if len(isolation.DefaultSurface.StoreMethods()) == 0 {
		t.Error("the surface has no store methods: the registry would be complete by being empty")
	}
}

// TestSurfaceIsEverythingTheAdaptersImplement ties the surface to the adapters: the methods
// of the SQLite store and of the PostgreSQL store that implement ports are exactly the
// methods the battery has a case for.
func TestSurfaceIsEverythingTheAdaptersImplement(t *testing.T) {
	want := append(isolation.StoreSurface{}.StoreMethods(), isolation.UncoveredStoreMethods...)
	slices.Sort(want)
	for adapter, got := range storetest.PortMethodNames() {
		if !slices.Equal(got, want) {
			t.Errorf("the %s store implements %v; the isolation surface is %v (add the port to isolation.Stores and write its cases)", adapter, got, want)
		}
	}
}

// TestCrossOrgSmoke runs every registered case for the chosen stores, each on a fresh
// database seeded with organizations A and B.
func TestCrossOrgSmoke(t *testing.T) {
	if isolation.RLSOnly {
		t.Skip("the rls-only mode needs the test hook that switches the filter of wall 1 off, which the full battery cut (SPEC-004 SE17) delivers; running the cases here would not test wall 3 alone")
	}
	reg := isolation.BuildRegistry()
	for _, st := range selectedStores(t) {
		t.Run(string(st), func(t *testing.T) {
			newStore := factoryOf(t, st)
			reg.Run(t.Context(), t, func(t *testing.T) *isolation.Env {
				t.Helper()
				env := isolation.Seed(t.Context(), t, newStore(t), st, isolation.RLSOnly)
				if env.A.ID == env.B.ID || env.A.Sentinel("x") == env.B.Sentinel("x") {
					t.Fatalf("organizations A and B are not distinct: %+v %+v", env.A.ID, env.B.ID)
				}
				return env
			})
		})
	}
}

// TestSeededOrganizationsAreIndependent checks the harness itself: what Seed stores for A
// is invisible under B's scope and the other way round, so the cases start from isolation
// that already holds for reads they did not write.
func TestSeededOrganizationsAreIndependent(t *testing.T) {
	for _, st := range selectedStores(t) {
		t.Run(string(st), func(t *testing.T) {
			env := isolation.Seed(t.Context(), t, factoryOf(t, st)(t), st, false)
			ctx := t.Context()
			if got, err := env.Stores.GetFlow(ctx, env.A.Scope, env.A.Flow.ID); err != nil || got.Name != env.A.Sentinel("flow-name") {
				t.Errorf("A's flow under A's scope = %+v, %v", got, err)
			}
			if got, err := env.Stores.GetFlow(ctx, env.B.Scope, env.A.Flow.ID); err == nil {
				t.Errorf("A's flow under B's scope = %+v", got)
			}
			if got, err := env.Stores.GetFlow(ctx, env.A.Scope, env.B.Flow.ID); err == nil {
				t.Errorf("B's flow under A's scope = %+v", got)
			}
		})
	}
}

func TestRegistryDetectsMissingAndOrphanCases(t *testing.T) {
	t.Parallel()
	noop := func(context.Context, testing.TB, *isolation.Env) {}
	reg := isolation.NewRegistry()
	for _, c := range []isolation.Case{
		{Subject: isolation.Route("GET /v1/flows"), Run: noop},
		{Subject: isolation.StoreMethod("FlowStore.Get"), Run: noop},
		{Subject: isolation.Route("GET /v1/removed"), Run: noop},
	} {
		if err := reg.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	surface := fakeSurface{
		routes:  []string{"GET /v1/flows", "POST /v1/flows"},
		methods: []string{"FlowStore.Get", "FlowStore.List"},
	}
	var missing []string
	for _, s := range reg.Missing(surface) {
		missing = append(missing, s.String())
	}
	if got, want := strings.Join(missing, "|"), "route POST /v1/flows|store method FlowStore.List"; got != want {
		t.Errorf("Missing = %q, want %q", got, want)
	}
	orphans := reg.Orphans(surface)
	if len(orphans) != 1 || orphans[0].Name != "GET /v1/removed" {
		t.Errorf("Orphans = %v, want the removed route", orphans)
	}
}

func TestRegistryRejectsBadCases(t *testing.T) {
	t.Parallel()
	noop := func(context.Context, testing.TB, *isolation.Env) {}
	reg := isolation.NewRegistry()
	if err := reg.Add(isolation.Case{Subject: isolation.Route("GET /a")}); !errors.Is(err, isolation.ErrInvalidCase) {
		t.Errorf("a case without a body: err = %v, want ErrInvalidCase", err)
	}
	if err := reg.Add(isolation.Case{Run: noop}); !errors.Is(err, isolation.ErrInvalidCase) {
		t.Errorf("a case without a subject: err = %v, want ErrInvalidCase", err)
	}
	if err := reg.Add(isolation.Case{Subject: isolation.Route("GET /a"), Run: noop}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Add(isolation.Case{Subject: isolation.Route("GET /a"), Run: noop}); !errors.Is(err, isolation.ErrDuplicateCase) {
		t.Errorf("a second case for one subject: err = %v, want ErrDuplicateCase", err)
	}
	// The same name as a route and as a store method are different subjects.
	if err := reg.Add(isolation.Case{Subject: isolation.StoreMethod("GET /a"), Run: noop}); err != nil {
		t.Errorf("same name, other kind: %v", err)
	}
}

func TestRegistryRunsEveryCase(t *testing.T) {
	t.Parallel()
	env, err := isolation.NewEnv(isolation.StoreSQLite, false)
	if err != nil {
		t.Fatal(err)
	}
	var ran []string
	reg := isolation.NewRegistry()
	for _, name := range []string{"B", "A"} {
		if err := reg.Add(isolation.Case{
			Subject: isolation.Route(name),
			Run: func(_ context.Context, _ testing.TB, got *isolation.Env) {
				if got != env {
					t.Errorf("case %s got another environment", name)
				}
				ran = append(ran, name)
			},
		}); err != nil {
			t.Fatal(err)
		}
	}
	reg.Run(t.Context(), t, func(*testing.T) *isolation.Env { return env })
	if got := strings.Join(ran, ""); got != "AB" {
		t.Errorf("cases ran as %q, want AB (sorted)", got)
	}
}

func TestNewEnvRefusesRLSOnlyOutsidePostgres(t *testing.T) {
	t.Parallel()
	if _, err := isolation.NewEnv(isolation.StoreSQLite, true); !errors.Is(err, isolation.ErrRLSOnlyNeedsPostgres) {
		t.Errorf("sqlite with rls-only: err = %v", err)
	}
	if env, err := isolation.NewEnv(isolation.StorePostgres, true); err != nil || !env.RLSOnly {
		t.Errorf("postgres with rls-only: env = %+v, err = %v", env, err)
	}
}

func TestParseStore(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"sqlite", "postgres"} {
		if got, err := isolation.ParseStore(ok); err != nil || string(got) != ok {
			t.Errorf("ParseStore(%q) = %q, %v", ok, got, err)
		}
	}
	for _, bad := range []string{"", "mysql", "SQLite"} {
		if _, err := isolation.ParseStore(bad); !errors.Is(err, isolation.ErrUnknownStore) {
			t.Errorf("ParseStore(%q): err = %v, want ErrUnknownStore", bad, err)
		}
	}
}

func TestParseStores(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in        string
		available bool
		want      string
	}{
		{"sqlite", false, "sqlite"}, {"postgres", true, "postgres"}, {"both", false, "sqlite,postgres"},
		{"auto", false, "sqlite"}, {"auto", true, "sqlite,postgres"},
	} {
		got, err := isolation.ParseStores(tc.in, tc.available)
		var names []string
		for _, s := range got {
			names = append(names, string(s))
		}
		if err != nil || strings.Join(names, ",") != tc.want {
			t.Errorf("ParseStores(%q, %v) = %v, %v; want %s", tc.in, tc.available, names, err, tc.want)
		}
	}
	if _, err := isolation.ParseStores("mysql", false); !errors.Is(err, isolation.ErrUnknownStore) {
		t.Errorf("ParseStores(mysql): err = %v, want ErrUnknownStore", err)
	}
}

func TestSentinelsFindLeaks(t *testing.T) {
	t.Parallel()
	env, err := isolation.NewEnv(isolation.StoreSQLite, false)
	if err != nil {
		t.Fatal(err)
	}
	secret := env.B.Sentinel("flow-name")
	if got := isolation.Leaks(`{"title":"path unavailable"}`, secret, env.B.ID); len(got) != 0 {
		t.Errorf("a clean body leaked %v", got)
	}
	body := `{"detail":"owned by ` + secret + ` in organization ` + env.B.ID + `"}`
	if got := isolation.Leaks(body, secret, env.B.ID, ""); len(got) != 2 {
		t.Errorf("Leaks = %v, want both values (an empty value never matches)", got)
	}
	if strings.Contains(env.A.Sentinel("flow-name"), env.B.Sentinel("flow-name")) || strings.Contains(env.B.Sentinel("flow-name"), env.A.Sentinel("flow-name")) {
		t.Error("one organization's sentinel contains the other's")
	}
}

// TestResolveUserOrganizationsOnlyActive (SPEC-004 section 7-3, SPEC-016 R-SH-ORG-04): the organizations of a
// user are those of its ACTIVE memberships in ACTIVE organizations. A suspended membership, an invitation, a
// suspended organization and an organization the user does not belong to are left out.
func TestResolveUserOrganizationsOnlyActive(t *testing.T) {
	for _, st := range selectedStores(t) {
		t.Run(string(st), func(t *testing.T) {
			ctx := t.Context()
			env := isolation.Seed(ctx, t, factoryOf(t, st)(t), st, isolation.RLSOnly)
			ana := env.A.User
			fresh := id.NewV7Generator(time.Now)
			now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
			membership := func(org identity.OrgID, status string) {
				t.Helper()
				scope := scopeOfOrg(t, org)
				rec := identity.MembershipRecord{ID: identity.MembershipID(fresh.New()), OrgID: org, UserID: ana.ID, Status: status, CreatedAt: now, UpdatedAt: now}
				if err := env.Stores.CreateMembership(ctx, scope, rec); err != nil {
					t.Fatalf("membership of ana in %s: %v", status, err)
				}
			}
			newOrg := func(status string) identity.OrgID {
				t.Helper()
				org := identity.OrgID(fresh.New())
				name := "org-" + status + "-" + org.String()
				if err := env.Stores.CreateOrganization(ctx, identity.OrgRecord{ID: org, Name: name, Slug: name, Status: status, CreatedAt: now, UpdatedAt: now}); err != nil {
					t.Fatalf("create organization %s: %v", status, err)
				}
				return org
			}
			// Ana is ACTIVE in A (seeded). Suspended membership in B, an invitation in C, and an
			// ACTIVE membership in a suspended organization E; D is an organization she does not belong to.
			membership(identity.OrgID(env.B.UUID), identity.MembershipSuspended)
			membership(newOrg(identity.OrgActive), identity.MembershipInvited)
			membership(newOrg(identity.OrgSuspended), identity.MembershipActive)
			newOrg(identity.OrgActive)

			got, err := env.Stores.ResolveUserOrganizations(ctx, ana.ID, identity.MaxResolvedOrganizations)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if len(got) != 1 || got[0].ID != identity.OrgID(env.A.UUID) {
				t.Fatalf("organizations of ana = %+v; want only A", got)
			}
		})
	}
}

// scopeOfOrg issues the scope of an organization created by the test.
func scopeOfOrg(t *testing.T, org identity.OrgID) orgscope.Org {
	t.Helper()
	scope, err := issue.New(id.UUID(org))
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	return scope
}
