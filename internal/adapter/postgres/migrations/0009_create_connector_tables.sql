-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


-- An installed connector package (ADR-0029 section 8). Instance table: a
-- package is installed once for the whole instance; organizations approve it
-- through connector_grant. The package archive itself is a blob (ADR-0016) and
-- joins this table when the blob store exists; the digest identifies it.
CREATE TABLE connector_package (
  id uuid NOT NULL,
  -- "<publisher>.<name>" (ADR-0029 section 3).
  connector_id text NOT NULL,
  -- SemVer 2.0.
  version text NOT NULL,
  -- "sha256:" and 64 lowercase hexadecimal digits.
  digest text NOT NULL,
  kind text NOT NULL,
  manifest_document bytea NOT NULL,
  manifest_document_format text NOT NULL,
  status text NOT NULL,
  installed_at timestamptz NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  installed_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_connector_package PRIMARY KEY (id),
  CONSTRAINT uq_connector_package_connector_id_version UNIQUE (connector_id, version),
  CONSTRAINT uq_connector_package_digest UNIQUE (digest),
  CONSTRAINT ck_connector_package_connector_id_length CHECK (char_length(connector_id) BETWEEN 3 AND 129),
  CONSTRAINT ck_connector_package_version_length CHECK (char_length(version) BETWEEN 5 AND 128),
  CONSTRAINT ck_connector_package_digest_length CHECK (char_length(digest) = 71),
  CONSTRAINT ck_connector_package_kind CHECK (kind IN ('DECLARATIVE', 'NATIVE', 'WASM', 'LIBRARY', 'RUNTIME')),
  CONSTRAINT ck_connector_package_manifest_document_format CHECK (manifest_document_format IN ('orcon/connector@1')),
  CONSTRAINT ck_connector_package_status CHECK (status IN ('INSTALLED', 'RETIRED')),
  CONSTRAINT fk_connector_package_installed_by_principal FOREIGN KEY (installed_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports fk_connector_package_installed_by_principal and the delete of its parent row.
CREATE INDEX ix_connector_package_installed_by_principal_id ON connector_package (installed_by_principal_id) WHERE installed_by_principal_id IS NOT NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON connector_package TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON connector_package TO orcon_engine;

-- The approval of a connector package by an organization (ADR-0029 section 8,
-- SPEC-004). A NULL project_id grants the whole organization.
CREATE TABLE connector_grant (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  connector_package_id uuid NOT NULL,
  project_id uuid,
  -- "sha256:" over the permissions block that was approved; a version with wider permissions needs a new grant.
  permissions_digest text NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  granted_by_principal_id uuid,
  granted_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_connector_grant PRIMARY KEY (id),
  CONSTRAINT uq_connector_grant_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_connector_grant_permissions_digest_length CHECK (char_length(permissions_digest) = 71),
  CONSTRAINT fk_connector_grant_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_connector_grant_connector_package FOREIGN KEY (connector_package_id) REFERENCES connector_package (id) ON DELETE RESTRICT,
  CONSTRAINT fk_connector_grant_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_connector_grant_granted_by_principal FOREIGN KEY (granted_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- One organization-wide grant per package (project_id IS NULL), and one per project otherwise.
CREATE UNIQUE INDEX uq_connector_grant_org_id_connector_package_id ON connector_grant (org_id, connector_package_id) WHERE project_id IS NULL;
CREATE UNIQUE INDEX uq_connector_grant_org_id_connector_package_id_project_id ON connector_grant (org_id, connector_package_id, project_id) WHERE project_id IS NOT NULL;

-- Supports fk_connector_grant_connector_package and the delete of its parent row.
CREATE INDEX ix_connector_grant_connector_package_id ON connector_grant (connector_package_id);

-- Supports fk_connector_grant_project and the delete of its parent row.
CREATE INDEX ix_connector_grant_org_id_project_id ON connector_grant (org_id, project_id) WHERE project_id IS NOT NULL;

-- Supports fk_connector_grant_granted_by_principal and the delete of its parent row.
CREATE INDEX ix_connector_grant_granted_by_principal_id ON connector_grant (granted_by_principal_id) WHERE granted_by_principal_id IS NOT NULL;

ALTER TABLE connector_grant ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector_grant FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON connector_grant FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE ON connector_grant TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON connector_grant TO orcon_engine;

-- A connector version pinned by a flow draft (flow_version_id IS NULL) or by a
-- published flow version (ADR-0034 section 6). Written in the transaction that
-- saves the draft or publishes the version, so counting references never reads
-- inside a flow document.
CREATE TABLE connector_reference (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  -- NULL for the draft.
  flow_version_id uuid,
  connector_id text NOT NULL,
  connector_version text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_connector_reference PRIMARY KEY (id),
  CONSTRAINT uq_connector_reference_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_connector_reference_connector_id_length CHECK (char_length(connector_id) BETWEEN 3 AND 129),
  CONSTRAINT ck_connector_reference_connector_version_length CHECK (char_length(connector_version) BETWEEN 5 AND 128),
  CONSTRAINT fk_connector_reference_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_connector_reference_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_connector_reference_flow_version FOREIGN KEY (org_id, flow_version_id) REFERENCES flow_version (org_id, id) ON DELETE CASCADE
);

-- A flow pins a connector version at most once in its draft, and once per published version.
-- Draft pins: (org_id, flow_id, connector_id, connector_version) where flow_version_id IS NULL.
CREATE UNIQUE INDEX uq_connector_reference_draft_pin ON connector_reference (org_id, flow_id, connector_id, connector_version) WHERE flow_version_id IS NULL;
-- Version pins: (org_id, flow_version_id, connector_id, connector_version) where flow_version_id IS NOT NULL.
CREATE UNIQUE INDEX uq_connector_reference_version_pin ON connector_reference (org_id, flow_version_id, connector_id, connector_version) WHERE flow_version_id IS NOT NULL;

-- Supports fk_connector_reference_flow and the replacement of a draft's references.
CREATE INDEX ix_connector_reference_org_id_flow_id ON connector_reference (org_id, flow_id);

-- Supports counting the flows that pin a connector version (retirement, ADR-0034 section 6).
CREATE INDEX ix_connector_reference_connector_id_connector_version ON connector_reference (connector_id, connector_version);

ALTER TABLE connector_reference ENABLE ROW LEVEL SECURITY;
ALTER TABLE connector_reference FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON connector_reference FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, DELETE ON connector_reference TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON connector_reference TO orcon_engine;
