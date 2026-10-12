<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Data access

Status: Accepted

> Amended 2026-10-12 (owner decision, SPEC-009 P4): DA-3 aligned with the orcon-db rule against querying inside JSON documents.

This document defines how Orcon reads and writes its durable state in SQLite (standalone) and PostgreSQL (distributed). It applies to the storage adapters `internal/adapter/sqlite`, `internal/adapter/postgres` and `internal/adapter/storetest`, and to the area ports they implement. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`DA-…`) are stable. The foundational choices are recorded in ADR 0076 §3.

## Principles

- **DA-1** SQL is hand-written in `internal/adapter/sqlite/queries/*.sql` and `internal/adapter/postgres/queries/*.sql`, compiled to Go by sqlc into each adapter's `sqlcgen` package. The generated code is committed and `make generate-check` fails when it is stale.
- **DA-2** No ORM, no query builder and no SQL assembled from strings at run time (GORM, ent, bun and sqlx are banned by depguard). SQL text appears only in the two storage adapters; areas see ports and area types (architecture-layers.md AL-2, AL-4).
- **DA-6** Queries name their columns explicitly (no `SELECT *`), filter every tenant-owned table by `org_id`, bound every result set (`LIMIT` or a keyed page), and every index exists for a query that uses it, with that query named in the migration comment.

## sqlc

- **DA-1.1** `sqlc.yaml` at the repository root declares one SQLite set (`engine: sqlite`, `modernc.org/sqlite` through `database/sql`) and one PostgreSQL set (`engine: postgresql`, `sql_package: pgx/v5`).
- **DA-1.2** sqlc reads the schema from the migrations directory of each adapter (DA-5), so the schema has a single source.
- **DA-1.3** Generated options: no JSON tags, no generated interfaces, empty slices instead of `nil`, value (not pointer) results and parameters, at most three positional parameters before a params struct; output files `db.gen.go` and `models.gen.go`.
- **DA-1.4** Every query has a sqlc annotation (`-- name: ClaimNextTask :one`) whose name reads as the operation in the area's vocabulary.
- **DA-1.5** Generated types (`sqlcgen.*`) never leave the adapter; the adapter converts them into area types (AL-5.1).
- **DA-8** A query sqlc cannot analyze (for example SQLite FTS5 virtual-table queries) is written as a hand-written adapter method using the same connection, with a comment that says why sqlc could not be used, and is covered by the contract suite.

## Two dialects

- **DA-3** Each dialect uses its strengths behind the same area ports:
  - SQLite: FTS5 for search, WAL mode with `synchronous=FULL`, a single writer connection with group commit.
  - PostgreSQL: `FOR UPDATE SKIP LOCKED` for task claims, `LISTEN/NOTIFY` for wake-ups, row-level security as a second tenant barrier.
- **DA-3a** SQL does not query inside JSON documents: no JSON-path expressions, no `jsonb` operators (`->`, `->>`, `#>`, `@>`, `?` and similar) and no SQLite `json_*` functions in `WHERE`, `ORDER BY` or `JOIN`, unless a written, justified exception exists for that query.
- **DA-3b** Every value that is filtered, sorted or joined on lives in a real column with its own type and constraints.
- **DA-3c** JSON columns hold whole documents only, written and read as a unit. In PostgreSQL their type is `bytea` by default; `jsonb` is used only together with an approved exception (DA-3a).
- **DA-3.1** The two query sets are separate files, not one portable subset. Do not lower one dialect to the other's capabilities.
- **DA-3.2** Behavior visible to areas MUST be identical in both dialects, including ordering, error mapping (AL-5.3) and timestamp precision; the contract suite proves it (DA-4).

## Contract test suite

- **DA-4** `internal/adapter/storetest` holds one suite of contract tests that runs against both adapters: always against real SQLite, and against real PostgreSQL with `make test-postgres` (testcontainers, a unique database per test).
- **DA-4.1** Every port method implemented by a storage adapter has contract tests, including its error cases (not found, conflict, lost lease) and concurrent claims.
- **DA-4.2** A behavior covered only by a dialect-specific test is a defect unless the port documents the difference.

## Migrations

- **DA-5** Migrations follow ADR 0015. Each adapter has its own ordered migrations directory; migrations are forward-only, transactional where the dialect allows, and never edited after release.
- **DA-5.1** A schema change adds a migration for both dialects in the same change, regenerates sqlc output and extends the contract suite.
- **DA-5.2** Migrations that rewrite large tables state their expected duration and locking in a comment.

## Transactions

- **DA-7** Transactions are short and never span an external call: network, sandbox, connector or model provider (AL-6.3, ADR 0005).
- **DA-7.1** Atomic multi-area writes use one port method or the consumer-declared `InTx` port (AL-6.1); `*sql.Tx` and `pgx.Tx` stay inside the adapter.
- **DA-7.2** Rollback and commit errors are handled per go-style.md GS-E8; a rollback after a successful commit is a no-op, not an error.
- **DA-7.3** Retries on serialization or busy errors happen inside the adapter, bounded and with backoff, and only for functions that are safe to repeat.
