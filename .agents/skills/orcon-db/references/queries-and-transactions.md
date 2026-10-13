<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Queries and transactions

Status: Proposed — pending owner approval

This document defines how SQL queries are written, annotated, bounded, indexed and run in transactions. Rule IDs (`QY-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

## Rules

- **QY-1** All SQL is compiled by sqlc in the two storage adapters (DA-1). Both dialect sets expose the same query names, except names in `policy/dialect_only_queries.txt`.
- **QY-2** Column lists are explicit: no `*` in a SELECT list or in RETURNING (`count(*)` is allowed) (DA-6).
- **QY-3** Values are parameters only.
- **QY-4** Hot-path queries carry `-- orcon:hot`: engine claim and lease renewal, journal reads and writes, execution event appends, attention inbox lists, execution lists, trigger route resolution. A query whose name matches `policy/hot_query_patterns.txt` must carry the tag. The plan test fails on a full table scan (SQLite `SCAN <table>` without an index; PostgreSQL `Seq Scan` with `enable_seqscan = off`) and, for list queries, on a sort step (SQLite `USE TEMP B-TREE FOR ORDER BY`; PostgreSQL `Sort` with `enable_sort = off`). Engine functions are checked through `auto_explain` nested plans.
- **QY-5** Lists use keyset pagination: `ORDER BY <sort columns>, id` and `WHERE (<sort columns>, id) < ($sort, $id)` (or `>`); the last ORDER BY key is unique (`id` or `sequence_number`). `OFFSET` is not used.
- **QY-6** Every multi-row query (`:many`) has a top-level `LIMIT`; the store clamps page sizes to at most 1000 rows (engine batches use their own documented limits, for example 512 journal records). Bulk UPDATE and DELETE are bounded with `WHERE id IN (SELECT id … LIMIT $n)`. UPDATE and DELETE without WHERE are rejected.
- **QY-7** Every index is justified by a query named in a comment above the index statement in the migration (DA-6), named (NM-11) and not a prefix of another non-unique index on the same table. Request-plane list indexes lead with `org_id` (ADR-0047 §5); claim indexes keep their leading columns because claims run as `orcon_engine`.
- **QY-8** Transactions are short. No network, connector, model provider, secret store or other external call happens inside a transaction (DA-7). A blob is written and synced before the transaction that references it commits (ADR-0016). A durable state transition and the enqueueing of the next runnable work commit in one transaction. PostgreSQL uses READ COMMITTED with conditional updates and `FOR UPDATE SKIP LOCKED` (ADR-0013); SQLite writes go through the single writer.
- **QY-9** Annotations, each on its own line after `-- name:`: `-- orcon:scope org|instance|engine`; `-- orcon:hot`; `-- orcon:delete audit:<action>|retention:<class>|bookkeeping:<reason>`; `-- orcon:include-deleted <reason>`; `-- orcon:exception <rule-id>` (must match a line in `policy/exceptions.txt`).
- **QY-10** A query sqlc cannot analyze (for example an FTS5 `MATCH`) is a hand-written adapter method with a comment and contract-test coverage, under the same rules (DA-8).
