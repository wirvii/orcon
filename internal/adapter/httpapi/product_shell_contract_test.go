// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/authz"
)

// requirementIs reports whether req is the requirement named by an
// x-orcon-permission value: authenticated, anonymous or one permission.
func requirementIs(req authz.Requirement, want string) bool {
	switch want {
	case "authenticated":
		return req.Mode == authz.ModeAuthenticated
	case "anonymous":
		return req.Mode == authz.ModeAnonymous
	}
	return len(req.Permissions) == 1 && string(req.Permissions[0]) == want
}

// queryParamDoc returns the raw parameter object named name of an operation
// document, or nil.
func queryParamDoc(doc map[string]any, name string) map[string]any {
	params, _ := doc["parameters"].([]any)
	for _, p := range params {
		m, _ := p.(map[string]any)
		if m["name"] == name {
			return m
		}
	}
	return nil
}

// requiredOf returns the required list of a component schema of the contract.
func requiredOf(t *testing.T, spec *Spec, schema string) []string {
	t.Helper()
	node, ok := spec.lookup("/components/schemas/" + schema)
	if !ok {
		t.Fatalf("schema %s is not declared", schema)
	}
	obj, err := spec.resolveObject(node)
	if err != nil {
		t.Fatalf("schema %s: %v", schema, err)
	}
	var out []string
	for _, r := range listOf(obj["required"]) {
		s, _ := r.(string)
		out = append(out, s)
	}
	return out
}

