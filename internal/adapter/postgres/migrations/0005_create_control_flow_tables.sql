-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE join_instance (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  node_id text NOT NULL,
  scope_path text NOT NULL,
  round_number bigint NOT NULL,
  -- The corresponding split instance of a LET_FINISH round; NULL otherwise.
  split_node_execution_id uuid,
  edge_state_document bytea NOT NULL,
  edge_state_document_format text NOT NULL,
  status text NOT NULL,
  continuation_token_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_join_instance PRIMARY KEY (id),
  CONSTRAINT uq_join_instance_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_join_instance_execution_node_scope_round UNIQUE (org_id, execution_id, node_id, scope_path, round_number),
  CONSTRAINT ck_join_instance_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_join_instance_round_number_range CHECK (round_number >= 1),
  CONSTRAINT ck_join_instance_edge_state_document_format CHECK (edge_state_document_format IN ('orcon/edge-state@1')),
  CONSTRAINT ck_join_instance_status CHECK (status IN ('OPEN', 'FIRED', 'FAILED', 'DEAD', 'CANCELED', 'RESET')),
  CONSTRAINT fk_join_instance_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_join_instance_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_join_instance_split_node_execution FOREIGN KEY (org_id, split_node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_join_instance_continuation_token FOREIGN KEY (org_id, continuation_token_id) REFERENCES token (org_id, id) ON DELETE CASCADE
);

-- One round per split instance and join node: (org_id, split_node_execution_id, node_id, round_number); the name is a purpose because the columns exceed 63 bytes.
CREATE UNIQUE INDEX uq_join_instance_split_round ON join_instance (org_id, split_node_execution_id, node_id, round_number) WHERE split_node_execution_id IS NOT NULL;

-- Supports fk_join_instance_continuation_token and the delete of its parent row.
CREATE INDEX ix_join_instance_org_id_continuation_token_id ON join_instance (org_id, continuation_token_id) WHERE continuation_token_id IS NOT NULL;

ALTER TABLE join_instance ENABLE ROW LEVEL SECURITY;
ALTER TABLE join_instance FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON join_instance FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON join_instance TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON join_instance TO orcon_engine;

CREATE TABLE join_arrival (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  node_id text NOT NULL,
  scope_path text NOT NULL,
  round_number bigint NOT NULL,
  edge_index bigint NOT NULL,
  token_id uuid NOT NULL,
  payload_reference text,
  is_live boolean NOT NULL,
  arrived_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_join_arrival PRIMARY KEY (id),
  CONSTRAINT uq_join_arrival_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_join_arrival_edge_per_round UNIQUE (org_id, execution_id, node_id, scope_path, round_number, edge_index),
  CONSTRAINT ck_join_arrival_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_join_arrival_round_number_range CHECK (round_number >= 1),
  CONSTRAINT ck_join_arrival_edge_index_range CHECK (edge_index >= 0),
  CONSTRAINT fk_join_arrival_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_join_arrival_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_join_arrival_token FOREIGN KEY (org_id, token_id) REFERENCES token (org_id, id) ON DELETE CASCADE
);

-- Supports fk_join_arrival_token and the delete of its parent row.
CREATE INDEX ix_join_arrival_org_id_token_id ON join_arrival (org_id, token_id);

ALTER TABLE join_arrival ENABLE ROW LEVEL SECURITY;
ALTER TABLE join_arrival FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON join_arrival FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON join_arrival TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON join_arrival TO orcon_engine;

CREATE TABLE merge_tracker (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  node_id text NOT NULL,
  scope_path text NOT NULL,
  edge_state_document bytea NOT NULL,
  edge_state_document_format text NOT NULL,
  status text NOT NULL,
  continuation_token_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_merge_tracker PRIMARY KEY (id),
  CONSTRAINT uq_merge_tracker_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_merge_tracker_org_id_execution_id_node_id_scope_path UNIQUE (org_id, execution_id, node_id, scope_path),
  CONSTRAINT ck_merge_tracker_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_merge_tracker_edge_state_document_format CHECK (edge_state_document_format IN ('orcon/edge-state@1')),
  CONSTRAINT ck_merge_tracker_status CHECK (status IN ('OPEN', 'MERGED', 'DEAD')),
  CONSTRAINT fk_merge_tracker_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_merge_tracker_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_merge_tracker_continuation_token FOREIGN KEY (org_id, continuation_token_id) REFERENCES token (org_id, id) ON DELETE CASCADE
);

-- Supports fk_merge_tracker_continuation_token and the delete of its parent row.
CREATE INDEX ix_merge_tracker_org_id_continuation_token_id ON merge_tracker (org_id, continuation_token_id) WHERE continuation_token_id IS NOT NULL;

ALTER TABLE merge_tracker ENABLE ROW LEVEL SECURITY;
ALTER TABLE merge_tracker FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON merge_tracker FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON merge_tracker TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON merge_tracker TO orcon_engine;

CREATE TABLE multi_instance (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  node_id text NOT NULL,
  scope_path text NOT NULL,
  total_count bigint NOT NULL,
  started_count bigint NOT NULL,
  completed_count bigint NOT NULL,
  in_flight_count bigint NOT NULL,
  status text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_multi_instance PRIMARY KEY (id),
  CONSTRAINT uq_multi_instance_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_multi_instance_org_id_execution_id_node_id_scope_path UNIQUE (org_id, execution_id, node_id, scope_path),
  CONSTRAINT ck_multi_instance_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_multi_instance_total_count_range CHECK (total_count >= 0),
  CONSTRAINT ck_multi_instance_started_count_range CHECK (started_count >= 0),
  CONSTRAINT ck_multi_instance_completed_count_range CHECK (completed_count >= 0),
  CONSTRAINT ck_multi_instance_in_flight_count_range CHECK (in_flight_count >= 0),
  CONSTRAINT ck_multi_instance_status CHECK (status IN ('RUNNING', 'COMPLETED', 'FAILED', 'CANCELED')),
  CONSTRAINT fk_multi_instance_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_multi_instance_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE
);

ALTER TABLE multi_instance ENABLE ROW LEVEL SECURITY;
ALTER TABLE multi_instance FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON multi_instance FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON multi_instance TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON multi_instance TO orcon_engine;

CREATE TABLE timer (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  token_id uuid NOT NULL,
  kind text NOT NULL,
  due_at timestamptz NOT NULL,
  status text NOT NULL,
  payload_document bytea,
  payload_document_format text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_timer PRIMARY KEY (id),
  CONSTRAINT uq_timer_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_timer_kind CHECK (kind IN ('WAIT', 'RETRY_DELAY', 'BOUNDARY', 'NODE_TIMEOUT', 'EXECUTION_TIMEOUT')),
  CONSTRAINT ck_timer_status CHECK (status IN ('SCHEDULED', 'FIRED', 'CANCELED')),
  CONSTRAINT ck_timer_payload_document_pair CHECK ((payload_document IS NULL) = (payload_document_format IS NULL)),
  CONSTRAINT ck_timer_payload_document_format CHECK (payload_document_format IN ('orcon/timer-payload@1')),
  CONSTRAINT fk_timer_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_timer_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_timer_token FOREIGN KEY (org_id, token_id) REFERENCES token (org_id, id) ON DELETE CASCADE
);

-- Supports FireDueTimers and orcon_engine.fire_due_timers: due timers span organizations.
CREATE INDEX ix_timer_due_at_id ON timer (due_at, id) WHERE status = 'SCHEDULED';

-- Supports fk_timer_execution and the delete of its parent row.
CREATE INDEX ix_timer_org_id_execution_id ON timer (org_id, execution_id);

-- Supports fk_timer_token and the delete of its parent row.
CREATE INDEX ix_timer_org_id_token_id ON timer (org_id, token_id);

ALTER TABLE timer ENABLE ROW LEVEL SECURITY;
ALTER TABLE timer FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON timer FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON timer TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON timer TO orcon_engine;

CREATE TABLE wait (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  token_id uuid NOT NULL,
  node_execution_id uuid NOT NULL,
  kind text NOT NULL,
  wait_group_key text,
  secret_hash bytea,
  expires_at timestamptz,
  status text NOT NULL,
  resume_payload_reference text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_wait PRIMARY KEY (id),
  CONSTRAINT uq_wait_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_wait_kind CHECK (kind IN ('TIMER', 'SIGNAL', 'HUMAN', 'CHILD')),
  CONSTRAINT ck_wait_secret_hash_length CHECK (octet_length(secret_hash) = 32),
  CONSTRAINT ck_wait_status CHECK (status IN ('OPEN', 'RESOLVED', 'EXPIRED', 'CANCELED')),
  CONSTRAINT fk_wait_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_wait_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_wait_token FOREIGN KEY (org_id, token_id) REFERENCES token (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_wait_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE
);

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

ALTER TABLE wait ENABLE ROW LEVEL SECURITY;
ALTER TABLE wait FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON wait FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON wait TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON wait TO orcon_engine;

CREATE TABLE human_task (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  wait_id uuid NOT NULL,
  node_execution_id uuid NOT NULL,
  kind text NOT NULL,
  title text NOT NULL,
  assignee_document bytea NOT NULL,
  assignee_document_format text NOT NULL,
  form_document bytea,
  form_document_format text,
  status text NOT NULL,
  response_document bytea,
  response_document_format text,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  resolved_by_principal_id uuid,
  resolved_at timestamptz,
  expires_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_human_task PRIMARY KEY (id),
  CONSTRAINT uq_human_task_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_human_task_org_id_wait_id UNIQUE (org_id, wait_id),
  CONSTRAINT ck_human_task_kind CHECK (kind IN ('FORM', 'APPROVAL', 'TOOL_APPROVAL')),
  CONSTRAINT ck_human_task_title_length CHECK (char_length(title) BETWEEN 1 AND 500),
  CONSTRAINT ck_human_task_assignee_document_format CHECK (assignee_document_format IN ('orcon/assignee@1')),
  CONSTRAINT ck_human_task_form_document_pair CHECK ((form_document IS NULL) = (form_document_format IS NULL)),
  CONSTRAINT ck_human_task_form_document_format CHECK (form_document_format IN ('orcon/form@1')),
  CONSTRAINT ck_human_task_status CHECK (status IN ('OPEN', 'RESOLVED', 'EXPIRED', 'CANCELED')),
  CONSTRAINT ck_human_task_response_document_pair CHECK ((response_document IS NULL) = (response_document_format IS NULL)),
  CONSTRAINT ck_human_task_response_document_format CHECK (response_document_format IN ('orcon/human-response@1')),
  CONSTRAINT fk_human_task_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_human_task_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_human_task_wait FOREIGN KEY (org_id, wait_id) REFERENCES wait (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_human_task_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_human_task_resolved_by_principal FOREIGN KEY (resolved_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports fk_human_task_execution and the delete of its parent row.
CREATE INDEX ix_human_task_org_id_execution_id ON human_task (org_id, execution_id);

-- Supports fk_human_task_node_execution and the delete of its parent row.
CREATE INDEX ix_human_task_org_id_node_execution_id ON human_task (org_id, node_execution_id);

-- Supports fk_human_task_resolved_by_principal and the delete of its parent row.
CREATE INDEX ix_human_task_resolved_by_principal_id ON human_task (resolved_by_principal_id) WHERE resolved_by_principal_id IS NOT NULL;

ALTER TABLE human_task ENABLE ROW LEVEL SECURITY;
ALTER TABLE human_task FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON human_task FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON human_task TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON human_task TO orcon_engine;
