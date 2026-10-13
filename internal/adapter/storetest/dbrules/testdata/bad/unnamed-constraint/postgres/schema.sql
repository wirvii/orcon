-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
CREATE SCHEMA orcon_context;
CREATE FUNCTION orcon_context.current_org() RETURNS uuid LANGUAGE sql STABLE AS $$ SELECT nullif(current_setting('orcon.org_id', true), '')::uuid $$;
CREATE FUNCTION orcon_context.current_org_ids() RETURNS SETOF uuid LANGUAGE sql STABLE AS $$ SELECT unnest(string_to_array(nullif(current_setting('orcon.org_ids', true), ''), ',')::uuid[]) $$;

CREATE TABLE org (
  id uuid NOT NULL,
  name text NOT NULL,
  status text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_org PRIMARY KEY (id),
  CONSTRAINT ck_org_status CHECK (status IN ('ACTIVE', 'DISABLED'))
);
ALTER TABLE org ENABLE ROW LEVEL SECURITY;
ALTER TABLE org FORCE ROW LEVEL SECURITY;
CREATE POLICY org_self_isolation ON org FOR ALL
  USING (id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON org TO orcon_application;

CREATE TABLE principal (
  id uuid NOT NULL,
  kind text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_principal PRIMARY KEY (id),
  CONSTRAINT ck_principal_kind CHECK (kind IN ('USER', 'SERVICE_ACCOUNT', 'SYSTEM'))
);
CREATE UNIQUE INDEX uq_principal_kind ON principal (kind) WHERE kind = 'SYSTEM';
GRANT SELECT, INSERT, UPDATE, DELETE ON principal TO orcon_application;

CREATE TABLE application_user (
  id uuid NOT NULL,
  principal_id uuid NOT NULL,
  email_normalized text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_application_user PRIMARY KEY (id),
  CONSTRAINT uq_application_user_principal_id UNIQUE (principal_id),
  CONSTRAINT fk_application_user_principal FOREIGN KEY (principal_id) REFERENCES principal (id) ON DELETE CASCADE
);
GRANT SELECT, INSERT, UPDATE, DELETE ON application_user TO orcon_application;

CREATE TABLE project (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_project PRIMARY KEY (id),
  CONSTRAINT uq_project_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_project_org_id_name UNIQUE (org_id, name),
  CONSTRAINT fk_project_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);
ALTER TABLE project ENABLE ROW LEVEL SECURITY;
ALTER TABLE project FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON project FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON project TO orcon_application;

CREATE TABLE flow (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  project_id uuid NOT NULL,
  name text NOT NULL,
  draft_document bytea NOT NULL,
  draft_document_format text NOT NULL,
  deleted_at timestamptz,
  deleted_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_flow PRIMARY KEY (id),
  CONSTRAINT ck_flow_draft_document_format CHECK (draft_document_format IN ('orcon/flow@1')),
  CONSTRAINT uq_flow_org_id_id UNIQUE (org_id, id),
  CONSTRAINT fk_flow_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_deleted_by_principal FOREIGN KEY (deleted_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX uq_flow_org_id_project_id_name ON flow (org_id, project_id, name) WHERE deleted_at IS NULL;
CREATE INDEX ix_flow_deleted_by_principal_id ON flow (deleted_by_principal_id);
ALTER TABLE flow ENABLE ROW LEVEL SECURITY;
ALTER TABLE flow FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON flow FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON flow TO orcon_application;

CREATE TABLE flow_version (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  version_number bigint NOT NULL,
  document bytea NOT NULL,
  document_format text NOT NULL,
  published_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_flow_version PRIMARY KEY (id),
  CONSTRAINT ck_flow_version_document_format CHECK (document_format IN ('orcon/flow@1')),
  CONSTRAINT uq_flow_version_org_id_flow_id_version_number UNIQUE (org_id, flow_id, version_number),
  CONSTRAINT fk_flow_version_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_version_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_version_published_by_principal FOREIGN KEY (published_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);
CREATE INDEX ix_flow_version_published_by_principal_id ON flow_version (published_by_principal_id);
ALTER TABLE flow_version ENABLE ROW LEVEL SECURITY;
ALTER TABLE flow_version FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON flow_version FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT ON flow_version TO orcon_application;

CREATE TABLE api_key (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  name text NOT NULL,
  token_hash bytea NOT NULL,
  sha256 bytea NOT NULL,
  is_active boolean NOT NULL,
  http_status bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_api_key PRIMARY KEY (id),
  CONSTRAINT uq_api_key_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_api_key_http_status CHECK (http_status BETWEEN 100 AND 599),
  CONSTRAINT fk_api_key_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);
ALTER TABLE api_key ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_key FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON api_key FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON api_key TO orcon_application;

CREATE TABLE oauth_token (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  api_key_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_oauth_token PRIMARY KEY (id),
  CONSTRAINT uq_oauth_token_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_oauth_token_org_id_api_key_id UNIQUE (org_id, api_key_id),
  CONSTRAINT fk_oauth_token_api_key FOREIGN KEY (org_id, api_key_id) REFERENCES api_key (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_oauth_token_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);
ALTER TABLE oauth_token ENABLE ROW LEVEL SECURITY;
ALTER TABLE oauth_token FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON oauth_token FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON oauth_token TO orcon_application;

CREATE TABLE execution_event (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  sequence_number bigint NOT NULL,
  node_id text NOT NULL,
  occurred_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_execution_event PRIMARY KEY (id),
  CONSTRAINT uq_execution_event_org_id_flow_id_sequence_number UNIQUE (org_id, flow_id, sequence_number),
  CONSTRAINT fk_execution_event_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_event_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
);
ALTER TABLE execution_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE execution_event FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON execution_event FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT ON execution_event TO orcon_application;

CREATE TABLE thing (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  name text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_thing PRIMARY KEY (id),
  CONSTRAINT uq_thing_org_id_id UNIQUE (org_id, id),
  UNIQUE (name),
  CONSTRAINT fk_thing_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);
ALTER TABLE thing ENABLE ROW LEVEL SECURITY;
ALTER TABLE thing FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON thing FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine' OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON thing TO orcon_application;

