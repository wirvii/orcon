// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/cli"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

const (
	slackTemplate = `{"workflow":{"id":1,"totalViews":10,"workflow":{"nodes":[
	  {"name":"Start","type":"n8n-nodes-base.manualTrigger","typeVersion":1,"parameters":{}},
	  {"name":"Post","type":"n8n-nodes-base.slack","typeVersion":2,"parameters":{"resource":"message","operation":"post"}}]}}}`
	coreTemplate = `{"workflow":{"id":2,"totalViews":5,"workflow":{"nodes":[
	  {"name":"Start","type":"n8n-nodes-base.manualTrigger","typeVersion":1,"parameters":{}}]}}}`
)

// corpusDir writes a two-template corpus the way tools/n8ncorpus does.
func corpusDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0o750); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"manifest.json":    `{"n8nVersion":"9.9.9","source":"test","fetchedAt":"2026-01-01T00:00:00Z","templates":2,"corpusSha256":"abc"}`,
		"templates/1.json": slackTemplate,
		"templates/2.json": coreTemplate,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = cli.N8n(t.Context(), args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestN8nUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
		out  string
	}{
		{"no arguments", nil, 2, "usage"},
		{"help", []string{"help"}, 0, "usage"},
		{"unknown command", []string{"convert-node"}, 2, ""},
		{"coverage without a corpus", []string{"coverage"}, 2, ""},
		{"target out of range", []string{"coverage", "--corpus", "x", "--target", "120"}, 2, ""},
		{"unknown flag", []string{"coverage", "--nope"}, 2, ""},
		{"coverage help", []string{"coverage", "-h"}, 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out, _ := run(t, tc.args...)
			if code != tc.code || !strings.Contains(out, tc.out) {
				t.Fatalf("exit %d, stdout %q; want %d and %q", code, out, tc.code, tc.out)
			}
		})
	}
}

func TestN8nCoveragePlanned(t *testing.T) {
	dir := corpusDir(t)
	out := t.TempDir()
	paths := map[string]string{
		"--out": filepath.Join(out, "report.json"), "--markdown": filepath.Join(out, "coverage.md"),
		"--roadmap": filepath.Join(out, "roadmap.md"), "--catalog-out": filepath.Join(out, "planned.json"),
	}
	args := []string{"coverage", "--corpus", dir, "--catalog", "planned"}
	for flag, path := range paths {
		args = append(args, flag, path)
	}
	code, stdout, stderr := run(t, args...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{"2 templates", "n8n 9.9.9", "catalog: planned", "100.00 %", "wave 1: 1 connectors"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("summary lacks %q:\n%s", want, stdout)
		}
	}
	for flag, path := range paths {
		data, err := os.ReadFile(path) //nolint:gosec // G304: a path under the test's temporary directory
		if err != nil || len(data) == 0 {
			t.Errorf("%s wrote nothing (%v)", flag, err)
		}
	}

	// The planned catalog it wrote is a valid catalog file that measures the same.
	code, stdout, stderr = run(t, "coverage", "--corpus", dir, "--catalog", paths["--catalog-out"])
	if code != 0 || !strings.Contains(stdout, "100.00 %") {
		t.Fatalf("catalog file: exit %d: %s\n%s", code, stderr, stdout)
	}
}

func TestN8nCoverageCore(t *testing.T) {
	code, stdout, stderr := run(t, "coverage", "--corpus", corpusDir(t), "--catalog", "core")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "50.00 %") {
		t.Errorf("only the core template imports:\n%s", stdout)
	}
}

func TestN8nCoverageFailures(t *testing.T) {
	dir := corpusDir(t)
	missing := filepath.Join(t.TempDir(), "absent")
	badCatalog := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badCatalog, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	inventory := filepath.Join(t.TempDir(), "inventory.json")
	if err := os.WriteFile(inventory, []byte(`{"n8nVersion":"9.9.9","nodes":[{"type":"n8n-nodes-base.brandNewCore","group":"CORE"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"installed catalog", []string{"--corpus", dir, "--catalog", "installed"}, "not available yet"},
		{"missing corpus", []string{"--corpus", missing}, "read corpus"},
		{"missing catalog file", []string{"--corpus", dir, "--catalog", missing}, "read catalog"},
		{"bad catalog file", []string{"--corpus", dir, "--catalog", badCatalog}, "connectors"},
		{"core type without counterpart", []string{"--corpus", dir, "--inventory", inventory}, "brandNewCore"},
		{"missing inventory", []string{"--corpus", dir, "--inventory", missing}, "read inventory"},
		{"unwritable output", []string{"--corpus", dir, "--out", filepath.Join(missing, "r.json")}, "write"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := run(t, append([]string{"coverage"}, tc.args...)...)
			if code != 1 || !strings.Contains(stderr, tc.want) {
				t.Fatalf("exit %d, stderr %q; want 1 and %q", code, stderr, tc.want)
			}
		})
	}
}
