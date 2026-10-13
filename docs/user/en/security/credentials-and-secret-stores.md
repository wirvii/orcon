---
title: Credentials and secret stores
description: How Orcon stores the credentials your flows use, why nobody can read them back, and how to keep secrets in an external store instead.
sources:
  - ADR-0043
  - ADR-0044
  - ADR-0045
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Credentials and secret stores

> This page describes behavior specified by ADR 0043, ADR 0044 and ADR 0045 that is not implemented yet. The slices that deliver the credential store remove this note.

A credential holds what a flow needs to call another service: a token, a password, an API key. Orcon keeps secrets so that they can be used, and never shown.

## Write-only

After you save a secret field, no screen, API answer or export returns it to anyone. Orcon shows only that the field is set, when it was updated and by whom. Updating a field replaces the value and creates a new version of the credential. Public fields, such as a username or a base address, can be read by users with the permission `credential:read`.

Not even instance administrators can read secrets through Orcon.

## Use without read

A project can use a credential without being able to read it. When a flow node runs, the engine checks that the credential belongs to the flow's project or has been shared with it, decrypts the value in memory, and gives the connector a handle. The check runs at every use. Values are kept in a per-process cache for at most 60 seconds, and wiped when they leave it.

## Bound to hosts

Each credential has a list of hosts it may be sent to. For a connector it defaults to the hosts of that service. For the generic HTTP node, the owner must set the hosts. Orcon applies the credential only to requests whose host matches; any other request fails with `CredentialHostNotAllowed`. Raw access to a credential, which skips this binding, must be approved for the organization.

## Masking

Every secret that a node resolves is added to the masking set of that execution. Exact matches are replaced in logs, error messages, events, history and notification payloads.

## Audit

Orcon records one `credential.used` event for each node execution and credential, with the number of requests and the target hosts. It does not record one event for each request.

## OAuth credentials

Orcon runs the consent flow (authorization code with PKCE where the provider supports it, or client credentials) and stores the tokens as secret fields. A token is refreshed when it expires within 60 seconds. Exactly one process in the cluster refreshes a given credential at a time, and the others wait for the result, so providers that rotate refresh tokens do not invalidate each other. If the provider answers `invalid_grant`, the credential moves to `NEEDS_REAUTH`, its owner is notified, and dispatches fail with an error that names the credential.

One risk remains and cannot be closed by Orcon: if the provider rotates the refresh token and the process crashes after the provider answered but before the new token is saved, the new refresh token is lost and you must give consent again.

## External secret stores

An organization can keep secrets in its own store and have Orcon read them when needed. Orcon persists no value that it reads this way.

| Store | Authentication |
| --- | --- |
| HashiCorp Vault or OpenBao (KV v1 and v2) | Token, AppRole, Kubernetes, JWT or OIDC |
| AWS Secrets Manager | Static keys or an assumed role |
| Google Cloud Secret Manager | Service-account key or workload identity |
| Azure Key Vault | Client secret or managed identity |
| Infisical | Machine identity (universal auth) |

Each store has an alias in the organization. The default aliases are `vault`, `awsSecretsManager`, `gcpSecretsManager`, `azureKeyVault` and `infisical`. Managing stores needs the permission `secret-store:manage`, and referencing them needs `secret-store:reference`.

Reference a secret in a credential field with `{{ $secrets.<alias>.<name> }}`, optionally followed by `.<version>`. You can use the reference only in credential fields. Publishing a flow fails if a reference appears in a node parameter or a data expression.

Values are cached in memory for 5 minutes by default (0 turns the cache off). If the store is unreachable, the cached value is served for up to one hour by default (0 turns this off), and then a transient error is raised. Resolved values join the masking set. A failed resolution is audited.

A server running in a cloud can authenticate with its own cloud identity. That identity belongs to the instance. An organization can use it only after an instance administrator grants it explicitly, and the grant is audited.
