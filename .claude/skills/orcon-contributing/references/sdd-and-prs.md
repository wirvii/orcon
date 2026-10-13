<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Specs and pull requests

Status: Accepted

This document defines how maintainer work tracked in mneme (backlog items, specs, slices and QA reports) maps to branches and pull requests. It is part of the `orcon-contributing` rulebook; the decision behind it is ADR 0081. Rule IDs (`SD-…`) are stable. The check that enforces a rule, when one exists, is named in brackets. Precedence: accepted ADRs (ADR 0081) > accepted `orcon-go` rules > this rulebook > reviewer preference.

## Rules

- **SD-1** Maintainers track work in mneme: backlog items, specs, plans, criteria and QA reports live outside the repository (only `.mneme/` configuration is committed). Pull requests are the only path to `main`.
- **SD-2** The pull request carries one line `Spec: SPEC-NNN` (trivial-lane specs too); outside contributions use `Spec: n/a`, and a maintainer may later link a backlog item. [`pr-advisory` spec]
- **SD-3** One slice of a spec = one branch (`<type>/spec-NNN-<slice>-<slug>`) = one pull request.
- **SD-4** QA reviews the pull request's commits (the review range mneme hands over) on the pull request head, before merge.
- **SD-5** Merge only when the QA report has no open finding the owner has not accepted; the owner moves the spec to done and merges; the spec's changes document records the pull request number and the squash commit.
- **SD-6** Never paste mneme reports into pull requests; summarize in English (PR-3).
- **SD-7** Release pull requests carry `Spec: n/a`; releases are recorded in mneme as session notes.
