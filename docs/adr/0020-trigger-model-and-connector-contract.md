<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0020 — Trigger model, catalog and connector trigger contract

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-002.

## Context
BL-002 (owner decisions T1–T7) defines what starts a flow execution. The owner requires the V1 trigger catalog to cover at least every trigger kind n8n offers. SPEC-001 (ADR-0007…0019, Accepted) defines the engine; triggers must only *create executions atomically* through it. ADR-0002 assigns trigger management to the scheduler role and HTTP to the api role.

## Decision

### 1. Trigger families
A trigger is a `startEvent` node whose `type` belongs to one family:
- `WEBHOOK` — inbound HTTP (generic webhook, hosted form, chat, MCP server).
- `APP_EVENT` — inbound HTTP whose endpoint is registered at a third-party provider by Orcon (ADR-0027).
- `SCHEDULE` — calendar or interval rules (ADR-0025).
- `POLL` — periodic pull with a durable cursor (ADR-0026).
- `LISTEN` — persistent connection to a broker/stream/mailbox/file system (ADR-0026).
- `INTERNAL` — manual run, called-by-flow, error flow, flow lifecycle, evaluation.

The **trigger key** is `(flowId, nodeId)`; node ids are stable across versions (ADR-0007 §3), so trigger state (addresses, deduplication window, schedule progress, cursors, holders, registrations) survives publication of new versions.

All families converge on one operation, `AcceptEvents(batch)` (ADR-0021), which creates executions durably.

### 2. Definition block
`startEvent` nodes may carry an additive `trigger` block in `orcon/flow@1`:
`{ family, addresses?: [Address], response?: ResponseSpec, dedupe?: { source: "HEADER" | "JSON_PATH" | "CONNECTOR", name?, window? (1 min – 90 d, default 24 h) }, schedule?: { timezone?, rules: [Rule], missed: { policy: "SKIP" | "CATCH_UP_ONCE", marginHours? } }, poll?: { rules: [Rule] }, listen?: { maxInFlight?, hostTag? }, registration?: {}, inputSchema?: Schema, callers?: "PROJECT" | [flowId] }`.
`trigger.configHash` = SHA-256 of the JCS form of the block plus node `type` and `parameters`.
Enumerated values in this block are UPPER_SNAKE (ADR-0080 §3): the flow document is returned verbatim by the API, so it follows the same spelling as every other public value.

### 3. Connector trigger contract (Go; packaging and sandboxing of third-party connectors: ADR-0029 and ADR-0032)
Every trigger node type declares: `family`, `delivery` (`AT_LEAST_ONCE` | `AT_MOST_ONCE` | `CURSOR_EXACT`), `dedupeKeySource`, `sensitiveFields`, `maxPayload`, `n8nTypes` (n8n node type names it replaces), and implements the hooks of its family:
- `WEBHOOK` / `APP_EVENT`: `Routes(params) []RouteSpec`; `Verify(req, secret) error`; `Handshake(req) (resp, handled bool)`; `Decode(req) ([]Item, dedupeKey string, err error)`. `Verify`, `Handshake` and `Decode` must be pure (no network), bounded to 50 ms and to the request size.
- `APP_EVENT` additionally: `Register(ctx, cred, callbackURL, params) (externalRef, secret, expiresAt, error)`, `Deregister(ctx, cred, externalRef)`, `List(ctx, cred) ([]Registered, error)`, `Renew(ctx, cred, externalRef) (expiresAt, error)`.
- `POLL`: `InitialCursor(ctx, cred, params) (cursor, emitInitial bool, error)`; `Poll(ctx, cred, params, cursor) (items []Item, next cursor, error)` — must be a pure function of the remote state and the cursor (re-polling with the same cursor is safe).
- `LISTEN`: `Open(ctx, cred, params, cursor) (Subscription, error)`; `Subscription.Next(ctx) ([]Message, error)` where `Message = { id, items, ackHandle, cursor? }`; `Ack(handles)`, `Nack(handles)`, `Close()`.
A node type without a valid declaration cannot be published.

### 4. Execution context
`$execution.trigger = { kind, nodeId, addressId?, eventId, receivedAt, scheduledFor?, catchUp?, dedupeKey?, source? }` (ADR-0008 layer 5; n8n-compatible expression context, not an API surface). Execution modes add `WEBHOOK`, `TRIGGER`, `TEST`, `EVALUATION` to ADR-0009 (API values, UPPER_SNAKE per ADR-0080 §3).

