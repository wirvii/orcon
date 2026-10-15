-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- A credential of a project (SPEC-004 R-CRED-01..03, ADR-0043 section 1). Secret values are never in this row:
-- they live encrypted in credential_version. allowed_host_document is the JSON list of hosts the credential may be
-- sent to; it is read whole and nothing is queried inside it.
CREATE TABLE credential (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  project_id uuid NOT NULL,
  name text NOT NULL,
  connector_reference text NOT NULL,
  status text NOT NULL,
  allowed_host_document bytea NOT NULL,
  allowed_host_document_format text NOT NULL,
  current_version_number bigint NOT NULL,
  created_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_credential PRIMARY KEY (id),
  CONSTRAINT uq_credential_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_credential_org_id_project_id_name UNIQUE (org_id, project_id, name),
  CONSTRAINT ck_credential_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
  CONSTRAINT ck_credential_connector_reference_length CHECK (char_length(connector_reference) BETWEEN 1 AND 255),
  CONSTRAINT ck_credential_status CHECK (status IN ('OK', 'NEEDS_SECRET', 'NEEDS_REAUTH', 'DISABLED')),
  CONSTRAINT ck_credential_allowed_host_document_format CHECK (allowed_host_document_format IN ('JSON')),
  CONSTRAINT ck_credential_allowed_host_document_length CHECK (length(allowed_host_document) BETWEEN 2 AND 65536),
  CONSTRAINT ck_credential_current_version_number CHECK (current_version_number >= 1),
  CONSTRAINT fk_credential_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_credential_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_credential_created_by_principal FOREIGN KEY (created_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports fk_credential_created_by_principal and the delete of the principal row.
CREATE INDEX ix_credential_created_by_principal_id ON credential (created_by_principal_id);

-- One version of a credential (R-CRED-02): replacing a secret creates the next version. The secret fields are one
-- ciphertext sealed under a data key of this version; the data key is wrapped by the organization key version named
-- in org_key_version_number. Nothing here is returned by any API: the public fields are the only readable part.
CREATE TABLE credential_version (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  credential_id uuid NOT NULL,
  version_number bigint NOT NULL,
  public_field_document bytea NOT NULL,
  public_field_document_format text NOT NULL,
  secret_field_ciphertext bytea NOT NULL,
  data_key_wrapped bytea NOT NULL,
  org_key_version_number bigint NOT NULL,
  created_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_credential_version PRIMARY KEY (id),
  CONSTRAINT uq_credential_version_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_credential_version_org_id_credential_id_version_number UNIQUE (org_id, credential_id, version_number),
  CONSTRAINT ck_credential_version_version_number CHECK (version_number >= 1),
  CONSTRAINT ck_credential_version_public_field_document_format CHECK (public_field_document_format IN ('JSON')),
  CONSTRAINT ck_credential_version_public_field_document_length CHECK (length(public_field_document) BETWEEN 2 AND 65536),
  CONSTRAINT ck_credential_version_secret_field_ciphertext_length CHECK (length(secret_field_ciphertext) BETWEEN 29 AND 1048576),
  CONSTRAINT ck_credential_version_data_key_wrapped_length CHECK (length(data_key_wrapped) BETWEEN 1 AND 4096),
  CONSTRAINT ck_credential_version_org_key_version_number CHECK (org_key_version_number >= 1),
  CONSTRAINT fk_credential_version_credential FOREIGN KEY (org_id, credential_id) REFERENCES credential (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_credential_version_created_by_principal FOREIGN KEY (created_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports fk_credential_version_created_by_principal and the delete of the principal row.
CREATE INDEX ix_credential_version_created_by_principal_id ON credential_version (created_by_principal_id);

-- A share of a credential with another project of the same organization (R-AZ-07, ADR-0043 section 5): the target
-- project may use the credential, never read its secrets. Composite foreign keys keep a share inside one organization.
CREATE TABLE credential_share (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  credential_id uuid NOT NULL,
  project_id uuid NOT NULL,
  granted_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_credential_share PRIMARY KEY (id),
  CONSTRAINT uq_credential_share_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_credential_share_org_id_credential_id_project_id UNIQUE (org_id, credential_id, project_id),
  CONSTRAINT fk_credential_share_credential FOREIGN KEY (org_id, credential_id) REFERENCES credential (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_credential_share_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_credential_share_granted_by_principal FOREIGN KEY (granted_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports the lookup of the credentials a project may use, and fk_credential_share_project.
CREATE INDEX ix_credential_share_org_id_project_id ON credential_share (org_id, project_id);
-- Supports fk_credential_share_granted_by_principal and the delete of the principal row.
CREATE INDEX ix_credential_share_granted_by_principal_id ON credential_share (granted_by_principal_id);

ALTER TABLE credential ENABLE ROW LEVEL SECURITY;
ALTER TABLE credential FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON credential FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON credential TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON credential TO orcon_engine;

ALTER TABLE credential_version ENABLE ROW LEVEL SECURITY;
ALTER TABLE credential_version FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON credential_version FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON credential_version TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON credential_version TO orcon_engine;

ALTER TABLE credential_share ENABLE ROW LEVEL SECURITY;
ALTER TABLE credential_share FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON credential_share FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON credential_share TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON credential_share TO orcon_engine;

-- The principal-clearing function (orcon-db IN-12) also clears the author columns of the credential tables. Migration
-- 0013 is immutable, so this migration replaces the function with the same signature; the statements are those of
-- 0013 plus one per new actor column (SQLite: the UPDATE queries of credential.sql and the clearers of identity.go).
CREATE OR REPLACE FUNCTION orcon_engine.clear_principal_references(p_principal_id uuid, p_limit integer)
RETURNS integer
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  WITH cleared_flow AS (
    UPDATE flow SET deleted_by_principal_id = NULL
    WHERE id IN (SELECT id FROM flow WHERE deleted_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_flow_version AS (
    UPDATE flow_version SET published_by_principal_id = NULL
    WHERE id IN (SELECT id FROM flow_version WHERE published_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_membership AS (
    UPDATE membership SET invited_by_principal_id = NULL
    WHERE id IN (SELECT id FROM membership WHERE invited_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_instance_alert AS (
    UPDATE instance_alert SET acknowledged_by_principal_id = NULL
    WHERE id IN (SELECT id FROM instance_alert WHERE acknowledged_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_external_operation AS (
    UPDATE external_operation SET resolved_by_principal_id = NULL
    WHERE id IN (SELECT id FROM external_operation WHERE resolved_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_human_task AS (
    UPDATE human_task SET resolved_by_principal_id = NULL
    WHERE id IN (SELECT id FROM human_task WHERE resolved_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_attention_request_acknowledged AS (
    UPDATE attention_request SET acknowledged_by_principal_id = NULL
    WHERE id IN (SELECT id FROM attention_request WHERE acknowledged_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  -- A row that names the principal in both columns is cleared by the first statement; this one leaves it
  -- for the next call, so no row is updated twice by the same statement.
  cleared_attention_request_resolved AS (
    UPDATE attention_request SET resolved_by_principal_id = NULL
    WHERE id IN (
      SELECT id FROM attention_request
      WHERE resolved_by_principal_id = p_principal_id AND acknowledged_by_principal_id IS DISTINCT FROM p_principal_id
      ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_operational_alert AS (
    UPDATE operational_alert SET acknowledged_by_principal_id = NULL
    WHERE id IN (SELECT id FROM operational_alert WHERE acknowledged_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_connector_package AS (
    UPDATE connector_package SET installed_by_principal_id = NULL
    WHERE id IN (SELECT id FROM connector_package WHERE installed_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_connector_grant AS (
    UPDATE connector_grant SET granted_by_principal_id = NULL
    WHERE id IN (SELECT id FROM connector_grant WHERE granted_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_credential AS (
    UPDATE credential SET created_by_principal_id = NULL
    WHERE id IN (SELECT id FROM credential WHERE created_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_credential_version AS (
    UPDATE credential_version SET created_by_principal_id = NULL
    WHERE id IN (SELECT id FROM credential_version WHERE created_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_credential_share AS (
    UPDATE credential_share SET granted_by_principal_id = NULL
    WHERE id IN (SELECT id FROM credential_share WHERE granted_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  )
  SELECT ((SELECT count(*) FROM cleared_flow)
    + (SELECT count(*) FROM cleared_flow_version)
    + (SELECT count(*) FROM cleared_membership)
    + (SELECT count(*) FROM cleared_instance_alert)
    + (SELECT count(*) FROM cleared_external_operation)
    + (SELECT count(*) FROM cleared_human_task)
    + (SELECT count(*) FROM cleared_attention_request_acknowledged)
    + (SELECT count(*) FROM cleared_attention_request_resolved)
    + (SELECT count(*) FROM cleared_operational_alert)
    + (SELECT count(*) FROM cleared_connector_package)
    + (SELECT count(*) FROM cleared_connector_grant)
    + (SELECT count(*) FROM cleared_credential)
    + (SELECT count(*) FROM cleared_credential_version)
    + (SELECT count(*) FROM cleared_credential_share))::integer
$$;

ALTER FUNCTION orcon_engine.clear_principal_references(uuid, integer) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.clear_principal_references(uuid, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.clear_principal_references(uuid, integer) TO orcon_application;
