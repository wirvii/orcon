<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0010 — Control-flow semantics: gateways, joins, loops, multi-instance, sub-flows, waits and events

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12, 2026-10-13

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
Owner decisions D3, D3b, D6, D7, D9, D10, D11. Formal semantics are BPMN 2.0 token semantics; every token carries an item batch. OR-joins are resolved locally by propagating "path not taken" marks, never by global reachability analysis.

## Decision

### 1. Scopes
Every token has a scope path. Entering a loop iteration appends `L:<loopNodeId>#<i>`; a multi-instance instance appends `MI:<nodeId>#<k>`. Joins, merge trackers and data reads match only within the same scope path.

### 2. Edge completion signals (dead path elimination)
- For each `(edge, scopePath)`, the engine delivers zero or more live tokens followed by exactly one **completion signal** meaning "no more tokens on this edge in this scope". An edge completed with zero live tokens is a *not-taken* edge.
- A node with one incoming edge: each live token causes one activation; when its input is complete and its activations have finished, all of its outgoing edges are completed (ports that emitted nothing complete with zero tokens).
- A node with several incoming edges and no `join` block (**uncontrolled merge**, BPMN default, n8n behaviour) activates once per live token. If a synchronizing join exists downstream in the same scope (determined statically), a `merge_tracker` row records input completion so the node can complete its outputs; otherwise no tracking is persisted.
- Completion signals are processed inline by the worker that produced them and persisted only when they reach a join or merge tracker (cheap common path).
- Back edges are excluded from completion accounting at the loop control node; only the entry edge counts.

### 3. Joins
One `join_instance` row per `(execution, node, scope path)` (unique per organization), with per-edge `liveCount` and `completed` flags in its edge-state document. Each arriving token or completion signal is applied in one transaction that upserts the row, records the arrival (`join_arrival`), and returns the new state ("mark me and tell me whether I am the last"). The transaction that observes the firing condition creates the continuation token (deterministic id, unique constraint) in the same transaction.
- `ALL` (AND-join): fires when every incoming edge completed with ≥ 1 live token. If any edge completes with zero live tokens or any feeding branch failed, the join fails with `JoinRequirementNotMet` (error routing of the join node applies).
- `ACTIVATED` (default; OR-join): fires when all incoming edges completed and ≥ 1 carried a live token; if all completed with zero live tokens, the join completes its outputs with zero tokens.
- `FIRST` (discriminator): fires on the first live arrival. In the same transaction it cancels every token whose fork lineage contains the corresponding split instance and which has not reached the join; external operations already DISPATCHING in those branches finish their ADR-0012 lifecycle and are reported. Later arrivals are consumed and recorded.
- Data modes: `APPEND` (batches concatenated in incoming-edge order, then arrival order within an edge), `MATCH_FIELD`, `MATCH_POSITION`.
- Multiple live tokens on one edge (after an uncontrolled merge) are all included in the join's combined data.

### 4. Splits and item routing
- `parallelGateway` split: one live token per outgoing edge.
- `exclusiveGateway` split (token-level, conditions on the batch): first true condition in declared order; else `default` edge; else error `NoPathTaken`. Non-chosen edges complete with zero tokens.
- `inclusiveGateway` split: every true condition; none → default or error.
- **Item-routing nodes** (`orcon.if`, `orcon.switch`): conditions are evaluated per item. `routing: EXCLUSIVE` sends each item to exactly one port (XOR per item); `routing: INCLUSIVE` may send an item to several ports (OR per item). Each non-empty port emits one live token carrying its item subset; empty ports complete with zero tokens. With a single item this is exactly BPMN XOR/OR.

### 5. Branch order
`PARALLEL` (default for new flows): sibling tokens are runnable immediately. `SEQUENTIAL` (default for n8n imports): siblings are created `DEFERRED` in edge `order`; sibling k+1 becomes RUNNABLE when every token descending from sibling k is CONSUMED, COMPLETED, WAITING or terminal. The importer computes `order` from n8n's documented ordering rule and stores it explicitly; the engine never reads canvas positions.

