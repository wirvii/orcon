<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0059 — Built-in MCP server for administering Orcon

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: owner answers Q1–Q18, human-attention surface, cross-spec consistency; ADR-0080 §4 `x-orcon-mcp` levels and enum form; ADR-0077 schema rules)

Accepted by the owner on 2026-10-12 via SPEC-005.

## Context
P4: a built-in MCP server on top of the same public API lets assistants create and edit drafts, test, inspect executions, explain errors and retry; it authenticates with P2 scoped keys and is audited as that key; publishing to production requires human confirmation by default (configurable per organization); tests prove it can never bypass permissions. Principle 8: permissions and approvals are enforced outside model prompts. ADR-0028 §3 covers a different feature (MCP server **trigger**, BL-006).

## Decision

### 1. Endpoint and transports
- `/api/v1/mcp` on the `api` role, MCP Streamable HTTP transport; the supported protocol revision is pinned per Orcon release and documented. Session state is persisted (`mcp_session`) so any `api` process can serve it (no sticky routing, as ADR-0028 §3).
- `orcon mcp serve --stdio`: local bridge for stdio-only clients, forwarding to the remote endpoint with the context's credentials.
- Library: the official MCP Go SDK if its license is permissive (confirmed in the implementing slice); otherwise an in-repo implementation of the subset used.

### 2. Tool catalog (amended by ADR-0080 §4)
Every operation declares `x-orcon-mcp: { level, tool?, description? }` with `level` ∈ `none` (not exposed), `read` (no state change: `GET`, or the `validate`/`evaluate` actions), `write` (changes Orcon state, no external effect and recoverable), `destructive` (deletes, publishes, activates, deactivates, runs, retries or resolves human attention — anything with external effects or irreversible loss). The key `confirm` no longer exists: confirmation is derived from `level: destructive` (§5). Tools are generated from the operations whose level is not `none`; each tool maps to exactly one operation and reuses its request schema; tool names are snake_case identifiers and unique. Every `DELETE` that is exposed is `destructive`. V1 classification: `read` — `list_projects`, `search_flows`, `get_flow`, `get_draft`, `validate_draft`, `list_node_types`, `get_node_type`, `list_credentials` (metadata only), `list_executions`, `get_execution`, `get_execution_events`, `explain_execution_error`, `list_awaiting_human`; `write` — `create_flow`, `update_draft` (expected revision required; JSON Patch), `test_run_draft` (ADR-0024 §6), `cancel_execution`; `destructive` — `run_flow`, `retry_execution`, `publish_flow`, `activate_flow`, `deactivate_flow`, `continue_awaiting_human`, `repeat_awaiting_human`, `cancel_awaiting_human`. The last three plus `list_awaiting_human` map one-to-one to the human-attention operations of ADR-0052 §6, with the same permissions, `Idempotency-Key` and expected `revision`. Resources: the `orcon/flow@1` JSON Schema and node type documents. No deletion tool exists in v1; any deletion tool added later is `destructive` (CI check on the annotations). Not exposed in v1 (`level: none`): evidence-based resolution of UNCERTAIN operations (`external-operations/{externalOperationId}/resolve`), human-task forms, credential values, keys, roles, members, project settings (including `protected`), Git, policy, instance operations. Exposing a new tool needs an accepted spec.

### 3. Same pipeline, no parallel logic
Each tool call is dispatched in process through the API router (ADR-0052 §4) as a request carrying the caller's `AuthContext`, with channel `mcp`; authentication, rate limits, authorization (ADR-0041), isolation (ADR-0046), idempotency, preconditions and audit are the same code paths. A CI check fails if any MCP handler calls store, engine or authz functions directly instead of the router.

### 4. Authentication and audit
Bearer API keys or OAuth tokens (ADR-0056); the organization permission `mcp:use` is required in addition to each operation's permission. OAuth protected-resource metadata is published (RFC 9728). Interactive OAuth for MCP clients is enabled when an authorization-code flow for users exists. Every call is audited as the key (`api.request`, channel `mcp`, client name/version, tool, `operationId`).

