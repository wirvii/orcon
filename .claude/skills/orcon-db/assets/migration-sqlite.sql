-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor <semver>
-- Template (references/migrations.md MG-14): copy to
-- internal/adapter/sqlite/migrations/NNNN_<verb>_<table>.sql together with the
-- PostgreSQL file of the same number and name. Keep the header values identical
-- in both files (MG-2): kind is additive, expand, contract, rebuild or data;
-- compatibility-floor is a semver or self; add "-- orcon:contracts NNNN" only to
-- a contract migration. Replace "example" with the singular table name, write
-- every word out (NM-3), and delete these instruction lines.
CREATE TABLE example (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  status TEXT NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  created_by_principal_id BLOB,
  CONSTRAINT pk_example PRIMARY KEY (id),
  CONSTRAINT uq_example_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_example_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_example_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_example_created_by_principal_id_length CHECK (length(created_by_principal_id) = 16),
  -- Enumerated values are UPPER_SNAKE, the spelling the API exposes (NM-16, IN-3, CK-24).
  CONSTRAINT ck_example_status CHECK (status IN ('ACTIVE', 'ARCHIVED')),
  -- org and principal are instance tables, so these foreign keys are single-column (TI-7).
  CONSTRAINT fk_example_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_example_created_by_principal FOREIGN KEY (created_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports the foreign key fk_example_created_by_principal and the batched
-- clearing of the column when a principal is deleted (IN-12).
CREATE INDEX ix_example_created_by_principal_id ON example (created_by_principal_id);

-- Every index names the query it supports in a comment like this one (QY-7);
-- request-plane list indexes lead with org_id.
-- Supports ListExamples.
-- CREATE INDEX ix_example_org_id_created_at ON example (org_id, created_at);
