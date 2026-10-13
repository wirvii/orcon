<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Flow format `orcon/flow@1`

Status: **Accepted** | Source: ADR-0007 (flow format), ADR-0008 §4 (data access), ADR-0010 §11 (static rules), ADR-0073 §1a (attention)

The JSON document that defines a flow. ADR-0007 is normative; if this page and the ADR disagree, the ADR wins. The machine-readable form is the JSON Schema `internal/adapter/codec/flowjson/schema/orcon-flow-1.schema.json` (draft 2020-12). The schema checks shape; the rules that need the whole graph are checked by the validator in `internal/flow` and are listed below.

## Document

```json
{
  "format": "orcon/flow@1",
  "id": "01930000-0000-7000-8000-000000000001",
  "name": "Order fulfilment",
  "settings": {},
  "variables": [],
  "nodes": [],
  "edges": [],
  "editor": {}
}
```

Required: `format`, `id`, `name`, `settings`, `variables`, `nodes`, `edges`, `editor`. Optional: `description`, `ext`. Every object is closed (unknown members are errors) except `parameters`, `editor`, `ext` and a wait's `form`, whose content the format leaves open and the reader never inspects.

The reader is strict so that reading and writing a document gives back the same data, and therefore the same `contentHash`: an empty string, a `0` where a count or limit is meant, and an empty optional array are errors (leave the member out). The required arrays `variables`, `nodes` and `edges` may be empty. An explicit `false` is kept. The published schema states the same rules (`minLength`, `minimum`, `minItems`). Every error carries the JSON Pointer of the offending member.

`ext` keys are namespaced `vendor.key`; `orcon.*` is reserved for keys an Accepted ADR defines: today only `orcon.import` in a node's `ext` (ADR-0034 §8 and its amendment), whose closed shape is checked at level 1.

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
| `nodes[].join.remaining` | `CANCEL`, `LET_FINISH` |
| `nodes[].onError` | `STOP_EXECUTION`, `CONTINUE_WITH_ERROR`, `ERROR_OUTPUT` |
| `nodes[].concurrency`, `nodes[].multiInstance.concurrency` | a number ≥ 1, or `ALL` |
| `nodes[].call.version` | a number ≥ 1, or `CURRENT` |
| `nodes[].call.mode` | `WAIT`, `FIRE_AND_FORGET` |
| `nodes[].wait.kind` | `TIMER`, `SIGNAL`, `HUMAN` |
| `nodes[].attention.notify` | `DEFAULT`, `ON`, `OFF` |
| `edges[].kind` | `SEQUENCE`, `BACK` |

The fields of `effect` contracts (`READ_ONLY`, `IDEMPOTENCY_KEY`, `PROVIDER_KEY`, `RESEND_WITH_KEY`, ...) belong to node types, not to flow documents; see "Node types and safety contracts".

`nodes[].element` is a BPMN 2.0 name and keeps its camelCase form: `startEvent`, `endEvent`, `task`, `serviceTask`, `scriptTask`, `userTask`, `receiveTask`, `sendTask`, `callActivity`, `exclusiveGateway`, `inclusiveGateway`, `parallelGateway`, `eventBasedGateway`, `complexGateway`, `intermediateCatchEvent`, `intermediateThrowEvent`, `boundaryEvent`.

## Patterns and limits

