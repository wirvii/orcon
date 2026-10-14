-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE instance_alert (
  id uuid NOT NULL,
  kind text NOT NULL,
  subject_reference text NOT NULL,
  status text NOT NULL,
  level bigint NOT NULL,
  summary text NOT NULL,
  affected_reference_document bytea,
  affected_reference_document_format text,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  acknowledged_by_principal_id uuid,
  acknowledged_at timestamptz,
  next_escalation_at timestamptz,
  closed_at timestamptz,
  closed_reason text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_instance_alert PRIMARY KEY (id),
  CONSTRAINT ck_instance_alert_kind CHECK (kind IN ('STORAGE_RETENTION_PRESSURE')),
  CONSTRAINT ck_instance_alert_subject_reference_length CHECK (char_length(subject_reference) BETWEEN 1 AND 500),
  CONSTRAINT ck_instance_alert_status CHECK (status IN ('OPEN', 'CLOSED')),
  CONSTRAINT ck_instance_alert_level_range CHECK (level >= 0),
  CONSTRAINT ck_instance_alert_summary_length CHECK (char_length(summary) BETWEEN 1 AND 2000),
  CONSTRAINT ck_instance_alert_affected_reference_document_pair CHECK ((affected_reference_document IS NULL) = (affected_reference_document_format IS NULL)),
  CONSTRAINT ck_instance_alert_affected_reference_document_format CHECK (affected_reference_document_format IN ('orcon/alert-reference@1')),
  CONSTRAINT ck_instance_alert_closed_reason_length CHECK (char_length(closed_reason) BETWEEN 1 AND 500),
  CONSTRAINT fk_instance_alert_acknowledged_by_principal FOREIGN KEY (acknowledged_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports fk_instance_alert_acknowledged_by_principal and the delete of its parent row.
CREATE INDEX ix_instance_alert_acknowledged_by_principal_id ON instance_alert (acknowledged_by_principal_id) WHERE acknowledged_by_principal_id IS NOT NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON instance_alert TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON instance_alert TO orcon_engine;

CREATE TABLE call_edge (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_version_id uuid NOT NULL,
  target_flow_id uuid NOT NULL,
  kind text NOT NULL,
  is_intentional boolean NOT NULL,
  maximum_depth bigint,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_call_edge PRIMARY KEY (id),
  CONSTRAINT uq_call_edge_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_call_edge_kind CHECK (kind IN ('STATIC', 'DYNAMIC')),
  CONSTRAINT ck_call_edge_maximum_depth_range CHECK (maximum_depth >= 1),
  CONSTRAINT fk_call_edge_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_call_edge_flow_version FOREIGN KEY (org_id, flow_version_id) REFERENCES flow_version (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_call_edge_target_flow FOREIGN KEY (org_id, target_flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
);

-- Supports fk_call_edge_flow_version and the delete of its parent row.
CREATE INDEX ix_call_edge_org_id_flow_version_id ON call_edge (org_id, flow_version_id);

-- Supports fk_call_edge_target_flow and the delete of its parent row.
CREATE INDEX ix_call_edge_org_id_target_flow_id ON call_edge (org_id, target_flow_id);

ALTER TABLE call_edge ENABLE ROW LEVEL SECURITY;
ALTER TABLE call_edge FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON call_edge FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT ON call_edge TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON call_edge TO orcon_engine;

CREATE TABLE execution (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  project_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  -- Exactly one of flow_version_id and flow_snapshot_id is set.
  flow_version_id uuid,
  flow_snapshot_id uuid,
  status text NOT NULL,
  mode text NOT NULL,
  priority bigint NOT NULL,
  parent_execution_id uuid,
  parent_token_id uuid,
  -- The execution itself for a root execution.
  root_execution_id uuid NOT NULL,
  call_depth bigint NOT NULL,
  call_chain_document bytea,
  call_chain_document_format text,
  -- Cleared before the retried execution is purged (RESTRICT).
  retried_execution_id uuid,
  run_generation bigint NOT NULL,
  cancel_requested_at timestamptz,
  undo_requested_at timestamptz,
  runnable_count bigint NOT NULL,
  leased_count bigint NOT NULL,
  waiting_count bigint NOT NULL,
  suspended_count bigint NOT NULL,
  active_milliseconds bigint NOT NULL,
  throttle_kind text,
  error_document bytea,
  error_document_format text,
  summary_document bytea,
  summary_document_format text,
  data_key_wrapped bytea,
  started_at timestamptz,
  finished_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_execution PRIMARY KEY (id),
  CONSTRAINT uq_execution_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_execution_status CHECK (status IN ('QUEUED', 'RUNNING', 'WAITING', 'SUSPENDED', 'CANCELING', 'COMPENSATING', 'SUCCEEDED', 'FAILED', 'CANCELED')),
  CONSTRAINT ck_execution_mode CHECK (mode IN ('MANUAL', 'TRIGGER', 'TEST', 'CHILD', 'RETRY')),
  CONSTRAINT ck_execution_priority_range CHECK (priority >= 0),
  CONSTRAINT ck_execution_call_depth_range CHECK (call_depth >= 0),
  CONSTRAINT ck_execution_call_chain_document_pair CHECK ((call_chain_document IS NULL) = (call_chain_document_format IS NULL)),
  CONSTRAINT ck_execution_call_chain_document_format CHECK (call_chain_document_format IN ('orcon/call-chain@1')),
  CONSTRAINT ck_execution_run_generation_range CHECK (run_generation >= 0),
  CONSTRAINT ck_execution_runnable_count_range CHECK (runnable_count >= 0),
  CONSTRAINT ck_execution_leased_count_range CHECK (leased_count >= 0),
  CONSTRAINT ck_execution_waiting_count_range CHECK (waiting_count >= 0),
  CONSTRAINT ck_execution_suspended_count_range CHECK (suspended_count >= 0),
  CONSTRAINT ck_execution_active_milliseconds_range CHECK (active_milliseconds >= 0),
  CONSTRAINT ck_execution_throttle_kind CHECK (throttle_kind IN ('NODE', 'EXECUTION', 'WORKER', 'EXTERNAL_SERVICE')),
  CONSTRAINT ck_execution_error_document_pair CHECK ((error_document IS NULL) = (error_document_format IS NULL)),
  CONSTRAINT ck_execution_error_document_format CHECK (error_document_format IN ('orcon/error@1')),
  CONSTRAINT ck_execution_summary_document_pair CHECK ((summary_document IS NULL) = (summary_document_format IS NULL)),
  CONSTRAINT ck_execution_summary_document_format CHECK (summary_document_format IN ('orcon/execution-summary@1')),
  CONSTRAINT ck_execution_revision CHECK ((flow_version_id IS NULL) <> (flow_snapshot_id IS NULL)),
  CONSTRAINT fk_execution_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_flow_version FOREIGN KEY (org_id, flow_version_id) REFERENCES flow_version (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_flow_snapshot FOREIGN KEY (org_id, flow_snapshot_id) REFERENCES flow_snapshot (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_parent_execution FOREIGN KEY (org_id, parent_execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_root_execution FOREIGN KEY (org_id, root_execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_retried_execution FOREIGN KEY (org_id, retried_execution_id) REFERENCES execution (org_id, id) ON DELETE RESTRICT
);

-- Supports ListExecutions (keyset by creation within a project) and the project foreign key.
CREATE INDEX ix_execution_org_id_project_id_created_at_id ON execution (org_id, project_id, created_at, id);

-- Supports ListFlowExecutions (keyset by creation within a flow) and the flow foreign key.
CREATE INDEX ix_execution_org_id_flow_id_created_at_id ON execution (org_id, flow_id, created_at, id);

-- Supports ListExecutionsByStatus.
CREATE INDEX ix_execution_org_id_status_created_at_id ON execution (org_id, status, created_at, id);

-- Supports fk_execution_flow_version and the delete of its parent row.
CREATE INDEX ix_execution_org_id_flow_version_id ON execution (org_id, flow_version_id) WHERE flow_version_id IS NOT NULL;

-- Supports fk_execution_flow_snapshot and the delete of its parent row.
CREATE INDEX ix_execution_org_id_flow_snapshot_id ON execution (org_id, flow_snapshot_id) WHERE flow_snapshot_id IS NOT NULL;

-- Supports fk_execution_parent_execution and the delete of its parent row.
CREATE INDEX ix_execution_org_id_parent_execution_id ON execution (org_id, parent_execution_id) WHERE parent_execution_id IS NOT NULL;

-- Supports fk_execution_parent_token and the delete of its parent row.
CREATE INDEX ix_execution_org_id_parent_token_id ON execution (org_id, parent_token_id) WHERE parent_token_id IS NOT NULL;

-- Supports fk_execution_root_execution and the delete of its parent row.
CREATE INDEX ix_execution_org_id_root_execution_id ON execution (org_id, root_execution_id);

-- Supports fk_execution_retried_execution and the delete of its parent row.
CREATE INDEX ix_execution_org_id_retried_execution_id ON execution (org_id, retried_execution_id) WHERE retried_execution_id IS NOT NULL;

ALTER TABLE execution ENABLE ROW LEVEL SECURITY;
ALTER TABLE execution FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON execution FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON execution TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON execution TO orcon_engine;

CREATE TABLE token (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  status text NOT NULL,
  node_id text NOT NULL,
  edge_id text,
  scope_path text NOT NULL,
  fork_lineage_document bytea NOT NULL,
  fork_lineage_document_format text NOT NULL,
  payload_reference text,
  fairness_key text NOT NULL,
  effective_priority bigint NOT NULL,
  available_at timestamptz NOT NULL,
  -- The worker holding the lease; no foreign key, a worker row is informational.
  lease_owner uuid,
  lease_epoch bigint NOT NULL,
  lease_expires_at timestamptz,
  claim_count bigint NOT NULL,
  abandoned_count bigint NOT NULL,
  deferred_until_at timestamptz,
  suspend_reason text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_token PRIMARY KEY (id),
  CONSTRAINT uq_token_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_token_status CHECK (status IN ('RUNNABLE', 'LEASED', 'WAITING', 'DEFERRED', 'SUSPENDED', 'CONSUMED', 'COMPLETED', 'CANCELED', 'FAILED')),
  CONSTRAINT ck_token_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_token_edge_id_length CHECK (char_length(edge_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_token_fork_lineage_document_format CHECK (fork_lineage_document_format IN ('orcon/fork-lineage@1')),
  CONSTRAINT ck_token_fairness_key_length CHECK (char_length(fairness_key) BETWEEN 1 AND 300),
  CONSTRAINT ck_token_effective_priority_range CHECK (effective_priority >= 0),
  CONSTRAINT ck_token_lease_epoch_range CHECK (lease_epoch >= 0),
  CONSTRAINT ck_token_claim_count_range CHECK (claim_count >= 0),
  CONSTRAINT ck_token_abandoned_count_range CHECK (abandoned_count >= 0),
  CONSTRAINT ck_token_suspend_reason CHECK (suspend_reason IN ('POISON', 'UNCERTAIN_OPERATION', 'COMPENSATION_REVIEW')),
  CONSTRAINT ck_token_lease CHECK ((status <> 'LEASED') OR (lease_owner IS NOT NULL AND lease_expires_at IS NOT NULL)),
  CONSTRAINT fk_token_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_token_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE
);

-- Supports ClaimTokens and orcon_engine.claim_tokens: claims run as orcon_engine, so the index does not lead with org_id.
CREATE INDEX ix_token_effective_priority_available_at_id ON token (effective_priority, available_at, id) WHERE status = 'RUNNABLE';

-- Supports ReleaseExpiredLeases (the reaper).
CREATE INDEX ix_token_lease_expires_at ON token (lease_expires_at) WHERE status = 'LEASED';

-- Supports RenewLeases and ReleaseWorkerLeases.
CREATE INDEX ix_token_lease_owner ON token (lease_owner) WHERE status = 'LEASED';

-- Supports fk_token_execution and the delete of its parent row.
CREATE INDEX ix_token_org_id_execution_id ON token (org_id, execution_id);

ALTER TABLE token ENABLE ROW LEVEL SECURITY;
ALTER TABLE token FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON token FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON token TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON token TO orcon_engine;

CREATE TABLE node_execution (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  token_id uuid NOT NULL,
  node_id text NOT NULL,
  scope_path text NOT NULL,
  status text NOT NULL,
  attempt_count bigint NOT NULL,
  item_state_document bytea,
  item_state_document_format text,
  item_route_document bytea,
  item_route_document_format text,
  output_reference_document bytea,
  output_reference_document_format text,
  error_document bytea,
  error_document_format text,
  started_at timestamptz,
  finished_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_node_execution PRIMARY KEY (id),
  CONSTRAINT uq_node_execution_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_node_execution_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_node_execution_status CHECK (status IN ('RUNNING', 'WAITING', 'SUCCEEDED', 'SUCCEEDED_WITH_ERRORS', 'FAILED', 'SUSPENDED', 'CANCELED')),
  CONSTRAINT ck_node_execution_attempt_count_range CHECK (attempt_count >= 0),
  CONSTRAINT ck_node_execution_item_state_document_pair CHECK ((item_state_document IS NULL) = (item_state_document_format IS NULL)),
  CONSTRAINT ck_node_execution_item_state_document_format CHECK (item_state_document_format IN ('orcon/item-state@1')),
  CONSTRAINT ck_node_execution_item_route_document_pair CHECK ((item_route_document IS NULL) = (item_route_document_format IS NULL)),
  CONSTRAINT ck_node_execution_item_route_document_format CHECK (item_route_document_format IN ('orcon/item-route@1')),
  CONSTRAINT ck_node_execution_output_reference_document_pair CHECK ((output_reference_document IS NULL) = (output_reference_document_format IS NULL)),
  CONSTRAINT ck_node_execution_output_reference_document_format CHECK (output_reference_document_format IN ('orcon/output-reference@1')),
  CONSTRAINT ck_node_execution_error_document_pair CHECK ((error_document IS NULL) = (error_document_format IS NULL)),
  CONSTRAINT ck_node_execution_error_document_format CHECK (error_document_format IN ('orcon/error@1')),
  CONSTRAINT fk_node_execution_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_node_execution_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_node_execution_token FOREIGN KEY (org_id, token_id) REFERENCES token (org_id, id) ON DELETE CASCADE
);

-- Supports ListNodeExecutions (keyset by identity within an execution) and the execution foreign key.
CREATE INDEX ix_node_execution_org_id_execution_id_id ON node_execution (org_id, execution_id, id);

-- Supports fk_node_execution_token and the delete of its parent row.
CREATE INDEX ix_node_execution_org_id_token_id ON node_execution (org_id, token_id);

ALTER TABLE node_execution ENABLE ROW LEVEL SECURITY;
ALTER TABLE node_execution FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON node_execution FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON node_execution TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON node_execution TO orcon_engine;

CREATE TABLE task_attempt (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  node_execution_id uuid NOT NULL,
  attempt_number bigint NOT NULL,
  lease_epoch bigint NOT NULL,
  -- NULL when the worker row was removed.
  worker_id uuid,
  status text NOT NULL,
  item_document bytea,
  item_document_format text,
  error_document bytea,
  error_document_format text,
  started_at timestamptz NOT NULL,
  finished_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_task_attempt PRIMARY KEY (id),
  CONSTRAINT uq_task_attempt_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_task_attempt_org_id_node_execution_id_attempt_number UNIQUE (org_id, node_execution_id, attempt_number),
  CONSTRAINT ck_task_attempt_attempt_number_range CHECK (attempt_number >= 1),
  CONSTRAINT ck_task_attempt_lease_epoch_range CHECK (lease_epoch >= 0),
  CONSTRAINT ck_task_attempt_status CHECK (status IN ('STARTED', 'SUCCEEDED', 'FAILED', 'TIMED_OUT', 'ABANDONED', 'CANCELED')),
  CONSTRAINT ck_task_attempt_item_document_pair CHECK ((item_document IS NULL) = (item_document_format IS NULL)),
  CONSTRAINT ck_task_attempt_item_document_format CHECK (item_document_format IN ('orcon/items@1')),
  CONSTRAINT ck_task_attempt_error_document_pair CHECK ((error_document IS NULL) = (error_document_format IS NULL)),
  CONSTRAINT ck_task_attempt_error_document_format CHECK (error_document_format IN ('orcon/error@1')),
  CONSTRAINT fk_task_attempt_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_task_attempt_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_task_attempt_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_task_attempt_worker FOREIGN KEY (worker_id) REFERENCES worker (id) ON DELETE SET NULL
);

-- Supports fk_task_attempt_execution and the delete of its parent row.
CREATE INDEX ix_task_attempt_org_id_execution_id ON task_attempt (org_id, execution_id);

-- Supports fk_task_attempt_worker and the delete of its parent row.
CREATE INDEX ix_task_attempt_worker_id ON task_attempt (worker_id) WHERE worker_id IS NOT NULL;

ALTER TABLE task_attempt ENABLE ROW LEVEL SECURITY;
ALTER TABLE task_attempt FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON task_attempt FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON task_attempt TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON task_attempt TO orcon_engine;

ALTER TABLE execution ADD CONSTRAINT fk_execution_parent_token FOREIGN KEY (org_id, parent_token_id) REFERENCES token (org_id, id) ON DELETE CASCADE;
