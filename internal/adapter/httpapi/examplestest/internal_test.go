// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest

import (
	"context"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func decode(t *testing.T, doc string) any {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("decode %s: %v", doc, err)
	}
	return v
}

func TestBind(t *testing.T) {
	const fixed = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60"
	b := Bindings{fixed: "real-id", `"7"`: `"42"`}
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"whole value", fixed, "real-id"},
		{"short key matches whole values only", `"7"`, `"42"`},
		{"short key is not replaced inside text", `x "7" y`, `x "7" y`},
		{"long key replaced inside text", "/flows/" + fixed + "/runs", "/flows/real-id/runs"},
		{"unbound text untouched", "plain", "plain"},
	}
	for _, tt := range tests {
		if got := b.bind(tt.in); got != tt.want {
			t.Errorf("%s: bind(%q) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
	}
	got, _ := b.bindValue(map[string]any{"a": []any{fixed, 1}, "n": nil}).(map[string]any)
	list, _ := got["a"].([]any)
	if len(list) == 0 || list[0] != "real-id" {
		t.Errorf("bindValue did not reach nested strings: %v", got)
	}
}

func TestCompare(t *testing.T) {
	c := &contract{root: map[string]any{}}
	volatile := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"token": map[string]any{"x-orcon-volatile": true},
			"items": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
		},
	}
	tests := []struct {
		name        string
		want, got   string
		schema      map[string]any
		wantPointer string
		wantFailure bool
	}{
		{"equal", `{"a":1,"b":["x"]}`, `{"a":1,"b":["x"],"extra":true}`, nil, "", false},
		{"missing field", `{"a":1}`, `{}`, nil, "/a", true},
		{"different number", `{"a":1}`, `{"a":2}`, nil, "/a", true},
		{"array length", `{"b":[1,2]}`, `{"b":[1]}`, nil, "/b", true},
		{"array item", `{"b":[1,2]}`, `{"b":[1,3]}`, nil, "/b/1", true},
		{"wrong kind object", `{"a":{"x":1}}`, `{"a":"s"}`, nil, "/a", true},
		{"wrong kind array", `{"a":[1]}`, `{"a":{}}`, nil, "/a", true},
		{"wrong kind string", `{"a":"s"}`, `{"a":1}`, nil, "/a", true},
		{"unbound uuid by format", `{"a":"0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a60"}`, `{"a":"0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a99"}`, nil, "", false},
		{"unbound date-time by format", `{"a":"2026-10-12T15:04:05Z"}`, `{"a":"2027-01-01T00:00:00.5Z"}`, nil, "", false},
		{"volatile by presence", `{"token":"a"}`, `{"token":"zzz"}`, volatile, "", false},
		{"volatile wrong kind", `{"token":"a"}`, `{"token":3}`, volatile, "/token", true},
		{"null equal", `{"a":null}`, `{"a":null}`, nil, "", false},
		{"number is not its text", `{"a":1}`, `{"a":"1"}`, nil, "/a", true},
		{"boolean is not its text", `{"a":true}`, `{"a":"true"}`, nil, "/a", true},
		{"null is not its Go text", `{"a":null}`, `{"a":"<nil>"}`, nil, "/a", true},
		{"null is not false", `{"a":null}`, `{"a":false}`, nil, "/a", true},
		{"true is not false", `{"a":true}`, `{"a":false}`, nil, "/a", true},
		{"text is not a number", `{"a":"1"}`, `{"a":1}`, nil, "/a", true},
		{"same number written differently", `{"a":1}`, `{"a":1.0}`, nil, "", false},
		{"different numbers", `{"a":1.5}`, `{"a":1.6}`, nil, "/a", true},
	}
	for _, tt := range tests {
		d := c.compare(decode(t, tt.want), decode(t, tt.got), tt.schema, "/schema", "")
		if (d != nil) != tt.wantFailure {
			t.Errorf("%s: diff = %v, want failure %v", tt.name, d, tt.wantFailure)
			continue
		}
		if d != nil && d.pointer != tt.wantPointer {
			t.Errorf("%s: pointer %q, want %q", tt.name, d.pointer, tt.wantPointer)
		}
	}
}

func TestLoadContractRejectsBadDocuments(t *testing.T) {
	for name, doc := range map[string]string{
		"not json":      `{`,
		"not an object": `[]`,
		"no openapi":    `{"paths":{}}`,
	} {
		if _, err := loadContract("c.json", []byte(doc)); err == nil {
			t.Errorf("%s: loadContract succeeded", name)
		}
	}
}

func TestResolveFailures(t *testing.T) {
	doc := `{"openapi":"3.1.0","components":{"a":{"$ref":"#/components/b"},"b":{"$ref":"#/components/a"},"ok":{"x":1}}}`
	c, err := loadContract("c.json", []byte(doc))
	if err != nil {
		t.Fatalf("loadContract: %v", err)
	}
	for name, node := range map[string]any{
		"cycle":    map[string]any{"$ref": "#/components/a"},
		"missing":  map[string]any{"$ref": "#/components/none"},
		"external": map[string]any{"$ref": "other.json#/x"},
		"not obj":  "text",
	} {
		if _, _, err := c.resolve(node, "/p"); err == nil {
			t.Errorf("%s: resolve succeeded", name)
		}
	}
	got, ptr, err := c.resolve(map[string]any{"$ref": "#/components/ok"}, "/p")
	if err != nil || ptr != "/components/ok" || got["x"] == nil {
		t.Errorf("resolve ok = %v, %q, %v", got, ptr, err)
	}
}

