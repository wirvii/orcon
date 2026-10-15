// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package anthropic

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

// DefaultBaseURL is the public Anthropic API root.
const DefaultBaseURL = "https://api.anthropic.com/v1"

// APIVersion is the value of the anthropic-version header this adapter speaks.
const APIVersion = "2023-06-01"

// DefaultMaxTokens is sent when a request leaves MaxOutputTokens at zero,
// because the Messages API requires the field.
const DefaultMaxTokens = 4096

// maxResponseBytes caps one response body.
const maxResponseBytes = 16 << 20

// Config configures a [Provider]. HTTPClient is required: the adapter never
// creates a client of its own, so a request cannot skip the egress policy.
type Config struct {
	// BaseURL is the API root, without a trailing slash. Empty means
	// [DefaultBaseURL].
	BaseURL string
	// APIKey is sent in the x-api-key header. It is never logged or written to
	// an error.
	APIKey redact.Secret
	// HTTPClient performs the requests. Injected by the composition root.
	HTTPClient *http.Client
}

// Provider is a [provider.ModelProvider] for the Messages API.
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
		return nil, errors.New("anthropic provider: HTTPClient is required")
	}
	base := cfg.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("anthropic provider: base URL %q is not an absolute URL", base)
	}
	return &Provider{
		baseURL:    strings.TrimSuffix(base, "/"),
		apiKey:     cfg.APIKey,
		httpClient: cfg.HTTPClient,
	}, nil
}

// Capabilities reports the features of a Messages model. Every model gets the
// conservative set this adapter implements in this cut; the native structured
// output mode and context sizes arrive with the structured-output and cost cuts.
func (p *Provider) Capabilities(_ string) provider.Capabilities {
	return provider.Capabilities{
		Tools:         true,
		ParallelTools: true,
		PromptCache:   true,
	}
}

// Generate performs one Messages call and returns the normalized answer.
// Failures are [*provider.Error] values; a context that ends first returns
// the context error.
func (p *Provider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	body, names, err := encodeRequest(req)
	if err != nil {
		return provider.Response{}, fmt.Errorf("anthropic generate: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return provider.Response{}, fmt.Errorf("anthropic generate: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("anthropic-version", APIVersion)
	if p.apiKey.IsSet() {
		httpReq.Header.Set("x-api-key", p.apiKey.Reveal())
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

// transportError classifies a failure before any status arrived; see the
// openai adapter, which uses the same rule.
func transportError(ctx context.Context) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return &provider.Error{Class: provider.ClassTransient, Code: "transport", Message: "connection to the provider failed"}
}
