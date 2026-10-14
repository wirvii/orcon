// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation_test

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/wirvii/orcon/test/isolation"
)

// storeFlag is set after -args: go test ./test/isolation/... -args -store=postgres.
var storeFlag = flag.String("store", "sqlite", "store under test: sqlite or postgres")

func testEnv(t *testing.T) *isolation.Env {
	t.Helper()
	store, err := isolation.ParseStore(*storeFlag)
	if err != nil {
		t.Fatal(err)
	}
	env, err := isolation.NewEnv(store, isolation.RLSOnly)
	if err != nil {
		t.Fatal(err)
	}
	env.Editor = editorOpener(store)
	return env
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
}

// TestEditorIsolation runs the cross-organization cases of the visual editor storage (SPEC-014 ED2)
// for the chosen store: every operation of the editor port, from organization A against B.
func TestEditorIsolation(t *testing.T) {
	env := testEnv(t)
	isolation.EditorRegistry().Run(t.Context(), t, env)
}

// TestCrossOrgSmoke runs every registered case for the chosen store.
func TestCrossOrgSmoke(t *testing.T) {
	env := testEnv(t)
	if env.A.ID == env.B.ID || env.A.Sentinel("x") == env.B.Sentinel("x") {
		t.Fatalf("organizations A and B are not distinct: %+v %+v", env.A, env.B)
	}
	isolation.BuildRegistry().Run(t.Context(), t, env)
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
	reg.Run(t.Context(), t, env)
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
