# ADR 0001: Hybrid monorepo

Status: Accepted

Use pnpm + Turborepo for web apps and `go.work` for independent Go modules. Ship one Go binary with embedded web assets. Keep runtime roles behind `--role`.
