// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
)

// syntheticSpec is a small contract that exercises every branch of the
// pipeline. The generated server has no operations yet, so the tests register
// fake handlers for these through build.
const syntheticSpec = `{
  "openapi": "3.1.0",
  "info": {"title": "synthetic", "version": "0.0.0"},
  "servers": [{"url": "/api/v1"}],
  "paths": {
    "/things/{thingId}": {
      "parameters": [{"$ref": "#/components/parameters/ThingId"}],
      "get": {"operationId": "getThing", "x-orcon-permission": "flow:read", "x-orcon-rate-class": "read", "x-orcon-stability": "stable"},
      "put": {"operationId": "updateThing", "x-orcon-permission": {"allOf": ["flow:read", "flow:update"]}, "x-orcon-rate-class": "write", "x-orcon-stability": "stable",
        "parameters": [{"$ref": "#/components/parameters/IfMatch"}]}
    },
    "/things": {
      "post": {"operationId": "createThing", "x-orcon-permission": {"anyOf": ["flow:create", "flow:update"]}, "x-orcon-rate-class": "write", "x-orcon-stability": "stable",
        "parameters": [{"$ref": "#/components/parameters/IdempotencyKey"}]}
    },
    "/things/stream": {
      "get": {"operationId": "streamThings", "x-orcon-permission": "flow:read", "x-orcon-rate-class": "stream", "x-orcon-stability": "stable"}
    },
    "/ping": {
      "get": {"operationId": "ping", "x-orcon-permission": "anonymous", "x-orcon-rate-class": "read", "x-orcon-stability": "stable"}
    },
    "/me": {
      "get": {"operationId": "getMe", "x-orcon-permission": "authenticated", "x-orcon-rate-class": "read", "x-orcon-stability": "stable"}
    },
    "/preview": {
      "get": {"operationId": "getPreview", "x-orcon-permission": "authenticated", "x-orcon-rate-class": "read", "x-orcon-stability": "experimental"}
    },
    "/secrets": {
      "post": {"operationId": "createSecret", "x-orcon-permission": "flow:create", "x-orcon-rate-class": "write", "x-orcon-stability": "stable",
        "parameters": [{"$ref": "#/components/parameters/IdempotencyKeyRequired"}],
        "responses": {"201": {"description": "created", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ThingWithSecret"}}}}}}
    },
    "/boom": {
      "get": {"operationId": "boom", "x-orcon-permission": "authenticated", "x-orcon-rate-class": "read", "x-orcon-stability": "stable"}
    },
    "/oauth/token": {
      "post": {"operationId": "token", "tags": ["protocol"], "security": [], "x-orcon-permission": "anonymous", "x-orcon-rate-class": "write", "x-orcon-stability": "stable"}
    }
  },
  "components": {
    "parameters": {
      "ThingId": {"name": "thingId", "in": "path", "required": true, "schema": {"type": "string"}},
      "IfMatch": {"name": "If-Match", "in": "header", "required": true, "schema": {"type": "string"}},
      "IdempotencyKey": {"name": "Idempotency-Key", "in": "header", "required": false, "schema": {"type": "string"}},
      "IdempotencyKeyRequired": {"name": "Idempotency-Key", "in": "header", "required": true, "schema": {"type": "string"}}
    },
    "schemas": {"ThingWithSecret": {"type": "object"}}
  }
}`

// harness is a router over the synthetic contract with fakes at its seams.
type harness struct {
	t      *testing.T
	clk    *clock.Fake
	router *Router
	decide func(authz.Request) authz.Decision
	// onStream, when set, runs inside the stream handler.
	onStream func()
}

var (
	userA = identity.Principal{ID: pid(1), Kind: identity.PrincipalUser, CredentialType: identity.CredentialPersonalKey, CredentialID: "key-a"}
	userB = identity.Principal{ID: pid(2), Kind: identity.PrincipalUser, CredentialType: identity.CredentialPersonalKey, CredentialID: "key-b"}
	web   = identity.Principal{ID: pid(3), Kind: identity.PrincipalUser, CredentialType: identity.CredentialSession, CredentialID: "session-c"}
)

