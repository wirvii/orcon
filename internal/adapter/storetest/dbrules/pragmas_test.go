// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// openPools opens the writer (one connection) and a reader pool on the same
// file, both from the DSNs sqliteDSN builds, so the file exists before readers
// attach. The omit list leaves settings out of both DSNs.
func openPools(t *testing.T, readers int, omit ...string) (writer, reader *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pools.db")
	writer, err := sql.Open("sqlite", sqliteDSN(path, true, omit...))
	if err != nil {
		t.Fatal(err)
	}
	writer.SetMaxOpenConns(1)
	reader, err = sql.Open("sqlite", sqliteDSN(path, false, omit...))
	if err != nil {
		t.Fatal(err)
	}
	reader.SetMaxOpenConns(readers)
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	return writer, reader
}

// holdConns takes n connections of db at the same time: successive queries
// reuse one pooled connection, so only holding them proves every one is set up.
func holdConns(t *testing.T, db *sql.DB, n int) []*sql.Conn {
	t.Helper()
	conns := make([]*sql.Conn, 0, n)
	for range n {
		c, err := db.Conn(t.Context())
		if err != nil {
			t.Fatalf("take connection: %v", err)
		}
		t.Cleanup(func() { _ = c.Close() })
		conns = append(conns, c)
	}
	return conns
}

// TestForeignKeysOnEveryConnection holds several reader connections and the
// writer at the same time and checks each of them (connections.md CN-6).
func TestForeignKeysOnEveryConnection(t *testing.T) {
	const readers = 4
	writer, reader := openPools(t, readers)
	for _, c := range holdConns(t, writer, 1) {
		if got := CheckSQLiteConnection(t.Context(), c, SQLiteWriter); len(got) != 0 {
			t.Errorf("writer:\n%s", describe(got))
		}
	}
	held := holdConns(t, reader, readers)
	seen := map[*sql.Conn]bool{}
	for i, c := range held {
		if seen[c] {
			t.Fatalf("connection %d repeated", i)
		}
		seen[c] = true
		if got := CheckSQLiteConnection(t.Context(), c, SQLiteReader); len(got) != 0 {
			t.Errorf("reader %d:\n%s", i, describe(got))
		}
	}
}

func TestOpenerWithoutForeignKeysIsDetected(t *testing.T) {
	writer, reader := openPools(t, 3, "foreign_keys")
	for _, c := range holdConns(t, writer, 1) {
		got := CheckSQLiteConnection(t.Context(), c, SQLiteWriter)
		if !strings.Contains(describe(got), "pragma foreign_keys") {
			t.Errorf("writer: foreign_keys=0 was not detected:\n%s", describe(got))
		}
	}
	for i, c := range holdConns(t, reader, 3) {
		got := CheckSQLiteConnection(t.Context(), c, SQLiteReader)
		if !strings.Contains(describe(got), "pragma foreign_keys") {
			t.Errorf("reader %d: foreign_keys=0 was not detected:\n%s", i, describe(got))
		}
	}
}

// A pragma executed once on the pool reaches only the connection that ran it.
func TestPragmaSetOnOneConnectionOnlyIsDetected(t *testing.T) {
	_, reader := openPools(t, 3, "foreign_keys")
	if _, err := reader.ExecContext(t.Context(), "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatal(err)
	}
	var missing int
	for _, c := range holdConns(t, reader, 3) {
		if strings.Contains(describe(CheckSQLiteConnection(t.Context(), c, SQLiteReader)), "pragma foreign_keys") {
			missing++
		}
	}
	if missing != 2 {
		t.Fatalf("%d of 3 connections lack foreign_keys, want 2 (the pragma reached only the connection that ran it)", missing)
	}
}

