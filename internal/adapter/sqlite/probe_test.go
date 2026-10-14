// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/sqlite"
)

func TestProbeMissingFileIsNotCreated(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "orcon.db")
	exists, err := sqlite.Probe(t.Context(), path)
	if err != nil || exists {
		t.Fatalf("Probe = %v, %v; want false, nil", exists, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Probe created the file: %v", err)
	}
	if _, err := sqlite.Probe(t.Context(), ""); err == nil {
		t.Fatal("an empty path must fail")
	}
}

func TestProbeReadsADatabaseThatAnotherStoreHasOpen(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "orcon.db")
	open(t, path) // holds the process lock and the WAL
	exists, err := sqlite.Probe(t.Context(), path)
	if err != nil || !exists {
		t.Fatalf("Probe = %v, %v; want true, nil while the server runs", exists, err)
	}
}

func TestProbeRefusesAFileThatIsNotADatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "orcon.db")
	if err := os.WriteFile(path, []byte("this is not a database, just some long enough text to look like a header"), 0o600); err != nil {
		t.Fatal(err)
	}
	exists, err := sqlite.Probe(t.Context(), path)
	if err == nil || !exists {
		t.Fatalf("Probe = %v, %v; want true and an error", exists, err)
	}
}

// names lists the files of a directory.
func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestProbeOfAStoppedDatabaseCreatesNoFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "orcon.db")
	s, err := sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: binaryVersion})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for _, leftover := range []string{"-wal", "-shm"} {
		_ = os.Remove(path + leftover) // a clean close usually removes them; make sure the stopped state is plain
	}
	before := names(t, dir)
	exists, err := sqlite.Probe(t.Context(), path)
	if err != nil || !exists {
		t.Fatalf("Probe = %v, %v; want true, nil", exists, err)
	}
	if after := names(t, dir); !slices.Equal(before, after) {
		t.Fatalf("the probe changed the directory: before %v, after %v", before, after)
	}
}
