// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package idempotency_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/idempotency"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/idempotencyclaim"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

const route = "POST /things/{thingId}"

// org is the organization of every principal of these tests.
var org = func() id.UUID { var u id.UUID; u[15] = 99; return u }()

func principal(n byte, credential string, kind identity.CredentialType) identity.Principal {
	var u id.UUID
	u[15] = n
	return identity.Principal{ID: identity.PrincipalID(u), OrgID: identity.OrgID(org), CredentialType: kind, CredentialID: credential}
}

var (
	keyA = principal(1, "key-a", identity.CredentialPersonalKey)
	keyB = principal(1, "key-b", identity.CredentialPersonalKey)
)

// rig is a mux with one idempotent route. Its handler stands for an operation that claims
// its key in its transaction: it writes the record with the store and answers 201.
type rig struct {
	t       *testing.T
	clk     *clock.Fake
	store   *idempotency.MemoryStore
	runs    atomic.Int32
	handler http.HandlerFunc
	who     identity.Principal
	mux     *http.ServeMux
}

func newRig(t *testing.T, cfg func(*idempotency.Config, *rig)) *rig {
	t.Helper()
	r := &rig{t: t, clk: clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)), who: keyA}
	r.store = idempotency.NewMemoryStore(r.clk)
	r.handler = func(w http.ResponseWriter, req *http.Request) {
		n := r.runs.Add(1)
		if err := r.store.Claim(req.Context(), scopeOf(), "things/created"); err != nil {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/api/v1/things/created")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"run":%d}`, n)
	}
	c := idempotency.Config{Store: r.store, Clock: r.clk, IDs: id.NewSequence(), Logger: slog.New(slog.DiscardHandler), Route: route}
	if cfg != nil {
		cfg(&c, r)
	}
	idem := idempotency.Middleware(c)
	r.mux = http.NewServeMux()
	r.mux.Handle(route, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := authz.WithOrgScope(middleware.WithPrincipal(req.Context(), r.who), scopeOf())
		req = req.WithContext(ctx)
		idem(r.handler).ServeHTTP(w, req)
	}))
	return r
}

// scopeOf is the organization scope of the rig's principals, issued the way tests may issue it.
func scopeOf() orgscope.Org {
	s, err := issue.New(org)
	if err != nil {
		panic(err)
	}
	return s
}

func (r *rig) post(key, body string) *httptest.ResponseRecorder {
	r.t.Helper()
	req := httptest.NewRequestWithContext(r.t.Context(), http.MethodPost, "/things/thing-1", strings.NewReader(body))
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	r.mux.ServeHTTP(rec, req)
	return rec
}

// preclaim writes the record of a request that is still running, as its transaction would.
func (r *rig) preclaim(key string, p identity.Principal, body string) {
	r.t.Helper()
	fp, err := idempotency.Fingerprint(route, []string{"thingId=thing-1"}, []byte(body))
	if err != nil {
		r.t.Fatal(err)
	}
	requester := "PRINCIPAL:" + p.ID.String()
	if p.CredentialType != identity.CredentialSession {
		requester = string(p.CredentialType) + ":" + p.CredentialID
	}
	ctx := idempotencyclaim.With(r.t.Context(), idempotencyclaim.Claim{
		ID: id.NewSequence().New(), Requester: requester, Value: key, Fingerprint: fp,
	})
	if err := r.store.Claim(ctx, scopeOf(), "things/created"); err != nil {
		r.t.Fatal(err)
	}
}

func problemCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var doc struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("not a problem document: %v (%s)", err, rec.Body.String())
	}
	return doc.Code
}

