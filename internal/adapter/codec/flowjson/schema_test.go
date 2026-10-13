// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flowjson_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/adapter/codec/flowjson"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestSchemaFileIsTheEmbeddedSchema(t *testing.T) {
	t.Parallel()
	onDisk, err := os.ReadFile("schema/orcon-flow-1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(string(onDisk), string(flowjson.Schema())); diff != "" {
		t.Errorf("Schema() differs from the published file (-file +embedded):\n%s", diff)
	}
	// Callers get a copy and cannot corrupt the embedded bytes.
	s := flowjson.Schema()
	s[0] = 'X'
	if flowjson.Schema()[0] == 'X' {
		t.Error("Schema() returned the embedded slice itself")
	}
}

func TestSchemaIsWellFormed(t *testing.T) {
	t.Parallel()
	c := schemaChecked(t)
	for key, want := range map[string]string{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     flowjson.SchemaID,
		"title":   "orcon/flow@1",
	} {
		got, _ := c.root.Get(key)
		if s, _ := got.Str(); s != want {
			t.Errorf("schema %s = %q, want %q", key, s, want)
		}
	}
	if f, _ := c.root.Get("properties"); f.Kind() != jsonvalue.KindObject {
		t.Fatal("schema has no properties")
	}
	format, _ := c.root.Get("properties")
	format, _ = format.Get("format")
	if v, _ := format.Get("const"); !jsonvalue.Equal(v, str(flow.FormatTag)) {
		t.Errorf("schema format const = %v, want %s", v, flow.FormatTag)
	}
	if ap, _ := c.root.Get("additionalProperties"); !jsonvalue.Equal(ap, jsonvalue.Bool(false)) {
		t.Error("the top level must be closed (additionalProperties: false)")
	}
}

// TestSchemaEnumsMatchTheDomain keeps the published value sets and the Go
// value sets identical: a value added on one side fails here until the other
// side follows.
func TestSchemaEnumsMatchTheDomain(t *testing.T) {
	t.Parallel()
	c := schemaChecked(t)
	want := map[string][]string{
		"branchMode":      mapStrings(flow.BranchModes()),
		"priority":        mapStrings(flow.Priorities()),
		"retention":       mapStrings(flow.Retentions()),
		"routing":         mapStrings(flow.Routings()),
		"joinMode":        mapStrings(flow.JoinModes()),
		"joinData":        mapStrings(flow.JoinDataModes()),
		"onError":         mapStrings(flow.OnErrors()),
		"callMode":        mapStrings(flow.CallModes()),
		"waitKind":        mapStrings(flow.WaitKinds()),
		"edgeKind":        mapStrings(flow.EdgeKinds()),
		"attentionNotify": mapStrings(flow.AttentionNotifies()),
		"varScope":        mapStrings(flow.VarScopes()),
		"accumulator":     mapStrings(flow.Accumulators()),
		"element":         mapStrings(flow.Elements()),
	}
	defs, _ := c.root.Get("$defs")
	for name, values := range want {
		def, ok := defs.Get(name)
		if !ok {
			t.Errorf("schema has no $defs/%s", name)
			continue
		}
		enum, _ := def.Get("enum")
		var got []string
		for _, e := range enum.Items() {
			s, _ := e.Str()
			got = append(got, s)
		}
		if diff := cmp.Diff(values, got); diff != "" {
			t.Errorf("$defs/%s differs from the Go value set (-go +schema):\n%s", name, diff)
		}
	}
	for _, name := range []string{"concurrency", "settings", "node"} {
		if _, ok := defs.Get(name); !ok {
			t.Errorf("schema has no $defs/%s", name)
		}
	}
	for def, keyword := range map[string]string{"concurrency": flow.KeywordAll} {
		node, _ := defs.Get(def)
		if !strings.Contains(string(mustMarshal(t, node)), `"`+keyword+`"`) {
			t.Errorf("$defs/%s does not accept the keyword %s", def, keyword)
		}
	}
}

func mapStrings[T ~string](in []T) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = string(v)
	}
	return out
}

