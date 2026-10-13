-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE schema_migration (
  version INTEGER NOT NULL,
  checksum TEXT NOT NULL,
  applied_at INTEGER NOT NULL,
  binary_version TEXT NOT NULL,
  compatibility_floor TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_schema_migration PRIMARY KEY (version),
  CONSTRAINT ck_schema_migration_version_range CHECK (version >= 1),
  CONSTRAINT ck_schema_migration_binary_version_length CHECK (length(binary_version) BETWEEN 1 AND 128),
  CONSTRAINT ck_schema_migration_compatibility_floor_length CHECK (length(compatibility_floor) BETWEEN 1 AND 128),
  CONSTRAINT ck_schema_migration_checksum_length CHECK (length(checksum) = 64)
) STRICT;


CREATE TABLE worker (
  id BLOB NOT NULL,
  role TEXT NOT NULL,
  host TEXT NOT NULL,
  version TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  heartbeat_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_worker PRIMARY KEY (id),
  CONSTRAINT ck_worker_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_worker_role CHECK (role IN ('STANDALONE', 'API', 'SCHEDULER', 'WORKER')),
  CONSTRAINT ck_worker_host_length CHECK (length(host) BETWEEN 1 AND 255),
  CONSTRAINT ck_worker_version_length CHECK (length(version) BETWEEN 1 AND 128)
) STRICT;


CREATE TABLE org (
  id BLOB NOT NULL,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_org PRIMARY KEY (id),
  CONSTRAINT ck_org_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_org_name_length CHECK (length(name) BETWEEN 1 AND 200),
  CONSTRAINT ck_org_status CHECK (status IN ('ACTIVE', 'SUSPENDED', 'DELETING'))
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

-- Exactly one SYSTEM principal per installation (IN-12).
CREATE UNIQUE INDEX uq_principal_kind ON principal (kind) WHERE kind = 'SYSTEM';
