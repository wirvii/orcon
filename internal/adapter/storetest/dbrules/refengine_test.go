// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
)

// The reference engines and migration runners below open and migrate databases
// the plain way for the fixtures of this package. SPEC-001 C2 supplies the real
// openers and the real runner (with schema_migration, checksum and
// compatibility floor) to the same harness.

// sqliteEngine is the reference Engine for SQLite.
type sqliteEngine struct {
	db      *sql.DB
	applied int
}

func (e *sqliteEngine) Name() string { return DialectSQLite }

func (e *sqliteEngine) Fresh(tb testing.TB) error {
	tb.Helper()
	if e.db != nil {
		if err := e.db.Close(); err != nil {
			return err
		}
	}
	db, err := sql.Open("sqlite", sqliteDSN(filepath.Join(tb.TempDir(), "upgrade.db"), true))
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	tb.Cleanup(func() { _ = db.Close() })
	e.db, e.applied = db, 0
	return nil
}

func (e *sqliteEngine) Exec(ctx context.Context, sqlText string) error {
	_, err := e.db.ExecContext(ctx, sqlText)
	return err
}

func (e *sqliteEngine) Query(ctx context.Context, statement string) ([][]string, error) {
	rows, err := e.db.QueryContext(ctx, statement)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only query
	return scanText(rows)
}

func scanText(rows *sql.Rows) ([][]string, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out [][]string
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]string, len(cols))
		for i, v := range values {
			row[i] = fmt.Sprint(v)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (e *sqliteEngine) OrganizationTables(ctx context.Context) ([]string, error) {
	rows, err := e.db.QueryContext(ctx, `SELECT m.name FROM sqlite_schema m, pragma_table_info(m.name) c WHERE m.type = 'table' AND c.name = 'org_id' ORDER BY m.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only query
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (e *sqliteEngine) CheckIntegrity(ctx context.Context) []Finding {
	return SQLiteIntegrity(ctx, e.db)
}

// postgresEngine is the reference Engine for PostgreSQL: each Fresh creates a
// database of its own.
type postgresEngine struct {
	t       *testing.T
	conn    *pgx.Conn
	applied int
}

func (e *postgresEngine) Name() string { return DialectPostgres }

func (e *postgresEngine) Fresh(testing.TB) error {
	if e.conn != nil {
		if err := e.conn.Close(context.WithoutCancel(e.t.Context())); err != nil {
			return err
		}
	}
	dsn := newPostgresDatabase(e.t)
	conn, err := pgx.Connect(e.t.Context(), dsn)
	if err != nil {
		return err
	}
	e.t.Cleanup(func() { _ = conn.Close(context.WithoutCancel(e.t.Context())) })
	e.conn, e.applied = conn, 0
	return nil
}

func (e *postgresEngine) Exec(ctx context.Context, sqlText string) error {
	_, err := e.conn.Exec(ctx, sqlText, pgx.QueryExecModeSimpleProtocol)
	return err
}

func (e *postgresEngine) Query(ctx context.Context, statement string) ([][]string, error) {
	rows, err := e.conn.Query(ctx, statement, pgx.QueryExecModeSimpleProtocol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		row := make([]string, len(values))
		for i, v := range values {
			row[i] = fmt.Sprint(v)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (e *postgresEngine) OrganizationTables(ctx context.Context) ([]string, error) {
	rows, err := e.conn.Query(ctx, `SELECT table_name FROM information_schema.columns WHERE table_schema = 'public' AND column_name = 'org_id' ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func (e *postgresEngine) CheckIntegrity(ctx context.Context) []Finding {
	return PostgresIntegrity(ctx, e.conn)
}

// referenceApplier returns a migration runner that executes the files of
// migrations in order, up to the requested number, on the engine's current
// database, and nothing else.
func referenceApplier(e Engine, migrations fs.FS) func(ctx context.Context, upTo int) error {
	return func(ctx context.Context, upTo int) error {
		var applied *int
		switch eng := e.(type) {
		case *sqliteEngine:
			applied = &eng.applied
		case *postgresEngine:
			applied = &eng.applied
		default:
			return fmt.Errorf("unknown engine %T", e)
		}
		for *applied < upTo {
			next := *applied + 1
			matches, err := fs.Glob(migrations, fmt.Sprintf("%04d_*.sql", next))
			if err != nil {
				return fmt.Errorf("migration %04d: %w", next, err)
			}
			if len(matches) != 1 {
				return fmt.Errorf("migration %04d: %d files", next, len(matches))
			}
			data, err := fs.ReadFile(migrations, matches[0])
			if err != nil {
				return err
			}
			if err := e.Exec(ctx, string(data)); err != nil {
				return fmt.Errorf("%s: %w", matches[0], err)
			}
			*applied = next
		}
		return nil
	}
}
