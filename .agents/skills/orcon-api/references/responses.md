<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Responses

Status: Proposed — pending owner approval

This document defines the shape of every public API response body and the status codes that carry it. Responses have no envelope (maintainers' decision of 2026-10-12, [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md) §7). MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`RESP-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by ADR-0080 (while ADR-0080 is Proposed, as design input), then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## Body shapes

- **RESP-1** A single resource is returned as the object itself: `GET /flows/{flowId}` → the `Flow` object. No wrapper. [`response-shape`]
- **RESP-2** A list is always exactly `{ "items": [...], "nextPageToken": "..." }`; `nextPageToken` is absent on the last page; the only allowed addition is `totalIsLowerBound`/`total` where `x-orcon-include-total` is declared ([PAGE-4](pagination.md)). [`response-shape`]
- **RESP-3** An error is always `application/problem+json` (RFC 9457) with `type`, `title`, `status`, `detail`, `instance`, `code`, `requestId` and `errors[] { pointer, code, message }` ([ERR-1](errors.md)). [`response-shape`, `problem-responses`]
- **RESP-4** Auxiliary data travels in headers: `Orcon-Request-Id`, `ETag`, `RateLimit`, `RateLimit-Policy`, `Retry-After`, `Location`, `Idempotent-Replayed`, `Deprecation`, `Sunset`, `Link`. Never copied into bodies. [`response-shape`]

## Status codes

- **RESP-5** Status codes: `200` with the resource; `201` with the created resource and `Location`; `202` with the `Operation` resource ([long-running.md](long-running.md)) or the execution (`wait` expired); `204` with no body. [`response-shape`, `long-running`]

## Forbidden envelope members

- **RESP-6** Forbidden at the root of a 2xx body: `data`, `meta`, `success`, `ok`, `result`, `results`, `payload`, `response`, `error`, `errors`, `pagination`, `links`, `requestId`, `etag`, and a boolean `status`. Visibility flags that describe the resource itself (`dataHidden`) are resource fields, not envelopes. [`response-shape`]
