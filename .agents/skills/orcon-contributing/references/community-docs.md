<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Community documents

Status: Proposed — pending owner approval

This document defines the community files of the repository (contributing guide, security policy, code of conduct, pull request and issue templates) and how they are written. It is part of the `orcon-contributing` rulebook; the decision behind it is ADR 0081. Rule IDs (`CD-…`) are stable. The check that enforces a rule, when one exists, is named in brackets. Precedence: accepted ADRs (ADR 0081) > accepted `orcon-go` rules > this rulebook > reviewer preference.

## Rules

- **CD-1** Files: `CONTRIBUTING.md` (summary of this skill), `SECURITY.md`, `CODE_OF_CONDUCT.md`, `.github/pull_request_template.md`, `.github/ISSUE_TEMPLATE/`.
- **CD-2** `CONTRIBUTING.md` summarizes and links to these references; it never restates a rule in detail.
- **CD-3** Security reports go only through GitHub private vulnerability reporting. Targets: acknowledge within 5 business days, initial assessment within 10, coordinated disclosure within 90 days by default; reporters are credited unless they decline. Supported versions: the latest minor release before 1.0.
- **CD-4** The Code of Conduct is the Contributor Covenant 2.1 text, unchanged except the contact method, which is `info@wirvii.com`; maintainers enforce it and keep reports confidential.
- **CD-5** Issue forms: bug report and feature request; blank issues disabled; the chooser links to private vulnerability reporting.
- **CD-6** Labels: `bug`, `enhancement`, `needs-triage`; `autorelease: pending` and `autorelease: tagged` belong to the release automation.
- **CD-7** Neutral wording ("contributors", "maintainers"); English; no names of the AI tools used to build Orcon; mneme is named where it explains how maintainers track work.
- **CD-8** Changes to these files follow the normal pull request flow (`docs(docs)`, or `docs(security)` for `SECURITY.md`).