### 5. Catalog coverage rule
For the pinned n8n version (SPEC-001 P8), every trigger node type listed in n8n's public documentation is mapped to a family and an Orcon node type, or to "delivered with connector X" with its family fixed. The importer's coverage report must show zero unmapped trigger types. App triggers ship with their connectors; the machinery for every family ships in V1.

### 6. Role placement (no new roles)
- `api`: HTTP ingress (webhooks, forms, chat, MCP), provider handshakes, response delivery.
- `scheduler`: schedule firing and missed-slot detection; trigger desired-state reconciliation (which polls are due, which holders/registrations must exist); test-session expiry. The scheduler decides when and by whom; it never runs connector code and never holds connector credentials (owner decision, SPEC-002 Q2).
- `worker`: connector-side trigger work requiring credentials and network — polls, persistent listeners, external registration calls — claimed as leased trigger tasks. A worker started with `--trigger-hosting=false` never hosts them.
- `standalone`: all of the above in one process.
Claims of due work across organizations (schedule slots, polls, holders, registrations) run through engine-plane functions (ADR-0047 as amended by ADR-0077).

### 7. Trigger events
A non-execution event stream `trigger_event (org_id, id, occurred_at, flow_id, node_id, type, schema_version, data_document)`, append-only, retained 30 days, with types: `ingress.accepted` (aggregated per batch), `ingress.rejected{reason}` (reasons include `CAP`, `CAP_BYTES`, `CAP_INSTANCE_BYTES`), `ingress.cap_warning{kind}`, `ingress.duplicate`, `schedule.fired|missed|catchup|skipped`, `poll.completed|failed`, `listener.acquired|lost|failed|rebalanced`, `registration.requested|succeeded|failed|renewed|removed|orphan_removed`, `flow.published|activated|deactivated|rolled_back`, `address.collision`, `test.session_opened|expired`. Event types keep their own form; enum values inside event data are UPPER_SNAKE (ADR-0080 §3). Sensitive values are redacted (ADR-0017). An `address.collision` event about another organization's address carries no detail of that organization (ADR-0022 §3). Catalog published in `docs/reference/trigger-events.md`.

### 8. Internal triggers
- Manual: API run on a draft snapshot or a published version; mode `MANUAL`; no deduplication or cap.
- Called-by-flow: `inputSchema` (typed fields, JSON example, or accept-all) and `callers` policy; mechanics per ADR-0010 §7.
- Error: input `{ execution: { id, url, error, lastNodeExecuted, mode, retryOf }, workflow: { id, name }, trigger? }` (the n8n-compatible expression shape); also started (rate-limited) for persistent trigger failures (registration, listener, poll).
- Lifecycle: events `INSTANCE_STARTED` (once per process start of each scheduler or standalone process; api and worker processes do not fire it; with N schedulers it fires N times, documented as a difference from single-process n8n — owner decision, SPEC-002 Q9), `FLOW_ACTIVATED`, `FLOW_PUBLISHED`, `FLOW_DEACTIVATED`; the execution is created in the same transaction as the lifecycle action.
- Evaluation: manual-only trigger iterating a dataset (one execution per row, bounded concurrency); dataset sources and metrics belong to the AI design (BL-006).

## Consequences
One acceptance path for every source; trigger state is keyed independently of versions; connector authors implement a small family-specific interface.

## Alternatives considered
Triggers as long-running executions (n8n-style "trigger node keeps running"): rejected — ties trigger lifetime to an execution and to one process. Per-version trigger identity: rejected — breaks stable URLs, deduplication windows and cursors across publications. Running connector trigger code in the scheduler: rejected by the owner (SPEC-002 Q2) — it would bring credentials and long-lived connections into the process that fires schedules and reclaims leases.

## Related
Human waits started by triggers (multi-page form pages, Wait-node resume by form or webhook) are SPEC-001 durable waits; any notification about them is governed by ADR-0073 (human-attention notifications), where they are the `END_USER_WAIT` kind: no notification by default, enabled per node with `attention.notify: ON` (owner decision SPEC-002 N1). This ADR defines none.

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-003.** §3: packaging and sandboxing are defined by ADR-0029 and ADR-0032; declarative triggers are defined by ADR-0030 §8; WASM connectors may implement `WEBHOOK`, `POLL` and `APP_EVENT`, not `LISTEN`.
- **SPEC-006.** §8: the evaluation trigger is defined by ADR-0069.
