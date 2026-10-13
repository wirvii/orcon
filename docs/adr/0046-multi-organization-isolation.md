<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0046 — Multi-organization isolation: data-access layer, static check, PostgreSQL RLS and SQLite behaviour

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12 (owner answers; ADR-0077 §6); 2026-10-13 (return type of current_org_ids)

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
S3: every row owned by an organization; filtering enforced in a single data-access layer where queries cannot be written without organization scope; a CI static check; per-organization keys; per-organization limits; adversarial cross-organization tests on both stores; PostgreSQL RLS as a second wall with an application role without BYPASSRLS and organization context per session/transaction; SQLite allowed without RLS with a warning. ADR-0077 (database standards) amends this ADR: instance-table list location, query scope annotation, no views, `public` schema privileges, and names without abbreviations.

## Decision

### 1. Schema
Every customer-data table has `org_id NOT NULL`, a unique constraint `(org_id, id)` when other tables reference it, and composite foreign keys `(org_id, parent_id) REFERENCES parent(org_id, id)`, so a child row cannot reference a parent in another organization. A table with an optional `org_id` is not allowed: rows without an organization live in separate instance tables (key canaries, audit chains, alerts). Instance tables are listed explicitly, one per line with its reason, in `internal/adapter/storetest/dbrules/policy/instance_tables.txt` (next to the other database policy lists); a test fails if a table is neither organization-scoped nor listed. Initial content defined by the security design: `principal`, `application_user`, `session`, `instance_role_binding`, `instance_key`, `instance_key_canary`, `instance_audit_pending`, `instance_audit_event`, `instance_audit_block`; other designs add theirs in the change that creates them (storage: `schema_migration`, `worker`, `org`, `instance_alert`; connectors: `connector_package`, `catalog_source`, `catalog_metadata`, `connector_revocation`, `connector_revocation_override`, `code_library`; AI: `ai_price_table`, `instance_ai_price_override`, the SQLite full-text table `knowledge_chunk_fts5`). The `org` table is isolated by its own `id`.

### 2. Wall 1: data-access layer
- All SQL lives in `internal/adapter/sqlite` and `internal/adapter/postgres` (per-dialect sqlc query files). Each query declares its scope on its own line after `-- name:`: `-- orcon:scope org|instance|engine` (sqlc accepts no extra tokens on the `-- name:` line).
- Organization-scoped methods take `orgscope.Org`, a type with an unexported constructor obtainable only from `authz.AuthContext.Org()` or from rows returned by engine functions (ADR-0047 §2). The scope value is bound to the `org_id` parameter by generated code.
- Handlers load resources within the scope and check parent/child consistency; mismatches return 404.

### 3. Wall 2: static checks in CI
- The `orgscope` analyzer of `tools/orconvet` (go/analysis): forbids `database/sql`, `pgx` and SQLite query calls outside `internal/adapter/sqlite` and `internal/adapter/postgres`, and string concatenation into SQL anywhere.
- `tools/sqlcheck`: parses every statement with the PostgreSQL parser (`pg_query_go`, BSD-3, CI only) and fails if a reference to an organization-scoped table in a statement annotated `-- orcon:scope org` is not constrained by `org_id = $<scope parameter>` (WHERE, JOIN ON, INSERT values, UPDATE/DELETE WHERE); statements annotated `-- orcon:scope engine` must be listed in `internal/adapter/storetest/dbrules/policy/engine_allowlist.yaml` (CODEOWNERS-reviewed). SQLite statements are checked with the same tool on their PostgreSQL-compatible form or with a SQLite parser.

### 4. Wall 3: PostgreSQL row-level security
- Roles: `orcon_owner` (owns schema, runs migrations), `orcon_application` (runtime; `NOSUPERUSER NOBYPASSRLS`, owner of nothing), `orcon_engine` (`NOLOGIN NOBYPASSRLS`, owns `orcon_engine` functions). With a single owner DSN that has `CREATEROLE`, Orcon creates `orcon_application` and `orcon_engine` and uses `SET ROLE orcon_application` on each pooled connection, asserted per transaction; without `CREATEROLE`, `orcon db setup-roles` prints the SQL for a DBA (owner decision, SPEC-004 Q12).
- Every organization-scoped table: `ENABLE` and `FORCE ROW LEVEL SECURITY`; policy `org_isolation FOR ALL USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids()))) WITH CHECK (same)`; `current_org()` = `NULLIF(current_setting('orcon.org_id', true), '')::uuid` and `current_org_ids()` `RETURNS SETOF uuid` (`STABLE`) = the optional list context of ADR-0047 §4: one row per organization in the transaction-local setting `orcon.org_ids` (comma-separated ID text form), no rows unless that variant is selected. It returns a set, not an array, because the policy compares `org_id` with the rows of the subquery `(SELECT orcon_context.current_org_ids())`. The `org` table uses the same expression on `id` (`org_self_isolation`). No context → no rows. Functions exist only in the schemas `orcon_context` and `orcon_engine`.
- No views over organization data: PostgreSQL 14, the oldest supported version, runs views with their owner's privileges, which bypasses row-level security. The first PostgreSQL migration revokes `CREATE` on schema `public` from `PUBLIC`. DDL uses only what PostgreSQL 14 supports (no `NULLS NOT DISTINCT`, `security_invoker` views, `MERGE` or `ON DELETE SET NULL (column)`); composite foreign keys never use `SET NULL`, because it would null `org_id`.
- Context is set only with `set_config(..., true)` (transaction-local), never session-level; a pool hook asserts the context is empty at transaction start.
- Startup and second-organization checks: with ≥ 2 organizations Orcon refuses to start (and refuses to create the second organization) if the runtime role is superuser, has BYPASSRLS, owns tables, or any organization-scoped table lacks enabled+forced RLS and the policy. With one organization: warning.
- Honest scope: RLS defends against missing or wrong filters in Orcon's own queries. It does not defend against arbitrary SQL executed as `orcon_application` (an injected statement could set the context); injection is prevented by parameterized queries only.

### 5. SQLite
Walls 1, 2 and 4 apply; there is no RLS. Creating a second organization shows a warning in the UI, logs it and audits `sqlite.multi_org_warning`; documentation recommends PostgreSQL for multi-organization deployments. Physical isolation remains available as one instance per customer. One SQLite file per organization is rejected.

### 6. Other boundaries
In-process caches of organization data include `org_id` in their keys; blob storage paths are prefixed `org/<orgId>/` and ownership checks include the organization; the webhook route index carries the organization; sub-flow calls never cross organizations; all egress configured by an organization (including SIEM sinks and human-attention notification channels) goes through the egress client.

### 7. Wall 4
ADR-0050 §1.

## Consequences
Two independent walls on PostgreSQL, each tested alone; SQLite users are told plainly what they do not get; no view or optional `org_id` can open a path around row-level security.

## Amendments

Changes applied on 2026-10-13 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-009 D4 (2026-10-13).** §4: `orcon_context.current_org_ids()` returns `SETOF uuid` (no rows unless the ADR-0047 list variant sets `orcon.org_ids`), because the row-level security policy compares `org_id` with the rows of `(SELECT orcon_context.current_org_ids())`; an array return type makes that comparison fail. The policy text does not change.

## Alternatives considered
Schema per organization (rejected: migrations × N, connection/catalog bloat, cross-organization engine batches impossible); database per organization (rejected: same, plus SQLite-per-org rejected by the owner); RLS only, without the application layer (rejected by S3); warning only when the runtime role does not satisfy the checks (rejected: RLS would stop being a guarantee, SPEC-004 Q12); `security_invoker` views (rejected: not available on PostgreSQL 14).
