<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0002 — Single binary, multiple runtime roles

Status: **Accepted (conceptual)** | Date: 2026-10-10

> Amended by [ADR 0075](0075-go-module-and-package-layout.md) for the Go module layout (accepted): the sentence "Shared libraries remain in packages/go" is replaced by its single-module layout.

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
- ADR-0020 §6 assigns trigger responsibilities: api receives HTTP ingress; scheduler fires schedules and drives trigger reconciliation; workers host connector-side trigger code (polling, persistent listeners, external registration). No new roles. (SPEC-002, accepted 2026-10-12.)
- The Orcon catalog service (`cmd/orcon-catalog`, SPEC-003 Q8; path per ADR-0075) is a separate binary in the same repository, not a role of the `orcon` binary; the Python runtime of the Code node (SPEC-003 Q7) is a data package loaded by the binary, not a second runtime installed separately. (SPEC-003, accepted 2026-10-12.)
- Server roles are selected with `orcon serve --role`; client subcommands (ADR-0058) are part of the same binary and only call the public API. (SPEC-005, accepted 2026-10-12.)
