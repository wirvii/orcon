-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- Visual flow editor storage (SPEC-014 section 6, ADR-0082). Node test outputs,
-- pins, test runs, seeds, effect previews and editing sessions are organization
-- scoped. The PostgreSQL file of this pair carries the same tables and the
-- orcon_engine pruning function; SQLite performs the same pruning in the adapter.

CREATE TABLE node_test_data (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  -- Identifier of the node inside the flow document; not a foreign key, like token.node_id.
  node_id TEXT NOT NULL,
  source_kind TEXT NOT NULL,
  -- Exactly one of output_document and blob_id is set; the document is inline up to 256 KiB (JS-6).
  output_document BLOB,
  output_document_format TEXT,
  blob_id BLOB,
  item_count INTEGER NOT NULL,
  size_bytes INTEGER NOT NULL,
  content_sha256 BLOB NOT NULL,
  has_masked_values INTEGER NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  created_by_principal_id BLOB,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_node_test_data PRIMARY KEY (id),
  CONSTRAINT uq_node_test_data_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_node_test_data_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_node_test_data_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_node_test_data_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_node_test_data_node_id_length CHECK (length(node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_node_test_data_source_kind CHECK (source_kind IN ('MANUAL', 'EXECUTION', 'TEST_RUN', 'N8N_PIN_DATA', 'MOCK', 'PRIOR_TEST_OUTPUT')),
  CONSTRAINT ck_node_test_data_output_document_pair CHECK ((output_document IS NULL) = (output_document_format IS NULL)),
  CONSTRAINT ck_node_test_data_output_document_format CHECK (output_document_format IN ('orcon/node-test-output@1')),
  CONSTRAINT ck_node_test_data_one_payload CHECK ((output_document IS NULL) <> (blob_id IS NULL)),
  CONSTRAINT ck_node_test_data_blob_id_length CHECK (blob_id IS NULL OR length(blob_id) = 16),
  CONSTRAINT ck_node_test_data_item_count_range CHECK (item_count >= 0),
  CONSTRAINT ck_node_test_data_size_bytes_range CHECK (size_bytes >= 0),
  CONSTRAINT ck_node_test_data_content_sha256_length CHECK (length(content_sha256) = 32),
  CONSTRAINT ck_node_test_data_has_masked_values CHECK (has_masked_values IN (0, 1)),
  CONSTRAINT fk_node_test_data_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_node_test_data_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_node_test_data_blob FOREIGN KEY (org_id, blob_id) REFERENCES blob (org_id, id) ON DELETE RESTRICT,
  CONSTRAINT ck_node_test_data_created_by_principal_id_length CHECK (length(created_by_principal_id) = 16),
  CONSTRAINT fk_node_test_data_created_by_principal FOREIGN KEY (created_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports ListNodeTestDataForNode (latest first, bounded) and fk_node_test_data_flow.
CREATE INDEX ix_node_test_data_org_id_flow_id_node_id_created_at_id ON node_test_data (org_id, flow_id, node_id, created_at, id);

-- Supports fk_node_test_data_blob and the delete of its parent blob.
CREATE INDEX ix_node_test_data_org_id_blob_id ON node_test_data (org_id, blob_id) WHERE blob_id IS NOT NULL;

-- Supports fk_node_test_data_created_by_principal and the batched clearing of the column (IN-12).
CREATE INDEX ix_node_test_data_created_by_principal_id ON node_test_data (created_by_principal_id) WHERE created_by_principal_id IS NOT NULL;

CREATE TABLE flow_node_pin (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  node_id TEXT NOT NULL,
  node_test_data_id BLOB NOT NULL,
  revision INTEGER NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  pinned_by_principal_id BLOB,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_flow_node_pin PRIMARY KEY (id),
  CONSTRAINT uq_flow_node_pin_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_node_pin_org_id_flow_id_node_id UNIQUE (org_id, flow_id, node_id),
  CONSTRAINT ck_flow_node_pin_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_node_pin_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_node_pin_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_flow_node_pin_node_test_data_id_length CHECK (length(node_test_data_id) = 16),
  CONSTRAINT ck_flow_node_pin_node_id_length CHECK (length(node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_flow_node_pin_revision_range CHECK (revision >= 1),
  CONSTRAINT fk_flow_node_pin_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_node_pin_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_node_pin_node_test_data FOREIGN KEY (org_id, node_test_data_id) REFERENCES node_test_data (org_id, id) ON DELETE CASCADE,
  CONSTRAINT ck_flow_node_pin_pinned_by_principal_id_length CHECK (length(pinned_by_principal_id) = 16),
  CONSTRAINT fk_flow_node_pin_pinned_by_principal FOREIGN KEY (pinned_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports fk_flow_node_pin_node_test_data and the delete of its parent row.
CREATE INDEX ix_flow_node_pin_org_id_node_test_data_id ON flow_node_pin (org_id, node_test_data_id);

-- Supports fk_flow_node_pin_pinned_by_principal_id and the batched clearing of the column (IN-12).
CREATE INDEX ix_flow_node_pin_pinned_by_principal_id ON flow_node_pin (pinned_by_principal_id) WHERE pinned_by_principal_id IS NOT NULL;

CREATE TABLE test_run (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  flow_snapshot_id BLOB NOT NULL,
  target_kind TEXT NOT NULL,
  target_node_id TEXT,
  effect_mode TEXT NOT NULL,
  plan_document BLOB NOT NULL,
  plan_document_format TEXT NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  started_by_principal_id BLOB,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_test_run PRIMARY KEY (id),
  CONSTRAINT uq_test_run_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_test_run_org_id_execution_id UNIQUE (org_id, execution_id),
  CONSTRAINT ck_test_run_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_test_run_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_test_run_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_test_run_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_test_run_flow_snapshot_id_length CHECK (length(flow_snapshot_id) = 16),
  CONSTRAINT ck_test_run_target_kind CHECK (target_kind IN ('FLOW', 'NODE', 'UP_TO_NODE')),
  CONSTRAINT ck_test_run_target_node_id_length CHECK (target_node_id IS NULL OR length(target_node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_test_run_target_node_id_presence CHECK ((target_kind = 'FLOW') = (target_node_id IS NULL)),
  CONSTRAINT ck_test_run_effect_mode CHECK (effect_mode IN ('CONFIRM', 'SUBSTITUTE')),
  CONSTRAINT ck_test_run_plan_document_format CHECK (plan_document_format IN ('orcon/test-plan@1')),
  CONSTRAINT fk_test_run_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_flow_snapshot FOREIGN KEY (org_id, flow_snapshot_id) REFERENCES flow_snapshot (org_id, id) ON DELETE CASCADE,
  CONSTRAINT ck_test_run_started_by_principal_id_length CHECK (length(started_by_principal_id) = 16),
  CONSTRAINT fk_test_run_started_by_principal FOREIGN KEY (started_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports the test history of a flow (ListTestRuns, keyset by created_at and id, at most 50 rows read per lookup).
CREATE INDEX ix_test_run_org_id_flow_id_created_at_id ON test_run (org_id, flow_id, created_at, id);

-- Supports fk_test_run_flow_snapshot and the delete of its parent row.
CREATE INDEX ix_test_run_org_id_flow_snapshot_id ON test_run (org_id, flow_snapshot_id);

-- Supports fk_test_run_started_by_principal and the batched clearing of the column (IN-12).
CREATE INDEX ix_test_run_started_by_principal_id ON test_run (started_by_principal_id) WHERE started_by_principal_id IS NOT NULL;

CREATE TABLE test_run_seed (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  test_run_id BLOB NOT NULL,
  node_id TEXT NOT NULL,
  seed_kind TEXT NOT NULL,
  node_test_data_id BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_test_run_seed PRIMARY KEY (id),
  CONSTRAINT uq_test_run_seed_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_test_run_seed_org_id_test_run_id_node_id UNIQUE (org_id, test_run_id, node_id),
  CONSTRAINT ck_test_run_seed_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_test_run_seed_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_test_run_seed_test_run_id_length CHECK (length(test_run_id) = 16),
  CONSTRAINT ck_test_run_seed_node_test_data_id_length CHECK (length(node_test_data_id) = 16),
  CONSTRAINT ck_test_run_seed_node_id_length CHECK (length(node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_test_run_seed_seed_kind CHECK (seed_kind IN ('PIN', 'PRIOR_TEST_OUTPUT', 'SUBSTITUTE')),
  CONSTRAINT fk_test_run_seed_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_seed_test_run FOREIGN KEY (org_id, test_run_id) REFERENCES test_run (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_seed_node_test_data FOREIGN KEY (org_id, node_test_data_id) REFERENCES node_test_data (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports fk_test_run_seed_node_test_data and the delete of its parent row.
CREATE INDEX ix_test_run_seed_org_id_node_test_data_id ON test_run_seed (org_id, node_test_data_id);

CREATE TABLE effect_preview (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  node_execution_id BLOB NOT NULL,
  node_id TEXT NOT NULL,
  status TEXT NOT NULL,
  effect TEXT NOT NULL,
  safety_level TEXT NOT NULL,
  item_count INTEGER NOT NULL,
  is_preview_available INTEGER NOT NULL,
  preview_document BLOB NOT NULL,
  preview_document_format TEXT NOT NULL,
  preview_sha256 BLOB NOT NULL,
  decision_document BLOB,
  decision_document_format TEXT,
  expires_at INTEGER NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  decided_by_principal_id BLOB,
  decided_at INTEGER,
  revision INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_effect_preview PRIMARY KEY (id),
  CONSTRAINT uq_effect_preview_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_effect_preview_org_id_node_execution_id UNIQUE (org_id, node_execution_id),
  CONSTRAINT ck_effect_preview_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_effect_preview_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_effect_preview_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_effect_preview_node_execution_id_length CHECK (length(node_execution_id) = 16),
  CONSTRAINT ck_effect_preview_node_id_length CHECK (length(node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_effect_preview_status CHECK (status IN ('OPEN', 'DISPATCH_CONFIRMED', 'SUBSTITUTED', 'EXPIRED', 'CANCELED')),
  CONSTRAINT ck_effect_preview_effect CHECK (effect IN ('WRITE', 'DELETE', 'UNKNOWN')),
  CONSTRAINT ck_effect_preview_safety_level CHECK (safety_level IN ('IDEMPOTENCY_KEY', 'RECONCILE', 'UNSAFE')),
  CONSTRAINT ck_effect_preview_item_count_range CHECK (item_count >= 0),
  CONSTRAINT ck_effect_preview_is_preview_available CHECK (is_preview_available IN (0, 1)),
  CONSTRAINT ck_effect_preview_preview_document_format CHECK (preview_document_format IN ('orcon/effect-preview@1')),
  CONSTRAINT ck_effect_preview_preview_sha256_length CHECK (length(preview_sha256) = 32),
  CONSTRAINT ck_effect_preview_decision_document_pair CHECK ((decision_document IS NULL) = (decision_document_format IS NULL)),
  CONSTRAINT ck_effect_preview_decision_document_format CHECK (decision_document_format IN ('orcon/effect-decision@1')),
  CONSTRAINT ck_effect_preview_revision_range CHECK (revision >= 1),
  CONSTRAINT fk_effect_preview_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_effect_preview_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_effect_preview_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT ck_effect_preview_decided_by_principal_id_length CHECK (length(decided_by_principal_id) = 16),
  CONSTRAINT fk_effect_preview_decided_by_principal FOREIGN KEY (decided_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports ListEffectPreviews of an execution, keyset by created_at and id.
CREATE INDEX ix_effect_preview_org_id_execution_id_created_at_id ON effect_preview (org_id, execution_id, created_at, id);

-- Supports fk_effect_preview_decided_by_principal and the batched clearing of the column (IN-12).
CREATE INDEX ix_effect_preview_decided_by_principal_id ON effect_preview (decided_by_principal_id) WHERE decided_by_principal_id IS NOT NULL;

CREATE TABLE flow_editing_session (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  opened_by_principal_id BLOB,
  client_label TEXT NOT NULL,
  last_seen_draft_revision INTEGER NOT NULL,
  opened_at INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL,
  revision INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_flow_editing_session PRIMARY KEY (id),
  CONSTRAINT uq_flow_editing_session_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_flow_editing_session_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_editing_session_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_editing_session_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_flow_editing_session_client_label_length CHECK (length(client_label) BETWEEN 1 AND 64),
  CONSTRAINT ck_flow_editing_session_last_seen_draft_revision_range CHECK (last_seen_draft_revision >= 0),
  CONSTRAINT ck_flow_editing_session_revision_range CHECK (revision >= 1),
  CONSTRAINT fk_flow_editing_session_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_editing_session_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT ck_flow_editing_session_opened_by_principal_id_length CHECK (length(opened_by_principal_id) = 16),
  CONSTRAINT fk_flow_editing_session_opened_by_principal FOREIGN KEY (opened_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports ListFlowEditingSessions (live sessions of one flow) and the presence expiry.
CREATE INDEX ix_flow_editing_session_org_id_flow_id_expires_at_id ON flow_editing_session (org_id, flow_id, expires_at, id);

-- Supports the instance-wide expiry of presence rows (orcon_engine prune of editor_presence).
CREATE INDEX ix_flow_editing_session_expires_at_id ON flow_editing_session (expires_at, id);

-- Supports fk_flow_editing_session_opened_by_principal and the batched clearing of the column (IN-12).
CREATE INDEX ix_flow_editing_session_opened_by_principal_id ON flow_editing_session (opened_by_principal_id) WHERE opened_by_principal_id IS NOT NULL;

-- Draft save times (A12): NULL until the first save after this migration.
ALTER TABLE flow ADD COLUMN draft_updated_at INTEGER;
ALTER TABLE flow ADD COLUMN draft_updated_by_principal_id BLOB
  CONSTRAINT ck_flow_draft_updated_by_principal_id_length CHECK (length(draft_updated_by_principal_id) = 16)
  CONSTRAINT fk_flow_draft_updated_by_principal REFERENCES principal (id) ON DELETE SET NULL;

-- Supports fk_flow_draft_updated_by_principal and the batched clearing of the column (IN-12).
CREATE INDEX ix_flow_draft_updated_by_principal_id ON flow (draft_updated_by_principal_id) WHERE draft_updated_by_principal_id IS NOT NULL;

-- Publication note (A7): NULL when the version has no note.
ALTER TABLE flow_version ADD COLUMN publication_note TEXT
  CONSTRAINT ck_flow_version_publication_note_length CHECK (publication_note IS NULL OR length(publication_note) <= 500);

-- Rollback origin (A7): the version number the rolled-back version was copied from.
ALTER TABLE flow_version ADD COLUMN rolled_back_from_version_number INTEGER
  CONSTRAINT ck_flow_version_rolled_back_from_version_number_range CHECK (rolled_back_from_version_number IS NULL OR rolled_back_from_version_number >= 1);
