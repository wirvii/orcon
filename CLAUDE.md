# Claude Code instructions

Follow [AGENTS.md](AGENTS.md) and [docs/engineering/implementation-contract.md](docs/engineering/implementation-contract.md). All implementation work must derive from accepted requirements and ADRs. Stop for unresolved architectural decisions rather than guessing.

For any Go change, use the `orcon-go` skill (`.claude/skills/orcon-go/`, generated from `.agents/skills/orcon-go/` by `make skill-sync`; edit only the latter). The human overview is [docs/engineering/go.md](docs/engineering/go.md).

<!-- mneme:managed:start v=1 -->
Process and operating instructions are managed globally via mneme.
See the mneme operating manual in your global ~/.claude/CLAUDE.md.

Project scope: see this file's sections below for stack, conventions, and module structure.
<!-- mneme:managed:end -->
