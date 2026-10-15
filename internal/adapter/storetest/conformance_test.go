// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"flag"
	"os"
	"testing"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/storetest"
)

// storeFlag selects the stores the conformance battery runs against:
//
//	auto      SQLite, and PostgreSQL when ORCON_TEST_POSTGRES=1 (default)
//	sqlite    SQLite only
//	postgres  PostgreSQL only; the test starts a server and fails if it cannot
//	both      SQLite and PostgreSQL; PostgreSQL must be available
var storeFlag = flag.String("store", "auto", "stores to test: auto, sqlite, postgres or both")

func TestMain(m *testing.M) {
	flag.Parse()
	switch *storeFlag {
	case "auto", "sqlite":
	case "postgres", "both":
		enablePostgres()
	}
	if os.Getenv("ORCON_TEST_POSTGRES") == "1" || postgresEnabled() {
		// The container client keeps goroutines of its own for the whole run.
		_ = runWithPostgres(m) // the test wrapper takes the exit code from m.Run
		return
	}
	goleak.VerifyTestMain(m)
}

func wantSQLite() bool   { return *storeFlag != "postgres" }
func wantPostgres() bool { return *storeFlag != "sqlite" }

// forEachStore runs fn as a subtest per selected store, with a factory of fresh
// stores of that kind. Tests named in a criterion use it, so one command selects
// the store through -args -store=.
func forEachStore(t *testing.T, fn func(t *testing.T, newStore storetest.Factory)) {
	t.Helper()
	if wantSQLite() {
		t.Run("sqlite", func(t *testing.T) { fn(t, sqliteFactory()) })
	}
	if wantPostgres() {
		t.Run("postgres", func(t *testing.T) { fn(t, postgresFactory()) })
	}
}

// TestConformance runs the shared battery against the selected stores
// (E-STO-1: the same battery passes on SQLite and PostgreSQL).
func TestConformance(t *testing.T) {
	switch *storeFlag {
	case "auto", "sqlite", "postgres", "both":
	default:
		t.Fatalf("unknown -store=%s (want auto, sqlite, postgres or both)", *storeFlag)
	}
	ran := 0
	if wantSQLite() {
		ran++
		t.Run("sqlite", func(t *testing.T) { storetest.Conformance(t, sqliteFactory()) })
	}
	if wantPostgres() {
		ran++
		t.Run("postgres", func(t *testing.T) { storetest.Conformance(t, postgresFactory()) })
	}
	if ran == 0 {
		t.Fatal("no store selected")
	}
}

// TestFloorSmoke is what a binary of an older release runs against a database
// migrated by the current binary (make test-compat-floor, E-STO-5). It needs the
// database the script prepared and fails, never skips, without it.
func TestFloorSmoke(t *testing.T) {
	path, dsn := os.Getenv("ORCON_FLOOR_SQLITE_PATH"), os.Getenv("ORCON_FLOOR_POSTGRES_DSN")
	if path == "" && dsn == "" {
		t.Skip("not a floor run: ORCON_FLOOR_SQLITE_PATH and ORCON_FLOOR_POSTGRES_DSN are unset (make test-compat-floor)")
	}
	if path != "" {
		t.Run("sqlite", func(t *testing.T) { storetest.Smoke(t, attachSQLite(path)) })
	}
	if dsn != "" {
		t.Run("postgres", func(t *testing.T) { storetest.Smoke(t, attachPostgres(dsn)) })
	}
}
