// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

const fixtureDir = "../../n8nimport/testdata/workflows"

func TestN8nConvertWritesFlowAndReport(t *testing.T) {
	dir := t.TempDir()
	flowPath := filepath.Join(dir, "flow.json")
	reportPath := filepath.Join(dir, "report.md")
	var stdout, stderr bytes.Buffer
	args := []string{
		filepath.Join(fixtureDir, "whatsapp-lead-intake.json"),
		"--out", flowPath, "--report", reportPath,
		"--id", "01930000-0000-7000-8000-0000000000bb",
	}
	if code := N8n(t.Context(), append([]string{"convert"}, args...), &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	data, err := os.ReadFile(flowPath) //nolint:gosec // G304: the test reads the file it wrote
	if err != nil {
		t.Fatal(err)
	}
	doc := decodeFlow(t, data)
	if doc.ID != "01930000-0000-7000-8000-0000000000bb" {
		t.Errorf("flow id %q", doc.ID)
	}
	if len(doc.Nodes) != 12 {
		t.Errorf("flow has %d nodes, want 12 (the sticky note is editor data)", len(doc.Nodes))
	}
	if rep := flow.Validate(&doc); rep.HasErrors() {
		for _, issue := range rep.Errors() {
			if issue.Code != "V1" {
				t.Errorf("unexpected validation issue: %s", issue)
			}
		}
	}
	md, err := os.ReadFile(reportPath) //nolint:gosec // G304: the test reads the file it wrote
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "| Save to CRM |") {
		t.Errorf("report has no row for Save to CRM")
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout is not empty when --out is set: %q", stdout.String())
	}
}

func TestN8nConvertPrintsFlowToStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := N8n(t.Context(), []string{"convert", filepath.Join(fixtureDir, "scheduled-followup.json")}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	decodeFlow(t, stdout.Bytes())
	if !strings.Contains(stderr.String(), "# n8n import report") {
		t.Errorf("the report did not go to stderr")
	}
}

func TestN8nConvertUsageAndErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "no file", args: []string{"convert"}, want: exitUsage},
		{name: "two files", args: []string{"convert", "a.json", "b.json"}, want: exitUsage},
		{name: "bad id", args: []string{"convert", filepath.Join(fixtureDir, "edge-cases.json"), "--id", "nope"}, want: exitError},
		{name: "missing file", args: []string{"convert", filepath.Join(t.TempDir(), "absent.json")}, want: exitError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := N8n(t.Context(), tt.args, &stdout, &stderr); got != tt.want {
				t.Errorf("exit %d, want %d (stderr: %s)", got, tt.want, stderr.String())
			}
		})
	}
}

// decodeFlow reads a flow document the way the flow codec does, failing the
// test when the bytes are not a flow.
func decodeFlow(t *testing.T, data []byte) flow.Document {
	t.Helper()
	v, err := jsonvalue.Parse(data)
	if err != nil {
		t.Fatalf("the output is not JSON: %v", err)
	}
	doc, err := flow.Read(v)
	if err != nil {
		t.Fatalf("the output is not a flow document: %v", err)
	}
	return doc
}
