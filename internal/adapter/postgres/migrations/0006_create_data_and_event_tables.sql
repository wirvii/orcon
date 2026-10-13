-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE variable (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  scope_path text NOT NULL,
  name text NOT NULL,
  value_document bytea NOT NULL,
  value_document_format text NOT NULL,
  version bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_variable PRIMARY KEY (id),
  CONSTRAINT uq_variable_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_variable_org_id_execution_id_scope_path_name UNIQUE (org_id, execution_id, scope_path, name),
  CONSTRAINT ck_variable_name_length CHECK (char_length(name) BETWEEN 1 AND 200),
  CONSTRAINT ck_variable_value_document_format CHECK (value_document_format IN ('orcon/value@1')),
  CONSTRAINT ck_variable_version_range CHECK (version >= 0),
  CONSTRAINT fk_variable_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_variable_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE
);

ALTER TABLE variable ENABLE ROW LEVEL SECURITY;
ALTER TABLE variable FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON variable FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON variable TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON variable TO orcon_engine;

CREATE TABLE variable_change (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  sequence_number bigint NOT NULL,
  execution_id uuid NOT NULL,
  scope_path text NOT NULL,
  name text NOT NULL,
  node_execution_id uuid,
  changed_at timestamptz NOT NULL,
  old_value_reference text,
  new_value_reference text,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_variable_change PRIMARY KEY (id),
  CONSTRAINT uq_variable_change_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_variable_change_org_id_execution_id_sequence_number UNIQUE (org_id, execution_id, sequence_number),
  CONSTRAINT ck_variable_change_sequence_number_range CHECK (sequence_number >= 1),
  CONSTRAINT ck_variable_change_name_length CHECK (char_length(name) BETWEEN 1 AND 200),
  CONSTRAINT fk_variable_change_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_variable_change_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_variable_change_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE
);

-- Supports fk_variable_change_node_execution and the delete of its parent row.
CREATE INDEX ix_variable_change_org_id_node_execution_id ON variable_change (org_id, node_execution_id) WHERE node_execution_id IS NOT NULL;

ALTER TABLE variable_change ENABLE ROW LEVEL SECURITY;
ALTER TABLE variable_change FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON variable_change FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT ON variable_change TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON variable_change TO orcon_engine;

CREATE TABLE flow_data (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  data_key text NOT NULL,
  value_document bytea NOT NULL,
  value_document_format text NOT NULL,
  version bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_flow_data PRIMARY KEY (id),
  CONSTRAINT uq_flow_data_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_data_org_id_flow_id_data_key UNIQUE (org_id, flow_id, data_key),
  CONSTRAINT ck_flow_data_data_key_length CHECK (char_length(data_key) BETWEEN 1 AND 200),
  CONSTRAINT ck_flow_data_value_document_format CHECK (value_document_format IN ('orcon/value@1')),
  CONSTRAINT ck_flow_data_version_range CHECK (version >= 0),
  CONSTRAINT fk_flow_data_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_data_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
);

ALTER TABLE flow_data ENABLE ROW LEVEL SECURITY;
ALTER TABLE flow_data FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON flow_data FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON flow_data TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON flow_data TO orcon_engine;

