// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

const conformanceDir = "testdata/conformance"

// conformanceNames lists the conformance flows by file name without extension.
func conformanceNames(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(conformanceDir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no conformance flows in %s (%v)", conformanceDir, err)
	}
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = strings.TrimSuffix(filepath.Base(f), ".json")
	}
	return names
}

// conformanceValue parses a conformance flow into its JSON value.
func conformanceValue(t *testing.T, name string) jsonvalue.Value {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(conformanceDir, name+".json")) //nolint:gosec // G304: the name comes from the test tables
	if err != nil {
		t.Fatalf("read conformance flow %s: %v", name, err)
	}
	v, err := jsonvalue.Parse(data)
	if err != nil {
		t.Fatalf("parse conformance flow %s: %v", name, err)
	}
	return v
}

// load returns a fresh, independent copy of a conformance flow, so a test may
// mutate it freely.
func load(t *testing.T, name string) flow.Document {
	t.Helper()
	doc, err := flow.Read(conformanceValue(t, name))
	if err != nil {
		t.Fatalf("read conformance flow %s: %v", name, err)
	}
	return doc
}

// node returns a pointer to the node with the given id inside doc. It panics
// when there is none, which fails the calling test; the mutation closures of
// the rule tables call it without a testing handle.
func node(doc *flow.Document, id string) *flow.Node {
	for i := range doc.Nodes {
		if doc.Nodes[i].ID == flow.NodeID(id) {
			return &doc.Nodes[i]
		}
	}
	panic(fmt.Sprintf("node %q not found", id))
}

// edge returns a pointer to the edge with the given id inside doc, or panics.
func edge(doc *flow.Document, id string) *flow.Edge {
	for i := range doc.Edges {
		if doc.Edges[i].ID == flow.EdgeID(id) {
			return &doc.Edges[i]
		}
	}
	panic(fmt.Sprintf("edge %q not found", id))
}

func dropEdge(doc *flow.Document, id string) {
	doc.Edges = slices.DeleteFunc(doc.Edges, func(e flow.Edge) bool { return e.ID == flow.EdgeID(id) })
}

// obj builds a JSON object from members with distinct keys, or panics.
func obj(members ...jsonvalue.Member) jsonvalue.Value {
	v, err := jsonvalue.Object(members...)
	if err != nil {
		panic(err)
	}
	return v
}

// params returns a parameters object with a single template string.
func params(template string) jsonvalue.Value {
	return obj(jsonvalue.Member{Key: "value", Value: jsonvalue.String(template)})
}

func ptrTo[T any](v T) *T { return &v }

func has(r flow.Report, code string, sev flow.Severity) bool { return r.Has(code, sev) }