### 5. Human confirmation
- "Production" means a project with `protected = true` (owner decision Q11). Every new project is created protected (including each organization's default project); holders of `project:update` can change it. Lowering `protected` from `true` to `false` requested by any `apiKey` principal, on any channel, always creates a confirmation request regardless of the policy below (otherwise a key could lift protection and then act without confirmation); raising it never needs confirmation. Changes are audited as `project.protection_changed`.
- Organization policy `mcp.confirmation`: `PROTECTED_ONLY` (default: tools of level `destructive` — publish/activate/deactivate, `run_flow`, `retry_execution`, any deletion, and the human-attention continue/repeat/cancel — on projects with `protected = true`; owner decision Q17, which confirms the Q11 wording "publishing, deleting or executing requires approval"; `test_run_draft` and `cancel_execution` are `write` and never need confirmation), `ALWAYS` (every `destructive` tool, any project), `NEVER`. Configuration may extend the same policy to every `apiKey` principal (`apiKeys.confirmation`) and require `aal2` for approvals; instance floors apply (ADR-0038 §8).
- A confirm-required call creates `confirmation_request(org_id, id, project_id, operation_name, request_reference, request_entity_tag, requested_by_principal_id?, requester_identity, channel, status: pending|approved|rejected|expired|executed, expires_at, decided_by_principal_id?, decided_at)` (default expiry 24 h; `operation_name` is the OpenAPI `operationId`; API status values `PENDING`, `APPROVED`, `REJECTED`, `EXPIRED`, `EXECUTED`), emits `confirmation.requested`, and returns a structured "pending confirmation" result with the URL.
- Approval requires a **person** with an interactive session (ADR-0038 §5; never an `apiKey` principal) holding the permission of the underlying operation in that project — one click in the editor. The approve operation declares `x-orcon-cli.none` with category `interactive-session-only` and `x-orcon-mcp.level: none`. On approval the stored request is executed through the pipeline with the original requester as actor and the approver recorded; preconditions are re-checked (a draft changed after the request cannot be published by an old approval; a human-attention item resolved meanwhile makes the approval fail with `attention-already-resolved`).
- Confirmation inside the MCP client (e.g. elicitation) is never treated as approval.

### 6. Outputs
`explain_execution_error` is deterministic server logic (failing node, error class, attempts, last error sanitized per ADR-0049, links); Orcon calls no model. Outputs are size-capped (default 32 KiB) with truncation markers and links; execution data only with `execution:read-data` (otherwise `dataHidden: true`), labelled as untrusted content.

### 7. Permission-bypass tests (release gate, part of the ADR-0050 suite)
For every tool: (a) missing operation permission → same denial as REST; (b) missing `mcp:use` → denied; (c) another organization → not found, including in RLS-only mode; (d) project-restricted key cannot reach other projects; (e) publish/activate/deactivate, `run_flow`, `retry_execution` and continue/repeat/cancel of a human-attention item on a protected project never take effect without human approval, and every tool annotated as a deletion is `destructive`; (f) approval attempts by any `apiKey` principal fail with `403`; (g) revoked/expired keys rejected mid-session within 1 s; (h) tool catalog equals the operations whose `x-orcon-mcp.level` is not `none`, and confirmation applies exactly to `destructive`; (i) a new project is protected by default, and lowering `protected` with an `apiKey` principal never takes effect without human approval. SQLite and PostgreSQL.

### 8. Logical schema additions (ADR-0015, as amended by ADR-0077)
`confirmation_request`, `mcp_session(org_id, id, requester_identity, started_by_principal_id?, client_name, client_version, protocol_version, holder, expires_at)` (`requester_identity` names the API key or OAuth token so its revocation closes the session).

## Consequences
Assistants get a capable, auditable interface whose power equals the key they hold; production changes stay behind server-side human approval by default.

## Alternatives considered
Tools implemented against the store (duplicates authorization); client-side confirmation only (client not trusted); exposing every operation as a tool in v1 (larger blast radius, worse tool selection; more tools can be annotated later without code); a per-tool `confirm` flag independent of the effect level (rejected by ADR-0080: confirmation derives from `destructive`).
