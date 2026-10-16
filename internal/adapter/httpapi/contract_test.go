// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/platform/config"
)

// opDoc returns the raw operation object of op.
func opDoc(t *testing.T, spec *Spec, op *Operation) map[string]any {
	t.Helper()
	paths, _ := spec.doc["paths"].(map[string]any)
	item, _ := paths[op.Path].(map[string]any)
	doc, ok := item[strings.ToLower(op.Method)].(map[string]any)
	if !ok {
		t.Fatalf("%s: operation object not found", op.ID)
	}
	return doc
}

// firstExampleRequest builds the request of the first x-orcon-examples case of op.
func firstExampleRequest(t *testing.T, spec *Spec, op *Operation) *http.Request {
	t.Helper()
	doc := opDoc(t, spec, op)
	cases, _ := doc["x-orcon-examples"].([]any)
	if len(cases) == 0 {
		t.Fatalf("%s has no example", op.ID)
	}
	c, _ := cases[0].(map[string]any)
	req, _ := c["request"].(map[string]any)

	target := "/api/v1" + op.Path
	pathParams, _ := req["pathParams"].(map[string]any)
	for name, v := range pathParams {
		target = strings.ReplaceAll(target, "{"+name+"}", url.PathEscape(fmt.Sprint(v)))
	}
	query := url.Values{}
	if q, ok := req["query"].(map[string]any); ok {
		for k, v := range q {
			query.Set(k, fmt.Sprint(v))
		}
	}
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var body []byte
	contentType := ""
	if name, ok := req["body"].(string); ok {
		rb, err := spec.resolveObject(doc["requestBody"])
		if err != nil {
			t.Fatalf("%s: %v", op.ID, err)
		}
		content, _ := rb["content"].(map[string]any)
		for media, mt := range content {
			m, _ := mt.(map[string]any)
			examples, _ := m["examples"].(map[string]any)
			ex, _ := examples[name].(map[string]any)
			if ex == nil {
				t.Fatalf("%s: request example %q not found", op.ID, name)
			}
			var err error
			if body, err = json.Marshal(ex["value"]); err != nil {
				t.Fatalf("%s: %v", op.ID, err)
			}
			contentType = media
		}
	}
	r := httptest.NewRequestWithContext(t.Context(), op.Method, target, bytes.NewReader(body))
	r.RemoteAddr = "192.0.2.10:4000"
	r.Header.Set("Authorization", "Bearer token-a")
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	if headers, ok := req["headers"].(map[string]any); ok {
		for k, v := range headers {
			r.Header.Set(k, fmt.Sprint(v))
		}
	}
	return r
}

