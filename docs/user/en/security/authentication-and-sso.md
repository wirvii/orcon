---
title: Authentication and SSO
description: How people sign in to Orcon, and how an organization can require two-factor authentication or single sign-on.
sources:
  - ADR-0038
  - ADR-0039
  - ADR-0040
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Authentication and SSO

> This page describes behavior specified by ADR 0038, ADR 0039 and ADR 0040 that is not implemented yet. The slices that deliver sign-in and federated identity remove this note.

A person has one Orcon account for the whole instance, and can be a member of several organizations. Each organization decides how its members must prove who they are.

## Local sign-in

Local sign-in works with no external service, and also offline.

- **Passwords.** Stored with argon2id. The minimum length is 12 characters and there are no composition rules. Orcon checks against a local list of common passwords. A check against a breached-password service exists, but is off by default because it needs network access.
- **Protection against guessing.** After 5 failures on an account, the delay before the next attempt grows exponentially up to 15 minutes, and a limit applies to each address. Accounts are never locked permanently. The answer is the same for an unknown email, a wrong password and a disabled account.
- **Second factor.** A time-based one-time code (TOTP, the six-digit codes of authenticator apps) or a passkey. You also receive 10 single-use recovery codes. Regenerating them cancels the previous set.
- **Passkeys.** Sign in without a password with a passkey that requires user verification. A passkey sign-in counts as two-factor.
- **Email flows.** Email verification and password reset use single-use links valid for 30 minutes. If no mail server is configured, an instance administrator can generate a reset link, and that action is audited.
- **Lost second factor.** An instance administrator runs `orcon user reset-2fa <email>`. The action is audited and it ends the user's sessions.

## Sessions

Sessions are kept on the server, so they can be ended at once. The cookie is `__Host-orcon_session`. By default a session ends after 8 hours without activity or after 7 days in total; an organization may require shorter times. Changing a password, resetting the second factor, removing a person from an organization or suspending a membership ends the affected sessions. State-changing requests need a matching origin and a CSRF token.

## Single sign-on for an organization

Each organization configures its own identity sources, with a secret stored encrypted under the organization's key.

| Source | Notes |
| --- | --- |
| OIDC | Authorization code with PKCE. The ID token is validated, including issuer, audience, expiry and signature with an allowed algorithm. |
| SAML 2.0 | Starts at Orcon by default. Sign-in that starts at the identity provider is off by default; turning it on shows a risk warning and is audited. Assertions must be signed, are accepted once only and are checked against replays. Single logout is not part of version 1. |
| LDAP and Active Directory | Requires LDAPS or StartTLS. Orcon binds as the user and never stores the password. Group membership is synchronized every hour by default. |
| SCIM 2.0 | Provisioning at `/scim/v2/orgs/{orgId}/`, with a bearer token for each organization. Deactivating a user ends their access to the organization and disables their API keys there within one second. |

Orcon links an external identity to an account by the identity itself, or by email only when the email is verified and the domain is verified by the organization (a DNS TXT record). Users can be created on first sign-in only for verified domains, with the Member role by default. An organization login page is at `/login/<orgSlug>`.

## Organization policy

An organization can set:

- `require2FA`: members need a second factor. During a grace period (7 days by default) members without one are prompted at each visit, and after it they are denied.
- `requireSSO` and `acceptedAuthSources`: which sources are accepted.
- `trustIdpMfaClaim` (off by default), `maxSessionAge` and `idleTimeout`.
- `breakGlassUsers`: up to two Owners with a passkey, who are exempt from `requireSSO` so that a failure of the identity provider does not lock everyone out.

A session from one organization's identity provider does not give access to another organization unless that organization accepts the source.
