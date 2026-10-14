// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestManifestSchemaAcceptsTheReference(t *testing.T) {
	t.Parallel()
	m := readOK(t, referenceManifest)
	if m.ID != "acme.shop" || m.Kind != connector.KindDeclarative || len(m.Nodes) != 2 {
		t.Fatalf("read manifest = %+v", m)
	}
	node, ok := m.NodeByType("acme.shop.order@1")
	if !ok || len(node.Operations) != 4 {
		t.Fatalf("order node = %+v, ok %v", node, ok)
	}
	op, _ := node.OperationByName("order.update")
	c := node.Contract(op)
	if c == nil || c.Verification == nil || c.Verification.OperationPurity != flow.PurityRead {
		t.Errorf("the contract of order.update must resolve its verification purity to READ, got %+v", c)
	}
	if problems := flow.CheckContract(c); len(problems) != 0 {
		t.Errorf("CheckContract(order.update) = %v", problems)
	}
}

// TestManifestSchema covers the shape rules: a closed document, required
// members, types and ranges. Each case breaks one thing in the reference.
func TestManifestSchema(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		rule string
		path string
		frag []string
	}{
		{"an unknown top-level member", mutate(t, `"license": "Apache-2.0",`, `"license": "Apache-2.0", "extra": 1,`), connector.RuleShape, "/extra", []string{"not a member"}},
		{"a missing display name", mutate(t, `"displayName": "Acme Shop",`, ``), connector.RuleShape, "/displayName", []string{"required"}},
		{"another format", mutate(t, `"format": "orcon/connector@1"`, `"format": "orcon/connector@2"`), connector.RuleValue, "/format", []string{"orcon/connector@1"}},
		{"an id without a publisher", mutate(t, `"id": "acme.shop"`, `"id": "shop"`), connector.RuleValue, "/id", []string{"connector id"}},
		{"an id with a capital letter", mutate(t, `"id": "acme.shop"`, `"id": "Acme.shop"`), connector.RuleValue, "/id", nil},
		{"a version that is not SemVer", mutate(t, `"version": "1.2.0",
  "kind"`, `"version": "1.2",
  "kind"`), connector.RuleValue, "/version", []string{"version"}},
		{"an unknown kind", mutate(t, `"kind": "DECLARATIVE"`, `"kind": "SCRIPT"`), connector.RuleEnum, "/kind", []string{"DECLARATIVE", "WASM"}},
		{"a node name with a dot", mutate(t, `"name": "order",`, `"name": "or.der",`), connector.RuleValue, "/nodes/0/name", nil},
		{"a node type of another connector", mutate(t, `"type": "acme.shop.order@1"`, `"type": "other.shop.order@1"`), connector.RuleIdentity, "/nodes/0/type", []string{"acme.shop.order@"}},
		{"a node type without a major", mutate(t, `"type": "acme.shop.order@1"`, `"type": "acme.shop.order@x"`), connector.RuleIdentity, "/nodes/0/type", nil},
		{"an action node without operations", mutate(t, `"group": "ACTION",`, `"group": "TRIGGER",`), connector.RuleShape, "/nodes/0/trigger", []string{"TRIGGER"}},
		{"a trigger with an unknown family", mutate(t, `"family": "POLL"`, `"family": "PUSH"`), connector.RuleEnum, "/nodes/1/trigger/family", []string{"WEBHOOK", "POLL"}},
		{"a sensitive field that is not a JSON Pointer", mutate(t, `"/customer/email"`, `"customer.email"`), connector.RuleValue, "/nodes/0/sensitiveFields/0", []string{"JSON Pointer"}},
		{"a purity outside the closed set", mutate(t, `"purity": "READ"`, `"purity": "READS"`), connector.RuleEnum, "/nodes/0/operations/0/purity", []string{"PURE", "EFFECT"}},
		{"an operation named without a resource", mutate(t, `"name": "order.get"`, `"name": "get"`), connector.RuleValue, "/nodes/0/operations/0/name", nil},
		{"an operation declared twice", mutate(t, `"name": "order.create"`, `"name": "order.get"`), connector.RuleValue, "/nodes/0/operations/1/name", []string{"declared twice"}},
		{"an operation naming a missing rate limit", mutate(t, `"rateLimit": "api"`, `"rateLimit": "slow"`), connector.RuleReference, "/nodes/0/operations/0/rateLimit", []string{"slow"}},
		{"a credential that tests with a missing operation", mutate(t, `"test": "order.get"`, `"test": "order.ping"`), connector.RuleReference, "/credentials/0/test", []string{"order.ping"}},
		{"an API key without a placement", mutate(t, `"in": "HEADER", "name": "X-Api-Key"`, `"name": "X-Api-Key"`), connector.RuleShape, "/credentials/0/scheme/in", []string{"required"}},
		{"an API key scheme with a foreign member", mutate(t, `"name": "X-Api-Key"`, `"name": "X-Api-Key", "grant": "PKCE"`), connector.RuleShape, "/credentials/0/scheme/grant", []string{"not a member"}},
		{"a changelog without the current version", mutate(t, `"version": "1.2.0", "date"`, `"version": "1.1.0", "date"`), connector.RuleReference, "/changelog", []string{"1.2.0"}},
		{"a changelog date that is not a date", mutate(t, `"2026-10-01"`, `"2026-13-45"`), connector.RuleValue, "/changelog/0/date", []string{"calendar date"}},
		{"a WASM ABI on a declarative connector", mutate(t, `"engineApi": ">=1.0.0 <2.0.0"`, `"engineApi": ">=1.0.0 <2.0.0", "wasmAbi": 1`), connector.RuleShape, "/compat/wasmAbi", []string{"WASM"}},
		{"an engine range that is not a range", mutate(t, `">=1.0.0 <2.0.0"`, `"newer than one"`), connector.RuleValue, "/compat/engineApi", nil},
		{"a repository that is not a URL", mutate(t, `"https://github.com/acme/orcon-shop"`, `"acme/orcon-shop"`), connector.RuleValue, "/source/repository", []string{"URL"}},
		{"a rate limit without a limit", mutate(t, `"requestsPerSecond": 10, `, ``), connector.RuleShape, "/rateLimits/0/requestsPerSecond", []string{"requestsPerSecond or concurrency"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantIssue(t, tt.text, tt.rule, tt.path, tt.frag...)
		})
	}
}

