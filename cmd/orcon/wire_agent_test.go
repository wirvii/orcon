// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/wirvii/orcon/internal/ai/cost"
	httpdispatch "github.com/wirvii/orcon/internal/connector/dispatch"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// TestModelPricesFollowTheLayers: the agent's price source answers from the
// price layers (ADR-0062 section 3). The embedded table ships no confirmed
// prices, so a model it does not list has an unknown cost, and a layer that
// lists it is used with its version.
func TestModelPricesFollowTheLayers(t *testing.T) {
	ref := flow.ModelRef{Provider: "acme", Name: "model-x"}
	if _, _, ok := newModelPrices().Price(t.Context(), identity.OrgID{}, ref); ok {
		t.Fatal("the embedded table priced a model it does not list")
	}
	layered := modelPrices{layers: cost.Layers{
		InstanceOverride: map[cost.ModelKey]cost.Price{{Provider: "acme", Model: "model-x"}: {Input: 7, Output: 9}},
		Embedded:         cost.Embedded(),
	}}
	p, version, ok := layered.Price(t.Context(), identity.OrgID{}, ref)
	if !ok || p.Input != 7 || p.Output != 9 || version != "instance-override" {
		t.Fatalf("Price() = %+v, %q, %v; want the override", p, version, ok)
	}
}

// TestModelResolverReadsTheStoredCredentialOfTheReference: the API key comes from the credential the model
// reference names, for the project of the execution and the host of the provider; a reference that names none
// is refused before any credential is read.
func TestModelResolverReadsTheStoredCredentialOfTheReference(t *testing.T) {
	org, project := identity.OrgID{1}, identity.ProjectID{2}
	var got httpdispatch.CredentialRequest
	m := modelResolver{credentials: func(_ context.Context, req httpdispatch.CredentialRequest) (httpdispatch.Credential, error) {
		got = req
		return httpdispatch.Credential{Fields: map[string]string{apiKeyField: "stored-key"}}, nil
	}}
	if _, err := m.Model(t.Context(), org, project, flow.ModelRef{Provider: providerOpenAI, Name: "gpt-4o-mini"}); err == nil {
		t.Fatal("a model reference without a credential was resolved")
	}
	if got != (httpdispatch.CredentialRequest{}) {
		t.Fatalf("a credential was read for a reference that names none: %+v", got)
	}
	want := httpdispatch.CredentialRequest{Org: org, Project: project, ID: "cred-1", Host: openAIHost}
	m.endpoint = &modelEndpoint{baseURL: "http://127.0.0.1:1", client: http.DefaultClient}
	if _, err := m.Model(t.Context(), org, project, flow.ModelRef{Provider: providerOpenAI, Name: "gpt-4o-mini", Credential: "cred-1"}); err != nil {
		t.Fatalf("resolve the model: %v", err)
	}
	if got != want {
		t.Fatalf("credential request = %+v, want %+v", got, want)
	}
}
