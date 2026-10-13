<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0062 — AI cost accounting, price table and budgets

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: table names per ADR-0077; UPPER_SNAKE values per ADR-0080 §3) (amended 2026-10-13: stored enumerated values are UPPER_SNAKE, ADR-0015 §2)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
A2: token and estimated cost accounting per execution, flow, project and organization, configurable budgets (stop + alert), price table updatable without a release. ADR-0014 avoids hot rows with chunked permits.

## Decision

### 1. Usage and cost
Every model, embedding, rerank, judge, summary and query-rewrite call records `usage` and `cost_microdollars = Σ tokens × price` from the price table in force, with `price_table_version` and a cost status (`ESTIMATED` by default; `UNKNOWN` without a price; `POSSIBLY_BILLED` for lost in-flight attempts). Provider invoices are authoritative; Orcon reports estimates.

### 2. Rollups
`ai_usage_rollup(org_id, id, scope_kind: EXECUTION|FLOW|PROJECT|ORG, execution_id?, flow_id?, project_id?, period_started_at, granularity: HOUR|DAY|MONTH, purpose, provider, model, input_token_count, output_token_count, cached_token_count, reasoning_token_count, cost_microdollars, call_count)`, with a check that the column named by `scope_kind` is set (none for `ORG`, which `org_id` already identifies), updated with additive upserts in the same confirmation-point transaction as the call (ADR-0060 §3); no extra transaction. Purposes: `AGENT`, `CHAIN`, `EMBEDDING`, `EVALUATION`, `MEMORY`, `RETRIEVAL`.

### 3. Price table
Document `orcon/ai-prices@1`: `{version, issuedAt, entries[{provider, model, region?, unit: PER_MILLION_TOKENS, input, output, cacheRead, cacheWrite, reasoning?, embedding?, perRequest?, validFrom}]}`. A default copy is embedded in the binary. Updates without a release: catalog package kind `PRICE_TABLE` signed through the TUF catalog (ADR-0033) and refreshed with catalog metadata (default 6 h) or imported offline (`orcon ai prices import`). Overrides: instance (`instance:ai-prices`) and organization (`ai-price:manage`), audited. Resolution order: org override > instance override > newest valid signed table > embedded table.

### 4. Budgets
`ai_budget(org_id, id, scope_kind, execution_id?, flow_id?, project_id?, period: PER_EXECUTION|DAY|MONTH|TOTAL, limit_microdollars, alert_threshold_document, alert_threshold_document_format, exceeded_action: FAIL|SUSPEND, unknown_price_action: ALLOW|BLOCK, mode: CHUNKED|PRECISE)` (default alert threshold 0.8). All budgets covering a call must grant (strictest wins). Exhaustion: `FAIL` (default) → permanent `AiBudgetExceeded` routed by the node's error rules; `SUSPEND` (optional) → run SUSPENDED until the budget is raised (resumption re-checks; humans are notified through ADR-0073). In-flight calls complete.

### 5. Reservation without hot rows
Before each call the worker reserves `estimate = countedInput × inPrice + maxOutputTokens × outPrice` from locally held permits. Permits are leased from `ai_budget_bucket(org_id, id, ai_budget_id, period_started_at, spent_microdollars, leased_microdollars, updated_at)` in chunks (default min(0.10 USD, 1 % of the limit)) with short expiry, the same way as ADR-0014 limit buckets; unused permits return at release; permits of expired leases are counted as spent. Maximum overshoot = chunk × active workers, documented. `PRECISE` mode reserves per call in a transaction (small budgets).

### 6. Alerts
Crossing a threshold or the limit emits `ai.budget.threshold` / `ai.budget.exceeded` (public events, ADR-0055) once per period and threshold; UI notice; optional error flow.

### 7. Logical schema additions
`ai_usage_rollup`, `ai_budget`, `ai_budget_bucket`, `ai_price_override` (organization overrides), `instance_ai_price_override` (instance overrides; an instance table, because one table with an optional `org_id` is not allowed), `ai_price_table(id, version, source, document, document_format, verified_at)` (instance table).

## Consequences
Spend is visible at every level and enforced without contention; estimates are honest about their nature.

## Alternatives considered
Per-call transactional reservation always (rejected: hot row under concurrency); post-hoc accounting only (rejected: cannot stop spend); prices only in code (rejected by A2).

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-009 K19 (2026-10-13, owner decision).** §1, §2 and §4 (cost status, `ai_usage_rollup.scope_kind`, `granularity`, `ai_budget.period`, `exceeded_action`, `unknown_price_action`, `mode`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
