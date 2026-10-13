<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Migrations

Status: Accepted

This document defines how schema migrations are written, named, checked, kept immutable and tested on upgrade. Rule IDs (`MG-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

## Rules

- **MG-1** Every migration is a pair: `internal/adapter/sqlite/migrations/NNNN_<name>.sql` and `internal/adapter/postgres/migrations/NNNN_<name>.sql`, same number, same name, both mandatory (DA-5, DA-5.1). Numbers are contiguous from `0001`.
- **MG-2** Each file starts, after the SPDX lines, with a header; both files of a pair carry identical values:
  ```
  -- orcon:kind additive|expand|contract|rebuild|data
  -- orcon:compatibility-floor <semver>|self
  -- orcon:contracts NNNN          (kind contract only)
  -- orcon:no-transaction           (PostgreSQL only; one CREATE INDEX CONCURRENTLY)
  ```
  The ADR-0015 §4 runner reads `compatibility-floor` into `schema_migration.compatibility_floor`.
- **MG-3** Forward-only: no down scripts. Rolling back means running the previous binary (safe because of expand/contract and compatibility floors) or restoring a backup.
- **MG-4** Migrations are embedded with `//go:embed` and applied automatically at startup under the migration lock; each applied migration is recorded in `schema_migration(version, checksum, applied_at, binary_version, compatibility_floor)`; a binary below the effective floor refuses to start; an applied migration whose checksum differs refuses to start (ADR-0015 §4).
- **MG-5** A migration merged to `main` is immutable (stricter than, and therefore satisfying, DA-5 "never edited after release"). `migrations.sum` in each dialect directory lists `<file>  <sha256>`; `make db-sum` appends new files and never rewrites existing lines; `make db-check` rejects a changed, removed or unlisted file and any change to a line present in the base branch. SQL files are LF-normalized (`.gitattributes`). A fix is a new migration.
- **MG-6** Kinds:
  - `additive`: new table, new nullable column or column with a default, new index, widened CHECK; compatibility floor normally the previous minor release.
  - `expand`: adds the new structure; the code of that release writes both forms; floor = previous release.
  - `contract`: removes the old structure; allowed only when the `expand` migration named in `orcon:contracts` shipped in an earlier release tag; floor = `self`.
  - `rebuild`: SQLite table rebuild (MG-8) with the matching PostgreSQL `ALTER`.
  - `data`: data-only change; idempotent; bounded batches.
- **MG-7** Outside `contract`, these are rejected: `DROP TABLE`, `DROP COLUMN`, `RENAME` of a table or column, `ALTER COLUMN … TYPE`, `SET NOT NULL`, `ADD COLUMN … NOT NULL` without `DEFAULT`. Narrowing a CHECK is a breaking change (expand/contract) and is caught by the upgrade test (MG-11).
- **MG-8** SQLite cannot alter constraints in place. A `rebuild` migration follows the twelve-step procedure of sqlite.org ("Making Other Kinds Of Table Schema Changes"): the runner turns `foreign_keys` off before the transaction, runs `PRAGMA foreign_key_check` before commit and fails on any row, and turns `foreign_keys` back on. Rebuilding large tables is slow; enumerations are designed with room to grow. A migration that rewrites a table that can be large states its expected duration and locking in a comment (DA-5.2).
- **MG-9** PostgreSQL: one transaction per migration, except `no-transaction` migrations, which hold exactly one `CREATE INDEX CONCURRENTLY IF NOT EXISTS` for tables that can be large in production. The runner sets `lock_timeout = '5s'` so a migration fails instead of queueing behind traffic.
- **MG-10** DDL runs on the oldest supported PostgreSQL (14) and on the SQLite bundled with the driver. Features newer than PostgreSQL 14 are not used (`NULLS NOT DISTINCT`, `security_invoker` views, `MERGE`, `ON DELETE SET NULL (column)`).
- **MG-11** Each migration ships, per dialect, `migrations/testdata/NNNN_seed.sql` (representative rows valid at version NNNN, at least two organizations, a row in every organization-scoped table it creates) and `NNNN_verify.sql` (SELECT statements that return zero rows when the data survived correctly). The upgrade test applies migrations 1..N−1 with their seeds, applies N, runs the verify file and the integrity checks, on SQLite and PostgreSQL 14 and 17.
- **MG-12** Backfills of large tables run in bounded batches and state their expected duration and locking in a comment (DA-5.2); a backfill that cannot be bounded in a migration is a background job of the owning spec during the `expand` release.
- **MG-13** A migration that creates an organization-scoped table also creates, in the PostgreSQL file: `org_id`, `uq_<table>_org_id_id` when referenced, composite foreign keys, `ENABLE` and `FORCE ROW LEVEL SECURITY`, the `org_isolation` policy, and the grants to `orcon_application` (TI-4, TI-5). A new instance table is added to `policy/instance_tables.txt` in the same change.
- **MG-14** Start from `assets/migration-sqlite.sql` and `assets/migration-postgres.sql`.
