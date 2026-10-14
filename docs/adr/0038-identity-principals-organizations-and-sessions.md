<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0038 — Identity, principals, organizations and sessions

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12 (owner answers; consistency with ADR-0077, including its value-spelling decision); 2026-10-13 (stored enumerated values are UPPER_SNAKE, ADR-0015 §2); 2026-10-13 (SPEC-004 Amendment 7)

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
BL-004 S3 requires multi-organization isolation from V1; S4 requires organization- and project-level roles; S6 requires several authentication methods with organization-wide enforcement; S1 requires every capability to be open and stricter policy to be configuration only. SPEC-001 reserved `project_id` only. SPEC-005 will add API keys and must plug into the same model. ADR-0077 fixes the database naming, actor and value-spelling rules this ADR follows.

## Decision

### 1. Hierarchy
Instance → Organization → Project → resources (flows, credentials, variables, executions, triggers, blobs, events). Every customer-data row carries `org_id NOT NULL`; project resources also carry `project_id`. A single-organization install creates organization `default` at first start; the UI hides organization selection until a second organization exists.
Each project has a `handle` unique per instance (owner decision, SPEC-004 Q11): the project slug by default, or `<orgSlug>-<projectSlug>` when the slug is taken; collision messages are generic and never reveal another organization's flows, projects or name. The handle is the `<project>` segment of native webhook addresses (ADR-0022 §1). An organization may optionally have its own ingress host.

### 2. Users and memberships
- `application_user(id, principal_id, email_normalized, display_name, status: ACTIVE|DISABLED, password_hash?, created_at, …)` is an instance table (`user` is a reserved word); `email_normalized` and `principal_id` are unique. `DISABLED` is set by an instance administrator holding `instance:users`: it blocks every authentication method and revokes the account's sessions as deprovisioning does (§5); memberships are kept.
- `membership(org_id, id, application_user_id, status: INVITED|ACTIVE|SUSPENDED, invited_by_principal_id?, created_at)`, unique `(org_id, application_user_id)`; a member's organization roles are `role_binding` rows with `scope_kind = ORG` and `subject_kind = USER` (ADR-0041 §1), created together with the membership when an invitation is accepted; a user without an active membership cannot observe the organization or its existence.
- `external_identity(org_id, id, application_user_id, authentication_source_id, issuer, subject)`, unique `(org_id, authentication_source_id, issuer, subject)`; identity sources belong to an organization (ADR-0040 §1), so the link does too.

### 3. Principals
`user`, `serviceAccount` (organization-scoped, non-interactive), `scimClient` (organization-scoped token), `system:<role>` (engine processes; attributed in audit). ADR-0056 defines the `apiKey` principal (API keys and OAuth access tokens of service-account clients), always bound to a user or service account and limited to a subset of its permissions.
In the database (ADR-0077 §8), whoever acts is one row of the instance table `principal(id, kind, created_at)` with `kind` in `USER`, `SERVICE_ACCOUNT`, `SYSTEM` — the spelling the API exposes (ADR-0077 decision 11) — enforced by `CONSTRAINT ck_principal_kind CHECK (kind IN ('USER', 'SERVICE_ACCOUNT', 'SYSTEM'))`, and exactly one `SYSTEM` row per installation (partial unique index `uq_principal_kind … WHERE kind = 'SYSTEM'`; engine processes, the role goes into audit details). `application_user` and `service_account` reference it through a unique `principal_id` with `ON DELETE CASCADE`. Every column that records who did something is `<verb>_by_principal_id`, nullable, a single-column foreign key to `principal(id)` with `ON DELETE SET NULL` and a supporting index; deleting a principal therefore never deletes history, and the audit log keeps the actor's identity (ADR-0048). API keys and OAuth tokens write the principal they are bound to into actor columns (the audit record keeps the key); SCIM clients write no actor columns and are identified in the audit record.

### 4. Instance administration
Instance roles `instanceOwner` and `instanceAdmin` (table `instance_role_binding`), separate from organization roles (owner decision, SPEC-004 Q1), govern: master key operations, connector installation and trust policy, organization lifecycle, policy floors, per-organization limits, instance SIEM sinks, ambient-cloud-identity grants. They grant no access to organization data; joining an organization requires an explicit membership, audited as `instance_admin.joined_org`. In a single-organization install the first user receives `instanceOwner` and org `Owner`.

