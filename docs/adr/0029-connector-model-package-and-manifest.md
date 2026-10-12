<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0029 — Connector model, package format, manifest and operation safety contract

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-003.

## Context
BL-003 decision K1 defines three connector kinds: declarative HTTP connectors executed natively in Go (the default), official native Go connectors for complex cases, and third-party code connectors in a WebAssembly sandbox. Running everything in JavaScript like n8n was rejected for performance and isolation. ADR-0005 (Accepted policy) left the machine-readable external-operation contract schema open; ADR-0012 (Proposed) defined the minimum runtime contract and deferred the full schema to the connector SDK ADR. ADR-0020 (Proposed) defined the trigger hooks per family and deferred packaging and sandboxing. Principle 10 requires separate trust and execution policies for official connectors and third-party extensions; the implementation contract forbids untrusted code in the host process without approved isolation. ADR-0073 (Proposed) defines human-attention notifications for suspended executions. ADR-0079 (Proposed) fixes connector authoring conventions that amend this ADR (§2, §4, §5). ADR-0080 (Proposed) fixes UPPER_SNAKE for every enumerated value that crosses a public interface (§10).

## Decision

### 1. Kinds
- `DECLARATIVE`: a data-only description of an HTTP API (ADR-0030), interpreted by the host. No embedded code; templates use the expression language of ADR-0018.
- `NATIVE`: Go code compiled into the `orcon` binary (ADR-0031). Official connectors only.
- `WASM`: a WebAssembly module executed by wazero under ADR-0032. The only kind third parties may ship as code.
The same catalog also distributes two auxiliary package kinds: `LIBRARY` (Code node libraries, ADR-0036 §6) and `RUNTIME` (Code node language runtimes, e.g. CPython, ADR-0036 §4).

### 2. Package
- File `<id>-<version>.orconpkg`: a tar archive compressed with zstd. Entries are sorted, owners/modes normalised, mtimes zero, so the same content always yields the same digest. `digest = sha256(archive bytes)`.
- Layout: `orcon-package.json` (manifest), `connector.json` (declarative body) or `module.wasm` (wasm body), `i18n/en.json` and `i18n/es.json` (message catalogs keyed by stable paths such as `node.<node>.displayName`, `operation.<resource>.<operation>.description`, `parameter.<node>.<path>.description`; ADR-0079 §2), `icons/*`, `README.md`, `LICENSE`, `NOTICE`, optional `ATTRIBUTION.json` (ADR-0035), `authoring.json` (`orcon/connector-authoring@1`: kind justification, verb exceptions, reasons for raw credential access or non-HTTPS networking; ADR-0079 §2), `changes/<from>..<to>.json` (ADR-0034), `tests/*.json` (recorded tests in `orcon/cassette@1`, ADR-0030 §10), optional `src/` reference for review.
- Official connectors, declarative or native, live in `internal/adapter/connectors/<name>/` with the folder layout of ADR-0079 §1; declarative ones are embedded with `embed.FS`, native ones are compiled in; both register in `cmd/orcon` and carry their manifest in the binary without an archive.
- Installed packages are stored as blobs (ADR-0016) with owner `system`; workers fetch by digest, verify SHA-256, and cache locally.

### 3. Identity
- `id`: `<publisher>.<name>` (`[a-z0-9-]` segments). `orcon.*` is reserved for official connectors; `community.*` is used by converter output (ADR-0035).
- `version`: SemVer 2.0, classified as amended by ADR-0079 §6 (ADR-0034 §5).
- Node type in flows: `<id>.<node>@<major>` (ADR-0007 §3). Each node instance also pins `connector { version, digest }` (ADR-0034).

### 4. Manifest `orcon/connector@1`
Normative JSON Schema: `internal/connector/schema/orcon-connector-1.schema.json`. Top-level fields:
`format`, `id`, `version`, `kind`, `displayName`, `description`, `publisher { name, url, contact }`, `license` (SPDX expression), `source { repository, commit, buildRecipe? }`, `compat { engineApi: semverRange, wasmAbi?: int }`, `nodes[]`, `credentials[]`, `permissions`, `rateLimits[]`, `n8n?`, `ai?`, `changelog`.
`displayName`, `description` and every user-facing string keep the English text as the default; translations live in the `i18n` catalogs (§2). `changelog` is `[{ version, date, notes: [messageKey] }]`, one entry per released version, with notes in English and Spanish through the catalogs (ADR-0079 §2).

