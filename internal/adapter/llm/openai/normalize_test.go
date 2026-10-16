// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package openai

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func mustJSON(t *testing.T, s string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(s))
	if err != nil {
		t.Fatalf("jsonvalue.Parse(%s): %v", s, err)
	}
	return v
}

func decodeAny(t *testing.T, b []byte) any {
	t.Helper()
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal(%s): %v", b, err)
	}
	return out
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name) //nolint:gosec // G304: name is a fixed fixture name chosen by the test
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestCanonicalNormalization(t *testing.T) {
	t.Parallel()

	t.Run("request encoding", func(t *testing.T) {
		t.Parallel()
		temp := 0.2
		req := provider.Request{
			Model: "gpt-4o",
			Messages: []provider.Message{
				{Role: provider.RoleSystem, Parts: []provider.Part{{Kind: provider.PartText, Text: "Eres un vendedor."}}},
				{Role: provider.RoleUser, Parts: []provider.Part{{Kind: provider.PartText, Text: "Tienes A-1?"}}},
				{Role: provider.RoleAssistant, Parts: []provider.Part{{
					Kind: provider.PartToolCall,
					ToolCall: provider.ToolCall{
						ID:        "call_a1",
						Name:      "crm.lookup",
						Arguments: mustJSON(t, `{"sku":"A-1"}`),
					},
				}}},
				{Role: provider.RoleTool, Parts: []provider.Part{{
					Kind:       provider.PartToolResult,
					ToolResult: provider.ToolResult{CallID: "call_a1", Name: "crm.lookup", Output: `{"stock":4}`},
				}}},
				{Role: provider.RoleTool, Parts: []provider.Part{{
					Kind:       provider.PartToolResult,
					ToolResult: provider.ToolResult{CallID: "call_x", Name: "crm.lookup", Output: "sku unknown", IsError: true},
				}}},
			},
			Tools: []provider.Tool{{
				Name:        "crm.lookup",
				Description: "Busca un SKU",
				Schema:      mustJSON(t, `{"type":"object","properties":{"sku":{"type":"string"}},"required":["sku"]}`),
			}},
			ToolChoice:      provider.ToolChoice{Mode: provider.ToolChoiceNamed, Name: "crm.lookup"},
			ResponseFormat:  provider.ResponseFormat{Kind: provider.FormatJSONSchema, Schema: mustJSON(t, `{"type":"object"}`), Strict: true},
			MaxOutputTokens: 256,
			Temperature:     &temp,
		}
		body, _, err := encodeRequest(req)
		if err != nil {
			t.Fatalf("encodeRequest: %v", err)
		}
		want := `{
		  "model":"gpt-4o",
		  "messages":[
		    {"role":"system","content":"Eres un vendedor."},
		    {"role":"user","content":"Tienes A-1?"},
		    {"role":"assistant","tool_calls":[{"id":"call_a1","type":"function","function":{"name":"crm_lookup","arguments":"{\"sku\":\"A-1\"}"}}]},
		    {"role":"tool","tool_call_id":"call_a1","content":"{\"stock\":4}"},
		    {"role":"tool","tool_call_id":"call_x","content":"ERROR: sku unknown"}
		  ],
		  "tools":[{"type":"function","function":{"name":"crm_lookup","description":"Busca un SKU","parameters":{"type":"object","properties":{"sku":{"type":"string"}},"required":["sku"]}}}],
		  "tool_choice":{"type":"function","function":{"name":"crm_lookup"}},
		  "response_format":{"type":"json_schema","json_schema":{"name":"final_answer","schema":{"type":"object"},"strict":true}},
		  "max_tokens":256,
		  "temperature":0.2
		}`
		if diff := cmp.Diff(decodeAny(t, []byte(want)), decodeAny(t, body)); diff != "" {
			t.Errorf("encodeRequest mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("text response decoding", func(t *testing.T) {
		t.Parallel()
		got, err := decodeResponse(readFixture(t, "text_response.json"), provider.ToolNames{})
		if err != nil {
			t.Fatalf("decodeResponse: %v", err)
		}
		if got.Text != "Tu pedido sale hoy." {
			t.Errorf("Text = %q, want %q", got.Text, "Tu pedido sale hoy.")
		}
		if got.FinishReason != provider.FinishStop {
			t.Errorf("FinishReason = %s, want %s", got.FinishReason, provider.FinishStop)
		}
		if got.ProviderRequestID != "chatcmpl-rec-001" || got.ModelVersion != "gpt-4o-2024-08-06" {
			t.Errorf("ids = (%q, %q), want (chatcmpl-rec-001, gpt-4o-2024-08-06)", got.ProviderRequestID, got.ModelVersion)
		}
	})

	t.Run("parallel tool calls decoding", func(t *testing.T) {
		t.Parallel()
		names, err := provider.NewToolNames([]provider.Tool{{Name: "crm.lookup"}, {Name: "stock.check"}})
		if err != nil {
			t.Fatalf("NewToolNames: %v", err)
		}
		got, err := decodeResponse(readFixture(t, "parallel_tool_calls_response.json"), names)
		if err != nil {
			t.Fatalf("decodeResponse: %v", err)
		}
		if len(got.ToolCalls) != 2 {
			t.Fatalf("len(ToolCalls) = %d, want 2", len(got.ToolCalls))
		}
		first := got.ToolCalls[0]
		if first.ID != "call_a1" || first.Name != "crm.lookup" || first.ParseError != "" {
			t.Errorf("first call = (%q, %q, parse error %q), want (call_a1, crm.lookup, none)", first.ID, first.Name, first.ParseError)
		}
		if b, _ := jsonvalue.Marshal(first.Arguments); string(b) != `{"sku":"A-1"}` {
			t.Errorf("first call arguments = %s, want {\"sku\":\"A-1\"}", b)
		}
		second := got.ToolCalls[1]
		if second.Name != "stock.check" {
			t.Errorf("second call name = %q, want stock.check", second.Name)
		}
		if second.ParseError == "" || second.RawArguments != `{"sku":` {
			t.Errorf("second call keeps raw arguments %q with parse error %q; want raw kept and an error set", second.RawArguments, second.ParseError)
		}
		if got.FinishReason != provider.FinishToolCalls {
			t.Errorf("FinishReason = %s, want %s", got.FinishReason, provider.FinishToolCalls)
		}
	})

	t.Run("undeclared tool keeps the wire name", func(t *testing.T) {
		t.Parallel()
		got, err := decodeResponse(readFixture(t, "parallel_tool_calls_response.json"), provider.ToolNames{})
		if err != nil {
			t.Fatalf("decodeResponse: %v", err)
		}
		if got.ToolCalls[0].Name != "crm_lookup" {
			t.Errorf("undeclared call name = %q, want the wire name crm_lookup", got.ToolCalls[0].Name)
		}
	})
}

func TestFinishReasonMapping(t *testing.T) {
	t.Parallel()
	refusal := "No puedo."
	tests := []struct {
		name    string
		value   string
		refusal *string
		want    provider.FinishReason
	}{
		{name: "stop", value: "stop", want: provider.FinishStop},
		{name: "length", value: "length", want: provider.FinishLength},
		{name: "tool_calls", value: "tool_calls", want: provider.FinishToolCalls},
		{name: "legacy function_call", value: "function_call", want: provider.FinishToolCalls},
		{name: "content_filter", value: "content_filter", want: provider.FinishContentFilter},
		{name: "refusal field wins over stop", value: "stop", refusal: &refusal, want: provider.FinishRefusal},
		{name: "unknown value is an error", value: "new_reason", want: provider.FinishError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := finishReason(tt.value, tt.refusal); got != tt.want {
				t.Errorf("finishReason(%q) = %s, want %s", tt.value, got, tt.want)
			}
		})
	}
}

func TestUsageMapping(t *testing.T) {
	t.Parallel()
	got, err := decodeResponse(readFixture(t, "text_response.json"), provider.ToolNames{})
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}
	// prompt 120 includes 100 cached; completion 9 has no reasoning.
	want := provider.Usage{Input: 20, Output: 9, CacheRead: 100}
	if got.Usage != want {
		t.Errorf("Usage = %+v, want %+v", got.Usage, want)
	}

	got, err = decodeResponse(readFixture(t, "parallel_tool_calls_response.json"), provider.ToolNames{})
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}
	// completion 30 includes 10 reasoning tokens, reported separately.
	wantReasoning := provider.Usage{Input: 200, Output: 20, Reasoning: 10}
	if got.Usage != wantReasoning {
		t.Errorf("Usage with reasoning = %+v, want %+v", got.Usage, wantReasoning)
	}
}

func TestRefusalFixtureMapsToRefusal(t *testing.T) {
	t.Parallel()
	got, err := decodeResponse(readFixture(t, "refusal_response.json"), provider.ToolNames{})
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}
	if got.FinishReason != provider.FinishRefusal {
		t.Errorf("FinishReason = %s, want %s", got.FinishReason, provider.FinishRefusal)
	}
}

func TestDecodeResponseRejectsEmptyChoices(t *testing.T) {
	t.Parallel()
	_, err := decodeResponse([]byte(`{"choices":[]}`), provider.ToolNames{})
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Class != provider.ClassPermanent {
		t.Fatalf("decodeResponse(no choices) error = %v, want a permanent *provider.Error", err)
	}
}
