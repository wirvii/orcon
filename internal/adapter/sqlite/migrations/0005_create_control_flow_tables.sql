-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE join_instance (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  node_id TEXT NOT NULL,
  scope_path TEXT NOT NULL,
  round_number INTEGER NOT NULL,
  -- The corresponding split instance of a LET_FINISH round; NULL otherwise.
  split_node_execution_id BLOB,
  edge_state_document BLOB NOT NULL,
  edge_state_document_format TEXT NOT NULL,
  status TEXT NOT NULL,
  continuation_token_id BLOB,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_join_instance PRIMARY KEY (id),
  CONSTRAINT uq_join_instance_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_join_instance_execution_node_scope_round UNIQUE (org_id, execution_id, node_id, scope_path, round_number),
  CONSTRAINT ck_join_instance_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_join_instance_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_join_instance_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_join_instance_node_id_length CHECK (length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_join_instance_round_number_range CHECK (round_number >= 1),
  CONSTRAINT ck_join_instance_split_node_execution_id_length CHECK (length(split_node_execution_id) = 16),
  CONSTRAINT ck_join_instance_edge_state_document_format CHECK (edge_state_document_format IN ('orcon/edge-state@1')),
  CONSTRAINT ck_join_instance_status CHECK (status IN ('OPEN', 'FIRED', 'FAILED', 'DEAD', 'CANCELED', 'RESET')),
  CONSTRAINT ck_join_instance_continuation_token_id_length CHECK (length(continuation_token_id) = 16),
  CONSTRAINT fk_join_instance_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_join_instance_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_join_instance_split_node_execution FOREIGN KEY (org_id, split_node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_join_instance_continuation_token FOREIGN KEY (org_id, continuation_token_id) REFERENCES token (org_id, id) ON DELETE CASCADE
) STRICT;

-- One round per split instance and join node: (org_id, split_node_execution_id, node_id, round_number); the name is a purpose because the columns exceed 63 bytes.
CREATE UNIQUE INDEX uq_join_instance_split_round ON join_instance (org_id, split_node_execution_id, node_id, round_number) WHERE split_node_execution_id IS NOT NULL;

-- Supports fk_join_instance_continuation_token and the delete of its parent row.
CREATE INDEX ix_join_instance_org_id_continuation_token_id ON join_instance (org_id, continuation_token_id) WHERE continuation_token_id IS NOT NULL;

CREATE TABLE join_arrival (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  node_id TEXT NOT NULL,
  scope_path TEXT NOT NULL,
  round_number INTEGER NOT NULL,
  edge_index INTEGER NOT NULL,
  token_id BLOB NOT NULL,
  payload_reference TEXT,
  is_live INTEGER NOT NULL,
  arrived_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_join_arrival PRIMARY KEY (id),
  CONSTRAINT uq_join_arrival_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_join_arrival_edge_per_round UNIQUE (org_id, execution_id, node_id, scope_path, round_number, edge_index),
  CONSTRAINT ck_join_arrival_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_join_arrival_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_join_arrival_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_join_arrival_node_id_length CHECK (length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_join_arrival_round_number_range CHECK (round_number >= 1),
  CONSTRAINT ck_join_arrival_edge_index_range CHECK (edge_index >= 0),
  CONSTRAINT ck_join_arrival_token_id_length CHECK (length(token_id) = 16),
  CONSTRAINT ck_join_arrival_is_live CHECK (is_live IN (0, 1)),
  CONSTRAINT fk_join_arrival_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_join_arrival_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_join_arrival_token FOREIGN KEY (org_id, token_id) REFERENCES token (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports fk_join_arrival_token and the delete of its parent row.
CREATE INDEX ix_join_arrival_org_id_token_id ON join_arrival (org_id, token_id);

CREATE TABLE merge_tracker (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  node_id TEXT NOT NULL,
  scope_path TEXT NOT NULL,
  edge_state_document BLOB NOT NULL,
  edge_state_document_format TEXT NOT NULL,
  status TEXT NOT NULL,
  continuation_token_id BLOB,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_merge_tracker PRIMARY KEY (id),
  CONSTRAINT uq_merge_tracker_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_merge_tracker_org_id_execution_id_node_id_scope_path UNIQUE (org_id, execution_id, node_id, scope_path),
  CONSTRAINT ck_merge_tracker_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_merge_tracker_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_merge_tracker_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_merge_tracker_node_id_length CHECK (length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_merge_tracker_edge_state_document_format CHECK (edge_state_document_format IN ('orcon/edge-state@1')),
  CONSTRAINT ck_merge_tracker_status CHECK (status IN ('OPEN', 'MERGED', 'DEAD')),
  CONSTRAINT ck_merge_tracker_continuation_token_id_length CHECK (length(continuation_token_id) = 16),
  CONSTRAINT fk_merge_tracker_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_merge_tracker_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_merge_tracker_continuation_token FOREIGN KEY (org_id, continuation_token_id) REFERENCES token (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports fk_merge_tracker_continuation_token and the delete of its parent row.
CREATE INDEX ix_merge_tracker_org_id_continuation_token_id ON merge_tracker (org_id, continuation_token_id) WHERE continuation_token_id IS NOT NULL;

CREATE TABLE multi_instance (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  node_id TEXT NOT NULL,
  scope_path TEXT NOT NULL,
  total_count INTEGER NOT NULL,
  started_count INTEGER NOT NULL,
  completed_count INTEGER NOT NULL,
  in_flight_count INTEGER NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_multi_instance PRIMARY KEY (id),
  CONSTRAINT uq_multi_instance_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_multi_instance_org_id_execution_id_node_id_scope_path UNIQUE (org_id, execution_id, node_id, scope_path),
  CONSTRAINT ck_multi_instance_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_multi_instance_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_multi_instance_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_multi_instance_node_id_length CHECK (length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_multi_instance_total_count_range CHECK (total_count >= 0),
  CONSTRAINT ck_multi_instance_started_count_range CHECK (started_count >= 0),
  CONSTRAINT ck_multi_instance_completed_count_range CHECK (completed_count >= 0),
  CONSTRAINT ck_multi_instance_in_flight_count_range CHECK (in_flight_count >= 0),
  CONSTRAINT ck_multi_instance_status CHECK (status IN ('RUNNING', 'COMPLETED', 'FAILED', 'CANCELED')),
  CONSTRAINT fk_multi_instance_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_multi_instance_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE
) STRICT;


CREATE TABLE timer (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  token_id BLOB NOT NULL,
  kind TEXT NOT NULL,
  due_at INTEGER NOT NULL,
  status TEXT NOT NULL,
  payload_document BLOB,
  payload_document_format TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_timer PRIMARY KEY (id),
  CONSTRAINT uq_timer_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_timer_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_timer_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_timer_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_timer_token_id_length CHECK (length(token_id) = 16),
  CONSTRAINT ck_timer_kind CHECK (kind IN ('WAIT', 'RETRY_DELAY', 'BOUNDARY', 'NODE_TIMEOUT', 'EXECUTION_TIMEOUT')),
  CONSTRAINT ck_timer_status CHECK (status IN ('SCHEDULED', 'FIRED', 'CANCELED')),
  CONSTRAINT ck_timer_payload_document_pair CHECK ((payload_document IS NULL) = (payload_document_format IS NULL)),
  CONSTRAINT ck_timer_payload_document_format CHECK (payload_document_format IN ('orcon/timer-payload@1')),
  CONSTRAINT fk_timer_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_timer_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_timer_token FOREIGN KEY (org_id, token_id) REFERENCES token (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports FireDueTimers and orcon_engine.fire_due_timers: due timers span organizations.
CREATE INDEX ix_timer_due_at_id ON timer (due_at, id) WHERE status = 'SCHEDULED';

-- Supports fk_timer_execution and the delete of its parent row.
CREATE INDEX ix_timer_org_id_execution_id ON timer (org_id, execution_id);

-- Supports fk_timer_token and the delete of its parent row.
CREATE INDEX ix_timer_org_id_token_id ON timer (org_id, token_id);

CREATE TABLE wait (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  token_id BLOB NOT NULL,
  node_execution_id BLOB NOT NULL,
  kind TEXT NOT NULL,
  wait_group_key TEXT,
  secret_hash BLOB,
  expires_at INTEGER,
  status TEXT NOT NULL,
  resume_payload_reference TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_wait PRIMARY KEY (id),
  CONSTRAINT uq_wait_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_wait_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_wait_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_wait_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_wait_token_id_length CHECK (length(token_id) = 16),
  CONSTRAINT ck_wait_node_execution_id_length CHECK (length(node_execution_id) = 16),
  CONSTRAINT ck_wait_kind CHECK (kind IN ('TIMER', 'SIGNAL', 'HUMAN', 'CHILD')),
  CONSTRAINT ck_wait_secret_hash_length CHECK (length(secret_hash) = 32),
  CONSTRAINT ck_wait_status CHECK (status IN ('OPEN', 'RESOLVED', 'EXPIRED', 'CANCELED')),
  CONSTRAINT fk_wait_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_wait_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_wait_token FOREIGN KEY (org_id, token_id) REFERENCES token (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_wait_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports the event-based gateway: the waits of one group resolve together.
CREATE INDEX ix_wait_org_id_wait_group_key ON wait (org_id, wait_group_key) WHERE wait_group_key IS NOT NULL;

-- Supports the expiry of open waits.
CREATE INDEX ix_wait_expires_at_id ON wait (expires_at, id) WHERE status = 'OPEN' AND expires_at IS NOT NULL;

-- Supports fk_wait_execution and the delete of its parent row.
CREATE INDEX ix_wait_org_id_execution_id ON wait (org_id, execution_id);

-- Supports fk_wait_token and the delete of its parent row.
CREATE INDEX ix_wait_org_id_token_id ON wait (org_id, token_id);

-- Supports fk_wait_node_execution and the delete of its parent row.
CREATE INDEX ix_wait_org_id_node_execution_id ON wait (org_id, node_execution_id);

CREATE TABLE human_task (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  wait_id BLOB NOT NULL,
  node_execution_id BLOB NOT NULL,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  assignee_document BLOB NOT NULL,
  assignee_document_format TEXT NOT NULL,
  form_document BLOB,
  form_document_format TEXT,
  status TEXT NOT NULL,
  response_document BLOB,
  response_document_format TEXT,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  resolved_by_principal_id BLOB,
  resolved_at INTEGER,
  expires_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_human_task PRIMARY KEY (id),
  CONSTRAINT uq_human_task_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_human_task_org_id_wait_id UNIQUE (org_id, wait_id),
  CONSTRAINT ck_human_task_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_human_task_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_human_task_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_human_task_wait_id_length CHECK (length(wait_id) = 16),
  CONSTRAINT ck_human_task_node_execution_id_length CHECK (length(node_execution_id) = 16),
  CONSTRAINT ck_human_task_kind CHECK (kind IN ('FORM', 'APPROVAL', 'TOOL_APPROVAL')),
  CONSTRAINT ck_human_task_title_length CHECK (length(title) BETWEEN 1 AND 500),
  CONSTRAINT ck_human_task_assignee_document_format CHECK (assignee_document_format IN ('orcon/assignee@1')),
  CONSTRAINT ck_human_task_form_document_pair CHECK ((form_document IS NULL) = (form_document_format IS NULL)),
  CONSTRAINT ck_human_task_form_document_format CHECK (form_document_format IN ('orcon/form@1')),
  CONSTRAINT ck_human_task_status CHECK (status IN ('OPEN', 'RESOLVED', 'EXPIRED', 'CANCELED')),
  CONSTRAINT ck_human_task_response_document_pair CHECK ((response_document IS NULL) = (response_document_format IS NULL)),
  CONSTRAINT ck_human_task_response_document_format CHECK (response_document_format IN ('orcon/human-response@1')),
  CONSTRAINT ck_human_task_resolved_by_principal_id_length CHECK (length(resolved_by_principal_id) = 16),
  CONSTRAINT fk_human_task_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_human_task_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_human_task_wait FOREIGN KEY (org_id, wait_id) REFERENCES wait (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_human_task_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_human_task_resolved_by_principal FOREIGN KEY (resolved_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports fk_human_task_execution and the delete of its parent row.
CREATE INDEX ix_human_task_org_id_execution_id ON human_task (org_id, execution_id);

-- Supports fk_human_task_node_execution and the delete of its parent row.
CREATE INDEX ix_human_task_org_id_node_execution_id ON human_task (org_id, node_execution_id);

-- Supports fk_human_task_resolved_by_principal and the delete of its parent row.
CREATE INDEX ix_human_task_resolved_by_principal_id ON human_task (resolved_by_principal_id) WHERE resolved_by_principal_id IS NOT NULL;