func TestIdempotencyReplay(t *testing.T) {
	r := newRig(t, nil)
	first := r.post("key-1", `{"name":"a"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first = %d, want 201 (%s)", first.Code, first.Body.String())
	}
	again := r.post("key-1", `{ "name" : "a" }`)
	if again.Code != http.StatusCreated {
		t.Fatalf("repeat = %d, want 201", again.Code)
	}
	if got := again.Header().Get(idempotency.ReplayedHeader); got != "true" {
		t.Errorf("%s = %q, want true", idempotency.ReplayedHeader, got)
	}
	if again.Body.String() != first.Body.String() {
		t.Errorf("repeat body = %q, want the first body %q", again.Body.String(), first.Body.String())
	}
	if again.Header().Get("Location") != "/api/v1/things/created" {
		t.Errorf("repeat Location = %q", again.Header().Get("Location"))
	}
	if n := r.runs.Load(); n != 1 {
		t.Errorf("the operation ran %d times, want 1", n)
	}
}

func TestIdempotencyFingerprintMismatch(t *testing.T) {
	r := newRig(t, nil)
	r.post("key-1", `{"name":"a"}`)
	rec := r.post("key-1", `{"name":"b"}`)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "idempotency-key-reused" {
		t.Fatalf("different body = %d %s, want 422 idempotency-key-reused", rec.Code, rec.Body.String())
	}
}

func TestClientErrorReleasesTheKey(t *testing.T) {
	r := newRig(t, nil)
	r.handler = func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) }
	if rec := r.post("key-1", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("first = %d, want 400", rec.Code)
	}
	r.handler = func(w http.ResponseWriter, req *http.Request) {
		if err := r.store.Claim(req.Context(), scopeOf(), "things/created"); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
	}
	if rec := r.post("key-1", `{}`); rec.Code != http.StatusCreated {
		t.Fatalf("after a client error = %d, want the key released and 201", rec.Code)
	}
}

func TestIdempotencyInProgress(t *testing.T) {
	r := newRig(t, nil)
	r.preclaim("key-1", keyA, `{}`)
	rec := r.post("key-1", `{}`)
	if rec.Code != http.StatusConflict || problemCode(t, rec) != "idempotency-in-progress" {
		t.Fatalf("running twin = %d %s, want 409 idempotency-in-progress", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("the conflict carries no Retry-After")
	}
	if r.runs.Load() != 0 {
		t.Errorf("the operation ran while its twin was running")
	}
}

// TestIdempotencySecretNotStored: a response that issues a secret is shown once. The record keeps
// no body, a repeat is refused with 409 secret-already-issued, the repeat does not leak the secret
// and it points at the resource.
func TestIdempotencySecretNotStored(t *testing.T) {
	const secret = "example-secret-1"
	r := newRig(t, func(c *idempotency.Config, _ *rig) { c.IssuesSecret = true })
	r.handler = func(w http.ResponseWriter, req *http.Request) {
		r.runs.Add(1)
		if err := r.store.Claim(req.Context(), scopeOf(), "api-keys/created"); err != nil {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/api/v1/api-keys/created")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"created","secret":"` + secret + `"}`))
	}
	first := r.post("key-1", `{}`)
	if first.Code != http.StatusCreated || !strings.Contains(first.Body.String(), secret) {
		t.Fatalf("first = %d %s: the creator must see the secret once", first.Code, first.Body.String())
	}
	key := identity.IdempotencyKey{Requester: string(keyA.CredentialType) + ":" + keyA.CredentialID, Value: "key-1"}
	rec, found, err := r.store.LookupIdempotency(t.Context(), scopeOf(), key, r.clk.Now())
	if err != nil || !found || !rec.Completed {
		t.Fatalf("record = %+v, %v, %v; want a completed record", rec, found, err)
	}
	if len(rec.Response.Body) != 0 || !rec.Response.SecretIssued {
		t.Errorf("the stored record holds a body %q (secret issued %v); it must hold none", rec.Response.Body, rec.Response.SecretIssued)
	}
	again := r.post("key-1", `{}`)
	if again.Code != http.StatusConflict || problemCode(t, again) != "secret-already-issued" {
		t.Fatalf("repeat = %d %s, want 409 secret-already-issued", again.Code, again.Body.String())
	}
	if strings.Contains(again.Body.String(), secret) {
		t.Error("the repeat leaks the secret")
	}
	if got := again.Header().Get("Location"); got != "/api/v1/api-keys/created" {
		t.Errorf("the repeat points at %q, want the resource", got)
	}
	if n := r.runs.Load(); n != 1 {
		t.Errorf("the operation ran %d times, want 1", n)
	}
}

// TestIdempotencyCrashAfterCommit (SPEC-005 Amendment 8 section 8-2 item 5): the process dies
// after the change committed and before the response is written. The record stays in progress and
// names the resource. A retry while the first request may still be alive answers 409; a retry after
// the record is stale answers the resource that was created, and the operation does not run again.
func TestIdempotencyCrashAfterCommit(t *testing.T) {
	r := newRig(t, func(c *idempotency.Config, _ *rig) {
		c.Rebuild = func(_ context.Context, rec identity.IdempotencyRecord) (identity.IdempotencyResponse, error) {
			if rec.ResourceRef != "things/created" {
				return identity.IdempotencyResponse{}, idempotency.ErrNoRebuild
			}
			return identity.IdempotencyResponse{Status: http.StatusCreated, ContentType: "application/json", Location: "/api/v1/things/created", Body: []byte(`{"run":1}`)}, nil
		}
	})
	// The first request commits (claims its record with the resource) and the process is killed
	// before the response: nothing after the change runs, as in a crash.
	r.handler = func(_ http.ResponseWriter, req *http.Request) {
		r.runs.Add(1)
		if err := r.store.Claim(req.Context(), scopeOf(), "things/created"); err != nil {
			t.Errorf("claim: %v", err)
		}
		panic(http.ErrAbortHandler)
	}
	func() {
		defer func() {
			recovered := recover()
			if err, _ := recovered.(error); !errors.Is(err, http.ErrAbortHandler) {
				t.Fatalf("recovered %v, want the abort of the first request", recovered)
			}
		}()
		r.post("key-1", `{"name":"a"}`)
	}()
	key := identity.IdempotencyKey{Requester: string(keyA.CredentialType) + ":" + keyA.CredentialID, Value: "key-1"}
	rec, found, err := r.store.LookupIdempotency(t.Context(), scopeOf(), key, r.clk.Now())
	if err != nil || !found || rec.Completed || rec.ResourceRef != "things/created" {
		t.Fatalf("record after the crash = %+v, %v, %v; want in progress and naming the resource", rec, found, err)
	}

	// The retry comes back at once: the first request may still be running.
	r.handler = func(http.ResponseWriter, *http.Request) { t.Error("the operation ran again") }
	if soon := r.post("key-1", `{"name":"a"}`); soon.Code != http.StatusConflict || problemCode(t, soon) != "idempotency-in-progress" {
		t.Fatalf("early retry = %d %s, want 409 idempotency-in-progress", soon.Code, soon.Body.String())
	}
	// The retry after the record is stale gets the resource of the first request.
	r.clk.Advance(40 * time.Second)
	late := r.post("key-1", `{"name":"a"}`)
	if late.Code != http.StatusCreated || late.Body.String() != `{"run":1}` || late.Header().Get("Location") != "/api/v1/things/created" {
		t.Fatalf("late retry = %d %s %q, want the created resource", late.Code, late.Body.String(), late.Header().Get("Location"))
	}
	if late.Header().Get(idempotency.ReplayedHeader) != "true" {
		t.Errorf("the answer is not marked as replayed")
	}
	if n := r.runs.Load(); n != 1 {
		t.Errorf("the operation ran %d times, want 1", n)
	}
}