func mustMarshal(t *testing.T, v jsonvalue.Value) []byte {
	t.Helper()
	b, err := jsonvalue.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestSchemaPatternsMatchTheDomain keeps the patterns of the schema equal to
// the ones the Go validator uses.
func TestSchemaPatternsMatchTheDomain(t *testing.T) {
	t.Parallel()
	c := schemaChecked(t)
	defs, _ := c.root.Get("$defs")
	node, _ := defs.Get("node")
	nodeProps, _ := node.Get("properties")
	nodeType, _ := nodeProps.Get("type")
	edge, _ := defs.Get("edge")
	edgeProps, _ := edge.Get("properties")
	from, _ := edgeProps.Get("from")
	fromProps, _ := from.Get("properties")
	port, _ := fromProps.Get("port")
	ext, _ := defs.Get("ext")
	names, _ := ext.Get("propertyNames")
	allOf, _ := names.Get("allOf")

	patternOf := func(v jsonvalue.Value) string {
		p, _ := v.Get("pattern")
		s, _ := p.Str()
		return s
	}
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"node and edge id", patternOf(mustGet(t, defs, "id")), flow.IDPattern},
		{"flow id", patternOf(mustGet(t, defs, "uuid")), flow.UUIDPattern},
		{"instant", patternOf(mustGet(t, defs, "instant")), flow.InstantPattern},
		{"variableName", patternOf(mustGet(t, defs, "variableName")), flow.VariableNamePattern},
		{"duration", patternOf(mustGet(t, defs, "duration")), flow.DurationPattern},
		{"cycle", patternOf(mustGet(t, defs, "cycle")), flow.CyclePattern},
		{"node type", patternOf(nodeType), flow.NodeTypePattern},
		{"port", patternOf(port), flow.PortPattern},
		{"ext key", patternOf(allOf.Items()[0]), flow.ExtKeyPattern},
	}
	for _, tt := range checks {
		if tt.got != tt.want {
			t.Errorf("%s pattern in the schema = %q, Go = %q", tt.name, tt.got, tt.want)
		}
	}
}

func mustGet(t *testing.T, v jsonvalue.Value, key string) jsonvalue.Value {
	t.Helper()
	m, ok := v.Get(key)
	if !ok {
		t.Fatalf("no member %q", key)
	}
	return m
}

// TestSchemaAcceptsConformanceFlows is the positive half of E-FMT-1: every
// conformance flow satisfies the published schema, decodes, and passes the
// static validation.
func TestSchemaAcceptsConformanceFlows(t *testing.T) {
	t.Parallel()
	c := schemaChecked(t)
	for name, data := range conformanceFiles(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			v, err := jsonvalue.Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			if errs := c.errors(v); len(errs) > 0 {
				t.Errorf("schema rejects %s: %v", name, errs)
			}
			if goRejectsShape(t, v) {
				t.Errorf("the reader and validator disagree with the schema about %s", name)
			}
			doc, err := flowjson.Decode(data)
			if err != nil {
				t.Fatalf("Decode(%s): %v", name, err)
			}
			if report := flow.Validate(&doc); report.HasErrors() {
				t.Errorf("Validate(%s) has errors: %v", name, report.Errors())
			}
		})
	}
}

// structuralCase mutates a conformance flow; the schema must reject it (it is
// a shape problem) and the Go side must reject it too, at decode time or in
// validation.
type structuralCase struct {
	name    string
	flow    string
	pointer string
	value   string // JSON text of the new value; empty removes the member
}

