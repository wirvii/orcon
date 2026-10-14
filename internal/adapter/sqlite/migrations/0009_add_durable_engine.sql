-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The durable engine (SPEC-001 C4a). An execution keeps, next to its token rows, one document with the
-- completion trackers of its run (which nodes completed, how far each join got, which loop instances are
-- armed): ADR-0010 section 2 persists them where tokens can reach a join or a loop, and this implementation
-- persists them for every node so that a step needs no static analysis. The document is read and written whole
-- (orcon-db JS-1). engine_state_revision counts the commits that wrote it: a worker whose copy is older than the
-- row replays its step on the new state instead of overwriting it. NULL document: no worker has run the execution.
ALTER TABLE execution ADD COLUMN engine_state_document BLOB;
ALTER TABLE execution ADD COLUMN engine_state_document_format TEXT
  CONSTRAINT ck_execution_engine_state_document_format CHECK (engine_state_document_format IN ('orcon/engine-state@1'))
  CONSTRAINT ck_execution_engine_state_document_pair CHECK ((engine_state_document IS NULL) = (engine_state_document_format IS NULL));
ALTER TABLE execution ADD COLUMN engine_state_revision INTEGER NOT NULL DEFAULT 0
  CONSTRAINT ck_execution_engine_state_revision_range CHECK (engine_state_revision >= 0);

-- The batch a token starts with: the execution input, the combined input of a join continuation, or of a loop
-- iteration that merged its returns. A node output belongs to a node execution (node_output); these belong to no
-- node execution yet, because the token has not been activated. The id is the payload reference.
CREATE TABLE token_input (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  item_count INTEGER NOT NULL,
  encoding TEXT NOT NULL,
  inline_document BLOB NOT NULL,
  inline_document_format TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_token_input PRIMARY KEY (id),
  CONSTRAINT uq_token_input_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_token_input_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_token_input_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_token_input_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_token_input_item_count_range CHECK (item_count >= 0),
  CONSTRAINT ck_token_input_encoding_length CHECK (length(encoding) BETWEEN 1 AND 64),
  CONSTRAINT ck_token_input_inline_document_format CHECK (inline_document_format IN ('orcon/items@1')),
  CONSTRAINT fk_token_input_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_token_input_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports fk_token_input_execution and the delete of its parent row.
CREATE INDEX ix_token_input_org_id_execution_id ON token_input (org_id, execution_id);

-- Supports ListLiveTokens (the tokens of one execution that are not terminal, which a worker reads to open the
-- execution) and RefreshExecutionCounters (their counts per state).
CREATE INDEX ix_token_org_id_execution_id_id ON token (org_id, execution_id, id)
  WHERE status IN ('RUNNABLE', 'LEASED', 'WAITING', 'DEFERRED', 'SUSPENDED');
