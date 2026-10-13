---
name: orcon-db
description: Applies the Orcon database schema standards - naming without abbreviations, UUIDv7 ids, SQLite and PostgreSQL type parity, forward-only paired migrations, organization isolation with row-level security, integrity constraints, deletion and retention, JSON documents, queries and indexes, SQLite connection settings - and the checks that enforce them. Use when creating or changing any table, column, index, constraint, migration, SQL query or storage adapter in the Orcon repository.
license: Apache-2.0
compatibility: Rules in references/ are self-contained; scripts/ require an Orcon repository checkout with Go (two latest stable releases), GNU make, git and uv, plus Docker for the PostgreSQL checks.
metadata:
  version: 1.1.0
  pinned: true
  owner: wirvii
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Orcon DB

The complete rules are in `references/`. This file is the procedure and the checklist; open the reference named in each step when the step applies.

Precedence: accepted ADRs > accepted `orcon-go` rules (`data-access.md`, DA) > `references/` > reviewer preference. Go code that calls the database also follows the `orcon-go` skill.

| Reference | Rules |
|---|---|
| `references/naming.md` | NM - tables, columns, keys, actor columns, allowed acronyms, constraint and index names, query and migration file names, value spelling |
| `references/identifiers.md` | ID - UUIDv7 generated in Go, storage, text form, sequence numbers |
| `references/types-and-time.md` | TY - type mapping, forbidden types, STRICT tables, UTC, unix microseconds |
| `references/migrations.md` | MG - pairs, header, kinds, immutability, expand/contract, seeds |
| `references/tenant-isolation.md` | TI - org_id, scoped layer, row-level security, roles, engine functions |
| `references/integrity.md` | IN - foreign keys, CHECK, NOT NULL, UNIQUE, error mapping, principals |
| `references/deletion-and-retention.md` | DL - hard delete with audit, flow soft delete and purge, retention |
| `references/json.md` | JS - whole documents, real columns, format tags, no JSON paths, public value spelling |
| `references/queries-and-transactions.md` | QY - sqlc, explicit columns, hot paths, keyset, LIMIT, indexes, transactions |
| `references/connections.md` | CN - SQLite pragmas and single writer, PostgreSQL session settings |
| `references/checks.md` | every automated check, naming lists, exceptions, mneme |

## When To Use

- You are about to create or change a table, column, index, constraint, row-level security policy, engine function, migration, seed or verify file in this repository.
- You are writing or changing a SQL query, a sqlc query file or a store method in `internal/adapter/sqlite` or `internal/adapter/postgres`.
- A design or spec lists tables or columns, or a review asks whether a schema follows Orcon conventions.
- You add an entry to a file in `internal/adapter/storetest/dbrules/policy/`.

## Critical Rules

1. Names: snake_case, singular tables, primary key `id`, foreign keys `<table>_id`, instants `_at`, booleans `is_`/`has_`, no reserved words, and no abbreviations except `id`, `org` and the acronyms in `policy/allowed_acronyms.txt` - write `reference`, `maximum`, `milliseconds` (NM, NM-3).
2. Every constraint and index is named: `pk_`, `fk_`, `uq_`, `ck_`, `ix_` (NM-11).
3. Ids are UUIDv7 generated in Go and passed as parameters; never a database default (ID-1).
4. Types follow the mapping; every SQLite table is STRICT; instants are UTC, `timestamptz` or unix microseconds; the application supplies times (TY).
5. Migrations come in pairs with the same number and name, carry the header, are forward-only, and are never edited after merge; breaking changes use expand then contract in a later release (MG).
6. Every migration ships seed and verify files for both dialects (MG-11).
7. Every organization-scoped table has `org_id`, composite foreign keys, and on PostgreSQL enabled and forced row-level security with the `org_isolation` policy; anything else is listed as an instance table (TI).
8. Context is transaction-local `set_config('orcon.org_id', …, true)`; queries are parameterized; no views; cross-organization work only through `orcon_engine` functions (TI).
9. Every reference is a foreign key with explicit `ON DELETE`; enumerations are TEXT with a CHECK that lists UPPER_SNAKE values, never ENUM types; NOT NULL and UNIQUE where they apply; every foreign key has an index; who-did-it columns are `<verb>_by_principal_id` with `ON DELETE SET NULL` (IN, IN-12).
10. Deletes are hard and audited in the same transaction; only `flow` has soft delete (30 days, then an audited purge), always filtered; accumulating data leaves through retention (DL).
11. JSON only for whole documents, `bytea`/`BLOB` with a `_document_format` tag; anything filtered or joined is a real column; no JSON paths in SQL and no `jsonb` without an approved exception; enum values inside documents the API returns verbatim are UPPER_SNAKE (JS, NM-16).
12. Queries: explicit columns, `-- orcon:scope` on every query, `LIMIT` on every multi-row query, keyset pagination, no `OFFSET`, hot paths tagged and indexed, same query names in both dialects (QY).
13. Transactions are short and never wrap a network or connector call (QY-8).
14. SQLite connections: `foreign_keys`, `busy_timeout` and `trusted_schema` on every connection; one writer in WAL; read-only readers (CN).
15. Edit this skill only under `.agents/skills/orcon-db/` and re-run `scripts/project.sh`; never run `mneme skills pin` on it.

## Automated Checks

