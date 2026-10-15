// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package anthropic

import (
	"errors"
	"net/http"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/ai/provider"
)

func TestToolChoiceEncoding(t *testing.T) {
	t.Parallel()
	names, err := provider.NewToolNames([]provider.Tool{{Name: "crm.lookup"}})
	if err != nil {
		t.Fatalf("NewToolNames: %v", err)
	}
	tests := []struct {
		name    string
		choice  provider.ToolChoice
		want    any
		wantErr bool
	}{
		{name: "empty mode means auto", choice: provider.ToolChoice{}, want: map[string]string{"type": "auto"}},
		{name: "none", choice: provider.ToolChoice{Mode: provider.ToolChoiceNone}, want: map[string]string{"type": "none"}},
		{name: "required maps to any", choice: provider.ToolChoice{Mode: provider.ToolChoiceRequired}, want: map[string]string{"type": "any"}},
		{
			name:   "named tool uses the wire name",
			choice: provider.ToolChoice{Mode: provider.ToolChoiceNamed, Name: "crm.lookup"},
			want:   map[string]string{"type": "tool", "name": "crm_lookup"},
		},
		{name: "named undeclared tool fails", choice: provider.ToolChoice{Mode: provider.ToolChoiceNamed, Name: "rm"}, wantErr: true},
		{name: "unknown mode fails", choice: provider.ToolChoice{Mode: "SOMETIMES"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := encodeToolChoice(tt.choice, names)
			if (err != nil) != tt.wantErr {
				t.Fatalf("encodeToolChoice(%+v) error = %v, wantErr %v", tt.choice, err, tt.wantErr)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("encodeToolChoice(%+v) mismatch (-want +got):\n%s", tt.choice, diff)
				}
			}
		})
	}
}

func TestResponseFormatGate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		kind            provider.ResponseFormatKind
		wantError       bool
		wantUnsupported bool
	}{
		{name: "text is accepted", kind: provider.FormatText},
		{name: "empty kind is text", kind: ""},
		{name: "JSON is unsupported until the cut", kind: provider.FormatJSON, wantError: true, wantUnsupported: true},
		{name: "unknown kind is an error", kind: "XML", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkResponseFormat(provider.ResponseFormat{Kind: tt.kind})
			if (err != nil) != tt.wantError {
				t.Fatalf("checkResponseFormat(%q) error = %v, wantError %v", tt.kind, err, tt.wantError)
			}
			if got := errors.Is(err, provider.ErrUnsupported); got != tt.wantUnsupported {
				t.Errorf("checkResponseFormat(%q) unsupported = %v, want %v", tt.kind, got, tt.wantUnsupported)
			}
		})
	}
}

func TestUnknownRoleIsRejected(t *testing.T) {
	t.Parallel()
	_, _, err := encodeRequest(provider.Request{Messages: []provider.Message{{Role: "ROBOT"}}})
	if err == nil {
		t.Error("encodeRequest with an unknown role: error = nil, want an error")
	}
}

func TestSystemMessagesGoToTopLevelField(t *testing.T) {
	t.Parallel()
	body, _, err := encodeRequest(provider.Request{
		Model: "claude-sonnet-4-5",
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Parts: []provider.Part{{Kind: provider.PartText, Text: "uno"}}},
			{Role: provider.RoleSystem, Parts: []provider.Part{{Kind: provider.PartText, Text: "dos"}}},
			{Role: provider.RoleUser, Parts: []provider.Part{{Kind: provider.PartText, Text: ""}}},
		},
	})
	if err != nil {
		t.Fatalf("encodeRequest: %v", err)
	}
	want := map[string]any{
		"model":      "claude-sonnet-4-5",
		"max_tokens": float64(DefaultMaxTokens),
		"system":     "uno\n\ndos",
		"messages":   []any{},
	}
	if diff := cmp.Diff(want, decodeAny(t, body)); diff != "" {
		t.Errorf("system placement mismatch (-want +got):\n%s", diff)
	}
}

func TestEmptyToolInputDecodesAsEmptyObject(t *testing.T) {
	t.Parallel()
	got, err := decodeResponse([]byte(`{"content":[{"type":"tool_use","id":"t","name":"ping"}],"stop_reason":"tool_use"}`), provider.ToolNames{})
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}
	if got.ToolCalls[0].ParseError != "" || got.ToolCalls[0].Arguments.Len() != 0 {
		t.Errorf("missing input: parse error %q; want an empty object without error", got.ToolCalls[0].ParseError)
	}
}

func TestRetryAfterForms(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "absent", value: "", want: 0},
		{name: "seconds", value: "5", want: 5 * time.Second},
		{name: "negative seconds ignored", value: "-1", want: 0},
		{name: "HTTP date in the future", value: now.Add(20 * time.Second).Format(http.TimeFormat), want: 20 * time.Second},
		{name: "HTTP date in the past ignored", value: now.Add(-time.Minute).Format(http.TimeFormat), want: 0},
		{name: "garbage ignored", value: "later", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := retryAfter(tt.value, now); got != tt.want {
				t.Errorf("retryAfter(%q) = %s, want %s", tt.value, got, tt.want)
			}
		})
	}
}

func TestErrorWithoutBodyUsesStatusText(t *testing.T) {
	t.Parallel()
	err := decodeError(http.StatusServiceUnavailable, http.Header{}, []byte(`upstream down`))
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Message != http.StatusText(http.StatusServiceUnavailable) || pe.Class != provider.ClassTransient {
		t.Errorf("decodeError with a plain body: %v, want the status text and a transient class", err)
	}
}

func TestTruncateKeepsUTF8Whole(t *testing.T) {
	t.Parallel()
	long := make([]byte, 0, maxMessageChars+4)
	for len(long) < maxMessageChars-1 {
		long = append(long, 'a')
	}
	long = append(long, "ñññ"...)
	got := truncate(string(long))
	if len(got) > maxMessageChars || !utf8.ValidString(got) {
		t.Errorf("truncate kept %d bytes or split a rune: valid=%v", len(got), utf8.ValidString(got))
	}
}
