-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- Cost accounting of the model calls (SPEC-006 cut AI2, ADR-0062 sections 2 and 4). A rollup row adds the calls of
-- one scope, grain period and purpose; a budget limits the spending of one scope and period; an alert row
-- records that a threshold was crossed in a period, so it is emitted once.
--
-- A scope is an execution, a flow, a project or the organization. Each scope has real foreign keys in its own
-- nullable column (execution_id, flow_id, project_id; none for the organization), and a scope_key that identifies
-- the scope uniquely for the unique keys. The check on each table ties the columns together.

CREATE TABLE ai_usage_rollup (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  scope_key BLOB NOT NULL,
  scope_kind TEXT NOT NULL,
  execution_id BLOB,
  flow_id BLOB,
  project_id BLOB,
  grain TEXT NOT NULL,
  period_started_at INTEGER NOT NULL,
  purpose TEXT NOT NULL,
  input_token_count INTEGER NOT NULL,
  output_token_count INTEGER NOT NULL,
  cache_read_token_count INTEGER NOT NULL,
  cache_write_token_count INTEGER NOT NULL,
  reasoning_token_count INTEGER NOT NULL,
  cost_microdollars INTEGER NOT NULL,
  unpriced_call_count INTEGER NOT NULL,
  call_count INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_ai_usage_rollup PRIMARY KEY (id),
  CONSTRAINT uq_ai_usage_rollup_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_usage_rollup_scope_key_grain_period_started_at_purpose UNIQUE (scope_key, grain, period_started_at, purpose),
  CONSTRAINT ck_ai_usage_rollup_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_ai_usage_rollup_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_ai_usage_rollup_scope_key_length CHECK (length(scope_key) = 16),
  CONSTRAINT ck_ai_usage_rollup_execution_id_length CHECK (length(execution_id) = 16 OR execution_id IS NULL),
  CONSTRAINT ck_ai_usage_rollup_flow_id_length CHECK (length(flow_id) = 16 OR flow_id IS NULL),
  CONSTRAINT ck_ai_usage_rollup_project_id_length CHECK (length(project_id) = 16 OR project_id IS NULL),
  CONSTRAINT ck_ai_usage_rollup_scope CHECK (
    (scope_kind = 'EXECUTION' AND execution_id IS NOT NULL AND flow_id IS NULL AND project_id IS NULL)
    OR (scope_kind = 'FLOW' AND flow_id IS NOT NULL AND execution_id IS NULL AND project_id IS NULL)
    OR (scope_kind = 'PROJECT' AND project_id IS NOT NULL AND execution_id IS NULL AND flow_id IS NULL)
    OR (scope_kind = 'ORG' AND execution_id IS NULL AND flow_id IS NULL AND project_id IS NULL)),
  CONSTRAINT ck_ai_usage_rollup_scope_kind CHECK (scope_kind IN ('EXECUTION', 'FLOW', 'PROJECT', 'ORG')),
  CONSTRAINT ck_ai_usage_rollup_grain CHECK (grain IN ('DAY', 'MONTH')),
  CONSTRAINT ck_ai_usage_rollup_purpose CHECK (purpose IN ('AGENT', 'CHAIN', 'EMBEDDING', 'EVALUATION', 'MEMORY', 'RETRIEVAL')),
  CONSTRAINT ck_ai_usage_rollup_counts CHECK (input_token_count >= 0 AND output_token_count >= 0 AND cache_read_token_count >= 0
    AND cache_write_token_count >= 0 AND reasoning_token_count >= 0 AND cost_microdollars >= 0 AND unpriced_call_count >= 0
    AND call_count >= 0),
  CONSTRAINT fk_ai_usage_rollup_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_usage_rollup_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_usage_rollup_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_usage_rollup_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE
) STRICT;

