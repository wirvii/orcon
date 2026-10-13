// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadPlanQueries(t *testing.T, dir, dialect string) []Query {
	t.Helper()
	qs, err := LoadQueries(os.DirFS(dir), "queries", dialect)
	if err != nil {
		t.Fatal(err)
	}
	return qs
}

func hotQueries(qs []Query) []Query {
	var out []Query
	for _, q := range qs {
		if q.Hot {
			out = append(out, q)
		}
	}
	return out
}

func TestLoadQueries(t *testing.T) {
	sq := loadPlanQueries(t, "testdata/plans/good/sqlite", DialectSQLite)
	if len(sq) != 3 || len(hotQueries(sq)) != 2 {
		t.Fatalf("sqlite: %d queries, %d hot; want 3 and 2", len(sq), len(hotQueries(sq)))
	}
	list := sq[0]
	if list.Name != "ListThings" || !list.List() || !list.Ordered || !strings.HasSuffix(list.SQL, "LIMIT ?") {
		t.Errorf("ListThings = %+v", list)
	}
	pg := loadPlanQueries(t, "testdata/plans/good/postgres", DialectPostgres)
	if got := pg[1].SQL; !strings.HasSuffix(got, "WHERE id = $1") {
		t.Errorf("named parameter was not numbered: %q", got)
	}
	if _, err := LoadQueries(os.DirFS("testdata"), "no-such-dir", DialectSQLite); err == nil {
		t.Error("a missing queries directory must be an error")
	}
}

func TestPositionalParameters(t *testing.T) {
	tests := []struct {
		dialect, in, want string
	}{
		{DialectSQLite, "SELECT 1 WHERE a = @a AND b = sqlc.arg(b)", "SELECT 1 WHERE a = ? AND b = ?"},
		{DialectPostgres, "SELECT 1 WHERE a = $1 AND b = @b AND c = sqlc.arg(b) AND d = sqlc.narg(d)", "SELECT 1 WHERE a = $1 AND b = $2 AND c = $2 AND d = $3"},
		{DialectPostgres, "SELECT 'a@b' FROM t WHERE x = $1", "SELECT 'a@b' FROM t WHERE x = $1"},
	}
	for _, tt := range tests {
		got, err := positionalParameters(tt.in, tt.dialect)
		if err != nil || got != tt.want {
			t.Errorf("positionalParameters(%q, %s) = %q, %v; want %q", tt.in, tt.dialect, got, err, tt.want)
		}
	}
}

// planSQLite applies the schema of a plan fixture to a new database.
func planSQLite(t *testing.T, dir string) func() []Finding {
	t.Helper()
	db := openSQLite(t)
	execSQLite(t, db, readFile(t, filepath.Join(dir, "schema.sql")))
	qs := loadPlanQueries(t, dir, DialectSQLite)
	return func() []Finding { return CheckPlansSQLite(t.Context(), db, qs) }
}

func TestPlansIndexedQueryPassesSQLite(t *testing.T) {
	check := planSQLite(t, "testdata/plans/good/sqlite")
	if got := check(); len(got) != 0 {
		t.Fatalf("findings:\n%s", describe(got))
	}
}

// TestPlansDetectFullScan plants a hot query without an index: a full scan
// must be detected in both engines, and the indexed query must pass.
func TestPlansDetectFullScan(t *testing.T) {
	t.Run("sqlite", func(t *testing.T) {
		check := planSQLite(t, "testdata/plans/bad/full-scan/sqlite")
		got := check()
		if len(got) != 1 || !strings.Contains(got[0].Message, "full table scan of thing") {
			t.Fatalf("findings:\n%s", describe(got))
		}
	})
	t.Run("postgres", func(t *testing.T) {
		requirePostgres(t)
		conn := connectPostgres(t, newPostgresDatabase(t))
		execPostgres(t, conn, readFile(t, "testdata/plans/bad/full-scan/postgres/schema.sql"))
		qs := loadPlanQueries(t, "testdata/plans/bad/full-scan/postgres", DialectPostgres)
		got := CheckPlansPostgres(t.Context(), conn, qs, nil, nil)
		if len(got) != 1 || !strings.Contains(got[0].Message, "Seq Scan on thing") {
			t.Fatalf("findings:\n%s", describe(got))
		}
		// The indexed fixture passes on the same engine.
		conn = connectPostgres(t, newPostgresDatabase(t))
		execPostgres(t, conn, readFile(t, "testdata/plans/good/postgres/schema.sql"))
		good := loadPlanQueries(t, "testdata/plans/good/postgres", DialectPostgres)
		if got := CheckPlansPostgres(t.Context(), conn, good, nil, nil); len(got) != 0 {
			t.Fatalf("the indexed queries must pass:\n%s", describe(got))
		}
	})
}

func TestPlansDetectSort(t *testing.T) {
	t.Run("sqlite", func(t *testing.T) {
		check := planSQLite(t, "testdata/plans/bad/sort/sqlite")
		got := check()
		if len(got) != 1 || !strings.Contains(got[0].Message, "temporary b-tree") {
			t.Fatalf("findings:\n%s", describe(got))
		}
	})
	t.Run("postgres", func(t *testing.T) {
		requirePostgres(t)
		conn := connectPostgres(t, newPostgresDatabase(t))
		execPostgres(t, conn, readFile(t, "testdata/plans/bad/sort/postgres/schema.sql"))
		qs := loadPlanQueries(t, "testdata/plans/bad/sort/postgres", DialectPostgres)
		got := CheckPlansPostgres(t.Context(), conn, qs, nil, nil)
		if len(got) == 0 || !strings.Contains(describe(got), "sorts (Sort") {
			t.Fatalf("a sort step in a list was not detected:\n%s", describe(got))
		}
	})
}

