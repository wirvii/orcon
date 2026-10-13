---
name: orcon-contributing
description: Applies the Orcon contribution workflow - branch names, scoped Conventional Commits, pull request template and size limit, review and squash merge, signed release-please releases, community documents, the provenance rule and mneme spec links. Use when creating a branch, writing a commit or pull request, reviewing, merging or releasing, or editing CONTRIBUTING, SECURITY, CODE_OF_CONDUCT, GitHub templates or release configuration in the Orcon repository.
license: Apache-2.0
compatibility: Rules in references/ are self-contained; scripts/ require an Orcon repository checkout with git, Go, GNU make, pnpm 9 and Node 22, and an authenticated GitHub CLI for the settings check.
metadata:
  version: 1.1.1
  pinned: true
  owner: wirvii
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Orcon Contributing

How a change travels from a branch to a signed release. The rules are in `references/`; this file is the procedure and the checklist.

Precedence: accepted ADRs (ADR 0081) > `references/` > reviewer preference. The provenance rule and `make check` belong to the `orcon-go` skill (`orcon-go/references/tooling.md`); this skill applies them to commits, pull requests and releases without restating them.

| Reference | Rules |
|---|---|
| [branches.md](references/branches.md) | BR - branch names, lifetime, updating, deletion |
| [commits.md](references/commits.md) | CM - format, types, the closed scope list, breaking changes, footers, local hooks |
| [pull-requests.md](references/pull-requests.md) | PR - title, template, Spec line, testing evidence, screenshots, skill checklists, size |
| [review.md](references/review.md) | RV - required checks, approvals and the second-maintainer trigger, conversations, squash merge, outside contributors |
| [releases.md](references/releases.md) | RL - semver, release pull request, signed tag, artifacts, signatures, provenance, failures |
| [community-docs.md](references/community-docs.md) | CD - CONTRIBUTING, SECURITY, CODE_OF_CONDUCT, issue forms, labels |
| [provenance.md](references/provenance.md) | PV - the provenance rule in commits, pull requests and releases |
| [sdd-and-prs.md](references/sdd-and-prs.md) | SD - how mneme specs, slices and QA map to branches and pull requests |
| [github-settings.md](references/github-settings.md) | GH - repository settings and rulesets, and how to audit them |

## When To Use

- You are creating or naming a branch, writing a commit message, or writing a pull request title or description.
- You are opening, reviewing, updating or merging a pull request.
- You are preparing, publishing or verifying a release, or editing `release-please-config.json`, `.goreleaser.yaml`, `build/release/` or a release workflow.
- You are editing `CONTRIBUTING.md`, `SECURITY.md`, `CODE_OF_CONDUCT.md`, `.github/pull_request_template.md`, `.github/ISSUE_TEMPLATE/` or the size exclusions in `.gitattributes`.
- You are linking a mneme spec slice to a pull request or recording a merged pull request in a spec.

## Critical Rules

1. Branch from an up-to-date `main` as `<type>/<slug>`; work tracked in mneme starts the slug with `spec-NNN-` (BR-1, BR-2).
2. Only pull requests reach `main`; never push to it, force-push it or ask for a bypass (RV-1).
3. The pull request title is the commit on `main`: `<type>(<scope>): <summary>`, one scope, imperative lowercase summary, no final period, at most 72 characters (CM-1..CM-5).
4. Scopes are only engine, trigger, connector, api, cli, mcp, web, ui, db, ai, security, notify, docs, ci, deps, tooling; a new scope needs an ADR 0081 amendment (CM-3, CM-4).
5. Breaking change: `!` before the colon and a filled `Breaking changes` section (CM-6).
6. Every commit of the branch follows rules 3 and 4; reword with an interactive rebase before pushing.
7. Keep every template section; write `Spec: SPEC-NNN` for work tracked in mneme and `Spec: n/a` otherwise (PR-2, SD-2).
8. `How it was tested` lists commands and results; UI changes attach screenshots (PR-4, PR-5).
9. Tick and paste the checklist of every skill whose paths you touched; `scripts/compose-pr-body.sh` does it (PR-6).
10. Stay under about 400 changed lines, not counting `pr-size-exclude` paths; above it, write the `Size justification` (PR-7, PR-9).
11. A new generated, catalog, screenshot or lockfile path gets its `pr-size-exclude` line in `.gitattributes` in the same pull request (PR-8).
12. Merge only by squash, with every required check green and every conversation resolved; approvals are zero while there is one maintainer and one from a code owner once a second maintainer has write access (RV-2..RV-5, GH-9).
13. Publish only by merging the release pull request; never create, move or delete a `v*` tag and never replace release assets (RL-3, RL-5).
14. No AI authorship trailers and no names of the AI tools used to build Orcon in commits, pull request text, release notes or documentation; mneme is named openly (PV-1, PV-2).
15. Vulnerabilities go to private vulnerability reporting, never to public issues (CD-3).
16. Edit this skill only under `.agents/skills/orcon-contributing/` and re-run `scripts/project.sh`; never run `mneme skills pin` on it.

