-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE variable (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  scope_path TEXT NOT NULL,
  name TEXT NOT NULL,
  value_document BLOB NOT NULL,
  value_document_format TEXT NOT NULL,
  version INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_variable PRIMARY KEY (id),
  CONSTRAINT uq_variable_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_variable_org_id_execution_id_scope_path_name UNIQUE (org_id, execution_id, scope_path, name),
  CONSTRAINT ck_variable_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_variable_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_variable_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_variable_name_length CHECK (length(name) BETWEEN 1 AND 200),
  CONSTRAINT ck_variable_value_document_format CHECK (value_document_format IN ('orcon/value@1')),
  CONSTRAINT ck_variable_version_range CHECK (version >= 0),
  CONSTRAINT fk_variable_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_variable_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE
) STRICT;


CREATE TABLE variable_change (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  sequence_number INTEGER NOT NULL,
  execution_id BLOB NOT NULL,
  scope_path TEXT NOT NULL,
  name TEXT NOT NULL,
  node_execution_id BLOB,
  changed_at INTEGER NOT NULL,
  old_value_reference TEXT,
  new_value_reference TEXT,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_variable_change PRIMARY KEY (id),
  CONSTRAINT uq_variable_change_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_variable_change_org_id_execution_id_sequence_number UNIQUE (org_id, execution_id, sequence_number),
  CONSTRAINT ck_variable_change_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_variable_change_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_variable_change_sequence_number_range CHECK (sequence_number >= 1),
  CONSTRAINT ck_variable_change_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_variable_change_name_length CHECK (length(name) BETWEEN 1 AND 200),
  CONSTRAINT ck_variable_change_node_execution_id_length CHECK (length(node_execution_id) = 16),
  CONSTRAINT fk_variable_change_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_variable_change_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_variable_change_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports fk_variable_change_node_execution and the delete of its parent row.
CREATE INDEX ix_variable_change_org_id_node_execution_id ON variable_change (org_id, node_execution_id) WHERE node_execution_id IS NOT NULL;

CREATE TABLE flow_data (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  data_key TEXT NOT NULL,
  value_document BLOB NOT NULL,
  value_document_format TEXT NOT NULL,
  version INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_flow_data PRIMARY KEY (id),
  CONSTRAINT uq_flow_data_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_data_org_id_flow_id_data_key UNIQUE (org_id, flow_id, data_key),
  CONSTRAINT ck_flow_data_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_data_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_data_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_flow_data_data_key_length CHECK (length(data_key) BETWEEN 1 AND 200),
  CONSTRAINT ck_flow_data_value_document_format CHECK (value_document_format IN ('orcon/value@1')),
  CONSTRAINT ck_flow_data_version_range CHECK (version >= 0),
  CONSTRAINT fk_flow_data_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_data_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
) STRICT;


CREATE TABLE flow_data_operation (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  operation_key TEXT NOT NULL,
  result_document BLOB NOT NULL,
  result_document_format TEXT NOT NULL,
  applied_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_flow_data_operation PRIMARY KEY (id),
  CONSTRAINT uq_flow_data_operation_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_data_operation_org_id_operation_key UNIQUE (org_id, operation_key),
  CONSTRAINT ck_flow_data_operation_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_data_operation_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_data_operation_operation_key_length CHECK (length(operation_key) BETWEEN 1 AND 200),
  CONSTRAINT ck_flow_data_operation_result_document_format CHECK (result_document_format IN ('orcon/value@1')),
  CONSTRAINT fk_flow_data_operation_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;


CREATE TABLE nondeterminism_record (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  node_execution_id BLOB NOT NULL,
  sequence_number INTEGER NOT NULL,
  value_document BLOB NOT NULL,
  value_document_format TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_nondeterminism_record PRIMARY KEY (id),
  CONSTRAINT uq_nondeterminism_record_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_nondeterminism_record_node_execution_sequence UNIQUE (org_id, node_execution_id, sequence_number),
  CONSTRAINT ck_nondeterminism_record_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_nondeterminism_record_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_nondeterminism_record_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_nondeterminism_record_node_execution_id_length CHECK (length(node_execution_id) = 16),
  CONSTRAINT ck_nondeterminism_record_sequence_number_range CHECK (sequence_number >= 1),
  CONSTRAINT ck_nondeterminism_record_value_document_format CHECK (value_document_format IN ('orcon/value@1')),
  CONSTRAINT fk_nondeterminism_record_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_nondeterminism_record_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_nondeterminism_record_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports fk_nondeterminism_record_execution and the delete of its parent row.
CREATE INDEX ix_nondeterminism_record_org_id_execution_id ON nondeterminism_record (org_id, execution_id);

CREATE TABLE execution_event (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  root_execution_id BLOB NOT NULL,
  sequence_number INTEGER NOT NULL,
  occurred_at INTEGER NOT NULL,
  type TEXT NOT NULL,
  schema_version INTEGER NOT NULL,
  reference_document BLOB NOT NULL,
  reference_document_format TEXT NOT NULL,
  data_document BLOB,
  data_document_format TEXT,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_execution_event PRIMARY KEY (id),
  CONSTRAINT uq_execution_event_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_execution_event_org_id_execution_id_sequence_number UNIQUE (org_id, execution_id, sequence_number),
  CONSTRAINT ck_execution_event_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_execution_event_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_execution_event_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_execution_event_root_execution_id_length CHECK (length(root_execution_id) = 16),
  CONSTRAINT ck_execution_event_sequence_number_range CHECK (sequence_number >= 1),
  CONSTRAINT ck_execution_event_type_length CHECK (length(type) BETWEEN 1 AND 100),
  CONSTRAINT ck_execution_event_schema_version_range CHECK (schema_version >= 1),
  CONSTRAINT ck_execution_event_reference_document_format CHECK (reference_document_format IN ('orcon/event-reference@1')),
  CONSTRAINT ck_execution_event_data_document_pair CHECK ((data_document IS NULL) = (data_document_format IS NULL)),
  CONSTRAINT ck_execution_event_data_document_format CHECK (data_document_format IN ('orcon/event-data@1')),
  CONSTRAINT fk_execution_event_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_event_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_event_root_execution FOREIGN KEY (org_id, root_execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports fk_execution_event_root_execution and the delete of its parent row.
CREATE INDEX ix_execution_event_org_id_root_execution_id ON execution_event (org_id, root_execution_id);

CREATE TABLE outbox (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  topic TEXT NOT NULL,
  payload_document BLOB NOT NULL,
  payload_document_format TEXT NOT NULL,
  lease_owner BLOB,
  lease_expires_at INTEGER,
  delivered_at INTEGER,
  attempt_count INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_outbox PRIMARY KEY (id),
  CONSTRAINT uq_outbox_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_outbox_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_outbox_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_outbox_topic_length CHECK (length(topic) BETWEEN 1 AND 200),
  CONSTRAINT ck_outbox_payload_document_format CHECK (payload_document_format IN ('orcon/outbox-payload@1')),
  CONSTRAINT ck_outbox_lease_owner_length CHECK (length(lease_owner) = 16),
  CONSTRAINT ck_outbox_attempt_count_range CHECK (attempt_count >= 0),
  CONSTRAINT fk_outbox_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

-- Supports ClaimOutbox and orcon_engine.claim_outbox: deliveries span organizations.
CREATE INDEX ix_outbox_created_at_id ON outbox (created_at, id) WHERE delivered_at IS NULL;

CREATE TABLE limit_bucket (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  bucket_key TEXT NOT NULL,
  capacity INTEGER NOT NULL,
  refill_per_second INTEGER NOT NULL,
  tokens INTEGER NOT NULL,
  refilled_at INTEGER NOT NULL,
  blocked_until_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_limit_bucket PRIMARY KEY (id),
  CONSTRAINT uq_limit_bucket_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_limit_bucket_org_id_bucket_key UNIQUE (org_id, bucket_key),
  CONSTRAINT ck_limit_bucket_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_limit_bucket_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_limit_bucket_bucket_key_length CHECK (length(bucket_key) BETWEEN 1 AND 300),
  CONSTRAINT ck_limit_bucket_capacity_range CHECK (capacity >= 0),
  CONSTRAINT ck_limit_bucket_refill_per_second_range CHECK (refill_per_second >= 0),
  CONSTRAINT ck_limit_bucket_tokens_range CHECK (tokens >= 0),
  CONSTRAINT fk_limit_bucket_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;


CREATE TABLE trigger_rate (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  window_started_at INTEGER NOT NULL,
  fire_count INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_trigger_rate PRIMARY KEY (id),
  CONSTRAINT uq_trigger_rate_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_trigger_rate_org_id_flow_id_window_started_at UNIQUE (org_id, flow_id, window_started_at),
  CONSTRAINT ck_trigger_rate_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_trigger_rate_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_trigger_rate_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_trigger_rate_fire_count_range CHECK (fire_count >= 0),
  CONSTRAINT fk_trigger_rate_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_trigger_rate_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
) STRICT;

