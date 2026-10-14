<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
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

A check fails the pull request when its title does not follow the format, because the title becomes the commit on `main`. The same check only warns about the commits of the pull request (merge commits are ignored). To fix a commit message, use `git rebase -i origin/main` and push with `--force-with-lease`.

Commit messages, pull requests and documentation describe the change itself. They never carry AI authorship trailers or name the AI tools used while writing the change; an automatic check enforces this.

## Pull requests

- The title follows the commit format and becomes the single commit on `main` (squash merge).
- Fill in every section of the template: what and why, how you tested it (commands and results), screenshots for UI changes, breaking changes, and the checklist of every area you touched.
- Keep pull requests under about 400 changed lines, not counting generated code, translation catalogs, screenshots and lockfiles. If a change cannot be split, explain why in the `Size justification` section; a check warns otherwise.
- All required checks must pass and every review conversation must be resolved before a maintainer merges. While Orcon has a single maintainer, no approving review is required; once a second maintainer with write access joins, every pull request needs one approval from a code owner.

## Connectors

Read [docs/engineering/connectors.md](docs/engineering/connectors.md) before writing or changing a connector. The complete connector rulebook lives in `.agents/skills/orcon-connector/references/` and is packaged as the `orcon-connector` mneme skill; templates for new connectors are in `.agents/skills/orcon-connector/assets/templates/`.

## Public API

Read [docs/engineering/api.md](docs/engineering/api.md) before changing the OpenAPI contract in `packages/openapi/`. The complete API design rulebook lives in `.agents/skills/orcon-api/references/` and is packaged as the `orcon-api` mneme skill; start from its templates in `.agents/skills/orcon-api/assets/templates/` and paste `.agents/skills/orcon-api/assets/pr-api-checklist.md` into the pull request.

## Database schema and SQL

Read [docs/engineering/db.md](docs/engineering/db.md) before changing a table, a migration or a SQL query. The complete database rulebook lives in `.agents/skills/orcon-db/references/` and is packaged as the `orcon-db` mneme skill. Run `make db-check` while iterating, and `make check` plus `make test-postgres` (needs a container runtime) before opening a pull request; paste `.agents/skills/orcon-db/assets/pr-db-checklist.md` into its description.

## Web application code

Read [docs/engineering/web.md](docs/engineering/web.md) before changing code in `apps/web` or `packages/js`. The complete web application rulebook lives in `.agents/skills/orcon-web/references/` and is packaged as the `orcon-web` mneme skill.

## Web UI code

Read [docs/design-system/README.md](docs/design-system/README.md) before restyling or adding components, canvas, editor, tokens, screens or translation catalogs under `apps/web` or `packages/js`. The design rules live in `.agents/skills/orcon-ui/references/` and are packaged as the `orcon-ui` mneme skill; run `sh .agents/skills/orcon-ui/scripts/verify.sh` before opening the pull request.

## How maintainers track work

Maintainers plan and review work with [mneme](https://github.com/juanftp/mneme), which keeps the project's backlog, specifications, acceptance criteria and QA reports. Pull requests for that work include a line such as `Spec: SPEC-013`. You do not need mneme to contribute: use `Spec: n/a`, and a maintainer links your pull request to the backlog if needed.

## Releases

Orcon follows semantic versioning. An automated release pull request collects the changelog, grouped by type of change; merging it publishes signed binaries, a container image, an SBOM and build provenance. See [docs/releases/verifying.md](docs/releases/verifying.md) to verify a release.