func pid(n byte) identity.PrincipalID {
	var u id.UUID
	u[15] = n
	return identity.PrincipalID(u)
}

// fakeAuthn authenticates by bearer token.
type fakeAuthn map[string]identity.Principal

func (f fakeAuthn) Authenticate(_ context.Context, c identity.Credentials) (identity.Principal, error) {
	token := c.Bearer
	if token == "" {
		token = c.Session
	}
	if p, ok := f[token]; ok {
		return p, nil
	}
	return identity.Principal{}, identity.ErrUnauthenticated
}

type authzFunc func(authz.Request) authz.Decision

func (f authzFunc) Authorize(_ context.Context, r authz.Request) (authz.Decision, error) {
	return f(r), nil
}

// fakeHandlers registers the fake handler of each synthetic operation.
func fakeHandlers(mux apigen.ServeMux, base string, onStream func()) {
	json200 := func(body any) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(body)
		}
	}
	mux.HandleFunc("GET "+base+"/things/{thingId}", func(w http.ResponseWriter, r *http.Request) {
		json200(map[string]string{"id": r.PathValue("thingId")})(w, r)
	})
	mux.HandleFunc("PUT "+base+"/things/{thingId}", json200(map[string]string{"updated": "yes"}))
	mux.HandleFunc("POST "+base+"/things", json200(map[string]string{"created": "yes"}))
	mux.HandleFunc("GET "+base+"/things/stream", func(w http.ResponseWriter, r *http.Request) {
		if onStream != nil {
			onStream()
		}
		json200(map[string]string{"stream": "yes"})(w, r)
	})
	mux.HandleFunc("GET "+base+"/ping", json200(map[string]string{"pong": "yes"}))
	mux.HandleFunc("GET "+base+"/me", json200(map[string]string{"me": "yes"}))
	mux.HandleFunc("GET "+base+"/preview", json200(map[string]string{"preview": "yes"}))
	mux.HandleFunc("POST "+base+"/secrets", json200(map[string]string{"secret": "example-secret-1"}))
	mux.HandleFunc("GET "+base+"/boom", func(http.ResponseWriter, *http.Request) { panic("kaboom with secret-value") })
	mux.HandleFunc("POST "+base+"/oauth/token", json200(map[string]string{"access_token": "x"}))
}

// newHarness builds the router with the given limits and authorization.
func newHarness(t *testing.T, mutate func(*Deps)) *harness {
	t.Helper()
	h := &harness{t: t, clk: clock.NewFake(time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC))}
	h.decide = func(authz.Request) authz.Decision { return authz.DecisionAllow }
	deps := &Deps{
		Role:  config.RoleAPI,
		Clock: h.clk,
		IDs:   id.NewSequence(),
		Authenticator: fakeAuthn{
			"token-a": userA, "token-b": userB, "cookie-c": web,
		},
		Authorizer: authzFunc(func(r authz.Request) authz.Decision { return h.decide(r) }),
	}
	if mutate != nil {
		mutate(deps)
	}
	rt, err := build(deps, []byte(syntheticSpec), func(mux apigen.ServeMux, base string) {
		fakeHandlers(mux, base, func() {
			if h.onStream != nil {
				h.onStream()
			}
		})
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	h.router = rt
	return h
}

// do sends a request and returns the recorded response.
func (h *harness) do(method, path, token string, mutate ...func(*http.Request)) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequestWithContext(h.t.Context(), method, path, strings.NewReader(""))
	req.RemoteAddr = "192.0.2.10:4000"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, m := range mutate {
		m(req)
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// problemOf decodes an application/problem+json answer and checks its media type.
func problemOf(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json (status %d, body %s)", ct, rec.Code, rec.Body.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("problem body is not JSON: %v", err)
	}
	return doc
}
