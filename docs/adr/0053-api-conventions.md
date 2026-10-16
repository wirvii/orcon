<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0053 — API conventions: pagination, filtering, errors, idempotency, concurrency, rate limits, long-running operations and response shapes

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: owner answers Q1–Q14, human-attention surface; ADR-0080 error catalog and response shapes; ADR-0077 schema rules) (amended 2026-10-13: stored enumerated values are UPPER_SNAKE, ADR-0015 §2) (amended 2026-10-14: the idempotent response body is stored in a column with its format) (amended 2026-10-14: error code `unsupported-media-type`)

Accepted by the owner on 2026-10-12 via SPEC-005.

## Context
ADR-0052 defines one public API used by the editor, CLI, SDKs and MCP server. Uniform conventions let generated clients offer the same helpers and make retries and concurrent edits (editor + API + Git) safe.

## Decision

### 1. Pagination
Cursor-based. Request: `limit` (default 50, max 200), `pageToken`. Response: exactly `{ "items": [...], "nextPageToken": "..." }` (`nextPageToken` absent on the last page). Tokens are opaque, MAC-protected with a key derived from the instance key (ADR-0042) and bind the organization, filters and sort; reuse with different parameters → `400 invalid-page-token`. Ordering is always total (sort key + id). `includeTotal=true` only where declared (`x-orcon-include-total`), capped (above 10,000 → `"totalIsLowerBound": true`).

### 2. Filtering and sorting
Typed filter parameters per list operation (e.g. `status`, `flowId`, `createdAfter`, `createdBefore`, `tag`, `q` for name prefix search). No generic filter language in v1. `sort=field,-field2` from a per-operation allowlist (`enum`), with the default named by `x-orcon-sort-default`.

### 3. Errors
`application/problem+json` (RFC 9457):
`{ "type": "urn:orcon:problem:<code>", "title", "status", "detail", "instance", "code", "requestId", "errors": [ { "pointer": "/nodes/3/parameters/url", "code", "message" } ] }`.
The code catalog is part of the contract (`components/schemas/ProblemCode` with `x-orcon-problem-catalog`; generated page `docs/reference/problem-codes.md`); each entry has a code, an HTTP status, a title, when it applies and whether it is retryable: `validation-failed` 400, `invalid-page-token` 400, `experimental-not-enabled` 400, `unauthenticated` 401, `forbidden` 403, `step-up-required` 403 (ADR-0038 §6 policy challenge), `personal-keys-disabled` 403, `not-found` 404, `method-not-allowed` 405 (answered by the router, with `Allow`, when the path exists in the contract and the method is not one of its operations), `conflict` 409, `idempotency-in-progress` 409, `secret-already-issued` 409, `attention-already-resolved` 409, `project-managed-by-git` 409, `deprecated-removed` 410, `precondition-failed` 412, `payload-too-large` 413, `unsupported-media-type` 415 (the media type of the request body is not one the operation declares), `idempotency-key-reused` 422, `precondition-required` 428, `rate-limited` 429, `internal-error` 500, `service-unavailable` 503; `confirmation-required` 403 (the request was stored as a confirmation a person must approve, ADR-0059 §5; extension members `confirmationId` and `expiresAt`; not retryable), `federated-token-rejected` 400 (ADR-0056 §9; it belongs to the protocol endpoint `oauth/token` and is sent in the RFC 6749 §5.2 format as `invalid_grant`). Every operation lists in `x-orcon-problems` every code it can return, including the minimum set for its shape (authenticated operations `unauthenticated`, `rate-limited`, `internal-error`; with a permission other than `authenticated` `forbidden`; with an id in the path `not-found`; with parameters or body `validation-failed`; with a body `payload-too-large`; with idempotency `idempotency-key-reused`, `idempotency-in-progress`; with `If-Match` `precondition-failed`, `precondition-required`; lists `invalid-page-token`; experimental `experimental-not-enabled`). Codes are stable kebab-case identifiers; a new code is added only for a new condition. Messages never contain secret values or data from another organization. Flow validation errors reuse ADR-0007 §9 levels with JSON Pointers. The field-error text member is `message`.

`test-input-missing` 422 (`errors[]` lists the nodes without seed data), `effect-preview-closed` 409, `pin-limit-exceeded` 409, `fragment-too-large` 422, `interactive-session-required` 403.

