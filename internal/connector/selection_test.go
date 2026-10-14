// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// singleOperationManifest is the reference connector with only order.get, to
// show that a node of a one-operation type must still select it.
func singleOperationManifest(t *testing.T) string {
	t.Helper()
	start := strings.Index(referenceManifest, ",\n        {\n          \"name\": \"order.create\"")
	end := strings.Index(referenceManifest, "\n      ],\n      \"parameters\"")
	if start < 0 || end < start {
		t.Fatal("the reference manifest changed shape; update singleOperationManifest")
	}
	return strings.ReplaceAll(referenceManifest[:start]+referenceManifest[end:], `1.2.0`, "1.3.0")
}

// publishErrors publishes the flow and returns the validation errors, nil when
// it published.
func publishErrors(t *testing.T, types flow.NodeTypes, text string) []flow.Issue {
	t.Helper()
	f := newFlow(t, text)
	_, err := f.PublishWith(t.Context(), types)
	if err == nil {
		return nil
	}
	var ve *flow.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("PublishWith() error = %v, want a *flow.ValidationError", err)
	}
	return ve.Report.Errors()
}

// TestOperationSelection: a package action node runs exactly the operation it
// selects, and the contract level 5 checks is that operation's, never the
// riskiest one of the node (SPEC-003 R-CON-09, rules S-1 to S-4).
func TestOperationSelection(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	rec := w.install("1.2.0")
	w.grant(w.orgA, identity.ProjectID{}, rec)
	single, err := w.registry.Install(t.Context(), buildOK(t, referenceFiles(singleOperationManifest(t))).Archive(), w.admin)
	if err != nil {
		t.Fatal(err)
	}
	w.grant(w.orgA, identity.ProjectID{}, single)
	types := w.registry.Catalog(w.orgA, w.project, &fakeCore{})
	pin := pinText("1.2.0", rec.Digest)

	t.Run("the contract is the one of the selected operation", func(t *testing.T) {
		t.Parallel()
		cat := w.registry.Catalog(w.orgA, w.project, nil)
		p := &flow.ConnectorPin{Version: "1.2.0", Digest: rec.Digest}
		for _, tt := range []struct {
			operation string
			purity    flow.Purity
			level     flow.SafetyLevel
		}{
			{"order.get", flow.PurityRead, ""},
			{"order.create", flow.PurityEffect, flow.SafetyIdempotencyKey},
			{"order.update", flow.PurityEffect, flow.SafetyReconcile},
			{"order.send", flow.PurityEffect, flow.SafetyUnsafe},
		} {
			res := cat.ResolveNode(t.Context(), packageNode(t, "acme.shop.order@1", p, tt.operation))
			if !res.Known || res.Selection != flow.SelectionOK || len(res.Problems) != 0 || res.Type.Purity != tt.purity {
				t.Errorf("%s: %+v", tt.operation, res)
				continue
			}
			if tt.level != "" && (res.Type.Contract == nil || res.Type.Contract.Level != tt.level) {
				t.Errorf("%s: contract = %+v, want level %s", tt.operation, res.Type.Contract, tt.level)
			}
			if tt.level == "" && res.Type.Contract != nil {
				t.Errorf("%s: a read operation got the contract %+v", tt.operation, res.Type.Contract)
			}
		}
	})

	t.Run("a read operation publishes although the node has an unsafe one", func(t *testing.T) {
		t.Parallel()
		if errs := publishErrors(t, types, flowText(pin, `{ "operation": "order.get" }`)); errs != nil {
			t.Errorf("errors = %v", errs)
		}
	})

	failures := []struct {
		name   string
		pin    string
		params string
		want   []string
	}{
		{"no selector", pin, `{ "orderId": "1" }`, []string{`node "n1"`, `"operation"`, "order.get, order.create, order.update, order.send"}},
		{"an empty parameter object", pin, `{}`, []string{`node "n1"`, `"operation"`}},
		{"an expression", pin, `{ "operation": "={{ $json.op }}" }`, []string{`node "n1"`, "expression", "literal"}},
		{"a value that is not a string", pin, `{ "operation": 7 }`, []string{`node "n1"`, "literal"}},
		{"an operation the version does not have", pin, `{ "operation": "order.refund" }`, []string{`node "n1"`, `"order.refund"`, "order.get, order.create"}},
		{"the resource without the verb", pin, `{ "operation": "order" }`, []string{`node "n1"`, `"order"`}},
		{"no selector on a type with one operation", pinText("1.3.0", single.Digest), `{}`, []string{`node "n1"`, `"operation"`, "has: order.get"}},
	}
	for _, tt := range failures {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			errs := publishErrors(t, types, flowText(tt.pin, tt.params))
			if len(errs) != 1 {
				t.Fatalf("errors = %v, want exactly one", errs)
			}
			e := errs[0]
			if e.Code != flow.CodeV10 || e.Path != "/nodes/1/parameters/operation" {
				t.Errorf("issue = %+v, want a V10 error at /nodes/1/parameters/operation", e)
			}
			for _, want := range tt.want {
				if !strings.Contains(e.Message, want) {
					t.Errorf("message %q does not mention %q", e.Message, want)
				}
			}
		})
	}

	t.Run("a single operation can be selected", func(t *testing.T) {
		t.Parallel()
		if errs := publishErrors(t, types, flowText(pinText("1.3.0", single.Digest), `{ "operation": "order.get" }`)); errs != nil {
			t.Errorf("errors = %v", errs)
		}
	})

	t.Run("a trigger selects no operation", func(t *testing.T) {
		t.Parallel()
		cat := w.registry.Catalog(w.orgA, w.project, nil)
		p := &flow.ConnectorPin{Version: "1.2.0", Digest: rec.Digest}
		with := cat.ResolveNode(t.Context(), packageNode(t, "acme.shop.orderCreated@1", p, "order.get"))
		if !with.Known || with.Selection != flow.SelectionOnTrigger || len(with.Problems) != 0 {
			t.Errorf("a trigger with a selector: %+v", with)
		}
		without := cat.ResolveNode(t.Context(), packageNode(t, "acme.shop.orderCreated@1", p, ""))
		if !without.Known || without.Selection != flow.SelectionOK || without.Type.Purity != flow.PurityRead || without.Type.Contract != nil {
			t.Errorf("a trigger without a selector: %+v", without)
		}

		text := strings.Replace(flowText(pin, `{ "operation": "order.get" }`), "acme.shop.order@1", "acme.shop.orderCreated@1", 1)
		errs := publishErrors(t, types, text)
		if len(errs) != 1 || errs[0].Code != flow.CodeV10 || errs[0].Path != "/nodes/1/parameters/operation" ||
			!strings.Contains(errs[0].Message, "trigger node types select no operation") {
			t.Errorf("errors = %v", errs)
		}
		if strings.Contains(fmt.Sprint(errs), "does not have") {
			t.Errorf("a trigger got the unknown-operation message: %v", errs)
		}
	})

	t.Run("a node without a pin still has its selection judged", func(t *testing.T) {
		t.Parallel()
		cat := w.registry.Catalog(w.orgA, w.project, nil)
		res := cat.ResolveNode(t.Context(), packageNode(t, "acme.shop.order@1", nil, ""))
		if res.Selection != flow.SelectionMissing || len(res.Problems) != 1 || !strings.Contains(res.Problems[0], "no connector pin") {
			t.Errorf("%+v", res)
		}
	})
}

