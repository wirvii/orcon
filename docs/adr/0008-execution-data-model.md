<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0008 — Execution data model: items, tokens and information layers

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12, 2026-10-13

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
Owner decisions D1 (n8n-like item lists with per-item lineage), D13 (five information layers), D18 (large data). The prior N2N engine merged a global mutable map between branches with last-write-wins semantics; this ADR forbids that by construction.

## Decision

### 1. Items and batches
- An **item** is `{ json: object, binary?: { [name]: BlobRef }, lineage: [{ port, index }] }`.
- Nodes receive one ordered item batch per input activation and emit one batch per output port.
- **Node execution modes**: `PER_ITEM` (engine iterates items with node concurrency, fills lineage 1:1 automatically) or `BATCH` (node receives the whole batch and must set lineage on each output item; missing lineage fails validation in tests and at runtime marks `lineage: UNKNOWN`, which makes `$('X').item` raise a clear error).

### 2. Tokens
A token carries: `id` (deterministic, §6), `executionId`, position (`nodeId`, inbound edge), `scopePath`, `forkLineage` (list of `{splitNodeExecutionId, branch}`), `payloadReference` (reference to an immutable node output batch — never a copy), and loop/multi-instance context. Token payloads are immutable.

### 3. Encoding
- Batches are stored as one document per `(nodeExecutionId, port)`: canonical JSON, numbers preserved as decimal text, zstd-compressed when ≥ 4 KiB.
- Documents > 256 KiB (after compression) are stored as blobs (ADR-0016) and referenced.

### 4. Information layers (D13)
1. **Token data** — the batch referenced by the token. Immutable.
2. **Node outputs** — stored per `(nodeExecution, port)`, per item and per iteration (scope path), immutable, with lineage. A node X may read the output of node Y iff:
   - Y dominates X in the flow graph within X's scope (every path from the start to X passes through Y), or
   - Y lies on a branch merged by a join upstream of X (after a join both branches are ancestors). If the join mode is `ACTIVATED` or `FIRST`, Y may not have executed: `$('Y').isExecuted` is `false` and item access returns empty and records a warning in history.
   - Reads of a node that may run concurrently with X (sibling branch) are rejected at publication.
   - Inside a loop or multi-instance scope, reads resolve to the same scope instance; reads of nodes outside the scope resolve to the enclosing scope instance.
3. **Execution variables** — declared in `variables[]` with `scope: "EXECUTION" | "ITERATION"` and optional `accumulator: "SUM" | "APPEND" | "MAX" | "MIN"` and `carry: bool` (iteration scope only: the final value of iteration i initialises iteration i+1). Sub-flow scope = the child execution's own variables (children never see the parent's). Every write appends a `variable_change` row `(sequence_number, execution_id, scope_path, name, node_execution_id, changed_at, old_value_reference, new_value_reference)`.
   - **Concurrent-writer rule**: two writer nodes W1, W2 *may be concurrent* if they are on different outgoing branches of the same split before the corresponding join, or W1 = W2 inside a parallel multi-instance body with concurrency > 1, or downstream of an uncontrolled merge with ≥ 2 possibly live inputs. Publication is blocked if a non-accumulator variable has concurrent writers.
   - Accumulator writes are deltas applied atomically in the writer's checkpoint transaction (`value = op(value, delta)`). `APPEND` yields commit order; the multiset is deterministic, the order across concurrent branches is not (documented).
   - Reading an accumulator inside a concurrent region is a warning; the value read is recorded as a nondeterministic value (ADR-0013 §7).
4. **Flow data** (persists across executions; key/value per flow): `get`, `set`, `increment(delta) → newValue`, `compareAndSet(expected, new) → bool`, `delete`. Each operation is an *internal effect*: executed in its own fenced transaction together with the token checkpoint and a result record (`flow_data_operation`) keyed by `operation_key = H(nodeExecutionId, itemIndex, operationIndex)`. Recomputation after a crash returns the recorded result and never reapplies the operation. (Exactly-once applies here only because the operation and its record share one local transaction; no claim is made for external systems.)
5. **Execution context** (read-only): execution id, mode, flow id/version, trigger metadata, iteration index, multi-instance index, call chain.

Credentials are outside all layers: encrypted under ADR-0042, resolved per dispatch under ADR-0043 (share check at every dispatch, host binding), injected only into the call context of the node they are assigned to, never serialized into payloads, events or history.

