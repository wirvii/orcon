// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package anthropic

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/platform/redact"
	"github.com/wirvii/orcon/test/mockllm"
)

const testKey = "sk-ant-test-0123456789"

func newTestProvider(t *testing.T, srv *mockllm.Server) *Provider {
	t.Helper()
	p, err := New(Config{BaseURL: srv.URL() + "/v1", APIKey: redact.New(testKey), HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func textRequest() provider.Request {
	return provider.Request{
		Model: "claude-sonnet-4-5",
		Messages: []provider.Message{
			{Role: provider.RoleUser, Parts: []provider.Part{{Kind: provider.PartText, Text: "hola"}}},
		},
	}
}

func TestGenerateSendsKeyAndVersionHeaders(t *testing.T) {
	t.Parallel()
	srv := mockllm.New(t, mockllm.Reply{Body: string(readFixture(t, "text_response.json"))})
	p := newTestProvider(t, srv)

	resp, err := p.Generate(t.Context(), textRequest())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != "Tu pedido sale hoy." {
		t.Errorf("Text = %q, want the fixture text", resp.Text)
	}

	reqs := srv.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	if reqs[0].Path != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", reqs[0].Path)
	}
	if got := reqs[0].Header.Get("x-api-key"); got != testKey {
		t.Errorf("x-api-key = %q, want the configured key", got)
	}
	if got := reqs[0].Header.Get("anthropic-version"); got != APIVersion {
		t.Errorf("anthropic-version = %q, want %q", got, APIVersion)
	}
	if strings.Contains(string(reqs[0].Body), testKey) {
		t.Error("request body contains the API key, want it only in the header")
	}
}

func TestStatusClassification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		status        int
		header        http.Header
		body          string
		wantClass     provider.Class
		wantCode      string
		wantRetrySecs int
	}{
		{
			name:          "rate limit with Retry-After is transient",
			status:        429,
			header:        http.Header{"Retry-After": []string{"7"}},
			body:          `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`,
			wantClass:     provider.ClassTransient,
			wantCode:      "rate_limit_error",
			wantRetrySecs: 7,
		},
		{
			name:      "529 overloaded is transient",
			status:    529,
			body:      `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
			wantClass: provider.ClassTransient,
			wantCode:  "overloaded_error",
		},
		{
			name:      "server error is transient",
			status:    500,
			body:      `{"type":"error","error":{"type":"api_error","message":"internal"}}`,
			wantClass: provider.ClassTransient,
		},
		{
			name:      "bad key is permanent",
			status:    401,
			body:      `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`,
			wantClass: provider.ClassPermanent,
			wantCode:  "authentication_error",
		},
		{
			name:      "validation error is permanent",
			status:    400,
			body:      `{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens too large"}}`,
			wantClass: provider.ClassPermanent,
			wantCode:  "invalid_request_error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := mockllm.New(t, mockllm.Reply{Status: tt.status, Header: tt.header, Body: tt.body})
			_, err := newTestProvider(t, srv).Generate(t.Context(), textRequest())
			var pe *provider.Error
			if !errors.As(err, &pe) {
				t.Fatalf("Generate error = %v, want *provider.Error", err)
			}
			if pe.Class != tt.wantClass || pe.Status != tt.status {
				t.Errorf("error = class %s status %d, want class %s status %d", pe.Class, pe.Status, tt.wantClass, tt.status)
			}
			if tt.wantCode != "" && pe.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", pe.Code, tt.wantCode)
			}
			if got := int(pe.RetryAfter / time.Second); got != tt.wantRetrySecs {
				t.Errorf("RetryAfter = %ds, want %ds", got, tt.wantRetrySecs)
			}
		})
	}
}

func TestDroppedConnectionIsTransient(t *testing.T) {
	t.Parallel()
	srv := mockllm.New(t, mockllm.Reply{Hangup: true})
	_, err := newTestProvider(t, srv).Generate(t.Context(), textRequest())
	if !provider.IsTransient(err) {
		t.Errorf("Generate after a dropped connection: error = %v, want a transient provider error", err)
	}
}

func TestCancelledContextReturnsContextError(t *testing.T) {
	t.Parallel()
	srv := mockllm.New(t, mockllm.Reply{Delay: 2 * time.Second, Body: string(readFixture(t, "text_response.json"))})
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	_, err := newTestProvider(t, srv).Generate(ctx, textRequest())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Generate with an expired context: error = %v, want context.DeadlineExceeded", err)
	}
	if provider.IsTransient(err) {
		t.Error("an expired caller context was classified as a transient provider failure")
	}
}

func TestNewValidatesConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "missing HTTP client", cfg: Config{}, wantErr: true},
		{name: "relative base URL", cfg: Config{BaseURL: "v1", HTTPClient: http.DefaultClient}, wantErr: true},
		{name: "default base URL", cfg: Config{HTTPClient: http.DefaultClient}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New(%s) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}
