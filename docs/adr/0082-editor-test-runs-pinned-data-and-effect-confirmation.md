<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR 0082: Editor test runs, pinned test data and effect confirmation

Status: Proposed

## Context

The visual editor must let people test a flow while building it in the way n8n users expect: run the whole flow, run up to a node, or run a single node, with input and output panels and data pinned per node. The owner decided (BL-016 Q1, Q4) to improve on n8n where it is weak: operations that change external systems must show what they will do and ask before running, or be skipped with pinned or mocked data; test executions must be flagged and kept apart from production; and a past execution's data must be loadable into the editor to reproduce a problem without calling external services again.

The engine design (ADR-0007 to ADR-0018, ADR-0024, ADR-0073) has draft snapshots and `TEST`/`MANUAL` execution modes, but a test run always executes the whole snapshot and every external effect goes from `PREPARED` to `DISPATCHING` without a pause (ADR-0012 §4). Flow data persists across executions (ADR-0008 §4.4). The flow document is content-hashed and exported to Git (ADR-0007 §6, ADR-0057), so it cannot carry test data. Execution safety rules (at-least-once for external effects, no transaction across network calls, durable transitions atomic with enqueueing work) are non-negotiable.

## Decision

### 1. Test runs
- A test run is an execution with mode `TEST` created from a document supplied by the editor (saved or not). The server stores it as a `flow_snapshot` (ADR-0007 §6) and never changes the draft. When the supplied document differs from the stored draft, the caller needs `flow:update` in addition to `flow:execute`.
- Each test run has a test plan (`orcon/test-plan@1`, stored with the run): the target (`FLOW`, `NODE`, `UP_TO_NODE` with a node id), the scope (node ids allowed to execute), seeds, the effect mode (`CONFIRM` or `SUBSTITUTE`), node ids confirmed automatically for this run, and the trigger input (pinned data, a test session, a retained past event, or none).
- Test runs are validated at the ADR-0007 §9 levels for the nodes in scope.

### 2. Scope and seeding
- `FLOW`: every node reachable from the triggers. `UP_TO_NODE`: the target and its ancestors. `NODE`: the target only.
- Edges leaving the scope are closed with the dead-path signal of ADR-0010 §2, so joins and termination behave as they do in production.
- A node outside the scope that a node in the scope reads (direct input or `$('Name')`) is seeded: its pinned output if pinned, otherwise its output in the most recent test run of the same flow. Seeds are copied as immutable editor test data and recorded with the run; a test run never reads another execution's rows at run time. A missing seed fails the request with `test-input-missing` before any execution is created.
- Seeded items have unknown lineage unless the seed came from a test run of the same snapshot; `.item` access on them raises the existing clear error.
- Child executions started by a test run inherit mode `TEST`, the effect mode and the gate.

### 3. Pinned test data
- Pinned data is stored outside the flow document, per flow and node, one document per output port (`orcon/node-test-output@1`), as immutable rows referenced by a current pin. It is never part of a draft, version, snapshot, content hash, export or Git file.
- In a `TEST` execution, a pinned node in scope does not execute; its pinned output becomes its node output. Executions of any other mode never read pinned data.
- Pins can be created from an execution's stored outputs. Because history is masked (ADR-0017 §3), such pins contain masked values and are marked so. Reading pin content requires `execution:read-data`; creating a pin from an execution is audited as `execution.data_exported`.

