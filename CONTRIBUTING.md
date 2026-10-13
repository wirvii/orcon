<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Contributing to Orcon

Thank you for helping build Orcon. Issues, design discussions and pull requests are welcome. Every contribution is licensed under the [Apache License 2.0](LICENSE), and everyone who takes part follows the [Code of Conduct](CODE_OF_CONDUCT.md).

This page is a summary. The complete rules are the contribution rulebook in [`.agents/skills/orcon-contributing/references/`](.agents/skills/orcon-contributing/references/); the coding rules of each area live in the other skills under `.agents/skills/`. For Go code, start with [docs/engineering/go.md](docs/engineering/go.md) and run `make check` before opening a pull request.

## Before you start

- Orcon implements only what accepted design documents authorize (`docs/engineering/implementation-contract.md`, `docs/adr/`). For anything beyond a small fix, open an issue first so maintainers can confirm the direction.
- Never report a security vulnerability in a public issue. See [SECURITY.md](SECURITY.md).
- Build and test instructions are in [README.md](README.md). Run `make hooks` once after cloning.

## Branches

Work on a short-lived branch named `<type>/<slug>`, for example `feat/flow-version-history` or `fix/execution-timeout`, created from an up-to-date `main`. Changes reach `main` only through pull requests; nobody pushes to `main` directly.

## Commit messages

Orcon uses [Conventional Commits](https://www.conventionalcommits.org/) with exactly one scope from a closed list:

    <type>(<scope>): <summary>

- Types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `build`, `ci`, `chore`, `revert`.
- Product scopes: `engine`, `trigger`, `connector`, `api`, `cli`, `mcp`, `web`, `ui`, `db`, `ai`, `security`, `notify`.
- Internal scopes: `docs`, `ci`, `deps`, `tooling`.
- The summary is in English, imperative, starts in lowercase and has no final period. Mark a breaking change with `!`, for example `feat(api)!: remove the v0 flow endpoint`.

A check runs on the pull request title and on every commit of the pull request. To fix a commit message, use `git rebase -i origin/main` and push with `--force-with-lease`.

Commit messages, pull requests and documentation describe the change itself. They never carry AI authorship trailers or name the AI tools used while writing the change; an automatic check enforces this.

## Pull requests

- The title follows the commit format and becomes the single commit on `main` (squash merge).
- Fill in every section of the template: what and why, how you tested it (commands and results), screenshots for UI changes, breaking changes, and the checklist of every area you touched.
- Keep pull requests under about 400 changed lines, not counting generated code, translation catalogs, screenshots and lockfiles. If a change cannot be split, explain why in the `Size justification` section; a check warns otherwise.
- All required checks must pass and every review conversation must be resolved before a maintainer merges. While Orcon has a single maintainer, no approving review is required; once a second maintainer with write access joins, every pull request needs one approval from a code owner.

## Connectors

Read [docs/engineering/connectors.md](docs/engineering/connectors.md) before writing or changing a connector. The complete connector rulebook lives in `.agents/skills/orcon-connector/references/` and is packaged as the `orcon-connector` mneme skill; templates for new connectors are in `.agents/skills/orcon-connector/assets/templates/`.

## How maintainers track work

Maintainers plan and review work with [mneme](https://github.com/juanftp/mneme), which keeps the project's backlog, specifications, acceptance criteria and QA reports. Pull requests for that work include a line such as `Spec: SPEC-013`. You do not need mneme to contribute: use `Spec: n/a`, and a maintainer links your pull request to the backlog if needed.

## Releases

Orcon follows semantic versioning. An automated release pull request collects the changelog, grouped by type of change; merging it publishes signed binaries, a container image, an SBOM and build provenance. See [docs/releases/verifying.md](docs/releases/verifying.md) to verify a release.
