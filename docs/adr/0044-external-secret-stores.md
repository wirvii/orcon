<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0044 — External secret stores

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12 (owner answers; consistency with ADR-0077)

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
Given in BL-004: Vault, AWS Secrets Manager, GCP Secret Manager, Azure Key Vault and Infisical at minimum, free (n8n parity).

## Decision
1. **Configuration**: per organization, `secret_store(org_id, id, alias, kind, configuration_ciphertext, cache_time_to_live_milliseconds, maximum_stale_milliseconds, status)`, unique `(org_id, alias)`; default aliases `vault`, `awsSecretsManager`, `gcpSecretsManager`, `azureKeyVault`, `infisical` (familiar to n8n users). Managed with `secret-store:manage`; referencing requires `secret-store:reference`.
2. **Providers**: Vault/OpenBao KV v1/v2 (auth: token, AppRole, Kubernetes, JWT/OIDC) via own REST client; AWS Secrets Manager (SDK v2; static keys or assumed role); GCP Secret Manager (service-account key or workload identity); Azure Key Vault secrets (client secret or managed identity); Infisical (universal-auth machine identity) via own REST client. The own clients avoid MPL-2.0 and BUSL SDK dependencies and serve OpenBao with the same code (owner decision, SPEC-004 Q15). Ambient cloud identity is instance-level; it is available to an organization only if an instance admin grants it to that organization explicitly, audited as `secret_store.ambient_identity_granted|revoked` (owner decision, SPEC-004 Q14). Ambient cloud identity also serves instance-level backup targets and the S3 blob backend (ADR 0083 §4).
3. **References**: `{{ $secrets.<alias>.<name> }}` (optionally `.<version>`) allowed only in credential fields; publication fails if found in node parameters or data expressions.
4. **Resolution and caching**: resolved at dispatch together with the credential; in-memory cache keyed `(org, alias, name, version)`; TTL default 5 min (0 disables); entries used within the last TTL are refreshed in the background before expiry; on provider failure the cached value is served up to the maximum stale age (default 1 h, 0 disables) and then a transient error (ADR-0011) is raised; values are never persisted; resolved values join the masking set.
5. **Network**: all store traffic goes through the egress client with the configured hosts.
6. **Observability**: `secret_store.resolution_failed` audited (coalesced), health shown per store.

## Consequences
Customers keep secrets in their own stores; no secret value is persisted by Orcon.

## Alternatives considered
Periodic sync into Orcon's encrypted store (rejected: duplicates secrets, stale copies); allowing `$secrets` anywhere (rejected: leaks into items and history); official Vault SDK `hashicorp/vault/api` (rejected: MPL-2.0 review burden for a small API surface); ambient cloud identity available to every organization (rejected: one organization could read the server's own secrets).

## Amendments
Changes applied on 2026-10-13 by specifications the owner accepted; each entry names its source.

- **SPEC-015 (2026-10-13).** §2: ambient cloud identity also serves instance-level backup targets and the S3 blob backend (ADR 0083 §4).
