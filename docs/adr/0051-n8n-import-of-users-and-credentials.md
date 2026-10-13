<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0051 — n8n import of users and credentials (without secret values)

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
Adoption from n8n (product principle 7, `n8n-compatibility.md`: "credentials mapping without exporting secrets"). BL-004 asks for user and credential import without secret values.

## Decision
1. **Users**: `orcon n8n import-users --n8n-url --n8n-api-key --org` reads the n8n public API user list (email, name, role, pending status) using the n8n owner's API key and creates invitations; passwords, hashes and second factors are never imported (owner decision, SPEC-004 Q16).
2. **Role mapping**: n8n instance owner → Org Owner; admin → Org Admin; member → Member; n8n project admin/editor/viewer → Project Admin/Editor/Viewer; n8n personal projects → Orcon projects named "Personal — <name>" with the user as Project Admin. Unmapped roles are reported, never approximated.
3. **Credentials**: from workflow exports (and the n8n credentials list when the API exposes it) Orcon creates placeholder credentials (`status = needs_secret`) with name, mapped type (SPEC-003 R-IMP-02), owning project and shares where known; no secret values are read or stored. Flows referencing placeholders stay drafts until completed.
4. **External secrets**: n8n `{{ $secrets.<provider>.<name> }}` references are kept when a store with the same alias exists; otherwise reported.
5. **Report**: a machine- and human-readable import report lists users, mappings, placeholders and pending actions.

## Consequences
Teams move with their structure intact and re-enter secrets once, on Orcon's write-only path.

## Alternatives considered
Reading the n8n database or decrypting n8n credential exports with the n8n encryption key (rejected: couples to n8n internals and handles secrets in bulk).
