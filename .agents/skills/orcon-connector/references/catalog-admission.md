<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Catalog admission

Status: Proposed — pending owner approval

This file covers how a connector is admitted: the automated checks, human review and seals (ADR-0033). Rule IDs (`CA-…`) are stable. Precedence is in `kinds.md`.

## Rules

- **CA-1** Automated admission = `orcon connector lint --admission`, `docs --check`, the cassette detector, `test --faults`, the conformance suite and `diff --verify-bump` against the previous version.
- **CA-2** [official] Official connectors pass `make connectors-check` and `make connectors-version-check` in CI and get the `OFFICIAL` seal from the release pipeline.
- **CA-3** Third-party submissions go through the catalog service (ADR-0033 §9): same automated stage, then human review; seal `VERIFIED` only after approval.
- **CA-4** License: SPDX, redistributable; converter output keeps its original licence and attribution (ADR-0035).
- **CA-5** Permissions match the declared domains and scopes; reviewers check them against the provider documentation.
- **CA-6** Safety contracts are checked against provider documentation; `proofLevel` `VERIFIED` only when demonstrated.
- **CA-7** Icon, i18n and documentation rules pass.
- **CA-8** Rejections cite rule ids.
- **CA-9** Reviewers read new cassettes for personal data that automatic detection can miss.
- **CA-10** Revocation follows ADR-0033 §6.
