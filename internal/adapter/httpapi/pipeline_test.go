// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/platform/config"
)

func TestProblemJSONShape(t *testing.T) {
	h := newHarness(t, nil)
	hidden := func(authz.Request) authz.Decision { return authz.DecisionHidden }
	forbidden := func(authz.Request) authz.Decision { return authz.DecisionForbidden }
	tests := []struct {
		name   string
		method string
		path   string
		token  string
		decide func(authz.Request) authz.Decision
		mut    func(*http.Request)
		status int
		code   string
	}{
		{"no credential", http.MethodGet, "/api/v1/things/1", "", nil, nil, 401, "unauthenticated"},
		{"bad credential", http.MethodGet, "/api/v1/things/1", "wrong", nil, nil, 401, "unauthenticated"},
		{"forbidden", http.MethodGet, "/api/v1/things/1", "token-a", forbidden, nil, 403, "forbidden"},
		{"hidden", http.MethodGet, "/api/v1/things/1", "token-a", hidden, nil, 404, "not-found"},
		{"unknown path", http.MethodGet, "/api/v1/nowhere", "token-a", nil, nil, 404, "not-found"},
		{"wrong method on a known path", http.MethodDelete, "/api/v1/things/1", "token-a", nil, nil, 404, "not-found"},
		{"announced body too large", http.MethodPost, "/api/v1/things", "token-a", nil, func(r *http.Request) {
			r.ContentLength = DefaultBodyLimit + 1
		}, 413, "payload-too-large"},
		{"panic in the handler", http.MethodGet, "/api/v1/boom", "token-a", nil, nil, 500, "internal-error"},
		{"experimental without the header", http.MethodGet, "/api/v1/preview", "token-a", nil, nil, 400, "experimental-not-enabled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h.decide = func(authz.Request) authz.Decision { return authz.DecisionAllow }
			if tt.decide != nil {
				h.decide = tt.decide
			}
			var mut []func(*http.Request)
			if tt.mut != nil {
				mut = append(mut, tt.mut)
			}
			rec := h.do(tt.method, tt.path, tt.token, mut...)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.status, rec.Body.String())
			}
			doc := problemOf(t, rec)
			if doc["code"] != tt.code {
				t.Errorf("code = %v, want %s", doc["code"], tt.code)
			}
			if doc["type"] != "urn:orcon:problem:"+tt.code {
				t.Errorf("type = %v, want urn:orcon:problem:%s", doc["type"], tt.code)
			}
			if int(doc["status"].(float64)) != tt.status { //nolint:forcetypeassert,errcheck // JSON numbers decode to float64
				t.Errorf("status member = %v, want %d", doc["status"], tt.status)
			}
			if rid := rec.Header().Get("Orcon-Request-Id"); rid == "" || doc["requestId"] != rid {
				t.Errorf("requestId = %v, header = %q: they must be equal and present", doc["requestId"], rid)
			}
			for _, secret := range []string{"token-a", "secret-value", "kaboom"} {
				if strings.Contains(rec.Body.String(), secret) {
					t.Errorf("the problem document leaks %q: %s", secret, rec.Body.String())
				}
			}
			for _, banned := range []string{"data", "error", "meta", "success"} {
				if _, present := doc[banned]; present {
					t.Errorf("the problem document has the envelope member %q", banned)
				}
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	h := newHarness(t, nil)
	const sent = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a62"
	rec := h.do(http.MethodGet, "/api/v1/ping", "", func(r *http.Request) { r.Header.Set("Orcon-Request-Id", sent) })
	if got := rec.Header().Get("Orcon-Request-Id"); got != sent {
		t.Errorf("a well-formed client id was not kept: %q", got)
	}
	rec = h.do(http.MethodGet, "/api/v1/ping", "", func(r *http.Request) { r.Header.Set("Orcon-Request-Id", "not a uuid; DROP TABLE") })
	got := rec.Header().Get("Orcon-Request-Id")
	if got == "" || strings.Contains(got, "DROP") {
		t.Errorf("a malformed client id was not replaced: %q", got)
	}
}

func TestRateLimitHeaders(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Limits = middleware.Limits{Read: 3, Write: 3, Run: 3, StreamConcurrent: 1, InteractiveFactor: 1, IP: 1000}
	})
	rec := h.do(http.MethodGet, "/api/v1/me", "token-a")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got, want := rec.Header().Get("RateLimit-Policy"), `"read";q=3;w=60`; got != want {
		t.Errorf("RateLimit-Policy = %q, want %q", got, want)
	}
	if got := rec.Header().Get("RateLimit"); !strings.HasPrefix(got, `"read";r=2;t=`) {
		t.Errorf("RateLimit = %q, want it to start with %q", got, `"read";r=2;t=`)
	}
	h.do(http.MethodGet, "/api/v1/me", "token-a")
	h.do(http.MethodGet, "/api/v1/me", "token-a")
	rec = h.do(http.MethodGet, "/api/v1/me", "token-a")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status after the quota = %d, want 429", rec.Code)
	}
	if doc := problemOf(t, rec); doc["code"] != "rate-limited" {
		t.Errorf("code = %v, want rate-limited", doc["code"])
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Errorf("Retry-After = %q, want a positive number of seconds", ra)
	}
	if rec.Header().Get("RateLimit") == "" || rec.Header().Get("RateLimit-Policy") == "" {
		t.Error("a 429 must carry RateLimit and RateLimit-Policy")
	}
	h.clk.Advance(30 * time.Second)
	if rec = h.do(http.MethodGet, "/api/v1/me", "token-a"); rec.Code != http.StatusOK {
		t.Errorf("status after the bucket refilled = %d, want 200", rec.Code)
	}
}

