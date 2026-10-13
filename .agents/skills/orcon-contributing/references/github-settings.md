<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# GitHub settings

Status: Accepted

This document lists the repository settings and rulesets the owner applies on GitHub, the change to make when a second maintainer joins, and how both are audited. It is part of the `orcon-contributing` rulebook; the decision behind it is ADR 0081. Rule IDs (`GH-…`) are stable. The check that enforces a rule, when one exists, is named in brackets. Precedence: accepted ADRs (ADR 0081) > accepted `orcon-go` rules > this rulebook > reviewer preference.

## Rules

- **GH-1** The owner applies the settings below; `scripts/check-github-settings.sh` compares them with `assets/github-settings.expected.json` (repository settings, rulesets with `bypass_actors`, `allowed_merge_methods`, required approvals, required checks with the GitHub Actions source, private vulnerability reporting, Actions permissions).
- **GH-2** Release GitHub App `orcon-release`: Contents, Pull requests and Issues read/write, Metadata read; installed only on this repository.
- **GH-3** Environment `release`: deployment refs `main` and tags `v*`; secrets `ORCON_RELEASE_APP_ID`, `ORCON_RELEASE_APP_PRIVATE_KEY`; no required reviewers.
- **GH-4** Squash merging only, default message "Pull request title", update-branch suggestions on, auto-merge off, auto-delete head branches on; Actions default permissions read-only, Actions cannot create or approve pull requests, fork workflows need approval for all external contributors.
- **GH-5** Ruleset `main`: empty bypass list; restrict deletions; block force pushes; linear history; pull request required with **zero required approvals while the owner is the only maintainer** (RV-3), stale approvals dismissed, conversation resolution required, squash only; required status checks, strict, source GitHub Actions; the check list is the table in "Required status checks" below and is extended by each spec that adds a required job, in that spec's own slice.
- **GH-6** Ruleset `release-tags` on `refs/tags/v*`: bypass only the `orcon-release` app; restrict creations, updates and deletions; block force pushes.
- **GH-7** Private vulnerability reporting, dependency graph, Dependabot alerts, secret scanning and push protection enabled.
- **GH-8** Package `ghcr.io/wirvii/orcon` public and linked to the repository after the first release.
- **GH-9** Second maintainer trigger. When a second maintainer with write access exists: (1) open `chore(tooling): require one approval on main` adding `.github/CODEOWNERS` with both maintainers, setting the expected approvals to 1 in `assets/github-settings.expected.json`, and recording the date and the maintainer here; (2) after it merges, set the `main` ruleset to one required approval, require review from code owners and require approval of the most recent push; (3) re-run the audit. The audit expects 0 approvals without `.github/CODEOWNERS` and 1 with it, so a half-applied change fails.

## Owner checklist

Only the owner can apply these settings. Required checks are added only after the workflow that reports them is on `main`: a required check that never reports blocks every pull request. Apply the sections in this order; the release App and the `release` environment exist before the first merge to `main` that runs `release-please.yml`.

### A. Release App (organization `wirvii`)

- [ ] Create the GitHub App `orcon-release`: no webhook; repository permissions Contents read/write, Pull requests read/write, Issues read/write, Metadata read; nothing else.
- [ ] Install it only on `wirvii/orcon`; generate its private key.

### B. Environment `release` (Settings → Environments)

- [ ] Deployment branches and tags: selected — branch `main` and tag `v*`.
- [ ] Environment secrets: `ORCON_RELEASE_APP_ID`, `ORCON_RELEASE_APP_PRIVATE_KEY`.
- [ ] No required reviewers (merging the release pull request is the decision to publish).
- [ ] Repository variable `ORCON_RELEASE_ENABLED` set to `true` once the App and the environment above exist. Until then the jobs of `release-please.yml` are skipped and never touch the environment.

### C. General (Settings → General → Pull Requests)

- [ ] Allow squash merging: on; default commit message **Pull request title** (`squash_merge_commit_title=PR_TITLE`, `squash_merge_commit_message=BLANK`).
- [ ] Allow merge commits: off. Allow rebase merging: off. Allow auto-merge: off.
- [ ] Always suggest updating pull request branches: on. Automatically delete head branches: on.

### D. Actions (Settings → Actions → General)

- [ ] Workflow permissions: read repository contents and packages.
- [ ] Allow GitHub Actions to create and approve pull requests: off.
- [ ] Fork pull request workflows from outside collaborators: require approval for all external contributors.

### E. Ruleset `main` (Settings → Rules → Rulesets; target: default branch; enforcement: active)

- [ ] Bypass list: **empty** (nobody, administrators and the owner included).
- [ ] Restrict deletions: on. Block force pushes: on. Require linear history: on.
- [ ] Require a pull request before merging: on — required approvals **0** while the owner is the only maintainer (RV-3, GH-9); dismiss stale pull request approvals when new commits are pushed: on; require conversation resolution before merging: on; allowed merge methods: **squash** only.
- [ ] Require status checks to pass: on — require branches to be up to date before merging: on; every check with source **GitHub Actions**; the list is "Required status checks" below.

### F. Ruleset `release-tags` (target: tags `refs/tags/v*`; enforcement: active)

- [ ] Bypass list: only the `orcon-release` App.
- [ ] Restrict creations, restrict updates, restrict deletions, block force pushes: on.

### G. Security (Settings → Code security)

- [ ] Private vulnerability reporting: on.
- [ ] Dependency graph and Dependabot alerts: on. Secret scanning and push protection: on.

### H. Issue labels

- [ ] `bug`, `enhancement` and `needs-triage` exist (release-please creates the `autorelease: *` labels).

### I. Package (after the first release)

- [ ] `ghcr.io/wirvii/orcon`: public, linked to the repository, write access for the Actions of `wirvii/orcon`.

### J. Audit

- [ ] `sh .agents/skills/orcon-contributing/scripts/check-github-settings.sh` prints PASS for every line.

### K. Second maintainer (future action, GH-9)

- [ ] In a pull request `chore(tooling): require one approval on main`: create `.github/CODEOWNERS` (`* @<owner> @<second maintainer>`), set the approvals to 1 in `assets/github-settings.expected.json` and record the date and the name in GH-9.
- [ ] After it merges, in ruleset `main`: required approvals **1**; require review from code owners: on; require approval of the most recent reviewable push: on. Re-run the audit (J).

## Required status checks

| Check (displayed name) | Workflow / job | Added by | When to require it |
|---|---|---|---|
| `commit-lint` | PR / commit-lint | SPEC-013 | when `pr.yml` is on `main` |
| `release-check` | Contributing / release-check | SPEC-013 | when `contributing.yml` is on `main` |
| `integration` | CI / integration | existing | now |
| `ui-quality`, `ui-visual`, `skills` | CI | SPEC-007 | when SPEC-007 merges |
| `check (go oldstable)`, `check (go stable)`, `provenance` | CI | SPEC-008 | when SPEC-008 merges (and remove `go` from the list) |
| `postgres` | CI / postgres (the aggregate job over the image matrix; do not require the matrix names `postgres (postgres:…@sha256:…)`) | SPEC-008/009 | when they merge |
| `web-e2e`, `web-perf` | CI | SPEC-010 | when it merges |
| `connectors` | CI | SPEC-011 | when it merges |
| `api-contract` | CI | SPEC-012 | when it merges |

Not required: `pr-advisory`, `bench`, nightly jobs, `connectors-live`, `visual-baselines`, `release-please.yml`, `release.yml`.
