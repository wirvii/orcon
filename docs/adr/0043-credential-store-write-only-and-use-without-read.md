<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0043 — Credential store: write-only secrets, versions, host binding and use-without-read

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12 (owner answers; consistency with ADR-0077); 2026-10-13 (stored enumerated values are UPPER_SNAKE, ADR-0015 §2)

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
S2: secrets write-only after save. S4: credentials shareable as use-without-read. ADR-0029 §6: connectors receive a handle.

## Decision
1. **Tables**: `credential(org_id, id, project_id, type, connector_reference, name, allowed_host_document, allowed_host_document_format, status: OK|NEEDS_SECRET|NEEDS_REAUTH|DISABLED, current_version_number, created_by_principal_id?, …)`; `credential_version(org_id, id, credential_id, version_number, public_field_document, public_field_document_format, secret_field_ciphertext, data_key_wrapped, org_key_version_number, created_at, created_by_principal_id?)`, unique `(org_id, credential_id, version_number)`. Documents are stored as `bytea`/`BLOB` and read whole; nothing is queried inside them (ADR-0077 §9, Go rulebook DA-3 as amended).
2. **Write-only**: secret fields (declared `sensitive` by the connector schema, plus OAuth tokens) are never returned by any API, export or UI to anyone; responses show `{set, updatedAt, updatedBy}`; updating replaces the value and creates a new version. Public fields readable with `credential:read`.
3. **Resolution**: at dispatch, the engine resolves `(credential, current version)` within the flow's organization, verifies that the credential is owned by or shared to the flow's project (every dispatch), decrypts in memory, and passes a handle to the connector (ADR-0029 §6). Values are not cached across executions beyond a per-process cache keyed by `(org, credential, version)` with 60 s TTL, zeroed on eviction.
4. **Host binding**: the allowed hosts default to the connector scheme's hosts; for the generic HTTP node the owner must set them. The egress client applies the credential only to requests whose host matches; otherwise the request fails with `CredentialHostNotAllowed`. Raw access (`credentialRawAccess`) bypasses binding and is approved per organization.
5. **Masking**: every resolved secret value (credential fields, external-store values, OAuth tokens) is added to the execution's masking set; exact matches are replaced in logs, error messages, events, history and human-attention notification payloads (ADR-0073).
6. **Testing**: credential tests run server-side; results carry status and a sanitized message.
7. **Use audit**: one `credential.used` event per node execution and credential, carrying the request count and the target hosts, written in the confirmation-point transaction (ADR-0048 §2; owner decision, SPEC-004 Q8).

## Consequences
Users with "use" cannot read or redirect a secret; even instance administrators cannot read secrets through Orcon.

## Alternatives considered
Reveal-on-demand for owners (rejected by S2); binding by connector type only (rejected: the generic HTTP node would leak); one audit event per HTTP request (rejected: millions of events in D20 S1, breaks the performance gate).

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-009 K19 (2026-10-13, owner decision).** §1 (`credential.status`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