### 5. Sessions
- Server-side; 256-bit random token in cookie `__Host-orcon_session` (`Secure`, `HttpOnly`, `SameSite=Lax`, path `/`); stored as SHA-256 in `session(id, token_hash UNIQUE, application_user_id, created_at, last_seen_at, expires_at, assurance_level, authentication_source_id?, authentication_method_references, identity_provider_session_reference?, ip_address, user_agent, revoked_at)` (instance table; `authentication_source_id` is a reference to an organization's source recorded as an external id because the session itself is instance-level).
- Defaults: idle 8 h, absolute 7 d; organizations may require shorter values, enforced when entering their context.
- Token rotated at login and at step-up. State-changing requests require a matching `Origin` and a double-submit CSRF token.
- Revocation triggers: password change, MFA reset, deprovisioning, membership or organization suspension. Revocations bump a session epoch distributed by `pg_notify('orcon_authz')` with a 1 s poll fallback (ADR-0041 §7); SQLite: in-process broadcast.
- Org-level revocation by an Org Admin removes the session's access to that organization only.

### 6. Authenticator chain and AuthContext
`type Authenticator interface { Authenticate(r *http.Request) (Principal, Assurance, error) }`, evaluated in order: session cookie, SCIM bearer (only on SCIM routes), and later API keys (SPEC-005). The result plus the organization selected by route or header (`Orcon-Org`, validated against memberships) forms `AuthContext{Principal, OrgID, Assurance{AAL, AuthSource, AMR, AuthTime}}`. Organization policy (ADR-0040 §6) is applied when the AuthContext is built; failure yields a step-up or source-specific re-authentication challenge, never partial access.

### 7. Organization lifecycle
Create (instance admins; self-service only if `orgs.selfService=true`), suspend (triggers stopped, sessions denied, data kept), delete: the organization enters status `DELETION_SCHEDULED` with `deletion_scheduled_at` and stays suspended for a 30-day grace period during which an instance admin may cancel the deletion; afterwards its organization keys are destroyed (ADR-0042 §7) and its rows and blobs are purged in bounded batches. The grace period is a lifecycle status, not soft delete in the sense of ADR-0077 §4: no table gains `deleted_at`. A default project is created with each organization.

### 8. Policy as configuration
All security settings are configuration with three layers: engine defaults < instance floors < organization settings. An organization may only tighten relative to instance floors; an instance setting marked `locked: true` is read only from the configuration file or environment and read-only in UI and API. No capability is compiled out or license-gated: every security capability ships in the Apache-2.0 engine. A commercial platform built around Orcon may only tighten policy through this same configuration; there are no withheld capabilities to unlock.

## Consequences
One identity model serves all methods and SPEC-005; organizations are hard boundaries; instance operators are separated from tenant data; deleting a person or service account never erases who did what; a principal kind is spelled the same way in the database and in the API.

## Alternatives considered
Users owned by organizations (rejected: duplicates accounts for consultants working for several organizations); first organization's Owner as instance admin (rejected: mixes planes); stateless JWT sessions (rejected: no immediate revocation); actor columns without foreign keys (rejected by ADR-0077: loses integrity); lowercase stored principal kinds translated by the adapter (rejected by ADR-0077 decision 11: the API spelling is stored directly).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-005.** §2: a project gains `environment` (optional label) and `protected` (boolean; default `true` for every new project, including the default project of each organization; column `is_protected`), changed with `project:update` and audited as `project.protection_changed`; lowering `protected` at the request of an `apiKey` principal always goes through confirmation (ADR-0059 §5). §8: new configurable policies: forbid personal keys; maximum lifetime of personal keys (default 30 days). §3: ADR-0056 defines the `apiKey` principal (API keys and OAuth access tokens of service-account clients), bound to a user or service account; the `service_account` row hangs from `principal` (ADR-0077 §8). §6: the authenticator chain includes `apiKeyAuthenticator` after the session cookie; the `OrgID` of an `apiKey` principal is fixed.
- **SPEC-009 K19 (2026-10-13, owner decision).** §2 and §7 (`membership.status`; organization status `DELETION_SCHEDULED`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
- **SPEC-004 Amendment 7 (2026-10-13, owner decision).** §2: `membership` no longer carries `org_role_id`; a member's organization roles are `role_binding` rows with `scope_kind = ORG` and `subject_kind = USER`, created with the membership when an invitation is accepted. The owner chose this model, which resolves the tension with ADR-0041 §1. `application_user.status` takes the values `ACTIVE|DISABLED`; `DISABLED` is set by an instance administrator holding `instance:users`, blocks every authentication method and revokes the account's sessions, and memberships are kept.
