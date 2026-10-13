<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0007 — Canonical flow definition format `orcon/flow@1`

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12; 2026-10-13

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
ADR-0003 requires immutable, version-pinned workflow graphs. The scaffold `workflow.Definition{ID, Version, Nodes, Edges}` is a placeholder. The owner decided (BL-001 D2, D8, D21) that Orcon uses its own public JSON format with full BPMN 2.0 semantics ("BPMN inside, n8n outside"), draft + publish versioning, and lossless BPMN XML export. V1 exposes only the n8n-style editor; a BPMN viewer and BPMN editing come later and must not require reworking the format or the engine. The API returns the flow document verbatim, so its enumerated values follow ADR-0080 §3 (§11).

## Decision

### 1. Document
- JSON (RFC 8259), UTF-8. Top-level `"format": "orcon/flow@1"`.
- Normative JSON Schema (draft 2020-12): `internal/adapter/codec/flowjson/schema/orcon-flow-1.schema.json`; human reference: `docs/reference/orcon-flow-1.md`.
- `additionalProperties: false` everywhere except inside `ext` objects. `ext` keys must be namespaced (`"vendor.key"`); `orcon.*` is reserved.

### 2. Top-level fields
`format`, `id` (stable flow id), `name`, `description?`, `settings`, `variables[]`, `nodes[]`, `edges[]`, `editor` (visual-only data: positions, sizes, sticky notes), `ext?`.

`settings`: `branchMode` (`"PARALLEL"` | `"SEQUENTIAL"`, default `PARALLEL`; importer sets `SEQUENTIAL`), `priority` (`"HIGH"` | `"NORMAL"` | `"LOW"`), `concurrency` (execution-level token limit), `timeouts.executionActive` (ISO-8601 duration, optional), `retention` (ADR-0017; `"ALWAYS"` | `"FAILED_ONLY"` | `"NEVER"` | `"DEFAULT"`), `memoryLimit` (ADR-0016), `errorFlow?` (flow id), `callDepthLimit?`, `importedFrom?` (n8n provenance: source version, import report id).

### 3. Nodes
Common fields: `id` (`[A-Za-z0-9_-]{1,64}`, unique, stable across versions), `name` (unique, used by `$('Name')`), `element` (BPMN element class: `startEvent`, `endEvent`, `task`, `serviceTask`, `scriptTask`, `userTask`, `receiveTask`, `sendTask`, `callActivity`, `exclusiveGateway`, `inclusiveGateway`, `parallelGateway`, `eventBasedGateway`, `complexGateway`, `intermediateCatchEvent`, `intermediateThrowEvent`, `boundaryEvent`; names fixed by BPMN 2.0), `type` (`<namespace>.<name>@<major>`, e.g. `orcon.http@1`, `orcon.loop@1`, `orcon.if@1`), `parameters` (object; string values starting with `=` are expressions, ADR-0018), `disabled?`, `ext?`.

Behaviour blocks (each optional, validated against `element`):
- `routing`: for item-routing nodes (`orcon.if`, `orcon.switch`): `"EXCLUSIVE"` | `"INCLUSIVE"` (ADR-0010 §4).
- `join`: `{ mode: "ALL" | "ACTIVATED" | "FIRST", data: "APPEND" | "MATCH_FIELD" | "MATCH_POSITION", matchField? }`.
- `loop`: `{ maxIterations: int ≥ 1, carry: [variableName] }` — only on `orcon.loop` nodes.
- `multiInstance`: `{ sequential: bool, concurrency: int | "ALL", inputCollection?: expr, batchSize: int ≥ 1, completionCondition?: expr }`.
- `retry`: `{ maxAttempts, initialInterval, factor, maxInterval, respectRetryAfter }`.
- `timeouts`: `{ attempt?, node? }`.
- `onError`: `"STOP_EXECUTION" | "CONTINUE_WITH_ERROR" | "ERROR_OUTPUT"`.
- `concurrency`: per-node item concurrency (`int` | `"ALL"`).
- `compensation`: `{ handler: nodeId, precondition?: expr }`.
- `call` (callActivity): `{ target: flowId | expr, allowedTargets?: [flowId], version: "CURRENT" | int, mode: "WAIT" | "FIRE_AND_FORGET", perItem: bool, intentionalRecursion?: { maxDepth: int } }`.
- `wait` (receiveTask / userTask / intermediateCatchEvent): `{ kind: "TIMER" | "SIGNAL" | "HUMAN", timer?: { duration? | date? }, timeout?: duration, form?: schema }`.
- Boundary events: `attachedTo: nodeId`, `cancelActivity: bool`, `eventDefinition: { timer: {duration|date|cycle} } | { error: { codes?: [string] } } | { compensation: {} }`.
- `effect`: reference to the node type's EOSC contract and per-node overrides allowed by that contract (ADR-0012).
- `attention` (wait / userTask / effect nodes): `{ notify: "DEFAULT" | "ON" | "OFF" }` — overrides the human-attention notification default of the wait kind (ADR-0073 §1a); `OFF` on a mandatory kind is a validation error.