func TestSchemaRejectsStructuralViolations(t *testing.T) {
	t.Parallel()
	cases := []structuralCase{
		{"missing format", "linear", "/format", ""},
		{"wrong format", "linear", "/format", `"orcon/flow@2"`},
		{"missing id", "linear", "/id", ""},
		{"id with a space", "linear", "/id", `"a b"`},
		{"id too long", "linear", "/id", `"` + strings.Repeat("a", 65) + `"`},
		{"empty name", "linear", "/name", `""`},
		{"name is a number", "linear", "/name", `7`},
		{"missing editor", "linear", "/editor", ""},
		{"missing nodes", "linear", "/nodes", ""},
		{"unknown top-level member", "linear", "/surprise", `1`},
		{"unknown setting", "effects-calls-and-settings", "/settings/turbo", `true`},
		{"memory limit beyond the safe integers", "effects-calls-and-settings", "/settings/memoryLimit", `9007199254740992`},
		{"memory limit that would share a hash", "effects-calls-and-settings", "/settings/memoryLimit", `9007199254740993`},
		{"concurrency above the maximum", "effects-calls-and-settings", "/settings/concurrency", `2147483648`},
		{"concurrency far above the maximum", "effects-calls-and-settings", "/settings/concurrency", `3000000000`},
		{"concurrency in exponent form above the maximum", "effects-calls-and-settings", "/settings/concurrency", `1e10`},
		{"loop iterations above the maximum", "loop", "/nodes/1/loop/maxIterations", `2147483648`},
		{"edge order above the maximum", "effects-calls-and-settings", "/edges/0/order", `2147483648`},
		{"memory limit zero", "effects-calls-and-settings", "/settings/memoryLimit", `0`},
		{"concurrency decimal", "effects-calls-and-settings", "/settings/concurrency", `1.5`},
		{"bad executionActive", "effects-calls-and-settings", "/settings/timeouts/executionActive", `"30 minutes"`},
		{"month duration", "effects-calls-and-settings", "/settings/timeouts/executionActive", `"P1M"`},
		{"empty duration", "effects-calls-and-settings", "/settings/timeouts/executionActive", `"PT"`},
		{"importedFrom without report", "effects-calls-and-settings", "/settings/importedFrom/reportId", ""},
		{"variable without scope", "loop", "/variables/0/scope", ""},
		{"variable name starts with a digit", "loop", "/variables/0/name", `"1x"`},
		{"node without parameters", "linear", "/nodes/1/parameters", ""},
		{"parameters not an object", "linear", "/nodes/1/parameters", `[]`},
		{"node type without major", "linear", "/nodes/1/type", `"orcon.http"`},
		{"node name too long", "linear", "/nodes/1/name", `"` + strings.Repeat("n", 129) + `"`},
		{"unknown node member", "linear", "/nodes/1/shade", `"red"`},
		{"disabled is a string", "linear", "/nodes/1/disabled", `"yes"`},
		{"variables node with no assignments", "variables", "/nodes/2/parameters/assignments", `[]`},
		{"variables node without assignments", "variables", "/nodes/2/parameters", `{}`},
		{"variable name is an expression", "variables", "/nodes/2/parameters/assignments/0/variable", `"={{ x }}"`},
		{"assignment without value", "variables", "/nodes/2/parameters/assignments/0/value", ""},
		{"assignment without variable", "variables", "/nodes/2/parameters/assignments/0/variable", ""},
		{"assignment with an unknown member", "variables", "/nodes/2/parameters/assignments/0/extra", `1`},
		{"variables node with an unknown parameter", "variables", "/nodes/2/parameters/mode", `"x"`},
		{"variables node that is not a task", "variables", "/nodes/2/element", `"serviceTask"`},
		{"writes is not a member of the format", "parallel-join", "/nodes/2/writes", `["seen"]`},
		{"effect is not a member of the format", "effects-calls-and-settings", "/nodes/1/effect", `{"effect":"READ"}`},
		{"flow id is not a UUID", "linear", "/id", `"linear"`},
		{"flow id with uppercase hexadecimal", "linear", "/id", `"01930000-0000-7000-8000-00000000000A"`},
		{"errorFlow is not a UUID", "effects-calls-and-settings", "/settings/errorFlow", `"notify-failure"`},
		{"call target neither a UUID nor an expression", "effects-calls-and-settings", "/nodes/4/call/target", `"ship-order"`},
		{"allowed target is not a UUID", "effects-calls-and-settings", "/nodes/5/call/allowedTargets/0", `"partner-a"`},
		{"empty allowedTargets", "effects-calls-and-settings", "/nodes/5/call/allowedTargets", `[]`},
		{"empty loop carry", "loop", "/nodes/1/loop/carry", `[]`},
		{"empty error codes", "waits-and-boundary", "/nodes/5/eventDefinition", `{"error":{"codes":[]}}`},
		{"timer date with an offset", "waits-and-boundary", "/nodes/1/wait/timer", `{"date":"2026-10-12T10:00:00+01:00"}`},
		{"timer date in lowercase z", "waits-and-boundary", "/nodes/1/wait/timer", `{"date":"2026-10-12T10:00:00z"}`},
		{"edge order zero", "effects-calls-and-settings", "/edges/0/order", `0`},
		{"join without data", "parallel-join", "/nodes/4/join/data", ""},
		{"loop without maxIterations", "loop", "/nodes/1/loop/maxIterations", ""},
		{"loop maxIterations zero", "loop", "/nodes/1/loop/maxIterations", `0`},
		{"multi-instance without batch size", "multi-instance", "/nodes/1/multiInstance/batchSize", ""},
		{"multi-instance concurrency zero", "multi-instance", "/nodes/1/multiInstance/concurrency", `0`},
		{"retry without attempts", "effects-calls-and-settings", "/nodes/1/retry/maxAttempts", ""},
		{"retry factor below one", "effects-calls-and-settings", "/nodes/1/retry/factor", `0.5`},
		{"retry interval not a duration", "effects-calls-and-settings", "/nodes/1/retry/initialInterval", `"1s"`},
		{"node timeout not a duration", "effects-calls-and-settings", "/nodes/1/timeouts/node", `"soon"`},
		{"node concurrency zero", "effects-calls-and-settings", "/nodes/1/concurrency", `0`},
		{"node concurrency lowercase keyword", "effects-calls-and-settings", "/nodes/1/concurrency", `"all"`},
		{"compensation without handler", "effects-calls-and-settings", "/nodes/1/compensation/handler", ""},
		{"call without perItem", "effects-calls-and-settings", "/nodes/4/call/perItem", ""},
		{"call version zero", "effects-calls-and-settings", "/nodes/4/call/version", `0`},
		{"call version other keyword", "effects-calls-and-settings", "/nodes/4/call/version", `"LATEST"`},
		{"recursion depth zero", "effects-calls-and-settings", "/nodes/4/call", `{"target":"01930000-0000-7000-8000-000000000007","version":1,"mode":"WAIT","perItem":false,"intentionalRecursion":{"maxDepth":0}}`},
		{"wait without kind", "waits-and-boundary", "/nodes/2/wait/kind", ""},
		{"wait timeout not a duration", "waits-and-boundary", "/nodes/2/wait/timeout", `"1 hour"`},
		{"timer date not a date-time", "waits-and-boundary", "/nodes/1/wait/timer", `{"date":"tomorrow"}`},
		{"timer cycle malformed", "waits-and-boundary", "/nodes/6/eventDefinition/timer/cycle", `"every 10s"`},
		{"unknown event definition", "waits-and-boundary", "/nodes/5/eventDefinition/signal", `{}`},
		{"compensation event definition not empty", "waits-and-boundary", "/nodes/5/eventDefinition", `{"compensation":{"x":1}}`},
		{"attention without notify", "effects-calls-and-settings", "/nodes/1/attention/notify", ""},
		{"extension key without namespace", "effects-calls-and-settings", "/ext", `{"plain":1}`},
		{"extension key in the reserved namespace", "effects-calls-and-settings", "/ext", `{"orcon.mine":1}`},
		{"edge without kind", "linear", "/edges/0/kind", ""},
		{"edge without from port", "linear", "/edges/0/from/port", ""},
		{"edge port with a space", "linear", "/edges/0/from/port", `"my port"`},
		{"edge to is a string", "linear", "/edges/0/to", `"fetch"`},
		{"edge order negative", "effects-calls-and-settings", "/edges/0/order", `-1`},
		{"edge default is a string", "linear", "/edges/0/default", `"yes"`},
		{"unknown edge member", "linear", "/edges/0/label", `"x"`},
	}
	c := schemaChecked(t)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := conformanceValue(t, tt.flow)
			if tt.value == "" {
				v = remove(t, v, tt.pointer)
			} else {
				v = set(t, v, tt.pointer, parse(t, tt.value))
			}
			if c.valid(v) {
				t.Errorf("the schema accepts %s although %s was changed", tt.flow, tt.pointer)
			}
			if !goRejectsShape(t, v) {
				t.Errorf("the reader and validator accept the document the schema rejects (%s)", tt.name)
			}
		})
	}
}