### 4. Idempotency
- `Idempotency-Key` (1–255 visible ASCII) accepted on every `POST`; required on: running a flow, creating/rotating API keys, creating OAuth clients, creating event subscriptions, redelivering webhooks, starting imports and Git syncs, the continue/repeat/cancel human-attention actions (ADR-0052 §6), and every long-running operation.
- `idempotency_record(org_id, id, requested_by_principal_id?, requester_identity, idempotency_key, fingerprint, status: IN_PROGRESS|COMPLETED, response_status, response_body_document, response_body_document_format, resource_reference, created_at, expires_at)`, unique `(org_id, requester_identity, idempotency_key)`, TTL 24 h. `requester_identity` is the exact requesting principal (a user, a service account, or the API key or OAuth token for key-authenticated calls), so two keys of the same user do not share records; `requested_by_principal_id` is the actor column of ADR-0077 §8. The stored response body lives in the row (`response_body_document`, with its format in `response_body_document_format`), up to 1 MiB; a larger body is not stored and a replay is rebuilt from `resource_reference`. Fingerprint = SHA-256 of method, route template, path parameters and canonical body.
- Same key and fingerprint after completion → stored response replayed with `Idempotent-Replayed: true`. Different fingerprint → `422 idempotency-key-reused`. In progress → `409 idempotency-in-progress` with `Retry-After`.
- The record is written in the same transaction as the state change; handlers that start durable work store the created resource id so a replay returns it even if the first response was lost.
- Responses disclosing a newly issued secret are stored without it; a replay returns `409 secret-already-issued` with the resource id.
- A secret Orcon generates (API key, OAuth client secret, webhook signing secret) is disclosed exactly once: in the response of the operation that creates or rotates it, as the read-only property `secret` of a response schema named `<Resource>WithSecret` that extends the resource; that operation requires `Idempotency-Key`. Secrets of the credential store are never returned (ADR-0043 §2).
- `PUT` and `DELETE` are idempotent by definition.

### 5. Optimistic concurrency
- Editable resources return a strong `ETag`: drafts `"<draftRevision>"` (ADR-0007 §6); other resources a per-row `revision`.
- `PUT`, `PATCH`, `DELETE` and state-guarded actions require `If-Match`: missing → `428`; mismatch → `412` with the current ETag and, for drafts, the current document. `If-None-Match: *` creates only if absent.
- Editor, API, CLI and Git sync all write drafts through the same revision check; no last-write-wins path.
- Draft `PATCH` accepts JSON Patch (RFC 6902) including `test`; other resources accept JSON Merge Patch (RFC 7396).

### 6. Rate limits
- Token buckets per principal and class: `read`, `write`, `run`, `stream` (concurrent SSE/MCP streams); each operation declares `x-orcon-rate-class` (`GET` → `read`; `text/event-stream` and the MCP endpoint → `stream`; operations that start executions, test runs, retries, repeats, redeliveries or replays → `run`; other mutations → `write`).
- Defaults (configuration with instance floors and organization overrides, ADR-0038 §8; per-principal overrides): read 1200/min, write 300/min, run 600/min, stream 20 concurrent; interactive sessions have a separate profile (2×). A pre-authentication per-IP limit protects token and login endpoints.
- Enforced per `api` process with the configured rate divided by the number of live `api` processes (worker registry heartbeat, ADR-0015); approximate by design, documented. No external store required.
- Headers `RateLimit-Policy` and `RateLimit` (IETF httpapi draft); `429 rate-limited` with `Retry-After`.
- Independent from engine fairness and limits (ADR-0014), which still apply.

### 7. Long-running operations
- Imports, Git syncs and conflict resolution, bulk export, bulk retry, connector installation from a remote catalog and webhook replay return `202 Accepted`, `Location: /api/v1/operations/{operationId}` and the `Operation` resource; declared with `x-orcon-long-running: { kind }`.
- `api_operation(org_id, id, project_id, kind, status, progress, request_reference, result_reference, error_document?, error_document_format?, created_by_principal_id?, created_at, started_at, finished_at, lease_owner, lease_epoch, lease_expires_at, cancel_requested_at)`; `Operation.status` in the API: `PENDING → RUNNING → SUCCEEDED | FAILED | CANCELED`.
- A failed operation carries `failure` (`code` from the problem-code catalog, `title`, `detail?`), built from `error_document`; no resource names a member `error` (§9).
- Executed by `worker` (or `standalone`), claimed through an `orcon_engine` function (ADR-0047 §2) with lease and fencing (ADR-0013 §4); a crashed holder's operation is reclaimed; each kind defines idempotent steps or checkpoints.
- Readable by its creator and by principals holding the read permission of its target; completion emits `operation.finished`.

