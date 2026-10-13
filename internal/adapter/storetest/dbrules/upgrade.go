// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Engine is the database the upgrade harness works on. One Engine value holds
// one current database; Fresh replaces it with a new empty one and the apply
// function given to RunUpgradeTests works on the current database.
type Engine interface {
	// Name is "sqlite" or "postgres".
	Name() string
	// Fresh replaces the current database with a new, empty one. Cleanup is
	// registered with t.
	Fresh(tb testing.TB) error
	// Exec runs SQL text that may hold several statements.
	Exec(ctx context.Context, sqlText string) error
	// Query runs one statement and returns its rows rendered as text.
	Query(ctx context.Context, statement string) ([][]string, error)
	// OrganizationTables lists the tables that have an org_id column.
	OrganizationTables(ctx context.Context) ([]string, error)
	// CheckIntegrity runs the engine's final integrity checks, for example
	// SQLiteIntegrity or PostgresIntegrity.
	CheckIntegrity(ctx context.Context) []Finding
}

var migrationFileRE = regexp.MustCompile(`^(\d{4})_[a-z][a-z0-9_]*\.sql$`)

// RunUpgradeTests proves that every migration upgrades seeded data
// (migrations.md MG-11, checks.md CK-13). For every migration N it starts
// from a new database, applies migrations 1..N-1 each followed by its
// NNNN_seed.sql, applies N, runs NNNN_verify.sql (every statement must return
// zero rows; a returned row is a failure with its text as the message) and
// runs the engine's integrity checks. The migrations file system holds the
// NNNN_<name>.sql files at its root, testdata the seed and verify files, and apply the
// migration runner: it applies every migration up to upTo that is not applied
// yet to the engine's current database (SPEC-001 C2 passes the real runner with
// schema_migration, checksum and compatibility floor; the fixtures use a runner
// that executes the files in order).
func RunUpgradeTests(t *testing.T, e Engine, migrations, testdata fs.FS, apply func(ctx context.Context, upTo int) error) {
	t.Helper()
	numbers, err := migrationNumbers(migrations)
	if err != nil {
		t.Fatalf("dbrules: %v", err)
	}
	for _, n := range numbers {
		t.Run(fmt.Sprintf("%s/%04d", e.Name(), n), func(t *testing.T) {
			findings, err := UpgradeFindings(t.Context(), t, e, n, migrations, testdata, apply)
			if err != nil {
				t.Fatalf("dbrules: upgrade to %04d: %v", n, err)
			}
			for _, f := range findings {
				t.Errorf("%s", f)
			}
		})
	}
}

func migrationNumbers(migrations fs.FS) ([]int, error) {
	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return nil, err
	}
	var out []int
	for _, e := range entries {
		if m := migrationFileRE.FindStringSubmatch(e.Name()); m != nil && !e.IsDir() {
			n, _ := strconv.Atoi(m[1])
			out = append(out, n)
		}
	}
	sort.Ints(out)
	if len(out) == 0 {
		return nil, errors.New("no migration files found")
	}
	for i, n := range out {
		if n != i+1 {
			return nil, fmt.Errorf("migrations are not contiguous from 0001: expected %04d, found %04d", i+1, n)
		}
	}
	return out, nil
}

// UpgradeFindings runs the upgrade to migration n and returns what is wrong
// with it. A returned error means the harness itself could not run (the
// database could not be created or the runner failed); a data or schema problem
// of the migrations under test is a finding.
func UpgradeFindings(ctx context.Context, tb testing.TB, e Engine, n int, migrations, testdata fs.FS, apply func(ctx context.Context, upTo int) error) ([]Finding, error) {
	tb.Helper()
	if err := e.Fresh(tb); err != nil {
		return nil, fmt.Errorf("new database: %w", err)
	}
	if _, err := migrationNumbers(migrations); err != nil {
		return nil, err
	}
	for k := 1; k <= n; k++ {
		before, err := e.OrganizationTables(ctx)
		if err != nil {
			return nil, err
		}
		if err := apply(ctx, k); err != nil {
			return nil, fmt.Errorf("apply migration %04d: %w", k, err)
		}
		if k < n {
			if found := seedFindings(ctx, e, k, testdata, before); len(found) > 0 {
				return found, nil // later steps would only repeat the failure
			}
			continue
		}
		out := runVerify(ctx, e, n, testdata)
		out = append(out, e.CheckIntegrity(ctx)...)
		// The seed of the newest migration is exercised too: no later upgrade runs it.
		return append(out, seedFindings(ctx, e, n, testdata, before)...), nil
	}
	return nil, nil
}

