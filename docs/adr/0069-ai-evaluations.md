<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0069 — AI evaluations: datasets, scorers, comparison, publish gate

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: table names per ADR-0077; UPPER_SNAKE values per ADR-0080 §3) (amended 2026-10-13: stored enumerated values are UPPER_SNAKE, ADR-0015 §2)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
A6: n8n parity minimum plus datasets (question + expected answer/criterion), scorers (exact match, semantic similarity, LLM-as-judge with rubric, custom code), side-by-side version comparison per case (draft vs published), optional publish gate (SPEC-002 draft/publish), one-click real conversation → test case with sensitive data redacted, evaluation spend counted in budgets. SPEC-002 defined an evaluation trigger and execution mode `EVALUATION`.

## Decision
1. **Datasets**: `evaluation_dataset(org_id, id, project_id, name, version_number)`, `evaluation_case(org_id, id, evaluation_dataset_id, input_document, expected_document?, criterion_document?, tool_mock_document?, source: MANUAL|IMPORT|CONVERSATION)` (each document with its `_format` column) plus `evaluation_case_tag(org_id, id, evaluation_case_id, tag)`; import from CSV/JSON, data tables, Google Sheets.
2. **Runs**: `evaluation_run(org_id, id, flow_id, flow_version_id?, flow_snapshot_id?, evaluation_dataset_version_number, status, concurrency, aggregate_score_document, cost_microdollars, started_at, finished_at)` with exactly one of `flow_version_id`, `flow_snapshot_id`; one execution in mode `EVALUATION` per case through `orcon.evaluation@1`; long-running operation (ADR-0053 §7).
3. **Scorers**: `EXACT`, `CONTAINS`, `REGEX`, `JSON_MATCH`, `STRING_SIMILARITY` (normalized Levenshtein), `SEMANTIC_SIMILARITY` (embedding cosine ≥ threshold), `LLM_JUDGE` (rubric, scale, threshold, judge model; presets `CORRECTNESS` and `HELPFULNESS`), `CATEGORIZATION`, `TOOLS_USED`, `CODE` (sandboxed JS/Python, ADR-0036), and flow-recorded metrics via `orcon.evaluation.record@1` (set outputs, set metrics, check if evaluating). Weighted aggregate. `evaluation_result(org_id, id, evaluation_run_id, evaluation_case_id, execution_id, output_reference, score_document, cost_microdollars, latency_milliseconds, step_count, tool_used_document)`.
4. **Comparison**: per-case side-by-side of two revisions: outputs, scores and deltas, cost, latency, steps, tools.
5. **Publish gate**: flow setting `evaluationGate {datasetId, minScore, maxCostIncrease?}`; the ADR-0024 publication transaction requires a completed run on the candidate `contentHash` meeting the gate; override requires `evaluation:manage` + `flow:publish`, a reason, and is audited.
6. **Conversation → case**: creates a case from a conversation or execution (input and preceding window; actual answer as editable expected); sensitive values replaced by permanent placeholders (no vault link) and the PII detector forced on; requires `conversation:read-data` + `evaluation:manage`; audited.
7. **Effects**: in `EVALUATION` mode, effect tools are not dispatched by default; they return the case's tool mocks or a synthetic `NOT_EXECUTED_IN_EVALUATION` result recorded in the trace; `allowEffects: true` per dataset is explicit and audited.
8. **Spend**: evaluated agents, judges and embeddings count in budgets with purpose `EVALUATION`; optional per-run budget.

## Consequences
Quality regressions are visible before publishing; evaluations cannot silently cause real side effects.

## Alternatives considered
Evaluations only in an external tool (rejected: n8n parity); real side effects by default (rejected: T6).

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-009 K19 (2026-10-13, owner decision).** §1 (`evaluation_case.source`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
