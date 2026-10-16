-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- Visual flow editor storage (SPEC-014 section 6, ADR-0082). Node test outputs,
-- pins, test runs, seeds, effect previews and editing sessions are organization
-- scoped. The orcon_engine function prune_batch removes unreferenced editor test
-- data (24 hours after creation) and expired editing sessions; the SQLite file of
-- this pair performs the same work in the adapter.

CREATE TABLE node_test_data (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  -- Identifier of the node inside the flow document; not a foreign key, like token.node_id.
  node_id text NOT NULL,
  source_kind text NOT NULL,
  -- Exactly one of output_document and blob_id is set; the document is inline up to 256 KiB (JS-6).
  output_document bytea,
  output_document_format text,
  blob_id uuid,
  item_count bigint NOT NULL,
  size_bytes bigint NOT NULL,
  content_sha256 bytea NOT NULL,
  has_masked_values boolean NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  created_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_node_test_data PRIMARY KEY (id),
  CONSTRAINT uq_node_test_data_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_node_test_data_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_node_test_data_source_kind CHECK (source_kind IN ('MANUAL', 'EXECUTION', 'TEST_RUN', 'N8N_PIN_DATA', 'MOCK', 'PRIOR_TEST_OUTPUT')),
  CONSTRAINT ck_node_test_data_output_document_pair CHECK ((output_document IS NULL) = (output_document_format IS NULL)),
  CONSTRAINT ck_node_test_data_output_document_format CHECK (output_document_format IN ('orcon/node-test-output@1')),
  CONSTRAINT ck_node_test_data_one_payload CHECK ((output_document IS NULL) <> (blob_id IS NULL)),
  CONSTRAINT ck_node_test_data_item_count_range CHECK (item_count >= 0),
  CONSTRAINT ck_node_test_data_size_bytes_range CHECK (size_bytes >= 0),
  CONSTRAINT ck_node_test_data_content_sha256_length CHECK (octet_length(content_sha256) = 32),
  CONSTRAINT fk_node_test_data_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_node_test_data_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_node_test_data_blob FOREIGN KEY (org_id, blob_id) REFERENCES blob (org_id, id) ON DELETE RESTRICT,
  CONSTRAINT fk_node_test_data_created_by_principal FOREIGN KEY (created_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports ListNodeTestDataForNode (latest first, bounded) and fk_node_test_data_flow.
CREATE INDEX ix_node_test_data_org_id_flow_id_node_id_created_at_id ON node_test_data (org_id, flow_id, node_id, created_at, id);

-- Supports fk_node_test_data_blob and the delete of its parent blob.
CREATE INDEX ix_node_test_data_org_id_blob_id ON node_test_data (org_id, blob_id) WHERE blob_id IS NOT NULL;

-- Supports fk_node_test_data_created_by_principal and the batched clearing of the column (IN-12).
CREATE INDEX ix_node_test_data_created_by_principal_id ON node_test_data (created_by_principal_id) WHERE created_by_principal_id IS NOT NULL;

CREATE TABLE flow_node_pin (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  node_id text NOT NULL,
  node_test_data_id uuid NOT NULL,
  revision bigint NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  pinned_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_flow_node_pin PRIMARY KEY (id),
  CONSTRAINT uq_flow_node_pin_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_node_pin_org_id_flow_id_node_id UNIQUE (org_id, flow_id, node_id),
  CONSTRAINT ck_flow_node_pin_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_flow_node_pin_revision_range CHECK (revision >= 1),
  CONSTRAINT fk_flow_node_pin_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_node_pin_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_node_pin_node_test_data FOREIGN KEY (org_id, node_test_data_id) REFERENCES node_test_data (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_node_pin_pinned_by_principal FOREIGN KEY (pinned_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports fk_flow_node_pin_node_test_data and the delete of its parent row.
CREATE INDEX ix_flow_node_pin_org_id_node_test_data_id ON flow_node_pin (org_id, node_test_data_id);

-- Supports fk_flow_node_pin_pinned_by_principal_id and the batched clearing of the column (IN-12).
CREATE INDEX ix_flow_node_pin_pinned_by_principal_id ON flow_node_pin (pinned_by_principal_id) WHERE pinned_by_principal_id IS NOT NULL;

CREATE TABLE test_run (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  flow_snapshot_id uuid NOT NULL,
  target_kind text NOT NULL,
  target_node_id text,
  effect_mode text NOT NULL,
  plan_document bytea NOT NULL,
  plan_document_format text NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  started_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_test_run PRIMARY KEY (id),
  CONSTRAINT uq_test_run_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_test_run_org_id_execution_id UNIQUE (org_id, execution_id),
  CONSTRAINT ck_test_run_target_kind CHECK (target_kind IN ('FLOW', 'NODE', 'UP_TO_NODE')),
  CONSTRAINT ck_test_run_target_node_id_length CHECK (target_node_id IS NULL OR char_length(target_node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_test_run_target_node_id_presence CHECK ((target_kind = 'FLOW') = (target_node_id IS NULL)),
  CONSTRAINT ck_test_run_effect_mode CHECK (effect_mode IN ('CONFIRM', 'SUBSTITUTE')),
  CONSTRAINT ck_test_run_plan_document_format CHECK (plan_document_format IN ('orcon/test-plan@1')),
  CONSTRAINT fk_test_run_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_flow_snapshot FOREIGN KEY (org_id, flow_snapshot_id) REFERENCES flow_snapshot (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_started_by_principal FOREIGN KEY (started_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports the test history of a flow (ListTestRuns, keyset by created_at and id, at most 50 rows read per lookup).
CREATE INDEX ix_test_run_org_id_flow_id_created_at_id ON test_run (org_id, flow_id, created_at, id);

-- Supports fk_test_run_flow_snapshot and the delete of its parent row.
CREATE INDEX ix_test_run_org_id_flow_snapshot_id ON test_run (org_id, flow_snapshot_id);

-- Supports fk_test_run_started_by_principal and the batched clearing of the column (IN-12).
CREATE INDEX ix_test_run_started_by_principal_id ON test_run (started_by_principal_id) WHERE started_by_principal_id IS NOT NULL;

CREATE TABLE test_run_seed (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  test_run_id uuid NOT NULL,
  node_id text NOT NULL,
  seed_kind text NOT NULL,
  node_test_data_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_test_run_seed PRIMARY KEY (id),
  CONSTRAINT uq_test_run_seed_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_test_run_seed_org_id_test_run_id_node_id UNIQUE (org_id, test_run_id, node_id),
  CONSTRAINT ck_test_run_seed_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_test_run_seed_seed_kind CHECK (seed_kind IN ('PIN', 'PRIOR_TEST_OUTPUT', 'SUBSTITUTE')),
  CONSTRAINT fk_test_run_seed_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_seed_test_run FOREIGN KEY (org_id, test_run_id) REFERENCES test_run (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_test_run_seed_node_test_data FOREIGN KEY (org_id, node_test_data_id) REFERENCES node_test_data (org_id, id) ON DELETE CASCADE
);

-- Supports fk_test_run_seed_node_test_data and the delete of its parent row.
CREATE INDEX ix_test_run_seed_org_id_node_test_data_id ON test_run_seed (org_id, node_test_data_id);

CREATE TABLE effect_preview (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  node_execution_id uuid NOT NULL,
  node_id text NOT NULL,
  status text NOT NULL,
  effect text NOT NULL,
  safety_level text NOT NULL,
  item_count bigint NOT NULL,
  is_preview_available boolean NOT NULL,
  preview_document bytea NOT NULL,
  preview_document_format text NOT NULL,
  preview_sha256 bytea NOT NULL,
  decision_document bytea,
  decision_document_format text,
  expires_at timestamptz NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  decided_by_principal_id uuid,
  decided_at timestamptz,
  revision bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_effect_preview PRIMARY KEY (id),
  CONSTRAINT uq_effect_preview_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_effect_preview_org_id_node_execution_id UNIQUE (org_id, node_execution_id),
  CONSTRAINT ck_effect_preview_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_effect_preview_status CHECK (status IN ('OPEN', 'DISPATCH_CONFIRMED', 'SUBSTITUTED', 'EXPIRED', 'CANCELED')),
  CONSTRAINT ck_effect_preview_effect CHECK (effect IN ('WRITE', 'DELETE', 'UNKNOWN')),
  CONSTRAINT ck_effect_preview_safety_level CHECK (safety_level IN ('IDEMPOTENCY_KEY', 'RECONCILE', 'UNSAFE')),
  CONSTRAINT ck_effect_preview_item_count_range CHECK (item_count >= 0),
  CONSTRAINT ck_effect_preview_preview_document_format CHECK (preview_document_format IN ('orcon/effect-preview@1')),
  CONSTRAINT ck_effect_preview_preview_sha256_length CHECK (octet_length(preview_sha256) = 32),
  CONSTRAINT ck_effect_preview_decision_document_pair CHECK ((decision_document IS NULL) = (decision_document_format IS NULL)),
  CONSTRAINT ck_effect_preview_decision_document_format CHECK (decision_document_format IN ('orcon/effect-decision@1')),
  CONSTRAINT ck_effect_preview_revision_range CHECK (revision >= 1),
  CONSTRAINT fk_effect_preview_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_effect_preview_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_effect_preview_node_execution FOREIGN KEY (org_id, node_execution_id) REFERENCES node_execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_effect_preview_decided_by_principal FOREIGN KEY (decided_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports ListEffectPreviews of an execution, keyset by created_at and id.
CREATE INDEX ix_effect_preview_org_id_execution_id_created_at_id ON effect_preview (org_id, execution_id, created_at, id);

-- Supports fk_effect_preview_decided_by_principal and the batched clearing of the column (IN-12).
CREATE INDEX ix_effect_preview_decided_by_principal_id ON effect_preview (decided_by_principal_id) WHERE decided_by_principal_id IS NOT NULL;

CREATE TABLE flow_editing_session (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  opened_by_principal_id uuid,
  client_label text NOT NULL,
  last_seen_draft_revision bigint NOT NULL,
  opened_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  revision bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_flow_editing_session PRIMARY KEY (id),
  CONSTRAINT uq_flow_editing_session_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_flow_editing_session_client_label_length CHECK (char_length(client_label) BETWEEN 1 AND 64),
  CONSTRAINT ck_flow_editing_session_last_seen_draft_revision_range CHECK (last_seen_draft_revision >= 0),
  CONSTRAINT ck_flow_editing_session_revision_range CHECK (revision >= 1),
  CONSTRAINT fk_flow_editing_session_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_editing_session_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_editing_session_opened_by_principal FOREIGN KEY (opened_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports ListFlowEditingSessions (live sessions of one flow) and the presence expiry.
CREATE INDEX ix_flow_editing_session_org_id_flow_id_expires_at_id ON flow_editing_session (org_id, flow_id, expires_at, id);

-- Supports the instance-wide expiry of presence rows (orcon_engine prune of editor_presence).
CREATE INDEX ix_flow_editing_session_expires_at_id ON flow_editing_session (expires_at, id);

-- Supports fk_flow_editing_session_opened_by_principal and the batched clearing of the column (IN-12).
CREATE INDEX ix_flow_editing_session_opened_by_principal_id ON flow_editing_session (opened_by_principal_id) WHERE opened_by_principal_id IS NOT NULL;

-- Draft save times (A12): NULL until the first save after this migration.
ALTER TABLE flow ADD COLUMN draft_updated_at timestamptz;
ALTER TABLE flow ADD COLUMN draft_updated_by_principal_id uuid;
ALTER TABLE flow ADD CONSTRAINT fk_flow_draft_updated_by_principal FOREIGN KEY (draft_updated_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL;

-- Supports fk_flow_draft_updated_by_principal and the batched clearing of the column (IN-12).
CREATE INDEX ix_flow_draft_updated_by_principal_id ON flow (draft_updated_by_principal_id) WHERE draft_updated_by_principal_id IS NOT NULL;

-- Row-level security on every organization-scoped table (TI-4, MG-13). The
-- policy text is exact; do not reword it.
ALTER TABLE node_test_data ENABLE ROW LEVEL SECURITY;
ALTER TABLE node_test_data FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON node_test_data FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));

ALTER TABLE flow_node_pin ENABLE ROW LEVEL SECURITY;
ALTER TABLE flow_node_pin FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON flow_node_pin FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));

ALTER TABLE test_run ENABLE ROW LEVEL SECURITY;
ALTER TABLE test_run FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON test_run FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));

ALTER TABLE test_run_seed ENABLE ROW LEVEL SECURITY;
ALTER TABLE test_run_seed FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON test_run_seed FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));

ALTER TABLE effect_preview ENABLE ROW LEVEL SECURITY;
ALTER TABLE effect_preview FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON effect_preview FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));

ALTER TABLE flow_editing_session ENABLE ROW LEVEL SECURITY;
ALTER TABLE flow_editing_session FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON flow_editing_session FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));

-- Runtime role grants (TI-5). Append-only tables grant SELECT, INSERT only to the
-- application; retention (orcon_engine) may delete unreferenced rows.
GRANT SELECT, INSERT ON node_test_data TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON node_test_data TO orcon_engine;
GRANT SELECT, INSERT, UPDATE, DELETE ON flow_node_pin TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON flow_node_pin TO orcon_engine;
GRANT SELECT, INSERT ON test_run TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON test_run TO orcon_engine;
GRANT SELECT, INSERT ON test_run_seed TO orcon_application;
GRANT SELECT, INSERT, DELETE ON test_run_seed TO orcon_engine;
GRANT SELECT, INSERT, UPDATE, DELETE ON effect_preview TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON effect_preview TO orcon_engine;
GRANT SELECT, INSERT, UPDATE, DELETE ON flow_editing_session TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON flow_editing_session TO orcon_engine;

-- Editor retention (ADR-0017 amendment, SPEC-014 section 6). Classes:
--   editor_test_data: node_test_data rows that no flow_node_pin and no test_run_seed
--     references, created more than 24 hours before p_now;
--   editor_presence: flow_editing_session rows whose expires_at is before p_now.
-- Each call deletes at most p_limit rows and returns how many it deleted. Any other
-- class is refused. Cross-organization work runs here, as orcon_engine (ADR-0047).
CREATE FUNCTION orcon_engine.prune_batch(p_class text, p_limit integer, p_now timestamptz)
RETURNS bigint
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
DECLARE
  removed bigint;
BEGIN
  IF p_class = 'editor_test_data' THEN
    WITH doomed AS (
      SELECT n.org_id, n.id
      FROM node_test_data n
      WHERE n.created_at < p_now - interval '24 hours'
        AND NOT EXISTS (SELECT 1 FROM flow_node_pin p WHERE p.org_id = n.org_id AND p.node_test_data_id = n.id)
        AND NOT EXISTS (SELECT 1 FROM test_run_seed s WHERE s.org_id = n.org_id AND s.node_test_data_id = n.id)
      ORDER BY n.created_at, n.id
      LIMIT p_limit
    ),
    deleted AS (
      DELETE FROM node_test_data AS d
      USING doomed
      WHERE d.org_id = doomed.org_id AND d.id = doomed.id
      RETURNING d.id
    )
    SELECT count(*) INTO removed FROM deleted;
  ELSIF p_class = 'editor_presence' THEN
    WITH doomed AS (
      SELECT f.org_id, f.id
      FROM flow_editing_session f
      WHERE f.expires_at < p_now
      ORDER BY f.expires_at, f.id
      LIMIT p_limit
      FOR UPDATE SKIP LOCKED
    ),
    deleted AS (
      DELETE FROM flow_editing_session AS f
      USING doomed
      WHERE f.org_id = doomed.org_id AND f.id = doomed.id
      RETURNING f.id
    )
    SELECT count(*) INTO removed FROM deleted;
  ELSE
    RAISE EXCEPTION 'unknown retention class %', p_class USING ERRCODE = 'invalid_parameter_value';
  END IF;
  RETURN removed;
END;
$$;

-- Ownership and grants of the engine function, as migration 0008 does for its functions (ADR-0047, TI-9):
-- the function runs as orcon_engine, which the row-level security policies let past the organization
-- context; PUBLIC gets nothing; the application may call it.
ALTER FUNCTION orcon_engine.prune_batch(text, integer, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.prune_batch(text, integer, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.prune_batch(text, integer, timestamptz) TO orcon_application;
