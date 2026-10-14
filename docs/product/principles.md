<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Orcon — Product Principles

Status: **Accepted** | Version: 0.1

1. **Freedom by default.** Apache-2.0 licensing for the intended platform, with third-party dependency licenses audited. No proprietary edition or paid unlocks.
2. **Engine first.** Durable, deterministic orchestration semantics take priority over visual polish and connector count. No unsupported claims of exactly-once side effects.
3. **Easy first run.** One Go binary contains the server and compiled React editor. Local mode targets zero external service dependencies; production distributed mode uses PostgreSQL.
4. **One artifact, many roles.** The same binary can serve API/editor, scheduling, worker execution, or standalone combined operation.
5. **Operational minimalism.** PostgreSQL is the durable distributed source of truth. Redis is optional, not a requirement. SQLite is the intended local backend.
6. **Portability and interoperability.** Exportable versioned workflow definitions, documented API and SDK contracts, HTTP/MCP interoperability, no forced proprietary cloud.
7. **Honest migration.** Familiar interaction and n8n import are adoption goals, not promises of full drop-in compatibility. Surface semantic differences before activation.
8. **Security as an execution boundary.** Permissions, tenant isolation, secret handling, connector isolation, and human approvals are enforced outside model prompts.
9. **Observable by design.** Execution state, attempts, timing, errors, and replay/debug traces must be explainable. Sensitive values must be redacted.
10. **Extensible without compromising the host.** Official trusted connectors and third-party extensions have separate trust and execution policies.
11. **Agentic automation uses the same core.** LLM calls, tools, approvals, timers and waits use shared state/recovery semantics.
12. **Evidence over slogans.** Benchmark Go vs n8n under comparable workloads; never assert performance superiority based solely on implementation language.

## Specification discipline

Mark each document or decision **Proposed**, **Accepted**, **Experimental**, or **Superseded**. Acceptance criteria must be measurable. Implementers may choose implementation details only where specifications explicitly delegate them. Changes to architecture require an ADR.
