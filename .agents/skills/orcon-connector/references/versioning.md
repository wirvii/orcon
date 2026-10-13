<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Versioning

Status: Accepted

This file covers semantic versioning per connector, how a change is classified, change descriptors and the changelog (ADR-0034 as amended by ADR-0079 §6). Rule IDs (`VR-…`) are stable. Precedence is in `kinds.md`.

## Rules

- **VR-1** Semantic versioning per connector; the first catalog release is `1.0.0`.
- **VR-2** Flows pin `{ version, digest }`; the editor offers upgrades with the changelog and the change diff (ADR-0034).
- **VR-3** Classification (ADR-0079 §6; `orcon connector diff --classify` prints the value of the second column):

  | Change | Bump |
  |---|---|
  | no difference in manifest or schemas (body fix, docs, tests, icon) | `PATCH` |
  | add node, operation, optional parameter, credential type, output port, option value; wider permissions; new trigger | `MINOR` |
  | remove or rename anything; change a type; add a required parameter without default; remove an option value; weaker safety level; narrower accepted values | `MAJOR` |

- **VR-4** `orcon connector diff --verify-bump` rejects a mismatch in either direction.
- **VR-5** Every version bump has `changes/<from>..<to>.json` covering the diff; majors list migration steps.
- **VR-6** Every version has a changelog entry written for users, in English and Spanish.
- **VR-7** Old versions stay in the catalog; retirement follows ADR-0034 §6.
- **VR-8** Wider permissions need a new admin approval (ADR-0033 §7); say so in the changelog.