func TestExamplesRejectsMalformedEntries(t *testing.T) {
	op := func(example string) string {
		return `{"openapi":"3.1.0","paths":{"/a":{"get":{"operationId":"a","x-orcon-examples":[` + example + `]}}}}`
	}
	for name, example := range map[string]string{
		"not object":        `1`,
		"no name":           `{"response":{"status":200}}`,
		"no response":       `{"name":"x"}`,
		"no status":         `{"name":"x","response":{}}`,
		"bad status":        `{"name":"x","response":{"status":"abc"}}`,
		"stream no trigger": `{"name":"x","response":{"status":200},"stream":{"expectTypes":["a"]}}`,
		"stream no types":   `{"name":"x","response":{"status":200},"stream":{"trigger":"t"}}`,
		"stream too long":   `{"name":"x","response":{"status":200},"stream":{"trigger":"t","expectTypes":["a"],"within":"11s"}}`,
		"stream bad within": `{"name":"x","response":{"status":200},"stream":{"trigger":"t","expectTypes":["a"],"within":"soon"}}`,
		"operation no end":  `{"name":"x","response":{"status":202},"operation":{}}`,
	} {
		c, err := loadContract("c.json", []byte(op(example)))
		if err != nil {
			t.Fatalf("%s: loadContract: %v", name, err)
		}
		if _, err := c.examples(); err == nil {
			t.Errorf("%s: examples succeeded", name)
		}
	}
	c, err := loadContract("c.json", []byte(op(`{"name":"x","response":{"status":"204"},"stream":{"trigger":"t","expectTypes":["a"]}}`)))
	if err != nil {
		t.Fatalf("loadContract: %v", err)
	}
	defs, err := c.examples()
	if err != nil || len(defs) != 1 || defs[0].Status != 204 || defs[0].Stream.Within != defaultStreamWithin {
		t.Fatalf("examples = %+v, %v", defs, err)
	}
}

func TestProtocolOperationsAreExempt(t *testing.T) {
	if !isProtocolOperation("/mcp", nil) {
		t.Error("/mcp is not exempt")
	}
	op := map[string]any{"x-orcon-cli": map[string]any{"none": map[string]any{"category": "protocol"}}}
	if !isProtocolOperation("/oauth/token", op) {
		t.Error("a protocol operation is not exempt")
	}
	if isProtocolOperation("/flows", map[string]any{}) {
		t.Error("an ordinary operation is exempt")
	}
}

func TestRunReportsContractProblems(t *testing.T) {
	ctx := t.Context()
	if _, err := Run(ctx, Config{Contract: []byte(`{}`)}); err == nil {
		t.Error("Run without BaseURL succeeded")
	}
	if _, err := Run(ctx, Config{BaseURL: "http://127.0.0.1:1", Contract: []byte(`{`)}); err == nil {
		t.Error("Run with a broken contract succeeded")
	}
}

func TestRegisterScenario(t *testing.T) {
	mustPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: no panic", name)
			}
		}()
		fn()
	}
	noop := func(context.Context, *Client) (Bindings, error) { return nil, nil }
	mustPanic("empty name", func() { RegisterScenario("", noop) })
	mustPanic("nil function", func() { RegisterScenario("x", nil) })
	RegisterScenario("registered-once", noop)
	t.Cleanup(func() {
		registryMu.Lock()
		delete(registry, "registered-once")
		registryMu.Unlock()
	})
	mustPanic("duplicate", func() { RegisterScenario("registered-once", noop) })
	if _, ok := registered()["registered-once"]; !ok {
		t.Error("registered scenario is missing from the registry")
	}
}

func TestResultString(t *testing.T) {
	ok := Result{Contract: "c.json", OperationID: "op", Example: "ex", DB: "postgres"}
	if got := ok.String(); !strings.Contains(got, "ok") || ok.Failed() {
		t.Errorf("passing result = %q", got)
	}
	bad := ok
	bad.Kind, bad.Pointer, bad.Detail = KindValue, "/name", "want a, got b"
	want := "c.json: operationId=op example=ex db=postgres: value: want a, got b at /name"
	if got := bad.String(); got != want {
		t.Errorf("failing result = %q, want %q", got, want)
	}
}

func TestStreamAndOperationFailures(t *testing.T) {
	r := &runner{byName: map[string][]exampleDef{
		"dup":  {{Name: "dup"}, {Name: "dup"}},
		"once": {{Name: "once"}},
	}}
	if _, f := r.findTrigger(exampleDef{Stream: &streamDef{Trigger: "none"}}); f == nil || f.kind != KindContract {
		t.Errorf("unknown trigger = %v", f)
	}
	if _, f := r.findTrigger(exampleDef{Stream: &streamDef{Trigger: "dup"}}); f == nil || !strings.Contains(f.detail, "ambiguous") {
		t.Errorf("ambiguous trigger = %v", f)
	}
	if got, f := r.findTrigger(exampleDef{Stream: &streamDef{Trigger: "once"}}); f != nil || got.Name != "once" {
		t.Errorf("trigger = %v, %v", got, f)
	}
	if typeOfData(`{"type":"a.b"}`) != "a.b" || typeOfData("nope") != "" {
		t.Error("typeOfData")
	}
}