// shapeCodes are the issue codes of the levels the JSON Schema also covers.
var shapeCodes = map[string]bool{
	flow.CodeStructure: true, flow.CodeBlock: true, flow.CodeFormat: true, flow.CodeEnum: true, flow.CodeValue: true, flow.CodeExt: true,
}

// goRejectsShape reports whether the reader fails or the validator finds a
// shape problem: the Go counterpart of "the schema rejects this document".
func goRejectsShape(t *testing.T, v jsonvalue.Value) bool {
	t.Helper()
	doc, err := flowjson.Decode(mustMarshal(t, v))
	if err != nil {
		return true
	}
	for _, issue := range flow.Validate(&doc).Errors() {
		if shapeCodes[issue.Code] {
			return true
		}
	}
	return false
}

func TestSchemaLeavesSemanticRulesToTheValidator(t *testing.T) {
	t.Parallel()
	// Each document has the right shape, so the schema accepts it, and breaks a
	// rule that needs the whole graph, so Validate must reject it.
	cases := []struct {
		name, flow, pointer, value, code string
	}{
		{"element shape: start event with an incoming edge", "linear", "/edges/1/to/node", `"start"`, flow.CodeElement},
		{"V3 loop block missing is a shape-valid document", "loop", "/nodes/1/loop", ``, flow.CodeV3},
		{"reference to an undeclared variable", "variables", "/nodes/2/parameters/assignments/0/variable", `"ghost"`, flow.CodeReference},
		{"V12 duplicate names", "linear", "/nodes/2/name", `"Fetch"`, flow.CodeV12},
		{"reference to an unknown node", "linear", "/edges/1/to/node", `"ghost"`, flow.CodeReference},
		{"V8 read of a downstream node", "linear", "/nodes/1/parameters", `{"v":"={{ $('Done').item }}"}`, flow.CodeV8},
	}
	c := schemaChecked(t)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := conformanceValue(t, tt.flow)
			if tt.value == "" {
				v = remove(t, v, tt.pointer)
			} else {
				v = set(t, v, tt.pointer, parse(t, tt.value))
			}
			if errs := c.errors(v); len(errs) > 0 {
				t.Fatalf("the schema rejects a document that only breaks a graph rule: %v", errs)
			}
			if goRejectsShape(t, v) {
				t.Fatalf("the reader reports a shape problem in a document the schema accepts")
			}
			data := mustMarshal(t, v)
			doc, err := flowjson.Decode(data)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if r := flow.Validate(&doc); !r.Has(tt.code, flow.SeverityError) {
				t.Errorf("Validate issues = %v, want error %s", r.Issues, tt.code)
			}
		})
	}
}