CREATE TABLE flow_data_operation (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  operation_key text NOT NULL,
  result_document bytea NOT NULL,
  result_document_format text NOT NULL,
  applied_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_flow_data_operation PRIMARY KEY (id),
  CONSTRAINT uq_flow_data_operation_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_data_operation_org_id_operation_key UNIQUE (org_id, operation_key),
  CONSTRAINT ck_flow_data_operation_operation_key_length CHECK (char_length(operation_key) BETWEEN 1 AND 200),
  CONSTRAINT ck_flow_data_operation_result_document_format CHECK (result_document_format IN ('orcon/value@1')),
  CONSTRAINT fk_flow_data_operation_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

ALTER TABLE flow_data_operation ENABLE ROW LEVEL SECURITY;
ALTER TABLE flow_data_operation FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON flow_data_operation FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON flow_data_operation TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON flow_data_operation TO orcon_engine;

CREATE TABLE nondeterminism_record (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  node_execution_id uuid NOT NULL,
  sequence_number bigint NOT NULL,
  value_document bytea NOT NULL,
  value_document_format text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_nondeterminism_record PRIMARY KEY (id),
  CONSTRAINT uq_nondeterminism_record_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_nondeterminism_record_node_execution_sequence UNIQUE (org_id, node_execution_id, sequence_number),
  CONSTRAINT ck_nondeterminism_record_sequence_number_range CHECK (sequence_number >= 1),
  CONSTRAINT ck_nondeterminism_record_value_document_format CHECK (value_document_format IN ('orcon/value@1')),
  CONSTRAINT fk_nondeterminism_record_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_nondeterminism_record_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_nondeterminism_record_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE
);

-- Supports fk_nondeterminism_record_execution and the delete of its parent row.
CREATE INDEX ix_nondeterminism_record_org_id_execution_id ON nondeterminism_record (org_id, execution_id);

ALTER TABLE nondeterminism_record ENABLE ROW LEVEL SECURITY;
ALTER TABLE nondeterminism_record FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON nondeterminism_record FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT ON nondeterminism_record TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON nondeterminism_record TO orcon_engine;

CREATE TABLE execution_event (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  root_execution_id uuid NOT NULL,
  sequence_number bigint NOT NULL,
  occurred_at timestamptz NOT NULL,
  type text NOT NULL,
  schema_version bigint NOT NULL,
  reference_document bytea NOT NULL,
  reference_document_format text NOT NULL,
  data_document bytea,
  data_document_format text,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_execution_event PRIMARY KEY (id),
  CONSTRAINT uq_execution_event_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_execution_event_org_id_execution_id_sequence_number UNIQUE (org_id, execution_id, sequence_number),
  CONSTRAINT ck_execution_event_sequence_number_range CHECK (sequence_number >= 1),
  CONSTRAINT ck_execution_event_type_length CHECK (char_length(type) BETWEEN 1 AND 100),
  CONSTRAINT ck_execution_event_schema_version_range CHECK (schema_version >= 1),
  CONSTRAINT ck_execution_event_reference_document_format CHECK (reference_document_format IN ('orcon/event-reference@1')),
  CONSTRAINT ck_execution_event_data_document_pair CHECK ((data_document IS NULL) = (data_document_format IS NULL)),
  CONSTRAINT ck_execution_event_data_document_format CHECK (data_document_format IN ('orcon/event-data@1')),
  CONSTRAINT fk_execution_event_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_event_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_event_root_execution FOREIGN KEY (org_id, root_execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE
);

-- Supports fk_execution_event_root_execution and the delete of its parent row.
CREATE INDEX ix_execution_event_org_id_root_execution_id ON execution_event (org_id, root_execution_id);

ALTER TABLE execution_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE execution_event FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON execution_event FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT ON execution_event TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON execution_event TO orcon_engine;

CREATE TABLE outbox (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  topic text NOT NULL,
  payload_document bytea NOT NULL,
  payload_document_format text NOT NULL,
  lease_owner uuid,
  lease_expires_at timestamptz,
  delivered_at timestamptz,
  attempt_count bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_outbox PRIMARY KEY (id),
  CONSTRAINT uq_outbox_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_outbox_topic_length CHECK (char_length(topic) BETWEEN 1 AND 200),
  CONSTRAINT ck_outbox_payload_document_format CHECK (payload_document_format IN ('orcon/outbox-payload@1')),
  CONSTRAINT ck_outbox_attempt_count_range CHECK (attempt_count >= 0),
  CONSTRAINT fk_outbox_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

-- Supports ClaimOutbox and orcon_engine.claim_outbox: deliveries span organizations.
CREATE INDEX ix_outbox_created_at_id ON outbox (created_at, id) WHERE delivered_at IS NULL;

ALTER TABLE outbox ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON outbox FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON outbox TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON outbox TO orcon_engine;

CREATE TABLE limit_bucket (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  bucket_key text NOT NULL,
  capacity bigint NOT NULL,
  refill_per_second bigint NOT NULL,
  tokens bigint NOT NULL,
  refilled_at timestamptz NOT NULL,
  blocked_until_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_limit_bucket PRIMARY KEY (id),
  CONSTRAINT uq_limit_bucket_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_limit_bucket_org_id_bucket_key UNIQUE (org_id, bucket_key),
  CONSTRAINT ck_limit_bucket_bucket_key_length CHECK (char_length(bucket_key) BETWEEN 1 AND 300),
  CONSTRAINT ck_limit_bucket_capacity_range CHECK (capacity >= 0),
  CONSTRAINT ck_limit_bucket_refill_per_second_range CHECK (refill_per_second >= 0),
  CONSTRAINT ck_limit_bucket_tokens_range CHECK (tokens >= 0),
  CONSTRAINT fk_limit_bucket_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

ALTER TABLE limit_bucket ENABLE ROW LEVEL SECURITY;
ALTER TABLE limit_bucket FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON limit_bucket FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON limit_bucket TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON limit_bucket TO orcon_engine;

CREATE TABLE trigger_rate (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  window_started_at timestamptz NOT NULL,
  fire_count bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_trigger_rate PRIMARY KEY (id),
  CONSTRAINT uq_trigger_rate_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_trigger_rate_org_id_flow_id_window_started_at UNIQUE (org_id, flow_id, window_started_at),
  CONSTRAINT ck_trigger_rate_fire_count_range CHECK (fire_count >= 0),
  CONSTRAINT fk_trigger_rate_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_trigger_rate_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
);

ALTER TABLE trigger_rate ENABLE ROW LEVEL SECURITY;
ALTER TABLE trigger_rate FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON trigger_rate FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON trigger_rate TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON trigger_rate TO orcon_engine;
