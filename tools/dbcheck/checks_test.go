// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// checkCase is one row of a check's table test: the files of a scratch tree,
// the check to run and either the expected rule ids (all findings must be of
// those rules) or none (no finding at all).
type checkCase struct {
	name     string
	files    map[string]string
	policy   map[string]*string
	rules    []string
	contains string // substring of one finding
}

func runCases(t *testing.T, check string, cases []checkCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tree(t, tt.files, tt.policy)
			findings, _, code := runCheck(t, check, cfg)
			if got := rulesOf(findings); !reflect.DeepEqual(got, tt.rules) && !(len(got) == 0 && len(tt.rules) == 0) {
				t.Fatalf("rules = %v, want %v\nfindings:\n%s", got, tt.rules, strings.Join(findings, "\n"))
			}
			if (code == 1) != (len(tt.rules) > 0) {
				t.Fatalf("exit code %d with rules %v", code, tt.rules)
			}
			if tt.contains != "" && !contains(findings, tt.contains) {
				t.Fatalf("findings lack %q:\n%s", tt.contains, strings.Join(findings, "\n"))
			}
		})
	}
}

// pair returns the files of a migration pair with seeds and verifies in both
// dialects (the body is the same SQL for both).
func pair(n int, name, header, body string) map[string]string {
	f := map[string]string{}
	for _, d := range []string{dialectSQLite, dialectPostgres} {
		f[fmt.Sprintf("%s/migrations/%04d_%s.sql", d, n, name)] = sqlHeader + header + body
		f[fmt.Sprintf("%s/migrations/testdata/%04d_seed.sql", d, n)] = sqlHeader + "SELECT 1;\n"
		f[fmt.Sprintf("%s/migrations/testdata/%04d_verify.sql", d, n)] = sqlHeader + "SELECT 1 WHERE 1 = 0;\n"
	}
	return f
}

func merge(parts ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, p := range parts {
		for k, v := range p {
			out[k] = v
		}
	}
	return out
}

const additive = "-- orcon:kind additive\n-- orcon:compatibility-floor self\n"

// withSums adds a correct migrations.sum to both dialect directories.
func withSums(files map[string]string) map[string]string {
	out := merge(files)
	for _, d := range []string{dialectSQLite, dialectPostgres} {
		var b strings.Builder
		b.WriteString("# header\n")
		for name, content := range files {
			prefix := d + "/migrations/"
			if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".sql") && !strings.Contains(name, "/testdata/") {
				fmt.Fprintf(&b, "%s  %s\n", strings.TrimPrefix(name, prefix), fileHash([]byte(content)))
			}
		}
		out[d+"/migrations/migrations.sum"] = b.String()
	}
	return out
}

func TestMigrationNames(t *testing.T) {
	runCases(t, "migration-names", []checkCase{
		{name: "valid pair", files: pair(1, "create_org", additive, "SELECT 1;\n")},
		{name: "uppercase name", files: pair(1, "Create_org", additive, ""), rules: []string{"NM-15"}},
		{name: "starts with a digit after the number", files: pair(1, "2fa", additive, ""), rules: []string{"NM-15"}},
		{name: "double underscore", files: pair(1, "create__org", additive, ""), rules: []string{"NM-15"}},
		{name: "abbreviation in the description", files: pair(1, "add_retry_cnt", additive, ""), rules: []string{"NM-3"}, contains: `abbreviation "cnt"`},
		{name: "allowed acronym in the description", files: pair(1, "create_api_key", additive, "")},
		{name: "gap", files: merge(pair(1, "a_one", additive, ""), pair(3, "a_three", additive, "")), rules: []string{"MG-1"}, contains: "expected 0002, found 0003"},
		{name: "does not start at 0001", files: pair(2, "a_two", additive, ""), rules: []string{"MG-1"}, contains: "expected 0001"},
		{name: "duplicate number", files: merge(pair(1, "a_one", additive, ""), pair(1, "b_one", additive, "")), rules: []string{"MG-1"}, contains: "duplicate migration number 0001"},
		{name: "contiguous", files: merge(pair(1, "a_one", additive, ""), pair(2, "a_two", additive, ""), pair(3, "a_three", additive, ""))},
	})
}

func TestMigrationNamesRequiresPolicy(t *testing.T) {
	cfg := tree(t, pair(1, "create_org", additive, ""), map[string]*string{fileDenylist: nil})
	findings, _, code := runCheck(t, "migration-names", cfg)
	if code != 1 || !contains(findings, "policy file is missing") {
		// A check that cannot load its list must fail by saying so: run through policy.
		pf, _, pcode := runCheck(t, "policy", cfg)
		if pcode != 1 || !contains(pf, "policy file is missing") {
			t.Fatalf("a missing list was not reported: %v / %v", findings, pf)
		}
	}
}

func TestMigrationPairs(t *testing.T) {
	only := func(dialect string, files map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range files {
			if strings.HasPrefix(k, dialect+"/") {
				out[k] = v
			}
		}
		return out
	}
	runCases(t, "migration-pairs", []checkCase{
		{name: "complete pair", files: pair(1, "a", additive, "")},
		{name: "sqlite only", files: only(dialectSQLite, pair(1, "a", additive, "")), rules: []string{"MG-1"}, contains: "no pair in the postgres directory"},
		{name: "postgres only", files: only(dialectPostgres, pair(1, "a", additive, "")), rules: []string{"MG-1"}, contains: "no pair in the sqlite directory"},
		{name: "different names", files: merge(only(dialectSQLite, pair(1, "a", additive, "")), only(dialectPostgres, pair(1, "b", additive, ""))), rules: []string{"MG-1"}, contains: "different names"},
		{name: "different kinds", files: merge(only(dialectSQLite, pair(1, "a", additive, "")), only(dialectPostgres, pair(1, "a", "-- orcon:kind expand\n-- orcon:compatibility-floor self\n", ""))), rules: []string{"MG-2"}, contains: "different kind headers"},
		{name: "different floors", files: merge(only(dialectSQLite, pair(1, "a", additive, "")), only(dialectPostgres, pair(1, "a", "-- orcon:kind additive\n-- orcon:compatibility-floor 1.2.0\n", ""))), rules: []string{"MG-2"}, contains: "compatibility-floor headers"},
		{name: "no-transaction does not count as a difference", files: merge(only(dialectSQLite, pair(1, "a", additive, "")), only(dialectPostgres, pair(1, "a", additive+"-- orcon:no-transaction\n", "")))},
		{name: "seed missing", files: func() map[string]string {
			f := pair(1, "a", additive, "")
			delete(f, "sqlite/migrations/testdata/0001_seed.sql")
			return f
		}(), rules: []string{"MG-11"}, contains: "0001_seed.sql"},
		{name: "verify missing in postgres", files: func() map[string]string {
			f := pair(1, "a", additive, "")
			delete(f, "postgres/migrations/testdata/0001_verify.sql")
			return f
		}(), rules: []string{"MG-11"}, contains: "0001_verify.sql"},
	})
}

