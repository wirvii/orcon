<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Credentials and egress

Status: Accepted

This file covers credential types, authentication schemes, OAuth and credential tests. Network permissions are in `manifest.md` (MF-5 to MF-7). Rule IDs (`CR-…`) are stable. Precedence is in `kinds.md`.

## Rules

- **CR-1** Connectors receive a credential handle; the host applies the scheme (ADR-0029 §6). Prefer a scheme of ADR-0030 §4 to any custom handling.
- **CR-2** One credential type per authentication method (schemes `API_KEY`, `OAUTH2`, …); credential type ids are lowerCamelCase identifiers (`apiKey`, `oauth2`).
- **CR-3** `credentialRawAccess` only for signing or non-HTTP protocols, with `authoring.json` `rawAccessReason`.
- **CR-4** OAuth 2.0: grant `PKCE` when the provider supports it, otherwise `AUTHORIZATION_CODE`, or `CLIENT_CREDENTIALS` for service accounts; the redirect is the instance's single `/oauth/callback`; scopes are the minimum for the operations offered, listed per operation in the documentation.
- **CR-5** Credential fields: secrets `sensitive: true` (write-only); the base URL of self-hosted services is a non-secret field used by `fromCredential`; allowed hosts derive from the manifest domains.
- **CR-6** Every credential type has `test`: a cheap read operation (current user, account) whose result never includes the secret.
- **CR-7** A 401 on a `READ` or `retrySafe` operation with OAuth lets the host refresh once and retry (ADR-0029 §6); never refresh inside connector code.
- **CR-8** `n8nTypes[]` lists every n8n credential type that maps to this one (`n8n-mapping.md`).
- **CR-9** Never log, echo or place a credential value in items, errors or test files.
