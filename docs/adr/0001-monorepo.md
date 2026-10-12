# ADR 0001: Hybrid monorepo

Status: Accepted

Use pnpm + Turborepo for web apps and `go.work` for independent Go modules. Ship one Go binary with embedded web assets. Keep runtime roles behind `--role`.

## Notes
- Language packages may live under `packages/<language>/` (e.g. `packages/python/orcon` for the Python SDK). (SPEC-005, accepted 2026-10-12.)
