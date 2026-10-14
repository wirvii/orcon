// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// xsdChunk is the number of files given to one xmllint run; the schema is
// loaded once per run.
const xsdChunk = 250

// TestRoundTripXSD validates the exported XML against the BPMN 2.0 XSD: the
// supported conformance flows and every generated flow of TestRoundTripGenerated.
// The OMG schema files are not part of this repository: CI fetches them with
// scripts/fetch-bpmn-xsd.sh and sets ORCON_BPMN_XSD_DIR to their directory;
// xmllint must be installed. Elsewhere the test is skipped, but when
// ORCON_BPMN_XSD_REQUIRED is set (only the check job sets it) a missing
// directory or tool is a failure, so the check cannot silently
// disappear from the pipeline.
func TestRoundTripXSD(t *testing.T) {
	dir := os.Getenv("ORCON_BPMN_XSD_DIR")
	lint, lintErr := exec.LookPath("xmllint")
	if dir == "" || lintErr != nil {
		const why = "ORCON_BPMN_XSD_DIR is not set or xmllint is not installed; see scripts/fetch-bpmn-xsd.sh"
		if os.Getenv("ORCON_BPMN_XSD_REQUIRED") != "" {
			t.Fatal(why)
		}
		t.Skip(why)
	}
	schema := filepath.Join(dir, "BPMN20.xsd")
	if _, err := os.Stat(schema); err != nil { //nolint:gosec // G703: the path comes from the test environment
		t.Fatalf("the schema directory has no BPMN20.xsd: %v", err)
	}

	docs := map[string]*flow.Document{}
	for name, doc := range supportedFlows(t) {
		docs[name] = doc
	}
	for i, doc := range generatedValidFlows(t, generatedFlows) {
		docs[fmt.Sprintf("generated-%04d", i)] = doc
	}
	names := make([]string, 0, len(docs))
	for name := range docs {
		names = append(names, name)
	}
	slices.Sort(names)

	tmp := t.TempDir()
	files := make([]string, 0, len(names))
	for _, name := range names {
		data, err := Export(docs[name])
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		file := filepath.Join(tmp, name+".bpmn")
		if err := os.WriteFile(file, data, 0o600); err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	for start := 0; start < len(files); start += xsdChunk {
		chunk := files[start:min(start+xsdChunk, len(files))]
		args := append([]string{"--noout", "--schema", schema}, chunk...)
		out, err := exec.CommandContext(t.Context(), lint, args...).CombinedOutput() //nolint:gosec // G204: the tool and schema path come from the test environment
		if err != nil {
			t.Fatalf("xmllint rejected a document:\n%s", out)
		}
	}
	t.Logf("%d exported documents validate against the BPMN 2.0 XSD", len(files))
}
