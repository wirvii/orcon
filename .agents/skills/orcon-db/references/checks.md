<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Checks

Status: Accepted

This document lists every automated check that enforces the database rulebook: the tool, its license, where it runs, what it verifies, how to fix a finding and how to ask for an exception. Rule IDs (`CK-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

No external linter knows these rules for both dialects at once, so Orcon has two small checkers of its own: the schema is checked by **introspection** of the real catalog after the migrations are applied on both engines, and migration and query files are checked by **lexical analysis** (an SQL lexer that respects comments, strings, quoted identifiers and the `$n` and `?` parameter markers). Both checkers read the lists in `policy/`; no list is duplicated in code. A check that cannot load a list it needs fails with a message; it never passes silently.

| Tool | Use | License | Pinning |
|---|---|---|---|
| `tools/dbcheck` | static checks | Apache-2.0 (in-repo; standard library only, no cgo) | `tools/` module |
| `internal/adapter/storetest/dbrules` | introspection, connection settings, plans, upgrades | Apache-2.0 (in-repo) | root module |
| modernc.org/sqlite | SQLite engine of the tests | BSD-3-Clause (ADR-0076) | `go.mod` |
| pgx/v5 | PostgreSQL client of the tests | MIT (ADR-0076) | `go.mod` |
| testcontainers-go | ephemeral PostgreSQL | MIT (ADR-0076) | `go.mod` |
| Official `postgres:14` and `postgres:17` images | test engines (oldest supported and latest) | PostgreSQL License (tests only, not distributed) | pinned by digest in CI |
| `auto_explain` (contrib module shipped in the image) | plans of statements inside `orcon_engine` functions | PostgreSQL License | — |
| sqlc | compiles queries | MIT (ADR-0076) | `tools/versions.mk` |

## Static checks

- **CK-1** `tools/dbcheck` runs in `make check` through `make db-check` (`.bin/dbcheck all`, built by `make tools`). It needs no database. Output is one line per finding: `<file>:<line>: <rule> <message> (fix: …)`. `DBCHECK_BASE` names the base revision for the immutability check (default: the merge base with `origin/main`; CI passes the pull request base).

| Check | Rules | Rejects | Fix |
|---|---|---|---|
| `migration-names` | NM-3, NM-15, MG-1 | a name not matching `^\d{4}_[a-z][a-z0-9]*(_[a-z0-9]+)*\.sql$`; numbers not contiguous from `0001`; duplicates; a description token NM-3 rejects | rename the new file; spell the word out |
| `migration-pairs` | MG-1, MG-2, MG-11 | a number without its pair in the other dialect; the same number with a different name; different headers in a pair; a missing `NNNN_seed.sql` or `NNNN_verify.sql` in either `testdata/` | add the missing file; align the pair |
| `migration-headers` | MG-2 | missing `-- orcon:kind` or `-- orcon:compatibility-floor`; `contract` without `-- orcon:contracts NNNN`; `no-transaction` in SQLite or with more than one statement | complete the header |
| `migration-immutable` | MG-5 | a file whose SHA-256 differs from its line in `migrations.sum`; a removed line; a file without a line; a line present in the base revision that was changed or removed. Without a base revision it says so and compares only against `migrations.sum` | revert the edit and write a new migration; run `make db-sum` for new files |
| `migration-expand-contract` | MG-6, MG-7 | outside `kind contract`: `DROP TABLE`, `DROP COLUMN`, `RENAME`, `ALTER COLUMN … TYPE`, `SET NOT NULL`, `ADD COLUMN … NOT NULL` without `DEFAULT`; a `contract` whose `contracts NNNN` is not in an earlier release tag (without release tags it says so and skips that condition) | split the change into `expand` and a later `contract` |
| `ddl-forbidden` | TY-2, TY-3, ID-1, TY-5, TI-10 | `CREATE TYPE … AS ENUM`, `DOMAIN`, forbidden types (`varchar(`, `char(`, `timestamp` without time zone, `date`, `time`, `interval`, `money`, `real`, `double precision`, `serial`, arrays), `DEFAULT` on `id` or `*_at`, `gen_random_uuid`, `uuidv7`, `now()`, `CURRENT_TIMESTAMP`, `AUTOINCREMENT`, `CREATE VIEW`, `CREATE TRIGGER` or `CREATE FUNCTION` outside what is allowed, an SQLite `CREATE TABLE` without `STRICT`, `WITHOUT ROWID` without an exception, a constraint without `CONSTRAINT <name>` | use the mapped type (TY-1); generate ids and times in Go; name the constraint |
| `ddl-enum-values` | NM-16, IN-3 | a string literal in a CHECK that is not UPPER_SNAKE, as CK-24 defines (an `IN`, `NOT IN`, `= ANY (ARRAY[…])` or `<> ALL (ARRAY[…])` list on any column; `=`, `<>` or `!=` against an enumerated column), except `format` columns and listed exceptions | write the value in UPPER_SNAKE |
| `query-select-star` | QY-2 | `*` in a SELECT list or in `RETURNING` (`count(*)` is allowed) | list the columns |
| `query-json-path` | JS-5 | PostgreSQL `->`, `->>`, `#>`, `#>>`, `#-`, `@?`, `?`, `?\|`, `?&`, `json_*` and `jsonb_*` functions; SQLite `->`, `->>`, `json_*` and `jsonb_*` functions (including `json_extract`, `json_each`, `json_tree`); unless an exception exists | move the value into a real column (JS-2) |
| `query-limit` | QY-6 | `:many` or `:batchmany` without a top-level `LIMIT`; `UPDATE` or `DELETE` without `WHERE` | add the bound |
| `query-offset` | QY-5 | any `OFFSET` | use keyset pagination |
| `query-keyset` | QY-5 | `:many` with an `ORDER BY` whose last term is not `id`, `<alias>.id` or `sequence_number` | end the ordering with a unique key |
| `query-annotations` | QY-9, DL-2, TI-2 | missing `-- orcon:scope org\|instance\|engine`; a `DELETE` without `-- orcon:delete audit:<action>\|retention:<class>\|bookkeeping:<reason>` | add the annotation |
| `query-soft-delete` | DL-4 | a query naming a table of `soft_delete_tables.txt` without `deleted_at IS NULL` or `-- orcon:include-deleted <reason>` | add the filter or the annotation |
| `query-hot-tag` | QY-4 | a query name matching a pattern of `hot_query_patterns.txt` without `-- orcon:hot` | add the tag |
| `query-parity` | QY-1 | a query name present in one dialect and not in the other, unless listed in `dialect_only_queries.txt` | add the query to the other dialect |
| `query-names` | NM-3, NM-14 | a name that does not start with a listed verb; a word of the name (split as NM-3 states) that NM-3 rejects, for example `GetFlowRef` | rename (`GetFlowReference`) |
| `policy` | NM-3 | a malformed policy or exception file; an exception without `approved-by:` or past its date; in `allowed_acronyms.txt` and `abbreviation_denylist.txt`: a token not matching `^[a-z][a-z0-9]*$`, a line without `# <expansion>`, a duplicate, a list out of alphabetical order, or a token present in both lists | correct the file |

- **CK-2** Known limit: the lexer does not know types, so `@>` and `<@` (also legitimate on arrays and ranges) are not flagged as JSON-path operators; review and JS-5 cover them. Arrays are forbidden (TY-2) and `jsonb` needs an exception (JS-4), so in practice this leaves no gap.
- **CK-3** Planted violations: every check has at least one case under `tools/dbcheck/testdata/bad/<case>/`; `scripts/check-db-fixtures.sh` exits 0 only if every bad case is detected and every good case passes.
- **CK-24** Enumeration values in CHECK constraints are UPPER_SNAKE (NM-16, IN-3; owner decision of 2026-10-13). A string literal inside a CHECK must match `^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$` when (a) it belongs to an `IN (…)` or `NOT IN (…)` list, or to `= ANY (ARRAY[…])` or `<> ALL (ARRAY[…])` (the form `pg_get_constraintdef` prints for an `IN` list), on any column; or (b) it is compared with `=`, `<>` or `!=` to an enumerated column, that is a column equal to or ending in `_<word>` for a word of `policy/enumeration_words.txt` (NM-8). Casts (`'X'::text`, `::text[]`) and parentheses around either side are ignored; literals inside comments are not values. Exempt: columns whose enumeration word is `format`, because their values are format tags such as `orcon/flow@1` (identifiers, JS-3); and a constraint listed in `policy/exceptions.txt` under rule `NM-16`, one line per dialect (for example values fixed by an external standard, such as OAuth client authentication methods). `tools/dbcheck` (`ddl-enum-values`) checks every migration file; `dbrules` checks the applied catalog on both engines (SQLite: the CHECK text in `sqlite_schema.sql`; PostgreSQL: `pg_get_constraintdef` of every constraint with `contype = 'c'`). Both read the same two policy files and apply the same rule; neither holds its own list. A finding reads `NM-16 enumeration value "<value>" in CHECK <constraint> is not UPPER_SNAKE (fix: '<VALUE>')`, where `<VALUE>` is the value uppercased with every run of other characters replaced by `_`. Fix: write the value in UPPER_SNAKE in the migration and in the code that writes it; the API already exposes that spelling, so nothing translates it. A merged migration is never edited (MG-5): changing stored values is an expand and contract change (MG-6).

## Schema checks

- **CK-4** `dbrules.LintSQLite(ctx, *sql.DB, Policy)` and `dbrules.LintPostgres(ctx, *pgx.Conn, Policy)` read the catalog after the migrations are applied (SQLite: `sqlite_schema`, `pragma_table_list`, `pragma_table_xinfo`, `pragma_foreign_key_list`, `pragma_index_list`, `pragma_index_xinfo`; PostgreSQL: `pg_class`, `pg_attribute`, `pg_constraint`, `pg_index`, `pg_policy`, `pg_type`, `pg_proc`, `pg_views`, `pg_trigger`, `pg_namespace`, `has_table_privilege`, `pg_roles`). They run in `go test` (SQLite) and `make test-postgres` (PostgreSQL 14 and 17, Docker required). Each finding names the rule; fix the migration that created the object with a new migration.

| Area | Rules | Verifies |
|---|---|---|
| Names | NM-1..NM-13 | snake_case and at most 63 bytes; singular table names unless in `singular_exceptions.txt`; NM-3 on tables, columns, constraints, indexes, functions, schemas and PostgreSQL roles starting with `orcon_`; no reserved word (`reserved_words.txt`); constraint and index prefixes; index names list their columns in order (or a purpose when that exceeds 63 bytes). SQLite constraint names are read from the `CONSTRAINT <name>` text of `sqlite_schema.sql` |
| Required columns | NM-4, NM-6, NM-7 | `id` as the only primary key column (except `schema_migration`); `created_at`; `updated_at` unless append-only; `_at` columns are `timestamptz` or `INTEGER` and every `timestamptz` column ends in `_at`; booleans are `boolean` or `INTEGER` with an `is_` or `has_` prefix |
| Organization | TI-1, TI-7, ID-8 | every table has `org_id` NOT NULL or is listed in `instance_tables.txt` (`org` is isolated by its own `id`); foreign keys between organization tables are composite `(org_id, <x>_id) → (org_id, id)`; referenced organization tables have `uq_<table>_org_id_id` |
| Row-level security and privileges (PostgreSQL) | TI-4, TI-5, TI-10, IN-11 | `ENABLE` and `FORCE ROW LEVEL SECURITY` on every organization table; policy `org_isolation` `FOR ALL` whose `USING` and `WITH CHECK` equal the TI-4 template; `org` has `org_self_isolation`; `orcon_application` is not a superuser, has no `BYPASSRLS` and owns no table; no `UPDATE` or `DELETE` on append-only tables for `orcon_application`; no views; functions only in `orcon_engine` and `orcon_context`; triggers only from the exception list; no ENUM type; `CREATE` on `public` revoked from `PUBLIC` |
| Integrity | IN-1..IN-10, NM-16 | every `*_id` column is a foreign key unless in `external_ids.txt`; every foreign key declares `ON DELETE`; no composite foreign key with `SET NULL`; every foreign key has a supporting index; every enumerated column (`status`, `kind`, `mode`, `state`, `level`, `role`, `*_status`, `*_kind`, `*_mode`, `*_state`, `*_format`) has a CHECK; SQLite booleans and ids carry their CHECKs; SQLite tables are `STRICT`; every enumeration value in a CHECK is UPPER_SNAKE (CK-24) |
| Actors | IN-12, NM-5 | every column ending in `_by` or containing `_by_` is `<verb>_by_principal_id`, nullable, with a single-column foreign key to `principal(id)` `ON DELETE SET NULL` |
| Soft delete | DL-4 | a table has `deleted_at` if and only if it is in `soft_delete_tables.txt`; listed tables also have `deleted_by_principal_id` and one partial `uq_` index `WHERE deleted_at IS NULL` over their business key |
| JSON | JS-3, JS-4 | every `document` or `*_document` column has a sibling `*_document_format` with a CHECK; `jsonb` only on columns with an exception |
| Indexes | QY-7 | no non-unique index whose columns are a prefix of another index on the same table |

## Connection checks

- **CK-5** `dbrules.CheckSQLiteConnection(ctx, *sql.Conn, role)` verifies `foreign_keys = 1`, `busy_timeout ≥ 1000`, `trusted_schema = 0`; on the writer `journal_mode = wal` and `synchronous = 2` (FULL) or the explicitly configured value; on a reader `query_only = 1` (CN-2..CN-6).
- **CK-6** `dbrules.CheckPostgresSession(ctx, pgx.Tx)` verifies `TimeZone = UTC`, `current_user = 'orcon_application'`, an empty `current_setting('orcon.org_id', true)` when the transaction starts, `transaction_isolation = 'read committed'`, and non-null `idle_in_transaction_session_timeout` and `lock_timeout` (CN-7, CN-8, TI-6).
- **CK-7** `TestForeignKeysOnEveryConnection` holds several reader connections and the writer at the same time and checks each one (successive queries are not enough: they reuse one connection). Fix: set the pragma through the driver's per-connection parameters.

## Plan checks

- **CK-8** Input: the queries tagged `-- orcon:hot` in each dialect's query files, with the schema applied (QY-4).
- **CK-9** SQLite: `EXPLAIN QUERY PLAN <sql>` with unbound parameters. Fails on a `SCAN <table>` line without `USING INDEX`, `USING COVERING INDEX` or `USING INTEGER PRIMARY KEY`, and, for `:many` queries with `ORDER BY`, on `USE TEMP B-TREE FOR ORDER BY`.
- **CK-10** PostgreSQL 14 and 17: `SET enable_seqscan = off`, `SET enable_sort = off` (only for `:many` with `ORDER BY`), `PREPARE`, `SET plan_cache_mode = force_generic_plan` and `EXPLAIN (FORMAT JSON) EXECUTE` with zero values of the inferred parameter types. With `enable_seqscan = off` the planner picks `Seq Scan` only when no index is usable, so any `Seq Scan` node fails, and so does a `Sort` node in a list. The result is deterministic without data or statistics.
- **CK-11** `orcon_engine` functions: `SECURITY DEFINER` bodies are not expanded by `EXPLAIN`, so the test loads `auto_explain` (`log_nested_statements = on`, `log_min_duration = 0`, `log_format = json`, `log_level = notice`) on a superuser test connection of the container, captures the notices with the pgx notice handler and applies the same rule to every nested plan.
- **CK-12** Fix: add or reorder an index justified by the query (QY-7), or rewrite the query to use keyset pagination (QY-5). Hot patterns at birth (`hot_query_patterns.txt`): `^Claim`, `^Renew`, `^(List|Count)Executions?`, `ExternalOperation`, `^(List|Count)AttentionRequests?`, `^(Get|Resolve)TriggerAddress`, `^Append(Journal|ExecutionEvent)`.

## Upgrade checks

- **CK-13** `dbrules.RunUpgradeTests(t, engine, migrations, testdata, apply)`: for every migration N, on a fresh database, apply migrations 1..N−1 each followed by its `NNNN_seed.sql`, apply N, run `N_verify.sql` (every statement must return zero rows; a row is a failure with its message), then the integrity checks (SQLite `PRAGMA foreign_key_check` and `PRAGMA integrity_check`; PostgreSQL: every constraint validated). Runs on SQLite and on PostgreSQL 14 and 17 (MG-11). A per-version template database or file copy is an allowed optimization.
- **CK-14** `apply` is the migration runner of the storage contract (ADR-0015 §4, with `schema_migration`, checksum and compatibility floor); fixtures use a reference applier that runs the files in order. Fix: correct the new migration or its seed and verify files; never edit an earlier migration (MG-5).

## Naming lists

- **CK-15** `policy/allowed_acronyms.txt` (allowed short forms) and `policy/abbreviation_denylist.txt` (the checked definition of an abbreviation) are the single source of the NM-3 decision. `tools/dbcheck` (`migration-names`, `query-names`, `policy`) and `dbrules` (introspection) both read them; neither holds its own list. The tokenizing and decision algorithm is defined once, in NM-3.
- **CK-16** Who changes each list: adding an acronym to `allowed_acronyms.txt` loosens the rule and needs a pull request approved by the maintainers; adding a token to `abbreviation_denylist.txt` tightens the rule and needs only ordinary review. A reviewer who finds an abbreviation the checker missed adds it to the denied list in the same change.
- **CK-17** The other declaration files of `policy/`: `instance_tables.txt`, `append_only_tables.txt`, `soft_delete_tables.txt` (changed only by an accepted spec that requires recovery, DL-4), `external_ids.txt`, `singular_exceptions.txt`, `reserved_words.txt`, `hot_query_patterns.txt`, `dialect_only_queries.txt`, `query_verbs.txt` (exactly the NM-14 verbs), `enumeration_words.txt` (the NM-8 words and `format`). In `external_ids.txt`, `*.<column>` applies to every table and is valid only when no table is named after the column without `_id`. Each line carries its reason after `# `.

## Checks owned by other specs

- **CK-18** `tools/sqlcheck` (scope `org` statements filter by `org_id`, TI-3) and the adversarial isolation battery `test/isolation` (TI-11) belong to SPEC-004.
- **CK-FLOOR** Compatibility floors (an older binary against a newer schema, ADR-0015 §4) are verified by the migration runner slice of SPEC-001; this rulebook only references it.
- **CK-19** UPPER_SNAKE spelling of public values (NM-16, JS-8) is checked on the API contract by the `enum-upper-snake` lint of SPEC-012. Stored values in enumeration CHECK constraints are checked by CK-24.

## Exceptions

- **CK-20** `policy/exceptions.txt` holds owner-approved exceptions (JSON-path queries, `jsonb` columns, `OFFSET`, `WITHOUT ROWID`, FTS5 triggers and similar). One line per exception: `<rule-id>  <dialect>:<object>  <reason>  <YYYY-MM-DD>  approved-by:<owner>`. The file is empty at birth.
- **CK-21** An exception is added only by a pull request approved by the maintainers. The query that uses it carries `-- orcon:exception <rule-id>` (QY-9), matching its line. The `policy` check rejects a line without `approved-by:` or past its date.

## mneme

- **CK-22** Specifications, acceptance criteria and QA reports of schema changes live in mneme, the project's tool for the backlog and specifications.
- **CK-23** After editing this skill, run `make skill-sync`, then `mneme skills lint orcon-db` and `mneme skills validate orcon-db`. Never run `mneme skills pin` on it: pinning rewrites the frontmatter and flattens `metadata`.
