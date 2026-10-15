// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
)

// BinaryVersionDefault is the version the factories give the stores they open. A
// compatibility-floor run sets ORCON_BINARY_VERSION to the version of the older
// release whose code is running the test.
const BinaryVersionDefault = "1.0.0"

// BinaryVersion returns the version the stores identify themselves with.
func BinaryVersion() string {
	if v := os.Getenv("ORCON_BINARY_VERSION"); v != "" {
		return v
	}
	return BinaryVersionDefault
}

// SQLiteFactory returns a factory of SQLite stores on a new file per call.
func SQLiteFactory() Factory {
	return func(t *testing.T) Store {
		t.Helper()
		return OpenSQLite(t, filepath.Join(t.TempDir(), "orcon.db"))
	}
}

// PostgresFactory returns a factory of PostgreSQL stores on a new database per call.
// The test is skipped when PostgreSQL tests are off.
func PostgresFactory() Factory {
	return func(t *testing.T) Store {
		t.Helper()
		return OpenPostgres(t, NewPostgresDatabase(t))
	}
}

// OpenSQLite opens (and migrates) the SQLite database at path and closes it when the test ends.
func OpenSQLite(t *testing.T, path string) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: BinaryVersion()})
	if err != nil {
		t.Fatalf("open sqlite store: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close sqlite store: %v", err)
		}
	})
	return s
}

// OpenPostgres opens (and migrates) the PostgreSQL database at dsn and closes it when the test ends.
func OpenPostgres(t *testing.T, dsn string) *postgres.Store {
	t.Helper()
	s, err := postgres.Open(t.Context(), postgres.Options{DSN: dsn, BinaryVersion: BinaryVersion()})
	if err != nil {
		t.Fatalf("open postgres store: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close postgres store: %v", err)
		}
	})
	return s
}
