<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Branches

Status: Accepted

This document defines how branches are named, based, updated and removed. It is part of the `orcon-contributing` rulebook; the decision behind it is ADR 0081. Rule IDs (`BR-…`) are stable. The check that enforces a rule, when one exists, is named in brackets. Precedence: accepted ADRs (ADR 0081) > accepted `orcon-go` rules > this rulebook > reviewer preference.

## Rules

- **BR-1** Name `<type>/<slug>`: `type` is a commit type (CM-2); `slug` is lowercase kebab-case `[a-z0-9]+(-[a-z0-9]+)*`; the whole name has at most 60 characters. Examples: `feat/spec-007-design-system`, `fix/execution-timeout`. [`pr-advisory` branch]
- **BR-2** Work tracked in mneme starts the slug with `spec-NNN-`, optionally followed by the slice id: `feat/spec-013-c4-prcheck`.
- **BR-3** One branch, one pull request. Open the pull request (draft is fine) on the first push; aim to merge within five working days; branches idle for 30 days may be closed.
- **BR-4** The base is always `main`. No long-lived integration branches; no release branches before 1.0 (RL-9).
- **BR-5** Update by rebasing onto `origin/main` (or GitHub's "Update branch"). Force-push only your own branch, with `--force-with-lease`.
- **BR-6** Branches are deleted automatically after merge; never reuse a merged branch name.
- **BR-7** `release-please--branches--main` belongs to the release automation; nobody else pushes to it.
- **BR-8** Outside contributors may use any branch name in their fork; the advisory check only warns.
- **BR-9** When resolving rebase or merge conflicts, run `git diff origin/main...HEAD --diff-filter=D` before pushing; it must list no deletion outside the pull request's own scope. A conflict resolved in favor of the old side can silently revert merged work. Incident: #132 silently reverted merged work.
