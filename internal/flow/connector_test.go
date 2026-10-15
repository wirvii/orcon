// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

const (
	pinDigest  = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	otherDigit = "sha256:fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
)

// withConnectorNode returns the linear flow with its HTTP node replaced by a
// connector node that carries the given pin.
func withConnectorNode(t *testing.T, pin *flow.ConnectorPin) flow.Document {
	t.Helper()
	doc := load(t, "linear")
	n := node(&doc, "fetch")
	n.Type = "acme.shop.order@1"
	n.Parameters = obj(jsonvalue.Member{Key: "operation", Value: jsonvalue.String("order.create")})
	n.Connector = pin
	return doc
}

// TestNodeConnectorPin: the pin is read, written back unchanged, checked for
// shape and part of the content hash (ADR-0034 section 1).
func TestNodeConnectorPin(t *testing.T) {
	t.Parallel()

	t.Run("round trip", func(t *testing.T) {
		t.Parallel()
		doc := withConnectorNode(t, &flow.ConnectorPin{Version: "1.2.0", Digest: pinDigest})
		back, err := flow.FromValue(doc.Value())
		if err != nil {
			t.Fatalf("FromValue(Value()) failed: %v", err)
		}
		if diff := cmp.Diff(doc.Nodes[1].Connector, back.Nodes[1].Connector); diff != "" {
			t.Errorf("the pin changed on the way (-before +after):\n%s", diff)
		}
	})

	t.Run("the pin is written as a connector member", func(t *testing.T) {
		t.Parallel()
		doc := withConnectorNode(t, &flow.ConnectorPin{Version: "1.2.0", Digest: pinDigest})
		nodes, _ := doc.Value().Get("nodes")
		c, ok := nodes.Items()[1].Get("connector")
		if !ok {
			t.Fatal("the node has no connector member")
		}
		if v, _ := c.Get("version"); !jsonvalue.Equal(v, jsonvalue.String("1.2.0")) {
			t.Errorf("connector.version = %v", v)
		}
		if _, ok := nodes.Items()[0].Get("connector"); ok {
			t.Error("a node without a pin gained a connector member")
		}
	})

	t.Run("hash", func(t *testing.T) {
		t.Parallel()
		hash := func(pin *flow.ConnectorPin) flow.Hash {
			doc := withConnectorNode(t, pin)
			h, err := flow.ContentHash(&doc)
			if err != nil {
				t.Fatal(err)
			}
			return h
		}
		base := hash(&flow.ConnectorPin{Version: "1.2.0", Digest: pinDigest})
		for name, pin := range map[string]*flow.ConnectorPin{
			"another version": {Version: "1.2.1", Digest: pinDigest},
			"another digest":  {Version: "1.2.0", Digest: otherDigit},
			"no pin":          nil,
		} {
			if hash(pin) == base {
				t.Errorf("%s: the pin did not change the content hash", name)
			}
		}
	})

	t.Run("shape", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name string
			pin  flow.ConnectorPin
			path string
		}{
			{"a version that is not SemVer", flow.ConnectorPin{Version: "1.2", Digest: pinDigest}, "/nodes/1/connector/version"},
			{"a version with a leading zero", flow.ConnectorPin{Version: "01.2.0", Digest: pinDigest}, "/nodes/1/connector/version"},
			{"a digest without the algorithm", flow.ConnectorPin{Version: "1.2.0", Digest: strings.TrimPrefix(pinDigest, "sha256:")}, "/nodes/1/connector/digest"},
			{"an upper-case digest", flow.ConnectorPin{Version: "1.2.0", Digest: strings.ToUpper(pinDigest)}, "/nodes/1/connector/digest"},
			{"a short digest", flow.ConnectorPin{Version: "1.2.0", Digest: "sha256:abc"}, "/nodes/1/connector/digest"},
		}
		for _, tt := range tests {
			doc := withConnectorNode(t, &tt.pin)
			r := flow.Validate(&doc)
			found := false
			for _, issue := range r.Errors() {
				found = found || (issue.Path == tt.path && issue.Code == flow.CodeValue)
			}
			if !found {
				t.Errorf("%s: no VALUE error at %s; got %v", tt.name, tt.path, r.Errors())
			}
		}
	})

	t.Run("accepted versions", func(t *testing.T) {
		t.Parallel()
		for _, v := range []string{"0.0.1", "1.2.0", "10.20.30", "1.0.0-rc.1", "1.0.0-alpha+001", "1.0.0+20130313144700", "2.0.0-x.7.z.92"} {
			doc := withConnectorNode(t, &flow.ConnectorPin{Version: v, Digest: pinDigest})
			if r := flow.Validate(&doc); r.HasErrors() {
				t.Errorf("version %q rejected: %v", v, r.Errors())
			}
		}
	})

	t.Run("only connector nodes carry a pin", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "linear")
		node(&doc, "fetch").Connector = &flow.ConnectorPin{Version: "1.2.0", Digest: pinDigest}
		r := flow.Validate(&doc)
		if !r.Has(flow.CodeConnector, flow.SeverityError) {
			t.Errorf("a pin on orcon.http@1 was accepted: %v", r.Issues)
		}
		loop := load(t, "loop")
		for i := range loop.Nodes {
			if strings.HasPrefix(loop.Nodes[i].Type, "orcon.loop") {
				loop.Nodes[i].Connector = &flow.ConnectorPin{Version: "1.2.0", Digest: pinDigest}
			}
		}
		if r := flow.Validate(&loop); !r.Has(flow.CodeConnector, flow.SeverityError) {
			t.Errorf("a pin on a type the format defines was accepted: %v", r.Issues)
		}
	})

	t.Run("a document without pins reads and writes as before", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "linear")
		back, err := flow.FromValue(doc.Value())
		if err != nil {
			t.Fatal(err)
		}
		for i := range back.Nodes {
			if back.Nodes[i].Connector != nil {
				t.Errorf("node %s gained a pin", back.Nodes[i].ID)
			}
		}
	})
}

func TestConnectorIDOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		nodeType string
		want     string
		ok       bool
	}{
		{"acme.shop.order@1", "acme.shop", true},
		{"acme.shop.orderCreated@12", "acme.shop", true},
		{"orcon.http@1", "", false},
		{"orcon.start@1", "", false},
		{"nonsense", "", false},
	}
	for _, tt := range tests {
		got, ok := flow.ConnectorIDOf(tt.nodeType)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ConnectorIDOf(%q) = %q, %v, want %q, %v", tt.nodeType, got, ok, tt.want, tt.ok)
		}
	}
}

// resolver is a node-type catalog fake that answers for connector nodes.
type resolver struct {
	resolve func(n *flow.Node) flow.NodeResolution
}

func (r resolver) Lookup(_ context.Context, nodeType string) (flow.NodeType, bool) {
	if strings.HasPrefix(nodeType, "orcon.") {
		return flow.NodeType{Purity: flow.PurityPure}, true
	}
	return flow.NodeType{}, false
}

func (r resolver) ResolveNode(_ context.Context, n *flow.Node) flow.NodeResolution {
	if strings.HasPrefix(n.Type, "orcon.") {
		return flow.NodeResolution{Type: flow.NodeType{Purity: flow.PurityPure}, Known: true}
	}
	return r.resolve(n)
}

// TestPublishLevel5ConnectorPins: what the catalog says about a pin decides
// whether the flow validates, and the issue names the node and the reason.
func TestPublishLevel5ConnectorPins(t *testing.T) {
	t.Parallel()
	doc := withConnectorNode(t, &flow.ConnectorPin{Version: "1.2.0", Digest: pinDigest})
	var seen struct {
		nodeType  string
		pin       *flow.ConnectorPin
		operation string
	}
	catalog := resolver{resolve: func(n *flow.Node) flow.NodeResolution {
		seen.nodeType, seen.pin, seen.operation = n.Type, n.Connector, flow.SelectionOf(n).Operation
		return flow.NodeResolution{Known: true, Type: flow.NodeType{Purity: flow.PurityRead}}
	}}
	if r := flow.ValidateWithTypes(t.Context(), &doc, catalog); r.HasErrors() {
		t.Fatalf("a usable pin was refused: %v", r.Errors())
	}
	if seen.nodeType != "acme.shop.order@1" || seen.operation != "order.create" || seen.pin == nil || seen.pin.Version != "1.2.0" {
		t.Errorf("the catalog was asked about %+v", seen)
	}

	t.Run("a pin problem is a CONNECTOR error naming the node", func(t *testing.T) {
		t.Parallel()
		blocked := resolver{resolve: func(*flow.Node) flow.NodeResolution {
			return flow.NodeResolution{Known: true, Type: flow.NodeType{Purity: flow.PurityRead}, Problems: []string{"uses acme.shop version 1.2.0, which is revoked"}}
		}}
		r := flow.ValidateWithTypes(t.Context(), &doc, blocked)
		errs := r.Errors()
		if len(errs) != 1 || errs[0].Code != flow.CodeConnector || errs[0].Path != "/nodes/1/connector" ||
			!strings.Contains(errs[0].Message, `node "fetch"`) || !strings.Contains(errs[0].Message, "revoked") {
			t.Errorf("errors = %v", errs)
		}
	})

	t.Run("a type no package defines is a V10 error", func(t *testing.T) {
		t.Parallel()
		none := resolver{resolve: func(*flow.Node) flow.NodeResolution { return flow.NodeResolution{} }}
		if r := flow.ValidateWithTypes(t.Context(), &doc, none); !r.Has(flow.CodeV10, flow.SeverityError) {
			t.Errorf("errors = %v", r.Errors())
		}
	})

	t.Run("an unknown operation is a V10 error at the parameter", func(t *testing.T) {
		t.Parallel()
		noOp := resolver{resolve: func(*flow.Node) flow.NodeResolution {
			return flow.NodeResolution{Known: true, Selection: flow.SelectionUnknown, Operations: []string{"order.create"}}
		}}
		r := flow.ValidateWithTypes(t.Context(), &doc, noOp)
		errs := r.Errors()
		if len(errs) != 1 || errs[0].Code != flow.CodeV10 || errs[0].Path != "/nodes/1/parameters/operation" {
			t.Errorf("errors = %v", errs)
		}
	})
}

func TestPublishWithAppliesLevel5(t *testing.T) {
	t.Parallel()
	doc := withConnectorNode(t, &flow.ConnectorPin{Version: "1.2.0", Digest: pinDigest})
	blocked := resolver{resolve: func(*flow.Node) flow.NodeResolution {
		return flow.NodeResolution{Known: true, Type: flow.NodeType{Purity: flow.PurityRead}, Problems: []string{"is not granted"}}
	}}
	f := flow.NewFlow(doc.ID)
	if _, err := f.SaveDraft(0, doc.Value()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Publish(); err != nil {
		t.Errorf("Publish() without a catalog failed: %v", err)
	}
	g := flow.NewFlow(doc.ID)
	if _, err := g.SaveDraft(0, doc.Value()); err != nil {
		t.Fatal(err)
	}
	if _, err := g.PublishWith(t.Context(), blocked); err == nil || !strings.Contains(err.Error(), "is not granted") {
		t.Errorf("PublishWith() = %v, want the pin problem", err)
	}
}
