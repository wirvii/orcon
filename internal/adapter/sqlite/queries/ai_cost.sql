-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- Queries of the cost accounting (ai_usage_rollup, ai_budget, ai_budget_alert; SPEC-006 AI2). The PostgreSQL file of
-- the same name holds the same query names. A call adds to its rollups and writes its alerts in one transaction.

-- name: UpsertAIUsageRollup :exec
-- orcon:scope org
INSERT INTO ai_usage_rollup (id, org_id, scope_key, scope_kind, execution_id, flow_id, project_id, grain, period_started_at, purpose,
  input_token_count, output_token_count, cache_read_token_count, cache_write_token_count, reasoning_token_count,
  cost_microdollars, unpriced_call_count, call_count, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(scope_key), sqlc.arg(scope_kind), sqlc.arg(execution_id), sqlc.arg(flow_id),
  sqlc.arg(project_id), sqlc.arg(grain), sqlc.arg(period_started_at), sqlc.arg(purpose), sqlc.arg(input_token_count),
  sqlc.arg(output_token_count), sqlc.arg(cache_read_token_count), sqlc.arg(cache_write_token_count), sqlc.arg(reasoning_token_count),
  sqlc.arg(cost_microdollars), sqlc.arg(unpriced_call_count), sqlc.arg(call_count), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (org_id, scope_key, grain, period_started_at, purpose) DO UPDATE SET
  input_token_count = ai_usage_rollup.input_token_count + excluded.input_token_count,
  output_token_count = ai_usage_rollup.output_token_count + excluded.output_token_count,
  cache_read_token_count = ai_usage_rollup.cache_read_token_count + excluded.cache_read_token_count,
  cache_write_token_count = ai_usage_rollup.cache_write_token_count + excluded.cache_write_token_count,
  reasoning_token_count = ai_usage_rollup.reasoning_token_count + excluded.reasoning_token_count,
  cost_microdollars = ai_usage_rollup.cost_microdollars + excluded.cost_microdollars,
  unpriced_call_count = ai_usage_rollup.unpriced_call_count + excluded.unpriced_call_count,
  call_count = ai_usage_rollup.call_count + excluded.call_count,
  updated_at = excluded.updated_at;

-- name: GetAIUsageSpend :one
-- orcon:scope org
SELECT CAST(COALESCE(SUM(cost_microdollars), 0) AS INTEGER) AS spent_microdollars
FROM ai_usage_rollup
WHERE org_id = sqlc.arg(org_id) AND scope_key = sqlc.arg(scope_key)
  AND grain = sqlc.arg(grain) AND period_started_at = sqlc.arg(period_started_at);

-- name: GetAIUsageSpendAllPeriods :one
-- orcon:scope org
SELECT CAST(COALESCE(SUM(cost_microdollars), 0) AS INTEGER) AS spent_microdollars
FROM ai_usage_rollup
WHERE org_id = sqlc.arg(org_id) AND scope_key = sqlc.arg(scope_key) AND grain = 'MONTH';

-- name: UpsertAIBudget :exec
-- orcon:scope org
INSERT INTO ai_budget (id, org_id, scope_key, scope_kind, execution_id, flow_id, project_id, period, limit_microdollars,
  alert_threshold_percent, exceeded_action, unknown_price_action, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(scope_key), sqlc.arg(scope_kind), sqlc.arg(execution_id), sqlc.arg(flow_id),
  sqlc.arg(project_id), sqlc.arg(period), sqlc.arg(limit_microdollars), sqlc.arg(alert_threshold_percent),
  sqlc.arg(exceeded_action), sqlc.arg(unknown_price_action), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (org_id, scope_key, period) DO UPDATE SET
  limit_microdollars = excluded.limit_microdollars,
  alert_threshold_percent = excluded.alert_threshold_percent,
  exceeded_action = excluded.exceeded_action,
  unknown_price_action = excluded.unknown_price_action,
  updated_at = excluded.updated_at;

-- name: ListAIBudgetsForScope :many
-- orcon:scope org
SELECT id, period, limit_microdollars, alert_threshold_percent, exceeded_action, unknown_price_action
FROM ai_budget
WHERE org_id = sqlc.arg(org_id) AND scope_key = sqlc.arg(scope_key)
ORDER BY period, id
LIMIT sqlc.arg(max_rows);

-- name: InsertAIBudgetAlertIfAbsent :execrows
-- orcon:scope org
INSERT INTO ai_budget_alert (id, org_id, ai_budget_id, period_started_at, kind, threshold_percent, spent_microdollars, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(org_id), sqlc.arg(ai_budget_id), sqlc.arg(period_started_at), sqlc.arg(kind),
  sqlc.arg(threshold_percent), sqlc.arg(spent_microdollars), sqlc.arg(created_at), sqlc.arg(updated_at))
ON CONFLICT (ai_budget_id, period_started_at, kind) DO NOTHING;
