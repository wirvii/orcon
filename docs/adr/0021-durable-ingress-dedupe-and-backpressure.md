<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0021 — Durable ingress: acceptance, group commit, deduplication and backpressure

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-002.

## Context
T1: persist the incoming event before acknowledging, at millisecond cost, with optional deduplication. T4: per-flow pending cap (default 100,000), 429 with Retry-After beyond it. Owner decisions (SPEC-002): Q3 byte caps per flow and per instance; Q4 deduplication window 24 h default, up to 90 days. ADR-0003: state and new runnable work committed atomically. D20 S2 benchmarks high webhook rates. ADR-0077 fixes the schema standards (organization column on every organization-scoped row, `id` keys, names without abbreviations, documents in `bytea`/`BLOB`).

## Decision

### 1. AcceptEvents
For each accepted event, one transaction (shared by a batch) inserts:
1. `ingress_event` row (with `deduplication_key` if any, `payload_reference`, `header_reference`, `source_reference`);
2. `execution` (QUEUED) pinned to `flow.active_flow_version_id` **read inside the transaction**; the transaction also verifies that the trigger is active in that version and that the flow is not deleted (else the event is rejected as `INACTIVE`);
3. the root token (RUNNABLE) and the trigger node's output (the event items) as its node output;
4. `pg_notify('orcon_work', fairnessKey)` (ADR-0013 §5);
5. cursor/offset updates for `CURSOR_EXACT` sources (ADR-0026) and holder/poll fence checks.
Only after commit does the caller acknowledge (HTTP response, broker ack, schedule advance). The transaction runs with the organization context returned by the address resolution (ADR-0022 §3) or carried by the claimed trigger row.

