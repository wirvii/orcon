<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR 0001: Hybrid monorepo

Status: Accepted

> Amended by [ADR 0075](0075-go-module-and-package-layout.md) for the Go module layout (accepted; the pnpm/Turborepo part of this decision is unchanged).

Use pnpm + Turborepo for web apps and `go.work` for independent Go modules. Ship one Go binary with embedded web assets. Keep runtime roles behind `--role`.

## Notes
- Language packages may live under `packages/<language>/` (e.g. `packages/python/orcon` for the Python SDK). (SPEC-005, accepted 2026-10-12.)
