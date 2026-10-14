// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package orgscope

import (
	"os"
	"path/filepath"
	"testing"
)

func mkdirs(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o750); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRepoRootIsTheOutermost(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	nested := filepath.Join(top, "internal", "adapter", "sqlite", "rogue")
	// A planted marker deeper in the tree must not become the root.
	mkdirs(t, filepath.Join(top, ".git"), filepath.Join(nested, ".git"))
	if err := os.WriteFile(filepath.Join(nested, "go.work"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := repoRoot(nested); got != top {
		t.Errorf("repoRoot = %q, want the outermost %q", got, top)
	}
}

func TestRepoRootFallsBackToGoWork(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	dir := filepath.Join(top, "a", "b")
	mkdirs(t, dir)
	if err := os.WriteFile(filepath.Join(top, "go.work"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := repoRoot(dir); got != top {
		t.Errorf("repoRoot = %q, want %q", got, top)
	}
}

func TestRepoRootNone(t *testing.T) {
	t.Parallel()
	if got := repoRoot(t.TempDir()); got != "" {
		// A temporary directory below a repository would be a test environment problem.
		t.Skipf("the temporary directory is inside a repository (%s)", got)
	}
}

func TestAllowedFilesIgnoresNestedCopies(t *testing.T) {
	top := t.TempDir()
	rogue := filepath.Join(top, "internal", "adapter", "sqlite", "rogue")
	listDir := func(root string) string {
		return filepath.Join(root, filepath.FromSlash(filepath.Dir(allowlistPath)))
	}
	mkdirs(t, filepath.Join(top, ".git"), listDir(top), listDir(rogue))
	line := "  because  2026-10-13  approved-by:owner\n"
	write := func(root, entry string) {
		if err := os.WriteFile(filepath.Join(listDir(root), "dynamic_sql_allowlist.txt"), []byte(entry+line), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(top, "internal/adapter/sqlite/good/good.go")
	write(rogue, "rogue.go")

	saved := allowlistFlag
	allowlistFlag = ""
	t.Cleanup(func() { allowlistFlag = saved })

	got := allowedFiles(rogue)
	if !got[filepath.ToSlash(filepath.Join(top, "internal/adapter/sqlite/good/good.go"))] {
		t.Errorf("the allowlist at the repository root was not read: %v", got)
	}
	if got[filepath.ToSlash(filepath.Join(rogue, "rogue.go"))] {
		t.Errorf("a nested copy of the allowlist authorized a file: %v", got)
	}
}
