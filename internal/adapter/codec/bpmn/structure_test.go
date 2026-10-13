// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import (
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
)

// checkStructure applies the XSD rules that matter for what Export writes:
// unique ids, resolvable references, the document-level namespaces and the
// order of the children of the process. The XSD itself is checked by
// TestRoundTripXSD when the schema files are available.
func checkStructure(tb testing.TB, data []byte) {
	tb.Helper()
	root, err := parseTree(data)
	if err != nil {
		tb.Fatalf("exported XML does not parse: %v", err)
	}
	if !isModel(root, "definitions") {
		tb.Fatalf("root is %v", root.name)
	}
	ids := map[string]bool{}
	var collect func(e *element)
	collect = func(e *element) {
		if id, ok := e.attr("", "id"); ok {
			if ids[id] {
				tb.Errorf("duplicate id %q", id)
			}
			ids[id] = true
		}
		for _, k := range e.kids {
			collect(k)
		}
	}
	collect(root)
	var refs func(e *element)
	refs = func(e *element) {
		for _, name := range []string{"sourceRef", "targetRef", "default", "bpmnElement"} {
			if ref, ok := e.attr("", name); ok && !ids[ref] {
				tb.Errorf("%s=%q does not resolve to an id", name, ref)
			}
		}
		for _, k := range e.kids {
			refs(k)
		}
	}
	refs(root)

	seenProcess := false
	for _, k := range root.kids {
		switch {
		case isModel(k, "process"):
			seenProcess = true
			checkProcessOrder(tb, k)
		case k.name.Local == "BPMNDiagram":
			if !seenProcess {
				tb.Error("the diagram comes before the process")
			}
		default:
			tb.Errorf("unexpected child %q of definitions", k.name.Local)
		}
	}
	if !seenProcess {
		tb.Error("no process")
	}
}

// checkProcessOrder checks the order the schema requires inside a process and
// inside each flow element: documentation, then extensionElements, then the rest.
func checkProcessOrder(tb testing.TB, p *element) {
	tb.Helper()
	rank := func(e *element) int {
		switch e.name.Local {
		case "documentation":
			return 0
		case "extensionElements":
			return 1
		default:
			return 2
		}
	}
	check := func(parent *element) {
		last := 0
		for _, k := range parent.kids {
			if r := rank(k); r < last {
				tb.Errorf("%q has %q after a later kind of child", parent.name.Local, k.name.Local)
			} else {
				last = r
			}
		}
	}
	check(p)
	for _, k := range p.kids {
		check(k)
		if k.name.Local == "sequenceFlow" {
			cond := -1
			for i, kk := range k.kids {
				if kk.name.Local == "conditionExpression" {
					cond = i
				}
			}
			if cond >= 0 && cond != len(k.kids)-1 {
				tb.Errorf("the condition of %q is not the last child", k.name.Local)
			}
		}
	}
}

// TestRoundTripXSD validates the exported XML against the BPMN 2.0 XSD. The OMG
// schema files are not part of this repository, so the test needs
// ORCON_BPMN_XSD_DIR to name a directory holding BPMN20.xsd and its imports,
// and the xmllint tool. Without them it is skipped.
func TestRoundTripXSD(t *testing.T) {
	dir := os.Getenv("ORCON_BPMN_XSD_DIR")
	if dir == "" {
		t.Skip("ORCON_BPMN_XSD_DIR is not set; the BPMN 2.0 XSD is not vendored")
	}
	lint, err := exec.LookPath("xmllint")
	if err != nil {
		t.Skip("xmllint is not installed")
	}
	flows := supportedFlows(t)
	docs := map[string]*flow.Document{}
	for name, doc := range flows {
		docs[name] = doc
	}
	// A sample of generated flows, whose odd strings stress the escaping.
	rng := rand.New(rand.NewPCG(7, 11)) //nolint:gosec // G404: deterministic test data
	for i := range 40 {
		doc := mutate(rng, flows["exclusive-and-inclusive"], false)
		if flow.Validate(doc).HasErrors() {
			continue
		}
		docs[fmt.Sprintf("generated-%d", i)] = doc
	}
	for name, doc := range docs {
		t.Run(name, func(t *testing.T) {
			data, err := Export(doc)
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), name+".bpmn")
			if err := os.WriteFile(file, data, 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.CommandContext(t.Context(), lint, "--noout", "--schema", filepath.Join(dir, "BPMN20.xsd"), file).CombinedOutput() //nolint:gosec // G204: the tool and schema path come from the test environment
			if err != nil {
				t.Fatalf("xmllint: %v\n%s", err, out)
			}
		})
	}
}