`nodes[]`: `name`, `type`, `displayName`, `group` (`ACTION` | `TRIGGER`), `operations[]` (for actions) or `trigger` (ADR-0020 §3 declarations: `family`, `delivery`, `dedupeKeySource`, `maxPayload`), `parameters` (parameter schema, ADR-0030 §3), `outputs` (named ports), `sensitiveFields` (JSON paths in inputs and outputs), `n8n.types[]` (`{ name, typeVersions: [number], parameterMap? }`).

`operations[]`: `name` (`resource.verb`, with a verb from the closed vocabulary of ADR-0079: `create`, `get`, `getMany`, `update`, `upsert`, `delete`, `send`, `search`, `add`, `remove`, `archive`, `unarchive`, `upload`, `download`, `copy`, `move`, `cancel`; any other verb needs a reasoned `verbExceptions[]` entry in `authoring.json`), `displayName`, `purity` (`PURE` | `READ` | `EFFECT` | `INTERNAL_EFFECT`, ADR-0008 §5), `itemMode` (`PER_ITEM` | `BATCH`), `safety` (§5, mandatory when `purity = EFFECT`), `rateLimit?` (reference), `retry?` (classification overrides only).

`credentials[]`: `id`, `displayName`, `fields[]` (`name`, `type`, `sensitive`, `required`), `scheme` (ADR-0030 §4), `test?` (an operation used to test the credential), `n8nTypes[]`.

`permissions`: `network { domains: [pattern], ports?: [int], schemes?: ["https"|"http"], allowPrivateNetworks: bool (default false), fromCredential?: [credentialField] }`, `credentials: [credentialId]`, `credentialRawAccess: bool` (default false), `resources { memoryMiB?, timeout?, maxConcurrent? }`. Domain patterns: exact host or a single leading `*.` label wildcard. URL schemes keep their standard spelling. `fromCredential` allows self-hosted services whose host is a credential field; the concrete host is approved when the credential is created (BL-004 hook) and checked by the egress client.

### 5. Operation safety contract (machine-readable ADR-0005 schema)
```
safety: {
  effect: "READ" | "WRITE" | "DELETE" | "UNKNOWN",
  level: "READ_ONLY" | "IDEMPOTENCY_KEY" | "RECONCILE" | "UNSAFE",
  idempotency?: { mode: "PROVIDER_KEY" | "NATURAL_KEY" | "NONE",
                  location?: { in: "HEADER" | "QUERY" | "BODY", name | path },
                  ttl?: duration, scope?: "CREDENTIAL" | "ACCOUNT" | "GLOBAL" },
  verification?: { operation: opName, match: expr, notFoundMeansFailed: bool,
                   consistency: "STRONG" | { eventualDelay: duration } },
  onUncertain: "RESEND_WITH_KEY" | "RECONCILE" | "SUSPEND",
  retrySafe: bool,
  compensation?: { operation: opName, precondition?: expr, verify?: opName },
  batch?: { maxItems: int, perItemResults: bool },
  manualCheck?: { description: template, url?: template },
  proofLevel: "DECLARED" | "TESTED" | "VERIFIED"
}
```
Rules (validated at install and at publication, ADR-0007 §9 level 5): `UNKNOWN` is treated as `WRITE`; `READ_ONLY` only with `effect = READ`; `IDEMPOTENCY_KEY` requires `idempotency.mode = PROVIDER_KEY | NATURAL_KEY`, `location` and `ttl`; `RECONCILE` requires `verification.operation` whose purity is `READ`; `UNSAFE` requires `retrySafe = false` and `onUncertain = SUSPEND`; `RESEND_WITH_KEY` requires `IDEMPOTENCY_KEY`. Mapping to ADR-0012 §1: `effect` → `effect`; `idempotency` → `idempotency`; `verification.operation` present → `verification.reconcile = true`; `onUncertain`, `retrySafe`, `compensation` unchanged. `proofLevel` is set by the publisher (`DECLARED`), raised to `TESTED` when the package's fixtures pass the conformance kit (ADR-0031 §5), and to `VERIFIED` only by Orcon review (ADR-0033); the editor displays it.

