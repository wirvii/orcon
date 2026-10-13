<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0048 — Tamper-evident audit log, SIEM export and retention

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12 (owner answers; consistency with ADR-0077, including UPPER_SNAKE values in the public format); 2026-10-13 (request channel and SDK language fields)

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
S5: coverage of logins, flow changes, credential changes and each credential use, role/permission changes, views of protected execution data, configuration and key changes; tamper-evident hash chain detecting deletion or alteration even directly in the database, with alerting; real-time SIEM export in standard formats; retention separate from execution history (default 1 year); batched writes to protect throughput.

## Decision

### 1. Event schema `orcon/audit@1`
`{ id, chain, seq, at, org_id?, actor{kind, id, display, auth_source, session_ref}, action, target{type, id, name?}, outcome, reason?, details{…no secrets, no execution data}, request{id, ip, user_agent}, prev_hash, hash }`, with `outcome` in `SUCCESS`, `FAILURE`, `DENIED` and `actor.kind` in `USER`, `SERVICE_ACCOUNT`, `SCIM_CLIENT`, `API_KEY`, `SYSTEM`. This is a public document format for SIEM consumers and the audit API; like every public Orcon format its enumerated values are UPPER_SNAKE (ADR-0080 §3, ADR-0077 decision 11), while action names (`login.succeeded`) and field names are identifiers and keep their form. Where an event records the request channel (`api.request`, ADR-0056 §8), `details.channel` is `API`, `CLI`, `SDK` or `MCP`, and the language of a generated SDK is a separate `details.sdkLanguage`, present only when `channel` is `SDK`: a lowercase language identifier (`typescript`, `go`, `python`) that keeps its form like other identifiers. Failed-login identifiers stored as HMAC with the instance key. In the database the actor is kept as `actor_principal_id` **without** a foreign key (the identity outlives a deleted principal; ADR-0077 §8), `actor_kind` (stored in the same UPPER_SNAKE spelling, so the document is stored and returned without translation) and the keyed hash of the visible identity.

### 2. Write path
- The acting transaction inserts the event into `audit_pending(org_id, id, occurred_at, body_document, body_document_format)` — atomic with the action — or, for instance-level events, into `instance_audit_pending(id, occurred_at, body_document, body_document_format)`. Denied actions are recorded in their own transaction.
- `credential.used` events are inserted in the confirmation-point transaction of the node execution (ADR-0013 §6), one per node execution and credential, with request count and target hosts (SPEC-004 Q8).
- `execution.human_action_taken` events are inserted in the same transaction as the execution transition the human action causes (ADR-0073).
- The sealer (scheduler role; one per chain via advisory lock; implemented as `orcon_engine.seal_audit`) takes pending events every ≤ 50 ms or 1,000 events, orders them by `(occurred_at, id)`, assigns `sequence_number`, computes `hash = SHA-256(previous_hash ‖ canonical JSON (RFC 8785) of the event without hash)`, writes `audit_event(org_id, id, sequence_number, occurred_at, actor_principal_id, actor_kind, action, outcome, body_document, body_document_format, previous_hash, hash)` rows and a block record `audit_block(org_id, id, first_sequence_number, last_sequence_number, head_hash, mac)` with `mac = HMAC-SHA-256(K_audit, chain ‖ last_sequence_number ‖ head_hash)`, where `K_audit` is derived (HKDF) from the chain's organization key (instance key for the instance chain), and deletes the pending rows — one transaction. The instance chain uses `instance_audit_event` and `instance_audit_block` with the same columns minus `org_id`.
- Chains: one per organization and one for the instance; organization chains live in tables with `org_id NOT NULL`, the instance chain in instance tables (ADR-0077 §5).

### 3. Verification and alerting
`orcon audit verify [--org]` and an hourly scheduler job check sequence continuity, every hash, every block MAC, and that the chain head is not behind the latest anchor. Any failure emits `audit.integrity_violation` on the instance chain, an error log, a metric, an admin banner and a SIEM event.