type enumPoint struct {
	flow    string
	pointer string
	def     string // name in $defs of the closed value set
}

// TestEnumValuesUpperSnake is the UPPER_SNAKE half of E-FMT-1 at the format
// level: at every place where the document holds an enumerated value, each
// allowed spelling passes the schema and the validator, and the lowercase and
// camelCase spellings are rejected by both (ADR-0007 section 11).
func TestEnumValuesUpperSnake(t *testing.T) {
	t.Parallel()
	points := []enumPoint{
		{"effects-calls-and-settings", "/settings/branchMode", "branchMode"},
		{"effects-calls-and-settings", "/settings/priority", "priority"},
		{"effects-calls-and-settings", "/settings/retention", "retention"},
		{"effects-calls-and-settings", "/nodes/1/onError", "onError"},
		{"effects-calls-and-settings", "/nodes/1/attention/notify", "attentionNotify"},
		{"effects-calls-and-settings", "/nodes/5/call/mode", "callMode"},
		{"loop", "/variables/0/scope", "varScope"},
		{"loop", "/variables/1/accumulator", "accumulator"},
		{"loop", "/edges/3/kind", "edgeKind"},
		{"parallel-join", "/nodes/4/join/mode", "joinMode"},
		{"parallel-join", "/nodes/4/join/data", "joinData"},
		{"item-routing", "/nodes/1/routing", "routing"},
		{"waits-and-boundary", "/nodes/2/wait/kind", "waitKind"},
	}
	c := schemaChecked(t)
	defs, _ := c.root.Get("$defs")
	for _, p := range points {
		def := mustGet(t, defs, p.def)
		enum, _ := def.Get("enum")
		for _, allowed := range enum.Items() {
			name, _ := allowed.Str()
			t.Run(p.def+"/"+name, func(t *testing.T) {
				t.Parallel()
				good := set(t, conformanceValue(t, p.flow), p.pointer, allowed)
				if errs := c.errors(good); len(errs) > 0 {
					t.Errorf("the schema rejects the valid spelling %s: %v", name, errs)
				}
				if codes := enumIssues(t, good); len(codes) > 0 {
					t.Errorf("the validator reports %v for the valid spelling %s", codes, name)
				}
				for _, bad := range wrongSpellings(name) {
					v := set(t, conformanceValue(t, p.flow), p.pointer, str(bad))
					if c.valid(v) {
						t.Errorf("the schema accepts %q at %s", bad, p.pointer)
					}
					if codes := enumIssues(t, v); len(codes) == 0 {
						t.Errorf("the validator accepts %q at %s", bad, p.pointer)
					}
				}
			})
		}
	}
	t.Run("keywords", func(t *testing.T) {
		t.Parallel()
		for _, tt := range []struct{ flow, pointer, good, bad string }{
			{"multi-instance", "/nodes/1/multiInstance/concurrency", "ALL", "all"},
			{"effects-calls-and-settings", "/nodes/5/call/version", "CURRENT", "current"},
		} {
			good := set(t, conformanceValue(t, tt.flow), tt.pointer, str(tt.good))
			if !c.valid(good) || len(enumIssues(t, good)) > 0 {
				t.Errorf("%s: %q should be accepted", tt.pointer, tt.good)
			}
			bad := set(t, conformanceValue(t, tt.flow), tt.pointer, str(tt.bad))
			if c.valid(bad) || len(enumIssues(t, bad)) == 0 {
				t.Errorf("%s: %q should be rejected by the schema and the validator", tt.pointer, tt.bad)
			}
		}
	})
}

