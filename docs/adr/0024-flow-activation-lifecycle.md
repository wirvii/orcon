<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0024 — Flow activation lifecycle: draft, publish, activate, deactivate, rollback, deletion and test sessions

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-002.

## Context
T3: saving edits a draft and never touches production; drafts are testable via their own test URL and with sample or past events; publishing is separate and gated by validation; switching versions loses no events; one-click rollback; stable production URL; "unpublished changes" indicator. ADR-0007 (Proposed) defines draft/version/snapshot. ADR-0077 §4 makes flow deletion a soft delete with a 30-day recovery window and leaves trigger behaviour to this ADR.

## Decision

### 1. States
`flow.active_flow_version_id` (nullable) and `flow.activation_state ∈ {ACTIVE, INACTIVE}` (ADR-0080 §3). Publishing creates version N+1; if the flow is active it becomes live in the same transaction. Activate/deactivate switch all automatic triggers on/off without creating a version (owner decision SPEC-002 Q6). `hasUnpublishedChanges = contentHash(draft) ≠ contentHash(active version)`.

### 2. Publication transaction
Validation (ADR-0007 §9) plus trigger rules: T-V1 address collisions (ADR-0022 §3, without detail when the owner is another organization); T-V2 schedule rules and time zone valid; T-V3 credentials present for `POLL`/`LISTEN`/`APP_EVENT`; T-V4 imported n8n addresses unchanged unless edited explicitly; T-V5 `RESPOND_NODE` mode with no reachable respond node → warning; T-V6 local-file trigger enabled on the instance and its path within the operator's allowed paths (ADR-0026 §2); T-V7 callers policy valid; T-V8 trigger declaration valid (ADR-0020 §3); T-V9 deduplication window within 1 minute – 90 days (ADR-0021 §3). Then, in one transaction: insert the version, set `active_flow_version_id`, upsert `trigger` rows (diffed by `configuration_hash`), rebuild address rows, adjust schedule/poll/listener/registration desired state, emit `flow.published` and lifecycle-trigger executions, and `pg_notify('orcon_routes')`.

### 3. Hot switch
Each event pins the version read in its acceptance transaction. Queued and running executions keep their version. Trigger state is preserved when `configuration_hash` is unchanged: schedules keep `last_fired_slot_at`; polls keep their cursor; listeners keep the open connection (subsequent events pin the new version without reconnecting). When changed: schedules recompute from now and never re-fire a recorded slot; polls follow the connector's rule (keep or reset; shown at publication); listeners restart gracefully (stop consuming, finish acknowledging persisted messages, reopen).

### 4. Rollback
Republish the content of an earlier version as N+1 (ADR-0007 §6), through the same transaction.

### 5. Deactivation
Sets `INACTIVE`; addresses answer as inactive; schedules stop; polls and listeners release; registrations are removed (ADR-0027). Running executions continue.

### 6. Test sessions
The editor opens a test session for the draft: snapshot created (ADR-0007 §6), test addresses active until `expires_at` (default 120 s, maximum 10 min) or `maximum_event_count` (default 1). For non-HTTP families a test session runs one schedule fire now, one poll (without advancing the production cursor), or a temporary listener (only when the source supports a non-destructive consumer; otherwise the editor offers sample data). Test executions use mode `TEST`, a separate deduplication namespace and no production cap.
"Run with sample/past event": starts a test execution of the draft with given JSON or a retained `ingress_event` (masked fields remain masked, with a warning).

### 7. Imports
n8n imports land as drafts, inactive (D7b).

### 8. Deletion (ADR-0077 §4)
Deleting a flow is a soft delete. In the transaction that sets `deleted_at` and `deleted_by_principal_id`, the flow's automatic triggers are switched off exactly as in §5 (addresses answer as inactive, schedules stop, polls and listeners release, registrations get `desired_state = ABSENT`). Running executions continue on their version; new automatic executions are refused. Restoring the flow within 30 days leaves it `INACTIVE`; triggers are never reactivated automatically and a person activates the flow explicitly. When retention purges the flow after 30 days, its trigger rows are deleted with it.

## Consequences
Production never runs unpublished content; version switches are atomic and observable; a deleted flow stops receiving events at once and can be recovered without surprises.

## Alternatives considered
Publish implies activate (n8n 2.x style; rejected by the owner, SPEC-002 Q6). Pausing ingress during switches (rejected: loses or delays events). Reactivating triggers on restore (rejected: a restored flow could start running unattended).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-004.** Publishing requires `flow:publish` and activating requires `flow:activate`; rule T-V3 verifies credential sharing.
