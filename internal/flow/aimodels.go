// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// CodeModel marks a model reference whose capabilities do not cover what the
// node asks for, primary model or fallback alike (SPEC-006 R-PRV-05, E-PRV-3).
const CodeModel = "MODEL"

// ModelNodeTypes is implemented by a node-type catalog that knows the
// capabilities of the models a node may name. Validation checks every AI node
// against it at publication. A catalog that does not implement it skips the
// check, as validation does without any catalog.
type ModelNodeTypes interface {
	NodeTypes
	// ModelCapabilities reports what the model named by ref supports. The
	// boolean is false when the catalog does not know the model.
	ModelCapabilities(ctx context.Context, ref ModelRef) (ModelCaps, bool)
}

// ModelRef names a model: the provider as the catalog spells it, the model
// name the provider serves and the credential that holds the provider key
// (ADR-0061 section 5; empty when the node names none).
type ModelRef struct {
	Provider   string
	Name       string
	Credential string
}

// ModelCaps are the features a node may require of a model. It mirrors the
// subset of the provider capabilities that a flow can ask for; the provider
// package owns the full set.
type ModelCaps struct {
	Tools      bool
	JSONSchema bool
	Vision     bool
	Files      bool
}

// modelNodeTypes are the node types whose parameters name models.
var modelNodeTypes = map[string]bool{
	"orcon.ai.llm@1":   true,
	"orcon.ai.agent@1": true,
}

// modelNeeds reads the capabilities a node's parameters require. A tools list
// needs tool calling; responseFormat JSON_SCHEMA needs schema output; images
// and files in input need vision and file support.
func modelNeeds(p jsonvalue.Value) ModelCaps {
	var c ModelCaps
	if t, ok := p.Get("tools"); ok && t.Kind() == jsonvalue.KindArray && t.Len() > 0 {
		c.Tools = true
	}
	if rf, ok := p.Get("responseFormat"); ok {
		if s, _ := rf.Str(); s == "JSON_SCHEMA" {
			c.JSONSchema = true
		}
	}
	if in, ok := p.Get("input"); ok {
		if im, ok := in.Get("images"); ok && im.Kind() == jsonvalue.KindArray && im.Len() > 0 {
			c.Vision = true
		}
		if fl, ok := in.Get("files"); ok && fl.Kind() == jsonvalue.KindArray && fl.Len() > 0 {
			c.Files = true
		}
	}
	return c
}

// missing lists the required features the model does not declare, in a fixed
// order so the messages are stable.
func (c ModelCaps) missing(need ModelCaps) []string {
	var out []string
	check := func(want, have bool, name string) {
		if want && !have {
			out = append(out, name)
		}
	}
	check(need.Tools, c.Tools, "tool calling")
	check(need.JSONSchema, c.JSONSchema, "JSON schema output")
	check(need.Vision, c.Vision, "image input")
	check(need.Files, c.Files, "file input")
	return out
}

// modelRefOf reads one model reference of the form {"provider", "name"}.
func modelRefOf(v jsonvalue.Value) (ModelRef, bool) {
	p, okP := v.Get("provider")
	n, okN := v.Get("name")
	if !okP || !okN {
		return ModelRef{}, false
	}
	prov, _ := p.Str()
	name, _ := n.Str()
	if prov == "" || name == "" {
		return ModelRef{}, false
	}
	var cred string
	if c, ok := v.Get("credential"); ok {
		cred, _ = c.Str()
	}
	return ModelRef{Provider: prov, Name: name, Credential: cred}, true
}

// models runs the model rules of SPEC-006 R-PRV-05 on every AI node: the
// capabilities the node needs must be declared by the primary model and by
// every fallback. A node that needs nothing is not checked against the catalog.
func (v *validator) models() {
	if v.cfg.models == nil {
		return
	}
	for i := range v.doc.Nodes {
		n := &v.doc.Nodes[i]
		if !modelNodeTypes[n.Type] || n.Parameters.IsZero() {
			continue
		}
		need := modelNeeds(n.Parameters)
		if need == (ModelCaps{}) {
			continue
		}
		base := ptr("nodes", i) + "/parameters"
		if m, ok := n.Parameters.Get("model"); ok {
			v.checkModel(base+"/model", n, m, need)
		}
		if fb, ok := n.Parameters.Get("fallbacks"); ok && fb.Kind() == jsonvalue.KindArray {
			for j, item := range fb.Items() {
				v.checkModel(fmt.Sprintf("%s/fallbacks/%d", base, j), n, item, need)
			}
		}
	}
}

// checkModel reports the capabilities a single model reference lacks.
func (v *validator) checkModel(path string, n *Node, raw jsonvalue.Value, need ModelCaps) {
	ref, ok := modelRefOf(raw)
	if !ok {
		v.r.errorf(CodeModel, path, "node %q names a model without a provider and a name", n.ID)
		return
	}
	caps, known := v.cfg.models(ref)
	if !known {
		v.r.errorf(CodeModel, path, "node %q names model %s/%s, which the model catalog does not know, so its capabilities cannot be checked", n.ID, ref.Provider, ref.Name)
		return
	}
	for _, feature := range caps.missing(need) {
		v.r.errorf(CodeModel, path, "node %q needs %s, and model %s/%s does not support it", n.ID, feature, ref.Provider, ref.Name)
	}
}