// TestProductShellContract (SPEC-016 section 3.1 and 3.2): the eight operations
// the product shell reads, their extensions, filters and schemas, the CSRF
// problem and the closed list of anonymous operations.
func TestProductShellContract(t *testing.T) {
	spec := newReal(t).Spec()
	byID := map[string]*Operation{}
	for i := range spec.Operations {
		byID[spec.Operations[i].ID] = &spec.Operations[i]
	}

	// Method and path of the eight operations (the first four are also in the core table).
	shell := []struct{ id, method, path string }{
		{"createSession", http.MethodPost, "/users/me/sessions"},
		{"revokeSession", http.MethodPost, "/users/me/sessions/{sessionId}/revoke"},
		{"getCurrentUser", http.MethodGet, "/users/me"},
		{"listOrgs", http.MethodGet, "/orgs"},
		{"listProjects", http.MethodGet, "/orgs/{orgId}/projects"},
		{"createProject", http.MethodPost, "/orgs/{orgId}/projects"},
		{"listFlows", http.MethodGet, "/projects/{projectId}/flows"},
		{"createFlow", http.MethodPost, "/projects/{projectId}/flows"},
	}
	for _, want := range shell {
		op, ok := byID[want.id]
		if !ok {
			t.Errorf("%s is not declared", want.id)
			continue
		}
		if op.Method != want.method || op.Path != want.path {
			t.Errorf("%s is %s %s, want %s %s", want.id, op.Method, op.Path, want.method, want.path)
		}
	}

	// x-orcon-audit: the sign-in and the new flows and projects are audited; the reads are not.
	audits := map[string]string{
		"createSession":  "login.succeeded",
		"revokeSession":  "logout",
		"getCurrentUser": "none",
		"listOrgs":       "none",
		"listProjects":   "none",
		"createProject":  "project.created",
		"listFlows":      "none",
		"createFlow":     "flow.created",
	}
	for id, want := range audits {
		if got := opDoc(t, spec, byID[id])["x-orcon-audit"]; got != want {
			t.Errorf("%s x-orcon-audit = %v, want %s", id, got, want)
		}
	}

	// Mutations that can be retried carry an optional Idempotency-Key.
	for _, id := range []string{"createSession", "revokeSession", "createProject", "createFlow"} {
		if byID[id].IdempotencyKey != "optional" {
			t.Errorf("%s idempotency = %q, want optional", id, byID[id].IdempotencyKey)
		}
	}

	// The CLI and MCP surfaces of the four new operations.
	cli := map[string]string{"getCurrentUser": "users whoami", "listOrgs": "orgs list"}
	for id, command := range cli {
		cliDoc, _ := opDoc(t, spec, byID[id])["x-orcon-cli"].(map[string]any)
		if cliDoc["command"] != command {
			t.Errorf("%s x-orcon-cli command = %v, want %s", id, cliDoc["command"], command)
		}
	}
	for _, id := range []string{"createSession", "revokeSession"} {
		cliDoc, _ := opDoc(t, spec, byID[id])["x-orcon-cli"].(map[string]any)
		none, _ := cliDoc["none"].(map[string]any)
		if none["category"] != "interactive-session-only" {
			t.Errorf("%s x-orcon-cli category = %v, want interactive-session-only", id, none["category"])
		}
	}
	for _, id := range []string{"createSession", "revokeSession", "getCurrentUser", "listOrgs"} {
		mcp, _ := opDoc(t, spec, byID[id])["x-orcon-mcp"].(map[string]any)
		if mcp["level"] != "none" {
			t.Errorf("%s x-orcon-mcp level = %v, want none", id, mcp["level"])
		}
	}

	// Filters and default order (SPEC-016 R-SH-API-03).
	if p := queryParamDoc(opDoc(t, spec, byID["listOrgs"]), "slug"); p == nil {
		t.Error("listOrgs has no slug filter")
	}
	if p := queryParamDoc(opDoc(t, spec, byID["listProjects"]), "handle"); p == nil {
		t.Error("listProjects has no handle filter")
	}
	sortParam := queryParamDoc(opDoc(t, spec, byID["listFlows"]), "sort")
	schema, _ := sortParam["schema"].(map[string]any)
	if schema["default"] != "-updatedAt" {
		t.Errorf("listFlows default sort = %v, want -updatedAt", schema["default"])
	}

	// Required fields of the shapes the screens read.
	wantRequired := map[string][]string{
		"Session":      {"id", "createdAt", "expiresAt", "idleExpiresAt", "assuranceLevel"},
		"CurrentUser":  {"id", "email", "displayName", "currentSession"},
		"Organization": {"id", "slug", "name"},
		"Project":      {"id", "orgId", "name", "handle", "protected", "createdAt", "updatedAt"},
		"Flow":         {"id", "projectId", "name", "activationState", "updatedAt", "createdAt"},
	}
	for schema, fields := range wantRequired {
		got := requiredOf(t, spec, schema)
		for _, f := range fields {
			if !slices.Contains(got, f) {
				t.Errorf("%s does not require %s", schema, f)
			}
		}
	}

	// The password is accepted and never returned.
	node, _ := spec.lookup("/components/schemas/SessionCredentials/properties/password")
	pw, _ := spec.resolveObject(node)
	if pw["writeOnly"] != true {
		t.Error("SessionCredentials.password is not writeOnly")
	}

	// The CSRF problem is in the catalog and in the operations that can meet it.
	catalog, _ := spec.lookup("/components/schemas/ProblemCode/x-orcon-problem-catalog/csrf-check-failed")
	entry, _ := catalog.(map[string]any)
	if fmt.Sprint(entry["status"]) != "403" {
		t.Errorf("csrf-check-failed status = %v, want 403", entry["status"])
	}
	for _, id := range []string{"revokeSession", "createProject", "createFlow"} {
		problems, _ := opDoc(t, spec, byID[id])["x-orcon-problems"].([]any)
		if !slices.Contains(anyStrings(problems), "csrf-check-failed") {
			t.Errorf("%s does not declare csrf-check-failed", id)
		}
	}

	// The cookie scheme describes the CSRF cookie and header.
	scheme, _ := spec.lookup("/components/securitySchemes/sessionCookie/description")
	text, _ := scheme.(string)
	for _, want := range []string{"__Host-orcon_csrf", "Orcon-Csrf-Token"} {
		if !strings.Contains(text, want) {
			t.Errorf("sessionCookie description does not name %s", want)
		}
	}

	// Only protocol operations and the closed list may be anonymous.
	for i := range spec.Operations {
		op := &spec.Operations[i]
		if op.Requirement.Mode == authz.ModeAnonymous && !op.Protocol && !anonymousOperation(op.ID) {
			t.Errorf("%s is anonymous but is neither a protocol operation nor on the closed list", op.ID)
		}
	}
}

// anyStrings returns the string elements of a decoded YAML list.
func anyStrings(list []any) []string {
	var out []string
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
