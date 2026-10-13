<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Flow format `orcon/flow@1`

Status: **Accepted** | Source: ADR-0007 (flow format), ADR-0008 §4 (data access), ADR-0010 §11 (static rules), ADR-0073 §1a (attention)

The JSON document that defines a flow. ADR-0007 is normative; if this page and the ADR disagree, the ADR wins. The machine-readable form is the JSON Schema `internal/adapter/codec/flowjson/schema/orcon-flow-1.schema.json` (draft 2020-12). The schema checks shape; the rules that need the whole graph are checked by the validator in `internal/flow` and are listed below.

## Document

```json
{
  "format": "orcon/flow@1",
  "id": "order-fulfilment",
  "name": "Order fulfilment",
  "settings": {},
  "variables": [],
  "nodes": [],
  "edges": [],
  "editor": {}
}
```

Required: `format`, `id`, `name`, `settings`, `variables`, `nodes`, `edges`, `editor`. Optional: `description`, `ext`. Every object is closed (unknown members are errors) except `parameters`, `editor`, `ext` and a wait's `form`, whose content the format leaves open. An empty string is never written: leave the member out.

`ext` keys are namespaced `vendor.key`; `orcon.*` is reserved.

## Enumerated values

Every enumerated value is UPPER_SNAKE (ADR-0007 §11), because the API returns the document as is. Identifiers keep their form: ids, names, node types, port names, BPMN element names, the `format` tag and expression text.

| Member | Values |
|---|---|
| `settings.branchMode` | `PARALLEL`, `SEQUENTIAL` |
| `settings.priority` | `HIGH`, `NORMAL`, `LOW` |
| `settings.retention` | `ALWAYS`, `FAILED_ONLY`, `NEVER`, `DEFAULT` |
| `variables[].scope` | `EXECUTION`, `ITERATION` |
| `variables[].accumulator` | `SUM`, `APPEND`, `MAX`, `MIN` |
| `nodes[].routing` | `EXCLUSIVE`, `INCLUSIVE` |
| `nodes[].join.mode` | `ALL`, `ACTIVATED`, `FIRST` |
| `nodes[].join.data` | `APPEND`, `MATCH_FIELD`, `MATCH_POSITION` |
| `nodes[].onError` | `STOP_EXECUTION`, `CONTINUE_WITH_ERROR`, `ERROR_OUTPUT` |
| `nodes[].concurrency`, `nodes[].multiInstance.concurrency` | a number ≥ 1, or `ALL` |
| `nodes[].call.version` | a number ≥ 1, or `CURRENT` |
| `nodes[].call.mode` | `WAIT`, `FIRE_AND_FORGET` |
| `nodes[].wait.kind` | `TIMER`, `SIGNAL`, `HUMAN` |
| `nodes[].attention.notify` | `DEFAULT`, `ON`, `OFF` |
| `nodes[].effect.effect` | `READ`, `WRITE`, `DELETE`, `UNKNOWN` |
| `nodes[].effect.idempotency.mode` | `PROVIDER_KEY`, `NATURAL_KEY`, `NONE` |
| `nodes[].effect.onUncertain` | `RESEND_WITH_KEY`, `RECONCILE`, `SUSPEND` |
| `edges[].kind` | `SEQUENCE`, `BACK` |

`nodes[].element` is a BPMN 2.0 name and keeps its camelCase form: `startEvent`, `endEvent`, `task`, `serviceTask`, `scriptTask`, `userTask`, `receiveTask`, `sendTask`, `callActivity`, `exclusiveGateway`, `inclusiveGateway`, `parallelGateway`, `eventBasedGateway`, `complexGateway`, `intermediateCatchEvent`, `intermediateThrowEvent`, `boundaryEvent`.

## Patterns and limits

| Value | Rule |
|---|---|
| flow, node and edge ids | `[A-Za-z0-9_-]{1,64}` |
| node `name` | 1 to 128 characters, unique in the flow (used by `$('Name')`) |
| flow `name` | 1 to 256 characters |
| node `type` | `<namespace>.<name>@<major>`, for example `orcon.http@1` |
| port names | identifiers such as `main`, `true`, `false`, `error`, `timeout`, `done`, `loop`, `case:<key>` |
| variable names | `[A-Za-z_][A-Za-z0-9_]{0,63}` |
| durations | ISO 8601 with weeks, days, hours, minutes and seconds only (`PT30S`, `PT1H30M`, `P2D`); years and months are not allowed |
| timer `date` | RFC 3339 date-time |
| timer `cycle` | repeating interval, `R/PT10M` or `R5/PT10M` |
| integers | at least 1 where a count or limit is meant (`maxIterations`, `batchSize`, `maxAttempts`, `concurrency`, `memoryLimit` in bytes); `edges[].order` may be 0 |

## Node blocks

A node has `id`, `name`, `element`, `type` and `parameters` (an object whose string values starting with `=` are expressions). Optional behaviour blocks:

