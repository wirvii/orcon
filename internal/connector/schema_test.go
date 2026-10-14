// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector_test

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func schemaValue(t *testing.T) jsonvalue.Value {
	t.Helper()
	return value(t, string(connector.Schema()))
}

// at walks object members from a value.
func at(t *testing.T, v jsonvalue.Value, path ...string) jsonvalue.Value {
	t.Helper()
	for _, key := range path {
		next, ok := v.Get(key)
		if !ok {
			t.Fatalf("the schema has no member %q on the way to %v", key, path)
		}
		v = next
	}
	return v
}

func strs(t *testing.T, v jsonvalue.Value) []string {
	t.Helper()
	var out []string
	for _, item := range v.Items() {
		s, ok := item.Str()
		if !ok {
			t.Fatalf("%v is not a string", item)
		}
		out = append(out, s)
	}
	return out
}

func asStrings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = string(s)
	}
	return out
}

func TestSchemaFileIsTheEmbeddedSchema(t *testing.T) {
	t.Parallel()
	onDisk, err := os.ReadFile("schema/orcon-connector-1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(string(onDisk), string(connector.Schema())); diff != "" {
		t.Errorf("Schema() differs from the published file (-file +embedded):\n%s", diff)
	}
	s := connector.Schema()
	s[0] = 'X'
	if connector.Schema()[0] == 'X' {
		t.Error("Schema() returned the embedded slice itself")
	}
}

func TestSchemaIsWellFormed(t *testing.T) {
	t.Parallel()
	root := schemaValue(t)
	for key, want := range map[string]string{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     connector.SchemaID,
		"title":   "orcon/connector@1",
	} {
		if got, _ := at(t, root, key).Str(); got != want {
			t.Errorf("schema %s = %q, want %q", key, got, want)
		}
	}
	if got, _ := at(t, root, "properties", "format", "const").Str(); got != connector.FormatTag {
		t.Errorf("schema format const = %q, want %q", got, connector.FormatTag)
	}
	if v := at(t, root, "additionalProperties"); !jsonvalue.Equal(v, jsonvalue.Bool(false)) {
		t.Error("the top level must be closed (additionalProperties: false)")
	}
}

// TestSchemaEnumsMatchTheDomain keeps every closed value set of the schema
// equal to the one the Go validator uses, so the two cannot drift.
func TestSchemaEnumsMatchTheDomain(t *testing.T) {
	t.Parallel()
	defs := at(t, schemaValue(t), "$defs")
	props := at(t, schemaValue(t), "properties")
	node := at(t, defs, "node", "properties")
	op := at(t, defs, "operation", "properties")
	safety := at(t, defs, "safety", "properties")
	trigger := at(t, defs, "trigger", "properties")
	cred := at(t, defs, "credential", "properties", "fields", "items", "properties")
	rate := at(t, defs, "rateLimit", "properties", "respectHeaders", "properties", "resetAt", "properties")
	idem := at(t, safety, "idempotency", "properties")
	oneOf := func(v jsonvalue.Value) jsonvalue.Value { return at(t, v, "enum") }

	checks := []struct {
		name string
		got  jsonvalue.Value
		want []string
	}{
		{"kind", oneOf(at(t, props, "kind")), asStrings(connector.Kinds())},
		{"group", oneOf(at(t, node, "group")), asStrings(connector.Groups())},
		{"purity", oneOf(at(t, op, "purity")), asStrings(connector.OperationPurities())},
		{"item mode", oneOf(at(t, op, "itemMode")), asStrings(connector.ItemModes())},
		{"effect", oneOf(at(t, safety, "effect")), asStrings(flow.EffectKinds())},
		{"level", oneOf(at(t, safety, "level")), asStrings(flow.SafetyLevels())},
		{"on uncertain", oneOf(at(t, safety, "onUncertain")), asStrings(flow.OnUncertains())},
		{"proof level", oneOf(at(t, safety, "proofLevel")), asStrings(connector.ProofLevels())},
		{"idempotency mode", oneOf(at(t, idem, "mode")), asStrings(flow.IdempotencyModes())},
		{"key location", oneOf(at(t, idem, "location", "properties", "in")), asStrings(flow.KeyLocations())},
		{"key scope", oneOf(at(t, idem, "scope")), asStrings(connector.KeyScopes())},
		{"trigger family", oneOf(at(t, trigger, "family")), asStrings(connector.TriggerFamilies())},
		{"delivery", oneOf(at(t, trigger, "delivery")), asStrings(connector.Deliveries())},
		{"dedupe source", oneOf(at(t, trigger, "dedupeKeySource")), asStrings(connector.DedupeSources())},
		{"field type", oneOf(at(t, cred, "type")), asStrings(connector.FieldTypes())},
		{"scheme type", oneOf(at(t, defs, "scheme", "properties", "type")), asStrings(connector.SchemeTypes())},
		{"reset format", oneOf(at(t, rate, "format")), asStrings(connector.ResetFormats())},
	}
	for _, tt := range checks {
		if diff := cmp.Diff(tt.want, strs(t, tt.got)); diff != "" {
			t.Errorf("%s values differ (-Go +schema):\n%s", tt.name, diff)
		}
	}
	for _, tt := range checks {
		for _, v := range tt.want {
			if v != upperSnake(v) {
				t.Errorf("%s value %q is not UPPER_SNAKE", tt.name, v)
			}
		}
	}
}

