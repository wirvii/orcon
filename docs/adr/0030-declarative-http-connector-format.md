<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0030 — Declarative HTTP connector format

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-003.

## Context
K1 makes declarative connectors the default: most integrations are REST APIs whose behaviour can be described as data. A declarative format needs no sandbox for code, executes at native Go speed, is easy to review, and is the target of the n8n community node converter (ADR-0035). ADR-0079 (Proposed) amends §5 (ambiguous outcomes and error codes) and §10 (recorded tests). Enumerated values follow ADR-0029 §10.

## Decision

### 1. Body `connector.json`
`{ format: "orcon/declarative@1", http: {...}, operations: {...}, pagination?: {...}, triggers?: {...} }`, validated by `internal/connector/schema/orcon-declarative-1.schema.json`. At install it is compiled into an execution plan cached by package digest.

### 2. HTTP defaults
`http { baseUrl: template, headers?: { name: template }, timeout?: duration (default 30 s), encoding?: "JSON" }`. `baseUrl` must resolve to a granted domain (ADR-0029 §8); otherwise the request fails as `PERMANENT` with `EgressDenied`.

### 3. Parameters
Parameter schema per node/operation: `name`, `type` (`STRING`, `NUMBER`, `BOOLEAN`, `OPTIONS`, `MULTI_OPTIONS`, `COLLECTION`, `FIXED_COLLECTION`, `JSON`, `DATE_TIME`, `RESOURCE_LOCATOR`, `BINARY_PROPERTY`), `required`, `default`, `options[]`, `displayWhen` (conditions on other parameters), `sensitive`, `description`. Defaults are explicit values; a default change between versions is a recorded change (ADR-0034). Option values a connector defines for its own parameters are connector data, not values of this schema.

### 4. Authentication schemes (applied by the host)
`NONE`; `API_KEY { in: HEADER|QUERY|COOKIE, name, prefix? }`; `BASIC`; `BEARER`; `OAUTH2 { grant: AUTHORIZATION_CODE|PKCE|CLIENT_CREDENTIALS, authUrl, tokenUrl, scopes[], tokenPlacement }` (flows and refresh owned by the security design); `CUSTOM_HEADERS { name: template }`; `AWS_SIG_V4 { service, region: template }`; `SESSION_TOKEN { login: opName, extract: path, ttl, placement }`. Templates in schemes may reference `$credentials.<field>`; these templates are evaluated by the host and their results never enter items, logs or history.

### 5. Operations
```
"<resource>.<verb>": {
  request: { method, path: template, query?: { k: template }, headers?: {...},
             body?: { type: JSON|FORM|MULTIPART|RAW|BINARY, template | fromBinary },
             itemMode: PER_ITEM | { batch: { maxItems, assemble: template } } },
  response: { select?: path|expr, split?: path, map?: template,
              binary?: { property, mediaType? }, empty: NO_ITEMS|ONE_EMPTY_ITEM,
              errors?: [ { when: { status?: [int|range], body?: expr }, class: TRANSIENT|PERMANENT|UNCERTAIN, code?, message?: template } ] },
  safety: (ADR-0029 §5),
  rateLimit?: name, retry?: { classify?: [...] }
}
```
Template scope: `$parameter`, `$json` (current item), `$item`, `$response` (in response mapping), `$page` (in pagination), the ADR-0018 `$` helpers. `$credentials` is not visible outside §4. Paths use a documented JSONPath subset (`$.a.b[*].c`, filters with `==`, `!=`, `<`, `>`, `exists`); they are evaluated by the host in Go, never by SQL.
Error mapping (ADR-0079 §4–§5): `class: UNCERTAIN` is allowed only on operations whose purity is `EFFECT`. For a write without an idempotency key (`RECONCILE` or `UNSAFE`), a 5xx response, a timeout or a connection failure after the request was sent is `UNCERTAIN` even without an explicit mapping; a 429 is `TRANSIENT` for every level and means the request was not applied. `code` is one of `AuthInvalid`, `AuthExpired`, `PermissionDenied`, `NotFound`, `Conflict`, `ValidationFailed`, `RateLimited`, `QuotaExceeded`, `PayloadTooLarge`, `ServiceUnavailable`, `Timeout`, `NetworkError`, `Unsupported`, `ProviderError` (identifiers); host-raised codes such as `EgressDenied` and `ConnectorRevoked` stay with the host.

