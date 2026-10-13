// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"path/filepath"
	"reflect"
	"testing"
)

// wantSchemaCases is the number of planted schema violations of SPEC-009 4.4.6.
const wantSchemaCases = 22

func TestSchemaLintGoodSQLite(t *testing.T) {
	db := openSQLite(t)
	execSQLite(t, db, readFile(t, "testdata/good/sqlite/schema.sql"))
	if got := LintSQLite(t.Context(), db, mustPolicy(t)); len(got) != 0 {
		t.Fatalf("the good fixture has findings:\n%s", describe(got))
	}
}

func TestSchemaLintPlantedSQLite(t *testing.T) {
	dirs := caseDirs(t, "bad")
	if len(dirs) != wantSchemaCases {
		t.Fatalf("found %d planted schema cases, SPEC-009 4.4.6 requires %d", len(dirs), wantSchemaCases)
	}
	ran := 0
	for _, dir := range dirs {
		if !hasDir(filepath.Join(dir, "sqlite")) {
			continue
		}
		ran++
		t.Run(filepath.Base(dir), func(t *testing.T) {
			db := openSQLite(t)
			execSQLite(t, db, readFile(t, filepath.Join(dir, "sqlite", "schema.sql")))
			got := LintSQLite(t.Context(), db, mustPolicy(t))
			if want := expectedRules(t, dir); !reflect.DeepEqual(rules(got), want) {
				t.Fatalf("rules = %v, want exactly %v\n%s", rules(got), want, describe(got))
			}
		})
	}
	if ran == 0 {
		t.Fatal("no SQLite case ran")
	}
}

func TestSchemaLintGoodPostgres(t *testing.T) {
	conn := connectPostgres(t, newPostgresDatabase(t))
	execPostgres(t, conn, readFile(t, "testdata/good/postgres/schema.sql"))
	if got := LintPostgres(t.Context(), conn, mustPolicy(t)); len(got) != 0 {
		t.Fatalf("the good fixture has findings:\n%s", describe(got))
	}
}

func TestSchemaLintPlantedPostgres(t *testing.T) {
	requirePostgres(t)
	dirs := caseDirs(t, "bad")
	if len(dirs) != wantSchemaCases {
		t.Fatalf("found %d planted schema cases, SPEC-009 4.4.6 requires %d", len(dirs), wantSchemaCases)
	}
	for _, dir := range dirs {
		if !hasDir(filepath.Join(dir, "postgres")) {
			continue
		}
		t.Run(filepath.Base(dir), func(t *testing.T) {
			conn := connectPostgres(t, newPostgresDatabase(t))
			execPostgres(t, conn, readFile(t, filepath.Join(dir, "postgres", "schema.sql")))
			got := LintPostgres(t.Context(), conn, mustPolicy(t))
			if want := expectedRules(t, dir); !reflect.DeepEqual(rules(got), want) {
				t.Fatalf("rules = %v, want exactly %v\n%s", rules(got), want, describe(got))
			}
		})
	}
}
