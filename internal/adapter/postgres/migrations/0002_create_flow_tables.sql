-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE project (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  name text NOT NULL,
  slug text NOT NULL,
  environment text NOT NULL,
  is_protected boolean NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_project PRIMARY KEY (id),
  CONSTRAINT uq_project_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_project_org_id_slug UNIQUE (org_id, slug),
  CONSTRAINT ck_project_name_length CHECK (char_length(name) BETWEEN 1 AND 200),
  CONSTRAINT ck_project_slug_length CHECK (char_length(slug) BETWEEN 1 AND 100),
  CONSTRAINT ck_project_environment_length CHECK (char_length(environment) BETWEEN 1 AND 100),
  CONSTRAINT fk_project_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

ALTER TABLE project ENABLE ROW LEVEL SECURITY;
ALTER TABLE project FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON project FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON project TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON project TO orcon_engine;

CREATE TABLE flow (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  project_id uuid NOT NULL,
  name text NOT NULL,
  draft_document bytea NOT NULL,
  draft_document_format text NOT NULL,
  draft_revision bigint NOT NULL,
  -- NULL until the first version is published.
  active_flow_version_id uuid,
  -- NULL while the flow is active; set by a soft delete (ADR-0077 section 4).
  deleted_at timestamptz,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  deleted_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_flow PRIMARY KEY (id),
  CONSTRAINT uq_flow_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_flow_name_length CHECK (char_length(name) BETWEEN 1 AND 200),
  CONSTRAINT ck_flow_draft_document_format CHECK (draft_document_format IN ('orcon/flow@1')),
  CONSTRAINT ck_flow_draft_revision_range CHECK (draft_revision >= 0),
  CONSTRAINT fk_flow_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_deleted_by_principal FOREIGN KEY (deleted_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports ListFlows (keyset by id within a project) and the project foreign key.
CREATE INDEX ix_flow_org_id_project_id_id ON flow (org_id, project_id, id);

-- A flow name is unique per project among non-deleted flows (ADR-0077 section 4); supports GetFlowByName.
CREATE UNIQUE INDEX uq_flow_org_id_project_id_name ON flow (org_id, project_id, name) WHERE deleted_at IS NULL;

-- Supports fk_flow_active_flow_version and the delete of its parent row.
CREATE INDEX ix_flow_org_id_active_flow_version_id ON flow (org_id, active_flow_version_id) WHERE active_flow_version_id IS NOT NULL;

-- Supports fk_flow_deleted_by_principal and the delete of its parent row.
CREATE INDEX ix_flow_deleted_by_principal_id ON flow (deleted_by_principal_id) WHERE deleted_by_principal_id IS NOT NULL;

ALTER TABLE flow ENABLE ROW LEVEL SECURITY;
ALTER TABLE flow FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON flow FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON flow TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON flow TO orcon_engine;

CREATE TABLE flow_version (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  version_number bigint NOT NULL,
  document bytea NOT NULL,
  document_format text NOT NULL,
  content_hash text NOT NULL,
  warning_document bytea,
  warning_document_format text,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  published_by_principal_id uuid,
  published_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_flow_version PRIMARY KEY (id),
  CONSTRAINT uq_flow_version_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_version_org_id_flow_id_version_number UNIQUE (org_id, flow_id, version_number),
  CONSTRAINT ck_flow_version_version_number_range CHECK (version_number >= 1),
  CONSTRAINT ck_flow_version_document_format CHECK (document_format IN ('orcon/flow@1')),
  CONSTRAINT ck_flow_version_content_hash_length CHECK (char_length(content_hash) BETWEEN 1 AND 128),
  CONSTRAINT ck_flow_version_warning_document_pair CHECK ((warning_document IS NULL) = (warning_document_format IS NULL)),
  CONSTRAINT ck_flow_version_warning_document_format CHECK (warning_document_format IN ('orcon/flow-warnings@1')),
  CONSTRAINT fk_flow_version_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_version_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_version_published_by_principal FOREIGN KEY (published_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports ListFlowVersions: keyset order (version_number, id) without a sort.
CREATE INDEX ix_flow_version_org_id_flow_id_version_number_id ON flow_version (org_id, flow_id, version_number, id);

-- Supports fk_flow_version_published_by_principal and the delete of its parent row.
CREATE INDEX ix_flow_version_published_by_principal_id ON flow_version (published_by_principal_id) WHERE published_by_principal_id IS NOT NULL;

ALTER TABLE flow_version ENABLE ROW LEVEL SECURITY;
ALTER TABLE flow_version FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON flow_version FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT ON flow_version TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON flow_version TO orcon_engine;

CREATE TABLE flow_snapshot (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  content_hash text NOT NULL,
  document bytea NOT NULL,
  document_format text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_flow_snapshot PRIMARY KEY (id),
  CONSTRAINT uq_flow_snapshot_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_snapshot_org_id_content_hash UNIQUE (org_id, content_hash),
  CONSTRAINT ck_flow_snapshot_content_hash_length CHECK (char_length(content_hash) BETWEEN 1 AND 128),
  CONSTRAINT ck_flow_snapshot_document_format CHECK (document_format IN ('orcon/flow@1')),
  CONSTRAINT fk_flow_snapshot_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_snapshot_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
);

-- Supports fk_flow_snapshot_flow and the delete of its parent row.
CREATE INDEX ix_flow_snapshot_org_id_flow_id ON flow_snapshot (org_id, flow_id);

ALTER TABLE flow_snapshot ENABLE ROW LEVEL SECURITY;
ALTER TABLE flow_snapshot FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON flow_snapshot FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT ON flow_snapshot TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON flow_snapshot TO orcon_engine;

ALTER TABLE flow ADD CONSTRAINT fk_flow_active_flow_version FOREIGN KEY (org_id, active_flow_version_id) REFERENCES flow_version (org_id, id) ON DELETE RESTRICT;
