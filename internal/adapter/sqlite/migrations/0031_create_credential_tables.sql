-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- A credential of a project (SPEC-004 R-CRED-01..03, ADR-0043 section 1). Secret values are never in this row:
-- they live encrypted in credential_version. allowed_host_document is the JSON list of hosts the credential may be
-- sent to; it is read whole and nothing is queried inside it.
CREATE TABLE credential (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  project_id BLOB NOT NULL,
  name TEXT NOT NULL,
  connector_reference TEXT NOT NULL,
  status TEXT NOT NULL,
  allowed_host_document BLOB NOT NULL,
  allowed_host_document_format TEXT NOT NULL,
  current_version_number INTEGER NOT NULL,
  created_by_principal_id BLOB,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_credential PRIMARY KEY (id),
  CONSTRAINT uq_credential_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_credential_org_id_project_id_name UNIQUE (org_id, project_id, name),
  CONSTRAINT ck_credential_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_credential_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_credential_project_id_length CHECK (length(project_id) = 16),
  CONSTRAINT ck_credential_created_by_principal_id_length CHECK (created_by_principal_id IS NULL OR length(created_by_principal_id) = 16),
  CONSTRAINT ck_credential_name_length CHECK (length(name) BETWEEN 1 AND 100),
  CONSTRAINT ck_credential_connector_reference_length CHECK (length(connector_reference) BETWEEN 1 AND 255),
  CONSTRAINT ck_credential_status CHECK (status IN ('OK', 'NEEDS_SECRET', 'NEEDS_REAUTH', 'DISABLED')),
  CONSTRAINT ck_credential_allowed_host_document_format CHECK (allowed_host_document_format IN ('JSON')),
  CONSTRAINT ck_credential_allowed_host_document_length CHECK (length(allowed_host_document) BETWEEN 2 AND 65536),
  CONSTRAINT ck_credential_current_version_number CHECK (current_version_number >= 1),
  CONSTRAINT fk_credential_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_credential_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_credential_created_by_principal FOREIGN KEY (created_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports fk_credential_created_by_principal and the delete of the principal row.
CREATE INDEX ix_credential_created_by_principal_id ON credential (created_by_principal_id);

-- One version of a credential (R-CRED-02): replacing a secret creates the next version. The secret fields are one
-- ciphertext sealed under a data key of this version; the data key is wrapped by the organization key version named
-- in org_key_version_number. Nothing here is returned by any API: the public fields are the only readable part.
CREATE TABLE credential_version (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  credential_id BLOB NOT NULL,
  version_number INTEGER NOT NULL,
  public_field_document BLOB NOT NULL,
  public_field_document_format TEXT NOT NULL,
  secret_field_ciphertext BLOB NOT NULL,
  data_key_wrapped BLOB NOT NULL,
  org_key_version_number INTEGER NOT NULL,
  created_by_principal_id BLOB,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_credential_version PRIMARY KEY (id),
  CONSTRAINT uq_credential_version_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_credential_version_org_id_credential_id_version_number UNIQUE (org_id, credential_id, version_number),
  CONSTRAINT ck_credential_version_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_credential_version_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_credential_version_credential_id_length CHECK (length(credential_id) = 16),
  CONSTRAINT ck_credential_version_created_by_principal_id_length CHECK (created_by_principal_id IS NULL OR length(created_by_principal_id) = 16),
  CONSTRAINT ck_credential_version_version_number CHECK (version_number >= 1),
  CONSTRAINT ck_credential_version_public_field_document_format CHECK (public_field_document_format IN ('JSON')),
  CONSTRAINT ck_credential_version_public_field_document_length CHECK (length(public_field_document) BETWEEN 2 AND 65536),
  CONSTRAINT ck_credential_version_secret_field_ciphertext_length CHECK (length(secret_field_ciphertext) BETWEEN 29 AND 1048576),
  CONSTRAINT ck_credential_version_data_key_wrapped_length CHECK (length(data_key_wrapped) BETWEEN 1 AND 4096),
  CONSTRAINT ck_credential_version_org_key_version_number CHECK (org_key_version_number >= 1),
  CONSTRAINT fk_credential_version_credential FOREIGN KEY (org_id, credential_id) REFERENCES credential (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_credential_version_created_by_principal FOREIGN KEY (created_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports fk_credential_version_created_by_principal and the delete of the principal row.
CREATE INDEX ix_credential_version_created_by_principal_id ON credential_version (created_by_principal_id);

-- A share of a credential with another project of the same organization (R-AZ-07, ADR-0043 section 5): the target
-- project may use the credential, never read its secrets. Composite foreign keys keep a share inside one organization.
CREATE TABLE credential_share (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  credential_id BLOB NOT NULL,
  project_id BLOB NOT NULL,
  granted_by_principal_id BLOB,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_credential_share PRIMARY KEY (id),
  CONSTRAINT uq_credential_share_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_credential_share_org_id_credential_id_project_id UNIQUE (org_id, credential_id, project_id),
  CONSTRAINT ck_credential_share_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_credential_share_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_credential_share_credential_id_length CHECK (length(credential_id) = 16),
  CONSTRAINT ck_credential_share_project_id_length CHECK (length(project_id) = 16),
  CONSTRAINT ck_credential_share_granted_by_principal_id_length CHECK (granted_by_principal_id IS NULL OR length(granted_by_principal_id) = 16),
  CONSTRAINT fk_credential_share_credential FOREIGN KEY (org_id, credential_id) REFERENCES credential (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_credential_share_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_credential_share_granted_by_principal FOREIGN KEY (granted_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports the lookup of the credentials a project may use, and fk_credential_share_project.
CREATE INDEX ix_credential_share_org_id_project_id ON credential_share (org_id, project_id);
-- Supports fk_credential_share_granted_by_principal and the delete of the principal row.
CREATE INDEX ix_credential_share_granted_by_principal_id ON credential_share (granted_by_principal_id);
