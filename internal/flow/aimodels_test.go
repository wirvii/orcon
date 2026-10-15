// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"context"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// The provider names of these tests are placeholders: the publication rules
// read capabilities, not names, so the flow package never spells a vendor name.
const (
	provA = "provider-a"
	provB = "provider-b"
)

// modelCatalog is a node-type catalog that knows a fixed set of models and
// the capabilities each one declares (SPEC-006 R-PRV-05).
type modelCatalog struct {
	caps map[flow.ModelRef]flow.ModelCaps
}

func (c modelCatalog) Lookup(_ context.Context, nodeType string) (flow.NodeType, bool) {
	if strings.HasPrefix(nodeType, "orcon.") {
		return flow.NodeType{Purity: flow.PurityAgentic}, true
	}
	return flow.NodeType{}, false
}

func (c modelCatalog) ModelCapabilities(_ context.Context, ref flow.ModelRef) (flow.ModelCaps, bool) {
	caps, ok := c.caps[ref]
	return caps, ok
}

// llmDoc returns the linear flow with its "fetch" node turned into an
// orcon.ai.llm@1 node with the given parameters.
func llmDoc(t *testing.T, params jsonvalue.Value) *flow.Document {
	t.Helper()
	doc := load(t, "linear")
	n := node(&doc, "fetch")
	n.Type = "orcon.ai.llm@1"
	n.Parameters = params
	return &doc
}

func modelRef(provider, name string) jsonvalue.Value {
	return obj(
		jsonvalue.Member{Key: "provider", Value: jsonvalue.String(provider)},
		jsonvalue.Member{Key: "name", Value: jsonvalue.String(name)},
	)
}

func publishWith(t *testing.T, doc *flow.Document, cat flow.NodeTypes) error {
	t.Helper()
	f := flow.NewFlow(doc.ID)
	if _, err := f.SaveDraft(0, doc.Value()); err != nil {
		t.Fatal(err)
	}
	_, err := f.PublishWith(t.Context(), cat)
	return err
}

// TestPublishRejectsMissingModelCapability: a node that declares tools cannot
// publish with a model that does not support tool calling.
func TestPublishRejectsMissingModelCapability(t *testing.T) {
	t.Parallel()
	params := obj(
		jsonvalue.Member{Key: "model", Value: modelRef(provA, "plain")},
		jsonvalue.Member{Key: "tools", Value: jsonvalue.Array(obj(jsonvalue.Member{Key: "name", Value: jsonvalue.String("lookup")}))},
	)
	doc := llmDoc(t, params)
	cat := modelCatalog{caps: map[flow.ModelRef]flow.ModelCaps{
		{Provider: provA, Name: "plain"}: {JSONSchema: true},
	}}

	err := publishWith(t, doc, cat)
	if err == nil || !strings.Contains(err.Error(), "tool calling") {
		t.Fatalf("publish error = %v, want a tool-calling capability error", err)
	}

	capable := modelCatalog{caps: map[flow.ModelRef]flow.ModelCaps{
		{Provider: provA, Name: "plain"}: {Tools: true},
	}}
	if err := publishWith(t, doc, capable); err != nil {
		t.Errorf("publish with a capable model failed: %v", err)
	}
}

// TestPublishRejectsFallbackCapabilityMismatch: a fallback that lacks a
// feature the primary model needs fails publication at the fallback.
func TestPublishRejectsFallbackCapabilityMismatch(t *testing.T) {
	t.Parallel()
	params := obj(
		jsonvalue.Member{Key: "model", Value: modelRef(provA, "strong")},
		jsonvalue.Member{Key: "fallbacks", Value: jsonvalue.Array(modelRef(provB, "weak"))},
		jsonvalue.Member{Key: "responseFormat", Value: jsonvalue.String("JSON_SCHEMA")},
	)
	doc := llmDoc(t, params)
	cat := modelCatalog{caps: map[flow.ModelRef]flow.ModelCaps{
		{Provider: provA, Name: "strong"}: {JSONSchema: true},
		{Provider: provB, Name: "weak"}:   {Tools: true},
	}}

	r := flow.ValidateWithTypes(t.Context(), doc, cat)
	var found bool
	for _, issue := range r.Errors() {
		if issue.Code == flow.CodeModel && issue.Path == "/nodes/1/parameters/fallbacks/0" {
			found = true
		}
	}
	if !found {
		t.Fatalf("errors = %v, want a MODEL error at the fallback", r.Errors())
	}
}

// TestPublishModelRulesNeedCatalog: without a model catalog the rule does not
// run, and a node that needs nothing is not checked against the catalog.
func TestPublishModelRulesNeedCatalog(t *testing.T) {
	t.Parallel()
	params := obj(
		jsonvalue.Member{Key: "model", Value: modelRef(provA, "unknown")},
		jsonvalue.Member{Key: "tools", Value: jsonvalue.Array(obj(jsonvalue.Member{Key: "name", Value: jsonvalue.String("lookup")}))},
	)
	doc := llmDoc(t, params)
	if err := publishWith(t, doc, modelCatalog{caps: map[flow.ModelRef]flow.ModelCaps{}}); err == nil || !strings.Contains(err.Error(), "does not know") {
		t.Errorf("publish with an unknown model = %v, want a model-unknown error", err)
	}

	plain := obj(jsonvalue.Member{Key: "model", Value: modelRef(provA, "unknown")})
	if err := publishWith(t, llmDoc(t, plain), modelCatalog{caps: map[flow.ModelRef]flow.ModelCaps{}}); err != nil {
		t.Errorf("publish of a node that needs no capability = %v, want success", err)
	}
}
