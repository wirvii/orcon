<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Commit messages

Status: Proposed — pending owner approval

This document defines the format of commit messages and pull request titles, the closed list of scopes, and the local and CI checks that enforce them. It is part of the `orcon-contributing` rulebook; the decision behind it is ADR 0081. Rule IDs (`CM-…`) are stable. The check that enforces a rule, when one exists, is named in brackets. Precedence: accepted ADRs (ADR 0081) > accepted `orcon-go` rules > this rulebook > reviewer preference.

## Rules

- **CM-1** Format: `<type>(<scope>)[!]: <summary>`, optional body after a blank line, optional footers. Header at most 72 characters. [`commit-lint`]
- **CM-2** Types: `feat` (new user-visible capability; minor), `fix` (bug fix; patch), `perf` (performance; patch), `refactor`, `docs`, `test`, `build`, `ci`, `chore`, `revert`. Changelog shows feat, fix, perf, revert and docs.
- **CM-3** Scopes (closed list; the table is checked against `commitlint.config.mjs`):

| Scope | Covers |
|---|---|
| `engine` | flow definitions, execution, journal, expressions, payloads (`internal/flow`, `internal/execution`, `internal/journal`, `internal/expr`, `internal/blob`) |
| `trigger` | triggers, ingress, schedules, polling (`internal/trigger`) |
| `connector` | connector model, catalog, sandbox, Code node, official connectors, connector SDK, n8n import (`internal/connector`, `internal/catalog`, `internal/sandbox`, `internal/codenode`, `internal/adapter/connectors`, `packages/go/connector`, `internal/n8nimport`) |
| `api` | public HTTP API, OpenAPI contract, SDKs, events and webhooks, flows as code (`packages/openapi`, `internal/adapter/httpapi`, `internal/event`, `internal/gitsync`, `packages/go/sdk`) |
| `cli` | the command-line interface (`internal/adapter/cli`) |
| `mcp` | MCP server and client (`internal/adapter/mcpserver`, `internal/adapter/mcpclient`) |
| `web` | the editor application (`apps/web`) |
| `ui` | design system packages (`packages/js/tokens`, `packages/js/ui`, `packages/js/lint-config`) |
| `db` | schema, migrations, queries, storage adapters (`internal/adapter/sqlite`, `internal/adapter/postgres`, `internal/adapter/storetest`) |
| `ai` | agents, model providers, knowledge retrieval (`internal/ai`, `internal/retrieval`, `internal/adapter/llm`) |
| `security` | identity, authorization, credentials, keys, audit, vulnerability fixes (`internal/identity`, `internal/authz`, `internal/credential`, `internal/audit`, `internal/adapter/idp`, `internal/adapter/secretstore`, `internal/adapter/kms`) |
| `notify` | human-attention notifications (`internal/notify`) |
| `docs` | documentation not tied to one product area |
| `ci` | GitHub workflows and CI configuration |
| `deps` | dependency updates |
| `tooling` | `tools/`, `scripts/`, `Makefile`, skills, release configuration, repository configuration |

  When a change spans areas, use the area a reader of the release notes would look for; when two areas both matter, split the pull request.
- **CM-4** Exactly one scope, lowercase, from the list. Adding a scope amends ADR 0081 and, in the same pull request, `commitlint.config.mjs`, this table and `release-please-config.json` if needed.
- **CM-5** Summary: English, imperative mood, lowercase first letter, no final period; it describes the change, never the process or the tools used to write it.
- **CM-6** Breaking change: `!` before the colon (`feat(api)!: remove the v0 flow endpoint`) and a filled `Breaking changes` section in the pull request. Before 1.0 it bumps the minor version.
- **CM-7** Body: why the change is needed; lines up to 100 characters. Allowed footers: `Refs: #<issue>`, `Spec: SPEC-NNN`. AI authorship trailers are forbidden (PV-1).
- **CM-8** Security fixes use `fix(security): …` and give no exploit detail until the advisory is published.
- **CM-9** Dependency updates use `chore(deps): …`; `fix(deps): …` when the update fixes a bug or vulnerability that users see.
- **CM-10** Reverts: `revert(<scope>): <original summary>` with body `This reverts commit <sha>.`
- **CM-11** Local checks: `make hooks` once per clone enables the `commit-msg` hook (provenance, owned by `orcon-go`) and the `pre-push` hook (this format). Never skip hooks with `--no-verify`.
- **CM-12** Tool: commitlint (MIT) with `commitlint.config.mjs`; the header pattern accepts a single scope. [`make commitlint-selftest`]
