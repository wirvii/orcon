# Orcon — Architecture Overview

Status: **Accepted at conceptual level** | Version: 0.1

## System boundary

One Go executable, built from a monorepo, embeds the production React/React Flow frontend using Go embed facilities. Runtime roles: `standalone`, `api`, `scheduler`, and `worker`. Roles are runtime selections, not separately maintained binaries. Other roles may be added via ADR.

## Responsibilities

- **API / web:** authentication, workflow CRUD, versioning, execution management, static editor hosting, authorization.
- **Workflow engine:** graph validation, scheduling eligibility, node transitions, branch/join semantics, lifecycle and cancellation.
- **Scheduler:** trigger handling, durable timers, retries, fair dispatch, concurrency quotas.
- **Worker:** leases tasks, executes trusted connectors or isolated extensions, records outcomes and heartbeats.
- **State store:** transactional workflow versions, executions, attempts, task leases, events and timers.
- **Connector subsystem:** versioned node contracts, credentials and external side-effect policies.
- **Agent subsystem:** model-provider abstraction, tools/MCP, memory, human approval, observability.

## Deployment profiles

| Profile | Runtime | Durable storage | External services |
| --- | --- | --- | --- |
| Local standalone | Single process with embedded editor | SQLite (planned) | None required |
| Distributed | Identical binary instances with distinct roles | PostgreSQL (planned) | PostgreSQL |
| Optimized distributed | Same | PostgreSQL | Optional Redis as acceleration |

SQLite is not a shared distributed coordinator. PostgreSQL task claims are planned using transactional leasing; exact lease/fencing and crash recovery semantics require an ADR and tests.

## Execution guarantees: design constraints

- Never hold database transactions open while invoking external services.
- Durable transitions must be atomic with enqueueing subsequent runnable tasks.
- Worker failure must not silently lose accepted work.
- External side effects may occur more than once unless the destination supports idempotency or reconciliation.
- Parallel branches must have explicit merge semantics, limits and cancellation rules.
- Versioned workflow definitions must remain available to active executions.
- Large payloads should be referenced or streamed where practical, not copied indiscriminately through every node.

## Development layout

> Amended by [ADR 0075](../adr/0075-go-module-and-package-layout.md) for the Go module layout (accepted).

`apps/orcon` contains the binary; `apps/web` the editor; `packages/go/*` shared modules coordinated with `go.work`; pnpm/Turborepo coordinates frontend tasks. Additional apps and packages are allowed.

## Deferred architecture decisions

Engine state machine; event history model; SQLite/PostgreSQL abstraction; scheduling and fairness; idempotency protocol; secrets and tenancy; plugin execution; API versioning; n8n expression compatibility; auth/IAM; object storage; observability schema. Do not implement these based on assumptions.