func TestRateLimitPerPrincipal(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Limits = middleware.Limits{Read: 2, Write: 2, Run: 2, StreamConcurrent: 1, InteractiveFactor: 2, IP: 1000}
	})
	for range 2 {
		h.do(http.MethodGet, "/api/v1/me", "token-a")
	}
	if rec := h.do(http.MethodGet, "/api/v1/me", "token-a"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("principal A past its quota got %d, want 429", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/v1/me", "token-b"); rec.Code != http.StatusOK {
		t.Errorf("principal B was affected by A: status %d, want 200", rec.Code)
	}
	// The interactive profile has twice the quota: four reads pass.
	for i := range 4 {
		if rec := h.do(http.MethodGet, "/api/v1/me", "cookie-c"); rec.Code != http.StatusOK {
			t.Fatalf("interactive session request %d got %d, want 200", i+1, rec.Code)
		}
	}
	if rec := h.do(http.MethodGet, "/api/v1/me", "cookie-c"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("interactive session past its profile got %d, want 429", rec.Code)
	}
}

func TestRateLimitClassesAreSeparate(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Limits = middleware.Limits{Read: 1, Write: 1, Run: 1, StreamConcurrent: 1, InteractiveFactor: 1, IP: 1000}
	})
	h.do(http.MethodGet, "/api/v1/me", "token-a")
	if rec := h.do(http.MethodGet, "/api/v1/me", "token-a"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("read past its quota got %d, want 429", rec.Code)
	}
	if rec := h.do(http.MethodPost, "/api/v1/things", "token-a"); rec.Code != http.StatusOK {
		t.Errorf("the write class was spent by reads: status %d", rec.Code)
	}
}

func TestRateLimitPerAddressBeforeAuthentication(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Limits = middleware.Limits{Read: 100, Write: 100, Run: 100, StreamConcurrent: 1, InteractiveFactor: 1, IP: 2}
	})
	for range 2 {
		if rec := h.do(http.MethodGet, "/api/v1/ping", ""); rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	}
	rec := h.do(http.MethodGet, "/api/v1/ping", "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status past the address quota = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" || rec.Header().Get("Orcon-Request-Id") == "" {
		t.Error("the address limit answer must carry Retry-After and the request id")
	}
	if other := h.do(http.MethodGet, "/api/v1/ping", "", func(r *http.Request) { r.RemoteAddr = "192.0.2.99:1" }); other.Code != http.StatusOK {
		t.Errorf("another address was limited: status %d", other.Code)
	}
}

func TestNotFoundVsForbidden(t *testing.T) {
	h := newHarness(t, nil)
	tests := []struct {
		name     string
		decision authz.Decision
		status   int
	}{
		{"allowed", authz.DecisionAllow, http.StatusOK},
		{"another organization or no read permission is hidden", authz.DecisionHidden, http.StatusNotFound},
		{"read without the action is forbidden", authz.DecisionForbidden, http.StatusForbidden},
		{"an unknown decision denies", authz.Decision(99), http.StatusForbidden},
	}
	for _, tt := range tests {
		h.decide = func(authz.Request) authz.Decision { return tt.decision }
		if rec := h.do(http.MethodGet, "/api/v1/things/1", "token-a"); rec.Code != tt.status {
			t.Errorf("%s: status = %d, want %d", tt.name, rec.Code, tt.status)
		}
	}
}

