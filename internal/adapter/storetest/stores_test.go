// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/adapter/storetest"
)

// binaryVersionDefault is the version the factories give the stores they open. A
// compatibility-floor run sets ORCON_BINARY_VERSION to the version of the older
// release whose code is running the test.
const binaryVersionDefault = "1.0.0"

// binaryVersion returns the version the stores identify themselves with.
func binaryVersion() string {
	if v := os.Getenv("ORCON_BINARY_VERSION"); v != "" {
		return v
	}
	return binaryVersionDefault
}

// sqliteFactory returns a factory of SQLite stores on a new file per call.
func sqliteFactory() storetest.Factory {
	return func(t *testing.T) storetest.Store {
		t.Helper()
		return openSQLite(t, filepath.Join(t.TempDir(), "orcon.db"))
	}
}

// attachSQLite returns a factory that opens an existing database file that
// another binary migrated, as the binary of a compatibility floor does.
func attachSQLite(path string) storetest.Factory {
	return func(t *testing.T) storetest.Store {
		t.Helper()
		return openSQLite(t, path)
	}
}

func openSQLite(t *testing.T, path string) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: binaryVersion()})
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

// postgresFactory returns a factory of PostgreSQL stores on a new database per call.
// The test is skipped when PostgreSQL tests are off.
func postgresFactory() storetest.Factory {
	return func(t *testing.T) storetest.Store {
		t.Helper()
		return openPostgres(t, newPostgresDatabase(t))
	}
}

// attachPostgres returns a factory that opens an existing database that
// another binary migrated.
func attachPostgres(dsn string) storetest.Factory {
	return func(t *testing.T) storetest.Store {
		t.Helper()
		return openPostgres(t, dsn)
	}
}

func openPostgres(t *testing.T, dsn string) *postgres.Store {
	t.Helper()
	s, err := postgres.Open(t.Context(), postgres.Options{DSN: dsn, BinaryVersion: binaryVersion()})
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
