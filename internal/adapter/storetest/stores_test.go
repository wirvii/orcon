// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
)

// binaryVersionDefault is the version the factories give the stores they open.
const binaryVersionDefault = storetest.BinaryVersionDefault

// sqliteFactory returns a factory of SQLite stores on a new file per call.
func sqliteFactory() storetest.Factory { return storetest.SQLiteFactory() }

// postgresFactory returns a factory of PostgreSQL stores on a new database per call.
// The test is skipped when PostgreSQL tests are off.
func postgresFactory() storetest.Factory { return storetest.PostgresFactory() }

// attachSQLite returns a factory that opens an existing database file that
// another binary migrated, as the binary of a compatibility floor does.
func attachSQLite(path string) storetest.Factory {
	return func(t *testing.T) storetest.Store {
		t.Helper()
		return storetest.OpenSQLite(t, path)
	}
}

// attachPostgres returns a factory that opens an existing database that
// another binary migrated.
func attachPostgres(dsn string) storetest.Factory {
	return func(t *testing.T) storetest.Store {
		t.Helper()
		return storetest.OpenPostgres(t, dsn)
	}
}
