# ADR-0002 — Single binary, multiple runtime roles

Status: **Accepted (conceptual)** | Date: 2026-10-10

## Context
Orcon must be easily distributed locally, in containers, and as horizontally scaled services, without separate runtime installations of the web frontend.

## Decision
A single Go executable embeds the compiled React/React Flow application through Go embed.FS. The executable selects a role with a CLI flag: standalone, api, scheduler, worker. Standalone combines the necessary roles for local operation. API serves the frontend and public APIs. Scheduler manages triggers, timers and task eligibility. Worker executes tasks. Each role may run in separate processes/containers built from the same artifact.

## Constraints
Frontend build tools (Node/pnpm) are build-time only. Never interpret one binary as one process. Use graceful shutdown, health/readiness signals and explicit role configuration. Shared libraries remain in packages/go; apps/web is separately developed and bundled at release build.

## Consequences
Simpler distribution and consistent versioning; binary size includes frontend assets; deploys must coordinate protocol/schema compatibility across roles. Detailed CLI, lifecycle and startup contracts remain to be specified.

## Notes
No decision of this ADR changes; these notes record how later accepted ADRs apply it.
- ADR-0013 assigns concrete responsibilities to the `scheduler` and `worker` roles without adding roles (SPEC-001, accepted 2026-10-12).
