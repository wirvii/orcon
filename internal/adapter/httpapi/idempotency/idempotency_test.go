// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package idempotency_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/idempotency"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

const route = "POST /things/{thingId}"

func principal(n byte, credential string, kind identity.CredentialType) identity.Principal {
	var u id.UUID
	u[15] = n
	var org id.UUID
	org[15] = 99
	return identity.Principal{ID: identity.PrincipalID(u), OrgID: identity.OrgID(org), CredentialType: kind, CredentialID: credential}
}

var (
	keyA    = principal(1, "key-a", identity.CredentialPersonalKey)
	keyB    = principal(1, "key-b", identity.CredentialPersonalKey)
	session = principal(1, "session-1", identity.CredentialSession)
)

// rig is a mux with one idempotent route and a handler that counts its runs.
type rig struct {
	t       *testing.T
	clk     *clock.Fake
	store   *idempotency.MemoryStore
	runs    atomic.Int32
	handler http.HandlerFunc
	who     identity.Principal
	mux     *http.ServeMux
}

func newRig(t *testing.T, cfg func(*idempotency.Config)) *rig {
	t.Helper()
	r := &rig{t: t, clk: clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)), who: keyA}
	r.store = idempotency.NewMemoryStore(r.clk)
	r.handler = func(w http.ResponseWriter, _ *http.Request) {
		n := r.runs.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/api/v1/things/created")
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"run":%d}`, n)
	}
	c := idempotency.Config{Store: r.store, Logger: slog.New(slog.DiscardHandler), Route: route}
	if cfg != nil {
		cfg(&c)
	}
	idem := idempotency.Middleware(c)
	r.mux = http.NewServeMux()
	r.mux.Handle(route, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req = req.WithContext(middleware.WithPrincipal(req.Context(), r.who))
		idem(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { r.handler(w, req) })).ServeHTTP(w, req)
	}))
	return r
}

func (r *rig) post(path, key, body string) *httptest.ResponseRecorder {
	r.t.Helper()
	req := httptest.NewRequestWithContext(r.t.Context(), http.MethodPost, path, strings.NewReader(body))
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	rec.Header().Set("Orcon-Request-Id", "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a62")
	r.mux.ServeHTTP(rec, req)
	return rec
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
	first := r.post("/things/1", "k1", `{"a":1,"b":2}`)
	if first.Code != http.StatusCreated || first.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("first = %d replayed %q", first.Code, first.Header().Get("Idempotent-Replayed"))
	}
	// The same content written differently has the same fingerprint (canonical JSON).
	second := r.post("/things/1", "k1", "{ \"b\": 2,\n \"a\": 1 }")
	if second.Code != http.StatusCreated {
		t.Fatalf("second = %d (%s)", second.Code, second.Body.String())
	}
	if second.Header().Get("Idempotent-Replayed") != "true" {
		t.Error("the replay is not marked Idempotent-Replayed: true")
	}
	if second.Body.String() != first.Body.String() || second.Header().Get("Location") != first.Header().Get("Location") {
		t.Errorf("the replay differs: %q vs %q", second.Body.String(), first.Body.String())
	}
	if got := r.runs.Load(); got != 1 {
		t.Errorf("the handler ran %d times, want 1", got)
	}
}

func TestIdempotencyFingerprintMismatch(t *testing.T) {
	r := newRig(t, nil)
	r.post("/things/1", "k1", `{"a":1}`)
	for name, rec := range map[string]*httptest.ResponseRecorder{
		"other body":       r.post("/things/1", "k1", `{"a":2}`),
		"other path value": r.post("/things/2", "k1", `{"a":1}`),
	} {
		if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "idempotency-key-reused" {
			t.Errorf("%s: %d %s, want 422 idempotency-key-reused", name, rec.Code, rec.Body.String())
		}
	}
	if got := r.runs.Load(); got != 1 {
		t.Errorf("the handler ran %d times, want 1", got)
	}
}

func TestIdempotencyInProgress(t *testing.T) {
	r := newRig(t, nil)
	entered := make(chan struct{})
	release := make(chan struct{})
	r.handler = func(w http.ResponseWriter, _ *http.Request) {
		r.runs.Add(1)
		close(entered)
		<-release
		w.WriteHeader(http.StatusCreated)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var first *httptest.ResponseRecorder
	go func() {
		defer wg.Done()
		first = r.post("/things/1", "k1", `{}`)
	}()
	<-entered
	twin := r.post("/things/1", "k1", `{}`)
	if twin.Code != http.StatusConflict || problemCode(t, twin) != "idempotency-in-progress" {
		t.Errorf("twin = %d %s, want 409 idempotency-in-progress", twin.Code, twin.Body.String())
	}
	if twin.Header().Get("Retry-After") == "" {
		t.Error("the in-progress answer has no Retry-After")
	}
	close(release)
	wg.Wait()
	if first.Code != http.StatusCreated {
		t.Errorf("first = %d", first.Code)
	}
	if got := r.runs.Load(); got != 1 {
		t.Errorf("the handler ran %d times, want 1", got)
	}
}

func TestIdempotencySecretNotStored(t *testing.T) {
	r := newRig(t, func(c *idempotency.Config) { c.IssuesSecret = true })
	r.handler = func(w http.ResponseWriter, _ *http.Request) {
		r.runs.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/api/v1/api-keys/0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a74")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a74","secret":"example-secret-1"}`))
	}
	first := r.post("/things/1", "k1", `{}`)
	if first.Code != http.StatusCreated || !strings.Contains(first.Body.String(), "example-secret-1") {
		t.Fatalf("first = %d %s: the creator must see the secret once", first.Code, first.Body.String())
	}
	key := idempotency.Key{Org: keyA.OrgID.String(), Requester: "PAK:key-a", Value: "k1"}
	rec, ok := r.store.Peek(key)
	if !ok || !rec.Completed {
		t.Fatalf("no completed record: %+v", rec)
	}
	if len(rec.Response.Body) != 0 || strings.Contains(string(rec.Response.Body), "secret") {
		t.Errorf("the stored record holds a body: %q", rec.Response.Body)
	}
	replay := r.post("/things/1", "k1", `{}`)
	if replay.Code != http.StatusConflict || problemCode(t, replay) != "secret-already-issued" {
		t.Errorf("replay = %d %s, want 409 secret-already-issued", replay.Code, replay.Body.String())
	}
	if strings.Contains(replay.Body.String(), "example-secret-1") {
		t.Error("the replay leaks the secret")
	}
	if replay.Header().Get("Location") != "/api/v1/api-keys/0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a74" {
		t.Errorf("the replay does not point at the resource: %q", replay.Header().Get("Location"))
	}
}

