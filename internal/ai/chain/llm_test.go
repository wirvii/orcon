// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package chain_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/wirvii/orcon/internal/ai/chain"
	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// reply is one scripted answer of a fake model.
type reply struct {
	resp provider.Response
	err  error
}

// fakeModel answers from a script and records the requests it got.
type fakeModel struct {
	mu       sync.Mutex
	script   []reply
	requests []provider.Request
}

func (f *fakeModel) Capabilities(string) provider.Capabilities {
	return provider.Capabilities{JSONSchema: true}
}

func (f *fakeModel) Generate(_ context.Context, req provider.Request) (provider.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)
	if len(f.script) == 0 {
		return provider.Response{}, errors.New("fake: no scripted reply left")
	}
	r := f.script[0]
	f.script = f.script[1:]
	return r.resp, r.err
}

// fakeModels resolves each provider name to one fake model, and refuses the names it does not know.
type fakeModels map[string]*fakeModel

func (m fakeModels) Model(_ context.Context, _ identity.OrgID, _ identity.ProjectID, ref flow.ModelRef) (provider.ModelProvider, error) { //nolint:ireturn // implements the chain.Models port
	f, ok := m[ref.Provider]
	if !ok {
		return nil, errors.New("no such provider")
	}
	return f, nil
}

func parse(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		t.Fatalf("parse %s: %v", text, err)
	}
	return v
}

// run executes one node with the parameters on one item.
func run(t *testing.T, models chain.Models, params, input string) (execution.NodeOutput, error) {
	t.Helper()
	node, err := chain.NewLLM(chain.Config{Models: models})
	if err != nil {
		t.Fatal(err)
	}
	return node.Execute(t.Context(), execution.NodeCall{
		Org:   identity.OrgID(id.Derive("org")),
		Node:  &flow.Node{ID: "llm", Name: "llm", Type: chain.LLMNodeType, Parameters: parse(t, params)},
		Input: execution.NewBatch(execution.Item{JSON: parse(t, input)}),
	})
}

func member(t *testing.T, out execution.NodeOutput, key string) jsonvalue.Value {
	t.Helper()
	if len(out.Ports) != 1 || out.Ports[0].Batch.Len() != 1 {
		t.Fatalf("output = %+v, want one item", out)
	}
	v, ok := out.Ports[0].Batch.Item(0).JSON.Get(key)
	if !ok {
		t.Fatalf("the output has no %q member", key)
	}
	return v
}

func code(err error) string {
	var ne *execution.NodeError
	if errors.As(err, &ne) {
		return ne.Code
	}
	return ""
}

const params = `{"model":{"provider":"a","name":"m1"}}`

// TestLLMAnswersEachItem: the answer joins the item with the model that gave it, and the prompt carries the
// system text and the chosen input field.
func TestLLMAnswersEachItem(t *testing.T) {
	t.Parallel()
	a := &fakeModel{script: []reply{{resp: provider.Response{Text: "hello"}}}}
	out, err := run(t, fakeModels{"a": a}, `{"model":{"provider":"a","name":"m1"},"system":"be brief","inputField":"q","maxOutputTokens":50}`, `{"q":"hi","keep":1}`)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if s, _ := member(t, out, "text").Str(); s != "hello" {
		t.Errorf("text = %q", s)
	}
	if s, _ := member(t, out, "model").Str(); s != "a/m1" {
		t.Errorf("model = %q", s)
	}
	if _, ok := out.Ports[0].Batch.Item(0).JSON.Get("keep"); !ok {
		t.Error("the input members were dropped")
	}
	req := a.requests[0]
	if len(req.Messages) != 2 || req.Messages[0].Role != provider.RoleSystem || req.MaxOutputTokens != 50 {
		t.Errorf("request = %+v", req)
	}
	empty, err := run(t, fakeModels{"a": a}, params, `{"text":"x"}`)
	if err == nil && len(empty.Ports) != 0 {
		t.Errorf("a spent script still answered: %+v", empty)
	}
}