// TestUnavailableAnswersEveryOperation: until an operation has a handler it
// answers 503 service-unavailable, and every example request of the contract
// reaches that answer, so the examples bind to the generated server.
func TestUnavailableAnswersEveryOperation(t *testing.T) {
	rt, err := New(&Deps{
		Role:          config.RoleStandalone,
		Idempotency:   memStore(),
		Authenticator: fakeAuthn{"token-a": userA},
		Authorizer:    authzFunc(func(authz.Request) authz.Decision { return authz.DecisionAllow }),
		Limits:        middleware.Limits{Read: 100000, Write: 100000, Run: 100000, StreamConcurrent: 100, InteractiveFactor: 1, IP: 100000},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	spec := rt.Spec()
	if len(spec.Operations) < 30 {
		t.Fatalf("the contract has %d operations; the core surface has more", len(spec.Operations))
	}
	for i := range spec.Operations {
		op := &spec.Operations[i]
		t.Run(op.ID, func(t *testing.T) {
			rec := httptest.NewRecorder()
			rt.ServeHTTP(rec, firstExampleRequest(t, spec, op))
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503 (%s)", rec.Code, rec.Body.String())
			}
			doc := problemOf(t, rec)
			if doc["code"] != "service-unavailable" {
				t.Errorf("code = %v, want service-unavailable", doc["code"])
			}
			if rec.Header().Get("Retry-After") == "" {
				t.Error("503 carries no Retry-After")
			}
		})
	}
}

// TestProblemCatalogMatchesContract keeps the catalog of package problem and
// components/schemas/ProblemCode in step (ERR-6, API-4).
func TestProblemCatalogMatchesContract(t *testing.T) {
	spec := newReal(t).Spec()
	node, ok := spec.lookup("/components/schemas/ProblemCode")
	if !ok {
		t.Fatal("the contract has no ProblemCode")
	}
	schema, _ := node.(map[string]any)
	catalog, _ := schema["x-orcon-problem-catalog"].(map[string]any)
	enum, _ := schema["enum"].([]any)
	var codes []string
	for _, e := range enum {
		codes = append(codes, fmt.Sprint(e))
	}
	var want []string
	for _, s := range problem.Catalog() {
		want = append(want, string(s.Code))
	}
	if !slices.Equal(codes, want) {
		t.Errorf("ProblemCode enum %v, catalog %v", codes, want)
	}
	for _, s := range problem.Catalog() {
		entry, _ := catalog[string(s.Code)].(map[string]any)
		if entry == nil {
			t.Errorf("%s has no catalog entry in the contract", s.Code)
			continue
		}
		if int(entry["status"].(float64)) != s.Status || entry["title"] != s.Title || entry["when"] != s.When || entry["retryable"] != s.Retryable { //nolint:forcetypeassert,errcheck // JSON numbers decode to float64
			t.Errorf("%s differs: contract %v, code %+v", s.Code, entry, s)
		}
	}
}

// coreOperations is the surface API2-core implements, as the spec and the
// amendments define it: operationId, method, path, permission, rate class,
// idempotency ("" for none) and MCP tool ("" for none).
var coreOperations = []struct {
	id, method, path, permission, class, idempotency, tool string
}{
	{"listProjects", "GET", "/orgs/{orgId}/projects", "org:read", "read", "", "list_projects"},
	{"listOrgFlows", "GET", "/orgs/{orgId}/flows", "flow:read", "read", "", ""},
	{"listOrgExecutions", "GET", "/orgs/{orgId}/executions", "execution:read", "read", "", ""},
	{"getOrgActivitySummary", "GET", "/orgs/{orgId}/activity-summary", "execution:read", "read", "", ""},
	{"createProject", "POST", "/orgs/{orgId}/projects", "project:create", "write", "optional", ""},
	{"getProject", "GET", "/projects/{projectId}", "project:read", "read", "", ""},
	{"updateProject", "PATCH", "/projects/{projectId}", "project:update", "write", "", ""},
	{"deleteProject", "DELETE", "/projects/{projectId}", "project:delete", "write", "", ""},
	{"listFlows", "GET", "/projects/{projectId}/flows", "flow:read", "read", "", "search_flows"},
	{"createFlow", "POST", "/projects/{projectId}/flows", "flow:create", "write", "optional", "create_flow"},
	{"getFlow", "GET", "/flows/{flowId}", "flow:read", "read", "", "get_flow"},
	{"updateFlow", "PATCH", "/flows/{flowId}", "flow:update", "write", "", ""},
	{"deleteFlow", "DELETE", "/flows/{flowId}", "flow:delete", "write", "", ""},
	{"getDraft", "GET", "/flows/{flowId}/draft", "flow:read", "read", "", "get_draft"},
	{"replaceDraft", "PUT", "/flows/{flowId}/draft", "flow:update", "write", "", ""},
	{"updateDraft", "PATCH", "/flows/{flowId}/draft", "flow:update", "write", "", "update_draft"},
	{"validateDraft", "POST", "/flows/{flowId}/draft/validate", "flow:read", "write", "optional", "validate_draft"},
	{"listFlowVersions", "GET", "/flows/{flowId}/versions", "flow:read", "read", "", ""},
	{"getFlowVersion", "GET", "/flows/{flowId}/versions/{version}", "flow:read", "read", "", ""},
	{"publishFlow", "POST", "/flows/{flowId}/publish", "flow:publish", "write", "optional", "publish_flow"},
	{"activateFlow", "POST", "/flows/{flowId}/activate", "flow:activate", "write", "optional", "activate_flow"},
	{"deactivateFlow", "POST", "/flows/{flowId}/deactivate", "flow:activate", "write", "optional", "deactivate_flow"},
	{"rollbackFlow", "POST", "/flows/{flowId}/rollback", "flow:rollback", "write", "optional", ""},
	{"createTestSession", "POST", "/flows/{flowId}/test-sessions", "flow:execute", "run", "optional", ""},
	{"getTestSession", "GET", "/flows/{flowId}/test-sessions/{testSessionId}", "flow:execute", "read", "", ""},
	{"cancelTestSession", "POST", "/flows/{flowId}/test-sessions/{testSessionId}/cancel", "flow:execute", "write", "optional", ""},
	{"listExecutions", "GET", "/projects/{projectId}/executions", "execution:read", "read", "", "list_executions"},
	{"createExecution", "POST", "/flows/{flowId}/executions", "flow:execute", "run", "required", "run_flow"},
	{"getExecution", "GET", "/executions/{executionId}", "execution:read", "read", "", "get_execution"},
	{"listExecutionData", "GET", "/executions/{executionId}/data", "execution:read-data", "read", "", ""},
	{"listExecutionNodes", "GET", "/executions/{executionId}/nodes", "execution:read", "read", "", ""},
	{"listExecutionEvents", "GET", "/executions/{executionId}/events", "execution:read", "read", "", "get_execution_events"},
	{"cancelExecution", "POST", "/executions/{executionId}/cancel", "execution:cancel", "write", "optional", "cancel_execution"},
	{"retryExecution", "POST", "/executions/{executionId}/retry", "execution:retry", "run", "optional", "retry_execution"},
	{"resolveExternalOperation", "POST", "/external-operations/{externalOperationId}/resolve", "execution:resolve", "write", "optional", ""},
	{"completeHumanTask", "POST", "/human-tasks/{humanTaskId}/complete", "human-task:resolve", "write", "optional", ""},
	{"listNodeTypes", "GET", "/projects/{projectId}/node-types", "project:read", "read", "", "list_node_types"},
	{"createSession", "POST", "/users/me/sessions", "anonymous", "write", "optional", ""},
	{"revokeSession", "POST", "/users/me/sessions/{sessionId}/revoke", "authenticated", "write", "optional", ""},
	{"getCurrentUser", "GET", "/users/me", "authenticated", "read", "", ""},
	{"listOrgs", "GET", "/orgs", "authenticated", "read", "", ""},
}

// TestCoreOperationsDeclared: the contract declares exactly the operations of
// API2-core and of the editor pieces M1 needs (A7, A10, A12), each with the
// permission, rate class, idempotency and MCP tool the spec gives it.
func TestCoreOperationsDeclared(t *testing.T) {
	spec := newReal(t).Spec()
	byID := map[string]*Operation{}
	for i := range spec.Operations {
		byID[spec.Operations[i].ID] = &spec.Operations[i]
	}
	if len(byID) != len(coreOperations) {
		var got []string
		for id := range byID {
			got = append(got, id)
		}
		t.Errorf("the contract has %d operations, the core surface %d: %v", len(byID), len(coreOperations), got)
	}
	for _, want := range coreOperations {
		op, ok := byID[want.id]
		if !ok {
			t.Errorf("%s is not declared", want.id)
			continue
		}
		if op.Method != want.method || op.Path != want.path {
			t.Errorf("%s is %s %s, want %s %s", want.id, op.Method, op.Path, want.method, want.path)
		}
		if !requirementIs(op.Requirement, want.permission) {
			t.Errorf("%s permission = %+v, want %s", want.id, op.Requirement, want.permission)
		}
		if string(op.Class) != want.class {
			t.Errorf("%s rate class = %s, want %s", want.id, op.Class, want.class)
		}
		if op.IdempotencyKey != want.idempotency {
			t.Errorf("%s idempotency = %q, want %q", want.id, op.IdempotencyKey, want.idempotency)
		}
		doc := opDoc(t, spec, op)
		mcp, _ := doc["x-orcon-mcp"].(map[string]any)
		tool, _ := mcp["tool"].(string)
		if tool != want.tool {
			t.Errorf("%s MCP tool = %q, want %q", want.id, tool, want.tool)
		}
		if op.Stability != "stable" {
			t.Errorf("%s stability = %s, want stable", want.id, op.Stability)
		}
	}
	// Mutating methods that edit a resource require If-Match.
	for _, op := range byID {
		if (op.Method == http.MethodPut || op.Method == http.MethodPatch || op.Method == http.MethodDelete) && !op.IfMatchRequired {
			t.Errorf("%s does not require If-Match", op.ID)
		}
	}
	// Every permission exists in the catalog.
	for _, op := range byID {
		for _, p := range op.Requirement.Permissions {
			if !permissionCatalog(t).Has(p) {
				t.Errorf("%s names the permission %s, which is not in the catalog", op.ID, p)
			}
		}
	}
}

// TestEditorPiecesOfTheCore: A7 (release note and version fields), A10 (node
// types without exampleOutput) and A12 (draft author and 412 members) are in
// the contract (SPEC-005 Amendments 5 and 6).
func TestEditorPiecesOfTheCore(t *testing.T) {
	spec := newReal(t).Spec()
	props := func(schema string) map[string]any {
		node, ok := spec.lookup("/components/schemas/" + schema)
		if !ok {
			t.Fatalf("schema %s is missing", schema)
		}
		resolved, err := spec.resolveObject(node)
		if err != nil {
			t.Fatalf("schema %s: %v", schema, err)
		}
		p, _ := resolved["properties"].(map[string]any)
		return p
	}
	for schema, names := range map[string][]string{
		"FlowVersion":         {"publishedBy", "note", "rolledBackFromVersion", "gitCommit"},
		"PublishFlowRequest":  {"note"},
		"RollbackFlowRequest": {"note", "version"},
		"FlowDraft":           {"updatedBy", "updatedAt"},
		"Problem":             {"currentDraftRevision", "updatedBy"},
		"NodeType":            {"type", "name", "description", "group", "seal", "proofLevel", "operations", "n8nTypes"},
		"NodeTypeOperation":   {"name", "purity", "safetyLevel"},
	} {
		p := props(schema)
		for _, name := range names {
			if _, ok := p[name]; !ok {
				t.Errorf("%s has no %s", schema, name)
			}
		}
	}
	if _, present := props("NodeTypeOperation")["exampleOutput"]; present {
		t.Error("listNodeTypes carries exampleOutput; it arrives with API2b-b")
	}
	if note, _ := props("PublishFlowRequest")["note"].(map[string]any); note["maxLength"] != float64(500) {
		t.Errorf("the release note limit is %v, want 500", note["maxLength"])
	}
}

// TestProjectsCarryEnvironmentAndProtection: projects have an environment label
// and a protection flag that is on by default (R-API-06, ADR-0038 section 2).
func TestProjectsCarryEnvironmentAndProtection(t *testing.T) {
	spec := newReal(t).Spec()
	node, _ := spec.lookup("/components/schemas/CreateProjectRequest")
	req, _ := spec.resolveObject(node)
	props, _ := req["properties"].(map[string]any)
	prot, _ := props["protected"].(map[string]any)
	if prot["default"] != true {
		t.Errorf("protected defaults to %v on creation, want true", prot["default"])
	}
	if _, ok := props["environment"]; !ok {
		t.Error("a project cannot be created with an environment label")
	}
}

// permissionCatalog reads the permission names of the embedded permissions.yaml
// with a line pattern: this package may not import the YAML codec, and the strict
// file format is checked in the codec's own tests.
func permissionCatalog(t *testing.T) *authz.Catalog {
	t.Helper()
	names := regexp.MustCompile(`(?m)^\s+- name: (\S+)\s*$`).FindAllStringSubmatch(string(authz.CatalogSource()), -1)
	var list []authz.Permission
	for _, m := range names {
		list = append(list, authz.Permission(m[1]))
	}
	cat, err := authz.NewCatalog(map[authz.Scope][]authz.Permission{authz.ScopeProject: list})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	return cat
}

// TestTestSessionsAreNotEditable: a test session is canceled with a POST and
// has no ETag, no CLOSED status and no DELETE (Amendment 8, 8-7).
func TestTestSessionsAreNotEditable(t *testing.T) {
	spec := newReal(t).Spec()
	for i := range spec.Operations {
		op := &spec.Operations[i]
		if !strings.Contains(op.Path, "test-sessions") {
			continue
		}
		if op.Method == http.MethodDelete || op.Method == http.MethodPut || op.Method == http.MethodPatch {
			t.Errorf("%s edits a test session", op.ID)
		}
		doc := opDoc(t, spec, op)
		responses, _ := doc["responses"].(map[string]any)
		for status := range responses {
			resp, found := declaredResponseByKey(t, spec, doc, status)
			if !found {
				continue
			}
			headers, _ := resp["headers"].(map[string]any)
			if _, has := headers["ETag"]; has {
				t.Errorf("%s answers %s with an ETag", op.ID, status)
			}
		}
	}
	node, _ := spec.lookup("/components/schemas/TestSession")
	session, _ := spec.resolveObject(node)
	status := session["properties"].(map[string]any)["status"].(map[string]any) //nolint:forcetypeassert,errcheck // the schema is known
	if values := fmt.Sprint(status["enum"]); strings.Contains(values, "CLOSED") {
		t.Errorf("TestSession.status still has CLOSED: %s", values)
	}
}

// TestStateGuardedActionsTakeIfMatch: publish takes the draft ETag and the
// actions on the flow row take its revision (Amendment 8, 8-9).
func TestStateGuardedActionsTakeIfMatch(t *testing.T) {
	spec := newReal(t).Spec()
	need := map[string]bool{"publishFlow": true, "activateFlow": true, "deactivateFlow": true, "rollbackFlow": true, "updateFlow": true, "deleteFlow": true, "updateProject": true, "deleteProject": true, "replaceDraft": true, "updateDraft": true}
	for i := range spec.Operations {
		op := &spec.Operations[i]
		if need[op.ID] && !op.IfMatchRequired {
			t.Errorf("%s does not require If-Match", op.ID)
		}
	}
}

// declaredResponseByKey resolves the response of op under the status key.
func declaredResponseByKey(t *testing.T, spec *Spec, doc map[string]any, key string) (map[string]any, bool) {
	t.Helper()
	responses, _ := doc["responses"].(map[string]any)
	raw, ok := responses[key]
	if !ok {
		return nil, false
	}
	resp, err := spec.resolveObject(raw)
	if err != nil {
		t.Fatalf("response %s: %v", key, err)
	}
	return resp, true
}

// TestOperationsWithBodiesDeclareTheMediaTypeProblem (BL-090): the 415 an
// operation can answer is part of its contract.
func TestOperationsWithBodiesDeclareTheMediaTypeProblem(t *testing.T) {
	spec := newReal(t).Spec()
	checked := 0
	for i := range spec.Operations {
		op := &spec.Operations[i]
		if !op.HasBody {
			continue
		}
		checked++
		doc := opDoc(t, spec, op)
		problems, _ := doc["x-orcon-problems"].([]any)
		if !slices.Contains(problems, any("unsupported-media-type")) {
			t.Errorf("%s does not declare unsupported-media-type", op.ID)
		}
		if _, ok := declaredResponseByKey(t, spec, doc, "415"); !ok {
			t.Errorf("%s declares no 415 response", op.ID)
		}
		if len(op.BodyMedia) == 0 {
			t.Errorf("%s declares no media type for its body", op.ID)
		}
	}
	if checked == 0 {
		t.Fatal("the contract has no operation with a body")
	}
}