func TestIdempotencyServerErrorsAreNotStored(t *testing.T) {
	r := newRig(t, nil)
	fail := true
	r.handler = func(w http.ResponseWriter, _ *http.Request) {
		r.runs.Add(1)
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}
	if rec := r.post("/things/1", "k1", `{}`); rec.Code != http.StatusInternalServerError {
		t.Fatalf("first = %d", rec.Code)
	}
	fail = false
	if rec := r.post("/things/1", "k1", `{}`); rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "" {
		t.Errorf("the retry after a 500 = %d replayed %q, want a fresh 201", rec.Code, rec.Header().Get("Idempotent-Replayed"))
	}
	if got := r.runs.Load(); got != 2 {
		t.Errorf("handler runs = %d, want 2", got)
	}
}

// TestIdempotencyClientErrorsReleaseTheKey: only a success is stored (Amendment 8
// section 8-2(c)), so a 4xx lets the client correct the request under the same key.
func TestIdempotencyClientErrorsReleaseTheKey(t *testing.T) {
	r := newRig(t, nil)
	r.handler = func(w http.ResponseWriter, _ *http.Request) {
		r.runs.Add(1)
		w.WriteHeader(http.StatusConflict)
	}
	r.post("/things/1", "k1", `{}`)
	rec := r.post("/things/1", "k1", `{}`)
	if rec.Header().Get("Idempotent-Replayed") != "" {
		t.Errorf("a 409 was replayed, want it released")
	}
	if got := r.runs.Load(); got != 2 {
		t.Errorf("handler runs = %d, want 2", got)
	}
}

