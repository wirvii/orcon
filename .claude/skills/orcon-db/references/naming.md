<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Naming

Status: Accepted

This document defines how Orcon names database objects (tables, columns, constraints, indexes, functions, schemas, roles, queries and migration files) and how values that cross a public interface are spelled. Rule IDs (`NM-…`) are stable. `policy/` means `internal/adapter/storetest/dbrules/policy/`.

Precedence: accepted ADRs > accepted Go rulebook rules (`orcon-go` DA-1..DA-8) > this rulebook > reviewer preference.

## Rules

- **NM-1** Every identifier Orcon defines (table, column, index, constraint, function, schema, database role, query file, migration file) is snake_case matching `^[a-z][a-z0-9]*(_[a-z0-9]+)*$` and at most 63 bytes (PostgreSQL truncates longer names silently). Names defined by the engines (pragmas, catalog tables, runtime parameters) are outside this rulebook.
- **NM-2** Table names are singular nouns (`execution`, `flow_version`, `org_group_member`). A table name ending in `s` needs a line in `policy/singular_exceptions.txt`.
- **NM-3** No abbreviations. The only short forms allowed are `id`, `org` and the acronyms below. Spell words out: `reference` not `ref`, `maximum` not `max`, `sequence_number` not `seq`, `milliseconds` not `ms`.
  - **Allowed list.** `policy/allowed_acronyms.txt` holds exactly these 25 tokens at birth, one per line as `<token>  # <expansion>; <standard or product>`, in alphabetical order:

    | Token | Expansion |
    |---|---|
    | `ai` | artificial intelligence |
    | `api` | application programming interface |
    | `dns` | Domain Name System (RFC 1035) |
    | `fts5` | SQLite FTS5 full-text search module |
    | `hmac` | keyed-hash message authentication code (RFC 2104) |
    | `http` | Hypertext Transfer Protocol (RFC 9110) |
    | `id` | identifier (permitted abbreviation, Q1) |
    | `ip` | Internet Protocol |
    | `kms` | key management service |
    | `ldap` | Lightweight Directory Access Protocol (RFC 4511) |
    | `mac` | message authentication code |
    | `mcp` | Model Context Protocol |
    | `n8n` | the n8n product (compatibility only) |
    | `oauth` | OAuth authorization framework (RFC 6749) |
    | `oidc` | OpenID Connect |
    | `org` | organization (permitted abbreviation, Q1) |
    | `saml` | Security Assertion Markup Language |
    | `scim` | System for Cross-domain Identity Management (RFC 7643) |
    | `sha256` | SHA-256 hash (FIPS 180-4) |
    | `siem` | security information and event management |
    | `smtp` | Simple Mail Transfer Protocol (RFC 5321) |
    | `tls` | Transport Layer Security |
    | `uri` | Uniform Resource Identifier (RFC 3986) |
    | `url` | Uniform Resource Locator |
    | `uuid` | universally unique identifier (RFC 9562) |

    An entry names a standard, protocol, algorithm or product, or is one of the two permitted abbreviations. Adding an entry requires a pull request approved by the maintainers; removing one is an ordinary change.
  - **Denied list.** `policy/abbreviation_denylist.txt` is the checked definition of an abbreviation. Each line is `<token>  # <full word(s)>`, alphabetical. Initial content: `ack` (acknowledgement), `addr` (address), `admin` (administrator), `app` (application), `attr` (attribute), `auth` (authentication or authorization), `avg` (average), `cfg` (configuration), `ck` (check; constraint prefix only), `cnt` (count), `compat` (compatibility), `conf` (configuration), `config` (configuration), `ctx` (context), `cur` (current), `curr` (current), `dedupe` (deduplication), `desc` (description), `doc` (document), `docs` (documents), `dst` (destination), `dt` (date), `env` (environment), `err` (error), `exec` (execution), `fk` (foreign key; constraint prefix only), `fts` (full-text search; the SQLite module is `fts5`), `idx` (index), `info` (information), `ix` (index; index prefix only), `max` (maximum), `mi` (multi-instance), `micros` (microseconds or microdollars), `min` (minimum), `ms` (milliseconds), `msg` (message), `no` (number), `ns` (nanoseconds), `num` (number), `op` (operation), `ops` (operations), `param` (parameter), `params` (parameters), `pct` (percent), `pk` (primary key; constraint prefix only), `prev` (previous), `pwd` (password), `qty` (quantity), `ref` (reference), `refs` (references), `repo` (repository), `req` (request), `res` (result or response), `resp` (response), `seq` (sequence number), `spec` (specification), `src` (source), `temp` (temporary), `tm` (time), `tmp` (temporary), `ts` (timestamp; use `_at`), `ttl` (time to live), `tz` (time zone), `uid` (identifier; use `id`), `uq` (unique; constraint prefix only), `us` (microseconds), `val` (value), `ver` (version). Adding a token tightens the rule and needs only ordinary review.
  - **Algorithm** (identical in `tools/dbcheck` and `dbrules`; both load the two files, neither holds its own list):
    1. Split the name into tokens: snake_case names on `_`; PascalCase query names before an uppercase letter that follows a lowercase letter or digit, and before the last uppercase letter of an uppercase run that is followed by a lowercase letter (`GetAPIKeyByHash` → `get`, `api`, `key`, `by`, `hash`); migration files use the description after `NNNN_`. Lowercase every token.
    2. A token is accepted if it is in the allowed list; or consists only of digits; or is the first token of a constraint or index name and equals the prefix NM-11 requires for that object (`pk`, `fk`, `uq`, `ck`, `ix`).
    3. Otherwise a token in the denied list is a finding: `NM-3 abbreviation "<token>" in <object> (fix: <full word(s)> from the denied list)`.
    4. Any other token is accepted. The denied list is the operational definition; a reviewer who finds an abbreviation the checker missed adds it to the denied list in the same change.
    5. The policy check fails if a token appears in both lists, a line lacks its `# ` expansion, a token is not `^[a-z][a-z0-9]*$`, a token is duplicated, or the list is not in alphabetical order.
  - Units are spelled out: `_milliseconds`, `_microseconds`, `_seconds`, `_bytes`, `_microdollars`.