-- Budgets: one budget per scope and period. The limit and the actions are the ADR-0062 closed sets.
CREATE TABLE ai_budget (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  scope_key BLOB NOT NULL,
  scope_kind TEXT NOT NULL,
  execution_id BLOB,
  flow_id BLOB,
  project_id BLOB,
  period TEXT NOT NULL,
  limit_microdollars INTEGER NOT NULL,
  alert_threshold_percent INTEGER NOT NULL,
  exceeded_action TEXT NOT NULL,
  unknown_price_action TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_ai_budget PRIMARY KEY (id),
  CONSTRAINT uq_ai_budget_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_budget_scope_key_period UNIQUE (scope_key, period),
  CONSTRAINT ck_ai_budget_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_ai_budget_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_ai_budget_scope_key_length CHECK (length(scope_key) = 16),
  CONSTRAINT ck_ai_budget_execution_id_length CHECK (length(execution_id) = 16 OR execution_id IS NULL),
  CONSTRAINT ck_ai_budget_flow_id_length CHECK (length(flow_id) = 16 OR flow_id IS NULL),
  CONSTRAINT ck_ai_budget_project_id_length CHECK (length(project_id) = 16 OR project_id IS NULL),
  CONSTRAINT ck_ai_budget_scope CHECK (
    (scope_kind = 'EXECUTION' AND execution_id IS NOT NULL AND flow_id IS NULL AND project_id IS NULL)
    OR (scope_kind = 'FLOW' AND flow_id IS NOT NULL AND execution_id IS NULL AND project_id IS NULL)
    OR (scope_kind = 'PROJECT' AND project_id IS NOT NULL AND execution_id IS NULL AND flow_id IS NULL)
    OR (scope_kind = 'ORG' AND execution_id IS NULL AND flow_id IS NULL AND project_id IS NULL)),
  CONSTRAINT ck_ai_budget_scope_kind CHECK (scope_kind IN ('EXECUTION', 'FLOW', 'PROJECT', 'ORG')),
  CONSTRAINT ck_ai_budget_period CHECK (period IN ('PER_EXECUTION', 'DAY', 'MONTH', 'TOTAL')),
  CONSTRAINT ck_ai_budget_limit CHECK (limit_microdollars > 0),
  CONSTRAINT ck_ai_budget_alert_threshold CHECK (alert_threshold_percent BETWEEN 1 AND 99),
  CONSTRAINT ck_ai_budget_exceeded_action CHECK (exceeded_action IN ('FAIL', 'SUSPEND')),
  CONSTRAINT ck_ai_budget_unknown_price_action CHECK (unknown_price_action IN ('ALLOW', 'BLOCK')),
  CONSTRAINT fk_ai_budget_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_budget_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_budget_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_budget_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE
) STRICT;

-- Alerts already emitted, one row per budget, period and kind (THRESHOLD is the budget's own percentage, EXCEEDED
-- the limit itself), so each is emitted once (SPEC-006 R-COST-06).
CREATE TABLE ai_budget_alert (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  ai_budget_id BLOB NOT NULL,
  period_started_at INTEGER NOT NULL,
  kind TEXT NOT NULL,
  threshold_percent INTEGER NOT NULL,
  spent_microdollars INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_ai_budget_alert PRIMARY KEY (id),
  CONSTRAINT uq_ai_budget_alert_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_budget_alert_ai_budget_id_period_started_at_kind UNIQUE (ai_budget_id, period_started_at, kind),
  CONSTRAINT ck_ai_budget_alert_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_ai_budget_alert_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_ai_budget_alert_ai_budget_id_length CHECK (length(ai_budget_id) = 16),
  CONSTRAINT ck_ai_budget_alert_kind CHECK (kind IN ('THRESHOLD', 'EXCEEDED')),
  CONSTRAINT ck_ai_budget_alert_threshold CHECK (threshold_percent BETWEEN 1 AND 100),
  CONSTRAINT fk_ai_budget_alert_ai_budget FOREIGN KEY (org_id, ai_budget_id) REFERENCES ai_budget (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_budget_alert_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

-- The organization-leading indexes of the composite foreign keys: deleting a parent row finds its children here.
CREATE INDEX ix_ai_usage_rollup_org_id_execution_id ON ai_usage_rollup (org_id, execution_id);
CREATE INDEX ix_ai_usage_rollup_org_id_flow_id ON ai_usage_rollup (org_id, flow_id);
CREATE INDEX ix_ai_usage_rollup_org_id_project_id ON ai_usage_rollup (org_id, project_id);
CREATE INDEX ix_ai_budget_org_id_execution_id ON ai_budget (org_id, execution_id);
CREATE INDEX ix_ai_budget_org_id_flow_id ON ai_budget (org_id, flow_id);
CREATE INDEX ix_ai_budget_org_id_project_id ON ai_budget (org_id, project_id);
CREATE INDEX ix_ai_budget_alert_org_id_ai_budget_id ON ai_budget_alert (org_id, ai_budget_id);
