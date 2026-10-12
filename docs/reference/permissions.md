<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Permission catalog

Status: **Accepted** | Catalog version: 2 | Source: ADR-0041 §2–§3 (accepted 2026-10-12 via SPEC-004) and its version 2 amendment (SPEC-005)

This page publishes the permission catalog and predefined roles of ADR-0041. The ADR is normative; if this page and the ADR disagree, the ADR wins. Once `internal/authz/permissions.yaml` exists, this page is generated from it (ADR-0041 §2) and must match it.

Permission names are identifiers and keep their `resource:action` form. A new permission requires a catalog version bump and a migration that decides its grant in predefined roles.

## Organization scope
| Permission | Note |
|---|---|
| `org:read` | |
| `org:update` | |
| `org:delete` | |
| `member:read` | |
| `member:invite` | |
| `member:remove` | |
| `member:update-role` | |
| `group:manage` | |
| `role:read` | |
| `role:manage` | |
| `project:create` | |
| `project:read-all` | |
| `project:delete` | |
| `auth-source:manage` | |
| `scim:manage` | |
| `domain:manage` | |
| `security-policy:manage` | |
| `service-account:manage` | |
| `audit:read` | |
| `audit:export` | |
| `siem:manage` | |
| `secret-store:manage` | |
| `secret-store:reference` | |
| `connector:approve` | |
| `key:rotate-org` | |
| `limits:read` | |
| `notification:manage` | Human-attention notification channels, recipients and escalation rules (ADR-0073). |
| `apikey:manage-org` | SPEC-005. |
| `apikey:create-personal` | Version 2 (SPEC-005). |
| `event-subscription:manage` | Version 2 (SPEC-005). |
| `mcp:use` | Version 2 (SPEC-005). |

## Project scope
| Permission | Note |
|---|---|
| `project:read` | |
| `project:update` | |
| `project:manage-members` | |
| `flow:read` | |
| `flow:create` | |
| `flow:update` | |
| `flow:delete` | |
| `flow:move` | |
| `flow:publish` | |
| `flow:activate` | |
| `flow:rollback` | |
| `flow:execute` | Manual and test runs, sub-flow calls into the project. |
| `flow:export` | |
| `flow:import` | |
| `flow:submit-form` | |
| `execution:read` | |
| `execution:read-data` | |
| `execution:retry` | |
| `execution:cancel` | |
| `execution:resolve` | `UNCERTAIN` operations, supervised acceptances. |
| `execution:delete` | |
| `human-task:resolve` | |
| `credential:read` | |
| `credential:create` | |
| `credential:update` | |
| `credential:delete` | |
| `credential:use` | |
| `credential:share` | |
| `variable:read` | |
| `variable:manage` | |
| `git:manage` | Version 2 (SPEC-005). |
| `git:sync` | Version 2 (SPEC-005). |

## Instance scope (ADR-0038 §4)
`instance:orgs`, `instance:keys`, `instance:connectors`, `instance:policy`, `instance:limits`, `instance:siem`, `instance:users`.

## Predefined roles (immutable)
| Role | Scope | Permissions |
|---|---|---|
| Owner | Organization | All organization and project permissions in every project, including `org:delete` and Owner transfer. |
| Admin | Organization | Owner minus `org:delete` and Owner transfer. |
| Member | Organization | `org:read`, `member:read`, `role:read`, `project:create` (granted by default; the organization policy `members.canCreateProjects=false` removes it), `secret-store:reference`; project access only through project bindings. |
| Project Admin | Project | All project permissions in that project. |
| Editor | Project | `project:read`, all `flow:*` except `flow:move`, `execution:*` except `execution:delete`, `human-task:resolve`, `credential:read`, `credential:create`, `credential:update`, `credential:delete`, `credential:use`, `credential:share` on credentials owned by the project, `variable:*`. |
| Operator | Project | `project:read`, `flow:read`, `flow:activate`, `execution:read`, `execution:retry`, `execution:cancel`, `human-task:resolve`; no `execution:read-data` by default; no edit. The documented custom role "Operator with data" is Operator + `execution:read-data`. |
| Viewer | Project | `project:read`, `flow:read`, `execution:read`, `credential:read`, `variable:read`. |

Version 2 grants (SPEC-005 Q14): `apikey:create-personal` and `mcp:use` to Owner, Admin and Member; `event-subscription:manage` to Owner and Admin; `git:manage` to Project Admin; `git:sync` to Project Admin and Editor.

Custom roles are composed from this catalog (ADR-0041 §4); an actor can only include permissions it holds in the same scope.
