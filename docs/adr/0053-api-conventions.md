<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0053 — API conventions: pagination, filtering, errors, idempotency, concurrency, rate limits, long-running operations and response shapes

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: owner answers Q1–Q14, human-attention surface; ADR-0080 error catalog and response shapes; ADR-0077 schema rules)

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
The code catalog is part of the contract (`components/schemas/ProblemCode` with `x-orcon-problem-catalog`; generated page `docs/reference/problem-codes.md`); each entry has a code, an HTTP status, a title, when it applies and whether it is retryable: `validation-failed` 400, `invalid-page-token` 400, `experimental-not-enabled` 400, `unauthenticated` 401, `forbidden` 403, `step-up-required` 403 (ADR-0038 §6 policy challenge), `personal-keys-disabled` 403, `not-found` 404, `conflict` 409, `idempotency-in-progress` 409, `secret-already-issued` 409, `attention-already-resolved` 409, `project-managed-by-git` 409, `deprecated-removed` 410, `precondition-failed` 412, `payload-too-large` 413, `idempotency-key-reused` 422, `precondition-required` 428, `rate-limited` 429, `internal-error` 500, `service-unavailable` 503; `confirmation-required` and `federated-token-rejected` have a proposed status confirmed by the implementing slices (the latter belongs to a protocol endpoint and uses the OAuth error format). Every operation lists in `x-orcon-problems` every code it can return, including the minimum set for its shape (authenticated operations `unauthenticated`, `rate-limited`, `internal-error`; with a permission other than `authenticated` `forbidden`; with an id in the path `not-found`; with parameters or body `validation-failed`; with a body `payload-too-large`; with idempotency `idempotency-key-reused`, `idempotency-in-progress`; with `If-Match` `precondition-failed`, `precondition-required`; lists `invalid-page-token`; experimental `experimental-not-enabled`). Codes are stable kebab-case identifiers; a new code is added only for a new condition. Messages never contain secret values or data from another organization. Flow validation errors reuse ADR-0007 §9 levels with JSON Pointers. The field-error text member is `message`.

`test-input-missing` 422 (`errors[]` lists the nodes without seed data), `effect-preview-closed` 409, `pin-limit-exceeded` 409, `fragment-too-large` 422, `interactive-session-required` 403.

### 4. Idempotency
- `Idempotency-Key` (1–255 visible ASCII) accepted on every `POST`; required on: running a flow, creating/rotating API keys, creating OAuth clients, creating event subscriptions, redelivering webhooks, starting imports and Git syncs, the continue/repeat/cancel human-attention actions (ADR-0052 §6), and every long-running operation.
- `idempotency_record(org_id, id, requested_by_principal_id?, requester_identity, idempotency_key, fingerprint, status: in_progress|completed, response_status, response_body_reference, resource_reference, created_at, expires_at)`, unique `(org_id, requester_identity, idempotency_key)`, TTL 24 h. `requester_identity` is the exact requesting principal (a user, a service account, or the API key or OAuth token for key-authenticated calls), so two keys of the same user do not share records; `requested_by_principal_id` is the actor column of ADR-0077 §8. Fingerprint = SHA-256 of method, route template, path parameters and canonical body.
- Same key and fingerprint after completion → stored response replayed with `Idempotent-Replayed: true`. Different fingerprint → `422 idempotency-key-reused`. In progress → `409 idempotency-in-progress` with `Retry-After`.
- The record is written in the same transaction as the state change; handlers that start durable work store the created resource id so a replay returns it even if the first response was lost.
- Responses disclosing a newly issued secret are stored without it; a replay returns `409 secret-already-issued` with the resource id.
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
- Executed by `worker` (or `standalone`), claimed through an `orcon_engine` function (ADR-0047 §2) with lease and fencing (ADR-0013 §4); a crashed holder's operation is reclaimed; each kind defines idempotent steps or checkpoints.
- Readable by its creator and by principals holding the read permission of its target; completion emits `operation.finished`.

### 8. Running a flow synchronously
`POST /flows/{flowId}/executions` (permission `flow:execute`) accepts `wait` (≤ 60 s). Finished in time → `200` with the `Execution` resource, which carries its output summary (data only with `execution:read-data`, else the resource field `dataHidden: true`); otherwise `202` with the `Execution` resource. The wait uses the cross-process notification pattern of ADR-0023 §2.

### 9. Response shapes (ADR-0080 §7; owner decision 2026-10-12)
- A single resource is returned as the object itself; a list is exactly `{ items, nextPageToken? }` (plus `total`/`totalIsLowerBound` only where `x-orcon-include-total` is declared); an error is always `application/problem+json` (§3).
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
