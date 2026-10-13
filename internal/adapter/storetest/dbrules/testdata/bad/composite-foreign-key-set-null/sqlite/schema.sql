-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

CREATE TABLE org (
  id BLOB NOT NULL,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_org PRIMARY KEY (id),
  CONSTRAINT ck_org_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_org_status CHECK (status IN ('ACTIVE', 'DISABLED'))
) STRICT;

CREATE TABLE principal (
  id BLOB NOT NULL,
  kind TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_principal PRIMARY KEY (id),
  CONSTRAINT ck_principal_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_principal_kind CHECK (kind IN ('USER', 'SERVICE_ACCOUNT', 'SYSTEM'))
) STRICT;
CREATE UNIQUE INDEX uq_principal_kind ON principal (kind) WHERE kind = 'SYSTEM';

CREATE TABLE application_user (
  id BLOB NOT NULL,
  principal_id BLOB NOT NULL,
  email_normalized TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_application_user PRIMARY KEY (id),
  CONSTRAINT ck_application_user_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_application_user_principal_id_length CHECK (length(principal_id) = 16),
  CONSTRAINT uq_application_user_principal_id UNIQUE (principal_id),
  CONSTRAINT fk_application_user_principal FOREIGN KEY (principal_id) REFERENCES principal (id) ON DELETE CASCADE
) STRICT;

CREATE TABLE project (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_project PRIMARY KEY (id),
  CONSTRAINT ck_project_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_project_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT uq_project_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_project_org_id_name UNIQUE (org_id, name),
  CONSTRAINT fk_project_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

CREATE TABLE flow (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  project_id BLOB NOT NULL,
  name TEXT NOT NULL,
  draft_document BLOB NOT NULL,
  draft_document_format TEXT NOT NULL,
  deleted_at INTEGER,
  deleted_by_principal_id BLOB,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_flow PRIMARY KEY (id),
  CONSTRAINT ck_flow_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_project_id_length CHECK (length(project_id) = 16),
  CONSTRAINT ck_flow_deleted_by_principal_id_length CHECK (deleted_by_principal_id IS NULL OR length(deleted_by_principal_id) = 16),
  CONSTRAINT ck_flow_draft_document_format CHECK (draft_document_format IN ('orcon/flow@1')),
  CONSTRAINT uq_flow_org_id_id UNIQUE (org_id, id),
  CONSTRAINT fk_flow_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_deleted_by_principal FOREIGN KEY (deleted_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;
CREATE UNIQUE INDEX uq_flow_org_id_project_id_name ON flow (org_id, project_id, name) WHERE deleted_at IS NULL;
CREATE INDEX ix_flow_deleted_by_principal_id ON flow (deleted_by_principal_id);

CREATE TABLE flow_version (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  version_number INTEGER NOT NULL,
  document BLOB NOT NULL,
  document_format TEXT NOT NULL,
  published_by_principal_id BLOB,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_flow_version PRIMARY KEY (id),
  CONSTRAINT ck_flow_version_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_version_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_version_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_flow_version_published_by_principal_id_length CHECK (published_by_principal_id IS NULL OR length(published_by_principal_id) = 16),
  CONSTRAINT ck_flow_version_document_format CHECK (document_format IN ('orcon/flow@1')),
  CONSTRAINT uq_flow_version_org_id_flow_id_version_number UNIQUE (org_id, flow_id, version_number),
  CONSTRAINT fk_flow_version_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_version_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_version_published_by_principal FOREIGN KEY (published_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;
CREATE INDEX ix_flow_version_published_by_principal_id ON flow_version (published_by_principal_id);

CREATE TABLE api_key (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  name TEXT NOT NULL,
  token_hash BLOB NOT NULL,
  sha256 BLOB NOT NULL,
  is_active INTEGER NOT NULL,
  http_status INTEGER NOT NULL,
  authentication_kind TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_api_key PRIMARY KEY (id),
  CONSTRAINT ck_api_key_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_api_key_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT uq_api_key_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_api_key_is_active CHECK (is_active IN (0, 1)),
  CONSTRAINT ck_api_key_http_status CHECK (http_status BETWEEN 100 AND 599),
  CONSTRAINT ck_api_key_authentication_kind CHECK (authentication_kind IN ('SECRET', 'OAUTH2')),
  CONSTRAINT fk_api_key_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

CREATE TABLE oauth_token (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  api_key_id BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_oauth_token PRIMARY KEY (id),
  CONSTRAINT ck_oauth_token_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_oauth_token_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT uq_oauth_token_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_oauth_token_api_key_id_length CHECK (length(api_key_id) = 16),
  CONSTRAINT uq_oauth_token_org_id_api_key_id UNIQUE (org_id, api_key_id),
  CONSTRAINT fk_oauth_token_api_key FOREIGN KEY (org_id, api_key_id) REFERENCES api_key (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_oauth_token_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

CREATE TABLE execution_event (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  sequence_number INTEGER NOT NULL,
  node_id TEXT NOT NULL,
  occurred_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_execution_event PRIMARY KEY (id),
  CONSTRAINT ck_execution_event_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_execution_event_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_execution_event_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT uq_execution_event_org_id_flow_id_sequence_number UNIQUE (org_id, flow_id, sequence_number),
  CONSTRAINT fk_execution_event_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_event_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
) STRICT;

CREATE TABLE bookmark (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_bookmark PRIMARY KEY (id),
  CONSTRAINT ck_bookmark_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_bookmark_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT uq_bookmark_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_bookmark_flow_id_length CHECK (flow_id IS NULL OR length(flow_id) = 16),
  CONSTRAINT uq_bookmark_org_id_flow_id UNIQUE (org_id, flow_id),
  CONSTRAINT fk_bookmark_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE SET NULL,
  CONSTRAINT fk_bookmark_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

