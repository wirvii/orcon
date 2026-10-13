<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0060 — Durable AI agent execution: stepped agent runs on the token engine

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: table names per ADR-0077; UPPER_SNAKE values per ADR-0080 §3) (amended 2026-10-13: stored enumerated values are UPPER_SNAKE, ADR-0015 §2)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
Principle 11 requires that LLM calls, tools, approvals, timers and waits use the shared state/recovery semantics; `vision.md` forbids a second runtime for AI agents. Owner decision A1 (BL-006): every model call and every tool call is a durable step; model responses are recorded as nondeterminism so recovery never re-queries the model for a recorded response (no re-billing, same decisions); write tools go through the external-operation journal (ADR-0005/0012); the agent resumes at the exact step; human approval for sensitive tools is a durable wait; per-agent caps on steps, spend and time. The n8n model (the whole loop inside one node, restarted from scratch on crash) is rejected.

## Decision

### 1. Agentic node class
Amend ADR-0008 §5 with a fifth purity class `AGENTIC`: a node whose activation is a sequence of durable steps with its own persisted state and which may enter WAITING several times. Agent nodes (`orcon.ai.agent@1`) and multi-step chains (`orcon.ai.summarize@1` `MAP_REDUCE`/`REFINE`, `orcon.ai.qa@1`, repair loops) are `AGENTIC`. Single-call chains (`orcon.ai.llm@1`, `orcon.ai.extract@1`, `orcon.ai.classify@1`, `orcon.ai.sentiment@1`) are `AGENTIC` with exactly one model step plus optional repair steps.

### 2. Agent run
- One run per item (`itemMode: PER_ITEM`, default) or per batch (`BATCH`). Run id `H(nodeExecutionKey, itemIndex, "agent", parentRunId?)` (ADR-0008 §6 hashing).
- `ai_agent_run(org_id, id, execution_id, node_execution_id, item_index, parent_ai_agent_run_id, depth, status, step_count, tool_call_count, spend_microdollars, active_milliseconds, is_tainted, loaded_skill_document, final_output_reference, error_document, created_at, updated_at)`; status: `RUNNING`, `WAITING` (approval, child flow, chat reply), `SUSPENDED` (UNCERTAIN tool, budget `SUSPEND`), `SUCCEEDED`, `FAILED`, `CANCELED`.
- `ai_agent_step(org_id, id, ai_agent_run_id, step_number, kind: MODEL|TOOL|APPROVAL|RETRIEVAL|MEMORY|SKILL, status, attempt_number, ai_model_call_id?, ai_tool_call_id?, started_at, finished_at)`, unique `(org_id, ai_agent_run_id, step_number)`.
- `ai_agent_message(org_id, id, ai_agent_run_id, sequence_number, role, provenance, content_document | content_blob_id, is_redacted, token_count)` — append-only transcript; documents > 256 KiB are blobs (ADR-0016). The context for a step is rebuilt by reading rows, never by rewriting the transcript.

### 3. Step protocol
1. **Run start** (confirmation point): run row, memory read watermark (ADR-0064), retrieval records, first step row `MODEL/CALLING attempt=1`.
2. **Model step**: outside any transaction, call the provider (ADR-0061) with a local budget reservation (ADR-0062). On response, a **mandatory** confirmation point persists in one fenced transaction: `ai_model_call` (redacted request reference, response, usage, cost, price-table version, model actually used incl. fallback), the assistant message, cost rollups and budget settlement, and, for every requested tool call, its intent: journal rows PREPARED (effect tools, ADR-0012), child execution rows (flow tools, ADR-0010 §7), `human_task` rows (approval, §6), or nothing (pure/read tools run immediately after the commit). If the response is final (no tool calls) the same transaction completes the run and emits the node output.
3. **Tool step(s)**: executed with bounded concurrency (`maxParallelToolCalls`, default 4) after the intent commit. Read/pure tool outputs are persisted at the next confirmation point; effect tools follow the journal; flow tools put the token in WAITING until children finish; approvals put the token in WAITING.
4. **Next model step**: when every tool call of the step is resolved, one confirmation point persists the tool result messages, the next step row (`MODEL/CALLING`), and releases or keeps the lease.
Steady state: one transaction per model step without tools; two per model step with tools; one at run start. No transaction is open during any network call.

### 4. Recovery
- A `MODEL` step with a recorded `ai_model_call` in `SUCCEEDED` is never re-issued; recovery reads it.
- A `MODEL` step in `CALLING` without a recorded response is re-issued exactly once with `attempt+1`; the lost attempt is recorded with cost status `POSSIBLY_BILLED` and its reserved estimate (budget permits of the lost lease are not returned). This single retry is documented to users. It does not conflict with the never-retry rule for dispatched `UNSAFE` writes: a model call has no external side effect on user systems, only its cost can be duplicated. Recovering a lost response by id from providers that offer it (background/stored responses) is a later per-provider optimization, not a V1 requirement.
- Tool steps recover per their class: journal recovery (ADR-0012 §6), child executions already linked, reads re-executed.
- Approval waits survive any restart (ADR-0010 §8).
- Recorded nondeterminism (ADR-0013 §7) is extended with model responses, retrieval results, memory watermarks and MCP tool lists; values observed outside the process (model calls) are flushed at the mandatory confirmation point, not lazily.