func upperSnake(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'a' && c <= 'z' {
			out[i] = c - 'a' + 'A'
		}
	}
	return string(out)
}

// TestSchemaPatternsMatchTheDomain keeps the patterns of the schema equal to
// the ones the Go validator uses.
func TestSchemaPatternsMatchTheDomain(t *testing.T) {
	t.Parallel()
	root := schemaValue(t)
	defs := at(t, root, "$defs")
	props := at(t, root, "properties")
	pattern := func(v jsonvalue.Value) string {
		s, _ := at(t, v, "pattern").Str()
		return s
	}
	node := at(t, defs, "node", "properties")
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"connector id", pattern(at(t, props, "id")), connector.IDPattern},
		{"engine range", pattern(at(t, props, "compat", "properties", "engineApi")), connector.SemverRangePattern},
		{"license", pattern(at(t, props, "license")), connector.LicensePattern},
		{"commit", pattern(at(t, props, "source", "properties", "commit")), connector.CommitPattern},
		{"version", pattern(at(t, defs, "version")), flow.ConnectorVersionPattern},
		{"duration", pattern(at(t, defs, "duration")), flow.DurationPattern},
		{"operation name", pattern(at(t, defs, "operationName")), connector.OperationNamePattern},
		{"pointer", pattern(at(t, defs, "pointer")), connector.PointerPattern},
		{"port", pattern(at(t, defs, "portName")), flow.PortPattern},
		{"node name", pattern(at(t, node, "name")), connector.NodeNamePattern},
		{"node type", pattern(at(t, node, "type")), flow.NodeTypePattern},
		{"credential id", pattern(at(t, defs, "credential", "properties", "id")), connector.CredentialIDPattern},
		{"credential field", pattern(at(t, defs, "credential", "properties", "fields", "items", "properties", "name")), connector.FieldNamePattern},
		{"rate limit name", pattern(at(t, defs, "rateLimit", "properties", "name")), connector.RateLimitNamePattern},
		{"domain", pattern(at(t, defs, "permissions", "properties", "network", "properties", "domains", "items")), connector.DomainPattern},
		{"changelog date", pattern(at(t, defs, "changelogEntry", "properties", "date")), connector.DatePattern},
		{"message key", pattern(at(t, defs, "changelogEntry", "properties", "notes", "items")), connector.MessageKeyPattern},
	}
	for _, tt := range checks {
		if tt.got != tt.want {
			t.Errorf("%s pattern in the schema = %q, Go = %q", tt.name, tt.got, tt.want)
		}
	}
}
