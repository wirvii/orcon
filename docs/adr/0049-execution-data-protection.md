<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0049 — Execution data protection and visibility

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
S4: execution data visibility is a separate permission from execution metadata. D17/ADR-0017: masking in history, per-execution keys for working state.

## Decision
1. `execution:read` exposes execution and node status, timing, attempts, throttling, sanitized errors and counts. `execution:read-data` is required for items, node inputs/outputs, ingress payloads and headers, journal request/response references, variable values, blobs, "load past data in editor", "run with past event" and exports.
2. Without `read-data`, API responses omit data fields and set `dataHidden: true`; error messages pass through the masking set and a sanitizer that removes item payload excerpts.
3. Each access to protected data emits `execution.data_viewed` (coalesced per session and execution for 15 minutes; SPEC-004 Q9) or `execution.data_exported`.
4. The per-execution data key of ADR-0017 §3 is wrapped by the organization key; organization deletion shreds all working state.
5. Retention and masking remain as ADR-0017.
6. Human-attention notifications (ADR-0073): by default, email, outbound-webhook and in-app payloads carry identifiers, flow and step names, the waiting reason, the deadline and a link to Orcon — never secret values and no protected execution data. Viewing data through the link requires `execution:read-data`. A project may opt in to include the data a step was about to send (owner decision SPEC-004 N1): off by default; enabling it requires both `notification:manage` and `execution:read-data` and is audited (`notification.data_inclusion_changed`); the data then reaches only recipients (or the subscription's owning principal) holding `execution:read-data`, always masked (the masking set applies) and capped at 4 KiB. No notification lets anyone act without opening Orcon and authenticating (owner decision SPEC-004 N2).
7. The predefined Operator role has no `read-data` (SPEC-004 Q2); a custom role adds it where needed.

## Consequences
Operators can supervise without seeing customer data; every data view is accountable; notifications leaving Orcon do not carry customer data by default.

## Alternatives considered
A single execution permission (rejected by S4); full execution data in every notification (rejected: data leaves Orcon's access control).
