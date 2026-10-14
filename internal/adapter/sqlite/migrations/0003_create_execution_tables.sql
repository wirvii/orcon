-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE instance_alert (
  id BLOB NOT NULL,
  kind TEXT NOT NULL,
  subject_reference TEXT NOT NULL,
  status TEXT NOT NULL,
  level INTEGER NOT NULL,
  summary TEXT NOT NULL,
  affected_reference_document BLOB,
  affected_reference_document_format TEXT,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  acknowledged_by_principal_id BLOB,
  acknowledged_at INTEGER,
  next_escalation_at INTEGER,
  closed_at INTEGER,
  closed_reason TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_instance_alert PRIMARY KEY (id),
  CONSTRAINT ck_instance_alert_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_instance_alert_kind CHECK (kind IN ('STORAGE_RETENTION_PRESSURE')),
  CONSTRAINT ck_instance_alert_subject_reference_length CHECK (length(subject_reference) BETWEEN 1 AND 500),
  CONSTRAINT ck_instance_alert_status CHECK (status IN ('OPEN', 'CLOSED')),
  CONSTRAINT ck_instance_alert_level_range CHECK (level >= 0),
  CONSTRAINT ck_instance_alert_summary_length CHECK (length(summary) BETWEEN 1 AND 2000),
  CONSTRAINT ck_instance_alert_affected_reference_document_pair CHECK ((affected_reference_document IS NULL) = (affected_reference_document_format IS NULL)),
  CONSTRAINT ck_instance_alert_affected_reference_document_format CHECK (affected_reference_document_format IN ('orcon/alert-reference@1')),
  CONSTRAINT ck_instance_alert_acknowledged_by_principal_id_length CHECK (length(acknowledged_by_principal_id) = 16),
  CONSTRAINT ck_instance_alert_closed_reason_length CHECK (length(closed_reason) BETWEEN 1 AND 500),
  CONSTRAINT fk_instance_alert_acknowledged_by_principal FOREIGN KEY (acknowledged_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports fk_instance_alert_acknowledged_by_principal and the delete of its parent row.
CREATE INDEX ix_instance_alert_acknowledged_by_principal_id ON instance_alert (acknowledged_by_principal_id) WHERE acknowledged_by_principal_id IS NOT NULL;

CREATE TABLE call_edge (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_version_id BLOB NOT NULL,
  target_flow_id BLOB NOT NULL,
  kind TEXT NOT NULL,
  is_intentional INTEGER NOT NULL,
  maximum_depth INTEGER,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_call_edge PRIMARY KEY (id),
  CONSTRAINT uq_call_edge_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_call_edge_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_call_edge_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_call_edge_flow_version_id_length CHECK (length(flow_version_id) = 16),
  CONSTRAINT ck_call_edge_target_flow_id_length CHECK (length(target_flow_id) = 16),
  CONSTRAINT ck_call_edge_kind CHECK (kind IN ('STATIC', 'DYNAMIC')),
  CONSTRAINT ck_call_edge_is_intentional CHECK (is_intentional IN (0, 1)),
  CONSTRAINT ck_call_edge_maximum_depth_range CHECK (maximum_depth >= 1),
  CONSTRAINT fk_call_edge_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_call_edge_flow_version FOREIGN KEY (org_id, flow_version_id) REFERENCES flow_version (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_call_edge_target_flow FOREIGN KEY (org_id, target_flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports fk_call_edge_flow_version and the delete of its parent row.
CREATE INDEX ix_call_edge_org_id_flow_version_id ON call_edge (org_id, flow_version_id);

-- Supports fk_call_edge_target_flow and the delete of its parent row.
CREATE INDEX ix_call_edge_org_id_target_flow_id ON call_edge (org_id, target_flow_id);

CREATE TABLE execution (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  project_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  -- Exactly one of flow_version_id and flow_snapshot_id is set.
  flow_version_id BLOB,
  flow_snapshot_id BLOB,
  status TEXT NOT NULL,
  mode TEXT NOT NULL,
  priority INTEGER NOT NULL,
  parent_execution_id BLOB,
  parent_token_id BLOB,
  -- The execution itself for a root execution.
  root_execution_id BLOB NOT NULL,
  call_depth INTEGER NOT NULL,
  call_chain_document BLOB,
  call_chain_document_format TEXT,
  -- Cleared before the retried execution is purged (RESTRICT).
  retried_execution_id BLOB,
  run_generation INTEGER NOT NULL,
  cancel_requested_at INTEGER,
  undo_requested_at INTEGER,
  runnable_count INTEGER NOT NULL,
  leased_count INTEGER NOT NULL,
  waiting_count INTEGER NOT NULL,
  suspended_count INTEGER NOT NULL,
  active_milliseconds INTEGER NOT NULL,
  throttle_kind TEXT,
  error_document BLOB,
  error_document_format TEXT,
  summary_document BLOB,
  summary_document_format TEXT,
  data_key_wrapped BLOB,
  started_at INTEGER,
  finished_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_execution PRIMARY KEY (id),
  CONSTRAINT uq_execution_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_execution_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_execution_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_execution_project_id_length CHECK (length(project_id) = 16),
  CONSTRAINT ck_execution_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_execution_flow_version_id_length CHECK (length(flow_version_id) = 16),
  CONSTRAINT ck_execution_flow_snapshot_id_length CHECK (length(flow_snapshot_id) = 16),
  CONSTRAINT ck_execution_status CHECK (status IN ('QUEUED', 'RUNNING', 'WAITING', 'SUSPENDED', 'CANCELING', 'COMPENSATING', 'SUCCEEDED', 'FAILED', 'CANCELED')),
  CONSTRAINT ck_execution_mode CHECK (mode IN ('MANUAL', 'TRIGGER', 'TEST', 'CHILD', 'RETRY')),
  CONSTRAINT ck_execution_priority_range CHECK (priority >= 0),
  CONSTRAINT ck_execution_parent_execution_id_length CHECK (length(parent_execution_id) = 16),
  CONSTRAINT ck_execution_parent_token_id_length CHECK (length(parent_token_id) = 16),
  CONSTRAINT ck_execution_root_execution_id_length CHECK (length(root_execution_id) = 16),
  CONSTRAINT ck_execution_call_depth_range CHECK (call_depth >= 0),
  CONSTRAINT ck_execution_call_chain_document_pair CHECK ((call_chain_document IS NULL) = (call_chain_document_format IS NULL)),
  CONSTRAINT ck_execution_call_chain_document_format CHECK (call_chain_document_format IN ('orcon/call-chain@1')),
  CONSTRAINT ck_execution_retried_execution_id_length CHECK (length(retried_execution_id) = 16),
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
  CONSTRAINT fk_execution_parent_token FOREIGN KEY (org_id, parent_token_id) REFERENCES token (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_root_execution FOREIGN KEY (org_id, root_execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_retried_execution FOREIGN KEY (org_id, retried_execution_id) REFERENCES execution (org_id, id) ON DELETE RESTRICT
) STRICT;

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

CREATE TABLE token (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  status TEXT NOT NULL,
  node_id TEXT NOT NULL,
  edge_id TEXT,
  scope_path TEXT NOT NULL,
  fork_lineage_document BLOB NOT NULL,
  fork_lineage_document_format TEXT NOT NULL,
  payload_reference TEXT,
  fairness_key TEXT NOT NULL,
  effective_priority INTEGER NOT NULL,
  available_at INTEGER NOT NULL,
  -- The worker holding the lease; no foreign key, a worker row is informational.
  lease_owner BLOB,
  lease_epoch INTEGER NOT NULL,
  lease_expires_at INTEGER,
  claim_count INTEGER NOT NULL,
  abandoned_count INTEGER NOT NULL,
  deferred_until_at INTEGER,
  suspend_reason TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_token PRIMARY KEY (id),
  CONSTRAINT uq_token_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_token_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_token_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_token_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_token_status CHECK (status IN ('RUNNABLE', 'LEASED', 'WAITING', 'DEFERRED', 'SUSPENDED', 'CONSUMED', 'COMPLETED', 'CANCELED', 'FAILED')),
  CONSTRAINT ck_token_node_id_length CHECK (length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_token_edge_id_length CHECK (length(edge_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_token_fork_lineage_document_format CHECK (fork_lineage_document_format IN ('orcon/fork-lineage@1')),
  CONSTRAINT ck_token_fairness_key_length CHECK (length(fairness_key) BETWEEN 1 AND 300),
  CONSTRAINT ck_token_effective_priority_range CHECK (effective_priority >= 0),
  CONSTRAINT ck_token_lease_owner_length CHECK (length(lease_owner) = 16),
  CONSTRAINT ck_token_lease_epoch_range CHECK (lease_epoch >= 0),
  CONSTRAINT ck_token_claim_count_range CHECK (claim_count >= 0),
  CONSTRAINT ck_token_abandoned_count_range CHECK (abandoned_count >= 0),
  CONSTRAINT ck_token_suspend_reason CHECK (suspend_reason IN ('POISON', 'UNCERTAIN_OPERATION', 'COMPENSATION_REVIEW')),
  CONSTRAINT ck_token_lease CHECK ((status <> 'LEASED') OR (lease_owner IS NOT NULL AND lease_expires_at IS NOT NULL)),
  CONSTRAINT fk_token_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_token_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports ClaimTokens and orcon_engine.claim_tokens: claims run as orcon_engine, so the index does not lead with org_id.
CREATE INDEX ix_token_effective_priority_available_at_id ON token (effective_priority, available_at, id) WHERE status = 'RUNNABLE';

-- Supports ReleaseExpiredLeases (the reaper).
CREATE INDEX ix_token_lease_expires_at ON token (lease_expires_at) WHERE status = 'LEASED';

-- Supports RenewLeases and ReleaseWorkerLeases.
CREATE INDEX ix_token_lease_owner ON token (lease_owner) WHERE status = 'LEASED';

-- Supports fk_token_execution and the delete of its parent row.
CREATE INDEX ix_token_org_id_execution_id ON token (org_id, execution_id);

CREATE TABLE node_execution (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  token_id BLOB NOT NULL,
  node_id TEXT NOT NULL,
  scope_path TEXT NOT NULL,
  status TEXT NOT NULL,
  attempt_count INTEGER NOT NULL,
  item_state_document BLOB,
  item_state_document_format TEXT,
  item_route_document BLOB,
  item_route_document_format TEXT,
  output_reference_document BLOB,
  output_reference_document_format TEXT,
  error_document BLOB,
  error_document_format TEXT,
  started_at INTEGER,
  finished_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_node_execution PRIMARY KEY (id),
  CONSTRAINT uq_node_execution_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_node_execution_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_node_execution_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_node_execution_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_node_execution_token_id_length CHECK (length(token_id) = 16),
  CONSTRAINT ck_node_execution_node_id_length CHECK (length(node_id) BETWEEN 1 AND 200),
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
) STRICT;

-- Supports ListNodeExecutions (keyset by identity within an execution) and the execution foreign key.
CREATE INDEX ix_node_execution_org_id_execution_id_id ON node_execution (org_id, execution_id, id);

-- Supports fk_node_execution_token and the delete of its parent row.
CREATE INDEX ix_node_execution_org_id_token_id ON node_execution (org_id, token_id);

CREATE TABLE task_attempt (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  node_execution_id BLOB NOT NULL,
  attempt_number INTEGER NOT NULL,
  lease_epoch INTEGER NOT NULL,
  -- NULL when the worker row was removed.
  worker_id BLOB,
  status TEXT NOT NULL,
  item_document BLOB,
  item_document_format TEXT,
  error_document BLOB,
  error_document_format TEXT,
  started_at INTEGER NOT NULL,
  finished_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_task_attempt PRIMARY KEY (id),
  CONSTRAINT uq_task_attempt_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_task_attempt_org_id_node_execution_id_attempt_number UNIQUE (org_id, node_execution_id, attempt_number),
  CONSTRAINT ck_task_attempt_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_task_attempt_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_task_attempt_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_task_attempt_node_execution_id_length CHECK (length(node_execution_id) = 16),
  CONSTRAINT ck_task_attempt_attempt_number_range CHECK (attempt_number >= 1),
  CONSTRAINT ck_task_attempt_lease_epoch_range CHECK (lease_epoch >= 0),
  CONSTRAINT ck_task_attempt_worker_id_length CHECK (length(worker_id) = 16),
  CONSTRAINT ck_task_attempt_status CHECK (status IN ('STARTED', 'SUCCEEDED', 'FAILED', 'TIMED_OUT', 'ABANDONED', 'CANCELED')),
  CONSTRAINT ck_task_attempt_item_document_pair CHECK ((item_document IS NULL) = (item_document_format IS NULL)),
  CONSTRAINT ck_task_attempt_item_document_format CHECK (item_document_format IN ('orcon/items@1')),
  CONSTRAINT ck_task_attempt_error_document_pair CHECK ((error_document IS NULL) = (error_document_format IS NULL)),
  CONSTRAINT ck_task_attempt_error_document_format CHECK (error_document_format IN ('orcon/error@1')),
  CONSTRAINT fk_task_attempt_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_task_attempt_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_task_attempt_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_task_attempt_worker FOREIGN KEY (worker_id) REFERENCES worker (id) ON DELETE SET NULL
) STRICT;

-- Supports fk_task_attempt_execution and the delete of its parent row.
CREATE INDEX ix_task_attempt_org_id_execution_id ON task_attempt (org_id, execution_id);

-- Supports fk_task_attempt_worker and the delete of its parent row.
CREATE INDEX ix_task_attempt_worker_id ON task_attempt (worker_id) WHERE worker_id IS NOT NULL;