// TestIdempotencyOversizedSuccessIsRefusedOnRetry: a success too large to store is
// not run again; the retry gets 409 with the address of the resource.
func TestIdempotencyOversizedSuccessIsRefusedOnRetry(t *testing.T) {
	r := newRig(t, nil)
	big := strings.Repeat("x", 1<<20+10)
	r.handler = func(w http.ResponseWriter, _ *http.Request) {
		r.runs.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/api/v1/things/created")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"v":"` + big + `"}`))
	}
	first := r.post("/things/1", "k1", `{}`)
	if first.Code != http.StatusCreated || first.Body.Len() < len(big) {
		t.Fatalf("first answer = %d with %d bytes, want the full 201", first.Code, first.Body.Len())
	}
	retry := r.post("/things/1", "k1", `{}`)
	if retry.Code != http.StatusConflict || problemCode(t, retry) != "conflict" {
		t.Errorf("retry = %d %s, want 409 conflict", retry.Code, retry.Body.String())
	}
	if retry.Header().Get("Location") != "/api/v1/things/created" {
		t.Errorf("retry Location = %q", retry.Header().Get("Location"))
	}
	if got := r.runs.Load(); got != 1 {
		t.Errorf("handler runs = %d, want 1", got)
	}
}

// TestUnavailableStoreAnswers503: without a durable store the request does not run.
func TestUnavailableStoreAnswers503(t *testing.T) {
	r := newRig(t, func(c *idempotency.Config) { c.Store = idempotency.UnavailableStore{} })
	rec := r.post("/things/1", "k1", `{}`)
	if rec.Code != http.StatusServiceUnavailable || problemCode(t, rec) != "service-unavailable" {
		t.Errorf("got %d %s, want 503 service-unavailable", rec.Code, rec.Body.String())
	}
	if got := r.runs.Load(); got != 0 {
		t.Errorf("handler runs = %d, want 0", got)
	}
}

func TestIdempotencyPanicReleasesTheKey(t *testing.T) {
	r := newRig(t, nil)
	crash := true
	r.handler = func(w http.ResponseWriter, _ *http.Request) {
		r.runs.Add(1)
		if crash {
			panic("boom")
		}
		w.WriteHeader(http.StatusCreated)
	}
	func() {
		defer func() { _ = recover() }()
		r.post("/things/1", "k1", `{}`)
	}()
	crash = false
	if rec := r.post("/things/1", "k1", `{}`); rec.Code != http.StatusCreated {
		t.Errorf("the retry after a crash = %d, want 201 (the key must be free)", rec.Code)
	}
}

func TestIdempotencyScopesByRequesterAndOrganization(t *testing.T) {
	r := newRig(t, nil)
	r.post("/things/1", "k1", `{}`)
	r.who = keyB
	if rec := r.post("/things/1", "k1", `{}`); rec.Header().Get("Idempotent-Replayed") != "" {
		t.Error("two keys of one user share a record")
	}
	r.who = session
	if rec := r.post("/things/1", "k1", `{}`); rec.Header().Get("Idempotent-Replayed") != "" {
		t.Error("a session shares the record of a key")
	}
	other := principal(2, "key-a", identity.CredentialPersonalKey)
	other.OrgID = identity.OrgID(id.UUID{15: 7})
	r.who = other
	if rec := r.post("/things/1", "k1", `{}`); rec.Header().Get("Idempotent-Replayed") != "" {
		t.Error("another organization shares the record")
	}
	if got := r.runs.Load(); got != 4 {
		t.Errorf("handler runs = %d, want 4", got)
	}
}

func TestIdempotencyKeyValidation(t *testing.T) {
	required := newRig(t, func(c *idempotency.Config) { c.Required = true })
	if rec := required.post("/things/1", "", `{}`); rec.Code != http.StatusBadRequest || problemCode(t, rec) != "validation-failed" {
		t.Errorf("missing required key = %d %s", rec.Code, rec.Body.String())
	}
	optional := newRig(t, nil)
	if rec := optional.post("/things/1", "", `{}`); rec.Code != http.StatusCreated {
		t.Errorf("missing optional key = %d, want the handler to run", rec.Code)
	}
	for name, key := range map[string]string{"space": "has space", "non ASCII": "clé", "too long": strings.Repeat("k", 256), "control": "a\x01b"} {
		if rec := optional.post("/things/1", key, `{}`); rec.Code != http.StatusBadRequest {
			t.Errorf("%s key = %d, want 400", name, rec.Code)
		}
	}
	if rec := optional.post("/things/1", strings.Repeat("k", 255), `{}`); rec.Code != http.StatusCreated {
		t.Errorf("a key of 255 characters = %d, want 201", rec.Code)
	}
	if rec := optional.post("/things/1", "bad-body", `{`); rec.Code != http.StatusBadRequest {
		t.Errorf("a body that is not JSON = %d, want 400", rec.Code)
	}
}