### 4. Edges
`{ id, from: { node, port }, to: { node }, condition?: expr, default?: bool, kind: "SEQUENCE" | "BACK", order?: int }`. Ports are named (`main`, `true`, `false`, `case:<key>`, `error`, `timeout`, `done`, `loop`); port names are identifiers, not enumerated values. `kind: "BACK"` is legal only when `to.node` is an `orcon.loop` node. `order` is used only in `SEQUENTIAL` branch mode.

### 5. BPMN mapping
Every node and edge maps to exactly one BPMN 2.0 element: activities → `task`/`serviceTask`/`scriptTask`/`userTask`/`receiveTask`/`sendTask`/`callActivity`; item-routing nodes → `inclusiveGateway` with `orcon:itemRouting` (`"EXCLUSIVE"` or `"INCLUSIVE"`); joins → `parallelGateway` (`ALL`), `inclusiveGateway` (`ACTIVATED`), `complexGateway` with activation count 1 (`FIRST`); loop control → `exclusiveGateway` + `orcon:loopControl`; multi-instance → `multiInstanceLoopCharacteristics`; waits → `intermediateCatchEvent` (timer/message) or `userTask`; boundary events → `boundaryEvent attachedToRef cancelActivity`; edges → `sequenceFlow` with `conditionExpression language="urn:orcon:expr:1"`. Orcon-only data goes to `extensionElements` in namespace `urn:orcon:bpmn:1`, with the same UPPER_SNAKE values as the JSON document. Editor data → BPMNDI.

### 6. Draft, publication, snapshots
- One mutable draft per flow with optimistic concurrency (`draftRevision`).
- Publish = full validation (§9) → new immutable `flow_version` row with `version_number = max+1`, `contentHash = SHA-256(JCS(semantic document))` where the semantic document excludes `editor`. Published rows are never updated or deleted while referenced.
- Manual test run of a draft = immutable `flow_snapshot` keyed by `contentHash` (deduplicated; not numbered).
- Rollback = publishing the content of an older version as version N+1. Version history is linear.
- Automatic triggers start only the flow's active published version. Every execution pins its revision at creation — exactly one of `flow_version_id` or `flow_snapshot_id` — and uses it until termination (including retries in place).

### 7. Format evolution
- Breaking or semantic changes create a new major (`orcon/flow@2`). Optional additive fields whose absence preserves @1 behaviour may be added within @1.
- Readers upgrade older formats through a chain of pure, total, unit-tested functions `upgrade_N_to_N+1`. Stored documents are never rewritten; the upgraded form is cached in memory by `contentHash`.
- A document with an unknown newer format is rejected with an explicit error naming the format and the binary's supported range.

### 8. BPMN XML export and round-trip
`orcon flow export --bpmn <flow> [--version N]` produces BPMN 2.0 XML (semantic + BPMNDI). Lossless means: for every valid flow `f`, `canonical(importOrconBPMN(exportBPMN(f))) == canonical(f)` byte for byte. The importer in V1 only accepts BPMN documents produced by Orcon (carrying `urn:orcon:bpmn:1`); arbitrary third-party BPMN import is out of scope (later phase).

### 9. Validation levels
1. Structural (JSON Schema, including the closed UPPER_SNAKE value sets of §11). 2. Referential (ids, ports, attachedTo, names). 3. Control-flow semantics (ADR-0010 §11). 4. Data access and variable concurrency (ADR-0008 §4). 5. Safety: every effect node has a valid EOSC contract (ADR-0005/0012). 6. Call graph (ADR-0010 §7). Errors block publication; warnings are shown and stored with the version. n8n imports always land as drafts with an import report.

### 10. Deletion (ADR-0077 §4)
Deleting a flow is a soft delete with a fixed 30-day recovery window: in one transaction the flow gets `deleted_at` and `deleted_by_principal_id` and the audit event `flow.deleted` is recorded; ordinary queries no longer see it and its name becomes free in the project (the unique name constraint covers non-deleted flows only). Running executions continue on their pinned revision; no new execution of a deleted flow starts (its automatic triggers are switched off by ADR-0024 §8). Restoring within 30 days clears both columns and records `flow.restored`; a name conflict with an active flow in the project is a conflict error and nothing changes. After 30 days retention purges the flow physically (ADR-0017 §8). Deleting a project or an organization hard-deletes its flows, including those inside the recovery window.

