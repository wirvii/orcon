<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0023 — Response modes and cross-process response channel

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-002.

## Context
T1 defines per-flow response modes: immediate (202 + execution id) or wait-for-result with a configurable timeout falling back to 202 + execution id. T7 requires n8n-compatible codes per mode on n8n addresses. Owner decision SPEC-002 Q1: the response profile is chosen per address. n8n also offers "respond with a dedicated node" and streaming. The HTTP request is held by an api process while the execution runs on a worker.

## Decision

### 1. Modes
- `IMMEDIATE`: answer after the ingress commit. Native: `202 {executionId}` with `Orcon-Execution-Id`. `N8N` profile: the captured n8n response for this mode.
- `LAST_NODE`: answer with the output of the last node to finish (data options `ALL_ITEMS`, `FIRST_ITEM_JSON`, `FIRST_ITEM_BINARY`, `NO_BODY`; status code configurable).
- `RESPOND_NODE`: answer from the first `orcon.respond@1` node reached (status, headers, body: JSON, text, binary, redirect, no data). Later respond nodes are recorded as warnings.
- `STREAM`: chunked/SSE response fed by stream-capable nodes (producers defined by BL-006).
- Timeout (`LAST_NODE`, `RESPOND_NODE`): default 30 s for the `NATIVE` profile and 300 s for the `N8N` profile (both configurable per trigger); on expiry `202 {executionId}` with header `Orcon-Execution-Id`; the execution continues. n8n has no such timeout; the `N8N` profile's timeout is the one deliberate difference from captured n8n behaviour and the importer reports it as a behaviour difference.

### 2. Channel
- The step that produces the response writes `ingress_response` (or `ingress_stream_chunk` rows, ordered by `sequence_number` and grouped by the ADR-0012 batch timer) in its confirmation-point transaction and issues `pg_notify('orcon_ingress', eventId)`.
- The holding api process LISTENs and polls every 250 ms as fallback; SQLite uses an in-process signal.
- The execution's terminal transaction writes the `LAST_NODE` response or an error response (500 with redacted error) if no response was produced.
- Responses larger than 256 KiB are blobs; the api streams them.
- A duplicate request (ADR-0021 §3) attaches to the same channel.
- If the holding api process dies, the client connection is lost; the execution is unaffected; a retried request with the same deduplication key gets the stored response while it is retained (10 minutes).

### 3. Compatibility suite
Status codes, headers and bodies for each mode/data option of the `N8N` profile are captured by running the pinned n8n version as a black box and checked by the same compatibility gate as expressions (ADR-0019 §5).

## Consequences
Waiting responses work across processes without sticky load balancing or a new service.

## Alternatives considered
Direct worker→api RPC (rejected: new internal network contract and discovery). Requiring sticky routing (rejected: fragile, n8n's MCP limitation). No timeout on n8n-profile addresses (rejected by the owner, SPEC-002 Q1: connections could hang indefinitely).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-006.** §2: model token fragments travel through the ephemeral channel of ADR-0063; fragments of non-AI nodes keep the persisted mechanism.
