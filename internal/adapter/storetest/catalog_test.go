// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/adapter/storetest"
)

// table is what the schema says about one table of the database.
type table struct {
	name string
	// hasOrgID is true when the table has a column named org_id; orgIDRequired
	// is true when that column is NOT NULL.
	hasOrgID      bool
	orgIDRequired bool
}

// foreignKey is one foreign key, columns in constraint order.
type foreignKey struct {
	table, refTable string
	columns, refs   []string
	onDelete        string // CASCADE, RESTRICT, SET NULL, NO ACTION
}

// catalog reads tables and foreign keys from the real database, whichever store it is.
type catalog struct {
	tables []table
	fks    []foreignKey
	// actorColumns are the columns named <verb>_by_principal_id, as "table.column", sorted.
	actorColumns []string
}

func readCatalog(ctx context.Context, tb testing.TB, s storetest.Store) catalog {
	tb.Helper()
	switch v := s.(type) {
	case *sqlite.Store:
		return readSQLiteCatalog(ctx, tb, v.Reader())
	case *postgres.Store:
		return readPostgresCatalog(ctx, tb, v)
	default:
		tb.Fatalf("no catalog for %T", s)
		return catalog{}
	}
}

// sqliteRows runs a query and hands every row to scan; the rows are closed when it returns.
func sqliteRows(ctx context.Context, tb testing.TB, db *sql.DB, query string, scan func(*sql.Rows) error) {
	tb.Helper()
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		tb.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // a read-only cursor; the error of Err below is the one that matters
	for rows.Next() {
		if err := scan(rows); err != nil {
			tb.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		tb.Fatal(err)
	}
}

func readSQLiteCatalog(ctx context.Context, tb testing.TB, db *sql.DB) catalog {
	tb.Helper()
	var names []string
	sqliteRows(ctx, tb, db, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name", func(rows *sql.Rows) error {
		var n string
		names = append(names, n)
		return rows.Scan(&names[len(names)-1])
	})
	var cat catalog
	for _, n := range names {
		t := table{name: n}
		sqliteRows(ctx, tb, db, fmt.Sprintf("SELECT name, \"notnull\" FROM pragma_table_info('%s')", n), func(rows *sql.Rows) error {
			var col string
			var notNull int
			if err := rows.Scan(&col, &notNull); err != nil {
				return err
			}
			if col == "org_id" {
				t.hasOrgID, t.orgIDRequired = true, notNull == 1
			}
			if strings.HasSuffix(col, "_by_principal_id") {
				cat.actorColumns = append(cat.actorColumns, n+"."+col)
			}
			return nil
		})
		cat.tables = append(cat.tables, t)

		byID := map[int]*foreignKey{}
		var order []int
		sqliteRows(ctx, tb, db, fmt.Sprintf("SELECT id, seq, \"table\", \"from\", \"to\", on_delete FROM pragma_foreign_key_list('%s') ORDER BY id, seq", n), func(rows *sql.Rows) error {
			var fkID, seq int
			var ref, from, to, onDelete string
			if err := rows.Scan(&fkID, &seq, &ref, &from, &to, &onDelete); err != nil {
				return err
			}
			fk, ok := byID[fkID]
			if !ok {
				fk = &foreignKey{table: n, refTable: ref, onDelete: onDelete}
				byID[fkID] = fk
				order = append(order, fkID)
			}
			fk.columns = append(fk.columns, from)
			fk.refs = append(fk.refs, to)
			return nil
		})
		for _, k := range order {
			cat.fks = append(cat.fks, *byID[k])
		}
	}
	sort.Strings(cat.actorColumns)
	return cat
}

// pgRows runs a query and hands every row to scan; the rows are closed when it returns.
func pgRows(ctx context.Context, tb testing.TB, pool *pgxpool.Pool, query string, scan func(pgx.Rows) error) {
	tb.Helper()
	rows, err := pool.Query(ctx, query)
	if err != nil {
		tb.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			tb.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		tb.Fatal(err)
	}
}

func readPostgresCatalog(ctx context.Context, tb testing.TB, s *postgres.Store) catalog {
	tb.Helper()
	pool := s.Pool()
	var cat catalog
	pgRows(ctx, tb, pool, `SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r' ORDER BY c.relname`, func(rows pgx.Rows) error {
		var n string
		if err := rows.Scan(&n); err != nil {
			return err
		}
		cat.tables = append(cat.tables, table{name: n})
		return nil
	})
	byName := map[string]*table{}
	for i := range cat.tables {
		byName[cat.tables[i].name] = &cat.tables[i]
	}
	pgRows(ctx, tb, pool, `SELECT c.relname, a.attnotnull FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r' AND a.attname = 'org_id' AND a.attnum > 0 AND NOT a.attisdropped`, func(rows pgx.Rows) error {
		var n string
		var notNull bool
		if err := rows.Scan(&n, &notNull); err != nil {
			return err
		}
		byName[n].hasOrgID, byName[n].orgIDRequired = true, notNull
		return nil
	})
	pgRows(ctx, tb, pool, `SELECT c.relname || '.' || a.attname FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r' AND a.attname LIKE '%\_by\_principal\_id' AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY 1`, func(rows pgx.Rows) error {
		var column string
		if err := rows.Scan(&column); err != nil {
			return err
		}
		cat.actorColumns = append(cat.actorColumns, column)
		return nil
	})
	pgRows(ctx, tb, pool, `SELECT c.conrelid::regclass::text, c.confrelid::regclass::text,
		ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY AS k(attnum, ord)
		      JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k.attnum ORDER BY k.ord),
		ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY AS k(attnum, ord)
		      JOIN pg_attribute a ON a.attrelid = c.confrelid AND a.attnum = k.attnum ORDER BY k.ord),
		CASE c.confdeltype WHEN 'c' THEN 'CASCADE' WHEN 'r' THEN 'RESTRICT' WHEN 'n' THEN 'SET NULL' WHEN 'a' THEN 'NO ACTION' ELSE 'OTHER' END
		FROM pg_constraint c JOIN pg_namespace n ON n.oid = c.connamespace
		WHERE c.contype = 'f' AND n.nspname = 'public' ORDER BY 1, 2`, func(rows pgx.Rows) error {
		var fk foreignKey
		if err := rows.Scan(&fk.table, &fk.refTable, &fk.columns, &fk.refs, &fk.onDelete); err != nil {
			return err
		}
		cat.fks = append(cat.fks, fk)
		return nil
	})
	return cat
}

// orgTables returns the names of the tables with an org_id column.
func (c catalog) orgTables() []string {
	var out []string
	for _, t := range c.tables {
		if t.hasOrgID {
			out = append(out, t.name)
		}
	}
	sort.Strings(out)
	return out
}

func (c catalog) table(name string) (table, bool) {
	for _, t := range c.tables {
		if t.name == name {
			return t, true
		}
	}
	return table{}, false
}

func (fk foreignKey) String() string {
	return fmt.Sprintf("%s(%s) -> %s(%s) ON DELETE %s", fk.table, strings.Join(fk.columns, ", "), fk.refTable, strings.Join(fk.refs, ", "), fk.onDelete)
}

func sameStrings(a, b []string) bool { return slices.Equal(a, b) }
