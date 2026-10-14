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

// flowText is a three-node flow whose middle node uses the connector. The pin
// and the node's parameters are filled in by the caller.
func flowText(pin, parameters string) string {
	return fmt.Sprintf(`{
  "format": "orcon/flow@1",
  "id": "01930000-0000-7000-8000-0000000000aa",
  "name": "Order flow",
  "settings": {},
  "variables": [],
  "nodes": [
    { "id": "start", "name": "Start", "element": "startEvent", "type": "orcon.start@1", "parameters": {} },
    { "id": "n1", "name": "Create order", "element": "serviceTask", "type": "acme.shop.order@1", "parameters": %s%s },
    { "id": "end", "name": "End", "element": "endEvent", "type": "orcon.end@1", "parameters": {} }
  ],
  "edges": [
    { "id": "e1", "from": { "node": "start", "port": "main" }, "to": { "node": "n1" }, "kind": "SEQUENCE" },
    { "id": "e2", "from": { "node": "n1", "port": "main" }, "to": { "node": "end" }, "kind": "SEQUENCE" }
  ],
  "editor": {}
}`, parameters, pin)
}

func pinText(version, digest string) string {
	return fmt.Sprintf(`, "connector": { "version": %q, "digest": %q }`, version, digest)
}

func newFlow(t *testing.T, text string) *flow.Flow {
	t.Helper()
	f := flow.NewFlow(flow.ID("01930000-0000-7000-8000-0000000000aa"))
	if _, err := f.SaveDraft(0, value(t, text)); err != nil {
		t.Fatal(err)
	}
	return f
}

const createOperation = `{ "operation": "order.create" }`

