// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"os"
	"path/filepath"
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
