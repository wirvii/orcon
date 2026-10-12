<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0066 — Agent tools: sources, schemas, permissions, external-operation safety and approval

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: UPPER_SNAKE values per ADR-0080 §3)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
A4: any connector operation (description auto-generated from the manifest), any flow as a tool (user description, full engine guarantees), external MCP servers, other agents as tools; every tool honours SPEC-004 permissions/credentials, EOSC duplicate protection and optional human approval.

## Decision

### 1. Sources
1. **Connector operation** with `ai.tool.exposable` (default true): name `<connector>_<operation>`, description from manifest `displayName`/`description` and parameter descriptions, input schema from the parameters the author leaves to the model with `$fromAI(name, description, type, default?)`; fixed parameters are invisible and immutable to the model.
2. **Flow as tool**: call activity (ADR-0010 §7) to a published flow with a called-by-flow trigger; input schema from that trigger; user-written description; wait mode; output bounded.
3. **MCP tool** (ADR-0067).
4. **Agent as tool**: inline sub-agent (nested run, parent caps and spend), or a flow-agent through source 2.
5. **Built-in**: ADR-0060 §9; HTTP request tool (`orcon.http`, EOSC per method); code tool (ADR-0036).

### 2. Identity and credentials
Tools execute as the flow's project (SPEC-004 R-AZ-09). Credentials are those configured on the tool; `credential:use` and sharing are re-checked at every dispatch; host binding applies (R-CRED-03). The model never selects credentials, hosts or projects.

### 3. Safety
Connector effect operations use their contract (ADR-0029 §5) and journal per call with the extended correlation key; flow tools inherit their nodes' guarantees; MCP tools without a trusted read-only declaration are `UNSAFE` writes; HTTP: GET/HEAD `READ`, other methods `UNSAFE` unless a key is declared. UNCERTAIN suspends the run.

### 4. Results
Bounded (default 16 KiB per result; overflow to blob with truncation marker), redacted (ADR-0068), tagged with provenance (`EXTERNAL` by default; a flow tool may declare `SYSTEM`).

### 5. Approval
Per tool `approval: NEVER | ALWAYS | AFTER_UNTRUSTED | POLICY` (default `AFTER_UNTRUSTED` for sensitive tools; sensitive = effect `WRITE`/`DELETE`/`UNKNOWN` or `sensitive: true`). Default approval expiry 24 h, maximum 30 days (ADR-0060 §6); pending approvals notify approvers immediately and escalate through ADR-0073. Channels: `ORCON` (human task; a person with an interactive session and `human-task:resolve` in the project; never an API key principal), `LINK` (a link delivered by an ADR-0073 notification or a notification flow to a member of the organization; it only opens the authenticated approval page in Orcon, where that person approves with an interactive session and `human-task:resolve`; the link alone never authorizes the approval — SPEC-004 R-AZ-13, owner decisions SPEC-004 N2 and SPEC-006 N1), `CHAT_USER` (the conversation's user confirms in the widget; not sufficient for sensitive tools in runs with public input, ADR-0068 §5). Approvers may approve, deny with reason, or approve with edited arguments (recorded and audited). The taint policy and the public-input floor (ADR-0068 §5) can force approval regardless of the tool setting.

### 6. Pinning
Tool names, descriptions and schemas (including MCP tool lists) are resolved at publication and stored with a hash in the version; runtime drift → `onToolDrift: FAIL | WARN` (default `FAIL` for sensitive tools).

## Consequences
Tools are as safe as the operations behind them; the model cannot widen authority.

## Alternatives considered
Tools receiving raw credentials (rejected: ADR-0029 §6); trusting MCP annotations (rejected: hints from untrusted servers); approval inside the model prompt only (rejected by principle 8).
