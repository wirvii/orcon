// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package middleware_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

var quiet = slog.New(slog.DiscardHandler)

func ok(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }

func serve(t *testing.T, h http.Handler, mut ...func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	for _, m := range mut {
		m(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestChainOrder(t *testing.T) {
	var order []string
	mark := func(name string) middleware.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	h := middleware.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { order = append(order, "handler") }), mark("a"), mark("b"))
	serve(t, h)
	if strings.Join(order, ",") != "a,b,handler" {
		t.Errorf("order = %v, want the first middleware outermost", order)
	}
}

func TestRequestIDInContext(t *testing.T) {
	var inside string
	h := middleware.RequestID(id.NewSequence())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inside = middleware.RequestIDFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := serve(t, h)
	if inside == "" || inside != rec.Header().Get(middleware.RequestIDHeader) {
		t.Errorf("context id %q, header id %q: they must be the same and not empty", inside, rec.Header().Get(middleware.RequestIDHeader))
	}
	if middleware.RequestIDFrom(t.Context()) != "" {
		t.Error("an id outside a request")
	}
}

func TestRecoverKeepsTheServerAlive(t *testing.T) {
	h := middleware.Recover(quiet)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := serve(t, h)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Error("the panic value reached the client")
	}
}

func TestRecoverRethrowsAbortHandler(t *testing.T) {
	h := middleware.Recover(quiet)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	defer func() {
		if recover() != http.ErrAbortHandler { //nolint:errorlint // identity comparison, as net/http documents
			t.Error("http.ErrAbortHandler was swallowed")
		}
	}()
	serve(t, h)
}

func TestBodyLimit(t *testing.T) {
	var readErr error
	h := middleware.BodyLimit(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			_, readErr = io.ReadAll(r.Body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := serve(t, h, func(r *http.Request) {
		r.Body = io.NopCloser(strings.NewReader("0123456789"))
		r.ContentLength = -1 // unknown length: the reader enforces the limit
	})
	var tooLarge *http.MaxBytesError
	if !errors.As(readErr, &tooLarge) {
		t.Errorf("read error = %v, want *http.MaxBytesError", readErr)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d", rec.Code)
	}
	rec = serve(t, h, func(r *http.Request) { r.ContentLength = 9 })
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("announced length over the limit = %d, want 413", rec.Code)
	}
	rec = serve(t, h, func(r *http.Request) { r.Body = nil })
	if rec.Code != http.StatusNoContent {
		t.Errorf("a request with no body = %d", rec.Code)
	}
}

type stubAuthn struct {
	p   identity.Principal
	err error
}

func (s stubAuthn) Authenticate(context.Context, identity.Credentials) (identity.Principal, error) {
	return s.p, s.err
}

func TestAuthenticateOutcomes(t *testing.T) {
	var uid id.UUID
	uid[15] = 7
	known := identity.Principal{ID: identity.PrincipalID(uid)}
	withBearer := func(r *http.Request) { r.Header.Set("Authorization", "Bearer abc") }
	tests := []struct {
		name string
		a    identity.Authenticator
		mut  func(*http.Request)
		want int
	}{
		{"no credential", stubAuthn{p: known}, nil, http.StatusUnauthorized},
		{"rejected", stubAuthn{err: identity.ErrUnauthenticated}, withBearer, http.StatusUnauthorized},
		{"csrf check failed", stubAuthn{err: identity.ErrCSRFCheckFailed}, withBearer, http.StatusForbidden},
		{"failed", stubAuthn{err: errors.New("database down")}, withBearer, http.StatusInternalServerError},
		{"empty principal", stubAuthn{}, withBearer, http.StatusInternalServerError},
		{"accepted", stubAuthn{p: known}, withBearer, http.StatusNoContent},
		{"scheme is case-insensitive", stubAuthn{p: known}, func(r *http.Request) { r.Header.Set("Authorization", "bearer abc") }, http.StatusNoContent},
		{"other scheme is no credential", stubAuthn{p: known}, func(r *http.Request) { r.Header.Set("Authorization", "Basic abc") }, http.StatusUnauthorized},
		{"cookie session", stubAuthn{p: known}, func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "__Host-orcon_session", Value: "s", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
		}, http.StatusNoContent},
	}
	for _, tt := range tests {
		h := middleware.Authenticate(tt.a, quiet, middleware.DefaultAuthConfig())(http.HandlerFunc(ok))
		var mut []func(*http.Request)
		if tt.mut != nil {
			mut = append(mut, tt.mut)
		}
		if rec := serve(t, h, mut...); rec.Code != tt.want {
			t.Errorf("%s: status = %d, want %d", tt.name, rec.Code, tt.want)
		}
	}
}

