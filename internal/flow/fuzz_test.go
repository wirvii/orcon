// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// FuzzRead feeds arbitrary bytes through the whole read path. Whatever the
// input, nothing may panic, and a document that reads and validates without
// errors must write back to the same JSON data and hash the same way.
func FuzzRead(f *testing.F) {
	files, err := filepath.Glob(filepath.Join(conformanceDir, "*.json"))
	if err != nil {
		f.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file) //nolint:gosec // G304: the path comes from a glob over the testdata directory
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte(`{"format":"orcon/flow@9"}`))
	f.Add([]byte(`{"format":"orcon/flow@1","id":"x","name":"X","settings":{},"variables":[],"nodes":[7],"edges":[],"editor":{}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		raw, err := jsonvalue.Parse(data)
		if err != nil {
			return
		}
		doc, err := flow.Read(raw)
		if err != nil {
			return
		}
		report := flow.Validate(&doc)
		for _, issue := range report.Issues {
			if issue.Code == "" || issue.Message == "" {
				t.Fatalf("issue without code or message: %+v", issue)
			}
		}
		if report.HasErrors() {
			return
		}
		want, errWant := jsonvalue.Canonical(raw)
		got, errGot := jsonvalue.Canonical(doc.Value())
		if (errWant == nil) != (errGot == nil) {
			t.Fatalf("canonical forms disagree on encodability: %v vs %v", errWant, errGot)
		}
		if errWant == nil && !bytes.Equal(want, got) {
			t.Fatalf("a valid document does not read back to the same data:\n in: %s\nout: %s", want, got)
		}
		if _, err := flow.ContentHash(&doc); err != nil && errWant == nil {
			t.Fatalf("ContentHash of an encodable valid document: %v", err)
		}
	})
}
