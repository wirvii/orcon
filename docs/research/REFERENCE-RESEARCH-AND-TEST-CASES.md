# Orcon — Research and Design Reference Appendix

**Status:** research notes and proposed requirements, not accepted implementation contracts. **Research snapshot:** October 2026. Verify current upstream versions before implementing.

## Competitive reference: n8n

### Feature checklist for parity research
- Workflow authoring: trigger nodes, manual starts, scheduling, webhooks, HTTP requests, data mapping, expressions, transformations, conditionals, branches, merges, loops, batches, waits, subworkflows, error workflows, retries and timeouts.
- Integration ecosystem: official/community nodes, credentials, HTTP and custom code, versioned node types.
- Operation: manual execution, history, logs, debugging, execution retry/resume, distributed queue workers, concurrency and resource management.
- Developer features: APIs, MCP, workflow SDK, workflow JSON, environment management and source control.
- Agentic features: model providers, AI Agent node, RAG/vector storage, memory, tool use, subagents, human approvals, evaluations, newer Agents and Assistant previews.
- Enterprise features: SSO, governance, projects, Git/environments, multi-main, secrets, support, plan-dependent feature gating.
- Reliability and security: queue-mode granularity, branch order, memory pressure, untrusted community extensions, task runner isolation, tracing preview.

### Primary sources
- License and permitted use: https://docs.n8n.io/n8n-community-license/community-license/license-faq/
- Community/enterprise differences: https://docs.n8n.io/hosting/community-edition-features/
- Queue mode: https://docs.n8n.io/hosting/scaling/queue-mode/
- Branch order: https://docs.n8n.io/build/flow-logic/understand-execution-order.md
- Code task runners: https://docs.n8n.io/deploy/host-n8n/configure-n8n/set-up-task-runners.md
- Memory errors: https://docs.n8n.io/hosting/scaling/memory-errors/
- AI agents: https://docs.n8n.io/integrations/builtin/cluster-nodes/root-nodes/n8n-nodes-langchain.agent
- New Agents preview: https://docs.n8n.io/build/build-and-manage-agents.md
- Assistant: https://docs.n8n.io/build/ways-of-building-workflows/n8n-assistant
- Human approval: https://docs.n8n.io/build/integrate-ai/ai-examples/human-in-the-loop-for-tools
- MCP: https://docs.n8n.io/connect/connect-to-n8n-mcp-server/mcp-server-tools-reference
- OpenTelemetry: https://docs.n8n.io/deploy/host-n8n/keep-n8n-running/trace-executions-with-opentelemetry
- Wait: https://docs.n8n.io/integrations/builtin/core-nodes/n8n-nodes-base.wait/
- Data Tables: https://docs.n8n.io/build/work-with-data/data-tables
- Prices: https://n8n.io/pricing/

## Critical semantic examples for tests

**External success then worker crash:** worker sends invoice creation; provider commits; worker crashes before local commit. Recovery must not blindly repeat; use durable correlation/idempotency within provider TTL, reconcile, or suspend UNCERTAIN.

**Multiple external writes:** create customer, create invoice, register payment. If payment is ambiguous, earlier writes remain real; a workflow-wide "rollback" is impossible without compatible transactional participants or explicit compensation. A Saga cannot promise ACID atomicity.

**Lease expiration:** worker A loses heartbeat while executing remote call; worker B obtains lease. Fencing prevents A from committing stale local state but cannot retroactively stop remote side effects. Need operation-level deduplication and recovery policy.

**Idempotency expiration:** provider guarantees a key for 24 hours but workflow resumes after 72 hours. Automatic retry cannot assume the original guarantee still holds.

**Compensation race:** a compensation may overwrite legitimate external updates made after the original write. Verify preconditions, version/ETag and actual external state; suspend if unsafe.

**2PC coordinator failure:** durable prepare/commit decision must survive coordinator crash. All participants must actually support prepare and durable commit/abort. Never simulate 2PC by sending ordinary HTTP writes.

**TCC partial confirm:** some participants may confirm while others time out; design reconciliation, expiration, confirm idempotency and manual resolution.

## UX implications

Workflow editor must visually distinguish: invalid safety contract, verified safe write, guarded/supervised operation, pending approval, UNCERTAIN remote result, compensation in progress, failed compensation, blocked consistency group. Never rely solely on color. Provide actionable explanations and repair suggestions. Importer should identify n8n nodes requiring explicit safety contracts. The workflow must not silently activate until required policies are valid.

## Planned quality gates

- Deterministic graph state transition tests.
- Transaction rollback tests for node result + event + downstream task.
- Worker kill/lease/fencing integration tests.
- Side-effect ambiguity fault injection with mock remote APIs.
- Provider idempotency TTL and reconciliation tests.
- Credential isolation, secret redaction, tenant quotas and malicious connector tests.
- Workflow migration fixture tests pinned to explicit n8n version.
- End-to-end single-binary test serving embedded React build.
- Benchmarks comparing equivalent n8n and Orcon workloads (CPU, RSS, throughput, p50/p95/p99 latency, crash recovery and external duplicates).
- Accessibility tests for canvas, node editor, approval and debugging UI.

## Implementation order suggested, not yet approved

1. Freeze execution model and invariants via ADR.
2. Define canonical graph/schema and storage contracts.
3. Define EOSC and the uncertainty protocol.
4. Design API, security, connector SDK and workflow editor UX.
5. Build minimal durable engine with fault injection.
6. Add familiar visual editor, importer and initial connector set.
7. Add agentic features and distributed hardening.
8. Publish evidence-based compatibility/reliability results.

## References to read together

- docs/research/project-background.md
- docs/product/vision.md
- docs/product/principles.md
- docs/product/n8n-compatibility.md
- docs/architecture/overview.md
- docs/design-system/foundations.md
- docs/engineering/implementation-contract.md
- docs/adr/0001-monorepo.md