| Value | Rule |
|---|---|
| flow `id`, `settings.errorFlow`, a literal `call.target`, `call.allowedTargets[]` | canonical lowercase UUID text, `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` (ADR-0015 sections 2 and 3) |
| node and edge ids | `[A-Za-z0-9_-]{1,64}` |
| node `name` | 1 to 128 Unicode code points, unique in the flow (used by `$('Name')`) |
| flow `name` | 1 to 256 Unicode code points |
| node `type` | `<namespace>.<name>@<major>`, for example `orcon.http@1` |
| port names | identifiers such as `main`, `true`, `false`, `error`, `timeout`, `done`, `loop`, `case:<key>` |
| variable names | `[A-Za-z_][A-Za-z0-9_]{0,63}` |
| durations | ISO 8601 with weeks, days, hours, minutes and seconds only (`PT30S`, `PT1H30M`, `P2D`; weeks are not mixed with other units); years and months are not allowed because their length varies. The schema marks them `format: duration` and enforces the pattern |
| timer `date` | RFC 3339 date-time in UTC written with `Z` (`2026-10-12T10:00:00Z`), enforced by pattern in the schema and by the reader |
| timer `cycle` | repeating interval, `R/PT10M` or `R5/PT10M` |
| integers | whole numbers from 1 to 2147483647 where a count or limit is meant (`maxIterations`, `batchSize`, `maxAttempts`, `concurrency`, `callDepthLimit`, `edges[].order`); `2.0` and `1e3` are the same integer as `2` and `1000` and are read as such |
| `settings.memoryLimit` | whole number of bytes from 1 to 9007199254740991 (2^53-1); the instance maximum is checked at publication with the instance configuration (cut C12), not by the schema |
| all numbers | every number in a document, including inside `parameters`, `ext`, `editor` and forms, must survive the canonical form exactly (RFC 7493, I-JSON): integers written without a fraction or exponent lie within ±(2^53-1), and decimals have no more digits than a double keeps. Larger values are written as strings. Otherwise two different numbers, for example 9007199254740992 and 9007199254740993, would share one `contentHash` |
| strings | well-formed Unicode: a `\uD800`-`\uDFFF` escape must be half of a valid pair; a lone surrogate is refused rather than replaced by U+FFFD (RFC 8785) |

## Node blocks

A node has `id`, `name`, `element`, `type` and `parameters` (an object whose string values starting with `=` are expressions). Optional behaviour blocks:

- `routing` for `orcon.if` and `orcon.switch` nodes.
- `join` `{ mode, data, matchField?, remaining? }`; `remaining` only with `mode: FIRST` (absent = `CANCEL`); `MATCH_FIELD`, `MATCH_POSITION` and `matchField` are rejected at publication until defined (V14).
- `loop` `{ maxIterations, carry? }` on `orcon.loop` nodes only.
- `multiInstance` `{ sequential, concurrency?, inputCollection?, batchSize, completionCondition? }`.
- `retry` `{ maxAttempts, initialInterval?, factor?, maxInterval?, respectRetryAfter? }`; `factor` is at least 1.
- `timeouts` `{ attempt?, node? }`, `onError`, `concurrency`.
- `compensation` `{ handler, precondition? }`; the handler has no compensation of its own.
- `call` `{ target, allowedTargets?, version, mode, perItem, intentionalRecursion? }` on `callActivity` nodes only. A `target` starting with `=` is an expression.
- `wait` `{ kind, timer?, timeout?, form? }` on `receiveTask`, `userTask` and `intermediateCatchEvent`. A `TIMER` wait needs a `timer` with a duration or a date.
- `attachedTo`, `cancelActivity` and `eventDefinition` on `boundaryEvent` nodes only; the event definition holds exactly one of `timer`, `error` or `compensation`.
- `attention` `{ notify }` on waits, user tasks and effect nodes.

A document that carries `effect` or `writes` is rejected as an unknown member (see "Open points").

### The variables node

