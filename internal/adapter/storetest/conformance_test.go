// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution/executiontest"
)

// storeFlag selects the stores the conformance battery runs against:
//
//	auto      SQLite, and PostgreSQL when ORCON_TEST_POSTGRES=1 (default)
//	sqlite    SQLite only
//	postgres  PostgreSQL only; the test starts a server and fails if it cannot
//	both      SQLite and PostgreSQL; PostgreSQL must be available
func storeFlag() string { return executiontest.StoreFlag("auto") }

func TestMain(m *testing.M) {
	flag.Parse()
	switch storeFlag() {
	case "auto", "sqlite":
	case "postgres", "both":
		enablePostgres()
	}
	if os.Getenv("ORCON_TEST_POSTGRES") == "1" || postgresEnabled() {
		// The container client keeps goroutines of its own for the whole run.
		runWithPostgres(m)
		return
	}
	goleak.VerifyTestMain(m)
}

func wantSQLite() bool   { return storeFlag() != "postgres" }
func wantPostgres() bool { return storeFlag() != "sqlite" }

// TestConformance runs the shared battery against the selected stores
// (E-STO-1: the same battery passes on SQLite and PostgreSQL).
func TestConformance(t *testing.T) {
	switch storeFlag() {
	case "auto", "sqlite", "postgres", "both":
	default:
		t.Fatalf("unknown -store=%s (want auto, sqlite, postgres or both)", storeFlag())
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

// TestAbandonCount runs the contract of the abandon count on the selected
// stores (E-REC-5): the parts that need a worker are in test/engine/durable.
func TestAbandonCount(t *testing.T) {
	switch storeFlag() {
	case "auto", "sqlite", "postgres", "both":
	default:
		t.Fatalf("unknown -store=%s (want auto, sqlite, postgres or both)", storeFlag())
	}
	if wantSQLite() {
		t.Run("sqlite", func(t *testing.T) {
			storetest.AbandonCount(t, sqliteFactory())
			t.Run("StartupReleaseAddsOne", func(t *testing.T) { sqliteStartupCounts(t) })
			t.Run("ThresholdBelowTwoIsRejected", func(t *testing.T) {
				_, err := sqlite.Open(t.Context(), sqlite.Options{Path: filepath.Join(t.TempDir(), "orcon.db"), BinaryVersion: binaryVersion(), PoisonThreshold: 1})
				if err == nil {
					t.Fatal("a poison threshold of 1 was accepted")
				}
			})
		})
	}
	if wantPostgres() {
		t.Run("postgres", func(t *testing.T) { storetest.AbandonCount(t, postgresFactory()) })
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

// sqliteStartupCounts closes a database with a token leased and opens it again:
// the open releases the lease and counts one abandonment, because the process
// that held it did not return it (SQLite has one process, so nothing else can).
func sqliteStartupCounts(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "orcon.db")
	open := func() *sqlite.Store {
		s, err := sqlite.Open(t.Context(), sqlite.Options{Path: path, BinaryVersion: binaryVersion()})
		if err != nil {
			t.Fatalf("open sqlite store: %v", err)
		}
		return s
	}
	first := open()
	left := storetest.SeedLeftLeased(t, first)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second := open()
	t.Cleanup(func() {
		if err := second.Close(); err != nil {
			t.Errorf("close sqlite store: %v", err)
		}
	})
	storetest.CheckStartupRelease(t, second, left)
}