### 4. Anchors
Every minute the head `(chain, sequence number, hash, mac)` is appended to `<data dir>/audit-anchors.log` (always) and written to each configured SIEM sink (when present) (owner decision, SPEC-004 Q17). Without an external anchor, truncation of the newest entries by someone controlling the database and the data directory is not detectable; while a chain has no external anchor, a persistent warning is shown to the administrators responsible for it (instance admins for the instance chain; organization Owners and Admins for their organization's chain) until an external sink exists.

### 5. SIEM export
Sinks: syslog RFC 5424 over TLS (RFC 5425), TCP or UDP; OpenTelemetry Logs (OTLP/HTTP and gRPC); HTTPS webhook (JSON lines, batches ≤ 1 s or 500 events, `Orcon-Signature` HMAC-SHA-256); JSON-lines file. Delivery at least once using a cursor per sink and chain that records the last delivered sequence number (`audit_sink_cursor` with `org_id` for organization sinks; `instance_audit_sink_cursor` for instance sinks, whose reads across organization chains run in the engine plane); target lag p99 ≤ 5 s; lag over 60 s raises `siem.sink_lagging`. Organization sinks (`siem:manage`) receive their chain; instance sinks receive all chains. Network sinks use the egress client.

### 6. Retention
Default 365 days, configurable per organization above the instance floor. Pruning deletes whole blocks older than the retention and records a MAC'd prune checkpoint `(chain, last pruned sequence number, hash)` so verification continues from it. Independent from ADR-0017.

### 7. Coverage
SPEC-004 Annex B is normative, including the human-action and notification-configuration events required from ADR-0073; a test enumerates audited actions per route/command and fails if a route marked `audit: required` emits nothing. Human-attention notifications (ADR-0073) are not audit sinks and never replace audit events.

## Consequences
Every committed action has an audit record; alterations and deletions are detected; SIEMs receive events within seconds; the write cost is one insert in an existing transaction plus asynchronous sealing; consumers see the same value spelling as in the API.

## Alternatives considered
Synchronous chaining in each action transaction (rejected: global hot row); external append-only store as the only copy (rejected: mandatory service); signing each event with an asymmetric key (rejected: cost; block MACs plus anchors suffice); requiring an external sink before enabling multi-organization (rejected: SPEC-004 Q17, a persistent warning instead); one audit table with an optional `org_id` for both chains (rejected by ADR-0077); lowercase outcome and actor-kind values (rejected by ADR-0080 §3 as revised: one spelling for every public value).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-005.** SPEC-004 Annex B adds `apikey.created|rotated|revoked|expired|used_during_overlap`, `api.request` (with channel), `oauth_client.created|rotated|revoked`, `oauth.token_issued|token_revoked` (with issuer and subject when the token comes from federated identity), `workload_identity.created|updated|deleted`, `apikey.personal_keys_forbidden` (enabling the organization switch, with the number of revoked keys), `project.protection_changed`, `confirmation.requested|approved|rejected|expired`, `git.binding_changed|sync_applied|sync_failed|conflict_resolved|secret_finding_overridden`, `event_subscription.created|updated|deleted|disabled` and `webhook.redelivered`.
- **SPEC-006.** SPEC-004 Annex B adds the AI actions: changes to budgets, prices and redaction or injection policy; tool approvals, denials and argument edits; evaluation gate overrides; conversion of a conversation into an evaluation case; reads of conversation content (coalesced per 15 minutes); changes to MCP servers; start of a reindex.
- **Owner decision on the audit request channel (2026-10-13, SPEC-004 and SPEC-005).** §1 (`details.channel`, `details.sdkLanguage`): the request channel is `API`, `CLI`, `SDK` or `MCP`, and the SDK language is a separate lowercase identifier, `sdkLanguage`, present only when the channel is `SDK`. This replaces the form `api|cli|sdk-<lang>|mcp`, whose variable part could not follow the UPPER_SNAKE rule for public values.
