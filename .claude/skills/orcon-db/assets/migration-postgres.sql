-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor <semver>
-- Template (references/migrations.md MG-14): copy to
-- internal/adapter/postgres/migrations/NNNN_<verb>_<table>.sql together with the
-- SQLite file of the same number and name. Keep the header values identical in
-- both files (MG-2): kind is additive, expand, contract, rebuild or data;
-- compatibility-floor is a semver or self; add "-- orcon:contracts NNNN" only to
-- a contract migration, and "-- orcon:no-transaction" only to a migration that
-- holds exactly one CREATE INDEX CONCURRENTLY IF NOT EXISTS (MG-9). Replace
-- "example" with the singular table name, write every word out (NM-3), and
-- delete these instruction lines.
CREATE TABLE example (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  status text NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  created_by_principal_id uuid,
  CONSTRAINT pk_example PRIMARY KEY (id),
  CONSTRAINT uq_example_org_id_id UNIQUE (org_id, id),
  -- Enumerated values are UPPER_SNAKE, the spelling the API exposes (NM-16, IN-3, CK-24).
  CONSTRAINT ck_example_status CHECK (status IN ('ACTIVE', 'ARCHIVED')),
  -- org and principal are instance tables, so these foreign keys are single-column (TI-7).
  CONSTRAINT fk_example_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_example_created_by_principal FOREIGN KEY (created_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports the foreign key fk_example_created_by_principal and the batched
-- clearing of the column when a principal is deleted (IN-12).
CREATE INDEX ix_example_created_by_principal_id ON example (created_by_principal_id);

-- Every index names the query it supports in a comment like this one (QY-7);
-- request-plane list indexes lead with org_id.
-- Supports ListExamples.
-- CREATE INDEX ix_example_org_id_created_at ON example (org_id, created_at);

-- Row-level security on every organization-scoped table (TI-4, MG-13). The
-- policy text is exact; do not reword it.
ALTER TABLE example ENABLE ROW LEVEL SECURITY;
ALTER TABLE example FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON example FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));

-- Runtime role grants (TI-5). An append-only table grants SELECT, INSERT only.
GRANT SELECT, INSERT, UPDATE, DELETE ON example TO orcon_application;
