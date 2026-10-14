<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Audited actions

Status: **Accepted** | Source: SPEC-004 Annex B (accepted 2026-10-12 via SPEC-004), with the actions added by the amendments of [ADR-0048](../adr/0048-tamper-evident-audit-log.md)

This page publishes the audited-actions coverage list of SPEC-004 Annex B. ADR-0048 §7 makes that annex normative, and [ADR-0052](../adr/0052-public-api-architecture-and-resource-model.md) refers to it for the audit event of each human action. This page adds no action and removes none; if it and Annex B disagree, Annex B wins.

## Event format
Every event uses the public format `orcon/audit@1` defined in [ADR-0048](../adr/0048-tamper-evident-audit-log.md) §1; that ADR is the single place where the fields are described, and this page does not repeat them. Points that matter when reading the tables:

- Every event carries the actor, the organization (or the instance), the target, the outcome, the reason, the request id, the IP address and the user agent, where they apply.
- `outcome` is `SUCCESS`, `FAILURE` or `DENIED`. A denied action is recorded in its own transaction (ADR-0048 §2). Annex B does not limit `DENIED` to particular actions, so this page does not either; `access.denied` records permission denials.
- `actor.kind` is `USER`, `SERVICE_ACCOUNT`, `SCIM_CLIENT`, `API_KEY` or `SYSTEM`. Enumerated values inside `details` are UPPER_SNAKE like every public Orcon value; action names and field names are identifiers and keep their form.
- Where an event records the request channel (`api.request`), `details.channel` is `API`, `CLI`, `SDK` or `MCP`, and `details.sdkLanguage` (a lowercase language identifier such as `typescript`, `go` or `python`) is present only when the channel is `SDK`.
- The `details` of an event never hold secrets or execution data.

## How to read the tables
Action names have the form `category.action`. Annex B writes several related actions together with `|` (for example `passkey.added|removed`); each table row below is one Annex B entry, written exactly as the annex writes it. The details column lists what Annex B says the event records. The notes column carries what Annex B states about the actor, the target, coalescing or the transaction of the event; an empty note means Annex B states nothing beyond the common fields above.

## Coverage by category

### auth

| Action | Details recorded | Notes |
|---|---|---|
| `login.succeeded` |  |  |
| `login.failed` | Identifier as an HMAC; reason |  |
| `logout` |  |  |
| `session.revoked` |  |  |
| `session.step_up` |  |  |
| `mfa.enrolled` |  |  |
| `mfa.removed` |  |  |
| `mfa.recovery_code_used` |  |  |
| `passkey.added\|removed` |  |  |
| `password.changed\|reset_requested\|reset_completed` |  |  |
| `lockout.triggered` |  |  |
| `sso.login` | Source |  |
| `sso.jit_provisioned` |  |  |
| `account.linked` |  |  |

### org/identity

| Action | Details recorded | Notes |
|---|---|---|
| `org.created\|suspended\|resumed\|deletion_scheduled\|deleted\|crypto_shredded` |  |  |
| `project.created\|updated\|deleted` |  | Includes a change of the project identifier. |
| `member.invited\|joined\|removed\|suspended\|role_changed` |  |  |
| `user.created` | Source (`LOCAL_COMMAND` until an instance user administration operation exists) | Instance audit log; actor `SYSTEM` for local commands. |
| `group.created\|deleted\|member_added\|member_removed` |  |  |
| `service_account.created\|deleted\|token_rotated` |  |  |
| `domain.verified\|removed` |  |  |
| `scim.token_created\|revoked` |  |  |
| `scim.user_provisioned\|deprovisioned` |  |  |
| `ldap.sync_completed` | Additions and removals |  |
| `instance_role.granted\|revoked` |  |  |
| `instance_admin.joined_org` |  |  |

### authz

| Action | Details recorded | Notes |
|---|---|---|
| `role.created\|updated\|deleted` |  |  |
| `role_binding.created\|deleted` |  |  |
| `credential.shared\|unshared` |  |  |
| `access.denied` | Permission; resource | Coalesced to one event per minute per principal and permission. |

### flow

| Action | Details recorded | Notes |
|---|---|---|
| `flow.created\|updated\|deleted\|restored\|purged\|moved\|imported\|exported` | `updated`: draft | `updated` is coalesced over 5 minutes. `deleted` is a soft delete, recoverable for 30 days. `restored` is a recovery inside that window. `purged` is the physical deletion when the window expires; its actor is the `SYSTEM` principal (SPEC-009 P3, DL-5). |
| `flow.published` | Version; `contentHash` |  |
| `flow.rolled_back` |  |  |
| `flow.activated\|deactivated` |  |  |
| `test_session.opened` |  |  |
| `flow.pin_changed` | Operation `CREATE`, `REPLACE` or `DELETE`; node; origin `MANUAL`, `EXECUTION`, `TEST_RUN` or `N8N_PIN_DATA`. No items and no values. | Pinned data of a node (SPEC-014 R-ED-PIN, ADR-0082 §3). Added by Enmienda 5 (from SPEC-014, R-AUD-10). When the origin is an execution, `execution.data_exported` with destination `EDITOR_PIN` is also emitted in the same transaction. |

### credential

