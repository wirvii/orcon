<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<p align="center">
  <img src="brand/exports/orcon-banner.png" alt="Orcon, orchestration controller" width="900">
</p>

<p align="center">
  <img src="brand/exports/mascot-at-work.png" alt="Orcon mascot: a green orc engineer at a laptop" width="480">
  <br>
  <em>Meet the orc engineer, the face of Orcon.</em>
</p>

<p align="center"><strong>Build. Orchestrate. Execute. Your way.</strong></p>

# Orcon

Orcon is an open-source workflow orchestration engine, licensed under the [Apache License 2.0](LICENSE). It is written in Go with a React Flow editor, and it is designed to be compatible with n8n-style flows (see [n8n compatibility](docs/product/n8n-compatibility.md)).

**Orcon is in early development. Nothing in this repository is production-ready, and there is no durable workflow execution yet.**

## Current status

Available on `main` today:

- Go engineering standards and a single Go module (`github.com/wirvii/orcon`), enforced by `make check`.
- The Orcon design system and the flow editor UI, developed and reviewed in Storybook (`@orcon/tokens`, `@orcon/ui`, `apps/web`).
- The `orcon/flow@1` flow format with reading, writing, upgrade and static validation in `internal/flow` ([format reference](docs/reference/orcon-flow-1.md)).
- Database standards and automated checks, with SQLite and PostgreSQL migration directories.
- The contribution workflow: scoped Conventional Commits, pull request rules and signed releases.

In progress:

- An in-memory flow interpreter.
- The expression language.

Next:

- Storage.
- Durable execution.
- The public API.

## Repository layout

| Path | Contents |
| --- | --- |
| `cmd/orcon` | The multi-role executable |
| `internal/<area>` | Business areas (`internal/flow`, `internal/execution`) |
| `internal/adapter` | Adapters: `codec`, `httpapi`, `postgres`, `sqlite`, `storetest`, `webui` |
| `internal/platform` | Shared platform code (`config`, `jsonvalue`) |
| `tools` | Project tools: `orconvet`, `provenance`, `spdxcheck`, `dbcheck` and others |
| `apps/web` | The web application (React, Vite, React Flow) |
| `packages/js/tokens` | `@orcon/tokens`, design tokens |
| `packages/js/ui` | `@orcon/ui`, themed components and Storybook |
| `packages/js/lint-config` | `@orcon/lint-config`, shared lint rules |
| `docs/adr` | Architecture decision records |
| `docs/engineering` | Short human summaries of the engineering standards |
| `.agents/skills` | Project skills (see below) |
| `brand` | Approved brand boards and derived exports |

Go modules are coordinated through `go.work`; JavaScript workspaces use pnpm and Turborepo.

## Build and run

Requirements: Go 1.25 or newer (`go.mod` sets `go 1.25.0`), Node.js 22 or newer, and pnpm 9.

```bash
pnpm install
make build                                   # builds the web app, embeds it, builds bin/orcon
./bin/orcon --role=standalone --addr=:8080
```

Open http://localhost:8080. The `/healthz` endpoint responds with `ok`.

The `--role` flag accepts `standalone`, `api`, `scheduler` and `worker`. Only `standalone` and `api` serve HTTP; `scheduler` and `worker` are placeholders.

Other commands:

```bash
make check     # the single gate, identical locally and in CI
make test      # Go tests
make smoke     # starts the built binary and checks HTTP startup
pnpm test      # JavaScript tests through Turborepo
pnpm --filter @orcon/ui storybook   # the design system in Storybook, port 6006
```

## Project skills

Each skill packages a set of accepted standards for contributors and their tools. The canonical copy lives in `.agents/skills/<name>/`; the generated copy that tools read is refreshed with `make skill-sync`.

| Skill | What it covers | Summary |
| --- | --- | --- |
| `orcon-go` | Idiomatic Go with hexagonal ports and adapters, and the checks that enforce it | [docs/engineering/go.md](docs/engineering/go.md) |
| `orcon-ui` | Web UI with the design system: tokens, components, canvas, i18n, accessibility | [docs/design-system](docs/design-system/README.md) |
| `orcon-db` | Schema naming, ids, migrations, tenant isolation and integrity | [docs/engineering/db.md](docs/engineering/db.md) |
| `orcon-web` | Web application code: feature folders, data fetching, routing, state | [docs/engineering/web.md](docs/engineering/web.md) |
| `orcon-api` | Public API design: paths, fields, pagination, errors, contract toolchain | [docs/engineering/api.md](docs/engineering/api.md) |
| `orcon-connector` | Connector authoring: kinds, naming, manifest, credentials | [docs/engineering/connectors.md](docs/engineering/connectors.md) |
| `orcon-contributing` | Branches, commits, pull requests, reviews and signed releases | [CONTRIBUTING.md](CONTRIBUTING.md) |

## Documentation

- [Architecture decision records](docs/adr)
- [Engineering summaries](docs/engineering)
- [Reference pages](docs/reference)
- [User documentation](docs/user/README.md)
- [Design system](docs/design-system/README.md)
- [Verifying a release](docs/releases/verifying.md)
- [Brand](brand/README.md)

## Contributing

<img src="brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="160" align="right">

Contributions are welcome. [CONTRIBUTING.md](CONTRIBUTING.md) explains branches, commit messages and pull requests, and everyone who takes part follows the [Code of Conduct](CODE_OF_CONDUCT.md). Report security vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

Specs, backlog and project memory are managed with mneme.

## License

Orcon is licensed under the [Apache License 2.0](LICENSE). See [NOTICE](NOTICE) for attributions.