func TestAuthorizerReceivesTheQuestion(t *testing.T) {
	h := newHarness(t, nil)
	var got authz.Request
	h.decide = func(r authz.Request) authz.Decision { got = r; return authz.DecisionAllow }
	if rec := h.do(http.MethodPut, "/api/v1/things/abc", "token-a", func(r *http.Request) { r.Header.Set("If-Match", `"1"`) }); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got.Principal.ID != userA.ID {
		t.Errorf("principal = %v, want %v", got.Principal.ID, userA.ID)
	}
	if got.Resource != (authz.ResourceRef{Kind: "thing", ID: "abc"}) {
		t.Errorf("resource = %+v, want thing/abc", got.Resource)
	}
	if got.Requirement.Mode != authz.ModeAll || len(got.Requirement.Permissions) != 2 {
		t.Errorf("requirement = %+v, want allOf of two permissions", got.Requirement)
	}
}

func TestAuthenticatedAndAnonymousOperationsSkipTheAuthorizer(t *testing.T) {
	h := newHarness(t, nil)
	h.decide = func(authz.Request) authz.Decision { return authz.DecisionForbidden }
	if rec := h.do(http.MethodGet, "/api/v1/me", "token-a"); rec.Code != http.StatusOK {
		t.Errorf("an authenticated-only operation was refused: %d", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/v1/ping", ""); rec.Code != http.StatusOK {
		t.Errorf("an anonymous operation required credentials: %d", rec.Code)
	}
	if rec := h.do(http.MethodPost, "/api/v1/oauth/token", ""); rec.Code != http.StatusOK {
		t.Errorf("a protocol operation with empty security required credentials: %d", rec.Code)
	}
}

func TestDefaultsRejectEveryProtectedRoute(t *testing.T) {
	// With no authenticator and no authorizer configured the router answers
	// 401 to every protected operation, and DenyAll would refuse the rest.
	rt, err := build(&Deps{Role: config.RoleStandalone}, []byte(syntheticSpec), func(mux apigen.ServeMux, base string) {
		fakeHandlers(mux, base, nil)
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/things/1", nil)
	req.Header.Set("Authorization", "Bearer anything")
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", rec.Header().Get("WWW-Authenticate"))
	}
}

func TestExperimentalGate(t *testing.T) {
	h := newHarness(t, nil)
	with := func(value string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("Orcon-Experimental", value) }
	}
	if rec := h.do(http.MethodGet, "/api/v1/preview", "token-a", with("getPreview")); rec.Code != http.StatusOK {
		t.Errorf("status with the feature named = %d, want 200", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/v1/preview", "token-a", with("other, GetPreview")); rec.Code != http.StatusOK {
		t.Errorf("status with the feature in a list = %d, want 200", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/v1/preview", "token-a", with("other")); rec.Code != http.StatusBadRequest {
		t.Errorf("status with another feature = %d, want 400", rec.Code)
	}
}

func TestStreamClassLimitsConcurrentStreams(t *testing.T) {
	h := newHarness(t, func(d *Deps) {
		d.Limits = middleware.Limits{Read: 100, Write: 100, Run: 100, StreamConcurrent: 1, InteractiveFactor: 1, IP: 1000}
	})
	entered := make(chan struct{})
	release := make(chan struct{})
	h.onStream = func() {
		entered <- struct{}{}
		<-release
	}
	var wg sync.WaitGroup
	first := httptest.NewRecorder()
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/things/stream", nil)
		req.Header.Set("Authorization", "Bearer token-a")
		h.router.ServeHTTP(first, req)
	}()
	<-entered
	h.onStream = nil
	if rec := h.do(http.MethodGet, "/api/v1/things/stream", "token-a"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("a second concurrent stream got %d, want 429", rec.Code)
	}
	if rec := h.do(http.MethodGet, "/api/v1/things/stream", "token-b"); rec.Code != http.StatusOK {
		t.Errorf("another principal's stream got %d, want 200", rec.Code)
	}
	close(release)
	wg.Wait()
	if first.Code != http.StatusOK {
		t.Errorf("the first stream got %d, want 200", first.Code)
	}
	if rec := h.do(http.MethodGet, "/api/v1/things/stream", "token-a"); rec.Code != http.StatusOK {
		t.Errorf("a stream after the first ended got %d, want 200", rec.Code)
	}
}
