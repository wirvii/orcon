// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/wirvii/orcon/internal/adapter/egress"
	"github.com/wirvii/orcon/internal/adapter/llm/anthropic"
	"github.com/wirvii/orcon/internal/adapter/llm/openai"
	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/ai/provider"
	httpdispatch "github.com/wirvii/orcon/internal/connector/dispatch"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/redact"
)

// Model provider names of the catalog this build reaches.
const (
	providerOpenAI    = "openai"
	providerAnthropic = "anthropic"
)

// apiKeyField is the credential field that holds the API key of a model provider.
const apiKeyField = "apiKey"

// Hosts of the model providers. A credential must be bound to the host of its provider, as it must for any
// other destination (ADR-0043 section 4).
const (
	openAIHost    = "api.openai.com"
	anthropicHost = "api.anthropic.com"
)

// modelEndpoint replaces where the providers are reached: a test points every provider at a scripted server. The
// credential is still resolved and checked against the host of the provider, as in production.
type modelEndpoint struct {
	baseURL string
	client  *http.Client
}

// modelResolver is the model resolver of the composition root (chain.Models).
// Each provider is reached through the egress client, on its own host and over
// https only, and its API key is read from the stored credential the model
// reference names, for the project of the execution (ADR-0061 section 5).
type modelResolver struct {
	client      *egress.Client
	credentials func(ctx context.Context, req httpdispatch.CredentialRequest) (httpdispatch.Credential, error)
	// endpoint is nil in production.
	endpoint *modelEndpoint
}

// Model implements [chain.Models]; the port returns the provider interface.
func (m modelResolver) Model(ctx context.Context, org identity.OrgID, project identity.ProjectID, ref flow.ModelRef) (provider.ModelProvider, error) { //nolint:ireturn // the chain port returns the provider interface
	switch ref.Provider {
	case providerOpenAI:
		key, err := m.apiKey(ctx, org, project, ref, openAIHost)
		if err != nil {
			return nil, err
		}
		cfg := openai.Config{APIKey: redact.New(key), HTTPClient: m.httpClient(openAIHost)}
		if m.endpoint != nil {
			cfg.BaseURL = m.endpoint.baseURL
		}
		return openai.New(cfg)
	case providerAnthropic:
		key, err := m.apiKey(ctx, org, project, ref, anthropicHost)
		if err != nil {
			return nil, err
		}
		cfg := anthropic.Config{APIKey: redact.New(key), HTTPClient: m.httpClient(anthropicHost)}
		if m.endpoint != nil {
			cfg.BaseURL = m.endpoint.baseURL
		}
		return anthropic.New(cfg)
	default:
		return nil, fmt.Errorf("model provider %q is not available in this build", ref.Provider)
	}
}

// httpClient is the client that reaches one provider host.
func (m modelResolver) httpClient(host string) *http.Client {
	if m.endpoint != nil {
		return m.endpoint.client
	}
	return m.client.HTTPClient(modelPolicy(host))
}

// apiKey reads the API key of a provider from the stored credential the model reference names. It runs on
// every call, so a removed share or a credential bound to another host stops the next one.
func (m modelResolver) apiKey(ctx context.Context, org identity.OrgID, project identity.ProjectID, ref flow.ModelRef, host string) (string, error) {
	if ref.Credential == "" {
		return "", fmt.Errorf("the model %s/%s names no credential", ref.Provider, ref.Name)
	}
	cred, err := m.credentials(ctx, httpdispatch.CredentialRequest{Org: org, Project: project, ID: ref.Credential, Host: host})
	if err != nil {
		return "", err
	}
	key := cred.Fields[apiKeyField]
	if key == "" {
		return "", fmt.Errorf("the credential of the model %s/%s has no %s field", ref.Provider, ref.Name, apiKeyField)
	}
	return key, nil
}

// modelPolicy grants one model host over https.
func modelPolicy(host string) egress.Policy {
	return egress.Policy{Domains: []string{host}, Schemes: []string{"https"}}
}

// modelPrices is the price source of the agent (ADR-0062 section 3): the table
// embedded in the binary. Overrides and signed tables join the layers when they
// are configured; a model no layer prices has an unknown cost.
type modelPrices struct{ layers cost.Layers }

// newModelPrices returns the price source of this build.
func newModelPrices() modelPrices {
	return modelPrices{layers: cost.Layers{Embedded: cost.Embedded()}}
}

// Price implements [agent.Prices].
func (p modelPrices) Price(_ context.Context, _ identity.OrgID, ref flow.ModelRef) (cost.Price, string, bool) {
	r, ok := p.layers.Resolve(cost.ModelKey{Provider: ref.Provider, Model: ref.Name})
	return r.Price, r.Version, ok
}