### 11. Value casing
The API returns this document verbatim, so every enumerated value in it is UPPER_SNAKE (ADR-0080 §3): branch mode, priority, retention policy, routing, join mode and data mode, multi-instance and node concurrency `"ALL"`, error routing, call version and mode, wait kind, edge kind, attention notification class, and the values of the additive blocks defined by other ADRs (the trigger block of ADR-0020, Code node parameters of ADR-0036). Identifiers keep their form: ids, names, node types, port names, BPMN element names, the `format` tag and expression text. The expression context (`$execution`, `$workflow`, …) is n8n-compatible and is not part of this rule.

## Consequences
- The engine interprets one model for both the n8n-style editor and future BPMN views.
- Content hashing makes snapshots and published versions comparable and deduplicable.
- The schema is a public contract; changing it needs a format version and an upgrade function.
- A flow deleted by mistake can be recovered for 30 days.
- A value reads the same in the flow document, the API and the database.

## Alternatives considered
- **BPMN XML as the canonical format**: rejected — verbose, awkward for the editor, item/expression data would live entirely in extensions anyway.
- **n8n JSON as the canonical format**: rejected — no formal semantics for joins/loops/waits, couples Orcon to n8n's format evolution and branding.
- **Mutable versions / in-place edits of published versions**: rejected — breaks ADR-0003 version pinning.
- **Lowercase values in the document and UPPER_SNAKE only in other API fields**: rejected — the API returns the document verbatim, so two spellings would cross the same interface.

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-002.** Nodes of type `startEvent` gain an optional `trigger` block, additive within `orcon/flow@1` (§7), with UPPER_SNAKE values: `{ family, addresses[], response, dedupe, schedule?, poll?, listen?, registration? }` (ADR-0020 §2). Publication validation (§9) adds the trigger rules T-V1..T-V9 (ADR-0024 §2). Publishing and activating are distinct: publishing creates a version; activation is the separate state `flow.activation_state` (`ACTIVE` | `INACTIVE`, ADR-0024).
- **SPEC-003.** Nodes gain an additive `connector { version, digest }` field (within `@1`, §7); the parameters of `orcon.code@1` (ADR-0036 §1) are part of the document; validation level 5 is extended to check that the connector is installed, the trust policy, permissions, revocation, that host-access nodes are enabled (ADR-0037 §7), and that imported `UNSAFE` nodes were reviewed before activation.
- **SPEC-005.** Nodes gain the additive block `credentials: { <slot>: { name } }` (ADR-0057 §4); `draftRevision` is exposed as `ETag` (ADR-0053 §5); the deterministic readable serialization of ADR-0057 §3 is the file form and does not change `contentHash` (JCS).
- **SPEC-006.** Nodes of type `orcon.ai.*` gain an `ai` block (model, fallbacks, tools, memory, redaction, caps, approval) whose enumerated values are UPPER_SNAKE (ADR-0080 §3); expressions gain `$fromAI`; validation levels 3 and 5 add rules for model capabilities and fallbacks, tools pinned by hash and the evaluation gate (ADR-0069); BPMN export represents an agent as `adHocSubProcess` with `orcon:agent` (ADR-0060 §11).
- **SPEC-001 (Amendment 6).** §3: the node type `orcon.variables@1` belongs to the format, like `orcon.loop@1`; its parameters are defined by ADR-0008 §4.3. Validation level 4 (§9) and rule V9 (ADR-0010 §11) read the written variables from `parameters.assignments[].variable`.
- **SPEC-001 (Amendment 9, 2026-10-13, owner decision).** §3: the `join` block gains the optional member `remaining: "CANCEL" | "LET_FINISH"`, valid only with `mode: "FIRST"`; absent means `CANCEL`, the behaviour `FIRST` had before this amendment. The block reads `{ mode: "ALL" | "ACTIVATED" | "FIRST", data: "APPEND" | "MATCH_FIELD" | "MATCH_POSITION", matchField?, remaining? }`. Until an amendment of ADR-0010 §3 defines `MATCH_FIELD` and `MATCH_POSITION`, publication rejects them and `matchField` (rule V14, ADR-0010 §11). §5: both values export as `complexGateway` with activation count 1 and `orcon:remaining` in `extensionElements`, so the round trip of §8 stays lossless; `LET_FINISH` is the BPMN 2.0 complex gateway's own wait-for-reset behaviour, `CANCEL` adds the cancellation of ADR-0010 §3. §11: the values of `remaining` are UPPER_SNAKE.
- **SPEC-003 (Amendment 9, CN2).** §1: `orcon.*` is reserved for keys an Accepted ADR defines, in the place it defines them; any other `orcon.*` key is a level 1 error. The only such key today is `orcon.import` in `nodes[].ext` (ADR-0034 §8 and its amendment define its closed shape, checked at level 1). It is not valid in the document's `ext`, where import provenance stays in `settings.importedFrom` (§2).