// TestReservedParameterNames: a manifest that declares a parameter called
// operation or resource does not install (ADR-0029 section 4, Amendment 8).
func TestReservedParameterNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"operation", "resource"} {
		text := mutate(t, `"parameters": { "orderId": { "type": "STRING", "required": true } }`,
			fmt.Sprintf(`"parameters": { "orderId": { "type": "STRING", "required": true }, %q: { "type": "STRING" } }`, name))
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			wantIssue(t, text, connector.RuleIdentity, "/nodes/0/parameters/"+name, "reserved", name)

			// Neither a package can be built, nor a built-in connector registered,
			// so the manifest never reaches the installed store.
			if _, err := connector.BuildPackage(referenceFiles(text), connector.Options{}); err == nil {
				t.Error("a package with a reserved parameter name was built")
			}
			w := newWorld(t)
			if _, err := w.registry.RegisterBuiltin(t.Context(), []byte(text)); err == nil {
				t.Error("a manifest with a reserved parameter name was registered")
			}
			if len(w.store.packages) != 0 {
				t.Error("a refused manifest left a record")
			}
		})
	}
	t.Run("an official connector is bound too", func(t *testing.T) {
		t.Parallel()
		text := mutate(t, `"parameters": { "orderId": { "type": "STRING", "required": true } }`,
			`"parameters": { "resource": { "type": "STRING" } }`)
		if _, err := connector.Parse([]byte(text), connector.Options{Official: true}); err == nil {
			t.Error("Parse() accepted a reserved name for an official connector")
		}
	})
	t.Run("other names, and similar ones, are free", func(t *testing.T) {
		t.Parallel()
		readOK(t, mutate(t, `"parameters": { "orderId": { "type": "STRING", "required": true } }`,
			`"parameters": { "orderId": { "type": "STRING" }, "operations": { "type": "STRING" }, "Resource": { "type": "STRING" }, "resourceId": { "type": "STRING" } }`))
	})
}

