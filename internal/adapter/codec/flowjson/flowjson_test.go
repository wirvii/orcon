// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flowjson_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/adapter/codec/flowjson"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// ruleCase breaks one static rule in a conformance flow, through JSON text.
type ruleCase struct {
	name    string
	flow    string
	pointer string
	value   string // empty removes the member
	code    string
	sev     flow.Severity
}

// TestValidateRulesFromJSON runs the V1 to V12 negative cases end to end: JSON
// text in, Decode, Validate, the rule's issue out. The domain tests cover the
// same rules on typed documents; this proves the codec feeds them faithfully.
func TestValidateRulesFromJSON(t *testing.T) {
	t.Parallel()
	cases := []ruleCase{
		{"V1 node nothing reaches", "linear", "/edges/0/to/node", `"done"`, flow.CodeV1, flow.SeverityError},
		{"V2 cycle without loop control", "loop", "/edges/3/kind", `"SEQUENCE"`, flow.CodeV2, flow.SeverityError},
		{"V3 loop without maxIterations block", "loop", "/nodes/1/loop", ``, flow.CodeV3, flow.SeverityError},
		{"V4 human wait without timeout", "waits-and-boundary", "/nodes/3/wait/timeout", ``, flow.CodeV4, flow.SeverityError},
		{"V5 split without default", "exclusive-and-inclusive", "/edges/2/default", ``, flow.CodeV5, flow.SeverityWarning},
		{"V6 join with one incoming edge", "parallel-join", "/edges/4", ``, flow.CodeV6, flow.SeverityError},
		{"V7 boundary on a non-activity", "waits-and-boundary", "/nodes/5/attachedTo", `"pause"`, flow.CodeV7, flow.SeverityError},
		{"V8 read of a sibling branch", "parallel-join", "/nodes/3/parameters", `{"v":"={{ $('Branch A').item }}"}`, flow.CodeV8, flow.SeverityError},
		{"V9 concurrent writers", "parallel-join", "/variables/0/accumulator", ``, flow.CodeV9, flow.SeverityError},
		{"V10 write effect without recovery", "effects-calls-and-settings", "/nodes/1/effect/onUncertain", ``, flow.CodeV10, flow.SeverityError},
		{"V11 call to itself", "effects-calls-and-settings", "/nodes/4/call/target", `"effects-calls-and-settings"`, flow.CodeV11, flow.SeverityError},
		{"V12 duplicate names", "linear", "/nodes/2/name", `"Fetch"`, flow.CodeV12, flow.SeverityError},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := conformanceValue(t, tt.flow)
			if tt.value == "" {
				v = remove(t, v, tt.pointer)
			} else {
				v = set(t, v, tt.pointer, parse(t, tt.value))
			}
			doc, err := flowjson.Decode(mustMarshal(t, v))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if r := flow.Validate(&doc); !r.Has(tt.code, tt.sev) {
				t.Errorf("Validate issues = %v, want %s %s", r.Issues, tt.sev, tt.code)
			}
		})
	}
}

func TestDecodeRoundTripsConformanceFlows(t *testing.T) {
	t.Parallel()
	for name, data := range conformanceFiles(t) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			doc, err := flowjson.Decode(data)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := flowjson.Encode(&doc)
			if err != nil {
				t.Fatal(err)
			}
			again, err := flowjson.Decode(encoded)
			if err != nil {
				t.Fatalf("Decode(Encode(doc)): %v", err)
			}
			a, errA := flowjson.Canonical(&doc)
			b, errB := flowjson.Canonical(&again)
			if errA != nil || errB != nil {
				t.Fatal(errA, errB)
			}
			if diff := cmp.Diff(string(a), string(b)); diff != "" {
				t.Errorf("Decode(Encode(doc)) changed the data (-before +after):\n%s", diff)
			}
			// The source file holds the same data as the model.
			want, err := jsonvalue.Canonical(parse(t, string(data)))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a, want) {
				t.Errorf("the model does not hold the data of the file:\n%s\n%s", a, want)
			}
			// Encoding keeps the member order of the format: format comes first.
			if !strings.HasPrefix(string(encoded), `{"format":"orcon/flow@1","id":`) {
				t.Errorf("Encode starts with %.40s", encoded)
			}
		})
	}
}

