// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSVGNamesTheFileAndExitsOne(t *testing.T) {
	t.Parallel()
	tests := []struct {
		fixture string
		exit    int
		out     string
	}{
		{"clean", 0, "brandcheck: ok"},
		{"image-lower", 1, "<image>"},
		{"data-fill", 1, "data: URI"},
		{"href-external", 1, "not a local #fragment"},
		{"text-element", 1, "<text>"},
	}
	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()
			file := filepath.Join("testdata", "svg", tc.fixture+".svg")
			var stdout, stderr bytes.Buffer
			if got := run([]string{"svg", file}, &stdout, &stderr); got != tc.exit {
				t.Fatalf("run(svg %s) = %d, want %d\nstdout: %s\nstderr: %s", file, got, tc.exit, &stdout, &stderr)
			}
			if !strings.Contains(stdout.String(), tc.out) {
				t.Errorf("stdout = %q, want it to contain %q", &stdout, tc.out)
			}
			if tc.exit == 1 && !strings.Contains(stdout.String(), file+":") {
				t.Errorf("stdout = %q, want findings that name %s", &stdout, file)
			}
		})
	}
}

func TestRunSVGChecksEveryFileGiven(t *testing.T) {
	t.Parallel()
	clean := filepath.Join("testdata", "svg", "clean.svg")
	bad := filepath.Join("testdata", "svg", "script.svg")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"svg", clean, bad}, &stdout, &stderr); got != 1 {
		t.Fatalf("run(svg clean bad) = %d, want 1", got)
	}
	if strings.Contains(stdout.String(), clean) || !strings.Contains(stdout.String(), bad) {
		t.Errorf("stdout = %q, want findings for %s only", &stdout, bad)
	}
}

func TestRunAllOnTree(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(tr *tree)
		exit   int
		out    string
	}{
		{"good tree", func(*tree) {}, 0, "brandcheck: ok"},
		{"no manifest", func(tr *tree) { tr.remove(manifestPath) }, 0, "note: no brand/manifest.json"},
		{"mascot redrawn", func(tr *tree) { tr.putFixture(mascotPath, "raster/mascot-bad-pixel.png") }, 1, "BC-4"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := goodTree(t)
			tc.mutate(tr)
			var stdout, stderr bytes.Buffer
			if got := run([]string{"-root", tr.root, "all"}, &stdout, &stderr); got != tc.exit {
				t.Fatalf("run(all) = %d, want %d\nstdout: %s\nstderr: %s", got, tc.exit, &stdout, &stderr)
			}
			if !strings.Contains(stdout.String(), tc.out) {
				t.Errorf("stdout = %q, want it to contain %q", &stdout, tc.out)
			}
		})
	}
}

func TestRunUsageErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		exit int
	}{
		{"no command", nil, 2},
		{"unknown command", []string{"fix"}, 2},
		{"svg without files", []string{"svg"}, 2},
		{"all with a stray argument", []string{"all", "extra"}, 2},
		{"unknown flag", []string{"-bogus", "all"}, 2},
		{"svg file that does not exist", []string{"svg", filepath.Join("testdata", "svg", "nope.svg")}, 2},
		{"help", []string{"-h"}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if got := run(tc.args, &stdout, &stderr); got != tc.exit {
				t.Errorf("run(%q) = %d, want %d\nstderr: %s", tc.args, got, tc.exit, &stderr)
			}
		})
	}
}

// TestRepositoryBrandTreePasses runs every rule over the real repository, so a
// brand asset that breaks a rule fails `go test` as well as `make brand-check`.
func TestRepositoryBrandTreePasses(t *testing.T) {
	t.Parallel()
	got, _, err := checkAll(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("checkAll(repository): %v", err)
	}
	requireExactly(t, got)
}