func TestManifestSchemaRejectsMalformedJSON(t *testing.T) {
	t.Parallel()
	for name, text := range map[string]string{
		"not JSON":       `{`,
		"not an object":  `[]`,
		"duplicate keys": `{"format":"orcon/connector@1","format":"orcon/connector@1"}`,
	} {
		if _, err := connector.Parse([]byte(text), connector.Options{}); err == nil {
			t.Errorf("%s: Parse() accepted %q", name, text)
		}
	}
}

func TestManifestKindRules(t *testing.T) {
	t.Parallel()
	lib := strings.Replace(referenceManifest, `"kind": "DECLARATIVE"`, `"kind": "LIBRARY"`, 1)
	wantIssue(t, lib, connector.RuleShape, "/nodes", "LIBRARY")
	wasm := strings.Replace(referenceManifest, `"kind": "DECLARATIVE"`, `"kind": "WASM"`, 1)
	wantIssue(t, wasm, connector.RuleShape, "/compat/wasmAbi", "required")
}

// TestSafetyContractRules gives every rule of ADR-0029 section 5 a negative
// case and checks the message names the field and the rule.
func TestSafetyContractRules(t *testing.T) {
	t.Parallel()
	const create = `"safety": {
            "effect": "WRITE", "level": "IDEMPOTENCY_KEY",`
	tests := []struct {
		name string
		text string
		path string
		frag []string
	}{
		{
			"READ_ONLY with a write",
			mutate(t, create, `"safety": {
            "effect": "WRITE", "level": "READ_ONLY",`),
			"/nodes/0/operations/1/safety", []string{"READ_ONLY", "effect READ"},
		},
		{
			"IDEMPOTENCY_KEY without a ttl",
			mutate(t, `"ttl": "PT24H", `, ``),
			"/nodes/0/operations/1/safety", []string{"IDEMPOTENCY_KEY", "ttl"},
		},
		{
			"IDEMPOTENCY_KEY without a location",
			mutate(t, `"location": { "in": "HEADER", "name": "Idempotency-Key" }, `, ``),
			"/nodes/0/operations/1/safety", []string{"IDEMPOTENCY_KEY", "location"},
		},
		{
			"IDEMPOTENCY_KEY without a key mode",
			mutate(t, `"mode": "PROVIDER_KEY"`, `"mode": "NONE"`),
			"/nodes/0/operations/1/safety", []string{"IDEMPOTENCY_KEY", "mode"},
		},
		{
			"RECONCILE without a verification",
			mutate(t, `"verification": { "operation": "order.get", "match": "$response.status == $parameter.status", "notFoundMeansFailed": false, "consistency": { "eventualDelay": "PT5S" } },`, ``),
			"/nodes/0/operations/2/safety", []string{"RECONCILE", "verification"},
		},
		{
			"RECONCILE verified by a write",
			mutate(t, `"verification": { "operation": "order.get"`, `"verification": { "operation": "order.create"`),
			"/nodes/0/operations/2/safety", []string{"order.create", "READ"},
		},
		{
			"UNSAFE that is retry safe",
			mutate(t, `"level": "UNSAFE", "onUncertain": "SUSPEND", "retrySafe": false`, `"level": "UNSAFE", "onUncertain": "SUSPEND", "retrySafe": true`),
			"/nodes/0/operations/3/safety", []string{"UNSAFE", "retrySafe"},
		},
		{
			"UNSAFE that does not suspend",
			mutate(t, `"level": "UNSAFE", "onUncertain": "SUSPEND"`, `"level": "UNSAFE", "onUncertain": "RECONCILE"`),
			"/nodes/0/operations/3/safety", []string{"UNSAFE", "SUSPEND"},
		},
		{
			"RESEND_WITH_KEY on a level without a key",
			mutate(t, `"onUncertain": "RECONCILE", "retrySafe": false, "proofLevel": "TESTED"`, `"onUncertain": "RESEND_WITH_KEY", "retrySafe": false, "proofLevel": "TESTED"`),
			"/nodes/0/operations/2/safety", []string{"RESEND_WITH_KEY", "IDEMPOTENCY_KEY"},
		},
		{
			"an EFFECT operation without a safety contract",
			mutate(t, `"safety": {
            "effect": "WRITE", "level": "UNSAFE", "onUncertain": "SUSPEND", "retrySafe": false,
            "batch": { "maxItems": 50, "perItemResults": true },
            "manualCheck": { "description": "Open the order {{ $parameter.orderId }} in the shop and check it was sent.", "url": "https://shop.example.com/orders/{{ $parameter.orderId }}" },
            "proofLevel": "DECLARED"
          }`, `"rateLimit": "api"`),
			"/nodes/0/operations/3/safety", []string{"EFFECT", "safety contract"},
		},
		{
			"a safety contract on a read operation",
			mutate(t, `"purity": "READ", "itemMode": "PER_ITEM", "rateLimit": "api"`, `"purity": "READ", "itemMode": "PER_ITEM", "rateLimit": "api", "safety": { "effect": "READ", "level": "READ_ONLY", "proofLevel": "DECLARED" }`),
			"/nodes/0/operations/0/safety", []string{"only EFFECT"},
		},
		{
			"a verification operation that does not exist",
			mutate(t, `"verification": { "operation": "order.get"`, `"verification": { "operation": "order.find"`),
			"/nodes/0/operations/2/safety/verification/operation", []string{"order.find"},
		},
		{
			"a missing proof level",
			mutate(t, `"onUncertain": "RESEND_WITH_KEY", "retrySafe": true, "proofLevel": "DECLARED"`, `"onUncertain": "RESEND_WITH_KEY", "retrySafe": true`),
			"/nodes/0/operations/1/safety/proofLevel", []string{"required"},
		},
		{
			"a batch without items",
			mutate(t, `"batch": { "maxItems": 50`, `"batch": { "maxItems": 0`),
			"/nodes/0/operations/3/safety/batch/maxItems", nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := connector.Parse([]byte(tt.text), connector.Options{})
			var me *connector.ManifestError
			if !errors.As(err, &me) {
				t.Fatalf("Parse() error = %v, want a *ManifestError", err)
			}
			for _, issue := range me.Issues {
				if issue.Path != tt.path {
					continue
				}
				for _, f := range tt.frag {
					if !strings.Contains(issue.Message, f) {
						t.Errorf("issue %q does not mention %q", issue, f)
					}
				}
				return
			}
			t.Fatalf("no issue at %s; issues: %v", tt.path, me.Issues)
		})
	}
}

