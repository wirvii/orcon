# Orcon — Handoff for Mneme

Date: 2026-10-10. Status: historical context, not a blanket implementation approval.

## Vision

Orcon (Orchestration Controller) is a genuinely open-source, Apache-2.0 alternative to n8n. The aim is community adoption, not direct monetization. Users must be free to fork, embed, modify, self-host and commercialize. Independent implementation, never a fork or direct port of n8n source. The founder previously explored N2N (Node to Node), a BPM engine.

Our role in this conversation is **only** architecture, ADRs, product specifications, brand kit, design system and agent handoff. Local Mneme agents implement and test. Proposed ideas are not automatically approved.

## Agreed technology

Go core, React/React Flow editor, frontend assets embedded using embed.FS in a single Go executable. Roles: standalone, api, scheduler, worker. Local SQLite without external dependencies; PostgreSQL for distributed durable state and task queue; Redis optional, not mandatory. Go workspace and pnpm/Turborepo hybrid monorepo. Temporal and Hatchet are architectural references, not required dependencies. The existing code is a scaffold, not a functioning engine.

## n8n research (October 2026; revalidate)

n8n stands for nodemation. It is source-available under Sustainable Use License, not OSI open source. Its implementation is largely TypeScript/Node.js with Vue editor, SQLite/PostgreSQL, Redis/Bull queue workers and JS/Python task runners. Community includes distributed queue mode; queue workers distribute executions, not automatically individual nodes. n8n already has waits/resume, branches, integrations, agents, human approvals, RAG, MCP, workflow SDK, Assistant preview, OpenTelemetry preview and execution history. Never claim those features are absent. Some governance functions are paid. Official documentation: https://docs.n8n.io/llms.txt ; license: https://docs.n8n.io/n8n-community-license/community-license/license-faq/ ; queue: https://docs.n8n.io/hosting/scaling/queue-mode/ ; code: https://github.com/n8n-io/n8n .

Migration requires an independent importer, version-pinned compatibility matrix, semantic diagnostics for nodes/edges/expressions/credentials/triggers/loops/AI, safe fixture tests, and explicit user review. No silent conversion or activation of unsafe operations.

## Temporal and Hatchet

Temporal: MIT self-hosted server; workflow/activity separation, durable history, deterministic replay, worker tasks, timers and versioning. Hatchet: Go-oriented durable task orchestration, PostgreSQL, scheduling, concurrency and workers. References: https://docs.temporal.io/ ; https://github.com/hatchet-dev/hatchet ; https://github.com/hatchet-dev/durable-execution-the-hard-way . Study patterns without requiring their servers.

## Execution design direction

Materialized persistent execution state **plus** append-only event history, updated in one local transaction. Not pure event sourcing. Resume from confirmed state before considering full deterministic replay. WorkflowDefinition immutable per version; WorkflowExecution, NodeExecution, TaskAttempt, ExecutionEvent, leases, timers and external operations separately modeled. Persist node completion, event and dependent runnable tasks atomically. Do not hold DB transactions while calling remote APIs. Worker leases require fencing. Node attempt and logical node state differ. External ambiguity is a first-class state. Parallel branches, joins, loops, cancellation, retry, data merge and history schema remain to be specified. Large payloads should use references rather than unrestricted JSON copying.

## Safety: the most important principle

External data integrity outranks uninterrupted execution. Never automatically retry an ambiguous non-idempotent remote write, and never record uncertain remote effects as successful. If no safe policy is available, block automatic activation/dispatch or suspend for explicit supervised resolution. Explicit acceptance of risk is permitted only under supervised execution, not as a silent bypass.

Two conceptual contracts:
1. **External Operations Safety Contract (EOSC)** per node: effect type; idempotency key/scope/TTL; authoritative verification/correlation; compensation/preconditions; failure policy (safe retry, reconciliation, suspension, manual); static and runtime checks. Persist operation intent before dispatch. States proposed: PREPARED, DISPATCHING, VERIFYING, SUCCEEDED, FAILED, UNCERTAIN. Official connectors should have verified contracts; custom connector claims are not automatically trusted.
2. **Consistency Group Contract** across related writes: desired business guarantee, participant capabilities, protocol, verification, compensation and failure resolution. Strategies considered: 2PC (strict atomicity only with prepared-transaction participants), TCC (reservation/confirm/cancel, not strict atomicity), Saga (compensation, not atomicity), guarded execution and supervised execution. Transactional outbox is local atomicity, not remote distributed atomicity. Three-phase commit and event sourcing do not magically guarantee arbitrary API atomicity.

**Latest decision:** advanced coordination strategies are **optional in ADRs**, not mandatory V1. Mandatory safe side-effect behavior remains. Do not claim exactly-once remote effects for arbitrary APIs.

## Brand and design

Working name Orcon, tagline "Build. Orchestrate. Execute. Your way." Identity: original muscular, intelligent, friendly **orc engineer/guardian** mascot; distinct minimal geometric logo. No copying characters from fictional franchises. Palette direction: dark charcoal, emerald, mint/teal, white. Preliminary unvalidated colors: #10221B, #22C55E, #BAF4C9, #F7FAF8, #334155. Prior generated concept boards showed mascot with laptop, wordmark, icon, UI and merchandise. Full master SVGs and brand kit are **not yet confirmed in GitHub**. Need complete accessible design system: semantic tokens, dark/light themes, typography, spacing, components, workflow editor states, keyboard accessibility, WCAG 2.2 AA, icons, mascot variants, usage rules. The name has potential pre-existing commercial use; do not block work, but do not assert legal clearance.

## Agent instructions

Read AGENTS.md, CLAUDE.md, docs/product/*, docs/architecture/*, docs/design-system/* and accepted ADRs. Accepted ADRs supersede proposals and scaffold. Open ADRs include execution semantics, storage, EOSC, optional coordination, workflow graph, compatibility, connector isolation, AI/MCP, security, observability, editor, design system, deployment and benchmarks. Mneme should use bounded risk-based review rather than endless expensive certification. No implementation without acceptance criteria and approved scope.
