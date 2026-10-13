<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Identifiers

Status: Accepted

This document defines row identity, its storage and text form, secrets, sequence numbers and natural keys. Rule IDs (`ID-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

## Rules

- **ID-1** Row identity is a UUIDv7 (RFC 9562) generated in Go by `internal/platform/id` and passed as a query parameter. No column default, `gen_random_uuid()`, `uuidv7()`, sequence, `AUTOINCREMENT` or rowid creates an identity.
- **ID-2** Storage: PostgreSQL `uuid`; SQLite `BLOB` in a STRICT table with `CONSTRAINT ck_<table>_id_length CHECK (length(id) = 16)` (and the same check on every `*_id` column). The 16 bytes use RFC 9562 network byte order, so byte order equals time order on both engines.
- **ID-3** The text form is the lowercase hyphenated 36-character form, used in the API, logs, events, metrics labels and file names. Conversion happens only in adapters.
- **ID-4** Tests inject identities through the `id.Generator` port (deterministic sequences); no test depends on the clock embedded in an id.
- **ID-5** An id is not a secret and never authorizes by possession. Bearer secrets (API keys, OAuth tokens, wait resume URLs, random webhook path suffixes) are separate random values of at least 128 bits, stored only as hashes (`token_hash`, `secret_hash`) and looked up through engine functions (TI-14).
- **ID-6** Ordering within a parent uses a non-key `sequence_number` column (PostgreSQL `bigint`, SQLite `INTEGER` assigned by the single writer) with a UNIQUE constraint (for example `execution_event`: unique `(org_id, execution_id, sequence_number)`). PostgreSQL identity values are not commit-ordered; a consumer that pages by sequence number follows the visibility rule of the spec that owns it.
- **ID-7** Natural keys (`(flow_id, version_number)`, `content_hash`, `correlation_key`, `deduplication_key`) are UNIQUE constraints that start with `org_id` on organization-scoped tables, never primary keys.
- **ID-8** An organization-scoped table that other tables reference declares `CONSTRAINT uq_<table>_org_id_id UNIQUE (org_id, id)` as the target of composite foreign keys.
