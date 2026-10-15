// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package anthropic

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
		req := provider.Request{
			Model: "claude-sonnet-4-5",
			Messages: []provider.Message{
				{Role: provider.RoleSystem, Parts: []provider.Part{{Kind: provider.PartText, Text: "Eres un vendedor."}}},
				{Role: provider.RoleUser, Parts: []provider.Part{{Kind: provider.PartText, Text: "Tienes A-1?"}}},
				{Role: provider.RoleAssistant, Parts: []provider.Part{
					{Kind: provider.PartText, Text: "Reviso."},
					{Kind: provider.PartToolCall, ToolCall: provider.ToolCall{ID: "toolu_a1", Name: "crm.lookup", Arguments: mustJSON(t, `{"sku":"A-1"}`)}},
					{Kind: provider.PartToolCall, ToolCall: provider.ToolCall{ID: "toolu_b2", Name: "stock.check", Arguments: mustJSON(t, `{"sku":"A-1"}`)}},
				}},
				{Role: provider.RoleTool, Parts: []provider.Part{
					{Kind: provider.PartToolResult, ToolResult: provider.ToolResult{CallID: "toolu_a1", Name: "crm.lookup", Output: `{"stock":4}`}},
				}},
				{Role: provider.RoleTool, Parts: []provider.Part{
					{Kind: provider.PartToolResult, ToolResult: provider.ToolResult{CallID: "toolu_b2", Name: "stock.check", Output: "warehouse closed", IsError: true}},
				}},
			},
			Tools: []provider.Tool{
				{Name: "crm.lookup", Description: "Busca un SKU", Schema: mustJSON(t, `{"type":"object","properties":{"sku":{"type":"string"}},"required":["sku"]}`)},
				{Name: "stock.check"},
			},
			ToolChoice: provider.ToolChoice{Mode: provider.ToolChoiceRequired},
		}
		body, _, err := encodeRequest(req)
		if err != nil {
			t.Fatalf("encodeRequest: %v", err)
		}
		// Consecutive tool results merge into one user turn, and the default
		// max_tokens is sent because the API requires it.
		want := `{
		  "model":"claude-sonnet-4-5",
		  "max_tokens":4096,
		  "system":"Eres un vendedor.",
		  "messages":[
		    {"role":"user","content":[{"type":"text","text":"Tienes A-1?"}]},
		    {"role":"assistant","content":[
		      {"type":"text","text":"Reviso."},
		      {"type":"tool_use","id":"toolu_a1","name":"crm_lookup","input":{"sku":"A-1"}},
		      {"type":"tool_use","id":"toolu_b2","name":"stock_check","input":{"sku":"A-1"}}
		    ]},
		    {"role":"user","content":[
		      {"type":"tool_result","tool_use_id":"toolu_a1","content":"{\"stock\":4}"},
		      {"type":"tool_result","tool_use_id":"toolu_b2","content":"warehouse closed","is_error":true}
		    ]}
		  ],
		  "tools":[
		    {"name":"crm_lookup","description":"Busca un SKU","input_schema":{"type":"object","properties":{"sku":{"type":"string"}},"required":["sku"]}},
		    {"name":"stock_check","input_schema":{"type":"object"}}
		  ],
		  "tool_choice":{"type":"any"}
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
		if got.ProviderRequestID != "msg_rec_001" || got.ModelVersion != "claude-sonnet-4-5-20250929" {
			t.Errorf("ids = (%q, %q), want (msg_rec_001, claude-sonnet-4-5-20250929)", got.ProviderRequestID, got.ModelVersion)
		}
	})

	t.Run("parallel tool use decoding", func(t *testing.T) {
		t.Parallel()
		names, err := provider.NewToolNames([]provider.Tool{{Name: "crm.lookup"}, {Name: "stock.check"}})
		if err != nil {
			t.Fatalf("NewToolNames: %v", err)
		}
		got, err := decodeResponse(readFixture(t, "tool_use_valid_response.json"), names)
		if err != nil {
			t.Fatalf("decodeResponse: %v", err)
		}
		if len(got.ToolCalls) != 2 {
			t.Fatalf("len(ToolCalls) = %d, want 2", len(got.ToolCalls))
		}
		if got.ToolCalls[0].Name != "crm.lookup" || got.ToolCalls[0].ID != "toolu_a1" {
			t.Errorf("first call = (%q, %q), want (crm.lookup, toolu_a1)", got.ToolCalls[0].Name, got.ToolCalls[0].ID)
		}
		if got.ToolCalls[1].Name != "stock.check" {
			t.Errorf("second call name = %q, want stock.check", got.ToolCalls[1].Name)
		}
		if b, _ := jsonvalue.Marshal(got.ToolCalls[1].Arguments); string(b) != `{"sku":"A-1","warehouse":"NEI"}` {
			t.Errorf("second call arguments = %s, want the object from the fixture", b)
		}
		if got.FinishReason != provider.FinishToolCalls {
			t.Errorf("FinishReason = %s, want %s", got.FinishReason, provider.FinishToolCalls)
		}
	})
}

func TestToolNameSanitizingOnTheWire(t *testing.T) {
	t.Parallel()
	names, err := provider.NewToolNames([]provider.Tool{{Name: "crm.lookup"}})
	if err != nil {
		t.Fatalf("NewToolNames: %v", err)
	}
	// A call the model made to an undeclared tool keeps its wire name, so the
	// engine can reject it.
	got, err := decodeResponse([]byte(`{"content":[{"type":"tool_use","id":"t","name":"rm_all","input":{}}],"stop_reason":"tool_use"}`), names)
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}
	if got.ToolCalls[0].Name != "rm_all" {
		t.Errorf("undeclared call name = %q, want rm_all", got.ToolCalls[0].Name)
	}
}

func TestFinishReasonMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value string
		want  provider.FinishReason
	}{
		{value: "end_turn", want: provider.FinishStop},
		{value: "stop_sequence", want: provider.FinishStop},
		{value: "max_tokens", want: provider.FinishLength},
		{value: "model_context_window_exceeded", want: provider.FinishLength},
		{value: "tool_use", want: provider.FinishToolCalls},
		{value: "refusal", want: provider.FinishRefusal},
		{value: "pause_turn", want: provider.FinishError},
		{value: "something_new", want: provider.FinishError},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			if got := finishReason(tt.value); got != tt.want {
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
	// input_tokens already excludes the cache fields, so they map one to one.
	want := provider.Usage{Input: 20, Output: 9, CacheRead: 1200, CacheWrite: 300}
	if got.Usage != want {
		t.Errorf("Usage = %+v, want %+v", got.Usage, want)
	}
}

func TestStructuredOutputUnsupportedUntilCut(t *testing.T) {
	t.Parallel()
	req := provider.Request{
		Model:          "claude-sonnet-4-5",
		Messages:       []provider.Message{{Role: provider.RoleUser, Parts: []provider.Part{{Kind: provider.PartText, Text: "x"}}}},
		ResponseFormat: provider.ResponseFormat{Kind: provider.FormatJSONSchema, Schema: mustJSON(t, `{"type":"object"}`)},
	}
	_, _, err := encodeRequest(req)
	if !errors.Is(err, provider.ErrUnsupported) {
		t.Errorf("encodeRequest with JSON_SCHEMA error = %v, want provider.ErrUnsupported", err)
	}
}