### 2. Group commit
The api role buffers acceptances and commits every ≤ 2 ms or ≤ 256 events (configurable), one transaction per batch, mirroring ADR-0012 §4. Bodies > 256 KiB are written to the blob store and synced before the batch commits (ADR-0016). Maximum request size: 16 MiB by default (instance and per-address settings). Within a batch, deduplication conflicts are resolved with `INSERT … ON CONFLICT DO NOTHING RETURNING`. A batch that spans organizations follows the per-organization statement rule of ADR-0047 (one transaction, context set before each organization's statements).

### 3. Deduplication
- Key source per trigger: `HEADER` (header name), `JSON_PATH`, or `CONNECTOR` (message id). A JSON path is read in Go while decoding the request; no SQL statement reads inside a JSON document (Go rulebook DA-3 as amended; ADR-0077 §9).
- Unique constraint `uq_ingress_event_org_id_flow_id_node_id_deduplication_key` on `(org_id, flow_id, node_id, deduplication_key)`; `deduplication_expires_at = received_at + window` (default 24 h; configurable per trigger from 1 minute to 90 days, the execution-log retention; a window outside that range blocks publication, rule T-V9). Expired keys are pruned in batches.
- A duplicate creates nothing and is answered with the original execution id (`Orcon-Duplicate: true`) and, while it is retained, the original's result; in wait modes it waits on the original's response channel.
- Test executions use a separate namespace and never collide with production keys.

### 4. Backpressure
- Pending = executions created by triggers of the flow still in QUEUED. Pending bytes = sum of `ingress_event.size_bytes` (body, headers and attached blobs) of those executions, per flow and instance-wide.
- Each api process keeps a snapshot per flow and one for the instance, refreshed every ≤ 1 s (`count`/`sum` over an index on `(org_id, flow_id, status)`; the instance-wide sum crosses organizations and is read through an engine-plane function, ADR-0047), plus local increments since the snapshot. When any of the three caps is reached (pending count per flow, pending bytes per flow, pending bytes per instance) → `429 Too Many Requests` with `Retry-After` in [1, 60] s estimated from the observed drain rate. Overshoot is bounded by (api processes × batch size × refresh interval × rate) and documented.
- Defaults (owner decision, SPEC-002 Q3, explicitly confirmed): `maxPending` 100,000 per flow; `maxPendingBytes` 1 GiB per flow; `maxPendingBytesInstance` = 10 % of the data-disk size on SQLite (measured at startup and every 5 minutes) and 20 GiB on PostgreSQL. All configurable. Each cap emits `ingress.cap_warning` at 80 %. The per-flow byte cap keeps one flow from consuming the instance budget; the instance cap protects every flow from a full disk.
- Non-HTTP families honour the same three checks: polls do not advance cursors, listeners stop consuming without acking, schedules record the outcome `SKIPPED_BACKPRESSURE`.
- A per-IP token bucket and a concurrent-request limit protect the api role before any persistence.

### 5. Logical schema additions (ADR-0015 as amended by ADR-0077)
All tables below are organization-scoped: `org_id` NOT NULL, `id` primary key (application-generated UUIDv7), `created_at`, and `updated_at` unless append-only; composite foreign keys `(org_id, <parent>_id)`; natural keys are unique constraints starting with `org_id`; enumerated columns are text with a CHECK over UPPER_SNAKE values; documents are `<name>_document` with `<name>_document_format`, stored as `bytea` (PostgreSQL) or `BLOB` (SQLite); every SQLite table is STRICT; no views; DDL runs on PostgreSQL 14.
- `trigger(org_id, id, flow_id, node_id, family, type, configuration_hash, flow_version_id, status, health, last_error)`; unique `(org_id, flow_id, node_id)`.
- `trigger_address(org_id, id, trigger_id, family, scope, pattern, static_prefix, host, random_suffix, response_profile, status, retired_at)`; `trigger_address_method(org_id, id, trigger_address_id, method)` (one row per allowed HTTP method; methods take part in route resolution and collision checks, so they are a real column).
- `ingress_event(org_id, id, flow_id, node_id, trigger_address_id, scope, received_at, deduplication_key, deduplication_expires_at, payload_reference, header_reference, source_reference, execution_id, size_bytes)`.
- `ingress_response(org_id, id, ingress_event_id, status_code, header_document, body_reference, is_final)`; `ingress_stream_chunk(org_id, id, ingress_event_id, sequence_number, data, is_final)` append-only, unique `(org_id, ingress_event_id, sequence_number)`.
- `schedule_state(org_id, id, flow_id, node_id, rule_index, time_zone, rule_specification, next_slot_at, last_fired_slot_at, configuration_hash, lease_owner, lease_epoch, lease_expires_at)`; `schedule_fire(org_id, id, flow_id, node_id, rule_index, slot_at, outcome, execution_id, range_ended_at)` append-only, unique `(org_id, flow_id, node_id, rule_index, slot_at)`.
- `poll_state(org_id, id, flow_id, node_id, cursor, cursor_version, seen_id_set_reference, next_poll_at, lease_owner, lease_epoch, lease_expires_at, failure_count, configuration_hash)`.
- `listener_holder(org_id, id, flow_id, node_id, lease_owner, lease_epoch, lease_expires_at, host_tag, cursor, status, last_error)`.
- `trigger_registration(org_id, id, flow_id, node_id, trigger_address_id, provider, token_hash, desired_state, actual_state, external_reference, secret_reference, expires_at, next_attempt_at, attempt_count, last_error, lease_owner, lease_epoch, lease_expires_at)`.
- `test_session(org_id, id, flow_id, flow_snapshot_id, opened_by_principal_id, expires_at, maximum_event_count, received_count)`.
- `trigger_event(…)` (ADR-0020 §7); `flow.activation_state` (`ACTIVE` | `INACTIVE`).
Enumerated values stored in these tables (`family`, `scope`, `response_profile`, `status`, `health`, `outcome`, `desired_state`, `actual_state`, `activation_state`) are the same UPPER_SNAKE values the API and the flow document use (ADR-0080 §3). Lookups before the organization is known use `orcon_engine.resolve_trigger_address` and `orcon_engine.resolve_public_entry` (ADR-0077 §6). Ingress events and responses follow the execution's data retention (ADR-0017); response rows are deleted 10 minutes after delivery.

### 6. Failure semantics
- Crash before commit: no 2xx was sent; the client's retry is accepted normally (deduplicated if keyed).
- Crash after commit, before the response is written to the socket: the event and execution exist; the client may retry; with a deduplication key it receives the original execution id. Without a key, a retry creates a second execution — documented; the editor recommends a deduplication key for webhook senders that retry.
- Fault-injection additions to ADR-0019 (rows of its §3 matrix): FI-50 kill api before/after ingress commit; FI-51 duplicate deliveries across three api processes.

## Consequences
Every acknowledged event survives any single-process crash; throughput relies on batching; one flood is contained to its flow; every ingress row carries its organization, so row-level security applies to the ingress path.

## Alternatives considered
Acknowledge then persist from memory (rejected by T1). Separate ingress queue converted to executions later (rejected: double writes, a second backlog to reason about). Exact pending counter row per flow (rejected: hot row under high rates; snapshot with bounded overshoot instead). Count-only cap (rejected by the owner, SPEC-002 Q3: 100,000 × 16 MiB could reach 1.6 TB for one flow). Storing allowed methods as an array (rejected by ADR-0077: arrays are not used and methods are filtered).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-004.** §2: ingress batches are split per organization inside the transaction; §4: pending counts are also kept per organization (level 0 of ADR-0014).