- `routing` for `orcon.if` and `orcon.switch` nodes.
- `join` `{ mode, data, matchField? }`; `matchField` is required exactly when `data` is `MATCH_FIELD`.
- `loop` `{ maxIterations, carry? }` on `orcon.loop` nodes only.
- `multiInstance` `{ sequential, concurrency?, inputCollection?, batchSize, completionCondition? }`.
- `retry` `{ maxAttempts, initialInterval?, factor?, maxInterval?, respectRetryAfter? }`; `factor` is at least 1.
- `timeouts` `{ attempt?, node? }`, `onError`, `concurrency`.
- `compensation` `{ handler, precondition? }`; the handler has no compensation of its own.
- `call` `{ target, allowedTargets?, version, mode, perItem, intentionalRecursion? }` on `callActivity` nodes only. A `target` starting with `=` is an expression.
- `wait` `{ kind, timer?, timeout?, form? }` on `receiveTask`, `userTask` and `intermediateCatchEvent`. A `TIMER` wait needs a `timer` with a duration or a date.
- `attachedTo`, `cancelActivity` and `eventDefinition` on `boundaryEvent` nodes only; the event definition holds exactly one of `timer`, `error` or `compensation`.
- `effect` `{ effect, idempotency?, verification?, onUncertain?, retrySafe? }`, the external-operation safety contract (ADR-0012 §1).
- `attention` `{ notify }` on waits, user tasks and effect nodes.
- `writes` (a list of variable names): the variables the node writes. This is how rule V9 finds concurrent writers. See "Open points".

## Edges

`{ id, from: { node, port }, to: { node }, condition?, default?, kind, order? }`. `kind: "BACK"` is legal only when the target is an `orcon.loop` node.

## Static rules

Validation runs in levels; the first level that has errors is reported alone so one mistake does not hide others behind follow-up noise (structure, then references; the rest run together).

| Code | Level | Rule | Severity |
|---|---|---|---|
| `STRUCTURE`, `FORMAT`, `ENUM`, `VALUE`, `EXT`, `BLOCK` | 1 | shape, closed UPPER_SNAKE value sets, ranges, patterns, blocks on the wrong element | error |
| `REFERENCE`, `V12` | 2 | ids, edge endpoints, `attachedTo`, handlers, variable names exist; node names unique | error |
| `ELEMENT` | 3 | start events have no incoming edges, end events no outgoing edges, boundary events no incoming edges | error |
| V1 | 3 | every node is reachable from a start event | error |
| V2 | 3 | forward edges form no cycle; every back edge returns to an `orcon.loop` node | error |
| V3 | 3 | a loop control has `maxIterations` ≥ 1 | error |
| V4 | 3 | `SIGNAL` and `HUMAN` waits have a `timeout` and an outgoing edge on port `timeout` | error |
| V5 | 3 | an exclusive or inclusive gateway with several outgoing edges has exactly one default edge | warning when none, error when several |
| V6 | 3 | a join has at least two incoming edges | error |
| V7 | 3 | a boundary event is attached to an activity | error |
| V8 | 4 | a node reads only nodes that dominate it, or that a join upstream merged; reads of unknown, own, sibling or downstream nodes are errors; a read through an `ACTIVATED` or `FIRST` join without `.isExecuted` is a warning | error / warning |
| V9 | 4 | a variable without `accumulator` has no writers that can run at the same time | error |
| V10 | 5 | an effect node that writes says how an uncertain outcome is recovered, and the recovery is backed by the contract; `attention.notify: OFF` is refused on a node with a write effect | error |
| V11 | 6 | a call activity has a call block and does not call its own flow without `intentionalRecursion`; dynamic targets without `allowedTargets` are warned about; across flows, a call cycle with an unmarked edge blocks publication (`CheckCallGraph`) | error / warning |

The expression scan behind V8 is lexical: it recognises `$('Name')` and `$node["Name"]` inside `{{ }}` segments of `=` strings and in edge and multi-instance conditions. The full expression grammar belongs to the expression engine (ADR-0018).

## Content hash and versions

`contentHash` is SHA-256 over the JSON Canonicalization Scheme text (RFC 8785) of the document without its `editor` member. Publishing validates the draft and creates version N+1; a test run freezes an immutable snapshot keyed by `contentHash`; rolling back republishes old content as N+1. See ADR-0007 §6.

## Evolution

A breaking change creates `orcon/flow@2` and an `upgrade_1_to_2` function; readers upgrade in memory and never rewrite stored documents. A document with a newer format than the binary knows is rejected with an error naming the format and the supported range, for example `unknown flow format "orcon/flow@2": this build supports orcon/flow@1 through orcon/flow@1`.

## Open points

These need an owner decision or an ADR amendment before the format is final; the implementation takes the interim reading shown.

- **Variable writes.** ADR-0007 §3 and ADR-0008 §4.3 do not say how a node declares that it writes a variable, which rule V9 needs. Interim: the optional `writes` member above.
- **Node-level `effect` block.** ADR-0007 §3 says `effect` references the node type's contract with per-node overrides, but connector contracts (SPEC-003) do not exist yet. Interim: the block carries the ADR-0012 §1 runtime contract inline, and V10 checks it. Whether a node is an effect node by its type waits for the node-type registry.
- **Additive blocks of later specs** (`trigger`, `connector`, `credentials`, `ai`) are not in the schema yet; each arrives with its own cut, which extends the schema.
- **Limits.** The ranges for `memoryLimit` (bytes), the id and name lengths, and the allowed duration forms are interim choices, listed in the tables above.