### 6. Loops (D6)
- Backward edges are allowed only into an `orcon.loop` node. Publication inserts nothing silently: an n8n cycle without loop control is converted by the importer with a review warning; a native cycle without loop control is a validation error.
- Loop control state: `iteration` (in the scope path and in every checkpoint), `maxIterations` (mandatory; exceeding it raises `LoopLimitExceeded`), carried variables (ADR-0008 §4.3).
- Entry token → iteration 0; back-edge token → iteration i+1; the exit port is taken by the loop body's routing (e.g. an IF node feeding either the back edge or an edge leaving the loop).
- Iterations whose effects were journaled are never re-executed after a crash; pure work since the last confirmation point is recomputed deterministically.
- Warning (not blocking): exit condition references nothing written inside the body.

### 7. Sub-flows (call activity) and call cycles (D10, D11)
- A call creates a child execution in the same transaction that moves the parent token to WAITING (`mode: WAIT`) or continues the parent with output `{ executionId }` (`FIRE_AND_FORGET`). Child records `parentExecutionId`, `parentTokenId`, `parentNodeExecutionId`, `rootExecutionId`, `callDepth`, `callChain` (list of flow ids, stored as a whole document).
- Version: active published version at call time (`CURRENT`), recorded on the child; or a pinned version.
- `perItem: true` = multi-instance over the call with the node's concurrency.
- The child receives only the data passed explicitly (no parent variables, no credentials).
- The child's terminal transaction wakes the parent wait atomically with the child's result reference. Child FAILED → parent node fails with `ChildExecutionFailed` and the node's error rules apply. Canceling the parent cancels awaited children (recursively); fire-and-forget children are not canceled.
- Runtime guard before each call: `callDepth + 1 ≤ limit` (default 10, instance hard cap 100) and, if the target is already in `callChain`, the call edge must be marked intentional recursion with `maxDepth` not exceeded; else fail with `CallCycleDetected`.
- Publication guard: build the call graph over the active published version (and pinned versions) of every flow in the project plus the candidate, from the `call_edge` rows (real columns; no query reads inside flow documents); dynamic targets with `allowedTargets` contribute edges to each listed flow; without a list the version publishes with a visible "not analyzable" warning. Strongly connected components (Tarjan) identify direct and indirect cycles of any length, including the cycle closed by the version being published; any cycle containing an unmarked edge blocks publication, listing the full path.
- Calls from Orcon to Orcon webhooks carry an `Orcon-Origin` header: HMAC-SHA-256 (instance key) over `{rootExecutionId, depth, chain}`; a webhook trigger that verifies it inherits `depth + 1` and rejects beyond the limit (HTTP 508).
- Trigger circuit breaker: per flow, a sliding-window trigger rate; when the rate exceeds both a configurable absolute threshold and a multiple of its trailing baseline, the flow's triggers are paused and an alert/event is emitted; resumption is manual. Defaults are set by measurement in the implementing slice.

### 8. Waits and events (D9)
- All waits are durable from the first second: the token moves to WAITING, the lease is released, and a `timer` and/or `wait` row is written in the same transaction. Waiting tokens hold no memory or worker slot.
- Timer (`kind: TIMER`): duration or absolute date, due time computed by the database clock. The scheduler fires due timers with `FOR UPDATE SKIP LOCKED` in short transactions, through an engine-plane function because due timers span organizations (ADR-0047).
- Signal (external notice, `kind: SIGNAL`): unique URL per wait `…/signal/<waitId>.<secret>`; 256-bit random secret, stored as SHA-256 hash (`secret_hash`), compared in constant time. The request arrives before its organization is known, so the secret is resolved through the engine-plane function `orcon_engine.resolve_wait_secret`, which returns the wait and its `org_id` (ADR-0077 §6); the rest of the request runs in that organization's context. The first valid delivery resolves the wait with its payload; a repeated delivery with the same payload hash returns success idempotently; a different payload returns 409.
- Human (`kind: HUMAN`): task entry (assignee/role, form schema) resolved through the API with an authorization hook (full IAM: SPEC-004, permission `human-task:resolve`). Opening a human task creates an attention request in the same transaction when its notification class is active (ADR-0073 §1a: operator tasks and AI tool approvals by default; waits continued by an end user filling a form only if the node enables it); the task's assignee/role are always among its recipients.
- Signal and human waits must declare a `timeout` with an outgoing `timeout` edge; missing → publication error.
- With several items, each item waits independently via multi-instance.
- Boundary events on any activity: interrupting timer — in one transaction the activity's node execution is canceled (in-flight reads aborted; dispatched writes finish their journal lifecycle) and a token follows the boundary edge; the race "activity completes vs timer fires" is decided by a conditional update on the node execution (first commit wins). Non-interrupting timer — spawns a token on the boundary edge (optionally repeating with a cycle), the activity continues. Error boundary — equivalent to `onError: ERROR_OUTPUT` filtered by error codes.
- Event-based gateway: N waits in one group (`wait_group_key`); first resolution wins; siblings canceled in the same transaction.

