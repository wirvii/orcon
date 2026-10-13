<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Idempotency and concurrency

Status: Accepted

This document defines when operations take an `Idempotency-Key`, how replays behave, and how optimistic concurrency with ETags works. It refines the idempotency and ETag conventions of ADR-0053. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`IDEM-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md), then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## Idempotency

- **IDEM-1** Every `POST` declares `x-orcon-idempotency: required | optional` and the `Idempotency-Key` header (1–255 visible ASCII). [`idempotency`]
- **IDEM-2** Required on: run a flow, create or rotate an API key, create an OAuth client, create an event subscription, redeliver, replay, start an import or Git sync, continue/repeat/cancel of a human-attention item, and every `x-orcon-long-running` operation. [`idempotency`]
- **IDEM-3** Replay with same fingerprint returns the stored response with `Idempotent-Replayed: true`; different fingerprint `idempotency-key-reused` (422); in progress `idempotency-in-progress` (409) with `Retry-After`; secrets are never stored, replay → `secret-already-issued`. [review]
- **IDEM-4** `PUT` and `DELETE` are idempotent by definition and do not take the key.

## Concurrency

- **IDEM-5** Editable resources return a strong `ETag`; `PUT`, `PATCH`, `DELETE` and state-guarded actions require `If-Match` (missing 428, mismatch 412 with the current ETag); `If-None-Match: *` creates only if absent. [`concurrency`]
- **IDEM-6** Draft `PATCH` takes JSON Patch (RFC 6902, with `test`); other `PATCH` take JSON Merge Patch (RFC 7396). [`concurrency`]
