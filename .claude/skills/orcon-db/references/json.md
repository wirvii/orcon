<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# JSON documents

Status: Accepted

This document defines how JSON documents are stored and what SQL may do with them. Rule IDs (`JS-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

## Rules

- **JS-1** JSON columns hold only documents stored and read whole: the flow document, node configuration, item payloads and outputs, error and summary documents, settings.
- **JS-2** Anything filtered, sorted, joined, counted, constrained or referenced is a real column, written in the same statement or transaction as the document, even when duplicated inside it.
- **JS-3** Every JSON column `<name>_document` has `<name>_document_format text NOT NULL` (NULL together with a NULL document) with a CHECK over the known tags `orcon/<kind>@<major>` (for example `orcon/flow@1`). Readers refuse an unknown major version; a new major version goes through expand/contract.
- **JS-4** Storage: PostgreSQL `bytea` by default, `jsonb` only for a column with an approved JSON-path exception; SQLite `BLOB`. The bytes are UTF-8 JSON written by `internal/platform/jsonvalue` (RFC 8785 canonical form where the document is hashed). Go rulebook rule DA-3, as amended, states the same.
- **JS-5** JSON-path operators and JSON functions are forbidden in SQL (PostgreSQL `->`, `->>`, `#>`, `#>>`, `#-`, `@?`, `?`, `?|`, `?&`, `json_*`, `jsonb_*`; SQLite `->`, `->>`, `json_*`, `jsonb_*`) unless `policy/exceptions.txt` holds an owner-approved line for that query. Go rulebook rule DA-3, as amended, states the same.
- **JS-6** A document above 256 KiB is stored as a blob and referenced (ADR-0016).
- **JS-7** Secret values are never stored in clear inside documents (credential fields are encrypted, SPEC-004).
- **JS-8** Documents the API returns verbatim (`orcon/flow@1` flow documents, connector manifests and any other format a spec declares public) spell their enum and state values in UPPER_SNAKE (NM-16, ADR-0080 §3); a document is stored exactly as it is returned, so no value is translated on the way out. Changing the spelling of a value inside an existing format major is a breaking change of that format (JS-3).
