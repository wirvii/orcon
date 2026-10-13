<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Orcon

**Open-source workflow orchestration engine**, implemented in Go with a React Flow editor.

Licensed under [Apache License 2.0](LICENSE). Early-stage scaffold: **no durable workflow execution yet**.

## Monorepo

- `apps/orcon` — multi-role Go executable; embeds the web build using `embed.FS`
- `apps/web` — React + Vite + React Flow
- `packages/go/config` — role configuration
- `packages/go/workflow` — workflow graph contracts
- `packages/go/engine` — execution engine interface
- `docs/architecture` and `docs/adr` — technical decisions

Go modules are coordinated through `go.work`; JS workspaces use pnpm and Turborepo.

## Build

Requires Go 1.24+, Node.js 22+, pnpm 9.

```bash
pnpm install
make build
./bin/orcon --role=standalone --addr=:8080
```

Open http://localhost:8080. The `/healthz` endpoint responds with `ok`.

## Runtime roles

```bash
./bin/orcon --role=standalone
./bin/orcon --role=api
./bin/orcon --role=scheduler
./bin/orcon --role=worker
```

**Only `standalone` and `api` currently serve HTTP**. Scheduler and worker are placeholders; distributed execution and SQLite/PostgreSQL persistence remain to be implemented.

## Verification

GitHub Actions runs Go tests, builds the React app, embeds its assets in the Go binary, and smoke-tests HTTP startup. A passing workflow is required before treating the scaffold as verified.