func TestMigrationHeaders(t *testing.T) {
	hdr := func(lines ...string) string { return strings.Join(lines, "\n") + "\n" }
	runCases(t, "migration-headers", []checkCase{
		{name: "complete", files: pair(1, "a", additive, "")},
		{name: "kind missing", files: pair(1, "a", hdr("-- orcon:compatibility-floor self"), ""), rules: []string{"MG-2"}, contains: "missing \"-- orcon:kind\""},
		{name: "floor missing", files: pair(1, "a", hdr("-- orcon:kind additive"), ""), rules: []string{"MG-2"}, contains: "missing \"-- orcon:compatibility-floor\""},
		{name: "unknown kind", files: pair(1, "a", hdr("-- orcon:kind sideways", "-- orcon:compatibility-floor self"), ""), rules: []string{"MG-2"}, contains: "unknown kind"},
		{name: "floor not a version", files: pair(1, "a", hdr("-- orcon:kind additive", "-- orcon:compatibility-floor soon"), ""), rules: []string{"MG-2"}, contains: "neither a semantic version"},
		{name: "semantic floor", files: pair(1, "a", hdr("-- orcon:kind additive", "-- orcon:compatibility-floor 1.4.0"), "")},
		{name: "contract without contracts", files: pair(1, "a", hdr("-- orcon:kind contract", "-- orcon:compatibility-floor self"), ""), rules: []string{"MG-2"}, contains: "needs \"-- orcon:contracts NNNN\""},
		{name: "contracts on additive", files: pair(1, "a", additive+"-- orcon:contracts 0001\n", ""), rules: []string{"MG-2"}, contains: "only valid with kind contract"},
		{name: "contracts malformed", files: pair(1, "a", hdr("-- orcon:kind contract", "-- orcon:compatibility-floor self", "-- orcon:contracts 12"), ""), rules: []string{"MG-2"}, contains: "not a four-digit"},
		{name: "unknown key", files: pair(1, "a", additive+"-- orcon:flavour sweet\n", ""), rules: []string{"MG-2"}, contains: "unknown header key"},
		{name: "repeated key", files: pair(1, "a", additive+"-- orcon:kind data\n", ""), rules: []string{"MG-2"}, contains: "repeated"},
		{name: "no-transaction in sqlite", files: map[string]string{
			"sqlite/migrations/0001_a.sql": sqlHeader + additive + "-- orcon:no-transaction\nCREATE INDEX ix_a_b ON a (b);\n",
		}, rules: []string{"MG-2"}, contains: "valid only in PostgreSQL"},
		{name: "no-transaction with one concurrent index", files: map[string]string{
			"postgres/migrations/0001_a.sql": sqlHeader + additive + "-- orcon:no-transaction\nCREATE INDEX CONCURRENTLY IF NOT EXISTS ix_a_b ON a (b);\n",
		}},
		{name: "no-transaction with two statements", files: map[string]string{
			"postgres/migrations/0001_a.sql": sqlHeader + additive + "-- orcon:no-transaction\nCREATE INDEX CONCURRENTLY ix_a_b ON a (b);\nCREATE INDEX CONCURRENTLY ix_a_c ON a (c);\n",
		}, rules: []string{"MG-9"}},
		{name: "concurrent index without the header", files: map[string]string{
			"postgres/migrations/0001_a.sql": sqlHeader + additive + "CREATE INDEX CONCURRENTLY ix_a_b ON a (b);\n",
		}, rules: []string{"MG-9"}, contains: "cannot run in a transaction"},
	})
}

func TestMigrationImmutableAgainstSumOnly(t *testing.T) {
	good := withSums(pair(1, "a", additive, "SELECT 1;\n"))
	edited := withSums(pair(1, "a", additive, "SELECT 1;\n"))
	edited["sqlite/migrations/0001_a.sql"] += "-- edited\n"
	unlisted := withSums(pair(1, "a", additive, "SELECT 1;\n"))
	unlisted["sqlite/migrations/0002_b.sql"] = sqlHeader + additive
	removed := withSums(pair(1, "a", additive, "SELECT 1;\n"))
	delete(removed, "postgres/migrations/0001_a.sql")
	badLine := withSums(pair(1, "a", additive, "SELECT 1;\n"))
	badLine["sqlite/migrations/migrations.sum"] += "garbage\n"
	dup := withSums(pair(1, "a", additive, "SELECT 1;\n"))
	dup["sqlite/migrations/migrations.sum"] += "0001_a.sql  " + strings.Repeat("0", 64) + "\n"
	noSum := pair(1, "a", additive, "")
	runCases(t, "migration-immutable", []checkCase{
		{name: "matches the sums", files: good},
		{name: "edited file", files: edited, rules: []string{"MG-5"}, contains: "checksum differs"},
		{name: "unlisted file", files: unlisted, rules: []string{"MG-5"}, contains: "not listed in migrations.sum"},
		{name: "removed file", files: removed, rules: []string{"MG-5"}, contains: "the file is missing"},
		{name: "malformed line", files: badLine, rules: []string{"MG-5"}, contains: "malformed line"},
		{name: "duplicate line", files: dup, rules: []string{"MG-5"}, contains: "listed twice"},
		{name: "sum file missing", files: noSum, rules: []string{"MG-5"}, contains: "migrations.sum is missing"},
	})
}

func TestMigrationImmutableDeclaresMissingBase(t *testing.T) {
	cfg := tree(t, withSums(pair(1, "a", additive, "")), nil)
	_, notes, code := runCheck(t, "migration-immutable", cfg)
	if code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !contains(notes, "no base revision available") || !contains(notes, "compared only against migrations.sum") {
		t.Fatalf("the missing base was not declared: %v", notes)
	}
}

