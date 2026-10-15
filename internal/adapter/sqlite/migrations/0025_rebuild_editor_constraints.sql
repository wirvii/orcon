-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind rebuild
-- orcon:compatibility-floor self

-- SQLite cannot alter a CHECK or a UNIQUE constraint in place, so both tables are
-- rebuilt with the twelve-step procedure (MG-8). The runner turns foreign keys off
-- for this transaction and checks them before commit. Expected duration: linear in
-- the rows of flow_data and wait, both small at this stage (flow_data holds flow
-- variables; wait holds open waits). Locking: the single writer for the duration.
-- The PostgreSQL file of this pair alters the same constraints in place.

-- flow_data gains the namespace (ADR-0008 section 4.4): PRODUCTION for production
-- executions, TEST for test runs. Existing rows become PRODUCTION.
CREATE TABLE new_flow_data (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  namespace TEXT NOT NULL DEFAULT 'PRODUCTION',
  data_key TEXT NOT NULL,
  value_document BLOB NOT NULL,
  value_document_format TEXT NOT NULL,
  version INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_flow_data PRIMARY KEY (id),
  CONSTRAINT uq_flow_data_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_data_org_id_flow_id_namespace_data_key UNIQUE (org_id, flow_id, namespace, data_key),
  CONSTRAINT ck_flow_data_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_data_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_data_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_flow_data_namespace CHECK (namespace IN ('PRODUCTION', 'TEST')),
  CONSTRAINT ck_flow_data_data_key_length CHECK (length(data_key) BETWEEN 1 AND 200),
  CONSTRAINT ck_flow_data_value_document_format CHECK (value_document_format IN ('orcon/value@1')),
  CONSTRAINT ck_flow_data_version_range CHECK (version >= 0),
  CONSTRAINT fk_flow_data_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_data_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
) STRICT;

INSERT INTO new_flow_data (id, org_id, flow_id, namespace, data_key, value_document, value_document_format, version, created_at, updated_at)
SELECT id, org_id, flow_id, 'PRODUCTION', data_key, value_document, value_document_format, version, created_at, updated_at
FROM flow_data;

DROP TABLE flow_data;
ALTER TABLE new_flow_data RENAME TO flow_data;

-- wait accepts EFFECT_CONFIRMATION (SPEC-014 section 15, ADR-0010 section 8): a test
-- execution waits for a person's decision before an external effect is dispatched.
CREATE TABLE new_wait (
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
  CONSTRAINT ck_wait_kind CHECK (kind IN ('TIMER', 'SIGNAL', 'HUMAN', 'CHILD', 'EFFECT_CONFIRMATION')),
  CONSTRAINT ck_wait_secret_hash_length CHECK (length(secret_hash) = 32),
  CONSTRAINT ck_wait_status CHECK (status IN ('OPEN', 'RESOLVED', 'EXPIRED', 'CANCELED')),
  CONSTRAINT fk_wait_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_wait_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_wait_token FOREIGN KEY (org_id, token_id) REFERENCES token (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_wait_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE
) STRICT;

INSERT INTO new_wait (id, org_id, execution_id, token_id, node_execution_id, kind, wait_group_key, secret_hash, expires_at, status, resume_payload_reference, created_at, updated_at)
SELECT id, org_id, execution_id, token_id, node_execution_id, kind, wait_group_key, secret_hash, expires_at, status, resume_payload_reference, created_at, updated_at
FROM wait;

DROP TABLE wait;
ALTER TABLE new_wait RENAME TO wait;

-- The indexes of wait were dropped with the old table; they come back under their names.
CREATE INDEX ix_wait_org_id_wait_group_key ON wait (org_id, wait_group_key) WHERE wait_group_key IS NOT NULL;
CREATE INDEX ix_wait_expires_at_id ON wait (expires_at, id) WHERE status = 'OPEN' AND expires_at IS NOT NULL;
CREATE INDEX ix_wait_org_id_execution_id ON wait (org_id, execution_id);
CREATE INDEX ix_wait_org_id_token_id ON wait (org_id, token_id);
CREATE INDEX ix_wait_org_id_node_execution_id ON wait (org_id, node_execution_id);
