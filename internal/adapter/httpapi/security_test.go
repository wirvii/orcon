// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/platform/config"
)

// TestSecurityEmptyDoesNotOpenAnOperation (BL-088): an empty security list
// together with a permission, a duplicated or mixed permission declaration, and
// the anonymous permission outside the closed list are refused when the
// contract is read, so no such contract is ever served.
func TestSecurityEmptyDoesNotOpenAnOperation(t *testing.T) {
	cases := map[string]struct {
		mutate func(string) string
		want   string
	}{
		"security [] with a permission": {
			func(s string) string {
				return strings.Replace(s, `"get": {"operationId": "getThing",`, `"get": {"operationId": "getThing", "security": [],`, 1)
			},
			"security: [] is only for anonymous",
		},
		"anonymous outside the closed list": {
			func(s string) string {
				return strings.Replace(s, `"operationId": "getSetupState"`, `"operationId": "listEverything"`, 1)
			},
			"anonymous permission is for protocol operations",
		},
		"anonymous without security []": {
			func(s string) string {
				return strings.Replace(s, `"operationId": "getSetupState", "security": [],`, `"operationId": "getSetupState",`, 1)
			},
			"declares security: []",
		},
		"allOf and anyOf together": {
			func(s string) string {
				return strings.Replace(s, `"x-orcon-permission": "flow:read", "x-orcon-rate-class": "stream"`, `"x-orcon-permission": {"allOf": ["flow:read"], "anyOf": ["flow:update"]}, "x-orcon-rate-class": "stream"`, 1)
			},
			"exactly one of allOf and anyOf",
		},
		"permission object with another key": {
			func(s string) string {
				return strings.Replace(s, `"x-orcon-permission": "flow:read", "x-orcon-rate-class": "stream"`, `"x-orcon-permission": {"allOf": ["flow:read"], "oneOf": ["x"]}, "x-orcon-rate-class": "stream"`, 1)
			},
			"exactly one of allOf and anyOf",
		},
	}
	for name, tc := range cases {
		spec := tc.mutate(syntheticSpec)
		if spec == syntheticSpec {
			t.Fatalf("%s: the mutation changed nothing", name)
		}
		_, err := ParseSpec([]byte(spec))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to contain %q", name, err, tc.want)
		}
		if _, err := build(&Deps{Role: config.RoleAPI}, []byte(spec), func(apigen.ServeMux, string) {}); err == nil {
			t.Errorf("%s: a router was built for the contract", name)
		}
	}
}

// TestOnlyAnonymousOperationsArePublic: the closed list and the protocol
// operations are served without credentials; every other operation, even one
// whose route is registered, demands authentication first.
func TestOnlyAnonymousOperationsArePublic(t *testing.T) {
	spec, err := ParseSpec([]byte(syntheticSpec))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	for i := range spec.Operations {
		op := &spec.Operations[i]
		wantPublic := op.ID == "getSetupState" || op.ID == "token"
		if op.Public != wantPublic {
			t.Errorf("%s public = %v, want %v", op.ID, op.Public, wantPublic)
		}
	}
	h := newHarness(t, nil)
	h.decide = func(authz.Request) authz.Decision { return authz.DecisionAllow }
	for _, op := range spec.Operations {
		if op.Public {
			continue
		}
		path := strings.NewReplacer("{thingId}", "1").Replace("/api/v1" + op.Path)
		rec := h.do(op.Method, path, "")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without credentials = %d, want 401", op.Method, op.Path, rec.Code)
		}
	}
	if rec := h.do(http.MethodGet, "/api/v1/ping", ""); rec.Code != http.StatusOK {
		t.Errorf("the anonymous setup state = %d, want 200", rec.Code)
	}
}

func TestRequirementKeepsItsOrder(t *testing.T) {
	spec, err := ParseSpec([]byte(syntheticSpec))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	for i := range spec.Operations {
		op := &spec.Operations[i]
		switch op.ID {
		case "updateThing":
			if op.Requirement.Mode != authz.ModeAll || len(op.Requirement.Permissions) != 2 {
				t.Errorf("updateThing = %+v", op.Requirement)
			}
		case "createThing":
			if op.Requirement.Mode != authz.ModeAny {
				t.Errorf("createThing = %+v", op.Requirement)
			}
		}
	}
}

// bodyRequest sends a request with a body and a media type.
func (h *harness) bodyRequest(token, contentType, body string) *httptest.ResponseRecorder {
	h.t.Helper()
	const path = "/api/v1/things"
	return h.do(http.MethodPost, path, token, func(r *http.Request) {
		r.Body = http.NoBody
		if body != "" {
			r.Body = httptest.NewRequestWithContext(h.t.Context(), http.MethodPost, path, strings.NewReader(body)).Body
			r.ContentLength = int64(len(body))
		}
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
	})
}