func TestExpandContract(t *testing.T) {
	k := func(kind string) string { return "-- orcon:kind " + kind + "\n-- orcon:compatibility-floor self\n" }
	runCases(t, "migration-expand-contract", []checkCase{
		{name: "create table is additive", files: pair(1, "a", additive, "CREATE TABLE a (id BLOB NOT NULL);\n")},
		{name: "drop table", files: pair(1, "a", additive, "DROP TABLE a;\n"), rules: []string{"MG-7"}, contains: "DROP TABLE"},
		{name: "drop column", files: pair(1, "a", additive, "ALTER TABLE a DROP COLUMN b;\n"), rules: []string{"MG-7"}, contains: "DROP COLUMN"},
		{name: "rename table", files: pair(1, "a", additive, "ALTER TABLE a RENAME TO b;\n"), rules: []string{"MG-7"}, contains: "RENAME"},
		{name: "rename column", files: pair(1, "a", additive, "ALTER TABLE a RENAME COLUMN b TO c;\n"), rules: []string{"MG-7"}, contains: "RENAME"},
		{name: "alter column type", files: pair(1, "a", additive, "ALTER TABLE a ALTER COLUMN b TYPE bigint;\n"), rules: []string{"MG-7"}, contains: "TYPE"},
		{name: "alter column set data type", files: pair(1, "a", additive, "ALTER TABLE a ALTER COLUMN b SET DATA TYPE bigint;\n"), rules: []string{"MG-7"}, contains: "TYPE"},
		{name: "alter column set default is fine", files: pair(1, "a", additive, "ALTER TABLE a ALTER COLUMN b SET DEFAULT 1;\n")},
		{name: "set not null", files: pair(1, "a", additive, "ALTER TABLE a ALTER COLUMN b SET NOT NULL;\n"), rules: []string{"MG-7"}, contains: "SET NOT NULL"},
		{name: "add column not null without default", files: pair(1, "a", additive, "ALTER TABLE a ADD COLUMN b TEXT NOT NULL;\n"), rules: []string{"MG-7"}, contains: "NOT NULL without DEFAULT"},
		{name: "add column not null with default", files: pair(1, "a", additive, "ALTER TABLE a ADD COLUMN b TEXT NOT NULL DEFAULT '';\n")},
		{name: "add nullable column", files: pair(1, "a", additive, "ALTER TABLE a ADD COLUMN b TEXT;\n")},
		{name: "add constraint is not a column", files: pair(1, "a", additive, "ALTER TABLE a ADD CONSTRAINT uq_a_b UNIQUE (b);\n")},
		{name: "if not exists is not NOT NULL", files: pair(1, "a", additive, "ALTER TABLE a ADD COLUMN IF NOT EXISTS b TEXT;\n")},
		{name: "words in strings and comments are ignored", files: pair(1, "a", additive, "-- DROP TABLE a;\nSELECT 'DROP TABLE a';\n")},
		{name: "rebuild in sqlite may drop and rename back", files: map[string]string{
			"sqlite/migrations/0001_a.sql": sqlHeader + k("rebuild") + "CREATE TABLE a_new (id BLOB NOT NULL) STRICT;\nDROP TABLE a;\nALTER TABLE a_new RENAME TO a;\n",
		}},
		{name: "rebuild in sqlite: drop without renaming back", files: map[string]string{
			"sqlite/migrations/0001_a.sql": sqlHeader + k("rebuild") + "CREATE TABLE a_new (id BLOB NOT NULL) STRICT;\nDROP TABLE a;\n",
		}, rules: []string{"MG-7"}, contains: "renamed back"},
		{name: "rebuild in sqlite: rename to another name", files: map[string]string{
			"sqlite/migrations/0001_a.sql": sqlHeader + k("rebuild") + "CREATE TABLE a_new (id BLOB NOT NULL) STRICT;\nDROP TABLE a;\nALTER TABLE a_new RENAME TO b;\n",
		}, rules: []string{"MG-7"}},
		{name: "rebuild in sqlite: rename without a drop", files: map[string]string{
			"sqlite/migrations/0001_a.sql": sqlHeader + k("rebuild") + "ALTER TABLE a RENAME TO b;\n",
		}, rules: []string{"MG-7"}},
		{name: "rebuild in sqlite: rename column", files: map[string]string{
			"sqlite/migrations/0001_a.sql": sqlHeader + k("rebuild") + "CREATE TABLE a_new (id BLOB NOT NULL) STRICT;\nDROP TABLE a;\nALTER TABLE a_new RENAME TO a;\nALTER TABLE a RENAME COLUMN x TO y;\n",
		}, rules: []string{"MG-7"}, contains: "RENAME"},
		{name: "rebuild in sqlite: drop column", files: map[string]string{
			"sqlite/migrations/0001_a.sql": sqlHeader + k("rebuild") + "ALTER TABLE a DROP COLUMN x;\n",
		}, rules: []string{"MG-7"}},
		{name: "rebuild in sqlite: alter column type", files: map[string]string{
			"sqlite/migrations/0001_a.sql": sqlHeader + k("rebuild") + "ALTER TABLE a ALTER COLUMN x TYPE bigint;\n",
		}, rules: []string{"MG-7"}},
		{name: "rebuild in postgres may not drop", files: map[string]string{
			"postgres/migrations/0001_a.sql": sqlHeader + k("rebuild") + "DROP TABLE a;\n",
		}, rules: []string{"MG-7"}},
		{name: "rebuild in postgres may not rename back", files: map[string]string{
			"postgres/migrations/0001_a.sql": sqlHeader + k("rebuild") + "CREATE TABLE a_new (id uuid NOT NULL);\nDROP TABLE a;\nALTER TABLE a_new RENAME TO a;\n",
		}, rules: []string{"MG-7"}},
		{name: "contract may drop", files: merge(
			pair(1, "a", k("expand"), "ALTER TABLE a ADD COLUMN b TEXT;\n"),
			pair(2, "b", k("contract")+"-- orcon:contracts 0001\n", "ALTER TABLE a DROP COLUMN c;\n"))},
		{name: "contract of a missing migration", files: pair(2, "b", k("contract")+"-- orcon:contracts 0001\n", "ALTER TABLE a DROP COLUMN c;\n"), rules: []string{"MG-6"}, contains: "does not exist"},
		{name: "contract of a non-expand migration", files: merge(
			pair(1, "a", additive, "ALTER TABLE a ADD COLUMN b TEXT;\n"),
			pair(2, "b", k("contract")+"-- orcon:contracts 0001\n", "ALTER TABLE a DROP COLUMN c;\n")), rules: []string{"MG-6"}, contains: "not expand"},
	})
}

