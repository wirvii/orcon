<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
## Database checklist

<!-- Paste this block into the description of every pull request that changes a table, column, index, constraint, row-level security policy, engine function, migration, seed or verify file, SQL query, store method or a file in internal/adapter/storetest/dbrules/policy/. -->

- [ ] The change derives from an accepted ADR or an approved spec (cite it: `SPEC-…`, `BL-…`, `docs/adr/…`), and I loaded the `orcon-db` skill and followed its Workflow.
- [ ] 1. Names: snake_case, singular tables, primary key `id`, foreign keys `<table>_id`, instants `_at`, booleans `is_`/`has_`, no reserved words, no abbreviations except `id`, `org` and the acronyms in `policy/allowed_acronyms.txt` (NM, NM-3).
- [ ] 2. Every constraint and index is named: `pk_`, `fk_`, `uq_`, `ck_`, `ix_` (NM-11).
- [ ] 3. Ids are UUIDv7 generated in Go and passed as parameters; never a database default (ID-1).
- [ ] 4. Types follow the mapping; every SQLite table is STRICT; instants are UTC, `timestamptz` or unix microseconds; the application supplies times (TY).
- [ ] 5. Migrations come in pairs with the same number and name, carry the header, are forward-only and were not edited after merge; breaking changes use expand now and contract in a later release (MG).
- [ ] 6. Every migration ships seed and verify files for both dialects, with at least two organizations (MG-11).
- [ ] 7. Every organization-scoped table has `org_id`, composite foreign keys, and on PostgreSQL enabled and forced row-level security with the `org_isolation` policy and the `orcon_application` grants; anything else is listed as an instance table (TI).
- [ ] 8. Context is transaction-local `set_config('orcon.org_id', …, true)`; queries are parameterized; no views; cross-organization work only through `orcon_engine` functions (TI).
- [ ] 9. Every reference is a foreign key with explicit `ON DELETE` and an index; enumerations are TEXT with a CHECK that lists UPPER_SNAKE values, never ENUM types; NOT NULL and UNIQUE where they apply; who-did-it columns are `<verb>_by_principal_id` with `ON DELETE SET NULL` (IN, IN-12).
- [ ] 10. Deletes are hard and audited in the same transaction; only `flow` has soft delete, always filtered; accumulating data leaves through retention (DL).
- [ ] 11. JSON only for whole documents, `bytea`/`BLOB` with a `_document_format` tag; anything filtered or joined is a real column; no JSON paths and no `jsonb` without an approved exception; enumerated values inside documents the API returns verbatim are UPPER_SNAKE (JS, NM-16).
- [ ] 12. Queries: explicit columns, `-- orcon:scope` on every query, `LIMIT` on every multi-row query, keyset pagination, no `OFFSET`, hot paths tagged and indexed, same query names in both dialects (QY).
- [ ] 13. Transactions are short and never wrap a network or connector call (QY-8).
- [ ] 14. SQLite connections set `foreign_keys`, `busy_timeout` and `trusted_schema`; one writer in WAL; read-only readers (CN).
- [ ] 15. The `orcon-db` skill was edited only under `.agents/skills/orcon-db/` and `make skill-sync` was run; `mneme skills pin` was not run on it.
- [ ] New migrations are recorded with `make db-sum`; no line already in `migrations.sum` on `main` changed.
- [ ] New store methods have their isolation battery case and contract test case.
- [ ] Any new entry in `policy/exceptions.txt` or `policy/allowed_acronyms.txt` has owner approval.

### Commands run

<!-- Paste each command with its result: sh .agents/skills/orcon-db/scripts/db-change-check.sh, make check, make test-postgres (PostgreSQL 14 and 17), and anything else you ran. State known limitations. -->

| Command | Result |
|---|---|
| `make check` | |
| `make test-postgres` | |