### 4. Effect confirmation gate
- In a `TEST` execution, an activation of an operation with purity `EFFECT`, or an internal effect without a test namespace (§5), stops between the journal's `PREPARED` and `DISPATCHING` states. In the transaction that inserts the `PREPARED` items, the engine writes an effect preview (the rendered requests, before authentication is applied and with the masking set applied), a wait of kind `EFFECT_CONFIRMATION` for the token, its expiry timer and the event `effect.preview_opened`. No network send can happen before a later transaction records a decision.
- Decisions: `DISPATCH` (optionally only the first item), substitute with the pin, substitute with mock items, substitute with no output, or cancel the run. `DISPATCH` continues with the same `PREPARED` items and correlation keys, so every ADR-0012 recovery rule applies unchanged. Substitution marks the items `NOT_DISPATCHED`. A decision may apply to the remaining activations of the same node in the same run.
- Only a person in an interactive session can record `DISPATCH`; API keys and MCP clients cannot (their test runs stop at the gate or use `SUBSTITUTE`).
- Automatic confirmation requested by the editor for a node is honored only for operations whose safety level is `IDEMPOTENCY_KEY` or `RECONCILE` and whose effect is not `DELETE`; the preview is still recorded. Each automatic confirmation is a decision like any other: in the transaction that confirms the preview, the engine records the decision `AUTO_DISPATCH` and emits the audit event `execution.effect_decided` with that decision and the setting that authorized it (the test plan's automatically confirmed node ids and the test run); the actor is the principal who started the test run. `AUTO_DISPATCH` is recorded only by the engine; no caller can submit it as a decision.
- An open preview expires (default 15 minutes, configurable from 1 minute to 24 hours): the items become `NOT_DISPATCHED` and the test execution is canceled with `EffectConfirmationExpired`.
- Native connectors may implement an optional `Describe` to summarize prepared non-HTTP requests; connectors that cannot describe a request still require a decision, and the preview says that no description is available.

### 5. Internal effects in tests
- Flow data operations of a `TEST` execution use a separate namespace (`TEST`) that never touches production values; the editor can clear it.
- Designs that own other internal state define their own test scoping (for example, AI memory uses a session key derived from the test run). An internal effect without such scoping is gated like an external effect.

### 6. Separation from production
- Test runs are listed as test runs; execution lists filter them by mode. Test executions do not start error flows.
- Retention (owner decision, 2026-10-12): `TEST` executions form their own retention class — execution data is kept 7 days and the execution log 30 days, independent of the flow's policy (an `ALWAYS` policy does not protect test data), and test data is pruned before other classes under the size cap. Editor test data that no pin and no test run references is pruned 24 hours after creation.
- Attention (owner decision, 2026-10-12): when a `TEST` execution needs a person, the attention request of ADR-0073 is created as usual and cannot be turned off, but it is routed only to the principal who started the test run, in the app; it does not escalate and is not delivered by email, outbound webhook or chat channels.

### 7. Isolation and audit
- Every new table is organization-scoped with row-level security and enters the isolation battery. Pins, previews and test plans never contain credential values. Decisions are recorded with the deciding principal and time and audited as `execution.effect_decided`; automatic confirmations are audited the same way with decision `AUTO_DISPATCH`, the authorizing setting and the test-run starter as actor, so every effect decision in a test run is auditable; pin changes are audited as `flow.pin_changed`.

## Consequences
- n8n users get the familiar run, run-to-here, run-this-node and pinned data, and a test can no longer send an email, charge a card or delete records without a person seeing it first.
- The editor can reproduce a production problem from stored data without calling external services again.
- The engine gains one wait kind, two event types, two error codes, a flow-data namespace, a test plan and a retention class; production executions behave exactly as before.
- Test data is kept out of versions and Git by construction, and test noise never reaches on-call people.
- More prompts during testing; mitigated by pins, substitution mode, loop-wide decisions and automatic confirmation for safe-to-retry writes.

## Alternatives considered
- **No gate, as in n8n**: rejected by the owner (BL-016 Q1); manual test runs silently cause real side effects.
- **Preview computed only in the browser before the run**: rejected; the request depends on upstream outputs that exist only during the run, and the browser must not evaluate expressions.
- **A global "dry run" with fake responses for every connector**: rejected; it would need fake behavior per connector and would test something different from production.
- **Pinned data inside the flow document**: rejected; it changes content hashes, leaks data into published versions and Git, and conflicts with ADR-0057 §8.
- **Letting API keys confirm effects**: rejected; automation could bypass the human check the owner asked for, as ADR-0059 already rejects for production actions.
- **Running the editor's test against the stored draft only**: rejected; it forces saves before each test and blocks testing in read-only Git-managed projects.
- **Production routing and retention for test runs**: rejected by the owner; tests would page on-call people and keep production-sized data.
