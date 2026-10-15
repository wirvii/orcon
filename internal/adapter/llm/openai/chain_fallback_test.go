// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package openai_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/llm/openai"
	"github.com/wirvii/orcon/internal/ai/chain"
	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/redact"
	"github.com/wirvii/orcon/test/mockllm"
)

// resolver maps model references to adapters that talk to mockllm servers.
type resolver struct {
	providers map[flow.ModelRef]provider.ModelProvider
}

func (r resolver) Model(_ context.Context, _ identity.OrgID, ref flow.ModelRef) (provider.ModelProvider, error) { //nolint:ireturn // implements the chain.Models port
	p, ok := r.providers[ref]
	if !ok {
		return nil, fmt.Errorf("no adapter for %s/%s", ref.Provider, ref.Name)
	}
	return p, nil
}

// adapterFor returns an OpenAI adapter that sends to srv.
func adapterFor(t *testing.T, srv *mockllm.Server) provider.ModelProvider { //nolint:ireturn // the test uses the port, not the concrete type
	t.Helper()
	p, err := openai.New(openai.Config{BaseURL: srv.URL() + "/v1", APIKey: redact.New("test-key"), HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// chatReply is a Chat Completions answer with the given text.
func chatReply(content string) mockllm.Reply {
	body, err := json.Marshal(map[string]any{
		"id":      "chatcmpl-test",
		"model":   "test-model",
		"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": content}, "finish_reason": "stop"}},
		"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	})
	if err != nil {
		panic(err)
	}
	return mockllm.Reply{Body: string(body)}
}

func errorReply(status int) mockllm.Reply {
	return mockllm.Reply{Status: status, Body: `{"error":{"message":"provider said no","type":"test","code":"test"}}`}
}

func llmNode(t *testing.T, params map[string]any) *flow.Node {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonvalue.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return &flow.Node{ID: "llm", Name: "llm", Type: chain.LLMNodeType, Parameters: v}
}

func ref(name string) map[string]any {
	return map[string]any{"provider": "openai", "name": name}
}

func input(text string) execution.Batch {
	v, err := jsonvalue.Object(jsonvalue.Member{Key: "text", Value: jsonvalue.String(text)})
	if err != nil {
		panic(err)
	}
	return execution.NewBatch(execution.Item{JSON: v})
}

func run(t *testing.T, runner *chain.LLM, node *flow.Node) (execution.NodeOutput, error) {
	t.Helper()
	return runner.Execute(t.Context(), execution.NodeCall{Node: node, Input: input("hola")})
}

func newRunner(t *testing.T, providers map[flow.ModelRef]provider.ModelProvider) *chain.LLM {
	t.Helper()
	r, err := chain.NewLLM(chain.Config{Models: resolver{providers: providers}})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestFallbackOn429And5xxAnd529: a transient failure of the primary model
// moves the item to the fallback, and the answer records which model gave it.
func TestFallbackOn429And5xxAnd529(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, 529} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			t.Parallel()
			primary := mockllm.New(t, errorReply(status))
			backup := mockllm.New(t, chatReply("respuesta de respaldo"))
			runner := newRunner(t, map[flow.ModelRef]provider.ModelProvider{
				{Provider: "openai", Name: "main"}:   adapterFor(t, primary),
				{Provider: "openai", Name: "backup"}: adapterFor(t, backup),
			})
			node := llmNode(t, map[string]any{
				"model":     ref("main"),
				"fallbacks": []any{ref("backup")},
			})
			out, err := run(t, runner, node)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			item := out.Ports[0].Batch.Item(0).JSON
			if got := textOf(t, item, "model"); got != "openai/backup" {
				t.Errorf("model = %q, want the fallback that answered", got)
			}
			if got := textOf(t, item, "text"); got != "respuesta de respaldo" {
				t.Errorf("text = %q", got)
			}
		})
	}
}

// TestNoFallbackOnValidationOrRefusal: a permanent failure (a rejected request)
// fails the item at once; the fallback receives no request at all.
func TestNoFallbackOnValidationOrRefusal(t *testing.T) {
	t.Parallel()
	primary := mockllm.New(t, errorReply(http.StatusBadRequest))
	backup := mockllm.New(t, chatReply("nunca se pide"))
	runner := newRunner(t, map[flow.ModelRef]provider.ModelProvider{
		{Provider: "openai", Name: "main"}:   adapterFor(t, primary),
		{Provider: "openai", Name: "backup"}: adapterFor(t, backup),
	})
	node := llmNode(t, map[string]any{
		"model":     ref("main"),
		"fallbacks": []any{ref("backup")},
	})
	_, err := run(t, runner, node)
	var ne *execution.NodeError
	if !errors.As(err, &ne) || ne.Code != chain.CodeProviderRejected || ne.Class != execution.ClassPermanent {
		t.Fatalf("Execute() error = %v, want a permanent %s", err, chain.CodeProviderRejected)
	}
	if n := len(backup.Requests()); n != 0 {
		t.Errorf("fallback received %d requests, want none", n)
	}
}

// TestLLMStructuredOutputNative: a JSON schema node against a model that
// supports schema output returns the parsed value under "json".
func TestLLMStructuredOutputNative(t *testing.T) {
	t.Parallel()
	srv := mockllm.New(t, chatReply(`{"city":"Bogota"}`))
	runner := newRunner(t, map[flow.ModelRef]provider.ModelProvider{
		{Provider: "openai", Name: "main"}: adapterFor(t, srv),
	})
	node := llmNode(t, map[string]any{
		"model":          ref("main"),
		"responseFormat": "JSON_SCHEMA",
		"schema":         map[string]any{"type": "object", "required": []any{"city"}},
	})
	out, err := run(t, runner, node)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	item := out.Ports[0].Batch.Item(0).JSON
	city, _ := item.Get("json")
	if v, _ := city.Get("city"); v.IsZero() {
		t.Errorf("item = %v, want the city under json", item)
	}
}

// TestLLMRejectsToolsParameter: tool calls belong to the agent node.
func TestLLMRejectsToolsParameter(t *testing.T) {
	t.Parallel()
	runner := newRunner(t, nil)
	node := llmNode(t, map[string]any{"model": ref("main"), "tools": []any{map[string]any{"name": "x"}}})
	_, err := run(t, runner, node)
	var ne *execution.NodeError
	if !errors.As(err, &ne) || ne.Code != chain.CodeNodeInvalid {
		t.Errorf("Execute() error = %v, want %s", err, chain.CodeNodeInvalid)
	}
}

func textOf(t *testing.T, v jsonvalue.Value, key string) string {
	t.Helper()
	f, ok := v.Get(key)
	if !ok {
		t.Fatalf("item has no %q: %v", key, v)
	}
	s, _ := f.Str()
	return s
}