## Automated Checks

| Check | What it verifies | How to fix |
|---|---|---|
| CI `commit-lint` title step (`make pr-title-lint`) | The pull request title is a Conventional Commit with one allowed scope | Edit the title; the check re-runs on edit |
| CI `commit-lint` commits step (`make commit-lint`) | Warns when a commit of the pull request does not follow the same format (merge commits are ignored; only the title reaches `main`) | Optional: `git rebase -i origin/main`, reword, push with `--force-with-lease` |
| `pre-push` hook (enabled by `make hooks`) | Outgoing commits follow the format before they leave your machine | Reword the commits; never use `--no-verify` |
| CI `pr-advisory` size | Changed lines without `pr-size-exclude` paths against 400 | Split the pull request or write the `Size justification` |
| CI `pr-advisory` spec, branch and skills | `Spec:` line, branch name, every applicable skill ticked | Add the line, tick and paste the checklist |
| `make commitlint-selftest` | The commitlint config accepts and rejects the fixture messages and its scopes equal `references/commits.md` | Fix `commitlint.config.mjs` or the table |
| `make prcheck-test` | Size, spec, branch and skill checks behave as documented | Fix `tools/prcheck` |
| `make workflows-lint` | Every GitHub workflow passes actionlint | Fix the workflow |
| `make release-config-check` | GoReleaser and release-please configurations are valid and release notes match their fixture | Fix the configuration or the notes script |
| CI `release-check` (`make release-check`) | A snapshot release builds six archives, checksums, the SBOM and the images | Fix `.goreleaser.yaml`, `build/release/` or the build |
| CI `provenance` and `make provenance` | No AI attribution in files, commits and pull request text | Follow `orcon-go/references/tooling.md` |
| Release `verify` job (`scripts/release/verify.sh`) | Tag signature, checksum signature, SLSA provenance of archives and image, image signature and SBOM attestation | Fix forward with a new patch release |
| `scripts/check-github-settings.sh` | Repository settings, rulesets and the approval count match `references/github-settings.md` | Ask the owner to apply the missing setting |
| `make skill-check` | This skill conforms and its generated copy is identical | Fix `SKILL.md`, then run `make skill-sync` |

## Verification

1. `make contrib-check` exits 0; `sh scripts/verify.sh` runs it together with `make skill-check`.
2. Before opening a pull request, `sh scripts/pre-pr-check.sh` shows a clean commit lint, the measured size and the skills that apply.
3. Release changes: `make release-check` exits 0; after a real release the `verify` job of the Release workflow is green.
4. This skill: `sh scripts/project.sh --home && mneme skills lint orcon-contributing && mneme skills validate orcon-contributing` pass.
5. Report the commands you ran and their results when you hand back.

## Workflow

1. Confirm the change is authorized: an accepted ADR or spec tracked in mneme, or an issue the maintainers agreed to.
2. `git switch main && git pull --ff-only && git switch -c <type>/<slug>` ([branches.md](references/branches.md)); run `make hooks` once per clone.
3. Commit in small steps with scoped messages ([commits.md](references/commits.md)).
4. Rebase on `origin/main`, run the checks of the other skills that apply, then `sh scripts/pre-pr-check.sh`.
5. `sh scripts/compose-pr-body.sh > pr-body.md`, complete every section, then open the pull request with that body and a Conventional Commit title ([pull-requests.md](references/pull-requests.md)).
6. Keep `commit-lint` and every required check green; answer and resolve every conversation ([review.md](references/review.md)).
7. Work tracked in mneme: QA reviews the pull request's commits; the owner merges by squash and the changes document records the pull request ([sdd-and-prs.md](references/sdd-and-prs.md)).
8. Releasing: review the release pull request with `assets/templates/release-pr-review.md`, merge it, and follow the Release workflow until `verify` is green ([releases.md](references/releases.md)).