### 9. Multi-instance (D3b)
- Instances receive slices of `inputCollection` (default: the item batch) of `batchSize` items; scope `MI:<node>#k`.
- `sequential: true` → one instance at a time; else up to `concurrency` (`"ALL"` = no node-level cap; other limits in ADR-0014 still apply).
- A `multi_instance` row counts completions; the last completing instance creates the continuation (same atomic pattern as joins). Output = concatenation in instance order.
- `completionCondition` true → remaining instances canceled.
- "Item 2 uses the response of item 1" is a loop with carried state, not multi-instance.

### 10. Termination
Implicit termination when no live tokens remain. A terminate end event cancels all other tokens of the execution (in-flight effects finish their journal lifecycle).

### 11. Static validation rules (publication)
V1 every node reachable from a start event; V2 every cycle contains a loop control and every back edge targets one; V3 `maxIterations` present; V4 signal/human waits have timeout path; V5 XOR/OR splits have default or are flagged "may raise NoPathTaken" (warning); V6 joins have ≥ 2 incoming edges; V7 boundary events attached to activities only; V8 data-access rule (ADR-0008 §4.2); V9 concurrent variable writers (ADR-0008 §4.3); V10 effect nodes have a valid EOSC contract; V11 call graph (§7); V12 names unique; V13 loop-exit heuristic (warning).

### 12. Compensation
Defined in ADR-0011 §7.

## Consequences
- OR-join behaviour is local and predictable; the editor can show which edges a join is still waiting for.
- Item routing keeps n8n behaviour while preserving BPMN semantics for single-item cases.
- Workflow Patterns support is declared per pattern (ADR-0019).

