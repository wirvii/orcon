<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Pull requests

Status: Proposed — pending owner approval

This document defines how a pull request is titled, described, sized and scoped, and which skill checklists it carries. It is part of the `orcon-contributing` rulebook; the decision behind it is ADR 0081. Rule IDs (`PR-…`) are stable. The check that enforces a rule, when one exists, is named in brackets. Precedence: accepted ADRs (ADR 0081) > accepted `orcon-go` rules > this rulebook > reviewer preference.

## Rules

- **PR-1** The title follows CM-1..CM-6 and becomes the commit on `main`. Editing it re-runs `commit-lint`.
- **PR-2** Keep every template section: `What and why`, the `Spec:` line, `How it was tested`, `Screenshots`, `Size justification`, `Breaking changes`, `Checklists`. Write `None` or `Not a UI change` instead of deleting a heading.
- **PR-3** The description is self-contained English: a reader without mneme, chat history or other tools must understand it. The spec id is a cross-reference, never the explanation.
- **PR-4** `How it was tested` lists the commands run, their results and known limitations. A link to CI alone is not evidence.
- **PR-5** Any visible change in `apps/web` or `packages/js/ui` attaches before/after screenshots in light and dark themes (and a phone width when layout changes), without personal data, uploaded to the pull request, not committed (visual baselines excepted).
- **PR-6** Skill checklists. For every skill whose paths the pull request touches, tick `- [x] <skill>` in the template and paste the skill's checklist below it (`scripts/compose-pr-body.sh` does both). The map is identical to `SkillMap` in `tools/prcheck/skills.go` (a test enforces it):

| Skill | Paths |
|---|---|
| `orcon-ui` | `apps/web/**`, `packages/js/tokens/**`, `packages/js/ui/**`, `packages/js/lint-config/**` |
| `orcon-web` | `apps/web/**`, `packages/js/**` |
| `orcon-go` | `**/*.go`, `**/go.mod`, `**/go.sum`, `go.work`, `.golangci.yml`, `sqlc.yaml` |
| `orcon-db` | `**/*.sql`, `**/migrations/**`, `internal/adapter/sqlite/**`, `internal/adapter/postgres/**` |
| `orcon-api` | `packages/openapi/**`, `internal/adapter/httpapi/**`, `internal/adapter/mcpserver/**`, `internal/adapter/cli/**`, `internal/event/**` |
| `orcon-connector` | `internal/adapter/connectors/**`, `packages/go/connector/**`, `internal/connector/**` |

  [`pr-advisory` skills]
- **PR-7** Size: soft limit of 400 changed lines (added plus deleted), counted by `tools/prcheck` against the merge base; binary files count zero; paths with the `pr-size-exclude` or `linguist-generated` attribute in `.gitattributes` are excluded. [`pr-advisory` size]
- **PR-8** Only these categories may be excluded: generated code, translation catalogs, screenshots and visual baselines, lockfiles and `go.sum`, generated skill copies, the release-please changelog. The pull request that creates such a path adds its `.gitattributes` line. Hand-written code is never excluded.
- **PR-9** Above the limit, write `## Size justification` explaining in at least one sentence why the change cannot be split. The advisory check warns; the reviewer may still ask for a split.
- **PR-10** One concern per pull request: refactors and mechanical renames go in their own pull request, separate from behavior changes.
- **PR-11** Open as draft until checks pass; mark ready when the template is complete.
- **PR-12** A new dependency states its license and the reason in `What and why` (`orcon-go` DEP, `orcon-web` DP rules).
- **PR-13** No secrets, credentials or personal data in the title, description, screenshots or pasted logs.
