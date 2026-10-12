// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodException = "example.com/lib@v1.2.3  MPL-2.0  file-level copyleft, no changes to the module  2026-10-12  approved-by:owner\n"

func runWith(t *testing.T, exceptions, report string, extra ...string) (int, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "exceptions.txt")
	if err := os.WriteFile(path, []byte(exceptions), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := append([]string{"-exceptions", path}, extra...)
	code := run(args, strings.NewReader(report), &stdout, &stderr)
	return code, stderr.String()
}

func TestRun(t *testing.T) {
	const header = "# format documentation only\n\n"
	tests := []struct {
		name       string
		exceptions string
		report     string
		wantCode   int
		wantErr    string
	}{
		{"allowed licenses pass", header, "a.com/x\tv1.0.0\tMIT\nb.com/y\tv2.0.0\tBSD-3-Clause\nc.com/z\tv1.0.0\tISC\nd.com/w\tv1.0.0\tApache-2.0\ne.com/v\tv1.0.0\tBSD-2-Clause\n", 0, ""},
		{"MPL-2.0 fails by default", header, "example.com/lib\tv1.2.3\tMPL-2.0\n", 1, "no approved exception"},
		{"0BSD fails by default", header, "example.com/lib\tv1.2.3\t0BSD\n", 1, "not allowed"},
		{"Unlicense fails by default", header, "example.com/lib\tv1.2.3\tUnlicense\n", 1, "not allowed"},
		{"CC0 fails by default", header, "example.com/lib\tv1.2.3\tCC0-1.0\n", 1, "not allowed"},
		{"unknown fails by default", header, "example.com/lib\tv1.2.3\tUnknown\n", 1, "not allowed"},
		{"exact module@version exception passes", header + goodException, "example.com/lib\tv1.2.3\tMPL-2.0\n", 0, ""},
		{"a new version needs a renewed line", header + goodException, "example.com/lib\tv1.2.4\tMPL-2.0\n", 1, "no approved exception"},
		{"exception for another license fails", header + goodException, "example.com/lib\tv1.2.3\t0BSD\n", 1, "names MPL-2.0"},
		{"GPL cannot be admitted", header, "example.com/lib\tv1.2.3\tGPL-3.0\n", 1, "never links"},
		{"malformed line fails", header + "example.com/lib@v1.2.3 MPL-2.0 because\n", "a.com/x\tv1.0.0\tMIT\n", 1, "malformed exception"},
		{"missing approved-by fails", header + "example.com/lib@v1.2.3  MPL-2.0  reason  2026-10-12\n", "a.com/x\tv1.0.0\tMIT\n", 1, "malformed exception"},
		{"bad date fails", header + "example.com/lib@v1.2.3  MPL-2.0  reason  2026-13-45  approved-by:owner\n", "a.com/x\tv1.0.0\tMIT\n", 1, "not a valid date"},
		{"exception for GPL is rejected", header + "example.com/lib@v1.2.3  GPL-3.0  reason  2026-10-12  approved-by:owner\n", "a.com/x\tv1.0.0\tMIT\n", 1, "cannot be admitted"},
		{"duplicate exception fails", header + goodException + goodException, "a.com/x\tv1.0.0\tMIT\n", 1, "duplicate"},
		{"bad report row is a usage error", header, "not-a-row\n", 2, "unexpected report row"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, stderr := runWith(t, tc.exceptions, tc.report)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d (stderr: %s)", code, tc.wantCode, stderr)
			}
			if tc.wantErr != "" && !strings.Contains(stderr, tc.wantErr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tc.wantErr)
			}
		})
	}
}

func TestIgnoreSkipsTheCheckedModule(t *testing.T) {
	code, stderr := runWith(t, "", "github.com/wirvii/orcon\tUnknown\tUnknown\ngithub.com/wirvii/orcon/internal/x\tUnknown\tUnknown\n", "-ignore", "github.com/wirvii/orcon")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr)
	}
}

func TestMissingExceptionsFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-exceptions", filepath.Join(t.TempDir(), "absent.txt")}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-h"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}
