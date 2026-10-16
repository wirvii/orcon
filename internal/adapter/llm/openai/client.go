// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package openai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/platform/redact"
)

// DefaultBaseURL is the public OpenAI API root.
const DefaultBaseURL = "https://api.openai.com/v1"

// maxResponseBytes caps one response body. A larger answer is a provider
// problem, not something to buffer.
const maxResponseBytes = 16 << 20

// Config configures a [Provider]. HTTPClient is required: the adapter never
// creates a client of its own, so a request cannot skip the egress policy.
type Config struct {
	// BaseURL is the API root, without a trailing slash. Empty means
	// [DefaultBaseURL].
	BaseURL string
	// APIKey is sent as a bearer token. It is never logged or written to an
	// error.
	APIKey redact.Secret
	// HTTPClient performs the requests. Injected by the composition root.
	HTTPClient *http.Client
}

// Provider is a [provider.ModelProvider] for Chat Completions endpoints.
type Provider struct {
	baseURL    string
	apiKey     redact.Secret
	httpClient *http.Client
}

var _ provider.ModelProvider = (*Provider)(nil)

// New builds a Provider. It fails when the base URL is malformed or the HTTP
// client is missing.
func New(cfg Config) (*Provider, error) {
	if cfg.HTTPClient == nil {
		return nil, errors.New("openai provider: HTTPClient is required")
	}
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("openai provider: base URL %q is not an absolute URL", base)
	}
	return &Provider{
		baseURL:    strings.TrimSuffix(base, "/"),
		apiKey:     cfg.APIKey,
		httpClient: cfg.HTTPClient,
	}, nil
}

// Capabilities reports the features of a Chat Completions model. Models are
// not looked up one by one yet, so every model gets the conservative set that
// all Chat Completions endpoints in scope support. Context sizes are unknown
// (zero) until the model catalog arrives with the cost cut (AI2).
func (p *Provider) Capabilities(_ string) provider.Capabilities {
	return provider.Capabilities{
		Tools:         true,
		ParallelTools: true,
		JSONSchema:    true,
	}
}

// Generate performs one Chat Completions call and returns the normalized
// answer. Failures are [*provider.Error] values; a context that ends first
// returns the context error.
func (p *Provider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	body, names, err := encodeRequest(req)
	if err != nil {
		return provider.Response{}, fmt.Errorf("openai generate: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return provider.Response{}, fmt.Errorf("openai generate: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if p.apiKey.IsSet() {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey.Reveal())
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return provider.Response{}, transportError(ctx)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return provider.Response{}, transportError(ctx)
	}
	if resp.StatusCode/100 != 2 {
		return provider.Response{}, decodeError(resp.StatusCode, resp.Header, raw)
	}
	return decodeResponse(raw, names)
}

// transportError classifies a failure before any status arrived. A canceled
// or expired context is returned as-is; every other transport failure
// (timeouts, resets, refused connections) is transient.
func transportError(ctx context.Context) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return &provider.Error{Class: provider.ClassTransient, Code: "transport", Message: "connection to the provider failed"}
}
