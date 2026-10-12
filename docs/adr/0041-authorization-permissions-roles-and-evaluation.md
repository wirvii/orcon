<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0041 — Authorization: permission catalog, roles, sharing, evaluation and caching

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12 (owner answers; consistency with ADR-0077)

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
S4: predefined organization roles (Owner, Admin, Member) and project roles (Project Admin, Editor, Operator, Viewer), custom roles composed from a fine-grained catalog in V1, credential sharing as use-without-read, and execution data visibility separate from execution metadata.

## Decision

### 1. Model
Role-based access control with scopes instance → organization → project → resource. Bindings: `role_binding(org_id, id, scope_kind: org|project, project_id?, subject_kind: user|group|service_account, application_user_id?, org_group_id?, service_account_id?, role_id, created_by_principal_id?)` with a check constraint that exactly the subject column named by `subject_kind` is set. Groups: `org_group(org_id, id, name, source)` + `org_group_member(org_id, id, org_group_id, application_user_id)`, unique `(org_id, org_group_id, application_user_id)`.

### 2. Permission catalog (version 1; source `internal/authz/permissions.yaml`, generated code and docs)
Organization scope: `org:read`, `org:update`, `org:delete`, `member:read`, `member:invite`, `member:remove`, `member:update-role`, `group:manage`, `role:read`, `role:manage`, `project:create`, `project:read-all`, `project:delete`, `auth-source:manage`, `scim:manage`, `domain:manage`, `security-policy:manage`, `service-account:manage`, `audit:read`, `audit:export`, `siem:manage`, `secret-store:manage`, `secret-store:reference`, `connector:approve`, `key:rotate-org`, `limits:read`, `notification:manage` (human-attention notification channels, recipients and escalation rules, ADR-0073), `apikey:manage-org` (SPEC-005).
Project scope: `project:read`, `project:update`, `project:manage-members`, `flow:read`, `flow:create`, `flow:update`, `flow:delete`, `flow:move`, `flow:publish`, `flow:activate`, `flow:rollback`, `flow:execute` (manual/test runs, sub-flow calls into the project), `flow:export`, `flow:import`, `flow:submit-form`, `execution:read`, `execution:read-data`, `execution:retry`, `execution:cancel`, `execution:resolve` (UNCERTAIN operations, supervised acceptances), `execution:delete`, `human-task:resolve`, `credential:read`, `credential:create`, `credential:update`, `credential:delete`, `credential:use`, `credential:share`, `variable:read`, `variable:manage`.
Instance scope (ADR-0038 §4): `instance:orgs`, `instance:keys`, `instance:connectors`, `instance:policy`, `instance:limits`, `instance:siem`, `instance:users`.
The catalog fits in 128 bits; a new permission requires a catalog version bump and a migration that decides its grant in predefined roles. Permission names are identifiers and keep their `resource:action` form.

### 3. Predefined roles (immutable)
- **Owner**: all organization and project permissions in every project, including `org:delete` and Owner transfer.
- **Admin**: Owner minus `org:delete` and Owner transfer.
- **Member**: `org:read`, `member:read`, `role:read`, `project:create` (granted by default; organization policy `members.canCreateProjects=false` removes it — SPEC-004 Q3), `secret-store:reference`; project access only through project bindings.
- **Project Admin**: all project permissions in that project.
- **Editor**: `project:read`, all `flow:*` except `flow:move`, `execution:*` except `execution:delete`, `human-task:resolve`, `credential:read|create|update|delete|use|share` on credentials owned by the project, `variable:*`.
- **Operator**: `project:read`, `flow:read`, `flow:activate`, `execution:read`, `execution:retry`, `execution:cancel`, `human-task:resolve`; no `execution:read-data` by default (SPEC-004 Q2); no edit. Organizations that want operators to see data create a custom role; the documentation includes the example "Operator with data" = Operator + `execution:read-data`.
- **Viewer**: `project:read`, `flow:read`, `execution:read`, `credential:read`, `variable:read`.

### 4. Custom roles
`role(org_id, id, name, scope_kind, permission_bitset, is_builtin)` (custom roles have `is_builtin = false`). Creating, updating or binding a role requires that every included permission is held by the actor in the same scope (no escalation). Instance configuration may disable custom roles or remove permissions from all roles (`authz.ceiling`), as tightening only.

### 5. Credential sharing
A credential is owned by one project. `credential_share(org_id, id, credential_id, project_id, granted_by_principal_id?)`, unique `(org_id, credential_id, project_id)`, gives the target project `credential:use` only. Shares never cross organizations (composite foreign keys). The dispatch path re-checks ownership or share at every dispatch (ADR-0043 §5).

### 6. Evaluation
`Authorize(ctx AuthContext, perm Permission, res ResourceRef) Decision{Allow bool, Reason}`. Effective permissions in project P = (org-role permissions that apply to all projects) ∪ (project bindings of the subject and its groups in P), intersected with the instance ceiling, and empty if organization policy is not satisfied. Deny by default; no explicit deny rules in V1. Resources are loaded within the organization scope and parent/child consistency is checked (an identifier from the request authorizes, the stored row operates); cross-organization misses return 404, in-organization denials 403.

### 7. Caching and revocation
- Per process, per `(principal, org)`: compiled bitsets per project, tagged with `org.authorization_epoch`.
- Every change to roles, bindings, groups, memberships, shares or org policy increments `org.authorization_epoch` in the same transaction and issues `pg_notify('orcon_authz', orgId)`; processes LISTEN and also poll epochs every 1 s. Revocation is effective cluster-wide within 1 s; SQLite: in-process, immediate.
- Sensitive operations (credential create/update/share, role and binding changes, key and policy operations) re-validate against the database inside their own transaction.
- Target: ≤ 2 µs per check with a warm cache.

### 8. Endpoint declaration
Every HTTP route declares its required permission and resource resolver; CI fails if a route lacks a declaration. Engine operations performed on behalf of a person (resolve UNCERTAIN, human tasks, retries) use the same evaluator. This includes human actions started from human-attention notifications (ADR-0073): the action is performed by an authenticated Orcon principal holding the permission for that action in the execution's project; a link delivered by email or webhook never authorizes an action by itself.

### 9. Out of V1
Attribute-based rules, explicit deny, time-bound grants, flow sharing across projects (SPEC-004 Q4). Moving a flow between projects of the same organization is allowed and requires `flow:move` on the source project and `flow:create` on the target; moves never cross organizations.

## Consequences
Fine-grained and predictable; the Operator role and execution-data separation exist from V1; revocation latency is bounded and measured.

## Alternatives considered
Policy engine such as OPA/Cedar (rejected for V1: extra runtime and evaluation cost for a closed catalog); per-request database evaluation (rejected: cost on every request); a polymorphic `(subject, subject_id)` pair in bindings (rejected by ADR-0077: no foreign key can protect it).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-005.** §2: permission catalog **version 2** adds `apikey:create-personal`, `event-subscription:manage` and `mcp:use` (organization scope) and `git:manage` and `git:sync` (project scope), granted in predefined roles per SPEC-005 Q14: `apikey:create-personal` and `mcp:use` to Owner, Admin and Member; `event-subscription:manage` to Owner and Admin; `git:manage` to Project Admin; `git:sync` to Project Admin and Editor. §6: for `apiKey` principals, evaluation adds the step scope ∩ permissions of `boundTo`. §7: key changes increment `authorization_epoch`. §8: the per-route permission declaration is the OpenAPI extension `x-orcon-permission`.
