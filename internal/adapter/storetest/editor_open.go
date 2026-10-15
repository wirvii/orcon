// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
)

// editorBinaryVersion is the version the stores opened for the editor battery identify themselves with.
const editorBinaryVersion = "1.0.0"

// OpenSQLiteEditorStore opens a fresh, migrated SQLite store for one test or case. The store is closed
// when tb ends. Test drivers outside this package reach the adapters only through this function.
func OpenSQLiteEditorStore(tb testing.TB) *sqlite.Store {
	tb.Helper()
	s, err := sqlite.Open(tb.Context(), sqlite.Options{Path: filepath.Join(tb.TempDir(), "orcon.db"), BinaryVersion: editorBinaryVersion})
	if err != nil {
		tb.Fatalf("open sqlite store: %v", err)
	}
	tb.Cleanup(func() { _ = s.Close() })
	return s
}

// OpenPostgresEditorStore opens a fresh, migrated PostgreSQL store on a database created for tb. The
// administrative connection string comes from ORCON_TEST_POSTGRES_DSN; the database is dropped when tb ends.
func OpenPostgresEditorStore(tb testing.TB) *postgres.Store {
	tb.Helper()
	dsn := newFreshEditorDatabase(tb)
	s, err := postgres.Open(tb.Context(), postgres.Options{DSN: dsn, BinaryVersion: editorBinaryVersion})
	if err != nil {
		tb.Fatalf("open postgres store: %v", err)
	}
	tb.Cleanup(func() { _ = s.Close() })
	return s
}

// newFreshEditorDatabase creates a database on the server named by ORCON_TEST_POSTGRES_DSN and returns its
// connection string. The database is dropped when tb ends.
func newFreshEditorDatabase(tb testing.TB) string {
	tb.Helper()
	admin := os.Getenv("ORCON_TEST_POSTGRES_DSN")
	if admin == "" {
		tb.Fatal("the PostgreSQL editor battery needs ORCON_TEST_POSTGRES_DSN (the administrative connection string of a PostgreSQL server)")
	}
	name := fmt.Sprintf("orcon_edit_%d", time.Now().UnixNano())
	if err := CreateDatabase(tb.Context(), admin, name); err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		// The test context is already canceled here; the cleanup keeps its values and gets its own deadline.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(tb.Context()), time.Minute)
		defer cancel()
		if err := DropDatabase(ctx, admin, name); err != nil {
			tb.Errorf("%v", err)
		}
	})
	u, err := url.Parse(admin)
	if err != nil {
		tb.Fatalf("ORCON_TEST_POSTGRES_DSN: %v", err)
	}
	u.Path = "/" + name
	return strings.Clone(u.String())
}