// TestPublishLevel5Connector: publishing a flow whose node pins a connector
// version fails unless the version is installed with that digest, granted to
// the project and allowed by the policy, and the error names the node and the
// reason (SPEC-003 E-MAN-2, ADR-0034 section 1).
func TestPublishLevel5Connector(t *testing.T) {
	t.Parallel()
	wrong := "sha256:" + strings.Repeat("1", 64)

	tests := []struct {
		name   string
		setup  func(w *world, rec connector.PackageRecord)
		pin    func(rec connector.PackageRecord) string
		params string
		want   []string
	}{
		{
			name:   "installed, granted and allowed",
			setup:  func(w *world, rec connector.PackageRecord) { w.grant(w.orgA, identity.ProjectID{}, rec) },
			pin:    func(rec connector.PackageRecord) string { return pinText("1.2.0", rec.Digest) },
			params: createOperation,
		},
		{
			name:   "a version that is not installed",
			setup:  func(w *world, rec connector.PackageRecord) { w.grant(w.orgA, identity.ProjectID{}, rec) },
			pin:    func(connector.PackageRecord) string { return pinText("1.3.0", wrong) },
			params: createOperation,
			want:   []string{`node "n1"`, "acme.shop version 1.3.0", "not installed"},
		},
		{
			name:   "a version the organization has not granted",
			setup:  func(*world, connector.PackageRecord) {},
			pin:    func(rec connector.PackageRecord) string { return pinText("1.2.0", rec.Digest) },
			params: createOperation,
			want:   []string{`node "n1"`, "acme.shop version 1.2.0", "has not granted"},
		},
		{
			name: "a version another organization granted",
			setup: func(w *world, rec connector.PackageRecord) {
				w.grant(w.orgB, identity.ProjectID{}, rec)
			},
			pin:    func(rec connector.PackageRecord) string { return pinText("1.2.0", rec.Digest) },
			params: createOperation,
			want:   []string{`node "n1"`, "has not granted"},
		},
		{
			name: "a version the policy blocks",
			setup: func(w *world, rec connector.PackageRecord) {
				w.grant(w.orgA, identity.ProjectID{}, rec)
				w.policy.block("acme.shop", "1.2.0", "the version is revoked")
			},
			pin:    func(rec connector.PackageRecord) string { return pinText("1.2.0", rec.Digest) },
			params: createOperation,
			want:   []string{`node "n1"`, "policy blocks", "the version is revoked"},
		},
		{
			name:   "a digest that is not the installed one",
			setup:  func(w *world, rec connector.PackageRecord) { w.grant(w.orgA, identity.ProjectID{}, rec) },
			pin:    func(connector.PackageRecord) string { return pinText("1.2.0", wrong) },
			params: createOperation,
			want:   []string{`node "n1"`, "digest " + wrong},
		},
		{
			name:   "a connector node without a pin",
			setup:  func(w *world, rec connector.PackageRecord) { w.grant(w.orgA, identity.ProjectID{}, rec) },
			pin:    func(connector.PackageRecord) string { return "" },
			params: createOperation,
			want:   []string{`node "n1"`, "no connector pin"},
		},
		{
			name:   "an operation the version does not have",
			setup:  func(w *world, rec connector.PackageRecord) { w.grant(w.orgA, identity.ProjectID{}, rec) },
			pin:    func(rec connector.PackageRecord) string { return pinText("1.2.0", rec.Digest) },
			params: `{ "operation": "order.refund" }`,
			want:   []string{`node "n1"`, `operation "order.refund"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			rec := w.install("1.2.0")
			tt.setup(w, rec)
			types := coreTypes{w.registry.Catalog(w.orgA, w.project)}
			f := newFlow(t, flowText(tt.pin(rec), tt.params))

			v, err := f.PublishWith(t.Context(), types)
			if len(tt.want) == 0 {
				if err != nil {
					t.Fatalf("PublishWith() failed: %v", err)
				}
				if v.Number != 1 {
					t.Errorf("version number = %d, want 1", v.Number)
				}
				return
			}
			var ve *flow.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("PublishWith() error = %v, want a *flow.ValidationError", err)
			}
			text := fmt.Sprint(ve.Report.Errors())
			for _, w := range tt.want {
				if !strings.Contains(text, w) {
					t.Errorf("the report does not mention %q:\n%s", w, text)
				}
			}
			if _, ok := f.Active(); ok {
				t.Error("a failed publication left a version behind")
			}
		})
	}
}

func TestPublishLevel5ConnectorRollbackAndSnapshot(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	w := newWorld(t)
	rec := w.install("1.2.0")
	w.grant(w.orgA, identity.ProjectID{}, rec)
	types := coreTypes{w.registry.Catalog(w.orgA, w.project)}
	f := newFlow(t, flowText(pinText("1.2.0", rec.Digest), createOperation))

	if _, err := f.SnapshotWith(ctx, types); err != nil {
		t.Fatalf("SnapshotWith() failed while the version is allowed: %v", err)
	}
	if _, err := f.PublishWith(ctx, types); err != nil {
		t.Fatalf("PublishWith() failed: %v", err)
	}
	// The version is revoked later: republishing it, or testing it, must stop.
	w.policy.block("acme.shop", "1.2.0", "revoked after publication")
	if _, err := f.RollbackWith(ctx, 1, types); err == nil || !strings.Contains(err.Error(), "revoked after publication") {
		t.Errorf("RollbackWith() = %v, want it blocked with the reason", err)
	}
	if _, err := f.SnapshotWith(ctx, types); err == nil || !strings.Contains(err.Error(), "revoked after publication") {
		t.Errorf("SnapshotWith() = %v, want it blocked with the reason", err)
	}
	if got := len(f.Versions()); got != 1 {
		t.Errorf("%d versions after a blocked rollback, want 1", got)
	}
}

// TestSelectedOperationDecidesTheContract: the contract that V10 and the
// attention rule check is the one of the operation the node selects.
func TestSelectedOperationDecidesTheContract(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	w := newWorld(t)
	rec := w.install("1.2.0")
	w.grant(w.orgA, identity.ProjectID{}, rec)
	types := coreTypes{w.registry.Catalog(w.orgA, w.project)}
	pin := pinText("1.2.0", rec.Digest)

	attentionOff := func(text string) string {
		return strings.Replace(text, `"type": "acme.shop.order@1", "parameters"`, `"type": "acme.shop.order@1", "attention": { "notify": "OFF" }, "parameters"`, 1)
	}
	attentionMessage := func(operation string) string {
		f := newFlow(t, attentionOff(flowText(pin, fmt.Sprintf(`{ "operation": %q }`, operation))))
		_, err := f.PublishWith(ctx, types)
		var ve *flow.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("PublishWith(%s) = %v, want a validation error", operation, err)
		}
		for _, issue := range ve.Report.Errors() {
			if issue.Code == flow.CodeAttention {
				return issue.Message
			}
		}
		return ""
	}
	// order.get reads, so the node is not an effect node and the block is out of place.
	if got := attentionMessage("order.get"); !strings.Contains(got, "neither waits for a person nor is an effect node") {
		t.Errorf("order.get: attention message = %q", got)
	}
	// order.send writes, so OFF is refused because a person must be told.
	if got := attentionMessage("order.send"); !strings.Contains(got, "cannot be turned OFF") {
		t.Errorf("order.send: attention message = %q", got)
	}
}

// TestNodeConnectorPinIsPartOfTheContentHash: the pin is part of the document,
// so a flow that pins another version has another content hash.
func TestNodeConnectorPinIsPartOfTheContentHash(t *testing.T) {
	t.Parallel()
	digest := "sha256:" + strings.Repeat("a", 64)
	hash := func(text string) flow.Hash {
		doc, err := flow.FromValue(value(t, text))
		if err != nil {
			t.Fatal(err)
		}
		h, err := flow.ContentHash(&doc)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	base := hash(flowText(pinText("1.2.0", digest), createOperation))
	if base != hash(flowText(pinText("1.2.0", digest), createOperation)) {
		t.Error("the same document gave two hashes")
	}
	for name, text := range map[string]string{
		"another version": flowText(pinText("1.2.1", digest), createOperation),
		"another digest":  flowText(pinText("1.2.0", "sha256:"+strings.Repeat("b", 64)), createOperation),
		"no pin":          flowText("", createOperation),
	} {
		if hash(text) == base {
			t.Errorf("%s: the pin did not change the content hash", name)
		}
	}
}

func TestConnectorUsesOfADocument(t *testing.T) {
	t.Parallel()
	digest := "sha256:" + strings.Repeat("a", 64)
	doc, err := flow.FromValue(value(t, flowText(pinText("1.2.0", digest), createOperation)))
	if err != nil {
		t.Fatal(err)
	}
	uses := flow.ConnectorUses(&doc)
	if len(uses) != 1 || uses[0] != (flow.ConnectorUse{ConnectorID: "acme.shop", ConnectorVersion: "1.2.0"}) {
		t.Errorf("ConnectorUses() = %v", uses)
	}
}
