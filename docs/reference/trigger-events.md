<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Trigger event catalog

Status: **Accepted** | Source: ADR-0020 §7 (accepted 2026-10-12 via SPEC-002)

This page publishes the trigger event catalog defined by ADR-0020 §7. The ADR is normative; if this page and the ADR disagree, the ADR wins.

Trigger events form a non-execution event stream, separate from the execution events of ADR-0009 (`docs/reference/execution-events.md`).

## Storage
`trigger_event (org_id, id, occurred_at, flow_id, node_id, type, schema_version, data_document)`, append-only, retained 30 days.

## Rules
- Event types keep their own form; enumerated values inside event data are UPPER_SNAKE (ADR-0080 §3).
- Sensitive values are redacted (ADR-0017).
- An `address.collision` event about another organization's address carries no detail of that organization (ADR-0022 §3).

## Catalog
| Area | Types | Notes |
|---|---|---|
| Ingress | `ingress.accepted` | Aggregated per batch. |
| | `ingress.rejected` | Carries `reason`; reasons include `CAP`, `CAP_BYTES`, `CAP_INSTANCE_BYTES`. |
| | `ingress.cap_warning` | Carries `kind`. |
| | `ingress.duplicate` | |
| Schedules | `schedule.fired`, `schedule.missed`, `schedule.catchup`, `schedule.skipped` | |
| Polling | `poll.completed`, `poll.failed` | |
| Persistent listeners | `listener.acquired`, `listener.lost`, `listener.failed`, `listener.rebalanced` | |
| External registration | `registration.requested`, `registration.succeeded`, `registration.failed`, `registration.renewed`, `registration.removed`, `registration.orphan_removed` | |
| Flow lifecycle | `flow.published`, `flow.activated`, `flow.deactivated`, `flow.rolled_back` | |
| Addresses | `address.collision` | No detail of another organization. |
| Test sessions | `test.session_opened`, `test.session_expired` | |
