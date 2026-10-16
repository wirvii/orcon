// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/adapter/egress"
	"github.com/wirvii/orcon/internal/adapter/llm/anthropic"
	"github.com/wirvii/orcon/internal/adapter/llm/openai"
	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/ai/provider"
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

// modelResolver is the model resolver of the composition root (chain.Models).
// Each provider is reached through the egress client, on its own host and over
// https only, and its API key is read from the credential of the same name.
type modelResolver struct {
	client      *egress.Client
	credentials credentialResolver
}

// Model implements [chain.Models]; the port returns the provider interface.
func (m modelResolver) Model(ctx context.Context, org identity.OrgID, ref flow.ModelRef) (provider.ModelProvider, error) { //nolint:ireturn // the chain port returns the provider interface
	switch ref.Provider {
	case providerOpenAI:
		key, err := m.apiKey(ctx, org, providerOpenAI)
		if err != nil {
			return nil, err
		}
		return openai.New(openai.Config{
			APIKey:     redact.New(key),
			HTTPClient: m.client.HTTPClient(modelPolicy("api.openai.com")),
		})
	case providerAnthropic:
		key, err := m.apiKey(ctx, org, providerAnthropic)
		if err != nil {
			return nil, err
		}
		return anthropic.New(anthropic.Config{
			APIKey:     redact.New(key),
			HTTPClient: m.client.HTTPClient(modelPolicy("api.anthropic.com")),
		})
	default:
		return nil, fmt.Errorf("model provider %q is not available in this build", ref.Provider)
	}
}

// apiKey reads the API key of a provider from its credential.
func (m modelResolver) apiKey(ctx context.Context, org identity.OrgID, name string) (string, error) {
	cred, err := m.credentials(ctx, org, name)
	if err != nil {
		return "", err
	}
	key := cred.Fields[apiKeyField]
	if key == "" {
		return "", fmt.Errorf("credential %q has no %s field", name, apiKeyField)
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