func TestForbiddenDDL(t *testing.T) {
	sqliteOnly := func(body string) map[string]string {
		return map[string]string{"sqlite/migrations/0001_a.sql": sqlHeader + additive + body}
	}
	pgOnly := func(body string) map[string]string {
		return map[string]string{"postgres/migrations/0001_a.sql": sqlHeader + additive + body}
	}
	table := func(cols string) string {
		return "CREATE TABLE a (\n  id uuid NOT NULL,\n" + cols + "  CONSTRAINT pk_a PRIMARY KEY (id)\n);\n"
	}
	runCases(t, "ddl-forbidden", []checkCase{
		{name: "clean postgres table", files: pgOnly(table("  name text NOT NULL,\n  created_at timestamptz NOT NULL,\n"))},
		{name: "enum type", files: pgOnly("CREATE TYPE s AS ENUM ('A');\n"), rules: []string{"TY-2"}, contains: "ENUM"},
		{name: "composite type", files: pgOnly("CREATE TYPE s AS (a int);\n"), rules: []string{"TY-2"}, contains: "composite"},
		{name: "domain", files: pgOnly("CREATE DOMAIN d AS text;\n"), rules: []string{"TY-2"}},
		{name: "varchar", files: pgOnly(table("  n varchar(20) NOT NULL,\n")), rules: []string{"TY-2"}, contains: "varchar"},
		{name: "character varying", files: pgOnly(table("  n character varying(20) NOT NULL,\n")), rules: []string{"TY-2"}},
		{name: "char", files: pgOnly(table("  n char(2) NOT NULL,\n")), rules: []string{"TY-2"}},
		{name: "timestamp without time zone", files: pgOnly(table("  n timestamp NOT NULL,\n")), rules: []string{"TY-2"}, contains: "without time zone"},
		{name: "timestamp with time zone spelled out", files: pgOnly(table("  n timestamp with time zone NOT NULL,\n"))},
		{name: "date", files: pgOnly(table("  n date NOT NULL,\n")), rules: []string{"TY-2"}},
		{name: "time", files: pgOnly(table("  n time NOT NULL,\n")), rules: []string{"TY-2"}},
		{name: "interval", files: pgOnly(table("  n interval NOT NULL,\n")), rules: []string{"TY-2"}},
		{name: "money", files: pgOnly(table("  n money NOT NULL,\n")), rules: []string{"TY-2"}},
		{name: "real", files: pgOnly(table("  n real NOT NULL,\n")), rules: []string{"TY-2"}},
		{name: "double precision", files: pgOnly(table("  n double precision NOT NULL,\n")), rules: []string{"TY-2"}},
		{name: "serial", files: pgOnly(table("  n serial NOT NULL,\n")), rules: []string{"TY-2"}},
		{name: "citext", files: pgOnly(table("  n citext NOT NULL,\n")), rules: []string{"TY-2"}},
		{name: "array", files: pgOnly(table("  n text[] NOT NULL,\n")), rules: []string{"TY-2"}, contains: "array"},
		{name: "jsonb without exception", files: pgOnly(table("  n jsonb NOT NULL,\n")), rules: []string{"JS-4"}},
		{name: "jsonb with exception", files: pgOnly(table("  n jsonb NOT NULL,\n")), policy: map[string]*string{fileExceptions: str("JS-4  postgres:a.n  needs containment  2027-01-01  approved-by:owner\n")}},
		{name: "numeric with exception", files: pgOnly(table("  n numeric NOT NULL,\n")), policy: map[string]*string{fileExceptions: str("TY-2  postgres:a.n  money math  2027-01-01  approved-by:owner\n")}},
		{name: "column named date is not a type", files: pgOnly(table("  \"date\" text NOT NULL,\n"))},
		{name: "default on id", files: pgOnly("CREATE TABLE a (\n  id uuid NOT NULL DEFAULT gen_random_uuid(),\n  CONSTRAINT pk_a PRIMARY KEY (id)\n);\n"), rules: []string{"ID-1"}},
		{name: "default on instant", files: pgOnly(table("  created_at timestamptz NOT NULL DEFAULT '2026-01-01',\n")), rules: []string{"TY-5"}},
		{name: "default on another column", files: pgOnly(table("  n text NOT NULL DEFAULT ''\n,"))},
		{name: "uuidv7", files: pgOnly("SELECT uuidv7();\n"), rules: []string{"ID-1"}},
		{name: "current_timestamp", files: pgOnly("SELECT CURRENT_TIMESTAMP;\n"), rules: []string{"TY-5"}},
		{name: "now in a string is fine", files: pgOnly("SELECT 'now()';\n")},
		{name: "autoincrement", files: sqliteOnly("CREATE TABLE a (id INTEGER PRIMARY KEY AUTOINCREMENT) STRICT;\n"), rules: []string{"ID-1", "NM-11"}},
		{name: "view", files: pgOnly("CREATE VIEW v AS SELECT 1;\n"), rules: []string{"TI-10"}},
		{name: "materialized view", files: pgOnly("CREATE MATERIALIZED VIEW v AS SELECT 1;\n"), rules: []string{"TI-10"}},
		{name: "trigger without exception", files: pgOnly("CREATE TRIGGER trg BEFORE INSERT ON a FOR EACH ROW EXECUTE FUNCTION orcon_engine.f();\n"), rules: []string{"TI-10"}, contains: "trg"},
		{name: "trigger with exception", files: pgOnly("CREATE TRIGGER trg BEFORE INSERT ON a FOR EACH ROW EXECUTE FUNCTION orcon_engine.f();\n"), policy: map[string]*string{fileExceptions: str("TI-10  postgres:trg  keeps the search index in step  2027-01-01  approved-by:owner\n")}},
		{name: "sqlite trigger body keeps its semicolons", files: sqliteOnly("CREATE TRIGGER trg AFTER INSERT ON a BEGIN INSERT INTO b VALUES (1); END;\n"), policy: map[string]*string{fileExceptions: str("TI-10  sqlite:trg  keeps the search index in step  2027-01-01  approved-by:owner\n")}},
		{name: "function in public", files: pgOnly("CREATE FUNCTION f() RETURNS int AS $$ SELECT 1 $$ LANGUAGE sql;\n"), rules: []string{"TI-10"}},
		{name: "function in orcon_engine", files: pgOnly("CREATE FUNCTION orcon_engine.claim_tokens() RETURNS int AS $$ SELECT now() $$ LANGUAGE sql;\n")},
		{name: "function in orcon_context", files: pgOnly("CREATE OR REPLACE FUNCTION orcon_context.current_org_ids() RETURNS int AS $$ SELECT 1 $$ LANGUAGE sql;\n")},
		{name: "sqlite not strict", files: sqliteOnly("CREATE TABLE a (id BLOB NOT NULL, CONSTRAINT pk_a PRIMARY KEY (id));\n"), rules: []string{"TY-3"}},
		{name: "sqlite strict", files: sqliteOnly("CREATE TABLE a (id BLOB NOT NULL, CONSTRAINT pk_a PRIMARY KEY (id)) STRICT;\n")},
		{name: "sqlite without rowid", files: sqliteOnly("CREATE TABLE a (id BLOB NOT NULL, CONSTRAINT pk_a PRIMARY KEY (id)) STRICT, WITHOUT ROWID;\n"), rules: []string{"TY-3"}, contains: "WITHOUT ROWID"},
		{name: "sqlite without rowid with exception", files: sqliteOnly("CREATE TABLE a (id BLOB NOT NULL, CONSTRAINT pk_a PRIMARY KEY (id)) STRICT, WITHOUT ROWID;\n"), policy: map[string]*string{fileExceptions: str("TY-3  sqlite:a  measured  2027-01-01  approved-by:owner\n")}},
		{name: "sqlite virtual table is not checked for strict", files: sqliteOnly("CREATE VIRTUAL TABLE a_fts5 USING fts5(body);\n")},
		{name: "sqlite real", files: sqliteOnly("CREATE TABLE a (id BLOB NOT NULL, n REAL NOT NULL, CONSTRAINT pk_a PRIMARY KEY (id)) STRICT;\n"), rules: []string{"TY-2"}},
		{name: "sqlite any", files: sqliteOnly("CREATE TABLE a (id BLOB NOT NULL, n ANY, CONSTRAINT pk_a PRIMARY KEY (id)) STRICT;\n"), rules: []string{"TY-2"}},
		{name: "unnamed table primary key", files: pgOnly("CREATE TABLE a (id uuid NOT NULL, PRIMARY KEY (id));\n"), rules: []string{"NM-11"}},
		{name: "unnamed table unique", files: pgOnly("CREATE TABLE a (id uuid NOT NULL, b int, CONSTRAINT pk_a PRIMARY KEY (id), UNIQUE (b));\n"), rules: []string{"NM-11"}},
		{name: "unnamed table check", files: pgOnly("CREATE TABLE a (id uuid NOT NULL, b int, CONSTRAINT pk_a PRIMARY KEY (id), CHECK (b > 0));\n"), rules: []string{"NM-11"}},
		{name: "unnamed table foreign key", files: pgOnly("CREATE TABLE a (id uuid NOT NULL, b uuid, CONSTRAINT pk_a PRIMARY KEY (id), FOREIGN KEY (b) REFERENCES c (id));\n"), rules: []string{"NM-11"}},
		{name: "unnamed column primary key", files: pgOnly("CREATE TABLE a (id uuid PRIMARY KEY);\n"), rules: []string{"NM-11"}},
		{name: "unnamed column references", files: pgOnly("CREATE TABLE a (id uuid NOT NULL, b uuid REFERENCES c (id), CONSTRAINT pk_a PRIMARY KEY (id));\n"), rules: []string{"NM-11"}},
		{name: "named column check", files: pgOnly("CREATE TABLE a (id uuid NOT NULL, b int CONSTRAINT ck_a_b CHECK (b > 0), CONSTRAINT pk_a PRIMARY KEY (id));\n")},
		{name: "unnamed index", files: pgOnly("CREATE INDEX ON a (b);\n"), rules: []string{"NM-11"}},
		{name: "named concurrent index", files: pgOnly("CREATE INDEX CONCURRENTLY IF NOT EXISTS ix_a_b ON a (b);\n")},
		{name: "alter add unnamed unique", files: pgOnly("ALTER TABLE a ADD UNIQUE (b);\n"), rules: []string{"NM-11"}},
		{name: "alter add named unique", files: pgOnly("ALTER TABLE a ADD CONSTRAINT uq_a_b UNIQUE (b);\n")},
		{name: "alter add column varchar", files: pgOnly("ALTER TABLE a ADD COLUMN b varchar(3);\n"), rules: []string{"TY-2"}},
		{name: "alter add column fine", files: pgOnly("ALTER TABLE IF EXISTS ONLY a ADD COLUMN b text;\n")},
	})
}

