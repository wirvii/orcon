<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0052 — Public API architecture and resource model

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: owner answers Q1–Q18, human-attention surface, cross-spec consistency; ADR-0080 path, enum and reference-site rules; ADR-0077 schema rules)

Accepted by the owner on 2026-10-12 via SPEC-005.

## Context
Owner decision P1 (BL-005): Orcon exposes a single public REST API described by OpenAPI; the Orcon editor uses exactly this API, with no private or internal API, so everything doable in the UI is doable programmatically. ADR-0002 states that the `api` role serves the frontend and public APIs. `overview.md` lists API versioning as deferred. SPEC-001/002/003/004 each defined only the minimal endpoints they needed and deferred the full API design to this ADR. Identity, authorization and isolation are defined by ADR-0038, ADR-0041 and ADR-0046. ADR-0080 fixes the path grammar, the action-verb list, the enum form, the response shapes and the reference site.

## Decision

### 1. One management API
- All management operations are served under `/api/v1` by the `api` role (and `standalone`). The editor (`apps/web`), the CLI (ADR-0058), the SDKs (ADR-0054) and the MCP server (ADR-0059) are clients of it.
- Outside `/api/v1` the `api` role serves only: `GET /healthz`, `GET /readyz`, the embedded UI assets, the API reference site at `GET /api/reference` and `/api/reference/*` (ADR-0080 §10: same origin as the API, content security policy `default-src 'self'; connect-src 'self'`, no external resources, readable without authentication like the OpenAPI document), trigger ingress addresses (ADR-0022, ADR-0028), `/.well-known/*` metadata, and protocol endpoints whose shape is fixed by external standards (OIDC/SAML callbacks, SCIM, OAuth credential consent callback; ADR-0040, ADR-0045). Protocol endpoints are documented in the OpenAPI document under the `protocol` tag and excluded from the versioning guarantees of ADR-0054. A new route outside `/api/v1` needs an ADR.
- CI gate: the set of routes registered under `/api/` (excluding `/api/reference`) equals the set of operations in the OpenAPI document, and no handler writes a 2xx body outside its declared schema; the editor bundle performs HTTP only through the generated TypeScript SDK (lint rule forbidding direct `fetch`/`XMLHttpRequest` outside the SDK package).
- UI parity registry: every editor action that reads or changes server state declares the `operationId` it calls; a CI test fails when an action has no operation.

### 2. URL and representation rules (ADR-0080 §2, §3, §7)
- Path grammar: every path item declares `x-orcon-resource` (`collection | item | singleton | view | action | stream`). Segments are kebab-case; collections are plural nouns (an invariant noun declares `x-orcon-plural-exception`). Collections nest at most one level under their owner, only when the child cannot exist without it: `/api/v1/orgs/{orgId}/projects`, `/api/v1/projects/{projectId}/flows`, `/api/v1/projects/{projectId}/executions`. At most two path parameters.
- Individual resources have a flat canonical URL by globally unique id (`/api/v1/projects/{projectId}`, `/api/v1/flows/{flowId}`, `/api/v1/executions/{executionId}`, `/api/v1/api-keys/{apiKeyId}`), or sit under their parent when they are dependent sub-resources (`/flows/{flowId}/versions/{version}`, `/executions/{executionId}/awaiting-human/{attentionId}`).
- Path parameters are `<resource>Id` in camelCase with `format: uuid`, never `{id}`; `{version}` for version numbers.
- Relations that are not ownership use typed filters, never extra nesting: executions of a flow are listed with `GET /projects/{projectId}/executions?flowId=`.
- Non-CRUD actions are `POST <item or singleton>/<verb>` with one of the thirty verbs of ADR-0080 §2: `/flows/{flowId}/publish`, `/flows/{flowId}/activate`, `/flows/{flowId}/deactivate`, `/flows/{flowId}/rollback`, `/executions/{executionId}/cancel`, `/executions/{executionId}/retry`, `/api-keys/{apiKeyId}/rotate`, `/api-keys/{apiKeyId}/revoke`.
- Ids are opaque strings (UUIDs per ADR-0015). Field names are `camelCase`. Timestamps are RFC 3339 UTC; durations ISO-8601. Bodies are JSON; flow documents are `orcon/flow@1` verbatim.
- Enum and state values in requests, responses and event payloads are UPPER_SNAKE (ADR-0080 §3), e.g. `UNCERTAIN_OPERATION`, `GROUPED`, `PROTECTED_ONLY`, `SERVICE_ACCOUNT`. Identifier catalogs keep their own form and are marked `x-orcon-enum-kind: identifier`: event types, permission names, problem codes, `operationId`s, MCP tool names, media types and values fixed by external standards (OAuth, CloudEvents, PagerDuty). Value lists in the logical-schema descriptions of ADR-0052..0059 are storage values governed by the database rulebook (ADR-0077); adapters map them to the API form.
- Responses carry no envelope (ADR-0080 §7, ADR-0053 §9).
- Organization context: for interactive sessions, the organization comes from the route or the `Orcon-Org` header (ADR-0038 §6); for `apiKey` principals it is fixed to the key's organization and a different `Orcon-Org` yields 404.
- Visibility follows ADR-0041 §6: another organization or no read permission → `404`; read permission without the requested action → `403`. Identifiers from the request authorize; handlers operate on rows loaded within the organization scope and check parent/child consistency (SPEC-004 R-ISO-07).