| Action | Details recorded | Notes |
|---|---|---|
| `credential.created\|updated\|deleted\|tested` | `updated`: changed fields, never values |  |
| `credential.used` | Execution; node; number of requests; hosts | One event per node execution and credential. |
| `credential.oauth_connected\|refreshed\|needs_reauth` |  | `refreshed` is coalesced over 1 hour. |
| `secret_store.created\|updated\|deleted\|resolution_failed` |  |  |
| `secret_store.ambient_identity_granted\|revoked` |  | From an Instance Admin to an organization. |

### execution

| Action | Details recorded | Notes |
|---|---|---|
| `execution.data_viewed` |  | Coalesced per session and execution over 15 minutes. |
| `execution.data_exported` |  |  |
| `execution.retried\|cancelled\|deleted` |  |  |
| `execution.uncertain_resolved` | Evidence; risk acceptance |  |
| `execution.human_action_taken` | Action `CONTINUE`, `REPEAT`, `CANCEL`, `ACKNOWLEDGE` or `RESOLVE`; step; origin `APP` or `NOTIFICATION_LINK`; time since the first notice. No execution data. | Defined by ADR-0073. Inserted in the same transaction as the execution transition the human action causes (ADR-0048 §2). |
| `execution.effect_decided` | Decision `DISPATCH`, `AUTO_DISPATCH`, `SUBSTITUTE_PIN`, `SUBSTITUTE_MOCK` or `SUBSTITUTE_EMPTY`; preview; node; number of items. With `AUTO_DISPATCH` also `authorizedBy`: rule `R-ED-EFX-05`, the `autoConfirmNodeIds` setting of the run plan and the id of the test run. No execution data, no simulated responses and no intended requests. | Decision on an effect preview in a `TEST` execution (SPEC-014 R-ED-EFX, ADR-0082 §4). Added by Enmienda 5 and 5b (from SPEC-014, R-AUD-10). With `AUTO_DISPATCH` the actor is the principal that started the run. Written in the same transaction as the preview transition. |

### notification

| Action | Details recorded | Notes |
|---|---|---|
| `notification.channel_created\|updated\|deleted` | Channel type; masked destination; changed fields. Never channel secrets. | Defined by ADR-0073. |
| `notification.routing_changed` | Recipients, escalation and deadline, before and after | Defined by ADR-0073. |
| `notification.data_inclusion_changed` | Whether data inclusion in the notices of a project was turned on or off (decision N1) | Defined by ADR-0073. The actor holds `notification:manage` and `execution:read-data`. |

### config/keys

| Action | Details recorded | Notes |
|---|---|---|
| `policy.changed` | Before and after | Includes turning on IdP-initiated SSO and `members.canCreateProjects`. |
| `key.master_rotated\|org_rotated\|rotation_completed` |  |  |
| `key.verification_failed` |  |  |
| `key.backup_acknowledged` |  |  |
| `connector.installed\|granted\|revoked_override` |  |  |
| `trust_policy.changed` |  |  |
| `host_node_policy.changed` | Enabling; command and path lists; host label. Marks `shellAllowed` when the list includes a command interpreter. | Nodes that touch the host (SPEC-003, decision N1). |
| `limits.changed` |  |  |
| `siem.sink_created\|updated\|deleted\|lagging` |  |  |
| `audit.verified\|integrity_violation\|pruned` |  |  |
| `sqlite.multi_org_warning` |  |  |
| `db.rls_check_failed` |  |  |

### retention

| Action | Details recorded | Notes |
|---|---|---|
| `retention.purged` | Quantities per class | One summary event per organization and pruning pass (DL-6 of `orcon-db`). |

## Actions added by the amendments of ADR-0048
The amendments of [ADR-0048](../adr/0048-tamper-evident-audit-log.md) record further actions that SPEC-004 Annex B takes in. They are listed here as that ADR writes them, with the same reading rules as above.

### Added by SPEC-005

| Action | Details recorded | Notes |
|---|---|---|
| `apikey.created\|rotated\|revoked\|expired\|used_during_overlap` |  |  |
| `api.request` | Request channel (`details.channel`; `details.sdkLanguage` when the channel is `SDK`) | Batched for key-authenticated calls (ADR-0056 §8). |
| `oauth_client.created\|rotated\|revoked` |  |  |
| `oauth.token_issued\|token_revoked` | Issuer and subject, when the token comes from federated identity |  |
| `workload_identity.created\|updated\|deleted` |  |  |
| `apikey.personal_keys_forbidden` | Number of revoked keys | Emitted when the organization switch is turned on. |
| `project.protection_changed` |  |  |
| `confirmation.requested\|approved\|rejected\|expired` |  |  |
| `git.binding_changed\|sync_applied\|sync_failed\|conflict_resolved\|secret_finding_overridden` |  |  |
| `event_subscription.created\|updated\|deleted\|disabled` |  |  |
| `webhook.redelivered` |  |  |

### Added by SPEC-006
ADR-0048 describes these actions without naming them, so this page lists them by description:

- changes to budgets, prices and the redaction or injection policy;
- tool approvals, denials and argument edits;
- evaluation gate overrides;
- conversion of a conversation into an evaluation case;
- reads of conversation content (coalesced per 15 minutes);
- changes to MCP servers;
- the start of a reindex.