// wrongSpellings returns the spellings of an UPPER_SNAKE value that must be
// refused: lowercase, Capitalized and camelCase.
func wrongSpellings(upper string) []string {
	lower := strings.ToLower(upper)
	parts := strings.Split(lower, "_")
	var camelWords, titleWords strings.Builder
	camelWords.WriteString(parts[0])
	titleWords.WriteString(strings.ToUpper(parts[0][:1]) + parts[0][1:])
	for _, p := range parts[1:] {
		camelWords.WriteString(strings.ToUpper(p[:1]) + p[1:])
		titleWords.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	camel, title := camelWords.String(), titleWords.String()
	seen := map[string]bool{upper: true}
	var out []string
	for _, s := range []string{lower, camel, title} {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// enumIssues decodes a document and returns the ENUM and STRUCTURE issues of
// reading and validating it.
func enumIssues(t *testing.T, v jsonvalue.Value) []string {
	t.Helper()
	doc, err := flowjson.Decode(mustMarshal(t, v))
	if err != nil {
		return []string{err.Error()}
	}
	var out []string
	for _, i := range flow.Validate(&doc).Errors() {
		if i.Code == flow.CodeEnum || i.Code == flow.CodeStructure {
			out = append(out, i.String())
		}
	}
	return out
}

// A conforming JSON Schema validator reads 2.0 and 1e3 as integers. The Go
// reader must agree, and a document written either way must have one meaning.
func TestSchemaAndReaderAgreeOnIntegralSpellings(t *testing.T) {
	t.Parallel()
	c := schemaChecked(t)
	base := conformanceValue(t, "effects-calls-and-settings")
	want, err := flowjson.Decode(mustMarshal(t, set(t, base, "/settings/concurrency", parse(t, "1000"))))
	if err != nil {
		t.Fatal(err)
	}
	wantCanonical, err := flowjson.Canonical(&want)
	if err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{"1000", "1000.0", "1e3", "1E3", "100e1"} {
		v := set(t, base, "/settings/concurrency", parse(t, spelling))
		if !c.valid(v) {
			t.Errorf("the schema rejects %s: %v", spelling, c.errors(v))
		}
		got, err := flowjson.Decode(mustMarshal(t, v))
		if err != nil {
			t.Errorf("the reader rejects %s: %v", spelling, err)
			continue
		}
		gotCanonical, err := flowjson.Canonical(&got)
		if err != nil || !bytes.Equal(gotCanonical, wantCanonical) {
			t.Errorf("%s does not read as 1000 (err %v)", spelling, err)
		}
	}
}

// The schema describes shape only; numbers that cannot be hashed exactly are
// refused by the reader wherever they appear, even where the schema leaves the
// content open (parameters, ext, editor, forms).
func TestReaderRefusesInexactNumbersInOpenContent(t *testing.T) {
	t.Parallel()
	c := schemaChecked(t)
	v := set(t, conformanceValue(t, "linear"), "/nodes/1/parameters/n", parse(t, "9007199254740993"))
	if !c.valid(v) {
		t.Fatalf("the schema should leave parameters open: %v", c.errors(v))
	}
	if _, err := flowjson.Decode(mustMarshal(t, v)); err == nil {
		t.Error("Decode accepted an integer that would share its hash with its neighbor")
	}
}

// RFC 8785 forbids lone surrogates; Decode refuses them before they can
// collapse into U+FFFD.
func TestDecodeRefusesLoneSurrogates(t *testing.T) {
	t.Parallel()
	_, err := flowjson.Decode([]byte(`{"format":"orcon/flow@1","description":"a\ud800b"}`))
	if !errors.Is(err, jsonvalue.ErrLoneSurrogate) {
		t.Errorf("Decode error = %v, want ErrLoneSurrogate", err)
	}
}
