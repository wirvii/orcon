// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/codec/flowjson"
	"github.com/wirvii/orcon/internal/flow"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// conformanceDir holds the conformance flows, which the flow area owns.
var conformanceDir = filepath.Join("..", "..", "..", "flow", "testdata", "conformance")

// deferredFlows are the conformance flows that use waits, events, calls or
// multi-instance, which the codec refuses until cut C9.
var deferredFlows = map[string]bool{
	"multi-instance":             true,
	"waits-and-boundary":         true,
	"effects-calls-and-settings": true,
}

func conformanceFlows(tb testing.TB) map[string]*flow.Document {
	tb.Helper()
	files, err := filepath.Glob(filepath.Join(conformanceDir, "*.json"))
	if err != nil || len(files) == 0 {
		tb.Fatalf("no conformance flows in %s (%v)", conformanceDir, err)
	}
	out := make(map[string]*flow.Document, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec // G304: the path comes from a glob over the testdata directory
		if err != nil {
			tb.Fatal(err)
		}
		doc, err := flowjson.Decode(data)
		if err != nil {
			tb.Fatalf("%s: %v", f, err)
		}
		out[strings.TrimSuffix(filepath.Base(f), ".json")] = &doc
	}
	return out
}

func canonical(tb testing.TB, doc *flow.Document) []byte {
	tb.Helper()
	data, err := flowjson.Canonical(doc)
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

// roundTrip exports and imports a flow and fails unless the canonical forms agree.
func roundTrip(tb testing.TB, doc *flow.Document) []byte {
	tb.Helper()
	xmlData, err := Export(doc)
	if err != nil {
		tb.Fatalf("export: %v", err)
	}
	back, err := Import(xmlData)
	if err != nil {
		tb.Fatalf("import: %v\n%s", err, xmlData)
	}
	if want, got := canonical(tb, doc), canonical(tb, &back); !bytes.Equal(want, got) {
		tb.Fatalf("round trip changed the flow:\nwant %s\n got %s", want, got)
	}
	return xmlData
}