// TestPermissionsSchema covers the network rules of ADR-0029 section 4 and 8.
func TestPermissionsSchema(t *testing.T) {
	t.Parallel()
	domains := `"domains": ["api.shop.example.com", "*.shop.example.com"]`
	tests := []struct {
		name    string
		text    string
		rule    string
		path    string
		frag    []string
		allowed bool
	}{
		{"an exact host and a one-label wildcard", referenceManifest, "", "", nil, true},
		{"a bare wildcard", mutate(t, domains, `"domains": ["*"]`), connector.RulePermission, "/permissions/network/domains/0", []string{"not a host name"}, false},
		{"a wildcard over a top-level domain", mutate(t, domains, `"domains": ["*.com"]`), connector.RulePermission, "/permissions/network/domains/0", []string{"public suffix"}, false},
		{"a wildcard over a registry suffix", mutate(t, domains, `"domains": ["*.co.uk"]`), connector.RulePermission, "/permissions/network/domains/0", []string{"public suffix"}, false},
		{"a wildcard in the middle", mutate(t, domains, `"domains": ["api.*.example.com"]`), connector.RulePermission, "/permissions/network/domains/0", []string{"single leading"}, false},
		{"two wildcards", mutate(t, domains, `"domains": ["*.*.example.com"]`), connector.RulePermission, "/permissions/network/domains/0", nil, false},
		{"an upper-case host", mutate(t, domains, `"domains": ["API.example.com"]`), connector.RulePermission, "/permissions/network/domains/0", nil, false},
		{"a host with a path", mutate(t, domains, `"domains": ["example.com/api"]`), connector.RulePermission, "/permissions/network/domains/0", nil, false},
		{"a single label host", mutate(t, domains, `"domains": ["localhost"]`), connector.RulePermission, "/permissions/network/domains/0", []string{"single label"}, false},
		{"a port out of range", mutate(t, `"schemes": ["https"]`, `"ports": [70000]`), connector.RuleValue, "/permissions/network/ports/0", []string{"port"}, false},
		{"an upper-case scheme", mutate(t, `"schemes": ["https"]`, `"schemes": ["HTTPS"]`), connector.RuleValue, "/permissions/network/schemes/0", []string{"https"}, false},
		{"a credential that is not declared", mutate(t, `"credentials": ["apiKey"]`, `"credentials": ["token"]`), connector.RuleReference, "/permissions/credentials/0", []string{"token"}, false},
		{"a host field that is not a credential field", mutate(t, `"schemes": ["https"]`, `"fromCredential": ["baseUrl"]`), connector.RuleReference, "/permissions/network/fromCredential/0", []string{"baseUrl"}, false},
		{"a timeout that is not a duration", mutate(t, `"timeout": "PT30S"`, `"timeout": "30s"`), connector.RuleValue, "/permissions/resources/timeout", []string{"ISO 8601"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.allowed {
				readOK(t, tt.text)
				return
			}
			wantIssue(t, tt.text, tt.rule, tt.path, tt.frag...)
		})
	}
}

