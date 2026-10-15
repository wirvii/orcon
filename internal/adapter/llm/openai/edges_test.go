// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package openai

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
		{name: "empty mode means auto", choice: provider.ToolChoice{}, want: "auto"},
		{name: "none", choice: provider.ToolChoice{Mode: provider.ToolChoiceNone}, want: "none"},
		{name: "required", choice: provider.ToolChoice{Mode: provider.ToolChoiceRequired}, want: "required"},
		{
			name:   "named tool uses the wire name",
			choice: provider.ToolChoice{Mode: provider.ToolChoiceNamed, Name: "crm.lookup"},
			want:   map[string]any{"type": "function", "function": map[string]string{"name": "crm_lookup"}},
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

func TestResponseFormatEncoding(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		format  provider.ResponseFormat
		want    any
		wantErr bool
	}{
		{name: "text sends nothing", format: provider.ResponseFormat{Kind: provider.FormatText}, want: nil},
		{name: "json object mode", format: provider.ResponseFormat{Kind: provider.FormatJSON}, want: map[string]string{"type": "json_object"}},
		{name: "schema without schema fails", format: provider.ResponseFormat{Kind: provider.FormatJSONSchema}, wantErr: true},
		{name: "unknown kind fails", format: provider.ResponseFormat{Kind: "XML"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := encodeResponseFormat(tt.format)
			if (err != nil) != tt.wantErr {
				t.Fatalf("encodeResponseFormat(%s) error = %v, wantErr %v", tt.format.Kind, err, tt.wantErr)
			}
			if !tt.wantErr {
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("encodeResponseFormat(%s) mismatch (-want +got):\n%s", tt.format.Kind, diff)
				}
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

func TestEmptyArgumentsDecodeAsEmptyObject(t *testing.T) {
	t.Parallel()
	body := []byte(`{"choices":[{"message":{"tool_calls":[{"id":"c","type":"function","function":{"name":"ping","arguments":"  "}}]},"finish_reason":"tool_calls"}]}`)
	got, err := decodeResponse(body, provider.ToolNames{})
	if err != nil {
		t.Fatalf("decodeResponse: %v", err)
	}
	if n := got.ToolCalls[0].Arguments.Len(); got.ToolCalls[0].ParseError != "" || n != 0 {
		t.Errorf("blank arguments: parse error %q, members %d; want no error and an empty object", got.ToolCalls[0].ParseError, n)
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
		{name: "seconds", value: "12", want: 12 * time.Second},
		{name: "negative seconds ignored", value: "-4", want: 0},
		{name: "HTTP date in the future", value: now.Add(30 * time.Second).Format(http.TimeFormat), want: 30 * time.Second},
		{name: "HTTP date in the past ignored", value: now.Add(-time.Minute).Format(http.TimeFormat), want: 0},
		{name: "garbage ignored", value: "soon", want: 0},
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

func TestErrorWithNumericCodeUsesType(t *testing.T) {
	t.Parallel()
	err := decodeError(400, http.Header{}, []byte(`{"error":{"message":"bad","type":"invalid_request_error","code":400}}`))
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Code != "invalid_request_error" {
		t.Errorf("decodeError with a numeric code: code = %v, want invalid_request_error", err)
	}
}

func TestErrorWithoutBodyUsesStatusText(t *testing.T) {
	t.Parallel()
	err := decodeError(http.StatusBadGateway, http.Header{}, []byte(`<html>bad gateway</html>`))
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Message != http.StatusText(http.StatusBadGateway) || pe.Class != provider.ClassTransient {
		t.Errorf("decodeError with an HTML body: %v, want the status text and a transient class", err)
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
