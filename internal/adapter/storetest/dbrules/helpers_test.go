// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite" // SQLite driver of the tests.
)

// sqliteDSN builds the DSN of a file database with the per-connection
// settings of connections.md CN-2 to CN-4 applied through the driver, so every
// pooled connection inherits them. Settings named in omit are left out; an
// entry starting with + adds a pragma such as +synchronous(1).
func sqliteDSN(path string, writer bool, omit ...string) string {
	pragmas := []string{"foreign_keys(1)", "busy_timeout(5000)", "trusted_schema(0)"}
	if writer {
		pragmas = append(pragmas, "journal_mode(WAL)", "synchronous(2)")
	} else {
		pragmas = append(pragmas, "query_only(1)")
	}
	var keep []string
	for _, p := range pragmas {
		skip := false
		for _, o := range omit {
			if strings.HasPrefix(p, o+"(") {
				skip = true
			}
		}
		if !skip {
			keep = append(keep, "_pragma="+p)
		}
	}
	for _, o := range omit {
		if extra, ok := strings.CutPrefix(o, "+"); ok {
			keep = append(keep, "_pragma="+extra)
		}
	}
	return "file:" + path + "?" + strings.Join(keep, "&")
}

// openSQLite opens a new SQLite database file in the test's directory.
func openSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", sqliteDSN(filepath.Join(t.TempDir(), "test.db"), true))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return db
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // fixture paths come from the test tables
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mustPolicy(t *testing.T) *Policy {
	t.Helper()
	p, err := LoadPolicy()
	if err != nil {
		t.Fatalf("the repository policy does not load: %v", err)
	}
	return p
}

func rules(fs []Finding) []string {
	seen := map[string]bool{}
	for _, f := range fs {
		seen[f.Rule] = true
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func describe(fs []Finding) string {
	lines := make([]string, len(fs))
	for i, f := range fs {
		lines[i] = f.String()
	}
	return strings.Join(lines, "\n")
}

func execSQLite(t *testing.T, db *sql.DB, statements string) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), statements); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func caseDirs(t *testing.T, kind string) []string {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join("testdata", kind, "*"))
	if err != nil {
		t.Fatal(err)
	}
	return dirs
}

func expectedRules(t *testing.T, dir string) []string {
	t.Helper()
	return strings.Fields(readFile(t, filepath.Join(dir, "expect.txt")))
}

func hasDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}
