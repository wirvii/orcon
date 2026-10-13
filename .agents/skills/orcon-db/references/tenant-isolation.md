<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Tenant isolation

Status: Accepted

This document defines how every organization's data is kept separate in both stores. Rule IDs (`TI-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

Encodes SPEC-004 S3 (R-ISO-01..11, ADR-0046, ADR-0047) and ADR-0077 §6.

## Rules

- **TI-1** Every row of an organization-scoped table carries `org_id` (`uuid NOT NULL` / `BLOB NOT NULL`). Tables without it are listed with a reason in `policy/instance_tables.txt`; the schema check fails on any other table. The `org` table is isolated by its own `id`. `principal` is an instance table (it holds only `id`, `kind` and `created_at`; names and contact data live in `application_user` and the organization-scoped `service_account`).
- **TI-2** Wall 1, single scoped data-access layer: SQL exists only in `internal/adapter/{sqlite,postgres}` (DA-2 and the depguard rules of the Go rulebook). Organization-scoped store methods take `orgscope.Org`, obtainable only from a resolved `AuthContext` or from rows returned by engine functions. Every query declares `-- orcon:scope org|instance|engine`.
- **TI-3** Wall 2, static check in CI: `tools/sqlcheck` rejects a `scope org` statement that references an organization-scoped table without `org_id = <scope parameter>` (WHERE, JOIN ON, INSERT values, UPDATE/DELETE WHERE); `scope engine` statements are listed in the reviewed engine allowlist.
- **TI-4** Wall 3, PostgreSQL row-level security on every organization-scoped table: `ALTER TABLE … ENABLE ROW LEVEL SECURITY` and `FORCE ROW LEVEL SECURITY`; policy exactly:
  ```sql
  CREATE POLICY org_isolation ON <table> FOR ALL
    USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
           OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
    WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
           OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
  ```
  The `org` table uses the same expression on `id` (`org_self_isolation`). `orcon_context.current_org()` returns `uuid`; `orcon_context.current_org_ids()` returns `SETOF uuid`, `STABLE`, one row per organization of the transaction-local setting `orcon.org_ids` and no rows when it is unset (ADR-0046, ADR-0047 §4). An array return type fails with the subquery form above (`uuid = uuid[]`).
- **TI-5** Roles: `orcon_owner` owns the schema and runs migrations; `orcon_application` is the runtime role, `NOSUPERUSER NOBYPASSRLS`, owner of nothing, granted `SELECT, INSERT, UPDATE, DELETE` (append-only tables: `SELECT, INSERT`); `orcon_engine` is `NOLOGIN NOBYPASSRLS` and owns the `orcon_engine` functions. With ≥ 2 organizations Orcon refuses to start, and refuses to create the second organization, if these hold false (R-ISO-06).
- **TI-6** Organization context is transaction-local only: `SELECT set_config('orcon.org_id', $1, true)`. Never a session-level `SET`. A pool hook asserts the context is empty when a transaction starts. No context means zero rows.
- **TI-7** Foreign keys between organization-scoped tables are composite: `FOREIGN KEY (org_id, <parent>_id) REFERENCES <parent> (org_id, id)`. A foreign key from an organization-scoped table to an instance table (`worker`, `principal`, `org`) is single-column.
- **TI-8** Parameterized queries only; SQL is never built from strings (DA-2). Row-level security does not defend against SQL injected into the runtime role (it could set the context); parameters are the defence.
- **TI-9** Engine plane: cross-organization discovery happens only in `orcon_engine` functions (`SECURITY DEFINER`, `SET search_path = orcon_engine, public`, `EXECUTE` granted only to `orcon_application`, every returned row carries `org_id`, bodies reviewed and covered by contract tests). Batched writes across organizations stay one transaction: per-organization statements, each preceded by `set_config`, sent as one pipelined batch (V1), or the measured list-context fallback (V2), per ADR-0047.
- **TI-10** No views over organization data (PostgreSQL 14 views run with owner privileges and bypass row-level security). No functions outside `orcon_engine` and `orcon_context`. No triggers except those listed in `policy/exceptions.txt`. The first PostgreSQL migration revokes `CREATE` on schema `public` from `PUBLIC`.
- **TI-11** Wall 4: the adversarial isolation battery (`test/isolation`) covers every endpoint and store method, on SQLite and PostgreSQL, and on PostgreSQL also in row-level-security-only mode. A new store method or endpoint adds its case in the same change.
- **TI-12** Caches of organization data include `org_id` in their key; blob paths start with `org/<orgId>/`; the webhook route index carries the organization (R-ISO-08).
- **TI-13** SQLite: walls 1, 2 and 4 apply; there is no row-level security; creating a second organization warns in the UI, the log and the audit log (R-ISO-09).
- **TI-14** Lookups that happen before the organization is known (incoming webhook route, API key or OAuth token hash, wait resume secret, public form or chat entry) use engine functions (`resolve_trigger_address`, `resolve_api_key`, `resolve_oauth_token`, `resolve_wait_secret`, `resolve_public_entry`) that return `org_id`; the request then continues with organization context.