type captureAuthn struct{ got *identity.Credentials }

func (c captureAuthn) Authenticate(_ context.Context, creds identity.Credentials) (identity.Principal, error) {
	*c.got = creds
	return identity.Principal{}, identity.ErrUnauthenticated
}

func TestAuthenticatePassesWhatTheRequestCarries(t *testing.T) {
	var got identity.Credentials
	h := middleware.Authenticate(captureAuthn{&got}, quiet, middleware.DefaultAuthConfig())(http.HandlerFunc(ok))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/x", nil)
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Orcon-Csrf-Token", "csrf")
	req.Header.Set("Origin", "https://orcon.example.test")
	req.Header.Set("Orcon-Org", "acme")
	req.AddCookie(&http.Cookie{Name: "__Host-orcon_session", Value: "sess", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	req.AddCookie(&http.Cookie{Name: "__Host-orcon_csrf", Value: "csrf", Secure: true, SameSite: http.SameSiteStrictMode}) //nolint:gosec // the CSRF cookie is readable by design
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got.Bearer != "tok" || got.Session != "sess" || got.CSRFToken != "csrf" || got.CSRFCookie != "csrf" || got.Origin != "https://orcon.example.test" || got.OrgHint != "acme" || !got.UnsafeMethod {
		t.Errorf("credentials = %#v", got)
	}
	got = identity.Credentials{}
	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got.UnsafeMethod {
		t.Error("GET was treated as an unsafe method")
	}
}

type stubAuthz struct {
	d   authz.Decision
	err error
}

func (s stubAuthz) Authorize(context.Context, authz.Request) (authz.Decision, error) {
	return s.d, s.err
}

func TestAuthorizeOutcomes(t *testing.T) {
	need := authz.Requirement{Mode: authz.ModeAll, Permissions: []authz.Permission{"flow:read"}}
	resource := func(*http.Request) authz.ResourceRef { return authz.ResourceRef{} }
	var uid id.UUID
	uid[15] = 7
	withPrincipal := func(r *http.Request) {
		*r = *r.WithContext(middleware.WithPrincipal(r.Context(), identity.Principal{ID: identity.PrincipalID(uid)}))
	}
	tests := []struct {
		name string
		a    authz.Authorizer
		need authz.Requirement
		mut  func(*http.Request)
		want int
	}{
		{"allowed", stubAuthz{d: authz.DecisionAllow}, need, withPrincipal, http.StatusNoContent},
		{"forbidden", stubAuthz{d: authz.DecisionForbidden}, need, withPrincipal, http.StatusForbidden},
		{"hidden", stubAuthz{d: authz.DecisionHidden}, need, withPrincipal, http.StatusNotFound},
		{"error", stubAuthz{err: errors.New("down")}, need, withPrincipal, http.StatusInternalServerError},
		{"no principal", stubAuthz{d: authz.DecisionAllow}, need, nil, http.StatusUnauthorized},
		{"needs no decision", stubAuthz{d: authz.DecisionForbidden}, authz.Requirement{Mode: authz.ModeAuthenticated}, nil, http.StatusNoContent},
	}
	for _, tt := range tests {
		h := middleware.Authorize(tt.a, quiet, tt.need, resource)(http.HandlerFunc(ok))
		var mut []func(*http.Request)
		if tt.mut != nil {
			mut = append(mut, tt.mut)
		}
		if rec := serve(t, h, mut...); rec.Code != tt.want {
			t.Errorf("%s: status = %d, want %d", tt.name, rec.Code, tt.want)
		}
	}
	if _, found := middleware.PrincipalFrom(t.Context()); found {
		t.Error("a principal outside a request")
	}
}

func TestLimiterRefillsAtTheConfiguredRate(t *testing.T) {
	clk := clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))
	l := middleware.NewLimiter(clk, middleware.Limits{Read: 60, Write: 1, Run: 1, StreamConcurrent: 1, InteractiveFactor: 1, IP: 10})
	for i := range 60 {
		if d := l.Allow("p", middleware.ClassRead, false); !d.Allowed {
			t.Fatalf("request %d refused inside the quota", i+1)
		}
	}
	d := l.Allow("p", middleware.ClassRead, false)
	if d.Allowed || d.RetryAfter <= 0 || d.RetryAfter > 1100*time.Millisecond {
		t.Fatalf("past the quota: %+v; with 1 token per second the wait is about one second", d)
	}
	clk.Advance(3 * time.Second)
	d = l.Allow("p", middleware.ClassRead, false)
	if !d.Allowed || d.Remaining != 2 {
		t.Errorf("after 3 s: %+v, want allowed with 2 tokens left", d)
	}
	clk.Advance(time.Hour)
	if d = l.Allow("p", middleware.ClassRead, false); d.Remaining != 59 || d.Quota != 60 {
		t.Errorf("a long idle time must not bank more than the quota: %+v", d)
	}
}