func TestSQLiteConnectionSettings(t *testing.T) {
	tests := []struct {
		name   string
		omit   []string
		role   SQLiteRole
		opts   []SQLiteOption
		object string // pragma expected in the findings, "" for none
	}{
		{name: "writer is fine", role: SQLiteWriter},
		{name: "reader is fine", role: SQLiteReader},
		{name: "busy timeout missing", omit: []string{"busy_timeout"}, role: SQLiteWriter, object: "pragma busy_timeout"},
		{name: "trusted schema left on", omit: []string{"trusted_schema"}, role: SQLiteReader, object: "pragma trusted_schema"},
		{name: "writer without wal", omit: []string{"journal_mode"}, role: SQLiteWriter, object: "pragma journal_mode"},
		{name: "writer with a weaker synchronous", omit: []string{"synchronous", "+synchronous(1)"}, role: SQLiteWriter, object: "pragma synchronous"},
		{name: "weaker synchronous allowed explicitly", omit: []string{"synchronous", "+synchronous(1)"}, role: SQLiteWriter, opts: []SQLiteOption{AllowSynchronous(1)}},
		{name: "reader is not query_only", role: SQLiteReader, omit: []string{"query_only"}, object: "pragma query_only"},
		{name: "unknown role", role: SQLiteRole("other"), object: "role other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, reader := openPools(t, 1, tt.omit...)
			db := writer
			if tt.role == SQLiteReader {
				db = reader
			}
			// A reader file must exist before the reader attaches.
			if tt.role == SQLiteReader {
				if _, err := writer.ExecContext(t.Context(), "CREATE TABLE x (a INTEGER) STRICT"); err != nil {
					t.Fatal(err)
				}
			}
			c := holdConns(t, db, 1)[0]
			got := describe(CheckSQLiteConnection(t.Context(), c, tt.role, tt.opts...))
			switch {
			case tt.object == "" && got != "":
				t.Fatalf("unexpected findings:\n%s", got)
			case tt.object != "" && !strings.Contains(got, tt.object):
				t.Fatalf("findings lack %q:\n%s", tt.object, got)
			}
		})
	}
}

// postgresSession opens a connection the way the application does (UTC, the
// timeouts, SET ROLE orcon_application) unless a setting is overridden.
func postgresSession(t *testing.T, params map[string]string, setRole bool) *pgx.Conn {
	t.Helper()
	cfg, err := pgx.ParseConfig(newPostgresDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.RuntimeParams["TimeZone"] = "UTC"
	cfg.RuntimeParams["idle_in_transaction_session_timeout"] = "10s"
	cfg.RuntimeParams["lock_timeout"] = "5s"
	for k, v := range params {
		cfg.RuntimeParams[k] = v
	}
	if setRole {
		cfg.AfterConnect = func(ctx context.Context, c *pgconn.PgConn) error {
			_, err := c.Exec(ctx, "SET ROLE orcon_application").ReadAll()
			return err
		}
	}
	conn, err := pgx.ConnectConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.WithoutCancel(t.Context())) })
	return conn
}

func TestPostgresSession(t *testing.T) {
	requirePostgres(t)
	tests := []struct {
		name   string
		params map[string]string
		role   bool
		setup  string // statement run before the transaction
		object string
	}{
		{name: "the application's session", role: true},
		{name: "wrong time zone", params: map[string]string{"TimeZone": "America/Bogota"}, role: true, object: "setting TimeZone"},
		{name: "not the application role", role: false, object: "current_user"},
		{name: "no idle timeout", params: map[string]string{"idle_in_transaction_session_timeout": "0"}, role: true, object: "idle_in_transaction_session_timeout"},
		{name: "no lock timeout", params: map[string]string{"lock_timeout": "0"}, role: true, object: "lock_timeout"},
		{name: "session-level organization leaked", role: true, setup: "SELECT set_config('orcon.org_id', '00000000-0000-0000-0000-000000000001', false)", object: "orcon.org_id"},
		{name: "serializable by default", params: map[string]string{"default_transaction_isolation": "serializable"}, role: true, object: "transaction_isolation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := postgresSession(t, tt.params, tt.role)
			if tt.setup != "" {
				if _, err := conn.Exec(t.Context(), tt.setup); err != nil {
					t.Fatal(err)
				}
			}
			tx, err := conn.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(context.WithoutCancel(t.Context())) }()
			got := describe(CheckPostgresSession(t.Context(), tx))
			switch {
			case tt.object == "" && got != "":
				t.Fatalf("unexpected findings:\n%s", got)
			case tt.object != "" && !strings.Contains(got, tt.object):
				t.Fatalf("findings lack %q:\n%s", tt.object, got)
			}
		})
	}
}