### 8. Running a flow synchronously
`POST /flows/{flowId}/executions` (permission `flow:execute`) accepts `wait` (≤ 60 s). Finished in time → `200` with the `Execution` resource, which carries its output summary (data only with `execution:read-data`, else the resource field `dataHidden: true`); otherwise `202` with the `Execution` resource. The wait uses the cross-process notification pattern of ADR-0023 §2.

### 9. Response shapes (ADR-0080 §7; owner decision 2026-10-12)
- A single resource is returned as the object itself; a list is exactly `{ items, nextPageToken? }` (plus `total`/`totalIsLowerBound` only where `x-orcon-include-total` is declared); an error is always `application/problem+json` (§3), except in `protocol`-tagged operations (ADR-0052 §1), whose errors follow their external standard (for example RFC 6749 §5.2 for the OAuth token endpoint).
- Auxiliary data travels in headers — `Orcon-Request-Id`, `ETag`, `RateLimit`, `RateLimit-Policy`, `Retry-After`, `Location`, `Idempotent-Replayed`, `Deprecation`, `Sunset`, `Link` — and is never copied into bodies.
- Status codes: `200` with the resource; `201` with the created resource and `Location`; `202` with the `Operation` resource or the execution (`wait` expired); `204` with no body.
- Forbidden at the root of a 2xx body: `data`, `meta`, `success`, `ok`, `result`, `results`, `payload`, `response`, `error`, `errors`, `pagination`, `links`, `requestId`, `etag`, and a boolean `status`. Visibility flags that describe the resource itself (`dataHidden`) are resource fields, not envelopes.
- Enforced by the contract lint rule `orcon/response-shape` and by the router/OpenAPI parity test (no 2xx body outside the declared schema).

## Consequences
Uniform helpers in all SDKs; safe retries; concurrent editing never silently loses work; clients see one response shape everywhere.

## Alternatives considered
Offset pagination (unstable under inserts); generic filter language in v1 (deferred); last-write-wins drafts (N2N lesson); Redis-backed global rate limiting as a requirement (principle 5); response envelopes with `data` and `meta` (rejected by the maintainers; they duplicate what headers already carry).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-014 (Amendment 5, carried by SPEC-005).** §3 gains error codes for new conditions. Additive only (ADR-0054 §3): no existing operation, field, permission, MCP level or tool changes; semantics of test runs, pinned data and effect confirmation: ADR-0082 (SPEC-014).
- **SPEC-009 K19 (2026-10-13, owner decision).** §4 (`idempotency_record.status`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
- **SPEC-012 (Amendment 2, 2026-10-13, owner decision).** §4: a secret Orcon generates is returned once, in `<Resource>WithSecret.secret`, by the operation that creates or rotates it, which requires idempotency. §7: a failed `Operation` reports `failure`, never `error`. §9: protocol-tagged operations are the only exception to problem+json errors.
- **SPEC-005 (Amendment 8, 2026-10-13).** §3: new code `method-not-allowed` 405 for a new condition; `confirmation-required` confirmed as 403 and `federated-token-rejected` as 400, as this section allowed the implementing slices to do.
- **SPEC-005 (Amendment 8, Q-API-C, 2026-10-14, owner decision).** §4: the stored idempotent response body is kept in a column of `idempotency_record` together with its format (`response_body_document` and `response_body_document_format`, as SPEC-005 §11.2 states), not as a reference to a blob (ADR-0016); this replaces the column `response_body_reference`.
- **SPEC-005 (Amendment 8, API1 QA, BL-090, 2026-10-14).** §3: new code `unsupported-media-type` 415 for a new condition: the media type of a request body is not one the operation declares.

- **SPEC-021 (2026-10-16, Proposed until the owner accepts ADR 0084).** §7: "Importing n8n workflow files supplied in the request body is synchronous (ADR-0084 §1); importing users or credentials from a remote n8n instance stays long-running."