### 6. Pagination (read operations only)
Strategies: `OFFSET { param, limitParam, pageSize }`, `PAGE { param, start }`, `CURSOR { from: PATH|HEADER, param, in: QUERY|BODY }`, `LINK_HEADER { rel: "next" }`, `NEXT_URL { from: PATH }`. Common: `maxPages` (mandatory, default 1000), `stopWhen?: expr`, `emit: "STREAM"` (pages appended to paged NDJSON blobs, ADR-0016 §5). Pagination on a non-read operation is a validation error. Reaching `maxPages` is a `PERMANENT` error unless `onMaxPages: "TRUNCATE"` is declared, in which case a warning is recorded. The link relation `next` is fixed by RFC 8288.

### 7. Rate limits and retries
`rateLimits[]: { name, key: "CREDENTIAL" | "HOST" | expr, requestsPerSecond?, burst?, concurrency?, respectHeaders?: { retryAfter?: header, resetAt?: { header, format: EPOCH|EPOCH_MS|SECONDS|HTTP_DATE }, remaining?: header } }`. Each maps to an ADR-0014 level-4 bucket with key `connectorId:name:hash(credentialId|host|value)`; operators can override capacity per instance. 429 and `Retry-After` set the bucket's blocked-until time. Retry policy and defaults are ADR-0011; connectors may only add classifications; a write retries only if its safety contract allows.

### 8. Declarative triggers (ADR-0020 families)
- `WEBHOOK`: `decode { items: path, dedupeKey?: path|header }`, `verify?: { preset: GITHUB|STRIPE|SLACK|SHOPIFY|META|STANDARD_WEBHOOKS | hmac {...} }` (ADR-0022 §5), `handshake?: { preset: META_CHALLENGE|SLACK_URL_VERIFICATION|GRAPH_VALIDATION_TOKEN }`.
- `POLL`: `request` (as §5), `items: path`, `cursor { from: PATH|HEADER|MAX_FIELD, initial: expr, emitInitial: bool }`, optional `seenIds { path, ttl }` for APIs without monotonic cursors (ADR-0026).
- `APP_EVENT`: `register`, `deregister`, `list`, `renew` as operations whose responses map to `externalRef`, `secret`, `expiresAt`.
- `LISTEN` is not declarative.

### 9. No embedded code
Anything not expressible by §2–§8 requires a native (official) or wasm connector. There is no inline script escape hatch. A need that the format cannot express is registered as a declarative gap (ADR-0079 §9).

### 10. Tests
`tests/<resource>.<operation>.<case>.json` use `orcon/cassette@1` (ADR-0079 §3): `{ format, operation, case, example, parameters, items, credential { type, fixture }, interactions: [ { request: { method, url, headers (credential-applied values replaced by `[REDACTED:credential.<field>]`), body }, response: { status, headers, body } } ], expect: { items } | { error: { class, code } }, recorded: { at, orconVersion, scrubber { version, rulesDigest } } }`; non-JSON bodies are `{ base64, mediaType }`. A case may hold several interactions (pagination, OAuth refresh, errors); one success case per operation is marked `example: true`. Recordings are scrubbed of secrets and personal data before they are written. Required for catalog submission; replayed offline by the conformance kit (ADR-0031 §5); recording, replay and request matching are implemented with the Go standard library in the shipped binary. Recorded tests are internal test artifacts and keep the spelling defined by ADR-0079 (ADR-0029 §10).

## Consequences
Most connectors are data, reviewable without code review and executed by one well-tested interpreter. The format is expressive enough for the n8n declarative node style without copying it. Ambiguous write outcomes are classified the same way in every declarative connector.

## Alternatives considered
OpenAPI-only import (rejected as the sole format: lacks pagination, auth flows, safety, item mapping; an OpenAPI-to-declarative generator may come later); inline JavaScript hooks (rejected: reintroduces code review and sandboxing for every connector); a single recorded exchange per test (rejected by ADR-0079 §3: cannot cover pagination, OAuth refresh or error sequences).
