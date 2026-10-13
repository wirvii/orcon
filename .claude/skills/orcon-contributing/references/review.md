<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Review and merge

Status: Accepted

This document defines how `main` is protected, what a pull request needs before it merges, and how maintainers review. It is part of the `orcon-contributing` rulebook; the decision behind it is ADR 0081. Rule IDs (`RV-…`) are stable. The check that enforces a rule, when one exists, is named in brackets. Precedence: accepted ADRs (ADR 0081) > accepted `orcon-go` rules > this rulebook > reviewer preference.

## Rules

- **RV-1** `main` is protected by a ruleset with an empty bypass list: no direct pushes, force pushes or deletion, for everyone including the owner. Never ask for a bypass.
- **RV-2** Every required check is green and the branch is up to date with `main` before merge (list in `github-settings.md`).
- **RV-3** Approvals. While the owner is the only maintainer, `main` requires zero approvals: GitHub never lets an author approve their own pull request, and nobody may bypass the protection. Trigger: as soon as a second maintainer with write access exists, `main` requires one approval from a code owner, and `.github/CODEOWNERS` lists both maintainers (procedure in GH-9). An author never approves their own pull request.
- **RV-4** Every review conversation is resolved before merge.
- **RV-5** Squash merge only, by a maintainer; the commit title is the pull request title and the message is empty.
- **RV-6** Reviewer checklist: the change is authorized by an accepted document or an agreed issue; the template is complete and honest; tests and evidence match the change; the applicable skill checklists are really satisfied; size is justified; no AI authorship or tool attribution; security and data-protection impact considered; documentation and translations updated.
- **RV-7** For work tracked in mneme, the spec's QA report has no open finding that the owner has not explicitly accepted (SD-5). While approvals are zero (RV-3), this record is the documented review.
- **RV-8** Outside contributions: workflow runs from forks need maintainer approval; no workflow in this repository uses `pull_request_target` or exposes secrets to pull request code.
- **RV-9** New commits dismiss earlier approvals.
- **RV-10** A broken `main` is fixed with a revert pull request under the same rules.