func TestPermissionsAllowSelfHostedServices(t *testing.T) {
	t.Parallel()
	text := mutate(t, `"network": { "domains": ["api.shop.example.com", "*.shop.example.com"], "schemes": ["https"] }`,
		`"network": { "domains": ["localhost"], "schemes": ["https", "http"], "ports": [8080], "allowPrivateNetworks": true, "fromCredential": ["key"] }`)
	m := readOK(t, text)
	n := m.Permissions.Network
	if !n.AllowPrivateNetworks || len(n.Ports) != 1 || len(n.FromCredential) != 1 {
		t.Errorf("network = %+v", n)
	}
}

// TestEnumValuesUpperSnake: every enumerated value of the manifest is
// UPPER_SNAKE, and a lowercase spelling is rejected with a hint (ADR-0029
// section 10).
func TestEnumValuesUpperSnake(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		path string
		want string
	}{
		{"a lowercase safety level", mutate(t, `"level": "UNSAFE"`, `"level": "unsafe"`), "/nodes/0/operations/3/safety/level", `"UNSAFE"`},
		{"a camel case safety level", mutate(t, `"level": "IDEMPOTENCY_KEY"`, `"level": "idempotencyKey"`), "/nodes/0/operations/1/safety/level", `"IDEMPOTENCY_KEY"`},
		{"a lowercase kind", mutate(t, `"kind": "DECLARATIVE"`, `"kind": "declarative"`), "/kind", `"DECLARATIVE"`},
		{"a lowercase group", mutate(t, `"group": "ACTION"`, `"group": "action"`), "/nodes/0/group", `"ACTION"`},
		{"a lowercase purity", mutate(t, `"purity": "READ"`, `"purity": "read"`), "/nodes/0/operations/0/purity", `"READ"`},
		{"a lowercase item mode", mutate(t, `"itemMode": "BATCH"`, `"itemMode": "batch"`), "/nodes/0/operations/3/itemMode", `"BATCH"`},
		{"a lowercase effect", mutate(t, `"effect": "WRITE", "level": "RECONCILE"`, `"effect": "write", "level": "RECONCILE"`), "/nodes/0/operations/2/safety/effect", `"WRITE"`},
		{"a lowercase idempotency mode", mutate(t, `"mode": "PROVIDER_KEY"`, `"mode": "provider_key"`), "/nodes/0/operations/1/safety/idempotency/mode", `"PROVIDER_KEY"`},
		{"a lowercase key location", mutate(t, `"in": "HEADER", "name": "Idempotency-Key"`, `"in": "header", "name": "Idempotency-Key"`), "/nodes/0/operations/1/safety/idempotency/location/in", `"HEADER"`},
		{"a lowercase recovery", mutate(t, `"onUncertain": "SUSPEND"`, `"onUncertain": "suspend"`), "/nodes/0/operations/3/safety/onUncertain", `"SUSPEND"`},
		{"a lowercase proof level", mutate(t, `"proofLevel": "TESTED"`, `"proofLevel": "tested"`), "/nodes/0/operations/2/safety/proofLevel", `"TESTED"`},
		{"a lowercase trigger family", mutate(t, `"family": "POLL"`, `"family": "poll"`), "/nodes/1/trigger/family", `"POLL"`},
		{"a lowercase delivery", mutate(t, `"delivery": "CURSOR_EXACT"`, `"delivery": "cursor_exact"`), "/nodes/1/trigger/delivery", `"CURSOR_EXACT"`},
		{"a lowercase authentication scheme", mutate(t, `"type": "API_KEY"`, `"type": "api_key"`), "/credentials/0/scheme/type", `"API_KEY"`},
		{"a lowercase key placement", mutate(t, `"in": "HEADER", "name": "X-Api-Key"`, `"in": "header", "name": "X-Api-Key"`), "/credentials/0/scheme/in", `"HEADER"`},
		{"a lowercase field type", mutate(t, `"name": "key", "type": "STRING"`, `"name": "key", "type": "string"`), "/credentials/0/fields/0/type", `"STRING"`},
		{"a lowercase scope", mutate(t, `"scope": "CREDENTIAL"`, `"scope": "credential"`), "/nodes/0/operations/1/safety/idempotency/scope", `"CREDENTIAL"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			wantIssue(t, tt.text, connector.RuleEnum, tt.path, "UPPER_SNAKE", tt.want)
		})
	}
}

func TestReservedNamespace(t *testing.T) {
	t.Parallel()
	text := strings.ReplaceAll(strings.ReplaceAll(referenceManifest, `"id": "acme.shop"`, `"id": "orcon.shop"`), `acme.shop.`, `orcon.shop.`)
	wantIssue(t, text, connector.RuleIdentity, "/id", "orcon.", "official")
	if _, err := connector.Parse([]byte(text), connector.Options{Official: true}); err != nil {
		t.Errorf("an official connector may use the orcon. namespace: %v", err)
	}
}

func TestManifestKeepsTheDocumentAndDigestsPermissions(t *testing.T) {
	t.Parallel()
	m := readOK(t, referenceManifest)
	if !jsonvalue.Equal(value(t, referenceManifest), m.Value()) {
		t.Error("Value() is not the document the manifest was read from")
	}
	d1, err := m.PermissionsDigest()
	if err != nil {
		t.Fatal(err)
	}
	other := readOK(t, mutate(t, `"maxConcurrent": 4`, `"maxConcurrent": 8`))
	d2, err := other.PermissionsDigest()
	if err != nil {
		t.Fatal(err)
	}
	if d1 == d2 || !strings.HasPrefix(d1, connector.DigestPrefix) || len(d1) != 71 {
		t.Errorf("permissions digests = %q and %q, want two different sha256 digests", d1, d2)
	}
	// The digest follows the permissions, not the member order of the document.
	shuffled := readOK(t, mutate(t, `"credentials": ["apiKey"],
    "resources": { "memoryMiB": 64, "timeout": "PT30S", "maxConcurrent": 4 }`, `"resources": { "maxConcurrent": 4, "timeout": "PT30S", "memoryMiB": 64 },
    "credentials": ["apiKey"]`))
	d3, err := shuffled.PermissionsDigest()
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d3 {
		t.Errorf("the permissions digest changed with member order: %s != %s", d1, d3)
	}
}