## Alternatives considered
- Global reachability analysis for OR-joins (BPMN "non-local" semantics): rejected — expensive, hard to explain, not needed with completion signals.
- Token-level XOR for IF nodes: rejected — silently changes the behaviour of every imported n8n flow with multi-item batches.
- In-memory short waits (n8n < 65 s): rejected by D9.

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-004.** §7: sub-flows never cross organizations.
- **SPEC-001 (Amendment 9, 2026-10-13, owner decision).** §1: entering a loop appends `L:<loopNodeId>@<entryTokenId>#<i>`, where `entryTokenId` is the deterministic id of the live token that arrived on the loop's entry edge; back-edge tokens keep it and increment `i`. Every live token arriving on the entry edge in the same scope therefore runs its own loop instance, with its own iteration count, `maxIterations` budget, carried variables, joins and merge trackers (§6). The loop's outgoing edges complete in the enclosing scope when its entry edge has completed there and every instance has finished; exits from several instances reach the next node as several live tokens on the same edge (§2, §3).
- **SPEC-001 (Amendment 9, 2026-10-13, owner decision).** §3 `ALL`: when every incoming edge completed with zero live tokens, the join is on a dead path: it neither fires nor fails, has no NodeExecution (ADR-0009 §3) and completes its outgoing edges with zero tokens. `JoinRequirementNotMet` applies when at least one incoming edge carried a live token and another completed with zero, or a feeding branch failed.
- **SPEC-001 (Amendment 9, 2026-10-13, owner decision).** §3 `FIRST`: the join's `remaining` option (ADR-0007 §3) selects what happens to the other branches. `CANCEL` (default) is the behaviour described above. `LET_FINISH` fires on the first live arrival and cancels nothing: the other branches run to completion, their arrivals are consumed and recorded without a continuation, and failures in them follow their own nodes' error routing. The corresponding split is the nearest split node that dominates the join in its scope and reaches it through at least two of its outgoing edges, computed before execution; a `LET_FINISH` join without one is a publication error (V14). Each instance of the corresponding split opens a round of the join; rounds are numbered from 1 in commit order within `(execution, node, scope path)`, and an arrival belongs to the round of the split instance in its fork lineage. A round resets when no token descending from its split instance can still reach the join (every descendant arrived, or is CONSUMED without a successor towards the join, COMPLETED, FAILED or CANCELED; WAITING and SUSPENDED descendants keep it open), tracked by the same descendant accounting as `SEQUENTIAL` branch order (§5). A round whose first live arrival comes before the previous round has reset waits, and fires in the transaction that resets the previous round. The continuation carries only the first live arrival's batch; later arrivals' outputs stay in history (ADR-0008 §4). Continuation and NodeExecution ids derive from the split instance, not from the round number. The join's outgoing edges complete when every incoming edge has completed and every round has reset. `ALL`, `ACTIVATED` and `FIRST` with `CANCEL` keep a single round per `(execution, node, scope path)`. Workflow pattern 9 (structured discriminator) and 28 (blocking discriminator) are `FIRST` with `LET_FINISH`; pattern 29 (cancelling discriminator) is `FIRST` with `CANCEL` (ADR-0019 §1).
- **SPEC-001 (Amendment 9, 2026-10-13, owner decision).** §3 data modes: `MATCH_FIELD` and `MATCH_POSITION` have no semantics yet; publication rejects them (V14) until an amendment of this section defines them (unpaired items, conflicting fields, batches of different length) in the slice that imports the n8n Merge node.
- **SPEC-001 (Amendment 9, 2026-10-13, owner decision).** §11: V14 join options: `remaining` only with `mode: FIRST`; `LET_FINISH` requires a corresponding split (§3); `MATCH_FIELD`, `MATCH_POSITION` and `matchField` are rejected until defined. Errors are level 3 (ADR-0007 §9) with a JSON Pointer to the offending member.
- **SPEC-001 (Amendment 10, 2026-10-13, owner decision).** §11 V8 also covers execution-variable reads: `$execution.variables` used other than with a literal name, or with a literal name not declared in `variables[]`, is an error (ADR-0008 §4.3).
- **SPEC-001 (Amendment 10, 2026-10-13, owner decision).** §11 V13: the condition that ends a loop must read something its body writes: the output of a node inside the body, the data flowing through the body (`$json`, `$input`, `$binary`, `$prevNode`, `$items`), the iteration counter (`$runIndex`), or an execution variable (`$execution.variables.<name>`) that a node inside the body writes or that the loop carries. The expressions examined are those of the body node that sends tokens out of the body and the conditions of its exit edges; a name the analysis cannot classify counts as a read. A loop whose body has no edge leaving it is also a V13 warning: it can only end with `LoopLimitExceeded`. V13 is a warning stored with the version and never blocks publication.
- **SPEC-014 (ADR-0082, 2026-10-13, owner decision).** §8: a wait of kind `EFFECT_CONFIRMATION` exists only in executions of mode `TEST` and is never declared in a flow document. The engine opens it in the transaction that inserts the activation's `PREPARED` journal items (ADR-0012 §4 as amended), together with the effect preview, an expiry timer (default 15 minutes, configurable from 1 minute to 24 hours) and the event `effect.preview_opened`; the token is WAITING and holds no lease. Only a recorded decision resolves it: a `continue` from an interactive session, or an automatic confirmation that the engine records itself (ADR-0082 §4). A decision marked for the remaining activations of the node in the same run is applied to each of them when it reaches the gate. On expiry the items become `NOT_DISPATCHED` and the test execution is canceled (ADR-0011 §6) with `EffectConfirmationExpired`. It needs no `timeout` edge and never belongs to an event-based gateway group. §7: a child execution started by a `TEST` execution has mode `TEST` and inherits the effect mode and the gate of the test plan (ADR-0082 §2).
