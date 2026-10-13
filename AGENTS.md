# Instructions for coding agents

Read `docs/product/vision.md`, `docs/product/principles.md`, `docs/architecture/overview.md`, `docs/engineering/implementation-contract.md`, and relevant ADRs **before implementing**.

This repository is an early scaffold. Do not infer that an engine, persistence layer, or worker is production-ready. Decisions labeled Proposed or Experimental are not authorization to implement them as settled architecture.

Preserve Go multi-role binary and embedded React editor architecture. The product is intended to be fully Apache-2.0 open source. Do not copy n8n implementation or proprietary source.

Do not modify foundational architecture without an accepted ADR. Include tests and verification evidence for code changes.

## Go code

Before writing, reviewing or refactoring Go code, `go.mod`, SQL, generated API handlers or Go CI configuration, load the `orcon-go` skill (`.agents/skills/orcon-go/`). Its `references/` folder is the complete Go rulebook; [docs/engineering/go.md](docs/engineering/go.md) is the short human overview. Edit the skill only under `.agents/skills/orcon-go/` and re-run `make skill-sync`. `make check` is the single gate, locally and in CI.

## Web UI code

Before writing, reviewing or restyling anything under `apps/web` or `packages/js` (components, canvas, editor, tokens, screens, i18n catalogs, UI checks), load the `orcon-ui` skill (`.agents/skills/orcon-ui/`). Its `references/` folder holds the design rules and [docs/design-system/README.md](docs/design-system/README.md) the owner review record. Edit the skill only under `.agents/skills/orcon-ui/` and re-run `make skill-sync`; the `.claude/skills/orcon-ui/` copy is generated.