func TestPlansIgnoreQueriesThatAreNotHot(t *testing.T) {
	db := openSQLite(t)
	execSQLite(t, db, readFile(t, "testdata/plans/good/sqlite/schema.sql"))
	// CountThingsByName scans, but it is not tagged hot.
	qs := []Query{{Name: "CountThingsByName", Command: ":one", SQL: "SELECT count(*) FROM thing WHERE name = ?"}}
	if got := CheckPlansSQLite(t.Context(), db, qs); len(got) != 0 {
		t.Fatalf("findings:\n%s", describe(got))
	}
}

func TestPlansReportABrokenQuery(t *testing.T) {
	db := openSQLite(t)
	execSQLite(t, db, readFile(t, "testdata/plans/good/sqlite/schema.sql"))
	qs := []Query{{Name: "Broken", Command: ":one", SQL: "SELECT nope FROM thing", Hot: true}}
	got := CheckPlansSQLite(t.Context(), db, qs)
	if len(got) != 1 || !strings.Contains(got[0].Message, "EXPLAIN QUERY PLAN failed") {
		t.Fatalf("a query that cannot be planned must be a finding:\n%s", describe(got))
	}
}

func TestFullScanLines(t *testing.T) {
	tests := []struct {
		detail string
		bad    bool
	}{
		{"SCAN thing", true},
		{"SCAN TABLE thing", true},
		{"SCAN thing AS t", true},
		{"SCAN thing USING INDEX ix_thing", false},
		{"SCAN thing USING COVERING INDEX ix_thing", false},
		{"SCAN thing USING INTEGER PRIMARY KEY", false},
		{"SEARCH thing USING INDEX ix_thing (org_id=?)", false},
		{"SCAN SUBQUERY 1", false},
		{"SCAN CONSTANT ROW", false},
		{"USE TEMP B-TREE FOR ORDER BY", false},
	}
	for _, tt := range tests {
		if _, bad := fullScan(tt.detail); bad != tt.bad {
			t.Errorf("fullScan(%q) = %v, want %v", tt.detail, bad, tt.bad)
		}
	}
}

func TestPlanFindingsWalkNestedPlans(t *testing.T) {
	plan, err := parsePlan(`[{"Plan": {"Node Type": "Limit", "Plans": [{"Node Type": "Sort", "Plans": [{"Node Type": "Seq Scan", "Relation Name": "thing"}]}]}}]`)
	if err != nil {
		t.Fatal(err)
	}
	if got := planFindings("query X", plan, true); len(got) != 2 {
		t.Fatalf("want a Seq Scan and a Sort finding, got:\n%s", describe(got))
	}
	if got := planFindings("query X", plan, false); len(got) != 1 {
		t.Fatalf("a sort is fine outside a list, got:\n%s", describe(got))
	}
	if _, err := parsePlan("not json"); err == nil {
		t.Error("unreadable plans must be an error")
	}
}

func TestEngineFunctionPlansPostgres(t *testing.T) {
	requirePostgres(t)
	dsn := newPostgresDatabase(t)
	run := func(t *testing.T, schemaFile string) []Finding {
		t.Helper()
		setup := connectPostgres(t, dsn)
		execPostgres(t, setup, readFile(t, schemaFile))
		conn, log, err := ConnectForPlans(t.Context(), dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close(t.Context()) //nolint:errcheck // test connection
		calls := []EngineCall{{Name: "orcon_engine.list_things", SQL: "SELECT * FROM orcon_engine.list_things(1, 10)", List: true}}
		return CheckPlansPostgres(t.Context(), conn, nil, calls, log)
	}
	t.Run("indexed function passes", func(t *testing.T) {
		if got := run(t, "testdata/plans/good/postgres/schema.sql"); len(got) != 0 {
			t.Fatalf("findings:\n%s", describe(got))
		}
	})
	t.Run("full scan inside the function is detected", func(t *testing.T) {
		dsn = newPostgresDatabase(t)
		got := run(t, "testdata/plans/bad/function-full-scan/postgres/schema.sql")
		if !strings.Contains(describe(got), "Seq Scan on thing") {
			t.Fatalf("a nested full scan was not detected:\n%s", describe(got))
		}
	})
	t.Run("a call needs the notice log", func(t *testing.T) {
		conn := connectPostgres(t, newPostgresDatabase(t))
		got := CheckPlansPostgres(t.Context(), conn, nil, []EngineCall{{Name: "f", SQL: "SELECT 1"}}, nil)
		if len(got) != 1 || !strings.Contains(got[0].Message, "NoticeLog") {
			t.Fatalf("findings:\n%s", describe(got))
		}
	})
}

func TestZeroValues(t *testing.T) {
	tests := map[string]string{
		"uuid":                     "'00000000-0000-0000-0000-000000000000'::uuid",
		"bigint":                   "0",
		"text":                     "''",
		"boolean":                  "false",
		"bytea":                    `'\x'::bytea`,
		"timestamp with time zone": "'epoch'::timestamptz",
		"uuid[]":                   "'{}'::uuid[]",
		"jsonb":                    "NULL::jsonb",
	}
	for typ, want := range tests {
		if got := zeroValue(typ); got != want {
			t.Errorf("zeroValue(%q) = %q, want %q", typ, got, want)
		}
	}
}