- **NM-4** The primary key is the single column `id`. The only exception is `schema_migration.version`.
- **NM-5** A foreign key column is named `<referenced_table>_id`, with a qualifier prefix when a table references the same table twice or the role matters: `parent_execution_id`, `root_execution_id`, `target_flow_id`, `retried_execution_id`. Actor columns are `<verb>_by_principal_id` (`published_by_principal_id`, `resolved_by_principal_id`, `deleted_by_principal_id`; IN-12); no other column name ends in `_by` or contains `_by_`. A column ending in `_id` that is not a foreign key (identifiers inside a stored document such as `node_id`, `edge_id`; identifiers issued by external systems such as `provider_request_id`; audit actor identities that must outlive the principal such as `audit_event.actor_principal_id`) is listed with its reason in `policy/external_ids.txt`.
- **NM-6** Instants end in `_at` and use a past participle or deadline form (`created_at`, `published_at`, `expires_at`, `deferred_until_at`). Every `_at` column is a timestamp and every timestamp column ends in `_at`. Durations end in their unit (`active_milliseconds`).
- **NM-7** Booleans start with `is_` or `has_`. When the moment matters, a nullable `_at` column replaces the boolean (`cancel_requested_at`, `committed_at`).
- **NM-8** Enumerated columns are named `status`, `kind`, `mode`, `state`, `level`, `role`, or end with one of those words; each has a CHECK constraint (IN-3). The checkers read these words, plus `format`, from `policy/enumeration_words.txt`.
- **NM-9** JSON document columns are named `document` or `<name>_document`; the format tag column is `<name>_document_format` (JS-3).
- **NM-10** Counters end in `_count`; money is an integer in the smallest unit with the unit in the name (`cost_microdollars`); hashes name the algorithm or purpose (`sha256`, `content_hash`, `token_hash`).
- **NM-11** Every constraint and index is named explicitly; engine-generated names are rejected:
  - `pk_<table>`;
  - `fk_<table>_<referenced_table>` or `fk_<table>_<qualifier>_<referenced_table>` when qualified;
  - `uq_<table>_<columns>` for unique constraints and unique (including partial) indexes;
  - `ck_<table>_<rule>` (`ck_execution_status`, `ck_token_id_length`, `ck_execution_revision`);
  - `ix_<table>_<columns>` for other indexes.
  `<columns>` are the column names in index order joined by `_`. When that exceeds 63 bytes, use a short snake_case purpose (`ix_token_claim_order`) and state the columns in a comment above the statement.
- **NM-12** No identifier equals a PostgreSQL reserved word or an SQLite keyword (`policy/reserved_words.txt`): for example `user` → `application_user`, `key` → `data_key`, `references` → `reference_document`, `order`, `group`, `values`, `table`.
- **NM-13** Tables live in schema `public`; functions only in schemas `orcon_engine` (engine plane, ADR-0047) and `orcon_context` (context helpers, ADR-0046 as amended by ADR-0077). Function names are `<verb>_<object>` (`claim_tokens`, `resolve_api_key`, `current_org_ids`). Database roles are `orcon_owner`, `orcon_application`, `orcon_engine`.
- **NM-14** sqlc query names are PascalCase, starting with a verb from: `Get`, `List`, `Count`, `Exists`, `Create`, `Insert`, `Update`, `Upsert`, `Delete`, `Purge`, `Restore`, `Claim`, `Renew`, `Release`, `Resolve`, `Mark`, `Lock`, `Append` (the checkers read the same list from `policy/query_verbs.txt`; both change together). The words of the name follow NM-3. The same name exists in both dialect query sets (QY-1).
- **NM-15** Migration files are `NNNN_<snake_description>.sql`, four digits, contiguous from `0001`, identical name in both dialect directories; the description follows NM-3.
- **NM-16** Value spelling (ADR-0077 decision 11). Identifiers follow NM-1 to NM-15; values do not. Every enum and state value that crosses a public interface — API requests and responses, event payloads, and documents the API returns verbatim such as `orcon/flow@1` flow documents and connector manifests — is UPPER_SNAKE (`RUNNING`, `SERVICE_ACCOUNT`), as ADR-0080 §3 states for the API; documents stored whole therefore hold UPPER_SNAKE values (JS-8). `principal.kind` stores the API spelling: `CHECK (kind IN ('USER', 'SERVICE_ACCOUNT', 'SYSTEM'))` (IN-12). Every other enumerated column stores its UPPER_SNAKE value too (ADR-0015 §2): the CHECK lists exactly the values the API exposes, and no adapter translates an enumerated value between the database and the API. `tools/dbcheck` and `dbrules` reject any other spelling in an enumeration CHECK (CK-24).