// TestLLMWithoutInputItems: an activation with no item emits nothing.
func TestLLMWithoutInputItems(t *testing.T) {
	t.Parallel()
	node, err := chain.NewLLM(chain.Config{Models: fakeModels{}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := node.Execute(t.Context(), execution.NodeCall{
		Node: &flow.Node{Type: chain.LLMNodeType, Parameters: parse(t, params)},
	})
	if err != nil || len(out.Ports) != 0 {
		t.Errorf("Execute = %+v, %v", out, err)
	}
}

// TestLLMFallsBackOnlyOnTransientFailures: a transient failure moves to the next model, a permanent one does not,
// and when every model fails transiently the node reports it as retryable.
func TestLLMFallsBackOnlyOnTransientFailures(t *testing.T) {
	t.Parallel()
	transient := &provider.Error{Class: provider.ClassTransient, Status: 503, Message: "busy"}
	permanent := &provider.Error{Class: provider.ClassPermanent, Status: 400, Message: "bad request"}
	withFallback := `{"model":{"provider":"a","name":"m1"},"fallbacks":[{"provider":"b","name":"m2"}]}`

	a := &fakeModel{script: []reply{{err: transient}}}
	b := &fakeModel{script: []reply{{resp: provider.Response{Text: "from b"}}}}
	out, err := run(t, fakeModels{"a": a, "b": b}, withFallback, `{"text":"x"}`)
	if err != nil {
		t.Fatalf("fallback: %v", err)
	}
	if s, _ := member(t, out, "model").Str(); s != "b/m2" {
		t.Errorf("the answering model = %q, want b/m2", s)
	}

	a = &fakeModel{script: []reply{{err: permanent}}}
	b = &fakeModel{}
	if _, err := run(t, fakeModels{"a": a, "b": b}, withFallback, `{"text":"x"}`); code(err) != chain.CodeProviderRejected || len(b.requests) != 0 {
		t.Errorf("permanent failure = %v (code %q), fallback requests %d", err, code(err), len(b.requests))
	}

	a = &fakeModel{script: []reply{{err: transient}}}
	b = &fakeModel{script: []reply{{err: errors.New("network down")}}}
	_, err = run(t, fakeModels{"a": a, "b": b}, withFallback, `{"text":"x"}`)
	var ne *execution.NodeError
	if !errors.As(err, &ne) || ne.Code != chain.CodeModelUnavailable || ne.Class != execution.ClassTransient {
		t.Errorf("every model failed = %v", err)
	}

	if _, err := run(t, fakeModels{}, params, `{"text":"x"}`); code(err) != chain.CodeModelUnavailable {
		t.Errorf("an unresolvable model = %v", err)
	}
}

// TestLLMStructuredAnswers: a JSON answer is parsed, an answer that is not JSON is refused permanently, and a
// JSON_SCHEMA answer goes through the structured path.
func TestLLMStructuredAnswers(t *testing.T) {
	t.Parallel()
	json := `{"model":{"provider":"a","name":"m1"},"responseFormat":"JSON"}`
	a := &fakeModel{script: []reply{{resp: provider.Response{Text: `{"n":1}`}}, {resp: provider.Response{Text: "not json"}}}}
	out, err := run(t, fakeModels{"a": a}, json, `{"text":"x"}`)
	if err != nil {
		t.Fatalf("JSON answer: %v", err)
	}
	if v := member(t, out, "json"); v.Kind() != jsonvalue.KindObject {
		t.Errorf("json member = %v", v)
	}
	if _, err := run(t, fakeModels{"a": a}, json, `{"text":"x"}`); code(err) != chain.CodeOutputInvalid {
		t.Errorf("an answer that is not JSON = %v (code %q)", err, code(err))
	}

	schema := `{"model":{"provider":"a","name":"m1"},"responseFormat":"JSON_SCHEMA","repairAttempts":0,` +
		`"schema":{"type":"object","required":["n"],"properties":{"n":{"type":"integer"}}}}`
	a = &fakeModel{script: []reply{{resp: provider.Response{Text: `{"n":2}`}}}}
	out, err = run(t, fakeModels{"a": a}, schema, `{"text":"x"}`)
	if err != nil {
		t.Fatalf("JSON_SCHEMA answer: %v", err)
	}
	if v := member(t, out, "json"); v.Kind() != jsonvalue.KindObject {
		t.Errorf("json member = %v", v)
	}
}

// TestLLMRefusesBadNodes: every malformed parameter set and every bad item is a permanent node error.
func TestLLMRefusesBadNodes(t *testing.T) {
	t.Parallel()
	bad := map[string]string{ //nolint:gosec // node parameter documents, not credentials
		"no model":              `{}`,
		"model without a name":  `{"model":{"provider":"a"}}`,
		"fallback without name": `{"model":{"provider":"a","name":"m"},"fallbacks":[{"provider":"b"}]}`,
		"tools":                 `{"model":{"provider":"a","name":"m"},"tools":[]}`,
		"zero max tokens":       `{"model":{"provider":"a","name":"m"},"maxOutputTokens":0}`,
		"unknown format":        `{"model":{"provider":"a","name":"m"},"responseFormat":"XML"}`,
		"schema without schema": `{"model":{"provider":"a","name":"m"},"responseFormat":"JSON_SCHEMA"}`,
		"too many repairs":      `{"model":{"provider":"a","name":"m"},"repairAttempts":6}`,
	}
	for name, p := range bad {
		if _, err := run(t, fakeModels{}, p, `{"text":"x"}`); code(err) != chain.CodeNodeInvalid {
			t.Errorf("%s: error %v (code %q), want %s", name, err, code(err), chain.CodeNodeInvalid)
		}
	}
	if _, err := run(t, fakeModels{"a": &fakeModel{}}, params, `{"other":"x"}`); code(err) != chain.CodeNodeInvalid {
		t.Errorf("an item without the input field = %v", err)
	}
	node, err := chain.NewLLM(chain.Config{Models: fakeModels{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := node.Execute(t.Context(), execution.NodeCall{Node: &flow.Node{Type: "orcon.other@1"}}); err == nil {
		t.Error("another node type was executed")
	}
	if _, err := chain.NewLLM(chain.Config{}); err == nil {
		t.Error("a runner without a model resolver was built")
	}
}
