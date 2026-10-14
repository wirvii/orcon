// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/codec/flowjson"
	"github.com/wirvii/orcon/internal/flow"
)

// settingsAndBlocks is the effects-calls-and-settings conformance flow without
// its call activities, which wait for cut C9: it keeps every document setting
// and the retry, timeout, error, concurrency, compensation and attention blocks.
const settingsAndBlocks = `{
  "format": "orcon/flow@1",
  "id": "01930000-0000-7000-8000-000000000007",
  "name": "Effects, compensation and every setting",
  "description": "Uses each document setting and the safety contract of an external write.",
  "settings": {
    "branchMode": "SEQUENTIAL",
    "priority": "HIGH",
    "concurrency": 16,
    "timeouts": { "executionActive": "PT2H" },
    "retention": "ALWAYS",
    "memoryLimit": 268435456,
    "errorFlow": "01930000-0000-7000-8000-000000000101",
    "callDepthLimit": 5,
    "importedFrom": { "sourceVersion": "1.80.0", "reportId": "import-0001" }
  },
  "variables": [
    { "name": "orderId", "scope": "EXECUTION" },
    { "name": "attempts", "scope": "EXECUTION", "accumulator": "MAX" }
  ],
  "nodes": [
    { "id": "start", "name": "Start", "element": "startEvent", "type": "orcon.start@1", "parameters": {} },
    {
      "id": "charge",
      "name": "Charge card",
      "element": "serviceTask",
      "type": "orcon.payments.charge@2",
      "parameters": { "amount": "={{ $json.amount }}" },
      "retry": { "maxAttempts": 4, "initialInterval": "PT1S", "factor": 2, "maxInterval": "PT5M", "respectRetryAfter": true },
      "timeouts": { "attempt": "PT20S", "node": "PT5M" },
      "onError": "ERROR_OUTPUT",
      "concurrency": 4,
      "compensation": { "handler": "refund", "precondition": "={{ $('Charge card').item.json.charged === true }}" },
      "attention": { "notify": "ON" },
      "ext": { "acme.billing": { "ledger": "main" } }
    },
    { "id": "refund", "name": "Refund card", "element": "serviceTask", "type": "orcon.payments.refund@2", "parameters": {} },
    { "id": "lookup", "name": "Look up customer", "element": "serviceTask", "type": "orcon.http@1", "parameters": { "url": "https://example.test/customers" }, "disabled": false },
    { "id": "end", "name": "End", "element": "endEvent", "type": "orcon.end@1", "parameters": {} },
    { "id": "failed", "name": "Failed", "element": "endEvent", "type": "orcon.end@1", "parameters": {} }
  ],
  "edges": [
    { "id": "e1", "from": { "node": "start", "port": "main" }, "to": { "node": "lookup" }, "kind": "SEQUENCE", "order": 1 },
    { "id": "e2", "from": { "node": "lookup", "port": "main" }, "to": { "node": "charge" }, "kind": "SEQUENCE" },
    { "id": "e3", "from": { "node": "charge", "port": "main" }, "to": { "node": "end" }, "kind": "SEQUENCE" },
    { "id": "e4", "from": { "node": "charge", "port": "error" }, "to": { "node": "failed" }, "kind": "SEQUENCE" }
  ],
  "editor": { "viewport": { "zoom": 1 } },
  "ext": { "acme.team": "payments" }
}`

// supportedFlows returns the flows the codec round-trips: the conformance flows
// that avoid later cuts, plus the settings flow above.
func supportedFlows(tb testing.TB) map[string]*flow.Document {
	tb.Helper()
	out := map[string]*flow.Document{}
	for name, doc := range conformanceFlows(tb) {
		if !deferredFlows[name] {
			out[name] = doc
		}
	}
	doc, err := flowjson.Decode([]byte(settingsAndBlocks))
	if err != nil {
		tb.Fatal(err)
	}
	if report := flow.Validate(&doc); report.HasErrors() {
		tb.Fatalf("the settings flow is invalid: %v", report.Errors())
	}
	out["settings-and-blocks"] = &doc
	return out
}

func TestRoundTripSettingsAndBlocks(t *testing.T) {
	doc := supportedFlows(t)["settings-and-blocks"]
	data := roundTrip(t, doc)
	checkStructure(t, data)
}