`orcon.variables@1` (`element: task`) is part of the format, like `orcon.loop@1`, and the only node type that writes execution variables (ADR-0008 §4.3). Its parameters are `{ "assignments": [ { "variable": "total", "value": 1 } ] }`: at least one assignment; `variable` is a literal name declared in `variables[]` (an expression there is an error, and so is an undeclared name); `value` is a literal or an expression, and for an accumulator variable it is the delta. Per-item evaluation order and writing several items in one activation belong to the cut that implements variables. `loop.carry` is unchanged.

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
| V8 | 4 | (expressions in `orcon.variables` values are scanned like any other) a node reads only nodes that dominate it, or that a join upstream merged; reads of unknown, own, sibling or downstream nodes are errors; a read through an `ACTIVATED` or `FIRST` join without `.isExecuted` is a warning; `$execution.variables` is read only with a literal name declared in `variables[]` (`.total` or `["total"]`), any other use is an error | error / warning |
| V9 | 4 | a variable without `accumulator` has no writers that can run at the same time; writers are the variables named by `parameters.assignments[].variable` of `orcon.variables@1` nodes (`writersOf`); two such nodes that can run at the same time and name the same plain variable are an error, sequential writes are fine. A non-interrupting boundary event runs beside the continuation of its host, so writers on both paths are concurrent; an interrupting one replaces the continuation and is not | error |
| V10 | 5 | with a node-type catalog: every node type is known, and the contract of an effect type is valid (ADR-0029 §5); `attention.notify: OFF` is refused on a node whose type writes | error |
| V11 | 6 | a call activity has a call block and does not call its own flow without `intentionalRecursion`; dynamic targets without `allowedTargets` are warned about; across flows, a call cycle with an unmarked edge blocks publication (`CheckCallGraph`) | error / warning |
| V13 | 4 | the condition that ends a loop reads something the loop body writes: the output of a node inside the body, the data flowing through it (`$json`, `$input`, `$prevNode`, `$binary`, `$items`), the iteration counter (`$runIndex`) or an execution variable (`$execution.variables.<name>`) that a node inside the body writes or that the loop carries. The expressions examined are those of the body node that sends tokens out of the body and the conditions of its exit edges. A loop with no edge leaving its body is warned about too. An expression the analysis cannot classify (syntax error, unknown name, node chosen at run time) counts as a read, so the warning appears only when every name is known to come from outside the body. Stored with the version; never blocks | warning |
| V14 | 3 | join options: `remaining` only with `FIRST`; `LET_FINISH` needs a split that dominates the join; `MATCH_FIELD`/`MATCH_POSITION`/`matchField` rejected | error |

### Node types and safety contracts

A node type, not a flow, owns its safety contract (ADR-0007 §3, ADR-0029 §5), so a document cannot declare itself safer than its type. `internal/flow` consumes a port, `NodeTypes.Lookup(ctx, type)`, that returns the purity (ADR-0008 §5) and, for `EFFECT` types, the contract. `ValidateWithTypes` applies rule V10 through it: an unknown type is a level 5 error naming the type, and the contract rules of ADR-0029 §5 (`CheckContract`) are a pure function: `UNKNOWN` is held to the `WRITE` rules, `READ_ONLY` only with `READ`, `IDEMPOTENCY_KEY` needs a key mode, location and ttl, `RECONCILE` needs a verification operation of purity `READ`, `UNSAFE` needs `retrySafe` false and `onUncertain` `SUSPEND`, and `RESEND_WITH_KEY` needs `IDEMPOTENCY_KEY`. `Validate` without a catalog skips these rules. The connector registry implements the port in SPEC-003; tests use a fake catalog. The types the format itself defines (`orcon.loop`, `orcon.if`, `orcon.switch`, `orcon.variables`) are not looked up.

The expression scan behind V8 is lexical: it recognises `$('Name')` and `$node["Name"]` inside `{{ }}` segments of `=` strings and in edge and multi-instance conditions. Rule V13 uses the expression analyzer of `internal/expr` (`expr.Analyze`), which parses the code and reports free identifiers, node references and method names (ADR-0018 section 7).

## Content hash and versions

`contentHash` is SHA-256 over the JSON Canonicalization Scheme text (RFC 8785) of the document without its `editor` member. Publishing validates the draft and creates version N+1; a test run freezes an immutable snapshot keyed by `contentHash`; rolling back republishes old content as N+1. See ADR-0007 §6.

## Evolution

A breaking change creates `orcon/flow@2` and an `upgrade_1_to_2` function; readers upgrade in memory and never rewrite stored documents. A document with a newer format than the binary knows is rejected with an error naming the format and the supported range, for example `unknown flow format "orcon/flow@2": this build supports orcon/flow@1 through orcon/flow@1`.

## Open points

These wait for an owner decision or a later cut; the implementation takes the reading shown.

- **The per-node override of a safety contract** (ADR-0007 §3 "per-node overrides allowed by that contract") is defined by the SPEC-003 cut that implements the node-type registry, as an additive amendment within `@1`.
- **Additive blocks of later specs** (`trigger`, `connector`, `credentials`, `ai`) are not in the schema yet; each arrives with its own cut, which extends the schema.
- **The in-memory cache of upgraded documents** (ADR-0007 §7) belongs to cut C4, the first cut that loads pinned revisions on the hot path.
- **Tests of the schema** use a small in-repository checker with a closed list of supported keywords and a differential check against the Go reader; a full validator library is a separate owner decision.