**Ambiguous outcomes (ADR-0079 §4, maintainer decision of 2026-10-12).** For a write without an idempotency key (`RECONCILE` or `UNSAFE`), a 5xx response, a timeout or a connection failure after the request was sent is an ambiguous outcome: `RECONCILE` asks the provider before deciding, `UNSAFE` becomes UNCERTAIN and asks a person, and neither is resent blindly. A 429 response means the request was not applied and is transient for every level.

**`UNSAFE` semantics (owner decision, SPEC-003 Q11).** `UNSAFE` is a valid, dispatchable contract. The operation is dispatched at most once per correlation key and is never retried after dispatch — neither by automatic retries nor by "re-run from failure" (ADR-0011). On an ambiguous outcome the item becomes `UNCERTAIN` and the dependent token is suspended (ADR-0012). In the same transaction that records `UNCERTAIN`, the engine emits the ADR-0073 human-attention event `execution.awaiting_human` through the outbox (ADR-0013 §8), so that it is delivered immediately on every configured channel. The payload contributed by this ADR is: flow id, name and version; execution id; node and step; connector id and version; operation; affected item and correlation key; a summary of what was sent (method, target, body) masked per ADR-0017 and included only for recipients holding execution-data permission (otherwise `dataHidden: true`); the resolved `manualCheck` text and link when declared; and the available actions, which are the ADR-0012 supervised resolutions: "it happened, continue" (`RESOLVED_SUCCEEDED`), "it did not happen, repeat" (`RESOLVED_FAILED` followed by a new dispatch with a new correlation key, requiring the resolver to confirm they checked the provider), "it did not happen, treat as error" (`RESOLVED_FAILED`, routed by the node's error rules), and "cancel the execution". Escalation when unattended (`execution.awaiting_human.escalated`), resolution (`execution.human_resolved`), channels, timeouts, recipients and the time-to-human-action metric are defined by ADR-0073, not here. Imported write operations without an inferable contract receive `UNSAFE` and must be reviewed before the flow can be activated (ADR-0037 §6).

### 6. Credentials contract (boundary with the security design)
- Connectors never receive credential values by default. The host passes a `CredentialHandle`; the egress client applies the declared scheme to outgoing requests (headers, query, signing, OAuth bearer).
- `credentialRawAccess` (approved by the admin at install) enables `Field(name)` for connectors that must compute signatures or speak non-HTTP protocols. Native connectors use the same handle API.
- Values are never serialized into items, events, logs or history (ADR-0008 §4, ADR-0017). On a 401 from a request using an OAuth credential, the host may refresh once and retry the request if the operation is `READ` or `retrySafe`.
- Storage, encryption, key management, OAuth consent and refresh, sharing and roles are defined by the security design (BL-004). Until then, "admin" means the instance owner via SPEC-001's minimal authorization hook.

### 7. Role placement
Actions execute in `worker`. Trigger `Verify/Handshake/Decode` run in `api`; poll, listen and registration run in `worker` (ADR-0020 §6). No new roles.

### 8. Egress client
All connector network I/O, for every kind, goes through `internal/adapter/egress`:
- Host must match a granted domain pattern (or an approved credential-derived host); scheme HTTPS unless `http` is declared and granted.
- DNS is resolved by the host; every resolved address is checked; loopback, RFC 1918, RFC 6598, link-local, unique-local IPv6, IPv4-mapped IPv6 forms of those, and cloud metadata addresses are refused unless `allowPrivateNetworks` is granted; the connection is made to the checked address (no second resolution).
- Redirects are followed only within granted domains and re-checked at every hop (max 10).
- Response size cap (default 64 MiB unless streamed to a blob), connect/read timeouts, optional instance HTTP proxy.
- Raw TCP (`Dial`) is available to native connectors only, with the same host checks.

Logical schema additions (ADR-0015 as amended by ADR-0077: `id` primary keys holding application-generated UUIDv7 values, `created_at` and `updated_at` unless append-only, names without abbreviations, enumerated columns as text with a CHECK over the UPPER_SNAKE values of §10, documents as `<name>_document` with `<name>_document_format` stored as `bytea`/`BLOB`, every SQLite table STRICT, no views, DDL valid on PostgreSQL 14):
- Instance tables (no `org_id`; listed in the instance-table policy): `connector_package(id, connector_id, version, digest, kind, manifest_document, seal, catalog_source_id, status, installed_at, installed_by_principal_id)` unique `(connector_id, version)`; `catalog_source(id, name, kind, location, root_metadata_document, policy_flag_document)`; `catalog_metadata(id, catalog_source_id, role, version, expires_at, document)`; `connector_revocation(id, connector_id, version, severity, reason, advisory_url, fixed_in_version, observed_at)`; `connector_revocation_override(id, connector_id, version, created_by_principal_id, reason, expires_at)`; `code_library(id, language, name, version, digest, seal, status)`; `host_node_policy` (ADR-0037 §7).
- Organization-scoped tables (`org_id` NOT NULL, composite foreign keys to organization tables, single-column foreign keys to instance tables): `connector_grant(org_id, id, connector_package_id, project_id, permissions_digest, granted_by_principal_id, granted_at)`; `connector_reference(org_id, id, flow_id, flow_version_id, connector_id, connector_version)` (ADR-0034 §6).
No query reads inside these documents (Go rulebook DA-3 as amended; ADR-0077 §9); anything filtered is a real column.

### 9. AI hook points (for the AI design)
Reserved, not specified here: `ai.tool` per operation (`exposable`, `description`, `inputSchemaFrom: parameters`) so any operation can become an agent tool; reserved `kind` values `MODEL`, `MEMORY`, `VECTOR_STORE`, `EMBEDDING`. The AI design (BL-006) defines their semantics.

### 10. Value casing
Every enumerated or state value that crosses a public interface is UPPER_SNAKE (ADR-0080 §3), with one spelling in the API, public events, the database (text with a CHECK) and every stored or exchanged document format of this ADR set: the manifest (§4, §5), the declarative body (ADR-0030), `authoring.json`, catalog metadata and the trust configuration (ADR-0033), Code node parameters (ADR-0036) and the error class of the WASM ABI (ADR-0032). Exempt: recorded tests in `orcon/cassette@1` (internal test artifacts); identifiers — node types, operation names and verbs, error codes, change-descriptor operations, ABI `op` names, TUF role and delegation names; and values fixed by external standards — HTTP methods, URL schemes, HMAC algorithm names.

## Consequences
One manifest drives editor, importer, catalog, validation and runtime checks for every kind. The safety contract is a public, machine-checkable schema. Network access is uniformly constrained, including for official code. Ambiguous `UNSAFE` writes never duplicate silently and always reach a person quickly through the shared human-attention mechanism. Every connector ships English and Spanish text and a per-version changelog. A value reads the same in the manifest, the API and the database.

## Alternatives considered
- **Everything in JavaScript/TypeScript (n8n model)**: rejected by K1 (performance, isolation, second runtime).
- **Per-kind manifests**: rejected — duplicates validation and UI logic.
- **Connectors receive raw secrets by default**: rejected — widens the blast radius of a malicious or buggy connector.
- **`UNSAFE` operations not dispatched automatically**: rejected (SPEC-003 Q11) — almost no imported flow with a POST would run without intervention.
- **A connector-specific notification mechanism for uncertain writes**: rejected — ADR-0073 is the single human-attention mechanism.
- **Treating a 5xx on a write as a definite failure**: rejected (ADR-0079 §4) — a write applied before the error could be repeated by hand and duplicated without warning.
- **Lowercase values in document formats and UPPER_SNAKE only in the API**: rejected — the API returns documents verbatim, so two spellings of the same value would cross the same interface.