| Check | What it verifies | How to fix |
|---|---|---|
| `make db-check` migration-names, migration-pairs | file names without abbreviations, contiguous numbers, every migration has its pair, same header, seed and verify files | Add or rename the missing file; copy the header to both files |
| `make db-check` migration-immutable | merged migrations match `migrations.sum` and the base branch | Revert the edit and write a new migration; run `make db-sum` for new files only |
| `make db-check` migration-expand-contract | no drop, rename, type change or new NOT NULL outside a contract migration | Split into expand now and contract in a later release |
| `make db-check` ddl-forbidden | no ENUM, forbidden types, id or time defaults, views, triggers, non-STRICT SQLite tables, unnamed constraints | Use the mapping in `references/types-and-time.md` and name the constraint |
| `make db-check` ddl-enum-values | every value in an enumeration CHECK is UPPER_SNAKE (CK-24) | Write the value in UPPER_SNAKE; a value fixed by an external standard needs an owner-approved `NM-16` exception |
| `make db-check` query-select-star, query-json-path | explicit columns; no JSON paths or JSON functions | List the columns; move filtered data to a real column |
| `make db-check` query-limit, query-offset, query-keyset | bounded multi-row queries, no OFFSET, unique last sort key | Add `LIMIT`, use keyset pagination ending in `id` |
| `make db-check` query-annotations, query-soft-delete, query-hot-tag | scope on every query, delete annotation, soft-delete filter, hot tag | Add the annotation or the `deleted_at IS NULL` filter |
| `make db-check` query-parity, query-names | same query names in both dialects, verb-first names without abbreviations | Write the missing dialect query or list it as dialect-only with a reason; spell the word out |
| `make db-check` policy | policy files well formed; acronym and abbreviation lists disjoint, sorted, with expansions | Fix the line; a new acronym needs owner approval |
| `make test` schema checks (SQLite) | names, mandatory columns, org_id, foreign keys, actor columns, soft delete only on listed tables, CHECKs, STRICT, JSON format columns | Fix the migration (new migration if already merged) |
| `make test-postgres` schema checks (PostgreSQL 14 and 17) | the above plus row-level security, policy text, roles and grants, no views or ENUM types, `jsonb` only with an exception | Add the RLS block and grants from `assets/migration-postgres.sql` |
| `make test` connection checks | `foreign_keys` and other pragmas on every SQLite connection; PostgreSQL session settings | Set the pragma in the connection parameters |
| `make test` / `make test-postgres` plan checks | hot queries and engine functions use an index and, for lists, no sort | Add or adjust the index, or rewrite the query |
| `make test` / `make test-postgres` upgrade checks | each migration upgrades seeded data from the previous version on both engines | Fix the migration or its seed and verify files |
| `scripts/check-db-fixtures.sh` | every planted violation is detected and every good fixture passes | Fix the checker, never the fixture |
| `make skill-check` | this skill conforms and its generated copy is identical | Fix `SKILL.md`, then run `make skill-sync` |
| `make check` | everything CI runs, in CI order | Fix the first failing target and rerun |

## Verification

1. `make check` exits 0 on your branch; CI runs the same command.
2. `make test-postgres` passes on PostgreSQL 14 and 17 (`sh scripts/verify.sh` runs both).
3. New store methods have their isolation battery case and contract test case.
4. This skill: `sh scripts/project.sh --home && mneme skills lint orcon-db && mneme skills validate orcon-db` pass.
5. Report the commands you ran, their results and known limitations when you hand back.

## Workflow

1. Read the accepted ADR or spec that authorizes the schema or query change (specs live in mneme); stop if it is only Proposed.
2. Classify the change: new table, new column, index, constraint, data fix or query only; pick the migration kind (`references/migrations.md` MG-6). A breaking change is an expand now and a contract in a later release.
3. Name every table, column, constraint and index (`references/naming.md`), spelling words out; decide organization-scoped or instance table (`references/tenant-isolation.md`); give every who-did-it column the `<verb>_by_principal_id` form (`references/integrity.md` IN-12).
4. Copy `assets/migration-sqlite.sql` and `assets/migration-postgres.sql` to the next free number with the same name; fill the header; write the DDL with types from `references/types-and-time.md`, constraints from `references/integrity.md`, and the row-level security block and grants in the PostgreSQL file.
5. Write `testdata/NNNN_seed.sql` and `testdata/NNNN_verify.sql` for both dialects, with at least two organizations.
6. Run `make db-sum` to record the new files. Never edit a migration that is already in `main`.
7. Write the queries in both dialect query files from `assets/queries-template.sql`: same names, `-- orcon:scope`, explicit columns, `LIMIT`, keyset order ending in `id`, `-- orcon:hot` on hot paths, `-- orcon:delete` on deletes (`references/queries-and-transactions.md`). Run `make generate`.
8. Write the store method: short transaction, no external call inside, audit event on user deletes, constraint errors mapped by name (`references/deletion-and-retention.md`, `references/integrity.md`).
9. Add the isolation battery case and the contract test case.
10. While iterating run `sh scripts/db-change-check.sh`; before handing back run `sh scripts/verify.sh`; paste `assets/pr-db-checklist.md` into the pull request.
11. Write the commit message in Conventional Commits, describing the change only; citing mneme identifiers (`SPEC-…`, `BL-…`) is fine.