func TestQueryChecks(t *testing.T) {
	q := func(dialect, body string) map[string]string {
		return map[string]string{dialect + "/queries/q.sql": body}
	}
	both := func(body string) map[string]string { return merge(q("sqlite", body), q("postgres", body)) }
	scope := "-- orcon:scope org\n"
	t.Run("select-star", func(t *testing.T) {
		runCases(t, "query-select-star", []checkCase{
			{name: "explicit columns", files: both("-- name: GetA :one\n" + scope + "SELECT a, b FROM t WHERE id = $1;\n")},
			{name: "count star", files: both("-- name: CountA :one\n" + scope + "SELECT count(*) FROM t;\n")},
			{name: "multiplication", files: both("-- name: GetA :one\n" + scope + "SELECT a * b AS c FROM t;\n")},
			{name: "star", files: both("-- name: GetA :one\n" + scope + "SELECT * FROM t;\n"), rules: []string{"QY-2"}},
			{name: "qualified star", files: both("-- name: GetA :one\n" + scope + "SELECT t.* FROM t;\n"), rules: []string{"QY-2"}},
			{name: "star after distinct", files: both("-- name: GetA :one\n" + scope + "SELECT DISTINCT * FROM t;\n"), rules: []string{"QY-2"}},
			{name: "star in second item", files: both("-- name: GetA :one\n" + scope + "SELECT a, * FROM t;\n"), rules: []string{"QY-2"}},
			{name: "star in subquery", files: both("-- name: GetA :one\n" + scope + "SELECT a FROM (SELECT * FROM t) AS s;\n"), rules: []string{"QY-2"}},
			{name: "returning star", files: both("-- name: CreateA :one\n" + scope + "INSERT INTO t (a) VALUES ($1) RETURNING *;\n"), rules: []string{"QY-2"}},
			{name: "returning columns", files: both("-- name: CreateA :one\n" + scope + "INSERT INTO t (a) VALUES ($1) RETURNING id;\n")},
			{name: "star in comment and string", files: both("-- name: GetA :one\n" + scope + "-- SELECT * FROM t\nSELECT 'SELECT * FROM t' AS s FROM t;\n")},
			{name: "exists subquery star is a finding", files: both("-- name: GetA :one\n" + scope + "SELECT a FROM t WHERE EXISTS (SELECT * FROM u);\n"), rules: []string{"QY-2"}},
		})
	})
	t.Run("json-path", func(t *testing.T) {
		runCases(t, "query-json-path", []checkCase{
			{name: "no json", files: both("-- name: GetA :one\n" + scope + "SELECT a FROM t WHERE b = $1;\n")},
			{name: "postgres arrow text", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT a FROM t WHERE d ->> 'k' = $1;\n"), rules: []string{"JS-5"}},
			{name: "postgres arrow", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT d -> 'k' FROM t;\n"), rules: []string{"JS-5"}},
			{name: "postgres path", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT d #> '{a}' FROM t;\n"), rules: []string{"JS-5"}},
			{name: "postgres path text", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT d #>> '{a}' FROM t;\n"), rules: []string{"JS-5"}},
			{name: "postgres delete path", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT d #- '{a}' FROM t;\n"), rules: []string{"JS-5"}},
			{name: "postgres jsonpath exists", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT a FROM t WHERE d @? '$.a';\n"), rules: []string{"JS-5"}},
			{name: "postgres key exists", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT a FROM t WHERE d ? 'k';\n"), rules: []string{"JS-5"}},
			{name: "postgres any key exists", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT a FROM t WHERE d ?| array['k'];\n"), rules: []string{"JS-5"}},
			{name: "postgres all keys exist", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT a FROM t WHERE d ?& array['k'];\n"), rules: []string{"JS-5"}},
			{name: "postgres jsonb function", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT jsonb_array_length(d) FROM t;\n"), rules: []string{"JS-5"}},
			{name: "postgres json function", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT json_build_object('a', 1);\n"), rules: []string{"JS-5"}},
			{name: "postgres to_jsonb", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT to_jsonb(a) FROM t;\n"), rules: []string{"JS-5"}},
			{name: "postgres containment is a known gap", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT a FROM t WHERE d @> $1;\n")},
			{name: "postgres question mark inside a string", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT 'is it?' FROM t;\n")},
			{name: "sqlite question mark is a parameter", files: q("sqlite", "-- name: GetA :one\n"+scope+"SELECT a FROM t WHERE b = ? AND c = ?;\n")},
			{name: "sqlite json_extract", files: q("sqlite", "-- name: GetA :one\n"+scope+"SELECT json_extract(d, '$.a') FROM t;\n"), rules: []string{"JS-5"}},
			{name: "sqlite json_each", files: q("sqlite", "-- name: GetA :many\n"+scope+"SELECT value FROM t, json_each(d) LIMIT 1;\n"), rules: []string{"JS-5"}},
			{name: "sqlite json_tree", files: q("sqlite", "-- name: GetA :many\n"+scope+"SELECT value FROM t, json_tree(d) LIMIT 1;\n"), rules: []string{"JS-5"}},
			{name: "sqlite arrow", files: q("sqlite", "-- name: GetA :one\n"+scope+"SELECT d -> '$.a' FROM t;\n"), rules: []string{"JS-5"}},
			{name: "sqlite arrow text", files: q("sqlite", "-- name: GetA :one\n"+scope+"SELECT d ->> '$.a' FROM t;\n"), rules: []string{"JS-5"}},
			{name: "sqlite json function", files: q("sqlite", "-- name: GetA :one\n"+scope+"SELECT json(d) FROM t;\n"), rules: []string{"JS-5"}},
			{name: "sqlite jsonb function", files: q("sqlite", "-- name: GetA :one\n"+scope+"SELECT jsonb_extract(d, '$.a') FROM t;\n"), rules: []string{"JS-5"}},
			{name: "approved exception", files: q("postgres", "-- name: GetA :one\n"+scope+"-- orcon:exception JS-5\nSELECT d ->> 'k' FROM t;\n"), policy: map[string]*string{fileExceptions: str("JS-5  postgres:GetA  one-off lookup  2027-01-01  approved-by:owner\n")}},
			{name: "exception for another query", files: q("postgres", "-- name: GetA :one\n"+scope+"-- orcon:exception JS-5\nSELECT d ->> 'k' FROM t;\n"), policy: map[string]*string{fileExceptions: str("JS-5  postgres:GetB  one-off lookup  2027-01-01  approved-by:owner\n")}, rules: []string{"JS-5"}},
			{name: "exception without the annotation", files: q("postgres", "-- name: GetA :one\n"+scope+"SELECT d ->> 'k' FROM t;\n"), policy: map[string]*string{fileExceptions: str("JS-5  postgres:GetA  one-off lookup  2027-01-01  approved-by:owner\n")}, rules: []string{"JS-5"}},
		})
	})
	t.Run("limit", func(t *testing.T) {
		runCases(t, "query-limit", []checkCase{
			{name: "many with limit", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM t ORDER BY id LIMIT $1;\n")},
			{name: "many without limit", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM t ORDER BY id;\n"), rules: []string{"QY-6"}},
			{name: "limit only in a subquery", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM (SELECT a FROM t LIMIT 5) AS s;\n"), rules: []string{"QY-6"}},
			{name: "limit in a CTE only", files: both("-- name: ListA :many\n" + scope + "WITH x AS (SELECT a FROM t LIMIT 5) SELECT a FROM x;\n"), rules: []string{"QY-6"}},
			{name: "batchmany without limit", files: both("-- name: ListA :batchmany\n" + scope + "SELECT a FROM t;\n"), rules: []string{"QY-6"}},
			{name: "one without limit", files: both("-- name: GetA :one\n" + scope + "SELECT a FROM t WHERE id = $1;\n")},
			{name: "update without where", files: both("-- name: UpdateA :exec\n" + scope + "UPDATE t SET a = $1;\n"), rules: []string{"QY-6"}},
			{name: "update with where", files: both("-- name: UpdateA :exec\n" + scope + "UPDATE t SET a = $1 WHERE id = $2;\n")},
			{name: "update where only in a subquery", files: both("-- name: UpdateA :exec\n" + scope + "UPDATE t SET a = (SELECT b FROM u WHERE id = $1);\n"), rules: []string{"QY-6"}},
			{name: "delete without where", files: both("-- name: DeleteA :exec\n" + scope + "-- orcon:delete bookkeeping:test\nDELETE FROM t;\n"), rules: []string{"QY-6"}},
			{name: "delete with bounded subquery", files: both("-- name: DeleteA :exec\n" + scope + "-- orcon:delete bookkeeping:test\nDELETE FROM t WHERE id IN (SELECT id FROM t LIMIT $1);\n")},
			{name: "upsert update clause is not an update statement", files: both("-- name: UpsertA :exec\n" + scope + "INSERT INTO t (id) VALUES ($1) ON CONFLICT (id) DO UPDATE SET a = 1;\n")},
		})
	})
	t.Run("offset", func(t *testing.T) {
		runCases(t, "query-offset", []checkCase{
			{name: "keyset", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM t WHERE id < $1 ORDER BY id DESC LIMIT $2;\n")},
			{name: "offset", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM t ORDER BY id LIMIT $1 OFFSET $2;\n"), rules: []string{"QY-5"}},
			{name: "offset in a subquery", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM (SELECT a FROM t LIMIT 1 OFFSET 1) AS s LIMIT 1;\n"), rules: []string{"QY-5"}},
			{name: "approved exception", files: q("postgres", "-- name: ListA :many\n"+scope+"-- orcon:exception QY-5\nSELECT a FROM t ORDER BY id LIMIT $1 OFFSET $2;\n"), policy: map[string]*string{fileExceptions: str("QY-5  postgres:ListA  admin export  2027-01-01  approved-by:owner\n")}},
		})
	})
	t.Run("keyset", func(t *testing.T) {
		runCases(t, "query-keyset", []checkCase{
			{name: "ends with id", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM t ORDER BY created_at DESC, id DESC LIMIT $1;\n")},
			{name: "alias id", files: both("-- name: ListA :many\n" + scope + "SELECT t.a FROM t ORDER BY t.created_at, t.id LIMIT $1;\n")},
			{name: "sequence number", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM t ORDER BY sequence_number LIMIT $1;\n")},
			{name: "nulls last", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM t ORDER BY created_at, id ASC NULLS LAST LIMIT $1;\n")},
			{name: "ends with a sort column", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM t ORDER BY id, created_at LIMIT $1;\n"), rules: []string{"QY-5"}},
			{name: "only a sort column", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM t ORDER BY created_at LIMIT $1;\n"), rules: []string{"QY-5"}},
			{name: "no order by", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM t LIMIT $1;\n")},
			{name: "order by inside a subquery only", files: both("-- name: ListA :many\n" + scope + "SELECT a FROM (SELECT a FROM t ORDER BY a LIMIT 3) AS s LIMIT $1;\n")},
			{name: "one query is not a list", files: both("-- name: GetA :one\n" + scope + "SELECT a FROM t ORDER BY created_at LIMIT 1;\n")},
		})
	})
	t.Run("annotations", func(t *testing.T) {
		runCases(t, "query-annotations", []checkCase{
			{name: "scope org", files: both("-- name: GetA :one\n" + scope + "SELECT a FROM t;\n")},
			{name: "scope instance", files: both("-- name: GetA :one\n-- orcon:scope instance\nSELECT a FROM t;\n")},
			{name: "scope engine", files: both("-- name: GetA :one\n-- orcon:scope engine\nSELECT a FROM t;\n")},
			{name: "scope missing", files: both("-- name: GetA :one\nSELECT a FROM t;\n"), rules: []string{"TI-2"}},
			{name: "scope invalid", files: both("-- name: GetA :one\n-- orcon:scope everyone\nSELECT a FROM t;\n"), rules: []string{"TI-2"}},
			{name: "scope repeated", files: both("-- name: GetA :one\n" + scope + scope + "SELECT a FROM t;\n"), rules: []string{"QY-9"}},
			{name: "scope after the statement belongs to nobody", files: both("-- name: GetA :one\nSELECT a FROM t;\n" + scope), rules: []string{"TI-2"}},
			{name: "delete without annotation", files: both("-- name: DeleteA :exec\n" + scope + "DELETE FROM t WHERE id = $1;\n"), rules: []string{"DL-2"}},
			{name: "delete audit", files: both("-- name: DeleteA :exec\n" + scope + "-- orcon:delete audit:a.deleted\nDELETE FROM t WHERE id = $1;\n")},
			{name: "delete retention", files: both("-- name: DeleteA :exec\n" + scope + "-- orcon:delete retention:executions\nDELETE FROM t WHERE id = $1;\n")},
			{name: "delete bookkeeping", files: both("-- name: DeleteA :exec\n" + scope + "-- orcon:delete bookkeeping:expired lease\nDELETE FROM t WHERE id = $1;\n")},
			{name: "delete malformed", files: both("-- name: DeleteA :exec\n" + scope + "-- orcon:delete whenever\nDELETE FROM t WHERE id = $1;\n"), rules: []string{"DL-2"}},
			{name: "delete in a CTE statement", files: both("-- name: DeleteA :exec\n" + scope + "WITH x AS (SELECT id FROM t LIMIT 1) DELETE FROM t WHERE id IN (SELECT id FROM x);\n"), rules: []string{"DL-2"}},
			{name: "hot with argument", files: both("-- name: GetA :one\n" + scope + "-- orcon:hot yes\nSELECT a FROM t;\n"), rules: []string{"QY-9"}},
			{name: "include-deleted without reason", files: both("-- name: GetA :one\n" + scope + "-- orcon:include-deleted\nSELECT a FROM t;\n"), rules: []string{"DL-4"}},
			{name: "unknown annotation", files: both("-- name: GetA :one\n" + scope + "-- orcon:wish upon\nSELECT a FROM t;\n"), rules: []string{"QY-9"}},
			{name: "exception without a line", files: both("-- name: GetA :one\n" + scope + "-- orcon:exception JS-5\nSELECT a FROM t;\n"), rules: []string{"QY-9"}, contains: "no owner-approved line"},
			{name: "exception rule malformed", files: both("-- name: GetA :one\n" + scope + "-- orcon:exception later\nSELECT a FROM t;\n"), rules: []string{"QY-9"}},
			{name: "unknown command", files: both("-- name: GetA :lots\n" + scope + "SELECT a FROM t;\n"), rules: []string{"QY-9"}},
			{name: "malformed name line", files: both("-- name: GetA\n" + scope + "SELECT a FROM t;\n"), rules: []string{"QY-9"}},
			{name: "statement before a name", files: both("SELECT 1;\n-- name: GetA :one\n" + scope + "SELECT a FROM t;\n"), rules: []string{"QY-9"}},
			{name: "two statements in one query", files: both("-- name: GetA :one\n" + scope + "SELECT a FROM t;\nSELECT b FROM t;\n"), rules: []string{"QY-9"}},
		})
	})
	t.Run("soft-delete", func(t *testing.T) {
		runCases(t, "query-soft-delete", []checkCase{
			{name: "filtered", files: both("-- name: ListFlows :many\n" + scope + "SELECT id FROM flow WHERE deleted_at IS NULL ORDER BY id LIMIT $1;\n")},
			{name: "filtered with alias", files: both("-- name: ListFlows :many\n" + scope + "SELECT f.id FROM flow f WHERE f.deleted_at IS NULL ORDER BY f.id LIMIT $1;\n")},
			{name: "unfiltered", files: both("-- name: ListFlows :many\n" + scope + "SELECT id FROM flow ORDER BY id LIMIT $1;\n"), rules: []string{"DL-4"}},
			{name: "is not null is not a filter", files: both("-- name: ListFlows :many\n" + scope + "SELECT id FROM flow WHERE deleted_at IS NOT NULL ORDER BY id LIMIT $1;\n"), rules: []string{"DL-4"}},
			{name: "joined", files: both("-- name: ListFlows :many\n" + scope + "SELECT e.id FROM execution e JOIN flow f ON f.id = e.flow_id ORDER BY e.id LIMIT $1;\n"), rules: []string{"DL-4"}},
			{name: "annotated", files: both("-- name: PurgeFlows :exec\n" + scope + "-- orcon:include-deleted purge after the recovery window\n-- orcon:delete retention:flow_purge\nDELETE FROM flow WHERE id = $1;\n")},
			{name: "insert is not a read", files: both("-- name: CreateFlow :exec\n" + scope + "INSERT INTO flow (id) VALUES ($1);\n")},
			{name: "another table", files: both("-- name: ListFlowVersions :many\n" + scope + "SELECT id FROM flow_version ORDER BY id LIMIT $1;\n")},
			{name: "a column named like the table", files: both("-- name: ListA :many\n" + scope + "SELECT flow FROM t ORDER BY id LIMIT $1;\n")},
		})
	})
	t.Run("hot-tag", func(t *testing.T) {
		runCases(t, "query-hot-tag", []checkCase{
			{name: "tagged", files: both("-- name: ClaimTokens :many\n" + scope + "-- orcon:hot\nSELECT id FROM t ORDER BY id LIMIT $1;\n")},
			{name: "untagged claim", files: both("-- name: ClaimTokens :many\n" + scope + "SELECT id FROM t ORDER BY id LIMIT $1;\n"), rules: []string{"QY-4"}},
			{name: "untagged execution list", files: both("-- name: ListExecutions :many\n" + scope + "SELECT id FROM t ORDER BY id LIMIT $1;\n"), rules: []string{"QY-4"}},
			{name: "untagged journal append", files: both("-- name: AppendJournalRecord :exec\n" + scope + "INSERT INTO t (id) VALUES ($1);\n"), rules: []string{"QY-4"}},
			{name: "untagged external operation", files: both("-- name: GetExternalOperation :one\n" + scope + "SELECT id FROM t WHERE id = $1;\n"), rules: []string{"QY-4"}},
			{name: "not a hot name", files: both("-- name: GetOrg :one\n" + scope + "SELECT id FROM t WHERE id = $1;\n")},
		})
	})
	t.Run("parity", func(t *testing.T) {
		one := "-- name: GetA :one\n" + scope + "SELECT a FROM t;\n"
		two := "-- name: GetB :one\n" + scope + "SELECT b FROM t;\n"
		runCases(t, "query-parity", []checkCase{
			{name: "same names", files: both(one)},
			{name: "only in sqlite", files: merge(q("sqlite", one+two), q("postgres", one)), rules: []string{"QY-1"}, contains: "exists in sqlite but not in postgres"},
			{name: "only in postgres", files: merge(q("sqlite", one), q("postgres", one+two)), rules: []string{"QY-1"}, contains: "exists in postgres but not in sqlite"},
			{name: "declared dialect only", files: merge(q("sqlite", one+two), q("postgres", one)), policy: map[string]*string{fileDialectOnly: str("sqlite:GetB  # FTS5 lookup\n")}},
			{name: "stale dialect only line", files: both(one), policy: map[string]*string{fileDialectOnly: str("sqlite:GetB  # gone\n")}, rules: []string{"QY-1"}, contains: "does not exist"},
			{name: "dialect only line for a shared query", files: both(one + two), policy: map[string]*string{fileDialectOnly: str("sqlite:GetB  # wrong\n")}, rules: []string{"QY-1"}, contains: "exists in both"},
			{name: "duplicate name", files: merge(q("sqlite", one+one), q("postgres", one)), rules: []string{"QY-1"}, contains: "defined twice"},
		})
	})
	t.Run("names", func(t *testing.T) {
		name := func(n string) map[string]string {
			return both("-- name: " + n + " :one\n" + scope + "SELECT a FROM t;\n")
		}
		runCases(t, "query-names", []checkCase{
			{name: "verb and object", files: name("GetFlow")},
			{name: "acronym words", files: name("GetAPIKeyByHash")},
			{name: "not a verb", files: name("FetchFlow"), rules: []string{"NM-14"}},
			{name: "verb as a prefix of a word", files: name("Getter"), rules: []string{"NM-14"}},
			{name: "bare verb", files: name("List")},
			{name: "abbreviation", files: name("GetFlowRef"), rules: []string{"NM-3"}, contains: "spell it out: reference"},
			{name: "two abbreviations", files: name("ListMaxSeq"), rules: []string{"NM-3"}},
			{name: "unlisted unusual word is accepted", files: name("GetWidget")},
			{name: "verb list comes from the policy file", files: name("FetchFlow"), policy: map[string]*string{fileQueryVerbs: str("Fetch  # read one\n")}},
			{name: "removing an allowed acronym rejects the word", files: name("GetAPIKey"), policy: map[string]*string{fileDenylist: str("api  # application programming interface\n"), fileAllowedAcronyms: str("id  # identifier\n")}, rules: []string{"NM-3"}},
		})
	})
}

func TestPolicyCheckReportsExceptionProblems(t *testing.T) {
	runCases(t, "policy", []checkCase{
		{name: "repository policy", files: nil},
		{name: "exception without approver", files: nil, policy: map[string]*string{fileExceptions: str("JS-5  postgres:GetA  why  2027-01-01\n")}, rules: []string{"CK-20"}},
		{name: "expired exception", files: nil, policy: map[string]*string{fileExceptions: str("JS-5  postgres:GetA  why  2026-01-01  approved-by:owner\n")}, rules: []string{"CK-21"}},
		{name: "token in both naming lists", files: nil, policy: map[string]*string{fileAllowedAcronyms: str("ref  # reference\n")}, rules: []string{"NM-3"}},
	})
}

func TestWorldLoadProblemsAreFindings(t *testing.T) {
	runCases(t, "migration-names", []checkCase{
		{name: "unreadable SQL", files: pair(1, "a", additive, "SELECT 'unterminated;\n"), rules: []string{"MG-2"}, contains: "SQL cannot be read"},
	})
	cfg := tree(t, pair(1, "a", additive, ""), nil)
	cfg.SQLiteDir += "-missing"
	findings, _, code := runCheck(t, "policy", cfg)
	if code != 1 || !contains(findings, "migrations directory cannot be read") {
		t.Fatalf("a missing directory was not reported: %v", findings)
	}
}

func TestUnknownCheckAndUsage(t *testing.T) {
	var out, errOut strings.Builder
	if code := run([]string{"no-such-check"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "unknown check") {
		t.Fatalf("code %d, stderr %q", code, errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := run(nil, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "usage: dbcheck") {
		t.Fatalf("code %d, stderr %q", code, errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := run([]string{"-h"}, &out, &errOut); code != 0 {
		t.Fatalf("-h: code %d", code)
	}
	if !strings.Contains(errOut.String(), "migration-immutable") {
		t.Fatalf("usage does not list the checks: %q", errOut.String())
	}
}

func TestCheckEnumValues(t *testing.T) {
	check := func(expr string) map[string]string {
		return map[string]string{"sqlite/migrations/0001_a.sql": sqlHeader + additive + "CREATE TABLE a (\n  id BLOB NOT NULL,\n  status TEXT NOT NULL,\n  CONSTRAINT ck_a_status CHECK (" + expr + ")\n) STRICT;\n"}
	}
	pgCheck := func(expr string) map[string]string {
		return map[string]string{"postgres/migrations/0001_a.sql": sqlHeader + additive + "CREATE TABLE a (\n  id uuid NOT NULL,\n  status text NOT NULL,\n  CONSTRAINT ck_a_status CHECK (" + expr + ")\n);\n"}
	}
	runCases(t, "ddl-enum-values", []checkCase{
		{name: "IN with a lowercase value", files: check("status IN ('ACTIVE', 'revoked')"), rules: []string{"NM-16"}, contains: `value "revoked" in CHECK ck_a_status`},
		{name: "IN all UPPER_SNAKE", files: check("status IN ('ACTIVE', 'NOT_FOUND', 'OAUTH2')")},
		{name: "NOT IN with a lowercase value", files: check("status NOT IN ('a')"), rules: []string{"NM-16"}},
		{name: "ANY ARRAY with a cast", files: pgCheck("status = ANY (ARRAY['x'::text])"), rules: []string{"NM-16"}},
		{name: "ANY ARRAY with several casts, correct", files: pgCheck("status = ANY (ARRAY['A'::text, 'B'::text])")},
		{name: "ANY ARRAY cast to text array", files: pgCheck("((status)::text = ANY ((ARRAY['A'::character varying, 'b'::character varying])::text[]))"), rules: []string{"NM-16"}},
		{name: "ALL ARRAY with <>", files: pgCheck("status <> ALL (ARRAY['x'])"), rules: []string{"NM-16"}},
		{name: "mixed case comparison", files: check("status = 'Active'"), rules: []string{"NM-16"}},
		{name: "comparison with the literal first", files: check("'Active' = status"), rules: []string{"NM-16"}},
		{name: "not equal comparison", files: check("status != 'draft'"), rules: []string{"NM-16"}},
		{name: "comparison on a column that is not enumerated", files: check("name <> ''")},
		{name: "empty value in IN", files: check("status IN ('')"), rules: []string{"NM-16"}},
		{name: "format column is exempt", files: check("body_document_format IN ('orcon/flow@1')")},
		{name: "enumerated column with digits", files: check("kind IN ('OAUTH2')")},
		{name: "escaped quote keeps the whole value", files: check("status IN ('it''s')"), rules: []string{"NM-16"}, contains: `value "it's"`},
		{name: "check inside a comment", files: check("status IN ('ACTIVE') -- status IN ('x')\n")},
		{name: "numeric list", files: check("is_active IN (0, 1)")},
		{name: "regular expression is not a value", files: pgCheck("status ~ '^[a-z]+$'")},
		{name: "approved exception", files: check("status IN ('client_secret_basic')"), policy: map[string]*string{fileExceptions: str("NM-16  sqlite:a.ck_a_status  fixed by an external standard  2027-01-01  approved-by:owner\n")}},
		{name: "exception for another dialect does not apply", files: check("status IN ('x')"), policy: map[string]*string{fileExceptions: str("NM-16  postgres:a.ck_a_status  other dialect  2027-01-01  approved-by:owner\n")}, rules: []string{"NM-16"}},
		{name: "list on a column of any name", files: check("name IN ('x')"), rules: []string{"NM-16"}},
		{name: "enumeration words missing", files: check("status IN ('ACTIVE')"), policy: map[string]*string{fileEnumWords: nil}, rules: []string{"CK-24"}, contains: "enumeration words are missing"},
	})
}

func TestEnumFixSuggestion(t *testing.T) {
	tests := map[string]string{"notification-link": "NOTIFICATION_LINK", "revoked": "REVOKED", "Revoked": "REVOKED", "it's": "IT_S", "a  b": "A_B", "": ""}
	for in, want := range tests {
		if got := upperSnakeFix(in); got != want {
			t.Errorf("upperSnakeFix(%q) = %q, want %q", in, got, want)
		}
	}
	runCases(t, "ddl-enum-values", []checkCase{{
		name:     "fix text",
		files:    map[string]string{"sqlite/migrations/0001_a.sql": sqlHeader + additive + "CREATE TABLE a (id BLOB NOT NULL, status TEXT NOT NULL, CONSTRAINT ck_a_status CHECK (status IN ('notification-link'))) STRICT;\n"},
		rules:    []string{"NM-16"},
		contains: "(fix: 'NOTIFICATION_LINK')",
	}})
}