// TestRequestBodiesFollowTheContract (BL-090).
func TestRequestBodiesFollowTheContract(t *testing.T) {
	h := newHarness(t, nil)
	tests := []struct {
		name        string
		contentType string
		body        string
		status      int
		code        string
		pointer     string
	}{
		{"valid", "application/json", `{"name":"ab","count":3}`, 200, "", ""},
		{"valid with a charset", "application/json; charset=utf-8", `{"name":"ab"}`, 200, "", ""},
		{"valid nullable", "application/json", `{"tag":null}`, 200, "", ""},
		{"empty optional body", "", ``, 200, "", ""},
		{"text/plain", "text/plain", `{"name":"ab"}`, 415, "unsupported-media-type", ""},
		{"no content type", "", `{"name":"ab"}`, 415, "unsupported-media-type", ""},
		{"merge patch on a route that does not declare it", "application/merge-patch+json", `{"name":"ab"}`, 415, "unsupported-media-type", ""},
		{"unknown field", "application/json", `{"surprise":1}`, 400, "validation-failed", "/surprise"},
		{"trailing value", "application/json", `{"name":"ab"}{"name":"cd"}`, 400, "validation-failed", ""},
		{"trailing garbage", "application/json", `{"name":"ab"} junk`, 400, "validation-failed", ""},
		{"not JSON", "application/json", `{`, 400, "validation-failed", ""},
		{"null for a non-nullable field", "application/json", `{"name":null}`, 400, "validation-failed", "/name"},
		{"null body for an object", "application/json", `null`, 400, "validation-failed", ""},
		{"too long", "application/json", `{"name":"toolong"}`, 400, "validation-failed", "/name"},
		{"below the minimum", "application/json", `{"count":0}`, 400, "validation-failed", "/count"},
		{"above the maximum", "application/json", `{"count":11}`, 400, "validation-failed", "/count"},
		{"wrong type", "application/json", `{"count":"3"}`, 400, "validation-failed", "/count"},
		{"fraction for an integer", "application/json", `{"count":1.5}`, 400, "validation-failed", "/count"},
		{"bad uuid", "application/json", `{"id":"not-a-uuid"}`, 400, "validation-failed", "/id"},
		{"array for an object", "application/json", `[1]`, 400, "validation-failed", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := h.bodyRequest("token-a", tt.contentType, tt.body)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.status, rec.Body.String())
			}
			if tt.status == 200 {
				if tt.body != "" && !strings.Contains(rec.Body.String(), "received") {
					t.Errorf("the handler did not get the body: %s", rec.Body.String())
				}
				return
			}
			doc := problemOf(t, rec)
			if doc["code"] != tt.code {
				t.Errorf("code = %v, want %s", doc["code"], tt.code)
			}
			if tt.pointer != "" && !strings.Contains(rec.Body.String(), `"pointer":"`+tt.pointer+`"`) {
				t.Errorf("no field error at %s: %s", tt.pointer, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "toolong") {
				t.Errorf("the answer echoes the value sent: %s", rec.Body.String())
			}
		})
	}
}

func TestRequestBodyHandlerReceivesTheBody(t *testing.T) {
	h := newHarness(t, nil)
	rec := h.bodyRequest("token-a", "application/json", `{"name":"ab"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `{\"name\":\"ab\"}`) {
		t.Errorf("handler body = %s", rec.Body.String())
	}
}

func TestMergePatchIsAcceptedWhereDeclared(t *testing.T) {
	h := newHarness(t, nil)
	rec := h.do(http.MethodPut, "/api/v1/drafts", "token-a", func(r *http.Request) {
		r.Header.Set("If-Match", `"1"`)
		r.Header.Set("Content-Type", "application/merge-patch+json")
		r.Body = httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", strings.NewReader(`{"name":"ab"}`)).Body
	})
	if rec.Code != http.StatusOK {
		t.Errorf("merge patch where declared = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
}

// TestBodyValidationAfterAuthentication: the schema is not revealed to a caller
// who is not authenticated or not allowed.
func TestBodyValidationAfterAuthentication(t *testing.T) {
	h := newHarness(t, nil)
	if rec := h.bodyRequest("", "text/plain", `x`); rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated bad body = %d, want 401", rec.Code)
	}
	h.decide = func(authz.Request) authz.Decision { return authz.DecisionForbidden }
	if rec := h.bodyRequest("token-a", "text/plain", `x`); rec.Code != http.StatusForbidden {
		t.Errorf("forbidden bad body = %d, want 403", rec.Code)
	}
}
