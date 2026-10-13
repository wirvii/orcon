<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Integrity

Status: Proposed — pending owner approval

This document defines the constraints the database enforces: foreign keys, CHECK constraints, NOT NULL, uniqueness, append-only tables and actor columns. Rule IDs (`IN-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

## Rules

- **IN-1** Every reference is a named foreign key with an explicit `ON DELETE CASCADE`, `ON DELETE RESTRICT` or `ON DELETE SET NULL`. `NO ACTION` and the implicit default are rejected. Ids never change, so `ON UPDATE` is not declared.
- **IN-2** SQLite enforces foreign keys only when `PRAGMA foreign_keys = ON` is set on each connection; every connection sets it (CN-2) and a test proves it on every pooled connection.
- **IN-3** States and enumerations are `text`/`TEXT` with `CONSTRAINT ck_<table>_<column> CHECK (<column> IN (…))`. PostgreSQL ENUM types are forbidden (no SQLite equivalent; hard to evolve). Adding a value is an `additive` migration that widens the CHECK (a `rebuild` on SQLite). Value spelling follows NM-16.
- **IN-4** `NOT NULL` wherever a value is required. A nullable column states in a comment what absence means.
- **IN-5** Business uniqueness is a named UNIQUE constraint or a partial unique index (`uq_flow_org_id_project_id_name … WHERE deleted_at IS NULL`).
- **IN-6** A composite foreign key never uses `ON DELETE SET NULL` (it would null `org_id`; PostgreSQL 14 and SQLite cannot limit SET NULL to one column). Use `RESTRICT` and clear the column in the same transaction before deleting the parent, or `CASCADE`. Single-column foreign keys to instance tables (IN-12, `task_attempt.worker_id`) may use `SET NULL`.
- **IN-7** Every foreign key has an index whose leading columns are the foreign key columns (or a primary key or unique constraint that starts with them), so parent deletes and joins do not scan.
- **IN-8** A rejected write is the intended fail-fast behavior. Adapters map violations by constraint name to area errors (unique → conflict, foreign key → not found or in use, check → invalid) and never retry them blindly (DA-7.3 retries only serialization and busy errors).
- **IN-9** Bounded text and numbers have range CHECKs (`ck_<table>_<column>_length`, `ck_<table>_<column>_range`).
- **IN-10** SQLite booleans carry `CHECK (x IN (0, 1))`; SQLite ids carry the length check (ID-2).
- **IN-11** Append-only tables are listed in `policy/append_only_tables.txt`: no `updated_at`; `orcon_application` has no UPDATE or DELETE privilege; rows leave only through retention (`orcon_engine.prune_batch`).
- **IN-12** Actors. Whoever acts (a user, a service account, the system) is one row of the instance table `principal(id, kind, created_at)`, `kind` in `USER`, `SERVICE_ACCOUNT`, `SYSTEM` — the spelling the API exposes (NM-16) — with `CONSTRAINT ck_principal_kind CHECK (kind IN ('USER', 'SERVICE_ACCOUNT', 'SYSTEM'))` and exactly one `SYSTEM` row (`uq_principal_kind` partial unique index `WHERE kind = 'SYSTEM'`, created by instance bootstrap with an application-generated id). `application_user` and `service_account` reference it through `principal_id` with `uq_<table>_principal_id` and `ON DELETE CASCADE`. Every column that records who did something is `<verb>_by_principal_id`, nullable, with `CONSTRAINT fk_<table>_<verb>_by_principal FOREIGN KEY (<verb>_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL` and its supporting index; the comment states "NULL when the principal was deleted; the audit log keeps the identity". Deleting a principal is a store method that, in bounded batches, sets those columns to NULL (`UPDATE … WHERE id IN (SELECT id … LIMIT $n)`), then deletes the `principal` row with its audit event in one short transaction; the foreign key action is the safety net, not the mechanism (QY-8). Audit tables keep `actor_principal_id` without a foreign key (listed in `policy/external_ids.txt`) plus the actor kind and the keyed hash of ADR-0048.
