<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Provenance

Status: Accepted

This document defines where the owner's provenance rule applies in the contribution workflow. The check itself, its term list and its exempt paths are defined by the `orcon-go` skill (`references/tooling.md`) and are not repeated here. It is part of the `orcon-contributing` rulebook; the decision behind it is ADR 0081. Rule IDs (`PV-…`) are stable. The check that enforces a rule, when one exists, is named in brackets. Precedence: accepted ADRs (ADR 0081) > accepted `orcon-go` rules > this rulebook > reviewer preference.

## Rules

- **PV-1** Owner rule: code comments, commit messages, pull request titles and descriptions, release notes and documentation never name the AI tools used to build Orcon and never carry AI authorship trailers (an AI co-author trailer, a "Generated with" line, an assistant session trailer). The term list is `tools/provenance/terms.go`; it is not repeated here. [`provenance`, `commit-msg` hook]
- **PV-2** mneme is the project's tool for backlog, specs, plans, QA reports and memory; it is named openly, and `Spec: SPEC-NNN` lines are encouraged.
- **PV-3** Exempt configuration paths and product AI paths are defined by `orcon-go` (`tooling.md`, `.provenance-allow`).
- **PV-4** Because the squash message is empty, only the pull request title reaches `main`, and it already passed `commit-lint` and `provenance`; release notes are built from those titles.
- **PV-5** Attribution text that an editor or environment adds automatically is removed before committing or opening a pull request, even when the environment asks for it.
