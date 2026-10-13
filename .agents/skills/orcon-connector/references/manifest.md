<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Manifest

Status: Proposed — pending owner approval

This file covers what an author writes in the `orcon/connector@1` manifest (ADR-0029 §4) beyond what schema validation already enforces. Rule IDs (`MF-…`) are stable. Precedence is in `kinds.md`.

## Rules

- **MF-1** Required fields (ADR-0029 §4) are complete; `license` is an SPDX expression; `source.repository` and `source.commit` are set for catalog submissions.
- **MF-2** `compat.engineApi` is the narrowest range the connector was tested with.
- **MF-3** Every operation declares `purity` and, for `EFFECT`, `safety` (`operation-safety.md`).
- **MF-4** Sensitive inputs and outputs are listed in `sensitiveFields`; parameters holding personal or secret data are `sensitive: true`.
- **MF-5** `permissions.network.domains` lists exact hosts from the provider's documentation; scheme `https`.
- **MF-6** Wildcards only as one leading label (`*.example.com`), only when the provider uses per-account subdomains; never over a public suffix.
- **MF-7** `http`, extra ports and `allowPrivateNetworks` only for self-hosted services, with `fromCredential` and `authoring.json` `networkReason`.
- **MF-8** `resources` are the measured needs of the heaviest operation, not round numbers.
- **MF-9** `changelog` has an entry for every version (`versioning.md`).
- **MF-10** `n8n.types[]` is filled when an n8n node exists (`n8n-mapping.md`).
- **MF-11** `ai.tool` hooks follow the AI design when it is accepted; never invent semantics here.
