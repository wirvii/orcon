<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0073 — Human-attention notifications for suspended executions

Status: **Accepted** | Date: 2026-10-12 | Amended: 2026-10-12 (ADR-0077, ADR-0080)

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
Several engine states stop an execution until a person acts: an external write whose outcome is unknown (UNCERTAIN, ADR-0012 §7; unsafe writes are never retried after dispatch and wait for human review, SPEC-003 Q11), a poison token (ADR-0013 §9), a failed or uncertain compensation step (ADR-0011 §7), a human task in a flow (ADR-0010 §8) and the human approval of an AI tool call (SPEC-006, ADR-0066; default expiry 24 h). Every minute such an execution waits is business delay, and a write in doubt left unattended may hide a duplicate or missing charge. The owner requires (2026-10-12) that Orcon tell the right people immediately, on every configured channel, with enough information to act, escalate when nobody acts, keep a visible queue, and measure how long humans take.

Some waits are not operator work: a multi-page form or a Wait node resumed by a form or webhook (SPEC-002 R-FORM-04) is continued by the end user, so notifying operators would be noise. Some situations needing an administrator are not tied to one execution, such as a `CRITICAL` connector revocation that will make every run of the affected flows fail (SPEC-003 N2).

This ADR reuses, and does not redesign: the transactional outbox (ADR-0013 §8), the public event log, SSE stream and signed outbound webhooks (ADR-0055), permissions (ADR-0041), audit log (ADR-0048), execution-data visibility (ADR-0049), masking (ADR-0017 §3), AI redaction placeholders (ADR-0068), organization isolation (ADR-0046, ADR-0077), the egress client (ADR-0029 §8), the organization key hierarchy (ADR-0042) and the instance SMTP configuration (ADR-0039 §3). It complies with SPEC-004 R-AUD-09 (no secrets or protected data by default; internal recipients need project access; channel secrets encrypted with the organization key; sends through the anti-SSRF egress client; signed webhooks) and R-AZ-13 (a link never authorizes an action by itself). Its enumerated values follow ADR-0080 §3 (§14).

## Decision

### 1. Attention request
- An `attention_request` row is created **in the same transaction** as the transition that needs a person. Causes (closed set; a new cause requires amending this ADR):
  - `UNCERTAIN_OPERATION` — token SUSPENDED because of UNCERTAIN journal entries; one request per node activation, with item count.
  - `POISON_TOKEN` — token SUSPENDED after repeated abandonment.
  - `COMPENSATION_REVIEW` — execution SUSPENDED by a compensation step.
  - `AI_BUDGET_SUSPENDED` — an AI agent run paused because a budget with policy `suspend` was reached (SPEC-006, ADR-0062). Agent runs paused by an uncertain tool result are `UNCERTAIN_OPERATION`.
  - `HUMAN_TASK` — a human task assigned to organization users, groups or roles (ADR-0010 §8), including `kind: TOOL_APPROVAL` from SPEC-006 (expiry default 24 h, maximum 30 days; expiry is a resolution recorded as "denied by timeout" and returned to the model). The execution is WAITING, not SUSPENDED, but it still waits for a person.
  - `END_USER_WAIT` — a wait continued by the end user rather than an operator: a page of a multi-page form, or a Wait node resumed by a form or a webhook (SPEC-002 R-FORM-04).
- `cause_reference` (token id, human task id) is unique per organization (`uq_attention_request_org_id_cause_reference`), so a retried transaction cannot create a duplicate.
- States: `OPEN` → `RESOLVED` (a person acted) | `EXPIRED` (a human task timed out and its timeout path was taken) | `CANCELED` (the execution was canceled or reached a terminal state by other means). `acknowledged_by_principal_id` and `acknowledged_at` are set by the *acknowledge* action without leaving `OPEN`.
- The same transaction writes the ADR-0009 event `execution.awaiting_human`, the ADR-0055 `public_event` row for it, the `notification` rows for in-app recipients and the `outbox` rows for email deliveries.

