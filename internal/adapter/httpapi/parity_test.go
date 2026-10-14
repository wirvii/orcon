// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/platform/config"
)

func newReal(t *testing.T) *Router {
	t.Helper()
	rt, err := New(&Deps{Role: config.RoleAPI, Idempotency: memStore()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return rt
}

// operationPatterns returns the patterns of the routes of kind.
func operationPatterns(routes []Route, kind RouteKind) []string {
	var out []string
	for _, r := range routes {
		if r.Kind == kind {
			out = append(out, r.Pattern)
		}
	}
	sort.Strings(out)
	return out
}

func specPatterns(spec *Spec) []string {
	var out []string
	for i := range spec.Operations {
		out = append(out, spec.Operations[i].Pattern)
	}
	sort.Strings(out)
	return out
}

// TestRouterMatchesOpenAPI is the parity gate of R-API-01: the routes the
// generated code registers are exactly the operations of the contract, in both
// directions.
func TestRouterMatchesOpenAPI(t *testing.T) {
	rt := newReal(t)
	if got, want := operationPatterns(rt.Routes(), KindOperation), specPatterns(rt.Spec()); !slices.Equal(got, want) {
		t.Errorf("registered operation routes %v, contract operations %v", got, want)
	}

	t.Run("synthetic contract passes", func(t *testing.T) {
		h := newHarness(t, nil)
		if got, want := operationPatterns(h.router.Routes(), KindOperation), specPatterns(h.router.Spec()); !slices.Equal(got, want) {
			t.Errorf("registered %v, contract %v", got, want)
		}
		if len(h.router.Spec().Operations) == 0 {
			t.Fatal("the synthetic contract has no operations")
		}
	})

	t.Run("a route that is no operation is refused", func(t *testing.T) {
		_, err := build(&Deps{Role: config.RoleAPI, Idempotency: memStore()}, []byte(syntheticSpec), func(mux apigen.ServeMux, base string) {
			fakeHandlers(mux, base, nil)
			mux.HandleFunc("GET "+base+"/hidden", func(http.ResponseWriter, *http.Request) {})
		})
		if err == nil || !strings.Contains(err.Error(), "/hidden") {
			t.Errorf("build error = %v, want it to name the stray route", err)
		}
	})

	t.Run("an operation without a route is refused", func(t *testing.T) {
		_, err := build(&Deps{Role: config.RoleAPI, Idempotency: memStore()}, []byte(syntheticSpec), func(mux apigen.ServeMux, base string) {
			mux.HandleFunc("GET "+base+"/ping", func(http.ResponseWriter, *http.Request) {})
		})
		if err == nil || !strings.Contains(err.Error(), "have no route") {
			t.Errorf("build error = %v, want it to name the missing routes", err)
		}
	})
}

// TestRouterParityExcludesAPIReference checks the parity rule as the criterion
// words it: the routes under /api/ other than /api/reference are exactly the
// operations of the contract.
func TestRouterParityExcludesAPIReference(t *testing.T) {
	rt := newReal(t)
	var underAPI []string
	for _, r := range rt.Routes() {
		path := r.Pattern
		if _, after, ok := strings.Cut(path, " "); ok {
			path = after
		}
		switch {
		case path == "/api/reference" || strings.HasPrefix(path, "/api/reference/"):
			if r.Kind != KindReference {
				t.Errorf("route %q is under /api/reference and not marked as reference", r.Pattern)
			}
		case r.Kind == KindFallback:
			// the catch-all answers with a problem document; it is no operation
		case strings.HasPrefix(path, "/api/"):
			underAPI = append(underAPI, r.Pattern)
		}
	}
	sort.Strings(underAPI)
	if want := specPatterns(rt.Spec()); !slices.Equal(underAPI, want) {
		t.Errorf("routes under /api/ %v, contract operations %v", underAPI, want)
	}
}

// protocolPaths are the endpoints whose shape an external standard fixes
// (ADR-0052 section 1): they must carry the protocol tag.
func isProtocolPath(path string) bool {
	if strings.HasPrefix(path, "/oauth/") {
		return true
	}
	return strings.HasPrefix(path, "/git-bindings/") && strings.HasSuffix(path, "/provider-hook")
}

// untaggedProtocol lists the operations on protocol paths without the tag.
func untaggedProtocol(spec *Spec) []string {
	var out []string
	for i := range spec.Operations {
		if op := &spec.Operations[i]; isProtocolPath(op.Path) && !op.Protocol {
			out = append(out, op.Pattern)
		}
	}
	return out
}

func TestProtocolEndpointsTagged(t *testing.T) {
	if bad := untaggedProtocol(newReal(t).Spec()); len(bad) > 0 {
		t.Errorf("protocol endpoints without the protocol tag: %v", bad)
	}

	spec, err := ParseSpec([]byte(syntheticSpec))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if bad := untaggedProtocol(spec); len(bad) != 0 {
		t.Errorf("synthetic contract: %v", bad)
	}
	untagged := strings.Replace(syntheticSpec, `"tags": ["protocol"], "security": [], "x-orcon-permission": "anonymous"`, `"x-orcon-permission": "flow:read"`, 1)
	spec, err = ParseSpec([]byte(untagged))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	if bad := untaggedProtocol(spec); len(bad) != 1 || !strings.Contains(bad[0], "/oauth/token") {
		t.Errorf("an untagged /oauth/token was not reported: %v", bad)
	}
}

func TestUnknownPathsUnderAPIAnswerWithAProblem(t *testing.T) {
	rt := newReal(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/flows", nil)
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if doc := problemOf(t, rec); doc["code"] != "not-found" {
		t.Errorf("code = %v, want not-found", doc["code"])
	}
}

func TestNewRefusesRolesThatDoNotServeTheAPI(t *testing.T) {
	for _, role := range []config.Role{config.RoleScheduler, config.RoleWorker, "bogus"} {
		if _, err := New(&Deps{Role: role, Idempotency: memStore()}); err == nil {
			t.Errorf("New accepted the role %q", role)
		}
	}
}

// TestNilIdempotencyStoreFailsClosed: the router has no silent in-memory default.
func TestNilIdempotencyStoreFailsClosed(t *testing.T) {
	_, err := New(&Deps{Role: config.RoleAPI})
	if !errors.Is(err, ErrNoIdempotencyStore) {
		t.Fatalf("New without a store = %v, want ErrNoIdempotencyStore", err)
	}
}
