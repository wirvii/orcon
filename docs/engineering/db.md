<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Database engineering standards

Status: Accepted

Orcon runs on SQLite (single server) and PostgreSQL (distributed) with one logical schema. Every schema change is a forward-only pair of migrations, one per database, every row of customer data carries its organization, and the database itself enforces integrity through named constraints. The decisions behind these standards are recorded in [ADR 0077](../adr/0077-database-schema-standards-and-storage-contract-corrections.md).

This page is a short orientation for contributors. It contains no rules of its own.

## The database rulebook

The complete database rulebook is in `.agents/skills/orcon-db/references/`:

- [Naming](../../.agents/skills/orcon-db/references/naming.md) — identifiers without abbreviations, keys, constraint and index prefixes, query and migration names, spelling of public values.
- [Identifiers](../../.agents/skills/orcon-db/references/identifiers.md) — UUIDv7 row identity, storage and text form, sequence numbers, natural keys.
- [Types and time](../../.agents/skills/orcon-db/references/types-and-time.md) — type mapping between the two databases, forbidden types, UTC instants.
- [Migrations](../../.agents/skills/orcon-db/references/migrations.md) — pairs, headers, immutability, expand and contract, upgrade tests.
- [Tenant isolation](../../.agents/skills/orcon-db/references/tenant-isolation.md) — `org_id`, row-level security, roles, organization context.
- [Integrity](../../.agents/skills/orcon-db/references/integrity.md) — foreign keys, CHECK constraints, uniqueness, append-only tables, actor columns.
- [Deletion and retention](../../.agents/skills/orcon-db/references/deletion-and-retention.md) — hard delete with audit, flow recovery, retention, personal data.
- [JSON documents](../../.agents/skills/orcon-db/references/json.md) — whole documents only, format tags, no queries inside JSON.
- [Queries and transactions](../../.agents/skills/orcon-db/references/queries-and-transactions.md) — sqlc, explicit columns, bounded and indexed queries, short transactions.
- [Connections](../../.agents/skills/orcon-db/references/connections.md) — SQLite pragmas and the single writer, PostgreSQL session settings.
- [Checks](../../.agents/skills/orcon-db/references/checks.md) — every automated check, how to fix a finding, how to ask for an exception.

Go code that reaches the database also follows `.agents/skills/orcon-go/references/data-access.md`.

## Before you open a pull request

- Run `make db-check` for the fast static checks of migrations and queries.
- Run `make check`; CI runs the same command.
- Run `make test-postgres` when a change touches the schema, a query or a storage adapter. It needs Docker and runs against PostgreSQL 14 and 17.

## Adding a migration

Write the SQLite and the PostgreSQL file with the same number and name, starting from the templates in `.agents/skills/orcon-db/assets/`, together with their seed and verify files. Then run `make db-sum` to record the new files in `migrations.sum`. A migration that has been merged is never edited: installations may already have applied it, and the runner refuses to start when a recorded checksum changes. A fix is always a new migration.

## Policy lists and exceptions

The lists the checks read live in `internal/adapter/storetest/dbrules/policy/`: instance tables, append-only tables, the soft-delete table, identifiers that are not foreign keys, hot query patterns, the allowed acronyms and the abbreviations that are rejected. Adding an abbreviation to the rejected list is an ordinary change. Adding an acronym, or an exception in `exceptions.txt`, needs a pull request approved by the maintainers.

## Project tooling — mneme

Schema changes start from a specification tracked in mneme, the project's tool for the backlog, specifications, acceptance criteria and QA reports. The database rulebook is packaged as the `orcon-db` mneme skill; after changing it, run `make skill-sync` and validate it with `mneme skills lint orcon-db` and `mneme skills validate orcon-db`.

## Proposing a rule change

Open a pull request that edits the relevant file in `.agents/skills/orcon-db/references/` and runs `make skill-sync`, so the generated copies stay identical. Rule changes are reviewed like code; a change that contradicts an accepted ADR needs a new ADR first.