### 5. Node purity classes
Each node type declares one: `PURE` (output is a function of inputs, parameters and recorded nondeterministic values), `READ` (external read without side effects; output persisted at the next confirmation point; re-executed if lost), `EFFECT` (journaled per item, ADR-0012), `INTERNAL_EFFECT` (flow data, §4.4).

### 6. Deterministic identifiers
- Token id: root token `H(executionId, "root")`; fork child `H(parentTokenId, splitNodeId, scopePath, branchIndex)`; join continuation `H(executionId, joinNodeId, scopePath)`; multi-instance instance `H(parentTokenId, nodeId, scopePath, instanceIndex)`.
- Node execution id: `H(executionId, tokenId, nodeId, scopePath)`, stored as `node_execution.id` (the former composite node execution key; ADR-0077 §3).
- `H` = first 128 bits of SHA-256 over the length-prefixed fields, encoded as UUID.
Deterministic ids make recomputation idempotent (`INSERT … ON CONFLICT DO NOTHING`) and give EOSC correlation keys stable inputs. They are the only row identities not generated as UUIDv7; ADR-0077 accepts them because they are still application-generated 16-byte UUID values.

## Consequences
- No shared mutable data between branches; merges happen only at joins with an explicit data mode.
- Per-item lineage enables per-item retries and `$('X').item` compatibility.
- Publication-time analysis must compute dominators and parallel regions; the editor reuses the same analysis.

## Alternatives considered
- **Single mutable execution context map (N2N, n8n-like global data)**: rejected — last-write-wins between branches.
- **One database row per item for every node output**: rejected for throughput; per-item rows exist only in the external-operation journal.

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-004.** §4: credentials are encrypted under ADR-0042 and resolved per dispatch under ADR-0043 (share check at every dispatch, host binding); they are never serialized.
- **SPEC-006.** §5: a fifth purity class, `AGENTIC`: a stepped node with its own durable state and multiple waits (ADR-0060 §1).
- **SPEC-001 (Amendment 6).** §4.3: an execution variable is written only by a node of the built-in type `orcon.variables@1` (`element: task`). Its parameters are `{ assignments: [ { variable, value } ] }` with at least one assignment; `variable` is a literal name declared in `variables[]` (an expression there is a validation error), and `value` is a literal or an expression (ADR-0018). For an accumulator variable the value is the delta. The set of variables a node writes is therefore known before publication, and the concurrent-writer rule applies to every `orcon.variables@1` node that names the variable. Loop carry (`loop.carry`, ADR-0007 §3) is unchanged. Per-item evaluation order and the write of several items in one activation are specified by the cut that implements variables. No other node type writes execution variables in `orcon/flow@1`; any later mechanism is an additive amendment.
- **SPEC-001 (Amendment 10, 2026-10-13, owner decision).** §4.3: an expression reads an execution variable as `$execution.variables.<name>` (ADR-0018 §1), with `<name>` literal (`$execution.variables.total` or `$execution.variables["total"]`) and declared in `variables[]`; any other use of `$execution.variables` is a publication error (rule V8, ADR-0010 §11). The set of variables a node reads is therefore known before publication, like the set it writes. The value is read-only and constant during the activation: for `EXECUTION` scope, the variable's value when the activation started; for `ITERATION` scope, its value in the current loop instance; `undefined` while the variable has never been written. A child execution reads only its own variables. How the value is captured and recorded for recomputation, and the read of an accumulator in a concurrent region (warning and recorded value, above), are specified by the cut that implements variables.
- **SPEC-014 (ADR-0082, 2026-10-13, owner decision).** §4.4: flow data has two namespaces, `PRODUCTION` and `TEST`. Executions of mode `TEST`, and the child executions they start, read and write only `TEST`; executions of every other mode read and write only `PRODUCTION`; no operation crosses namespaces. A value is identified by flow, namespace and key. The rules of this section (own fenced transaction, `flow_data_operation` record, recomputation returns the recorded result) apply unchanged in both namespaces. The `TEST` namespace of a flow can be cleared from the editor; nothing a test does can change `PRODUCTION`. §5: in a `TEST` execution an `INTERNAL_EFFECT` operation runs directly only when its design gives it a test scope (flow data: the `TEST` namespace); otherwise it is stopped by the effect confirmation gate like an `EFFECT` operation (ADR-0082 §4–§5).
