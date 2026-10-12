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

`apps/orcon` contains the binary; `apps/web` the editor; `packages/go/*` shared modules coordinated with `go.work`; pnpm/Turborepo coordinates frontend tasks. Additional apps and packages are allowed.

## Deferred architecture decisions

Still deferred: idempotency protocol at the connector SDK level; secrets and tenancy; plugin execution; API versioning; auth/IAM; observability schema. Object storage is partially specified by ADR-0016. Do not implement these based on assumptions.

Resolved by accepted ADRs:
- Engine state machine → ADR-0009 (SPEC-001, 2026-10-12).
- Event history model → ADR-0009 and ADR-0017 (SPEC-001, 2026-10-12).
- SQLite/PostgreSQL abstraction → ADR-0015 (SPEC-001, 2026-10-12).
- Scheduling and fairness → ADR-0014 (SPEC-001, 2026-10-12).
- n8n expression compatibility → ADR-0018 (SPEC-001, 2026-10-12).
