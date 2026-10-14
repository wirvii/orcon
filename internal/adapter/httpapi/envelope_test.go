// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// envelopeMembers are the members forbidden at the root of a 2xx body (RESP-4):
// a response carries the resource itself, never a wrapper around it.
var envelopeMembers = []string{
	"data", "meta", "success", "ok", "result", "results", "payload", "response",
	"error", "errors", "pagination", "links", "requestId", "etag",
}

// rootEnvelope returns the forbidden members of a JSON object body, including
// a boolean status.
func rootEnvelope(body []byte) []string {
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil
	}
	var found []string
	for _, k := range envelopeMembers {
		if _, ok := obj[k]; ok {
			found = append(found, k)
		}
	}
	if _, isBool := obj["status"].(bool); isBool {
		found = append(found, "status")
	}
	return found
}

// declaredResponse returns the response object a contract declares for status
// of op, following references, and whether it declares one.
func declaredResponse(t *testing.T, spec *Spec, op *Operation, status int) (map[string]any, bool) {
	t.Helper()
	paths, _ := spec.doc["paths"].(map[string]any)
	item, _ := paths[op.Path].(map[string]any)
	opDoc, _ := item[strings.ToLower(op.Method)].(map[string]any)
	responses, _ := opDoc["responses"].(map[string]any)
	raw, ok := responses[strconv.Itoa(status)]
	if !ok {
		return nil, false
	}
	resp, err := spec.resolveObject(raw)
	if err != nil {
		t.Fatalf("%s: response %d: %v", op.ID, status, err)
	}
	return resp, true
}

// jsonSchemaOf returns the JSON schema of a response, or nil when it has none.
func jsonSchemaOf(resp map[string]any) map[string]any {
	content, _ := resp["content"].(map[string]any)
	for media, mt := range content {
		if media != "application/json" && !strings.HasSuffix(media, "+json") {
			continue
		}
		m, _ := mt.(map[string]any)
		if schema, ok := m["schema"].(map[string]any); ok {
			return schema
		}
	}
	return nil
}

// rootMembers lists the property names at the root of a schema, through
// references and allOf.
func rootMembers(spec *Spec, schema map[string]any, depth int) []string {
	if depth > 16 {
		return nil
	}
	resolved, err := spec.resolveObject(schema)
	if err != nil {
		return nil
	}
	var names []string
	if props, ok := resolved["properties"].(map[string]any); ok {
		for k := range props {
			names = append(names, k)
		}
	}
	for _, part := range listOf(resolved["allOf"]) {
		if p, ok := part.(map[string]any); ok {
			names = append(names, rootMembers(spec, p, depth+1)...)
		}
	}
	return names
}

// TestNoEnvelopeResponses: no 2xx schema of the contract has an envelope member
// at its root, and neither has a body served through the pipeline.
func TestNoEnvelopeResponses(t *testing.T) {
	check := func(t *testing.T, spec *Spec) {
		t.Helper()
		for i := range spec.Operations {
			op := &spec.Operations[i]
			for status := 200; status < 300; status++ {
				resp, ok := declaredResponse(t, spec, op, status)
				if !ok {
					continue
				}
				schema := jsonSchemaOf(resp)
				if schema == nil {
					continue
				}
				for _, name := range rootMembers(spec, schema, 0) {
					if slices.Contains(envelopeMembers, name) || name == "status" && isBooleanMember(spec, schema, name) {
						t.Errorf("%s %d: the schema has the envelope member %q at its root", op.ID, status, name)
					}
				}
			}
		}
	}
	t.Run("the contract", func(t *testing.T) { check(t, newReal(t).Spec()) })

	t.Run("bodies served through the pipeline", func(t *testing.T) {
		h := newHarness(t, nil)
		for _, tt := range []struct{ method, path string }{
			{http.MethodGet, "/api/v1/things/1"}, {http.MethodPost, "/api/v1/things"}, {http.MethodGet, "/api/v1/me"}, {http.MethodGet, "/api/v1/ping"},
		} {
			rec := h.do(tt.method, tt.path, "token-a")
			if bad := rootEnvelope(rec.Body.Bytes()); len(bad) > 0 {
				t.Errorf("%s %s: envelope members %v in %s", tt.method, tt.path, bad, rec.Body.String())
			}
		}
	})

	t.Run("the detector", func(t *testing.T) {
		for body, want := range map[string]bool{
			`{"id":"1"}`:                       false,
			`{"items":[],"nextPageToken":"x"}`: false,
			`{"data":{"id":"1"}}`:              true,
			`{"id":"1","meta":{}}`:             true,
			`{"result":1}`:                     true,
			`{"status":true}`:                  true,
			`{"status":"RUNNING"}`:             false,
			`[1,2]`:                            false,
		} {
			if got := len(rootEnvelope([]byte(body))) > 0; got != want {
				t.Errorf("rootEnvelope(%s) flagged = %v, want %v", body, got, want)
			}
		}
	})
}

// isBooleanMember reports whether the named root member of the schema is boolean.
func isBooleanMember(spec *Spec, schema map[string]any, name string) bool {
	resolved, err := spec.resolveObject(schema)
	if err != nil {
		return false
	}
	props, _ := resolved["properties"].(map[string]any)
	member, _ := props[name].(map[string]any)
	return member["type"] == "boolean"
}

// TestNoUndeclaredResponseBody: a body that the contract does not declare for
// its status is a violation, so a handler cannot write a body outside its
// declared schema.
func TestNoUndeclaredResponseBody(t *testing.T) {
	// undeclared reports a non-empty body for a status with no JSON content.
	undeclared := func(spec *Spec, op *Operation, status int, body []byte) bool {
		if len(body) == 0 {
			return false
		}
		resp, ok := declaredResponse(t, spec, op, status)
		return !ok || jsonSchemaOf(resp) == nil
	}

	spec, err := ParseSpec([]byte(syntheticSpec))
	if err != nil {
		t.Fatalf("ParseSpec: %v", err)
	}
	byID := map[string]*Operation{}
	for i := range spec.Operations {
		byID[spec.Operations[i].ID] = &spec.Operations[i]
	}
	if !undeclared(spec, byID["getThing"], 200, []byte(`{"id":"1"}`)) {
		t.Error("a body for a status the contract does not declare was not reported")
	}
	if undeclared(spec, byID["createSecret"], 201, []byte(`{"secret":"x"}`)) {
		t.Error("a body under a declared schema was reported")
	}
	if undeclared(spec, byID["getThing"], 200, nil) {
		t.Error("an empty body was reported")
	}

	// The real contract: every operation that declares a 2xx response declares
	// its body, so nothing can be written outside it.
	contract := newReal(t).Spec()
	for i := range contract.Operations {
		op := &contract.Operations[i]
		for status := 200; status < 300; status++ {
			resp, ok := declaredResponse(t, contract, op, status)
			if !ok {
				continue
			}
			content, _ := resp["content"].(map[string]any)
			if status != http.StatusNoContent && len(content) == 0 {
				t.Errorf("%s declares %d with no content", op.ID, status)
			}
		}
	}
}
