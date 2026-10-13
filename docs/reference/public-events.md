<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Public event catalog

Status: **Accepted** | Source: ADR-0055 §1–§2 (accepted 2026-10-12 via SPEC-005), with the types added by SPEC-006

This page publishes the public event catalog defined by ADR-0055. The ADR is normative; if this page and the ADR disagree, the ADR wins. Public events are delivered by the real-time streams of ADR-0055 §3 and by signed outbound webhooks (ADR-0055 §4–§6). They are a curated subset, separate from the execution events of ADR-0009 (`docs/reference/execution-events.md`) and from the trigger events of ADR-0020 §7 (`docs/reference/trigger-events.md`).

## Storage
`public_event (org_id, id, sequence_number, project_id, flow_id?, execution_id?, type, subject, occurred_at, data_document, data_document_format, data_schema)`. `sequence_number` is a per-organization monotonic sequence, unique together with `org_id`. A row is inserted in the same transaction as the state change it describes. Retention defaults to 7 days and is configurable.

## Representation
CloudEvents 1.0 JSON structured mode: `specversion`, `id`, `source` (`/orgs/{org}/projects/{project}`), `type` (`dev.orcon.<area>.<event>.v1`), `subject`, `time`, `datacontenttype`, `dataschema`, `data`, and the extension attribute `orconseq`. The attribute names are fixed by the CloudEvents standard.

## Rules
- Event type names are identifiers and keep their lowercase dotted form; enumerated values inside payloads are UPPER_SNAKE (ADR-0080 §3).
- Payloads are thin: ids, status, timings and a sanitized error summary.
- Execution output data is included only when `includeData` is requested and the principal holds `execution:read-data`; otherwise `dataHidden: true`. Data is capped at 64 KiB with a link.
- Each type declares the permission needed to receive it, for example `execution:read` on its project.
- Each type is described as an entry of the OpenAPI `webhooks` section with `x-orcon-event { type, cloudEventsType, permission, stability, since }`, a named payload schema, descriptions and an example (ADR-0080 §6).
- Delivery is at least once. Receivers deduplicate by the stable `webhook-id`. Exactly-once delivery is never claimed.

## Catalog (v1)
| Area | Types |
|---|---|
| Executions | `execution.started`, `execution.succeeded`, `execution.failed`, `execution.canceled`, `execution.suspended`, `execution.waiting`, `execution.timeout` |
| External operations | `external_operation.uncertain`, `external_operation.resolved` |
| Human attention | `human_task.created`, `execution.awaiting_human`, `execution.awaiting_human.escalated`, `execution.human_resolved` |
| Flows | `flow.published`, `flow.activated`, `flow.deactivated`, `flow.rolled_back`, `flow.deleted` |
| Triggers | `trigger.health_changed` |
| API keys | `api_key.expiring`, `api_key.revoked` |
| Git | `git.sync.succeeded`, `git.sync.failed`, `git.conflict` |
| Confirmations | `confirmation.requested`, `confirmation.decided` |
| Operations | `operation.finished` |
| Webhooks | `webhook.subscription_disabled`, `webhook.delivery_dead` |
| API lifecycle | `api.deprecation_notice` |
| Operational alerts | `connector.revoked` |
| AI (SPEC-006) | `ai.budget.threshold`, `ai.budget.exceeded`, `ai.approval.requested`, `evaluation.run.finished`, `conversation.created` |

## Human-attention events
The semantics, triggering conditions, notification routing and escalation are defined by ADR-0073; ADR-0055 §2 defines only their public shape and delivery.

| Type | CloudEvents type | Meaning |
|---|---|---|
| `execution.awaiting_human` | `dev.orcon.execution.awaiting_human.v1` | An execution started waiting for a person. |
| `execution.awaiting_human.escalated` | `dev.orcon.execution.awaiting_human.escalated.v1` | An unattended wait moved to a higher escalation level. |
| `execution.human_resolved` | `dev.orcon.execution.human_resolved.v1` | A person continued, repeated or canceled; the payload carries `action` (`CONTINUE`, `REPEAT` or `CANCEL`) and the actor's id and kind. |

The payload is redacted by default: `attentionId`, organization, `executionId`, `projectId`, `flowId`, `flowVersion`, `nodeId`, flow and step names, `reason` (UPPER_SNAKE), `since`, deadline, `escalationLevel`, `availableActions`, and links to the REST operation and the editor view. Secrets never appear. `includeData` adds nothing to these types. The sent data (`sentData`) is present only when the project enabled data inclusion in notifications and the receiving principal holds `execution:read-data`; it is masked and capped at 4 KiB, and otherwise `dataHidden: true`. The permission needed to receive these types is `execution:read` on the project. Links in payloads never authorize an action by themselves.

## Operational alerts
`connector.revoked` (CloudEvents type `dev.orcon.connector.revoked.v1`) is emitted for each affected organization when a `CRITICAL` connector revocation affects installed packages (ADR-0073 §13). The payload carries the connector, the revoked version, the severity, the reason, the fixed version and the affected flows (id, name and project, capped with a link), filtered to the projects the receiver can read. It never carries secrets or execution data. The permission needed to receive it is read access to at least one affected project.

## Relation to execution events
The per-execution stream (`GET /api/v1/executions/{executionId}/events/stream`) carries the full ADR-0009 catalog for one execution, including live-only events, and requires `execution:read` (`execution:read-data` for data). Types of the ADR-0009 catalog that are not listed above are not public events and are not delivered through the organization stream or webhooks.