// seedFindings runs the seed file of migration k, which has just been applied,
// and checks that every organization table the migration created holds rows of
// two or more organizations. The list before names the organization tables that
// existed before the migration.
func seedFindings(ctx context.Context, e Engine, k int, testdata fs.FS, before []string) []Finding {
	seedName := fmt.Sprintf("%04d_seed.sql", k)
	object := fmt.Sprintf("migration %04d", k)
	data, err := fs.ReadFile(testdata, seedName)
	if err != nil {
		return []Finding{{Rule: "MG-11", Object: object, Message: "missing " + seedName}}
	}
	if err := e.Exec(ctx, string(data)); err != nil {
		return []Finding{{Rule: "MG-11", Object: object, Message: seedName + " does not apply: " + err.Error()}}
	}
	after, err := e.OrganizationTables(ctx)
	if err != nil {
		return []Finding{{Rule: "MG-11", Object: object, Message: "cannot list the organization tables: " + err.Error()}}
	}
	known := map[string]bool{}
	for _, t := range before {
		known[t] = true
	}
	var out []Finding
	for _, t := range after {
		if known[t] {
			continue
		}
		rows, err := e.Query(ctx, `SELECT count(DISTINCT org_id) FROM "`+strings.ReplaceAll(t, `"`, `""`)+`"`)
		if err != nil || len(rows) != 1 || len(rows[0]) != 1 {
			out = append(out, Finding{Rule: "MG-11", Object: object, Message: fmt.Sprintf("cannot count the organizations in table %s: %v", t, err)})
			continue
		}
		if count, _ := strconv.Atoi(rows[0][0]); count < 2 {
			out = append(out, Finding{Rule: "MG-11", Object: object, Message: fmt.Sprintf("%s leaves %d organization(s) in table %s; seeds put rows of two organizations in every organization table", seedName, count, t)})
		}
	}
	return out
}

// runVerify runs the verify file of migration n: every statement must return
// zero rows.
func runVerify(ctx context.Context, e Engine, n int, testdata fs.FS) []Finding {
	name := fmt.Sprintf("%04d_verify.sql", n)
	object := fmt.Sprintf("migration %04d", n)
	data, err := fs.ReadFile(testdata, name)
	if err != nil {
		return []Finding{{Rule: "MG-11", Object: object, Message: "missing " + name}}
	}
	statements, err := splitStatements(string(data))
	if err != nil {
		return []Finding{{Rule: "MG-11", Object: object, Message: name + " cannot be read: " + err.Error()}}
	}
	if len(statements) == 0 {
		return []Finding{{Rule: "MG-11", Object: object, Message: name + " holds no statement"}}
	}
	var out []Finding
	for i, s := range statements {
		rows, err := e.Query(ctx, s)
		if err != nil {
			out = append(out, Finding{Rule: "MG-11", Object: object, Message: fmt.Sprintf("%s statement %d fails: %v", name, i+1, err)})
			continue
		}
		for _, r := range rows {
			out = append(out, Finding{Rule: "MG-11", Object: object, Message: fmt.Sprintf("%s statement %d returned a row: %s", name, i+1, strings.Join(r, " | "))})
		}
	}
	return out
}

// SQLiteIntegrity runs PRAGMA foreign_key_check and PRAGMA integrity_check.
func SQLiteIntegrity(ctx context.Context, db *sql.DB) []Finding {
	out := foreignKeyCheck(ctx, db)
	var result string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return append(out, Finding{Rule: "MG-11", Object: "integrity_check", Message: err.Error()})
	}
	if result != "ok" {
		out = append(out, Finding{Rule: "MG-11", Object: "database", Message: "PRAGMA integrity_check: " + result})
	}
	return out
}

func foreignKeyCheck(ctx context.Context, db *sql.DB) []Finding {
	rows, err := db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return []Finding{{Rule: "MG-11", Object: "foreign_key_check", Message: err.Error()}}
	}
	defer rows.Close() //nolint:errcheck // read-only query
	var out []Finding
	for rows.Next() {
		var table, parent string
		var rowid, fkid sql.NullInt64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return append(out, Finding{Rule: "MG-11", Object: "foreign_key_check", Message: err.Error()})
		}
		out = append(out, Finding{Rule: "MG-11", Object: "table " + table, Message: "a row refers to a missing row of " + parent + " (PRAGMA foreign_key_check)"})
	}
	if err := rows.Err(); err != nil {
		out = append(out, Finding{Rule: "MG-11", Object: "foreign_key_check", Message: err.Error()})
	}
	return out
}

// PostgresIntegrity reports every constraint that is not validated.
func PostgresIntegrity(ctx context.Context, conn *pgx.Conn) []Finding {
	rows, err := conn.Query(ctx, `SELECT conrelid::regclass::text, conname FROM pg_constraint WHERE NOT convalidated ORDER BY 1, 2`)
	if err != nil {
		return []Finding{{Rule: "MG-11", Object: "pg_constraint", Message: err.Error()}}
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		var table, name string
		if err := rows.Scan(&table, &name); err != nil {
			return append(out, Finding{Rule: "MG-11", Object: "pg_constraint", Message: err.Error()})
		}
		out = append(out, Finding{Rule: "MG-11", Object: "constraint " + name, Message: "table " + table + ": the constraint is not validated"})
	}
	if err := rows.Err(); err != nil {
		out = append(out, Finding{Rule: "MG-11", Object: "pg_constraint", Message: err.Error()})
	}
	return out
}