func TestStaleRecordIsRebuiltFromItsResource(t *testing.T) {
	r := newRig(t, func(c *idempotency.Config, _ *rig) {
		c.Rebuild = func(_ context.Context, rec identity.IdempotencyRecord) (identity.IdempotencyResponse, error) {
			if rec.ResourceRef != "things/created" {
				return identity.IdempotencyResponse{}, idempotency.ErrNoRebuild
			}
			return identity.IdempotencyResponse{Status: http.StatusCreated, ContentType: "application/json", Location: "/api/v1/things/created", Body: []byte(`{"rebuilt":true}`)}, nil
		}
	})
	r.preclaim("key-1", keyA, `{}`)
	r.clk.Advance(40 * time.Second)
	rec := r.post("key-1", `{}`)
	if rec.Code != http.StatusCreated || rec.Body.String() != `{"rebuilt":true}` {
		t.Fatalf("stale record = %d %s, want the rebuilt 201", rec.Code, rec.Body.String())
	}
	if rec.Header().Get(idempotency.ReplayedHeader) != "true" {
		t.Errorf("the rebuilt answer is not marked as replayed")
	}
	if again := r.post("key-1", `{}`); again.Body.String() != `{"rebuilt":true}` {
		t.Errorf("the rebuilt record was not completed: %s", again.Body.String())
	}
}

func TestStaleRecordWithoutRebuildConflicts(t *testing.T) {
	r := newRig(t, nil)
	r.preclaim("key-1", keyA, `{}`)
	r.clk.Advance(40 * time.Second)
	if rec := r.post("key-1", `{}`); rec.Code != http.StatusConflict {
		t.Fatalf("stale record without a rebuild = %d, want 409", rec.Code)
	}
}

func TestOtherRequesterDoesNotShareTheRecord(t *testing.T) {
	r := newRig(t, nil)
	r.post("key-1", `{}`)
	r.who = keyB
	if rec := r.post("key-1", `{}`); rec.Header().Get(idempotency.ReplayedHeader) == "true" {
		t.Fatalf("another key of the same user replayed the first key's response")
	}
	if n := r.runs.Load(); n != 2 {
		t.Errorf("the operation ran %d times, want 2", n)
	}
}

func TestRequiredKeyIsEnforced(t *testing.T) {
	r := newRig(t, func(c *idempotency.Config, _ *rig) { c.Required = true })
	rec := r.post("", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing key = %d, want 400", rec.Code)
	}
}

func TestInvalidKeyIsRefused(t *testing.T) {
	r := newRig(t, nil)
	if rec := r.post("has space", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("a key with a space = %d, want 400", rec.Code)
	}
}

func TestUnavailableStoreRefusesKeyedRequests(t *testing.T) {
	r := newRig(t, func(c *idempotency.Config, _ *rig) { c.Store = idempotency.UnavailableStore{} })
	if rec := r.post("key-1", `{}`); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("keyed request without a store = %d, want 503", rec.Code)
	}
	if r.runs.Load() != 0 {
		t.Errorf("the operation ran without its idempotency store")
	}
}

func TestRequestWithoutKeyRunsWithoutRecord(t *testing.T) {
	r := newRig(t, nil)
	r.post("", `{}`)
	r.post("", `{}`)
	if n := r.runs.Load(); n != 2 {
		t.Errorf("unkeyed requests ran %d times, want 2", n)
	}
}
