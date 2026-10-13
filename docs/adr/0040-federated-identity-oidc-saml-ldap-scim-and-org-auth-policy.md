<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0040 — Federated identity (OIDC, SAML, LDAP/AD), SCIM provisioning and organization authentication policy

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12 (owner answers; consistency with ADR-0077); 2026-10-13 (stored enumerated values are UPPER_SNAKE, ADR-0015 §2)

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
S6: OIDC, SAML, LDAP/AD, SCIM and organization-wide enforcement of 2FA or SSO, all free. S3: organizations are isolation boundaries, and users may belong to several.

## Decision

### 1. Identity sources
Configured per organization in `authentication_source(org_id, id, kind: OIDC|SAML|LDAP, name, configuration_ciphertext, status)`; secrets in configuration (client secrets, bind passwords, SP keys) encrypted with the organization key.

### 2. OIDC
Authorization code + PKCE (S256); `state` and `nonce` bound to a pre-auth record; validation of `iss`, `aud`, `azp`, `exp`, `iat` (skew 2 min), signature with a per-source algorithm allowlist (default RS256, ES256, PS256, EdDSA; `none` and HS* with discovered keys rejected); JWKS cached and refreshed on unknown `kid` (rate-limited). Claims mapping: email (requires `email_verified=true` for linking by email), name, groups claim → group mapping. Libraries `coreos/go-oidc/v3` (Apache-2.0), `golang.org/x/oauth2` (BSD-3).

### 3. SAML 2.0
SP-initiated by default. IdP-initiated SSO is disabled by default; an organization may enable it per source, which shows a risk warning in the UI and is audited as `policy.changed` (owner decision, SPEC-004 Q10). Requirements: signed Response or Assertion (unsigned rejected); exactly one Assertion; XML signature wrapping defenses (signature references resolved against the element actually consumed); `Audience`, `Recipient`, `Destination`, `InResponseTo` (stored request id; not applicable to enabled IdP-initiated logins, which instead require a fresh assertion and replay rejection), `NotBefore/NotOnOrAfter` (skew 2 min); consumed assertion ids stored until expiry (replay rejection); SP metadata endpoint per source; SP signing/decryption key per source encrypted with the organization key. Library chosen in the implementing slice between `crewjam/saml` (BSD-2-Clause) and `russellhaering/gosaml2` (Apache-2.0) after a security review against known signature-wrapping CVEs. Single Logout out of V1.

### 4. LDAP / Active Directory
LDAPS or StartTLS required (plaintext only with `insecure: true`, audited); service-account search with a configurable filter, then bind as the user (passwords never stored by Orcon); filter values escaped per RFC 4515; group resolution via `memberOf` or AD `LDAP_MATCHING_RULE_IN_CHAIN`; periodic sync (default 1 h) suspends memberships of users no longer found or disabled. LDAP logins are `aal1`; organization 2FA policy applies on top (Orcon TOTP/passkey). Library `go-ldap/ldap/v3` (MIT).

### 5. SCIM 2.0
Endpoint `/scim/v2/orgs/{orgId}/` with `Users`, `Groups`, `ServiceProviderConfig`, `Schemas`, `ResourceTypes`; PATCH (including common IdP dialects), filter `userName eq` and `externalId eq`, pagination. Bearer tokens per organization (hashed, expiry, rotation). Effects are limited to the organization: create/link a membership (creating a global user only if none exists for the email; linking to an existing user only if the email domain is verified by that organization, otherwise an invitation is sent), set `active=false` or DELETE → membership suspended, sessions' access to the organization revoked, the user's service-account tokens and API keys in that organization disabled within 1 s. Group membership changes bump the organization's authorization epoch. Implemented in-house per RFC 7643/7644.

### 6. Linking and organization policy
- Linking: by `(source, issuer, subject)`; by email only with `email_verified` and the domain verified by the source's organization (`domain(org_id, id, name, verified_at, verification_token)`, DNS TXT `_orcon-challenge.<domain>`; one organization per domain per instance, enforced by a unique constraint on `name`). Otherwise the existing user must confirm the link by logging in.
- Home-realm discovery: email domain → organization's sources; organization login URL `/login/{orgSlug}`.
- JIT provisioning: optional per source, only for verified domains; default org role Member; group mappings assign roles.
- Organization policy `{ require2FA, requireSSO, acceptedAuthSources (default: local + the organization's own sources), trustIdpMfaClaim (default false), maxSessionAge, idleTimeout, enrollmentGrace (default 7 d), breakGlassUsers (≤ 2 Owners, passkey required, exempt from requireSSO) }` (owner decision, SPEC-004 Q5).
- Enforcement happens when an AuthContext for the organization is built: a session whose authentication source is not accepted, or whose AAL is insufficient, receives a re-authentication or step-up challenge. A session from organization A's IdP does not grant access to organization B unless B lists that source.
- When `require2FA` is enabled, members without a second factor are prompted at each access during the grace period and denied after it.

## Consequences
Organizations control their own sign-in without affecting other organizations; one global account per person.

## Alternatives considered
Instance-wide IdPs only (rejected: breaks per-organization SSO); trusting any instance session in every organization (rejected: a compromised IdP of one organization would reach others); Single Logout in V1 (deferred: SPEC-004 Q10).

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-009 K19 (2026-10-13, owner decision).** §1 (`authentication_source.kind`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
