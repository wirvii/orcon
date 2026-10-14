// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector_test

import (
	"errors"
	"strings"
	"testing"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// referenceManifest is a complete, valid manifest of a made-up shop connector.
// It has an action node with one operation of every safety level and a trigger
// node, so negative cases can break one rule at a time.
const referenceManifest = `{
  "format": "orcon/connector@1",
  "id": "acme.shop",
  "version": "1.2.0",
  "kind": "DECLARATIVE",
  "displayName": "Acme Shop",
  "description": "Orders of the Acme Shop service.",
  "publisher": { "name": "Acme", "url": "https://acme.example.com", "contact": "support@acme.example.com" },
  "license": "Apache-2.0",
  "source": { "repository": "https://github.com/acme/orcon-shop", "commit": "0123456789abcdef0123456789abcdef01234567" },
  "compat": { "engineApi": ">=1.0.0 <2.0.0" },
  "nodes": [
    {
      "name": "order",
      "type": "acme.shop.order@1",
      "displayName": "Order",
      "group": "ACTION",
      "operations": [
        { "name": "order.get", "displayName": "Get an order", "purity": "READ", "itemMode": "PER_ITEM", "rateLimit": "api" },
        {
          "name": "order.create", "displayName": "Create an order", "purity": "EFFECT", "itemMode": "PER_ITEM",
          "safety": {
            "effect": "WRITE", "level": "IDEMPOTENCY_KEY",
            "idempotency": { "mode": "PROVIDER_KEY", "location": { "in": "HEADER", "name": "Idempotency-Key" }, "ttl": "PT24H", "scope": "CREDENTIAL" },
            "onUncertain": "RESEND_WITH_KEY", "retrySafe": true, "proofLevel": "DECLARED"
          }
        },
        {
          "name": "order.update", "displayName": "Update an order", "purity": "EFFECT", "itemMode": "PER_ITEM",
          "safety": {
            "effect": "WRITE", "level": "RECONCILE",
            "verification": { "operation": "order.get", "match": "$response.status == $parameter.status", "notFoundMeansFailed": false, "consistency": { "eventualDelay": "PT5S" } },
            "onUncertain": "RECONCILE", "retrySafe": false, "proofLevel": "TESTED"
          }
        },
        {
          "name": "order.send", "displayName": "Send an order", "purity": "EFFECT", "itemMode": "BATCH",
          "safety": {
            "effect": "WRITE", "level": "UNSAFE", "onUncertain": "SUSPEND", "retrySafe": false,
            "batch": { "maxItems": 50, "perItemResults": true },
            "manualCheck": { "description": "Open the order {{ $parameter.orderId }} in the shop and check it was sent.", "url": "https://shop.example.com/orders/{{ $parameter.orderId }}" },
            "proofLevel": "DECLARED"
          }
        }
      ],
      "parameters": { "orderId": { "type": "STRING", "required": true } },
      "outputs": ["main"],
      "sensitiveFields": ["/customer/email"],
      "n8n": { "types": [{ "name": "n8n-nodes-base.acmeShop", "typeVersions": [1, 1.1] }] }
    },
    {
      "name": "orderCreated",
      "type": "acme.shop.orderCreated@1",
      "displayName": "Order created",
      "group": "TRIGGER",
      "trigger": { "family": "POLL", "delivery": "CURSOR_EXACT", "dedupeKeySource": "JSON_PATH", "maxPayload": 1048576 }
    }
  ],
  "credentials": [
    {
      "id": "apiKey", "displayName": "Acme API key",
      "fields": [{ "name": "key", "type": "STRING", "sensitive": true, "required": true }],
      "scheme": { "type": "API_KEY", "in": "HEADER", "name": "X-Api-Key" },
      "test": "order.get"
    }
  ],
  "permissions": {
    "network": { "domains": ["api.shop.example.com", "*.shop.example.com"], "schemes": ["https"] },
    "credentials": ["apiKey"],
    "resources": { "memoryMiB": 64, "timeout": "PT30S", "maxConcurrent": 4 }
  },
  "rateLimits": [{ "name": "api", "key": "CREDENTIAL", "requestsPerSecond": 10, "burst": 20, "respectHeaders": { "retryAfter": "Retry-After" } }],
  "changelog": [{ "version": "1.2.0", "date": "2026-10-01", "notes": ["changelog.1_2_0.orders"] }]
}`

// readOK reads a manifest that must be valid.
func readOK(t *testing.T, text string) connector.Manifest {
	t.Helper()
	m, err := connector.Parse([]byte(text), connector.Options{})
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	return m
}

// mutate replaces one exact piece of the reference manifest and fails if the
// piece is not there, so a test cannot silently stop testing what it claims.
func mutate(t *testing.T, from, to string) string {
	t.Helper()
	if strings.Count(referenceManifest, from) != 1 {
		t.Fatalf("the reference manifest must contain %q exactly once", from)
	}
	return strings.Replace(referenceManifest, from, to, 1)
}

// wantIssue reads text and expects a *ManifestError with an issue of the rule
// at the path whose message contains every fragment.
func wantIssue(t *testing.T, text, rule, path string, fragments ...string) {
	t.Helper()
	_, err := connector.Parse([]byte(text), connector.Options{})
	if err == nil {
		t.Fatalf("Parse() accepted the manifest; want a %s issue at %s", rule, path)
	}
	var me *connector.ManifestError
	if !errors.As(err, &me) {
		t.Fatalf("Parse() error = %v, want a *ManifestError", err)
	}
	for _, issue := range me.Issues {
		if issue.Rule != rule || issue.Path != path {
			continue
		}
		for _, f := range fragments {
			if !strings.Contains(issue.Message, f) {
				t.Errorf("issue %q does not mention %q", issue, f)
			}
		}
		return
	}
	t.Fatalf("no %s issue at %s; issues: %v", rule, path, me.Issues)
}

// value parses JSON text for tests that need a jsonvalue.
func value(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return v
}
