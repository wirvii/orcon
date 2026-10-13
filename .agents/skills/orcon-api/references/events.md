<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Events, webhooks and streams

Status: Proposed — pending owner approval

This document defines how public event types are named and described in the contract, what their payloads may carry, and how SSE streams and outbound webhooks deliver them. It refines ADR-0055; describing events in the OpenAPI `webhooks` section is fixed by [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md) §6. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`EVT-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by ADR-0080 (while ADR-0080 is Proposed, as design input), then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## Event types in the contract

- **EVT-1** Public event types are `<area>.<event>` in lowercase snake segments, past tense (`execution.succeeded`, `flow.published`); CloudEvents `type` is `dev.orcon.<area>.<event>.v<N>`. [`events`]
- **EVT-2** Each type is an entry of the OpenAPI `webhooks` section with `x-orcon-event { type, cloudEventsType, permission, stability, since }`, a named payload schema, descriptions and an example. [`events`]
- **EVT-3** Payloads are thin (ids, status, timings, sanitized error summary); execution data only with `includeData` and `execution:read-data`, otherwise `dataHidden: true`; human-attention payloads follow ADR-0055 §2 redaction. Never secrets. [`events`, review]

## Delivery

- **EVT-4** Events are written in the same transaction as the change; delivery is at-least-once; consumers deduplicate by `webhook-id`/event id; exactly-once is never claimed. [review]
- **EVT-5** SSE: `GET …/stream`, SSE `id` = `orconseq`, resume with `Last-Event-ID`, `stream.reset` when out of retention, heartbeat 15 s. [`rate-class`, review]
- **EVT-6** Outbound webhooks are signed with Standard Webhooks (`webhook-id`, `webhook-timestamp`, `webhook-signature`). [review]
- **EVT-7** A breaking change to a payload creates `.v<N+1>`, emitted alongside the old type for the deprecation window ([VER-6](versioning.md)). [review]
