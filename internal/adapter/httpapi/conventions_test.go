// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/conditional"
	"github.com/wirvii/orcon/internal/adapter/httpapi/idempotency"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/platform/config"
)

// TestIdempotencyInThePipeline: the idempotency step is installed on the
// operations that take a key, after authorization, with the key required where
// the contract says so and the secret kept out of the record where the
// operation returns one.
func TestIdempotencyInThePipeline(t *testing.T) {
	h := newHarness(t, nil)
	post := func(key, body string) *httptest.ResponseRecorder {
		return h.do(http.MethodPost, "/api/v1/secrets", "token-a", func(r *http.Request) {
			r.Body = io.NopCloser(strings.NewReader(body))
			r.ContentLength = int64(len(body))
			if key != "" {
				r.Header.Set("Idempotency-Key", key)
			}
		})
	}
	if rec := post("", `{"name":"ci"}`); rec.Code != http.StatusBadRequest || problemOf(t, rec)["code"] != "validation-failed" {
		t.Errorf("a missing required key = %d %s", rec.Code, rec.Body.String())
	}
	first := post("k1", `{"name":"ci"}`)
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), "example-secret-1") {
		t.Fatalf("first = %d %s", first.Code, first.Body.String())
	}
	replay := post("k1", `{"name":"ci"}`)
	if replay.Code != http.StatusConflict || problemOf(t, replay)["code"] != "secret-already-issued" {
		t.Errorf("replay = %d %s, want 409 secret-already-issued", replay.Code, replay.Body.String())
	}
	if strings.Contains(replay.Body.String(), "example-secret-1") {
		t.Error("the replay leaks the secret")
	}
	if rec := post("k1", `{"name":"other"}`); rec.Code != http.StatusUnprocessableEntity || problemOf(t, rec)["code"] != "idempotency-key-reused" {
		t.Errorf("a different body under the same key = %d %s", rec.Code, rec.Body.String())
	}

	// An optional key on an ordinary operation replays the stored response.
	create := func() *httptest.ResponseRecorder {
		return h.do(http.MethodPost, "/api/v1/things", "token-a", func(r *http.Request) { r.Header.Set("Idempotency-Key", "k2") })
	}
	one, two := create(), create()
	if one.Code != http.StatusOK || two.Code != http.StatusOK || two.Header().Get("Idempotent-Replayed") != "true" || one.Body.String() != two.Body.String() {
		t.Errorf("optional key: %d/%d replayed %q", one.Code, two.Code, two.Header().Get("Idempotent-Replayed"))
	}
	// The step runs after authorization: a forbidden caller never reaches it.
	h.decide = func(authz.Request) authz.Decision { return authz.DecisionForbidden }
	if rec := post("k3", `{}`); rec.Code != http.StatusForbidden {
		t.Errorf("a forbidden caller = %d, want 403", rec.Code)
	}
}

// TestMissingIfMatchIs428: a change without If-Match is 428 precondition-required,
// not a generic 400, even though the contract declares the header required.
func TestMissingIfMatchIs428(t *testing.T) {
	rt, err := New(&Deps{
		Role:          config.RoleAPI,
		Idempotency:   memStore(),
		Authenticator: fakeAuthn{"token-a": userA},
		Authorizer:    authzFunc(func(authz.Request) authz.Decision { return authz.DecisionAllow }),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	spec := rt.Spec()
	checked := 0
	for i := range spec.Operations {
		op := &spec.Operations[i]
		if !op.IfMatchRequired {
			continue
		}
		req := firstExampleRequest(t, spec, op)
		req.Header.Del("If-Match")
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, req)
		if rec.Code != http.StatusPreconditionRequired || problemOf(t, rec)["code"] != "precondition-required" {
			t.Errorf("%s without If-Match = %d %s, want 428", op.ID, rec.Code, rec.Body.String())
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("the contract has no operation with a required If-Match")
	}
}

// TestKnownErrorsAreMapped: the errors every handler shares become their problem.
func TestKnownErrorsAreMapped(t *testing.T) {
	h := errorHandler{logger: slog.New(slog.DiscardHandler)}
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{errors.Join(errors.New("list flows"), page.ErrInvalidToken), 400, "invalid-page-token"},
		{page.ErrInvalidLimit, 400, "validation-failed"},
		{page.ErrInvalidSort, 400, "validation-failed"},
		{conditional.ErrRequired, 428, "precondition-required"},
		{conditional.ErrMismatch, 412, "precondition-failed"},
		{conditional.ErrMalformed, 412, "precondition-failed"},
		{errUnavailable, 503, "service-unavailable"},
		{errors.New("database down"), 500, "internal-error"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.response(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/flows", nil), tt.err)
		if rec.Code != tt.status {
			t.Errorf("%v = %d, want %d", tt.err, rec.Code, tt.status)
			continue
		}
		if code := problemOf(t, rec)["code"]; code != tt.code {
			t.Errorf("%v code = %v, want %s", tt.err, code, tt.code)
		}
	}
	// A bad request body from the generated binding is validation-failed; a body over the limit is 413.
	rec := httptest.NewRecorder()
	h.request(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/flows", nil), &http.MaxBytesError{Limit: 1})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("a body over the limit = %d, want 413", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.request(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/flows", nil), errors.New("invalid character"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("an unreadable body = %d, want 400", rec.Code)
	}
}

// TestIdempotencyStoreCanBeReplaced: Deps.Idempotency is the seam for the durable store.
func TestIdempotencyStoreCanBeReplaced(t *testing.T) {
	calls := 0
	store := &countingStore{onBegin: func() { calls++ }}
	h := newHarness(t, func(d *Deps) { d.Idempotency = store })
	h.do(http.MethodPost, "/api/v1/things", "token-a", func(r *http.Request) { r.Header.Set("Idempotency-Key", "k") })
	if calls != 1 {
		t.Errorf("the replaced store was asked %d times, want 1", calls)
	}
}

// countingStore claims every key and counts the claims.
type countingStore struct {
	onBegin func()
}

func (c *countingStore) Begin(context.Context, idempotency.Key, [sha256.Size]byte) (idempotency.Record, bool, error) {
	c.onBegin()
	return idempotency.Record{}, true, nil
}

func (*countingStore) Complete(context.Context, idempotency.Key, *idempotency.Response) error {
	return nil
}

func (*countingStore) Abandon(context.Context, idempotency.Key) error { return nil }