### 5. Caps
`maxSteps` (default 10), `maxToolCalls` (50), `maxParallelToolCalls` (4), `maxSpend` (unset), `maxActiveTime` (10 min of RUNNING time; waits excluded; accumulated at checkpoints like ADR-0011 §5), `maxWallTime` (unset), `maxNestedDepth` (3). Exceeding a cap raises the permanent error `AgentLimitExceeded{kind}` routed by the node's error rules, unless `onLimit: FINAL_ANSWER` requests one last tool-less model call (counted as a step).

### 6. Approval waits
A tool call requiring approval (ADR-0066 §5, ADR-0068 §5) creates a `human_task` of kind `TOOL_APPROVAL` with `{tool, arguments (redacted form), reason, runId, stepNo, callIndex}` and a mandatory timeout (default 24 h, maximum 30 days; a larger value is a publication error). Resolution `approve | deny(reason) | approveWithEdits(arguments)` makes the token RUNNABLE; timeout yields a `denied(timeout)` tool result (default) or a failure (`onApprovalTimeout: FAIL`). Approval never comes from an API key principal on the `ORCON` channel.
Pending approvals use the human-attention notification system of ADR-0073 (SPEC-001), which this ADR does not redefine: creating the task emits `execution.awaiting_human` (flow, step, tool, placeholder-form arguments, reason such as tainted run, expiry), so approvers are notified immediately on their configured channels; if nobody acts within the configured escalation delay (which must be shorter than the expiry), `execution.awaiting_human.escalated` notifies the secondary contact or administrator; approval, denial, edit or expiry emits `execution.human_resolved` (expiry reported as resolved by timeout). Runs SUSPENDED by an UNCERTAIN tool (§7) or by a budget whose exceeded action is `SUSPEND` (ADR-0062 §4) use the same events. The `ai.approval.*` events remain AI-specific detail; notifications are driven by the ADR-0073 events.

### 7. Uncertainty and cancellation
An UNCERTAIN effect tool suspends the run (ADR-0012 §7); the model is never told a guessed outcome. Supervised resolution feeds the resolved result back and resumes. Cancellation aborts the in-flight model call (recorded, possibly billed), cancels pending approvals and awaited child flows, and lets DISPATCHING effects finish their lifecycle (ADR-0011 §6).

### 8. Skills and reusable agents
Skills are named bundles `{name, description, instructions, tools[]}` declared on the agent node. The model sees names and descriptions; the built-in tool `load_skill` (a recorded step) appends the instructions and enables the tools. A reusable agent is a flow (chat/channel/called-by-flow trigger + agent node); "message an agent" is a sub-flow call. No separate agent entity.

### 9. Built-in tools and chains
Built-in tools: `think`, `calculator`, `load_skill`, `final_answer` (structured output), `knowledge_search` (ADR-0065), `code` (ADR-0036 sandbox). Tool names are identifiers and keep their form. Chain nodes listed in SPEC-006 R-CH-01 use the same step protocol with fixed step plans.

### 10. Logical schema additions (ADR-0015, as amended by ADR-0077)
`ai_agent_run`, `ai_agent_step`, `ai_agent_message`, `ai_model_call(org_id, id, execution_id, ai_agent_run_id, step_number, attempt_number, purpose, provider, model, model_version, fallback_index, request_reference, request_hash, response_reference, finish_reason, usage_document, usage_document_format, cost_microdollars, cost_status: EXACT|ESTIMATED|POSSIBLY_BILLED|UNKNOWN, price_table_version, latency_milliseconds, provider_request_id, status, error_document, created_at)`, `ai_tool_call(org_id, id, ai_agent_run_id, step_number, call_index, source, tool_name, arguments_reference, external_operation_id?, child_execution_id?, human_task_id?, status, result_reference, provenance)`. All with `org_id`, composite foreign keys and forced RLS (ADR-0046); names, document columns and identifiers follow the database rulebook (`provider_request_id` is an external identifier, listed in `external_ids.txt`). Enumerated columns store the UPPER_SNAKE values the API exposes (ADR-0015 §2).

### 11. BPMN export
The agent node exports as `adHocSubProcess` with `orcon:agent` extension elements carrying the agent block; tools are listed as extension data. Round trip is lossless (ADR-0007 §8).

## Consequences
Agents share leases, fencing, checkpoints, journal, waits and cancellation with every other node; crashes never re-bill recorded calls nor duplicate journaled effects; approvals cost no memory while waiting. Steps add one or two local transactions per model call, negligible compared with model latency and measured by ADR-0072.

## Alternatives considered
- **Whole loop in one node, restart on crash (n8n)**: rejected by A1.
- **Compiling the loop into a static BPMN graph (loop node + model node + tool switch)**: rejected — tool choice is dynamic, parallel tool calls and approvals per call do not map to a fixed graph, and the editor would show engine internals.
- **One child execution per step**: rejected — multiplies execution rows and history noise without adding guarantees.
- **Lazy flushing of model responses at the next periodic checkpoint**: rejected — the response was observed and billed externally.

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-009 K19 (2026-10-13, owner decision).** §2, §3, §4, §6 and §10 (`ai_agent_step.kind`, `ai_model_call.cost_status`, `human_task.kind` `TOOL_APPROVAL`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
