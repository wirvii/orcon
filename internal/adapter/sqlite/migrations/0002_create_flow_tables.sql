-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE project (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  -- A free label; NULL when the project has none.
  environment TEXT,
  is_protected INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_project PRIMARY KEY (id),
  CONSTRAINT uq_project_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_project_org_id_slug UNIQUE (org_id, slug),
  CONSTRAINT ck_project_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_project_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_project_name_length CHECK (length(name) BETWEEN 1 AND 200),
  CONSTRAINT ck_project_slug_length CHECK (length(slug) BETWEEN 1 AND 100),
  CONSTRAINT ck_project_environment_length CHECK (length(environment) BETWEEN 1 AND 100),
  CONSTRAINT ck_project_is_protected CHECK (is_protected IN (0, 1)),
  CONSTRAINT fk_project_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;


CREATE TABLE flow (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  project_id BLOB NOT NULL,
  name TEXT NOT NULL,
  draft_document BLOB NOT NULL,
  draft_document_format TEXT NOT NULL,
  draft_revision INTEGER NOT NULL,
  -- NULL until the first version is published.
  active_flow_version_id BLOB,
  -- NULL while the flow is active; set by a soft delete (ADR-0077 section 4).
  deleted_at INTEGER,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  deleted_by_principal_id BLOB,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_flow PRIMARY KEY (id),
  CONSTRAINT uq_flow_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_flow_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_project_id_length CHECK (length(project_id) = 16),
  CONSTRAINT ck_flow_name_length CHECK (length(name) BETWEEN 1 AND 200),
  CONSTRAINT ck_flow_draft_document_format CHECK (draft_document_format IN ('orcon/flow@1')),
  CONSTRAINT ck_flow_draft_revision_range CHECK (draft_revision >= 0),
  CONSTRAINT ck_flow_active_flow_version_id_length CHECK (length(active_flow_version_id) = 16),
  CONSTRAINT ck_flow_deleted_by_principal_id_length CHECK (length(deleted_by_principal_id) = 16),
  CONSTRAINT fk_flow_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_active_flow_version FOREIGN KEY (org_id, active_flow_version_id) REFERENCES flow_version (org_id, id) ON DELETE RESTRICT,
  CONSTRAINT fk_flow_deleted_by_principal FOREIGN KEY (deleted_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports ListFlows (keyset by id within a project) and the project foreign key.
CREATE INDEX ix_flow_org_id_project_id_id ON flow (org_id, project_id, id);

-- A flow name is unique per project among non-deleted flows (ADR-0077 section 4); supports GetFlowByName.
CREATE UNIQUE INDEX uq_flow_org_id_project_id_name ON flow (org_id, project_id, name) WHERE deleted_at IS NULL;

-- Supports fk_flow_active_flow_version and the delete of its parent row.
CREATE INDEX ix_flow_org_id_active_flow_version_id ON flow (org_id, active_flow_version_id) WHERE active_flow_version_id IS NOT NULL;

-- Supports fk_flow_deleted_by_principal and the delete of its parent row.
CREATE INDEX ix_flow_deleted_by_principal_id ON flow (deleted_by_principal_id) WHERE deleted_by_principal_id IS NOT NULL;

CREATE TABLE flow_version (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  version_number INTEGER NOT NULL,
  document BLOB NOT NULL,
  document_format TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  warning_document BLOB,
  warning_document_format TEXT,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  published_by_principal_id BLOB,
  published_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_flow_version PRIMARY KEY (id),
  CONSTRAINT uq_flow_version_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_version_org_id_flow_id_version_number UNIQUE (org_id, flow_id, version_number),
  CONSTRAINT ck_flow_version_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_version_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_version_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_flow_version_version_number_range CHECK (version_number >= 1),
  CONSTRAINT ck_flow_version_document_format CHECK (document_format IN ('orcon/flow@1')),
  CONSTRAINT ck_flow_version_content_hash_length CHECK (length(content_hash) BETWEEN 1 AND 128),
  CONSTRAINT ck_flow_version_warning_document_pair CHECK ((warning_document IS NULL) = (warning_document_format IS NULL)),
  CONSTRAINT ck_flow_version_warning_document_format CHECK (warning_document_format IN ('orcon/flow-warnings@1')),
  CONSTRAINT ck_flow_version_published_by_principal_id_length CHECK (length(published_by_principal_id) = 16),
  CONSTRAINT fk_flow_version_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_version_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_version_published_by_principal FOREIGN KEY (published_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports ListFlowVersions: keyset order (version_number, id) without a sort.
CREATE INDEX ix_flow_version_org_id_flow_id_version_number_id ON flow_version (org_id, flow_id, version_number, id);

-- Supports fk_flow_version_published_by_principal and the delete of its parent row.
CREATE INDEX ix_flow_version_published_by_principal_id ON flow_version (published_by_principal_id) WHERE published_by_principal_id IS NOT NULL;

CREATE TABLE flow_snapshot (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  content_hash TEXT NOT NULL,
  document BLOB NOT NULL,
  document_format TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_flow_snapshot PRIMARY KEY (id),
  CONSTRAINT uq_flow_snapshot_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_snapshot_org_id_content_hash UNIQUE (org_id, content_hash),
  CONSTRAINT ck_flow_snapshot_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_snapshot_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_snapshot_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_flow_snapshot_content_hash_length CHECK (length(content_hash) BETWEEN 1 AND 128),
  CONSTRAINT ck_flow_snapshot_document_format CHECK (document_format IN ('orcon/flow@1')),
  CONSTRAINT fk_flow_snapshot_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_snapshot_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports fk_flow_snapshot_flow and the delete of its parent row.
CREATE INDEX ix_flow_snapshot_org_id_flow_id ON flow_snapshot (org_id, flow_id);
