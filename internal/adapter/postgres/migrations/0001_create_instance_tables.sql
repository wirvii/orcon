-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

REVOKE CREATE ON SCHEMA public FROM PUBLIC;

-- Cluster roles (tenant-isolation.md TI-5). They are created here when missing so
-- a first start on an empty server works; creating a role needs CREATEROLE.
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'orcon_owner') THEN
    CREATE ROLE orcon_owner NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'orcon_application') THEN
    CREATE ROLE orcon_application NOLOGIN NOSUPERUSER NOBYPASSRLS;
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'orcon_engine') THEN
    CREATE ROLE orcon_engine NOLOGIN NOBYPASSRLS;
  END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO orcon_application, orcon_engine;

-- Context helpers read by the row-level security policies (ADR-0046 as amended by ADR-0077).
CREATE SCHEMA orcon_context;
GRANT USAGE ON SCHEMA orcon_context TO orcon_application, orcon_engine;
CREATE FUNCTION orcon_context.current_org() RETURNS uuid LANGUAGE sql STABLE AS
$$ SELECT nullif(current_setting('orcon.org_id', true), '')::uuid $$;
CREATE FUNCTION orcon_context.current_org_ids() RETURNS SETOF uuid LANGUAGE sql STABLE AS
$$ SELECT CAST(organization_id_text AS uuid)
   FROM string_to_table(NULLIF(current_setting('orcon.org_ids', true), ''), ',') AS organization_id_text $$;

-- Engine plane (ADR-0047): cross-organization functions live in this schema and run as orcon_engine.
CREATE SCHEMA orcon_engine;
GRANT USAGE ON SCHEMA orcon_engine TO orcon_application;

CREATE TABLE schema_migration (
  version bigint NOT NULL,
  checksum text NOT NULL,
  applied_at timestamptz NOT NULL,
  binary_version text NOT NULL,
  compatibility_floor text NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_schema_migration PRIMARY KEY (version),
  CONSTRAINT ck_schema_migration_version_range CHECK (version >= 1),
  CONSTRAINT ck_schema_migration_binary_version_length CHECK (char_length(binary_version) BETWEEN 1 AND 128),
  CONSTRAINT ck_schema_migration_compatibility_floor_length CHECK (char_length(compatibility_floor) BETWEEN 1 AND 128),
  CONSTRAINT ck_schema_migration_checksum_length CHECK (length(checksum) = 64)
);

GRANT SELECT, INSERT ON schema_migration TO orcon_application;

CREATE TABLE worker (
  id uuid NOT NULL,
  role text NOT NULL,
  host text NOT NULL,
  version text NOT NULL,
  started_at timestamptz NOT NULL,
  heartbeat_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_worker PRIMARY KEY (id),
  CONSTRAINT ck_worker_role CHECK (role IN ('STANDALONE', 'API', 'SCHEDULER', 'WORKER')),
  CONSTRAINT ck_worker_host_length CHECK (char_length(host) BETWEEN 1 AND 255),
  CONSTRAINT ck_worker_version_length CHECK (char_length(version) BETWEEN 1 AND 128)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON worker TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON worker TO orcon_engine;

CREATE TABLE org (
  id uuid NOT NULL,
  name text NOT NULL,
  slug text NOT NULL,
  status text NOT NULL,
  -- NULL unless the organization is scheduled for deletion.
  deletion_scheduled_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_org PRIMARY KEY (id),
  CONSTRAINT uq_org_slug UNIQUE (slug),
  CONSTRAINT ck_org_name_length CHECK (char_length(name) BETWEEN 1 AND 200),
  CONSTRAINT ck_org_slug_length CHECK (char_length(slug) BETWEEN 1 AND 100),
  CONSTRAINT ck_org_status CHECK (status IN ('ACTIVE', 'SUSPENDED', 'DELETION_SCHEDULED')),
  CONSTRAINT ck_org_deletion_scheduled CHECK ((status = 'DELETION_SCHEDULED') = (deletion_scheduled_at IS NOT NULL))
);

ALTER TABLE org ENABLE ROW LEVEL SECURITY;
ALTER TABLE org FORCE ROW LEVEL SECURITY;
CREATE POLICY org_self_isolation ON org FOR ALL
  USING (id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON org TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON org TO orcon_engine;

CREATE TABLE principal (
  id uuid NOT NULL,
  kind text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_principal PRIMARY KEY (id),
  CONSTRAINT ck_principal_kind CHECK (kind IN ('USER', 'SERVICE_ACCOUNT', 'SYSTEM'))
);

-- Exactly one SYSTEM principal per installation (IN-12).
CREATE UNIQUE INDEX uq_principal_kind ON principal (kind) WHERE kind = 'SYSTEM';

GRANT SELECT, INSERT, UPDATE, DELETE ON principal TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON principal TO orcon_engine;