### 3. V1 resource catalog
Paths below omit the `/api/v1` prefix.

| Area | Resources (collection → item → actions) |
|---|---|
| Tenancy and identity (ADR-0038/0040) | `orgs`, `orgs/{orgId}/projects` (projects carry `environment` and `protected`, default `true`; see ADR-0059 §5), `orgs/{orgId}/members`, `projects/{projectId}/members`, `orgs/{orgId}/groups`, `users/me`, `users/me/sessions`, `orgs/{orgId}/auth-sources`, `orgs/{orgId}/domains`, `orgs/{orgId}/security-policy`, `orgs/{orgId}/scim-tokens` |
| Authorization (ADR-0041) | `permissions` (catalog, read-only), `orgs/{orgId}/roles`, `orgs/{orgId}/role-bindings`, `credentials/{credentialId}/shares` |
| Programmatic identity (ADR-0056) | `orgs/{orgId}/service-accounts`, `api-keys`, `api-keys/{apiKeyId}/rotate`, `api-keys/{apiKeyId}/revoke`, `service-accounts/{serviceAccountId}/oauth-clients`, `service-accounts/{serviceAccountId}/workload-identity-trusts`; protocol-tagged, outside the grammar: `oauth/token` (client credentials and token exchange), `oauth/revoke`, `oauth/introspect` |
| Flows | `projects/{projectId}/flows`, `flows/{flowId}`, `flows/{flowId}/draft` (GET, PUT, PATCH), `flows/{flowId}/draft/validate`, `flows/{flowId}/draft/evaluate` (expression preview; was `draft/expression-preview`, renamed by ADR-0080 §2), `flows/{flowId}/versions`, `flows/{flowId}/versions/{version}`, `flows/{flowId}/publish|activate|deactivate|rollback|move`, `flows/{flowId}/export` (`format=ORCON|BPMN`), `projects/{projectId}/imports` (n8n import, long-running) |
| Testing (ADR-0024 §6) | `flows/{flowId}/test-sessions`, `flows/{flowId}/test-runs` |
| Executions | `projects/{projectId}/executions` (list; filter `flowId`), `flows/{flowId}/executions` (`POST` only: runs the flow; there is no list under the flow), `executions/{executionId}`, `executions/{executionId}/data` (`execution:read-data`), `executions/{executionId}/nodes`, `executions/{executionId}/events`, `executions/{executionId}/events/stream`, `executions/{executionId}/cancel|retry`, `external-operations/{externalOperationId}/resolve`, `human-tasks/{humanTaskId}/complete`, `waits/{waitId}/signal` |
| Human attention (semantics: ADR-0073) | `orgs/{orgId}/awaiting-human`, `projects/{projectId}/awaiting-human` (views; `execution:read`), `executions/{executionId}/awaiting-human/{attentionId}` (get), `executions/{executionId}/awaiting-human/{attentionId}/continue` (`execution:resolve`), `/repeat` (`execution:retry`; for `UNCERTAIN_OPERATION` also `execution:resolve`), `/cancel` (`execution:cancel`), `/acknowledge` — see §6. `awaiting-human` declares `x-orcon-plural-exception` |
| Notification settings and alerts (semantics: ADR-0073) | `orgs/{orgId}/attention-routing`, `projects/{projectId}/attention-routing`, `flows/{flowId}/attention-routing` (singletons; GET, PUT; `notification:manage`), `projects/{projectId}/notification-data-inclusion` (singleton; GET, PUT; PUT needs `notification:manage` and `execution:read-data`), `orgs/{orgId}/operational-alerts`, `operational-alerts/{operationalAlertId}/acknowledge` (visible to the alert's recipients per ADR-0073 §13), `instance/operational-alerts` (instance administrators) |
| Triggers | `flows/{flowId}/triggers`, `triggers/{triggerId}/health`, `projects/{projectId}/addresses` |
| Credentials and secrets (ADR-0043/0044/0045) | `projects/{projectId}/credentials` (secret fields write-only), `credential-types`, `credentials/{credentialId}/test`, `credentials/{credentialId}/authorize` (starts OAuth consent; was `credentials/{id}/oauth/authorize`, renamed by ADR-0080 §2), `orgs/{orgId}/secret-stores` |
| Variables | `projects/{projectId}/variables` |
| Connectors (ADR-0029/0033) | `catalog/connectors`, `connectors`, `connectors/{connectorId}/grants`, `connectors/{connectorId}/upgrade` |
| Git (ADR-0057) | `projects/{projectId}/git-binding`, `git-bindings/{gitBindingId}/status`, `git-bindings/{gitBindingId}/sync`, `git-bindings/{gitBindingId}/conflicts/{flowId}/resolve`; protocol-tagged, outside the grammar: `git-bindings/{gitBindingId}/provider-hook` |
| Events (ADR-0055) | `events`, `events/stream` (SSE), `orgs/{orgId}/event-subscriptions`, `event-subscriptions/{eventSubscriptionId}/deliveries`, `deliveries/{deliveryId}/redeliver`, `event-subscriptions/{eventSubscriptionId}/replay` |
| Async | `operations/{operationId}`, `operations/{operationId}/cancel` |
| Governance | `confirmations`, `confirmations/{confirmationId}/approve|reject`, `orgs/{orgId}/audit-log` (view; `x-orcon-plural-exception`), `orgs/{orgId}/siem-sinks`, `orgs/{orgId}/settings` |
| Instance (ADR-0038 §4) | `instance/orgs`, `instance/settings`, `instance/keys`, `instance/trust-policy`, `instance/siem-sinks`, `instance/operational-alerts` |
| MCP | `mcp` (MCP Streamable HTTP endpoint, ADR-0059; outside the path grammar) |

### 4. Request pipeline (every operation, in this order)
1. Pre-auth per-IP rate limit and body size limit.
2. Request id (`Orcon-Request-Id`, accepted from the client if well-formed, otherwise generated).
3. Authentication through the `Authenticator` chain (ADR-0038 §6: session cookie with CSRF check on unsafe methods, then API key/OAuth token per ADR-0056) → `AuthContext`.
4. Principal rate limit (ADR-0053 §6).
5. Stability gate (`experimental` operations require `Orcon-Experimental`).
6. Authorization: `Authorize(AuthContext, x-orcon-permission, resource)` (ADR-0041 §6, with the `apiKey` intersection of ADR-0056 §2).
7. Idempotency lookup (ADR-0053 §4) and precondition checks (ADR-0053 §5).
8. Handler (organization-scoped data access per ADR-0046; never holds a DB transaction across network calls).
9. Audit (ADR-0048: in the action's transaction for mutations; batched `api.request` for key-authenticated calls, ADR-0056 §8) and metrics.
Implemented once in `internal/adapter/httpapi`; the MCP server dispatches through the same pipeline (ADR-0059 §3).

### 5. Logical schema additions (ADR-0015, as amended by ADR-0077)
`api_operation` (ADR-0053 §7) and `idempotency_record` (ADR-0053 §4); identity, event, git and confirmation tables are listed in their ADRs. All carry `org_id NOT NULL` with composite foreign keys and forced RLS (ADR-0046), follow the database rulebook (names without abbreviations, `id` primary keys, actor columns `<verb>_by_principal_id` referencing `principal`, lists as child tables, JSON documents as `bytea`/`BLOB` read whole with no in-JSON queries, no views); instance-level operations use instance tables only. Project gains `environment` (optional label) and `protected` (boolean, default `true`; column `is_protected`) (amendment to ADR-0038 §2).

### 6. Human attention operations
ADR-0073 defines when an execution waits for a person, the reasons, notification routing, escalation and the exact effect of each action. This ADR fixes only the API contract:
- List items awaiting human action per organization (only projects the principal can read) or per project; filters `flowId`, `reason`, `escalated`, `since`; ADR-0053 pagination and sort. Item fields: `attentionId`, `executionId`, `projectId`, `flowId`, `flowVersion`, `nodeId`, `reason` (ADR-0073 enum, UPPER_SNAKE), `since`, `escalationLevel`, `availableActions` (the subset of `CONTINUE|REPEAT|CANCEL` the caller is authorized for), `revision` (ETag). Execution data only with `execution:read-data` (ADR-0049); never secrets.
- One `POST` operation per action, each declaring its permission per the cause/action table of ADR-0073 §8 that SPEC-004 R-AZ-13 adopts: `continue` → `execution:resolve`; `repeat` → `execution:retry`, plus `execution:resolve` when the cause is `UNCERTAIN_OPERATION` (a repeat may duplicate an external effect); `cancel` → `execution:cancel`; `acknowledge` (stops escalation) → any permission that allows one of the item's actions. Each item offers only the actions its cause allows. Only an authenticated Orcon principal (interactive session or `apiKey`) authorized by the ADR-0041 evaluator in the execution's project may call them: unauthenticated → `401`, no permission → `403`, other organization → `404`. A link received by email, webhook or SSE never authorizes an action by itself; there is no unauthenticated or secret-in-URL variant. `Idempotency-Key` and `If-Match` (the item's `revision`) are required: concurrent resolutions — first commit wins; the other gets `412`, or `409 attention-already-resolved` once resolved. Optional `note` (≤ 2 KiB) plus any acknowledgement fields ADR-0073 requires for `repeat`.
- Each action writes, in the transaction of the execution transition, the audit event `execution.human_action_taken` defined in SPEC-004 Annex B (action, step, origin `app|notification-link`, time since first notice, no execution data; actor and channel in the common `orcon/audit@1` fields) and the public event `execution.human_resolved` (ADR-0055 §2); key-authenticated calls also produce `api.request`.
- MCP exposure and confirmation: ADR-0059 §2, §5.
- `external-operations/{externalOperationId}/resolve` (the four supervised resolutions of an UNCERTAIN operation, with evidence) and `human-tasks/{humanTaskId}/complete` (forms and approvals) remain; notifications for `UNCERTAIN_OPERATION` and `HUMAN_TASK` link to them; MCP does not expose them in V1 (owner delegated this to the architect, SPEC-005 Q16; table in ADR-0073 §8).
- Notification settings and operational alerts (§3): routing per organization, project and flow (`notification:manage`); per-project data inclusion (`notification:manage` and `execution:read-data`; audited `notification.data_inclusion_changed`); list and acknowledge operational alerts, visible only to their recipients per ADR-0073 §13, with the affected-flow list filtered to projects the caller can read.

## Consequences
- The editor cannot drift from the API; third parties get the full product surface.
- Every new UI feature costs an OpenAPI change first (spec-first, ADR-0054).
- One pipeline means one place for authentication, authorization, isolation, auditing and limits; the MCP server and CLI inherit them.
- Every installation serves the reference for exactly the API it runs.

## Alternatives considered
- **Private editor API + smaller public API** (n8n-style): rejected by P1.
- **RPC (gRPC/Connect) as primary**: rejected; P1 mandates REST/OpenAPI; HTTP/JSON is the most accessible to scripts, low-code tools and assistants.
- **GraphQL**: rejected; per-field authorization and rate limiting are harder; P1 chose REST.
- **Fully nested item URLs**: rejected; long URLs and moves between projects break links; global ids plus server-side authorization suffice.
- **Lowercase enum values** (the first draft of this ADR): rejected by ADR-0080 §3; they differ from the engine states.
