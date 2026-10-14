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

	"github.com/jackc/pgx/v5"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	name := fmt.Sprintf("orcon_iso_%d", time.Now().UnixNano())
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		tb.Fatalf("connect to the administrative database: %v", err)
	}
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		_ = conn.Close(ctx)
		tb.Fatalf("create database %s: %v", name, err)
	}
	_ = conn.Close(ctx)
	tb.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer ccancel()
		c, err := pgx.Connect(cctx, admin)
		if err != nil {
			tb.Errorf("drop database %s: connect: %v", name, err)
			return
		}
		defer func() { _ = c.Close(cctx) }()
		if _, err := c.Exec(cctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			tb.Errorf("drop database %s: %v", name, err)
		}
	})
	u, err := url.Parse(admin)
	if err != nil {
		tb.Fatalf("ORCON_TEST_POSTGRES_DSN: %v", err)
	}
	u.Path = "/" + name
	return strings.Clone(u.String())
}