// TestCompositeCatalog: the catalog answers for every class of node type
// (SPEC-003 Amendment 8, section 8-5).
func TestCompositeCatalog(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	rec := w.install("1.2.0")
	w.grant(w.orgA, identity.ProjectID{}, rec)
	pin := &flow.ConnectorPin{Version: "1.2.0", Digest: rec.Digest}

	httpNode := func(t *testing.T, params string) *flow.Node {
		t.Helper()
		return &flow.Node{ID: "n1", Type: "orcon.http@1", Parameters: value(t, params)}
	}

	t.Run("a core type is resolved by the core types, without pin or selection", func(t *testing.T) {
		t.Parallel()
		core := &fakeCore{}
		cat := w.registry.Catalog(w.orgA, w.project, core)
		get := cat.ResolveNode(t.Context(), httpNode(t, `{ "method": "GET", "operation": "x.y" }`))
		if !get.Known || get.Type.Purity != flow.PurityRead || len(get.Problems) != 0 || get.Selection != flow.SelectionOK {
			t.Errorf("GET: %+v", get)
		}
		for _, params := range []string{`{ "method": "POST" }`, `{ "method": "={{ $json.m }}" }`, `{}`} {
			res := cat.ResolveNode(t.Context(), httpNode(t, params))
			if !res.Known || res.Type.Purity != flow.PurityEffect || res.Type.Contract == nil || res.Type.Contract.Level != flow.SafetyUnsafe {
				t.Errorf("%s: %+v", params, res)
			}
		}
		if len(core.resolved) != 4 {
			t.Errorf("the core types resolved %v, want 4 nodes", core.resolved)
		}
	})

	t.Run("a core type with a pin is not judged on the pin here", func(t *testing.T) {
		t.Parallel()
		cat := w.registry.Catalog(w.orgA, w.project, &fakeCore{})
		n := httpNode(t, `{ "method": "GET" }`)
		n.Connector = pin
		if res := cat.ResolveNode(t.Context(), n); !res.Known || len(res.Problems) != 0 {
			t.Errorf("%+v (a pin on a core type is a level 1 error, not a catalog one)", res)
		}
	})

	t.Run("lookup delegates to the core types for a type no package owns", func(t *testing.T) {
		t.Parallel()
		core := &fakeCore{}
		cat := w.registry.Catalog(w.orgA, w.project, core)
		nt, ok := cat.Lookup(t.Context(), "orcon.http@1")
		if !ok || nt.Purity != flow.PurityEffect {
			t.Errorf("Lookup(orcon.http@1) = %+v, %v; the palette shows the worst case", nt, ok)
		}
		if len(core.lookups) != 1 || core.lookups[0] != "orcon.http@1" {
			t.Errorf("the core types were asked %v", core.lookups)
		}
		order, ok := cat.Lookup(t.Context(), "acme.shop.order@1")
		if !ok || order.Contract == nil || order.Contract.Level != flow.SafetyUnsafe {
			t.Errorf("Lookup(order) = %+v, %v; a package type shows its riskiest operation", order, ok)
		}
		if len(core.lookups) != 1 {
			t.Errorf("a package type reached the core types: %v", core.lookups)
		}
	})

	t.Run("an unknown type is unknown", func(t *testing.T) {
		t.Parallel()
		cat := w.registry.Catalog(w.orgA, w.project, &fakeCore{})
		for _, nodeType := range []string{"vendor.thing@1", "other.shop.order@1", "acme.shop.missing@1"} {
			res := cat.ResolveNode(t.Context(), &flow.Node{ID: "n1", Type: nodeType, Connector: nil, Parameters: value(t, `{}`)})
			if res.Known && res.Selection == flow.SelectionOK && len(res.Problems) == 0 {
				t.Errorf("%s resolved as known: %+v", nodeType, res)
			}
		}
		text := strings.Replace(flowText("", createOperation), "acme.shop.order@1", "vendor.thing@1", 1)
		errs := publishErrors(t, cat, text)
		if len(errs) != 1 || errs[0].Code != flow.CodeV10 || !strings.Contains(errs[0].Message, "does not know") {
			t.Errorf("errors = %v", errs)
		}
	})

	t.Run("without core types a core type is unknown", func(t *testing.T) {
		t.Parallel()
		cat := w.registry.Catalog(w.orgA, w.project, nil)
		if res := cat.ResolveNode(t.Context(), httpNode(t, `{}`)); res.Known || len(res.Problems) != 0 {
			t.Errorf("%+v", res)
		}
		if _, ok := cat.Lookup(t.Context(), "orcon.http@1"); ok {
			t.Error("Lookup() found a core type that nobody provides")
		}
	})

	t.Run("a flow with a core node and a package node publishes", func(t *testing.T) {
		t.Parallel()
		cat := w.registry.Catalog(w.orgA, w.project, &fakeCore{})
		text := strings.Replace(flowText(pinText("1.2.0", rec.Digest), `{ "operation": "order.get" }`),
			`"type": "orcon.end@1"`, `"type": "orcon.http@1"`, 1)
		if errs := publishErrors(t, cat, text); errs != nil {
			t.Errorf("errors = %v", errs)
		}
	})
}
