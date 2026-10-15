-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The instance key (IK, ADR-0042 section 1): 256 random bits, stored wrapped by the master key, never in clear.
-- key_version starts at 1; exactly one version is ACTIVE (partial unique index).
CREATE TABLE instance_key (
  id bigint GENERATED ALWAYS AS IDENTITY,
  key_level text NOT NULL,
  key_version bigint NOT NULL,
  wrapped bytea NOT NULL,
  master_key_reference text NOT NULL,
  status text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_instance_key PRIMARY KEY (id),
  CONSTRAINT uq_instance_key_key_level_key_version UNIQUE (key_level, key_version),
  CONSTRAINT ck_instance_key_level CHECK (key_level = 'INSTANCE'),
  CONSTRAINT ck_instance_key_version CHECK (key_version >= 1),
  CONSTRAINT ck_instance_key_status CHECK (status IN ('ACTIVE', 'RETIRING', 'RETIRED', 'DESTROYED')),
  CONSTRAINT ck_instance_key_wrapped_length CHECK (octet_length(wrapped) BETWEEN 1 AND 4096),
  CONSTRAINT ck_instance_key_master_key_reference_length CHECK (char_length(master_key_reference) BETWEEN 1 AND 255)
);

CREATE UNIQUE INDEX uq_instance_key_key_level ON instance_key (key_level) WHERE status = 'ACTIVE';

GRANT SELECT, INSERT, UPDATE, DELETE ON instance_key TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON instance_key TO orcon_engine;

-- The probes of the keys: a fixed plaintext sealed under the key, checked at startup before any write (R-KEY-05).
-- key_level is MASTER (the master key, version 1, has only this row) or INSTANCE; the probe of an INSTANCE row
-- belongs to the instance_key row of the same version and is checked by the adapter, not by a foreign key,
-- because a foreign key cannot be conditional on key_level.
CREATE TABLE instance_key_canary (
  id bigint GENERATED ALWAYS AS IDENTITY,
  key_level text NOT NULL,
  key_version bigint NOT NULL,
  probe bytea NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_instance_key_canary PRIMARY KEY (id),
  CONSTRAINT uq_instance_key_canary_key_level_key_version UNIQUE (key_level, key_version),
  CONSTRAINT ck_instance_key_canary_level CHECK (key_level IN ('MASTER', 'INSTANCE')),
  CONSTRAINT ck_instance_key_canary_version CHECK (key_version >= 1),
  CONSTRAINT ck_instance_key_canary_probe_length CHECK (octet_length(probe) BETWEEN 1 AND 8192)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON instance_key_canary TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON instance_key_canary TO orcon_engine;

-- An organization key (OK): one row per version, with its probe; exactly one version is ACTIVE per organization.
CREATE TABLE org_key (
  id bigint GENERATED ALWAYS AS IDENTITY,
  org_id uuid NOT NULL,
  key_version bigint NOT NULL,
  wrapped bytea NOT NULL,
  master_key_reference text NOT NULL,
  status text NOT NULL,
  probe bytea NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_org_key PRIMARY KEY (id),
  CONSTRAINT uq_org_key_org_id_key_version UNIQUE (org_id, key_version),
  CONSTRAINT ck_org_key_version CHECK (key_version >= 1),
  CONSTRAINT ck_org_key_status CHECK (status IN ('ACTIVE', 'RETIRING', 'RETIRED', 'DESTROYED')),
  CONSTRAINT ck_org_key_wrapped_length CHECK (octet_length(wrapped) BETWEEN 1 AND 4096),
  CONSTRAINT ck_org_key_master_key_reference_length CHECK (char_length(master_key_reference) BETWEEN 1 AND 255),
  CONSTRAINT ck_org_key_probe_length CHECK (octet_length(probe) BETWEEN 1 AND 8192),
  -- org is an instance table, so this foreign key is single-column (TI-7).
  CONSTRAINT fk_org_key_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_org_key_org_id ON org_key (org_id) WHERE status = 'ACTIVE';

ALTER TABLE org_key ENABLE ROW LEVEL SECURITY;
ALTER TABLE org_key FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON org_key FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON org_key TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON org_key TO orcon_engine;

-- The active organization keys of every organization, for startup verification and the per-scope key lookup
-- (ADR-0042 section 3). Read in the engine plane: a SECURITY DEFINER function owned by orcon_engine, because the
-- application role sees one organization at a time.
CREATE FUNCTION orcon_engine.list_active_org_keys()
RETURNS TABLE (org_id uuid, key_version bigint, wrapped bytea, master_key_reference text, status text, created_at timestamptz, probe bytea)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  SELECT k.org_id, k.key_version, k.wrapped, k.master_key_reference, k.status, k.created_at, k.probe
  FROM org_key AS k
  WHERE k.status = 'ACTIVE'
$$;

ALTER FUNCTION orcon_engine.list_active_org_keys() OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.list_active_org_keys() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.list_active_org_keys() TO orcon_application;
