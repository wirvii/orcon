-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- Cost accounting of the model calls (SPEC-006 cut AI2, ADR-0062 sections 2 and 4). The SQLite counterpart of this
-- migration has the same tables and columns; see that file for the meaning of each one.

CREATE TABLE ai_usage_rollup (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  scope_key uuid NOT NULL,
  scope_kind text NOT NULL,
  execution_id uuid,
  flow_id uuid,
  project_id uuid,
  grain text NOT NULL,
  period_started_at timestamptz NOT NULL,
  purpose text NOT NULL,
  input_token_count bigint NOT NULL,
  output_token_count bigint NOT NULL,
  cache_read_token_count bigint NOT NULL,
  cache_write_token_count bigint NOT NULL,
  reasoning_token_count bigint NOT NULL,
  cost_microdollars bigint NOT NULL,
  unpriced_call_count bigint NOT NULL,
  call_count bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_ai_usage_rollup PRIMARY KEY (id),
  CONSTRAINT uq_ai_usage_rollup_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_usage_rollup_scope_key_grain_period_started_at_purpose UNIQUE (scope_key, grain, period_started_at, purpose),
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
);

CREATE TABLE ai_budget (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  scope_key uuid NOT NULL,
  scope_kind text NOT NULL,
  execution_id uuid,
  flow_id uuid,
  project_id uuid,
  period text NOT NULL,
  limit_microdollars bigint NOT NULL,
  alert_threshold_percent integer NOT NULL,
  exceeded_action text NOT NULL,
  unknown_price_action text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_ai_budget PRIMARY KEY (id),
  CONSTRAINT uq_ai_budget_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_budget_scope_key_period UNIQUE (scope_key, period),
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
);

CREATE TABLE ai_budget_alert (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  ai_budget_id uuid NOT NULL,
  period_started_at timestamptz NOT NULL,
  kind text NOT NULL,
  threshold_percent integer NOT NULL,
  spent_microdollars bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_ai_budget_alert PRIMARY KEY (id),
  CONSTRAINT uq_ai_budget_alert_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_ai_budget_alert_ai_budget_id_period_started_at_kind UNIQUE (ai_budget_id, period_started_at, kind),
  CONSTRAINT ck_ai_budget_alert_kind CHECK (kind IN ('THRESHOLD', 'EXCEEDED')),
  CONSTRAINT ck_ai_budget_alert_threshold CHECK (threshold_percent BETWEEN 1 AND 100),
  CONSTRAINT fk_ai_budget_alert_ai_budget FOREIGN KEY (org_id, ai_budget_id) REFERENCES ai_budget (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_ai_budget_alert_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

CREATE INDEX ix_ai_usage_rollup_org_id_execution_id ON ai_usage_rollup (org_id, execution_id);
CREATE INDEX ix_ai_usage_rollup_org_id_flow_id ON ai_usage_rollup (org_id, flow_id);
CREATE INDEX ix_ai_usage_rollup_org_id_project_id ON ai_usage_rollup (org_id, project_id);
CREATE INDEX ix_ai_budget_org_id_execution_id ON ai_budget (org_id, execution_id);
CREATE INDEX ix_ai_budget_org_id_flow_id ON ai_budget (org_id, flow_id);
CREATE INDEX ix_ai_budget_org_id_project_id ON ai_budget (org_id, project_id);
CREATE INDEX ix_ai_budget_alert_org_id_ai_budget_id ON ai_budget_alert (org_id, ai_budget_id);

-- Tenant isolation, as every organization table of this schema: rows are visible to their own organization only.
ALTER TABLE ai_usage_rollup ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_usage_rollup FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON ai_usage_rollup FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
ALTER TABLE ai_budget ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_budget FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON ai_budget FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
ALTER TABLE ai_budget_alert ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_budget_alert FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON ai_budget_alert FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));

GRANT SELECT, INSERT, UPDATE, DELETE ON ai_usage_rollup TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_usage_rollup TO orcon_engine;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_budget TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_budget TO orcon_engine;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_budget_alert TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON ai_budget_alert TO orcon_engine;
