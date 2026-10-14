-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The instance key (IK, ADR-0042 section 1): 256 random bits, stored wrapped by the master key, never in clear.
-- key_version starts at 1; exactly one version is ACTIVE (partial unique index).
CREATE TABLE instance_key (
  id BLOB NOT NULL,
  key_level TEXT NOT NULL,
  key_version INTEGER NOT NULL,
  wrapped BLOB NOT NULL,
  master_key_reference TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_instance_key PRIMARY KEY (id),
  CONSTRAINT ck_instance_key_id_length CHECK (length(id) = 16),
  CONSTRAINT uq_instance_key_key_level_key_version UNIQUE (key_level, key_version),
  CONSTRAINT ck_instance_key_level CHECK (key_level = 'INSTANCE'),
  CONSTRAINT ck_instance_key_version CHECK (key_version >= 1),
  CONSTRAINT ck_instance_key_status CHECK (status IN ('ACTIVE', 'RETIRING', 'RETIRED', 'DESTROYED')),
  CONSTRAINT ck_instance_key_wrapped_length CHECK (length(wrapped) BETWEEN 1 AND 4096),
  CONSTRAINT ck_instance_key_master_key_reference_length CHECK (length(master_key_reference) BETWEEN 1 AND 255)
) STRICT;

CREATE UNIQUE INDEX uq_instance_key_key_level ON instance_key (key_level) WHERE status = 'ACTIVE';

-- The probes of the keys: a fixed plaintext sealed under the key, checked at startup before any write (R-KEY-05).
-- key_level is MASTER (the master key, version 1, has only this row) or INSTANCE; the probe of an INSTANCE row
-- belongs to the instance_key row of the same version and is checked by the adapter, not by a foreign key,
-- because a foreign key cannot be conditional on key_level.
CREATE TABLE instance_key_canary (
  id BLOB NOT NULL,
  key_level TEXT NOT NULL,
  key_version INTEGER NOT NULL,
  probe BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_instance_key_canary PRIMARY KEY (id),
  CONSTRAINT ck_instance_key_canary_id_length CHECK (length(id) = 16),
  CONSTRAINT uq_instance_key_canary_key_level_key_version UNIQUE (key_level, key_version),
  CONSTRAINT ck_instance_key_canary_level CHECK (key_level IN ('MASTER', 'INSTANCE')),
  CONSTRAINT ck_instance_key_canary_version CHECK (key_version >= 1),
  CONSTRAINT ck_instance_key_canary_probe_length CHECK (length(probe) BETWEEN 1 AND 8192)
) STRICT;

-- An organization key (OK): one row per version, with its probe; exactly one version is ACTIVE per organization.
CREATE TABLE org_key (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  key_version INTEGER NOT NULL,
  wrapped BLOB NOT NULL,
  master_key_reference TEXT NOT NULL,
  status TEXT NOT NULL,
  probe BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_org_key PRIMARY KEY (id),
  CONSTRAINT ck_org_key_id_length CHECK (length(id) = 16),
  CONSTRAINT uq_org_key_org_id_key_version UNIQUE (org_id, key_version),
  CONSTRAINT ck_org_key_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_org_key_version CHECK (key_version >= 1),
  CONSTRAINT ck_org_key_status CHECK (status IN ('ACTIVE', 'RETIRING', 'RETIRED', 'DESTROYED')),
  CONSTRAINT ck_org_key_wrapped_length CHECK (length(wrapped) BETWEEN 1 AND 4096),
  CONSTRAINT ck_org_key_master_key_reference_length CHECK (length(master_key_reference) BETWEEN 1 AND 255),
  CONSTRAINT ck_org_key_probe_length CHECK (length(probe) BETWEEN 1 AND 8192),
  -- org is an instance table, so this foreign key is single-column (TI-7).
  CONSTRAINT fk_org_key_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

CREATE UNIQUE INDEX uq_org_key_org_id ON org_key (org_id) WHERE status = 'ACTIVE';
