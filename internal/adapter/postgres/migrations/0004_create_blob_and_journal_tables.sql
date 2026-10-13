-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE blob (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid,
  flow_id uuid,
  size_bytes bigint NOT NULL,
  sha256 bytea NOT NULL,
  media_type text NOT NULL,
  backend text NOT NULL,
  location text,
  -- NULL until the transaction that references the blob commits.
  committed_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_blob PRIMARY KEY (id),
  CONSTRAINT uq_blob_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_blob_size_bytes_range CHECK (size_bytes >= 0),
  CONSTRAINT ck_blob_sha256_length CHECK (octet_length(sha256) = 32),
  CONSTRAINT ck_blob_media_type_length CHECK (char_length(media_type) BETWEEN 1 AND 255),
  CONSTRAINT ck_blob_backend CHECK (backend IN ('FILESYSTEM', 'POSTGRES', 'S3')),
  CONSTRAINT ck_blob_owner CHECK ((execution_id IS NULL) <> (flow_id IS NULL)),
  CONSTRAINT fk_blob_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_blob_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_blob_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
);

-- Supports the garbage collection of blobs never committed (ADR-0016 section 4).
CREATE INDEX ix_blob_created_at ON blob (created_at) WHERE committed_at IS NULL;

-- Supports fk_blob_execution and the delete of its parent row.
CREATE INDEX ix_blob_org_id_execution_id ON blob (org_id, execution_id) WHERE execution_id IS NOT NULL;

-- Supports fk_blob_flow and the delete of its parent row.
CREATE INDEX ix_blob_org_id_flow_id ON blob (org_id, flow_id) WHERE flow_id IS NOT NULL;

ALTER TABLE blob ENABLE ROW LEVEL SECURITY;
ALTER TABLE blob FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON blob FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON blob TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON blob TO orcon_engine;

CREATE TABLE blob_chunk (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  blob_id uuid NOT NULL,
  chunk_index bigint NOT NULL,
  data bytea NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_blob_chunk PRIMARY KEY (id),
  CONSTRAINT uq_blob_chunk_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_blob_chunk_org_id_blob_id_chunk_index UNIQUE (org_id, blob_id, chunk_index),
  CONSTRAINT ck_blob_chunk_chunk_index_range CHECK (chunk_index >= 0),
  CONSTRAINT ck_blob_chunk_data_length CHECK (octet_length(data) <= 1048576),
  CONSTRAINT fk_blob_chunk_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_blob_chunk_blob FOREIGN KEY (org_id, blob_id) REFERENCES blob (org_id, id) ON DELETE CASCADE
);

ALTER TABLE blob_chunk ENABLE ROW LEVEL SECURITY;
ALTER TABLE blob_chunk FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON blob_chunk FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON blob_chunk TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON blob_chunk TO orcon_engine;

CREATE TABLE node_output (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  node_execution_id uuid NOT NULL,
  port text NOT NULL,
  item_count bigint NOT NULL,
  encoding text NOT NULL,
  inline_document bytea,
  inline_document_format text,
  blob_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_node_output PRIMARY KEY (id),
  CONSTRAINT uq_node_output_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_node_output_org_id_node_execution_id_port UNIQUE (org_id, node_execution_id, port),
  CONSTRAINT ck_node_output_port_length CHECK (char_length(port) BETWEEN 1 AND 200),
  CONSTRAINT ck_node_output_item_count_range CHECK (item_count >= 0),
  CONSTRAINT ck_node_output_encoding_length CHECK (char_length(encoding) BETWEEN 1 AND 64),
  CONSTRAINT ck_node_output_inline_document_pair CHECK ((inline_document IS NULL) = (inline_document_format IS NULL)),
  CONSTRAINT ck_node_output_inline_document_format CHECK (inline_document_format IN ('orcon/items@1')),
  CONSTRAINT ck_node_output_location CHECK ((inline_document IS NULL) <> (blob_id IS NULL)),
  CONSTRAINT fk_node_output_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_node_output_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_node_output_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_node_output_blob FOREIGN KEY (org_id, blob_id) REFERENCES blob (org_id, id) ON DELETE CASCADE
);

-- Supports fk_node_output_execution and the delete of its parent row.
CREATE INDEX ix_node_output_org_id_execution_id ON node_output (org_id, execution_id);

-- Supports fk_node_output_blob and the delete of its parent row.
CREATE INDEX ix_node_output_org_id_blob_id ON node_output (org_id, blob_id) WHERE blob_id IS NOT NULL;

ALTER TABLE node_output ENABLE ROW LEVEL SECURITY;
ALTER TABLE node_output FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON node_output FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON node_output TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON node_output TO orcon_engine;

CREATE TABLE external_operation (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  node_execution_id uuid NOT NULL,
  item_index bigint NOT NULL,
  correlation_key text NOT NULL,
  contract_reference text NOT NULL,
  state text NOT NULL,
  batch_key text,
  request_reference text,
  response_reference text,
  dispatch_deadline_at timestamptz,
  first_dispatched_at timestamptz,
  retry_epoch bigint NOT NULL,
  compensation_state text NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  resolved_by_principal_id uuid,
  evidence_reference text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_external_operation PRIMARY KEY (id),
  CONSTRAINT uq_external_operation_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_external_operation_org_id_correlation_key UNIQUE (org_id, correlation_key),
  CONSTRAINT ck_external_operation_item_index_range CHECK (item_index >= 0),
  CONSTRAINT ck_external_operation_correlation_key_length CHECK (char_length(correlation_key) BETWEEN 1 AND 64),
  CONSTRAINT ck_external_operation_state CHECK (state IN ('PREPARED', 'DISPATCHING', 'VERIFYING', 'SUCCEEDED', 'FAILED', 'UNCERTAIN', 'NOT_DISPATCHED', 'RESOLVED_SUCCEEDED', 'RESOLVED_FAILED')),
  CONSTRAINT ck_external_operation_retry_epoch_range CHECK (retry_epoch >= 0),
  CONSTRAINT ck_external_operation_compensation_state CHECK (compensation_state IN ('NONE', 'PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED', 'UNCERTAIN', 'SKIPPED')),
  CONSTRAINT fk_external_operation_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_external_operation_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_external_operation_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_external_operation_resolved_by_principal FOREIGN KEY (resolved_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports ListExternalOperations (the journal of a node execution, keyset by item) and the node execution foreign key.
CREATE INDEX ix_external_operation_org_id_node_execution_id_item_index_id ON external_operation (org_id, node_execution_id, item_index, id);

-- Supports the recovery of operations left in DISPATCHING (ADR-0012 section 6).
CREATE INDEX ix_external_operation_dispatch_deadline_at ON external_operation (dispatch_deadline_at) WHERE state = 'DISPATCHING';

-- Supports fk_external_operation_execution and the delete of its parent row.
CREATE INDEX ix_external_operation_org_id_execution_id ON external_operation (org_id, execution_id);

-- Supports fk_external_operation_resolved_by_principal and the delete of its parent row.
CREATE INDEX ix_external_operation_resolved_by_principal_id ON external_operation (resolved_by_principal_id) WHERE resolved_by_principal_id IS NOT NULL;

ALTER TABLE external_operation ENABLE ROW LEVEL SECURITY;
ALTER TABLE external_operation FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON external_operation FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON external_operation TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON external_operation TO orcon_engine;
