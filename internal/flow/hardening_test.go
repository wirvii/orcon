// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func readJSON(t *testing.T, text string) (flow.Document, error) {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		t.Fatalf("test input is not JSON: %v", err)
	}
	return flow.Read(v)
}

// Two integers beyond 2^53 are the same double, so before the reader refused
// them, memoryLimit 9007199254740992 and ...993 read fine and hashed alike.
func TestReadRefusesNumbersThatWouldShareAHash(t *testing.T) {
	t.Parallel()
	node := func(params string) string {
		return `{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":` + params + `}`
	}
	tests := []struct {
		name   string
		doc    string
		wantOK bool
		path   string
	}{
		{"memoryLimit at the safe maximum", docWith(`"memoryLimit":9007199254740991`, goodNode, "", ""), true, ""},
		{"memoryLimit just beyond it", docWith(`"memoryLimit":9007199254740992`, goodNode, "", ""), false, "/settings/memoryLimit"},
		{"memoryLimit one more", docWith(`"memoryLimit":9007199254740993`, goodNode, "", ""), false, "/settings/memoryLimit"},
		{"memoryLimit as a decimal beyond the range", docWith(`"memoryLimit":9007199254740993.0`, goodNode, "", ""), false, "/settings/memoryLimit"},
		{"parameters hold a large integer", docWith("", node(`{"n":9007199254740993}`), "", ""), false, "/nodes/0/parameters/n"},
		{"parameters hold a negative large integer", docWith("", node(`{"n":[0,-9007199254740993]}`), "", ""), false, "/nodes/0/parameters/n/1"},
		{"parameters hold a long decimal", docWith("", node(`{"n":1.00000000000000001}`), "", ""), false, "/nodes/0/parameters/n"},
		{"ext holds a large integer", docWith("", goodNode, "", `,"ext":{"acme.n":9007199254740993}`), false, "/ext/acme.n"},
		{"editor holds a large integer", strings.Replace(docWith("", goodNode, "", ""), `"editor":{}`, `"editor":{"n":9007199254740993}`, 1), false, "/editor/n"},
		{"parameters hold a safe integer", docWith("", node(`{"n":9007199254740991,"f":0.1,"big":1e21}`), "", ""), true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := readJSON(t, tt.doc)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("Read error: %v", err)
				}
				return
			}
			var ve *flow.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("Read error = %v, want a ValidationError", err)
			}
			for _, issue := range ve.Report.Issues {
				if issue.Path == tt.path {
					return
				}
			}
			t.Errorf("no issue at %s: %v", tt.path, ve.Report.Issues)
		})
	}
}

func TestContentHashRefusesNumbersThatWouldCollide(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"9007199254740993", "9007199254740995", "1.00000000000000001"} {
		v := conformanceValue(t, "linear").With("description", mustNumber(t, n))
		if _, err := flow.ContentHashOfValue(v); err == nil {
			t.Errorf("ContentHashOfValue accepted %s", n)
		}
	}
}

func TestReadSpellsIntegralNumbersAlike(t *testing.T) {
	t.Parallel()
	hashes := map[string]flow.Hash{}
	for _, spelling := range []string{"2", "2.0", "2e0", "20e-1"} {
		doc, err := readJSON(t, docWith(`"concurrency":`+spelling, goodNode, "", ""))
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		if doc.Settings.Concurrency != 2 {
			t.Errorf("%s read as %d, want 2", spelling, doc.Settings.Concurrency)
		}
		h, err := flow.ContentHash(&doc)
		if err != nil {
			t.Fatal(err)
		}
		hashes[spelling] = h
	}
	for spelling, h := range hashes {
		if h != hashes["2"] {
			t.Errorf("%s hashes differently from 2", spelling)
		}
	}
}

func TestReadRejectsOutOfRangeIntegers(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"2147483648", "3000000000", "1e10", "0.5", "-1", "0", "1e999"} {
		_, err := readJSON(t, docWith(`"concurrency":`+n, goodNode, "", ""))
		var ve *flow.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("concurrency %s: Read error = %v, want a ValidationError", n, err)
			continue
		}
		var b strings.Builder
		for _, i := range ve.Report.Issues {
			b.WriteString(i.Message + ";")
		}
		joined := b.String()
		if !strings.Contains(joined, "must be an integer from 1 to 2147483647") && !strings.Contains(joined, "cannot be hashed") {
			t.Errorf("concurrency %s: messages %q do not state the allowed range", n, joined)
		}
	}
}

// A non-interrupting boundary event is another branch of its host, so what it
// starts runs while the host's continuation runs (ADR-0010 section 8).
func TestVariablesWritersOnANonInterruptingBoundaryAreConcurrent(t *testing.T) {
	t.Parallel()
	const tmpl = `{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-0000000000bb","name":"Boundary writers","settings":{},
"variables":[{"name":"x","scope":"EXECUTION"}],
"nodes":[
 {"id":"start","name":"Start","element":"startEvent","type":"orcon.start@1","parameters":{}},
 {"id":"h","name":"Host","element":"serviceTask","type":"orcon.http@1","parameters":{}},
 {"id":"b","name":"Timer","element":"boundaryEvent","type":"orcon.boundary@1","parameters":{},"attachedTo":"h","cancelActivity":CANCEL,"eventDefinition":{"timer":{"duration":"PT5S"}}},
 {"id":"w1","name":"Write after host","element":"task","type":"orcon.variables@1","parameters":{"assignments":[{"variable":"x","value":1}]}},
 {"id":"w2","name":"Write after timer","element":"task","type":"orcon.variables@1","parameters":{"assignments":[{"variable":"x","value":2}]}},
 {"id":"end1","name":"End one","element":"endEvent","type":"orcon.end@1","parameters":{}},
 {"id":"end2","name":"End two","element":"endEvent","type":"orcon.end@1","parameters":{}}
],
"edges":[
 {"id":"e1","from":{"node":"start","port":"main"},"to":{"node":"h"},"kind":"SEQUENCE"},
 {"id":"e2","from":{"node":"h","port":"main"},"to":{"node":"w1"},"kind":"SEQUENCE"},
 {"id":"e3","from":{"node":"b","port":"main"},"to":{"node":"w2"},"kind":"SEQUENCE"},
 {"id":"e4","from":{"node":"w1","port":"main"},"to":{"node":"end1"},"kind":"SEQUENCE"},
 {"id":"e5","from":{"node":"w2","port":"main"},"to":{"node":"end2"},"kind":"SEQUENCE"}
],"editor":{}}`
	tests := []struct {
		name   string
		cancel string
		want   bool
	}{
		{"non-interrupting boundary", "false", true},
		{"interrupting boundary replaces the continuation", "true", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc, err := readJSON(t, strings.Replace(tmpl, "CANCEL", tt.cancel, 1))
			if err != nil {
				t.Fatal(err)
			}
			r := flow.Validate(&doc)
			if got := r.Has(flow.CodeV9, flow.SeverityError); got != tt.want {
				t.Errorf("V9 error = %v, want %v; issues %v", got, tt.want, r.Issues)
			}
			if tt.want {
				doc.Variables[0].Accumulator = flow.AccumulatorSum
				if r := flow.Validate(&doc); r.HasErrors() {
					t.Errorf("an accumulator should make the concurrent writes legal: %v", r.Errors())
				}
			}
		})
	}
}