func TestDecodeRejectsBadText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		want error
	}{
		{"not JSON", `{`, jsonvalue.ErrSyntax},
		{"empty", ``, jsonvalue.ErrSyntax},
		{"duplicate key", `{"format":"orcon/flow@1","format":"orcon/flow@1"}`, jsonvalue.ErrDuplicateKey},
		{"trailing data", `{} {}`, jsonvalue.ErrSyntax},
		{"invalid UTF-8", "{\"format\":\"\xff\"}", jsonvalue.ErrSyntax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := flowjson.Decode([]byte(tt.text)); !errors.Is(err, tt.want) {
				t.Errorf("Decode(%q) error = %v, want %v", tt.text, err, tt.want)
			}
		})
	}
	t.Run("wrong shape names every problem", func(t *testing.T) {
		t.Parallel()
		_, err := flowjson.Decode([]byte(`{"format":"orcon/flow@1","name":3,"extra":1}`))
		var ve *flow.ValidationError
		if !errors.As(err, &ve) || len(ve.Report.Issues) < 3 {
			t.Errorf("Decode error = %v, want a ValidationError with several issues", err)
		}
	})
}

// TestUnknownFormat: a document of a format newer than this build knows is
// rejected with an error that names the format and the supported range
// (E-FMT-3).
func TestUnknownFormat(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"orcon/flow@2", "orcon/flow@17", "orcon/flow@0", "other/flow@1", "orcon/flow"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			doc := set(t, conformanceValue(t, "linear"), "/format", str(format))
			_, err := flowjson.Decode(mustMarshal(t, doc))
			var unknown *flow.UnknownFormatError
			if !errors.As(err, &unknown) {
				t.Fatalf("Decode error = %v, want *flow.UnknownFormatError", err)
			}
			for _, fragment := range []string{format, "orcon/flow@1 through orcon/flow@1"} {
				if !strings.Contains(err.Error(), fragment) {
					t.Errorf("error %q does not mention %q", err, fragment)
				}
			}
		})
	}
}

// TestUpgrade: the codec reads through the upgrade chain without rewriting
// what is stored. With one format version the chain is the identity; the
// stepwise behavior is covered in the flow package with injected steps.
func TestUpgradeLeavesStoredBytesAlone(t *testing.T) {
	t.Parallel()
	for name, data := range conformanceFiles(t) {
		before := string(data)
		if _, err := flowjson.Decode(data); err != nil {
			t.Fatalf("Decode(%s): %v", name, err)
		}
		if string(data) != before {
			t.Errorf("Decode(%s) modified the stored bytes", name)
		}
	}
}

func TestCanonicalIgnoresMemberOrder(t *testing.T) {
	t.Parallel()
	a := `{"format":"orcon/flow@1","id":"x","name":"X","settings":{"concurrency":10},"variables":[],"nodes":[],"edges":[],"editor":{}}`
	b := `{"editor":{},"edges":[],"nodes":[],"variables":[],"settings":{"concurrency":10},"name":"X","id":"x","format":"orcon/flow@1"}`
	da, errA := flowjson.Decode([]byte(a))
	db, errB := flowjson.Decode([]byte(b))
	if errA != nil || errB != nil {
		t.Fatalf("Decode errors: %v %v", errA, errB)
	}
	ca, _ := flowjson.Canonical(&da)
	cb, _ := flowjson.Canonical(&db)
	if !bytes.Equal(ca, cb) {
		t.Errorf("canonical forms differ: %s vs %s", ca, cb)
	}
}

func FuzzDecode(f *testing.F) {
	for _, data := range conformanceSeeds(f) {
		f.Add(data)
	}
	f.Add([]byte(`{"format":"orcon/flow@2"}`))
	f.Add([]byte(`{"format":"orcon/flow@1","id":"x","name":"X","settings":{},"variables":[],"nodes":[],"edges":[],"editor":{}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := flowjson.Decode(data)
		if err != nil {
			return
		}
		report := flow.Validate(&doc)
		if report.HasErrors() {
			return
		}
		encoded, err := flowjson.Encode(&doc)
		if err != nil {
			t.Fatalf("Encode of a valid document: %v", err)
		}
		again, err := flowjson.Decode(encoded)
		if err != nil {
			t.Fatalf("Decode(Encode(doc)): %v", err)
		}
		a, errA := flowjson.Canonical(&doc)
		b, errB := flowjson.Canonical(&again)
		if (errA == nil) != (errB == nil) || (errA == nil && !bytes.Equal(a, b)) {
			t.Fatalf("round trip changed the document:\n%s\n%s", a, b)
		}
	})
}
