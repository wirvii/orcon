# Instructions for coding agents

Read `docs/product/vision.md`, `docs/product/principles.md`, `docs/architecture/overview.md`, `docs/engineering/implementation-contract.md`, and relevant ADRs **before implementing**.

This repository is an early scaffold. Do not infer that an engine, persistence layer, or worker is production-ready. Decisions labeled Proposed or Experimental are not authorization to implement them as settled architecture.

Preserve Go multi-role binary and embedded React editor architecture. The product is intended to be fully Apache-2.0 open source. Do not copy n8n implementation or proprietary source.

Do not modify foundational architecture without an accepted ADR. Include tests and verification evidence for code changes.

## Go code

Before writing, reviewing or refactoring Go code, `go.mod`, SQL, generated API handlers or Go CI configuration, load the `orcon-go` skill (`.agents/skills/orcon-go/`). Its `references/` folder is the complete Go rulebook; [docs/engineering/go.md](docs/engineering/go.md) is the short human overview. Edit the skill only under `.agents/skills/orcon-go/` and re-run `make skill-sync`. `make check` is the single gate, locally and in CI.

## Web UI code

Before writing, reviewing or restyling anything under `apps/web` or `packages/js` (components, canvas, editor, tokens, screens, i18n catalogs, UI checks), load the `orcon-ui` skill (`.agents/skills/orcon-ui/`). Its `references/` folder holds the design rules and [docs/design-system/README.md](docs/design-system/README.md) the owner review record. Edit the skill only under `.agents/skills/orcon-ui/` and re-run `make skill-sync`; the `.claude/skills/orcon-ui/` copy is generated.

## Connectors

Before creating, changing or reviewing a connector under `internal/adapter/connectors/` or a catalog submission, load the `orcon-connector` skill (`.agents/skills/orcon-connector/`). Its `references/` folder is the complete connector rulebook; [docs/engineering/connectors.md](docs/engineering/connectors.md) is the short human overview. Native connector code also follows `orcon-go`. Edit the skill only under `.agents/skills/orcon-connector/` and re-run `sh .agents/skills/orcon-connector/scripts/project.sh`.

## Public API

Before adding, changing or reviewing an endpoint, event, webhook payload, CLI command or MCP tool in the OpenAPI contract (`packages/openapi/`), load the `orcon-api` skill (`.agents/skills/orcon-api/`). Its `references/` folder is the complete API design rulebook; [docs/engineering/api.md](docs/engineering/api.md) is the short human overview. Code generation and handlers stay in `orcon-go`. Edit the skill only under `.agents/skills/orcon-api/` and re-run `sh .agents/skills/orcon-api/scripts/project.sh`.

## Database schema and SQL

Before creating or changing a table, column, index, constraint, migration, SQL query or storage adapter, load the `orcon-db` skill (`.agents/skills/orcon-db/`). Its `references/` folder is the complete database rulebook; [docs/engineering/db.md](docs/engineering/db.md) is the short human overview. Go code that reaches the database also follows `orcon-go`. Edit the skill only under `.agents/skills/orcon-db/` and re-run `sh .agents/skills/orcon-db/scripts/project.sh`.

## Web application code

Before writing, reviewing or refactoring TypeScript or React code in `apps/web` or `packages/js`, load the `orcon-web` skill (`.agents/skills/orcon-web/`), and the `orcon-ui` skill when the change touches anything visual. Its `references/` folder is the complete web application rulebook; [docs/engineering/web.md](docs/engineering/web.md) is the short human overview. Edit the skill only under `.agents/skills/orcon-web/` and re-run `sh .agents/skills/orcon-web/scripts/project.sh`.

## Contributing

Branches, commits, pull requests and releases follow the `orcon-contributing` skill (`.agents/skills/orcon-contributing/`).