func TestLimiterDivisorSharesTheRateAcrossProcesses(t *testing.T) {
	clk := clock.NewFake(time.Now())
	processes := 4
	l := middleware.NewLimiter(clk, middleware.Limits{Read: 8, Write: 8, Run: 8, StreamConcurrent: 4, InteractiveFactor: 1, IP: 8},
		middleware.WithDivisor(func() int { return processes }))
	if d := l.Allow("p", middleware.ClassRead, false); d.Quota != 2 {
		t.Errorf("quota with 4 live processes = %d, want 2", d.Quota)
	}
	processes = 100
	if d := l.Allow("q", middleware.ClassRead, false); d.Quota != 1 {
		t.Errorf("quota never goes below 1: %d", d.Quota)
	}
	if d := l.Allow("p", middleware.ClassWrite, false); d.Quota != 1 {
		t.Errorf("write quota = %d", d.Quota)
	}
}

func TestLimiterZeroQuotaRefuses(t *testing.T) {
	l := middleware.NewLimiter(clock.NewFake(time.Now()), middleware.Limits{})
	if d := l.Allow("p", middleware.ClassRead, false); d.Allowed {
		t.Error("a class with no quota allowed a request")
	}
}

func TestLimiterBoundsItsMemory(t *testing.T) {
	clk := clock.NewFake(time.Now())
	l := middleware.NewLimiter(clk, middleware.DefaultLimits(), middleware.WithMaxKeys(50))
	for i := range 500 {
		l.Allow("key-"+string(rune('a'+i%26))+string(rune('a'+i/26)), middleware.ClassRead, false)
	}
	// Every key above is distinct; the table never grows past the bound.
	if n := l.Tracked(); n > 50 {
		t.Errorf("tracked keys = %d, want at most 50", n)
	}
}

func TestLimiterStreams(t *testing.T) {
	l := middleware.NewLimiter(clock.NewFake(time.Now()), middleware.Limits{StreamConcurrent: 2, InteractiveFactor: 2})
	r1, _, ok1 := l.AcquireStream("p", false)
	r2, _, ok2 := l.AcquireStream("p", false)
	_, _, ok3 := l.AcquireStream("p", false)
	if !ok1 || !ok2 || ok3 {
		t.Fatalf("acquire results %v %v %v, want true true false", ok1, ok2, ok3)
	}
	r1()
	r1() // releasing twice must not free a second slot
	if _, _, ok := l.AcquireStream("p", false); !ok {
		t.Error("no slot after a release")
	}
	if _, _, ok := l.AcquireStream("p", false); ok {
		t.Error("a double release freed two slots")
	}
	r2()
	if _, q, ok := l.AcquireStream("web", true); !ok || q != 4 {
		t.Errorf("interactive profile quota = %d, ok = %v, want 4", q, ok)
	}
}

func TestLimiterIsSafeForConcurrentUse(t *testing.T) {
	l := middleware.NewLimiter(clock.NewFake(time.Now()), middleware.Limits{Read: 1_000_000, Write: 1, Run: 1, StreamConcurrent: 100, InteractiveFactor: 1, IP: 1})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				l.Allow("p", middleware.ClassRead, false)
				if release, _, ok := l.AcquireStream("p", false); ok {
					release()
				}
			}
		}()
	}
	wg.Wait()
}

func TestLimitAddressUsesTheConnectionAddress(t *testing.T) {
	l := middleware.NewLimiter(clock.NewFake(time.Now()), middleware.Limits{IP: 1})
	h := middleware.LimitAddress(l)(http.HandlerFunc(ok))
	if rec := serve(t, h); rec.Code != http.StatusNoContent {
		t.Fatalf("first = %d", rec.Code)
	}
	if rec := serve(t, h); rec.Code != http.StatusTooManyRequests {
		t.Errorf("second from the same address = %d, want 429", rec.Code)
	}
	if rec := serve(t, h, func(r *http.Request) { r.RemoteAddr = "192.0.2.77" }); rec.Code != http.StatusNoContent {
		t.Errorf("an address without a port = %d, want 204", rec.Code)
	}
}

func TestLimitPrincipalNeedsAnAuthenticatedPrincipal(t *testing.T) {
	l := middleware.NewLimiter(clock.NewFake(time.Now()), middleware.DefaultLimits())
	h := middleware.LimitPrincipal(l, middleware.ClassRead)(http.HandlerFunc(ok))
	if rec := serve(t, h); rec.Code != http.StatusUnauthorized {
		t.Errorf("no principal = %d, want 401", rec.Code)
	}
}
