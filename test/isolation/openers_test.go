// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation_test

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
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/test/isolation"
)

// binaryVersion is the version the stores identify themselves with in a run.
const binaryVersion = "1.0.0"

// editorOpener returns the opener of fresh backends for the store under test. Each backend is a new,
// migrated database that the case owns and that is removed when the case ends.
func editorOpener(store isolation.Store) isolation.EditorOpener {
	return func(tb testing.TB) isolation.EditorBackend {
		tb.Helper()
		switch store {
		case isolation.StoreSQLite:
			s, err := sqlite.Open(context.Background(), sqlite.Options{Path: filepath.Join(tb.TempDir(), "orcon.db"), BinaryVersion: binaryVersion})
			if err != nil {
				tb.Fatalf("open sqlite store: %v", err)
			}
			tb.Cleanup(func() { _ = s.Close() })
			return s
		case isolation.StorePostgres:
			dsn := freshPostgresDatabase(tb)
			s, err := postgres.Open(context.Background(), postgres.Options{DSN: dsn, BinaryVersion: binaryVersion})
			if err != nil {
				tb.Fatalf("open postgres store: %v", err)
			}
			tb.Cleanup(func() { _ = s.Close() })
			return s
		default:
			tb.Fatalf("isolation: no editor opener for store %q", store)
			return nil
		}
	}
}

// freshPostgresDatabase creates a database on the server named by ORCON_TEST_POSTGRES_DSN and returns
// its connection string. The database is dropped when the case ends.
func freshPostgresDatabase(tb testing.TB) string {
	tb.Helper()
	admin := os.Getenv("ORCON_TEST_POSTGRES_DSN")
	if admin == "" {
		tb.Fatal("isolation: -store=postgres needs ORCON_TEST_POSTGRES_DSN (the administrative connection string of a PostgreSQL server)")
	}
	name := fmt.Sprintf("orcon_iso_%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := storetest.CreateDatabase(ctx, admin, name); err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer ccancel()
		if err := storetest.DropDatabase(cctx, admin, name); err != nil {
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