### 1a. Notification class per wait kind, with per-node override
| Wait kind | Default | Per-node override (`attention.notify`, ADR-0007 §3) |
|---|---|---|
| `UNCERTAIN_OPERATION` | notify | none — mandatory, cannot be turned off |
| `HUMAN_TASK` with `kind: TOOL_APPROVAL` | notify | none — mandatory, cannot be turned off |
| `POISON_TOKEN`, `COMPENSATION_REVIEW`, `AI_BUDGET_SUSPENDED` | notify | none (not configured on a node) |
| `HUMAN_TASK` (operator task) | notify | `OFF` allowed |
| `END_USER_WAIT` | do not notify | `ON` allowed |
- `DEFAULT` follows the table. `OFF` on a mandatory kind is a publication error. When the class resolves to "do not notify", no attention request is created; the wait remains visible through `execution.waiting` and the execution view. The resolved class is stored on the request (`notify_class`: `NOTIFY` or `SILENT`).
- These defaults are owner decisions (SPEC-002 N1, 2026-10-12): end-user waits do not notify unless the node enables it; unsafe-write ambiguity and AI tool approvals always notify and cannot be turned off.

### 2. Channels
1. **In-app / editor**: one persisted `notification` row per recipient user (seen on next login if offline) and live delivery through the authenticated SSE stream of ADR-0055 §3 (`execution.awaiting_human` filtered to the user). Always enabled.
2. **Email**: through the instance SMTP configuration; one outbox row per recipient; `Message-ID` derived from the deduplication key. Without SMTP configured the channel is shown as inactive with a persistent admin warning; other channels continue. Email recipients outside the organization are not allowed.
3. **Outbound webhook**: ADR-0055 event subscriptions, signed with Standard Webhooks, sent through the anti-SSRF egress client (ADR-0029 §8), secrets encrypted under the organization key (ADR-0042), retried with dead letter as defined there. Creating, changing or deleting a notification channel requires `notification:manage` and emits `notification.channel_created|updated|deleted`. Subscriptions can be marked `purpose: ATTENTION`; such subscriptions receive the three attention events only when the routing (§4) names them for the current level, default `deliveryMode: GROUPED` (§7). Other subscriptions to these event types receive every event (`deliveryMode: EACH`). Body formats (owner decision SPEC-001 N3): `purpose: ATTENTION` subscriptions choose `bodyFormat: STANDARD | SLACK | PAGERDUTY` (ADR-0055 §4). `SLACK` is a Slack incoming-webhook message (text and blocks with the fields of §6, no interactive buttons); `PAGERDUTY` is a PagerDuty Events API v2 message whose `dedup_key` is the group key of §7, so a burst opens one incident, with the PagerDuty event action `trigger` on `execution.awaiting_human`/`.escalated` and `resolve` when the group has no open request left (values fixed by PagerDuty). All formats keep the Standard Webhooks signature headers and the redaction rules of §6.
- Delivery workers give attention deliveries a dedicated priority class so a backlog of other events never delays them (requirement on ADR-0055 delivery workers).
- Targets: p99 ≤ 1 s from commit to SSE delivery; p99 ≤ 2 s from commit to the start of the email/webhook send (excluding the remote provider's latency).

### 3. Delivery guarantees
- At least once. Deduplication key `(attentionRequestId, level, messageKind ∈ {INITIAL, GROUPED, REMINDER, ESCALATION, EXPIRING_SOON}, recipient, channel)`, unique in `notification_delivery` (`deduplication_key`); retries reuse it (`webhook-id`, email `Message-ID`). Exactly-once delivery is never claimed.
- A crash between commit and send loses nothing: the outbox / ADR-0055 dispatcher redelivers.
- A failing channel does not block other channels, other requests or the execution. Dead deliveries are listed for admins.

### 4. Routing
- `attention_routing` at three scopes (`scope_kind`: `ORGANIZATION`, `PROJECT`, `FLOW`). The most specific scope that defines a value wins; a scope may define per-cause overrides. Changing routing requires `notification:manage` and emits `notification.routing_changed`.
- Recipients: users, groups, roles (ADR-0041) and `purpose: ATTENTION` webhook subscriptions. The assignee/role of a human task are always included.
- Recipients are filtered by permission at send time: a user is notified only if they have access to the project (`project:read`) and can at least read the execution (`execution:read`); recipients skipped this way are recorded and shown as a configuration warning. If no recipient remains, the organization's Owners and Admins are notified and a configuration warning is raised. No request is left without a recipient.

### 5. Escalation, acknowledgement and reminders
- Escalation policy: ordered levels `[{ after: duration, recipients }]`. Default: one level after 15 min to the organization's Owners and Admins. A request is *unattended* while it is neither acknowledged nor resolved; escalation timers are durable (`next_escalation_at`, fired by the scheduler with `FOR UPDATE SKIP LOCKED` through an engine-plane function).
- Each escalation notifies only the new level's recipients (no duplicates to earlier levels) and emits `execution.awaiting_human.escalated { level, recipients }`.
- For waits with a timeout (human tasks, AI tool approvals), escalation always happens before expiry: saving a routing configuration or publishing a flow whose escalation delay is greater than or equal to the wait's timeout is rejected; at run time, a level that would fall after expiry is brought forward to half of the remaining time.
- *Acknowledge* ("I am handling this") stops escalation; it requires the same permission as resolving and is audited.
- Reminders to current recipients while `OPEN`: default first after 60 min, interval doubling up to 24 h; configurable or disabled. Human tasks and approvals with a timeout get an "expires soon" notice (default 1 h before expiry, or 10 % of the timeout if shorter).

### 6. Payload and redaction
- Fields: organization (id, name), project (id, name), flow (id, name, version), node (id, name), affected items (indexes, capped, with a link), execution id, attention request id, cause, sanitized reason, created at, age, escalation level, acknowledged by, deep link, and `actions[]` — supplied by the cause as data: `{ id, label, permission, requiresRiskAcknowledgement }`, with action ids `CONTINUE`, `REPEAT`, `CANCEL`, `ACKNOWLEDGE` and the cause-specific outcomes of §8.
- For `UNCERTAIN_OPERATION` (SPEC-003 Q11 content), "what was sent": connector, operation, destination host, correlation key, item count, dispatch time, a masked summary of the request (subject to the data-inclusion rule below), and optional `manualCheck`: static text from the connector manifest explaining how to verify the outcome at the provider (no execution data; included in every channel). Its `actions[]` are the four supervised resolutions of ADR-0012 §2/§7 as defined by SPEC-003 Q11, plus *cancel*. For `HUMAN_TASK`: task title, form name, assignee; for AI tool approvals: tool name and argument names.
- Email and webhook bodies never contain credentials, secrets or protected execution data by default — only identifiers, names, counts and sanitized reasons (ADR-0049 §2). Including sent data is an optional per-project setting, off by default; enabling it requires both `notification:manage` and `execution:read-data` and emits `notification.data_inclusion_changed`; data is then included only for recipients (or the subscription's owning principal) holding `execution:read-data`, always masked (ADR-0017 §3), with AI redaction placeholders (ADR-0068), capped at 4 KiB. Credentials are never included in any form (owner decision, SPEC-004 N1).
- In-app, data is shown only to holders of `execution:read-data`; each view is audited (ADR-0049 §3).
- The deep link opens the authenticated execution view filtered to the request; it carries no token, secret or action. Following it without a session leads to login and changes nothing (SPEC-004 R-AZ-13). In V1 there are no actions directly from email or Slack; Slack buttons may come later, only for chat identities verified as linked to an Orcon user (owner decision, SPEC-004 N2).

### 7. Coalescing
- Group key `(org, flowId, cause)`. Per group and channel: if no message was sent within the window (default 60 s, configurable per scope), the message is sent immediately and the window opens; requests arriving inside the window are counted; when it closes, if the count is above zero, **one** grouped message is sent with the count, the oldest/newest times and a link to the inbox filtered by flow and cause; the next request after that starts a new cycle. The first notification is never delayed.
- Reminders and escalations are grouped by the same key.
- Email has a per-recipient cap (default 30 messages per hour); excess goes into an hourly digest.
- Inbox entries and public events remain one per request; `GROUPED` webhook delivery applies the same window to the subscription and includes the group key (usable as a deduplication key by on-call tools).

### 8. Actions
| Cause | Actions (owner wording → engine operation) | Permission |
|---|---|---|
| `UNCERTAIN_OPERATION` | *continue* (`CONTINUE`) → `RESOLVED_SUCCEEDED` with explicit risk acceptance; *repeat* (`REPEAT`) → `RESOLVED_FAILED` + new dispatch with `retryEpoch++`, only with explicit acknowledgement of a possible duplicate; *cancel* (`CANCEL`) → cancel execution. The full set of four supervised resolutions of ADR-0012 §2/§7 (SPEC-003 Q11), including *verify again* (`UNCERTAIN → VERIFYING`) and resolutions with attached evidence, stays on the evidence-based resolution operation | *continue*: `execution:resolve`; *repeat*: `execution:retry` **and** `execution:resolve`; *cancel*: `execution:cancel` |
| `POISON_TOKEN` | *repeat* → token RUNNABLE with `abandoned_count` reset; *cancel* | *repeat*: `execution:retry`; *cancel*: `execution:cancel` |
| `COMPENSATION_REVIEW` | *repeat* → retry the step; *continue* → skip with acknowledgement; *cancel* → abort | *repeat*: `execution:retry`; *continue*: `execution:resolve`; *cancel*: `execution:cancel` |
| `HUMAN_TASK` | the task's outcomes (submit form, approve, deny, edit arguments for AI tools) through the human-task operation; *cancel* | outcomes: `human-task:resolve` (AI approvals: interactive human session only, as SPEC-006 R-TL-05); *cancel*: `execution:cancel` |
| `AI_BUDGET_SUSPENDED` | *continue* → resume the run after the budget is raised or an override is granted; *cancel* | *continue*: `execution:resolve` (raising the budget itself needs the budget permission of ADR-0062); *cancel*: `execution:cancel` |
| `END_USER_WAIT` (when enabled) | *cancel*; the wait itself is continued by the end user | `execution:cancel` |
- API mapping (SPEC-005 ADR-0052 §6; architect decision delegated by the owner in SPEC-005 Q16): the operations *continue*, *repeat*, *cancel* and *acknowledge* on an item awaiting human action are the single entry point linked from notifications, public events and MCP; each item lists only the actions its cause allows and the caller is authorized for. Two specific operations remain: evidence-based resolution of UNCERTAIN operations (the four supervised resolutions with evidence) and completion of human tasks (forms, approvals); notifications for `UNCERTAIN_OPERATION` and `HUMAN_TASK` link to them, and MCP does not expose them in V1.
- *Acknowledge* requires any permission that allows one of the item's actions.
- Every action is an authenticated, authorized API call (ADR-0041), audited (ADR-0048) as `execution.human_action_taken`, and applied with a conditional update: the first committed action wins; a concurrent later action receives 409 with the current outcome.
- Every closing of a request (RESOLVED, EXPIRED, CANCELED) emits `execution.human_resolved { resolution: ACTION | TIMEOUT | CANCELLATION, action?, actor? }`. For AI tool approvals, `TIMEOUT` means "denied by timeout", which is the tool result returned to the model.
- Bulk actions in the inbox are allowed only for requests with the same cause and flow, require confirmation, and produce one audit entry per request.
- Signal URLs (ADR-0010 §8) are flow-level features chosen by the flow author; this ADR never uses them. SPEC-006's one-time approval link for public-chat runs (R-INJ-01a) only opens an authenticated approval page in Orcon; it never authorizes the action by itself (SPEC-004 R-AZ-13). The public-chat user can never approve their own sensitive actions; the approver must be a member of the organization with `human-task:resolve` (owner decision, SPEC-006 N1).

### 9. Inbox
- Per organization and project: "awaiting human action" list with filters (mine, my groups, all visible), sorting by age, columns cause, flow, step, age, level, acknowledged by, overdue flag; live counter via SSE. Keyboard operable, screen-reader labels, status never conveyed by color alone (WCAG 2.2 AA).

### 10. Metrics
- `created_at`, `acknowledged_at`, `resolved_at` (database clock) on the request. Exposed: histogram `orcon_attention_time_to_action_seconds{cause, resolution}`, histogram `orcon_attention_time_to_acknowledge_seconds{cause}`, gauge `orcon_attention_open{cause}`, gauge `orcon_attention_oldest_open_seconds`. Per-flow figures are served by the API from the table, not as metric labels (cardinality). Names are provisional until the observability ADR.
- Optional overdue threshold (`overdue_after_milliseconds`) per organization/project: an open request older than it is flagged overdue in the inbox and triggers an alert to admins.

### 11. Release gates
Added to ADR-0019: atomic creation for each cause on both stores; FI-17 and FI-18; latency targets of §2 under 1,000 concurrent suspensions; redaction scan with sentinel values over captured email and webhook bodies; escalation/reminder timing with a simulated clock; authorization and audit of every action; coalescing under bursts.

### 12. Isolation and retention
Organization-scoped tables carry `org_id` and follow ADR-0046 as amended by ADR-0077 (`attention_request`, `attention_routing`, `notification`, `notification_delivery`, `notification_group`, `operational_alert`). Alerts that belong to no organization are rows of the instance table `instance_alert` (ADR-0077 §3) and are visible only to instance admins. Requests are kept with the execution log (90 d); operational and instance alerts 90 d; `notification`, `notification_delivery` and `notification_group` rows 30 d. Channel, routing, escalation, reminder, coalescing, overdue and data-inclusion settings are audited configuration (`notification:manage`).

### 13. Operational alerts (not tied to one execution)
- ADR-0073 also covers situations that need an administrator but belong to no single execution. They use the same channels, deduplication, coalescing, escalation, acknowledgement and inbox (an "alerts" tab), and no execution actions. Closed set of kinds:
  - `CONNECTOR_REVOKED` — a `CRITICAL` revocation of a connector version (SPEC-003 N2); one `operational_alert` row per affected organization. Payload: connector, revoked version, severity, reason, fixed version, affected flows (id, name, project; capped with a link). Recipients, notified immediately and subject to escalation: the organization's Owners and Admins plus the owners of each affected project (owner decision, SPEC-003 N2). Event `connector.revoked` (public; SPEC-005 adds it to the ADR-0055 catalog). The alert closes when no active flow still uses the revoked version; acknowledging it stops escalation but does not close it. `ADVISORY` revocations raise no alert.
  - `STORAGE_RETENTION_PRESSURE` — ADR-0017 §5–6; an `instance_alert` row. Recipients: instance admins. Event `storage.retention_pressure`.
- A new kind requires amending this ADR.

### 14. Value casing
Every enumerated value this ADR defines is UPPER_SNAKE wherever it crosses a public interface — the API, public event data, the database and the flow document (ADR-0080 §3; ADR-0007 §11): causes, notification classes, request states, action ids, resolutions, message kinds, routing scopes, `purpose`, `deliveryMode`, `bodyFormat`, alert kinds and the per-node `attention.notify` values (`DEFAULT`, `ON`, `OFF`). Event types (`execution.awaiting_human`), permission names (`execution:resolve`) and values fixed by external standards (PagerDuty `trigger`/`resolve`, `dedup_key`) keep their own form.

## Consequences
- People learn within seconds that an execution needs them, through channels they already watch, without exposing secrets in email or chat tools.
- Bursts produce one grouped message per flow and cause instead of hundreds.
- Time-to-human-action becomes a measured number that operators can set targets on.
- Waits continued by end users stay quiet by default, so operators are not flooded; mandatory kinds cannot be silenced by a flow author.
- Depends on SPEC-004 (permissions, audit, data visibility) and SPEC-005 (public events, SSE, webhooks); its slice cannot start before those ADRs are Accepted. SPEC-006 slice AI4 depends on this ADR being Accepted.

## Alternatives considered
- Leaving notification to user-built flows triggered by `execution.suspended` (rejected: a notification path that depends on another execution can itself be stuck, and nothing guarantees escalation or measurement).
- Action links in emails that resolve without login (rejected by the owner: every action must be authenticated, authorized and audited).
- Delaying the first message to batch bursts (rejected: the first notice must be immediate).
- A separate notification service or message broker (rejected: single binary; ADR-0004 forbids mandatory services beyond the database).
- A nullable `org_id` on operational alerts for instance-level alerts (rejected by ADR-0077: breaks NOT NULL scoping and row-level security).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-005.** Cross-reference: "Programmatic surface (public events `execution.awaiting_human`, `execution.awaiting_human.escalated`, `execution.human_resolved`; REST operations to list items awaiting human action and to continue/repeat/cancel; MCP tools) is defined by ADR-0052 §3, ADR-0055 §2 and ADR-0059; this ADR defines the semantics, notification routing and escalation."
