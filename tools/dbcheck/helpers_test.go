// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// realPolicy is the policy directory of the repository: the tests exercise
// the files the checks really read.
const realPolicy = "../../internal/adapter/storetest/dbrules/policy"

// tree writes files (paths relative to a scratch root) and returns a config
// whose adapter directories are <root>/sqlite and <root>/postgres. The policy
// is a copy of the real one with overrides applied (a nil value deletes).
func tree(t *testing.T, files map[string]string, policyOverrides map[string]*string) config {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"sqlite", "postgres"} {
		if err := os.MkdirAll(filepath.Join(root, d, "migrations"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	policyDir := filepath.Join(root, "policy")
	copyDir(t, realPolicy, policyDir)
	blankDialectOnlyQueries(t, policyDir)
	for name, content := range policyOverrides {
		path := filepath.Join(policyDir, name)
		if content == nil {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(*content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return config{
		SQLiteDir:   filepath.Join(root, "sqlite"),
		PostgresDir: filepath.Join(root, "postgres"),
		PolicyDir:   policyDir,
		Now:         func() time.Time { return time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC) },
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func str(s string) *string { return &s }

// runCheck runs one check (or "all") on cfg and returns the finding lines,
// the notes, and the exit code.
func runCheck(t *testing.T, command string, cfg config) (findings, notes []string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = execute(command, cfg, &out, &errOut)
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		switch {
		case line == "":
		case strings.HasPrefix(line, "note: "):
			notes = append(notes, strings.TrimPrefix(line, "note: "))
		case strings.HasPrefix(line, "dbcheck: "):
		default:
			findings = append(findings, line)
		}
	}
	return findings, notes, code
}

// rulesOf extracts the rule ids ("MG-1") of finding lines.
func rulesOf(findings []string) []string {
	seen := map[string]bool{}
	for _, f := range findings {
		// file:line: RULE message
		parts := strings.SplitN(f, ": ", 3)
		if len(parts) < 2 {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) > 0 {
			seen[fields[0]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func contains(findings []string, sub string) bool {
	for _, f := range findings {
		if strings.Contains(f, sub) {
			return true
		}
	}
	return false
}

const sqlHeader = "-- SPDX-FileCopyrightText: The Orcon Authors\n-- SPDX-License-Identifier: Apache-2.0\n"

func mig(kind, floor, body string) string {
	return sqlHeader + "-- orcon:kind " + kind + "\n-- orcon:compatibility-floor " + floor + "\n" + body
}

// blankDialectOnlyQueries keeps the comment lines of dialect_only_queries.txt and
// drops its entries. The entries of the real file name queries of the real
// adapters; a planted case has queries of its own, and an entry that names none
// of them would be reported as stale.
func blankDialectOnlyQueries(t *testing.T, policyDir string) {
	t.Helper()
	path := filepath.Join(policyDir, "dialect_only_queries.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "#") {
			kept = append(kept, line)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixturePolicy returns a copy of the real policy for the planted cases.
func fixturePolicy(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "policy")
	copyDir(t, realPolicy, dir)
	blankDialectOnlyQueries(t, dir)
	return dir
}