func TestIdempotencyRecordsExpire(t *testing.T) {
	r := newRig(t, nil)
	r.post("/things/1", "k1", `{}`)
	r.clk.Advance(idempotency.TTL - time.Minute)
	if rec := r.post("/things/1", "k1", `{}`); rec.Header().Get("Idempotent-Replayed") != "true" {
		t.Error("the record expired early")
	}
	r.clk.Advance(2 * time.Minute)
	if rec := r.post("/things/1", "k1", `{}`); rec.Header().Get("Idempotent-Replayed") != "" {
		t.Error("the record outlived its 24 hours")
	}
}

func TestFingerprintIgnoresWhitespaceAndMemberOrder(t *testing.T) {
	a, err := idempotency.Fingerprint("POST /x", []string{"id=1"}, []byte(`{"a":1,"b":[1,2]}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := idempotency.Fingerprint("POST /x", []string{"id=1"}, []byte("{\n \"b\": [1, 2],\n \"a\": 1\n}"))
	if a != b {
		t.Error("equivalent bodies have different fingerprints")
	}
	for name, other := range map[string][sha256.Size]byte{
		"route":   fp(t, "POST /y", []string{"id=1"}, `{"a":1,"b":[1,2]}`),
		"path":    fp(t, "POST /x", []string{"id=2"}, `{"a":1,"b":[1,2]}`),
		"body":    fp(t, "POST /x", []string{"id=1"}, `{"a":1,"b":[2,1]}`),
		"no body": fp(t, "POST /x", []string{"id=1"}, ``),
	} {
		if other == a {
			t.Errorf("a different %s has the same fingerprint", name)
		}
	}
	if _, err := idempotency.Fingerprint("POST /x", nil, []byte(`{`)); err == nil {
		t.Error("a body that is not JSON was fingerprinted")
	}
}

func fp(t *testing.T, route string, params []string, body string) [sha256.Size]byte {
	t.Helper()
	out, err := idempotency.Fingerprint(route, params, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// failingStore fails the step named in fail.
type failingStore struct {
	idempotency.Store
	fail string
}

var errStore = errors.New("store down")

func (f failingStore) Begin(ctx context.Context, k idempotency.Key, fpr [sha256.Size]byte) (idempotency.Record, bool, error) {
	if f.fail == "begin" {
		return idempotency.Record{}, false, errStore
	}
	return f.Store.Begin(ctx, k, fpr)
}

func (f failingStore) Complete(ctx context.Context, k idempotency.Key, resp *idempotency.Response) error {
	if f.fail == "complete" {
		return errStore
	}
	return f.Store.Complete(ctx, k, resp)
}

func TestIdempotencyStoreFailures(t *testing.T) {
	for _, step := range []string{"begin", "complete"} {
		r := newRig(t, nil)
		r.store = idempotency.NewMemoryStore(r.clk)
		failing := failingStore{Store: r.store, fail: step}
		idem := idempotency.Middleware(idempotency.Config{Store: failing, Logger: slog.New(slog.DiscardHandler), Route: route})
		mux := http.NewServeMux()
		runs := 0
		mux.Handle(route, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req = req.WithContext(middleware.WithPrincipal(req.Context(), keyA))
			idem(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { runs++; w.WriteHeader(http.StatusCreated) })).ServeHTTP(w, req)
		}))
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/things/1", strings.NewReader(`{}`))
		req.Header.Set("Idempotency-Key", "k1")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		switch step {
		case "begin":
			if rec.Code != http.StatusInternalServerError || runs != 0 {
				t.Errorf("begin failure: %d, handler runs %d; want 500 and no run", rec.Code, runs)
			}
		case "complete":
			if rec.Code != http.StatusCreated || runs != 1 {
				t.Errorf("complete failure: %d, handler runs %d; the client still gets the answer", rec.Code, runs)
			}
		}
	}
}

func TestMemoryStoreLimits(t *testing.T) {
	clk := clock.NewFake(time.Now())
	s := idempotency.NewMemoryStore(clk)
	if err := s.Complete(t.Context(), idempotency.Key{Value: "nope"}, &idempotency.Response{}); !errors.Is(err, idempotency.ErrNotClaimed) {
		t.Errorf("completing an unclaimed key: %v", err)
	}
	if err := s.Abandon(t.Context(), idempotency.Key{Value: "nope"}); err != nil {
		t.Errorf("abandoning an unclaimed key: %v", err)
	}
	if _, ok := s.Peek(idempotency.Key{Value: "nope"}); ok {
		t.Error("Peek found a key that was never claimed")
	}
	_ = io.Discard
}
