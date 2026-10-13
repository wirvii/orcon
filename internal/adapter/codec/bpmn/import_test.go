// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
)

func exportedLinear(t *testing.T) string {
	t.Helper()
	doc := supportedFlows(t)["linear"]
	data, err := Export(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestImportRejects(t *testing.T) {
	good := exportedLinear(t)
	cases := []struct {
		name string
		xml  string
		want string
	}{
		{"empty", ``, "empty"},
		{"not xml", `{"format":"orcon/flow@1"}`, "outside the root"},
		{"truncated", good[:len(good)/2], "well-formed"},
		{"wrong root", `<bpmn:process xmlns:bpmn="http://www.omg.org/spec/BPMN/20100524/MODEL"/>`, "definitions"},
		{"not orcon", `<bpmn:definitions xmlns:bpmn="http://www.omg.org/spec/BPMN/20100524/MODEL"><bpmn:process id="p"/></bpmn:definitions>`, "not an Orcon document"},
		{"doctype", `<!DOCTYPE x [<!ENTITY a "b">]>` + good, "document type"},
		{"processing instruction", strings.Replace(good, "<bpmn:definitions", `<?php echo 1 ?><bpmn:definitions`, 1), "processing instructions"},
		{"two processes", strings.Replace(good, "</bpmn:process>", "</bpmn:process><bpmn:process id=\"flow_x\" name=\"x\"/>", 1), "more than one process"},
		{"no process", `<bpmn:definitions xmlns:bpmn="http://www.omg.org/spec/BPMN/20100524/MODEL" xmlns:orcon="urn:orcon:bpmn:1" orcon:format="orcon/flow@1"/>`, "no process"},
		{"stranger in definitions", strings.Replace(good, "<bpmndi:BPMNDiagram", "<bpmn:message id=\"m\"/><bpmndi:BPMNDiagram", 1), "unexpected element"},
		{"unknown flow element", strings.Replace(good, "<bpmn:sequenceFlow", "<bpmn:laneSet id=\"l\"/><bpmn:sequenceFlow", 1), "not a BPMN element"},
		{"unknown value element", strings.ReplaceAll(good, "orcon:str", "orcon:bogus"), "unknown value element"},
		{"id without prefix", strings.Replace(good, `id="node_start"`, `id="start"`, 1), "prefix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Import([]byte(tc.xml))
			if err == nil {
				t.Fatal("want an error")
			}
			var fe *FormatError
			if !errors.As(err, &fe) {
				t.Fatalf("want FormatError, got %T %v", err, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

func TestImportRejectsBadValues(t *testing.T) {
	good := exportedLinear(t)
	// Corrupt each typed leaf of the exported document in turn.
	for _, kind := range []string{"str", "num", "bool", "null", "obj", "arr"} {
		open := "<orcon:" + kind
		if !strings.Contains(good, open) {
			continue
		}
		mutated := strings.Replace(good, open, "<orcon:wrong", 1)
		closing := "</orcon:" + kind + ">"
		mutated = strings.Replace(mutated, closing, "</orcon:wrong>", 1)
		if _, err := Import([]byte(mutated)); err == nil {
			t.Errorf("kind %s: corrupted document was accepted", kind)
		}
	}
}

func TestImportDeepNesting(t *testing.T) {
	deep := strings.Repeat("<orcon:arr>", maxXMLDepth+5) + strings.Repeat("</orcon:arr>", maxXMLDepth+5)
	doc := `<bpmn:definitions xmlns:bpmn="http://www.omg.org/spec/BPMN/20100524/MODEL" xmlns:orcon="urn:orcon:bpmn:1" orcon:format="orcon/flow@1">` + deep + `</bpmn:definitions>`
	_, err := Import([]byte(doc))
	var fe *FormatError
	if !errors.As(err, &fe) || !strings.Contains(err.Error(), "deeper") {
		t.Fatalf("want a depth FormatError, got %v", err)
	}
}

func TestImportLaterCutsFromXML(t *testing.T) {
	good := exportedLinear(t)
	mutated := strings.ReplaceAll(good, "bpmn:endEvent", "bpmn:boundaryEvent")
	_, err := Import([]byte(mutated))
	var un *UnsupportedError
	if !errors.As(err, &un) {
		t.Fatalf("want UnsupportedError, got %v", err)
	}
}

func TestImportDoesNotValidate(t *testing.T) {
	// Import checks the shape only: a duplicate node id is for flow.Validate.
	good := exportedLinear(t)
	ids := regexp.MustCompile(`<bpmn:\w+ id="(node_[^"]+)"`).FindAllStringSubmatch(good, -1)
	if len(ids) < 2 {
		t.Fatal("the linear flow needs two nodes")
	}
	doc, err := Import([]byte(strings.Replace(good, `id="`+ids[1][1]+`"`, `id="`+ids[0][1]+`"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if !flow.Validate(&doc).HasErrors() {
		t.Fatal("a duplicated node id must fail validation")
	}
}

func TestImportIgnoresDerivedParts(t *testing.T) {
	doc := supportedFlows(t)["discriminator"]
	data, err := Export(doc)
	if err != nil {
		t.Fatal(err)
	}
	// Drop the whole diagram and every derived attribute: nothing may change.
	text := string(data)
	if i := strings.Index(text, "<bpmndi:BPMNDiagram"); i >= 0 {
		text = text[:i] + "</bpmn:definitions>\n"
	}
	text = strings.ReplaceAll(text, "<bpmn:activationCondition>1</bpmn:activationCondition>", "")
	back, err := Import([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical(t, doc), canonical(t, &back)) {
		t.Fatal("derived parts changed the imported flow")
	}
}

func TestExactTextSurvives(t *testing.T) {
	doc := supportedFlows(t)["linear"]
	doc.Description = "  two spaces, tab\t, newline\n, carriage\r, <tag> & \"quotes\" 'x' ]]> \u2028  "
	doc.Nodes = append([]flow.Node(nil), doc.Nodes...)
	doc.Nodes[0].Name = " \t\n\r<&>\"' "
	roundTrip(t, doc)
}

func FuzzImport(f *testing.F) {
	for _, doc := range supportedFlows(f) {
		data, err := Export(doc)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte(`<bpmn:definitions xmlns:bpmn="http://www.omg.org/spec/BPMN/20100524/MODEL"/>`))
	f.Add([]byte(`<?xml version="1.0"?><a/>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := Import(data)
		if err != nil {
			return
		}
		// Whatever imports and validates must export again and come back equal.
		if flow.Validate(&doc).HasErrors() {
			return
		}
		again, err := Export(&doc)
		if err != nil {
			var un *UnsupportedError
			var bad *UnrepresentableError
			if errors.As(err, &un) || errors.As(err, &bad) {
				return
			}
			t.Fatalf("export of an imported valid flow: %v", err)
		}
		back, err := Import(again)
		if err != nil {
			t.Fatalf("import of an export: %v", err)
		}
		if !bytes.Equal(canonical(t, &doc), canonical(t, &back)) {
			t.Fatal("second round trip changed the flow")
		}
	})
}
