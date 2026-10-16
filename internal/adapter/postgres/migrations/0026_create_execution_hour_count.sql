-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- Hourly counters of the executions of a project (SPEC-019 R-UX-BE-02): the activity summary reads these
-- rows instead of scanning the executions. The row ACCEPTED counts the executions accepted in the hour;
-- each terminal status counts the executions accepted in the hour that currently have that status. Each
-- row is written in the transaction of the change it counts (acceptance, terminal transition or reopening
-- of a terminal execution), so it never drifts from a recount of the execution table after a crash or a
-- retry. The id of a row written by the adapter is derived from the natural key; the rows of the backfill
-- take a random id. The upsert and the range reads use the unique key.
CREATE TABLE execution_hour_count (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  project_id uuid NOT NULL,
  -- Start of the hour (UTC), a whole hour.
  hour_started_at timestamptz NOT NULL,
  mode text NOT NULL,
  status text NOT NULL,
  execution_count bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_execution_hour_count PRIMARY KEY (id),
  -- Columns: org_id, project_id, hour_started_at, mode, status (the name would exceed the identifier limit).
  CONSTRAINT uq_execution_hour_count_bucket UNIQUE (org_id, project_id, hour_started_at, mode, status),
  CONSTRAINT ck_execution_hour_count_mode CHECK (mode IN ('MANUAL', 'TRIGGER', 'TEST', 'CHILD', 'RETRY')),
  CONSTRAINT ck_execution_hour_count_status CHECK (status IN ('ACCEPTED', 'SUCCEEDED', 'FAILED', 'CANCELED')),
  CONSTRAINT ck_execution_hour_count_execution_count_range CHECK (execution_count >= 0),
  CONSTRAINT fk_execution_hour_count_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_hour_count_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE
);

-- Backfill of the executions that exist, as the owner: the execution table is lifted from forced row-level
-- security for these reads and restored at once (the same pattern as migration 0012). The migration is one
-- transaction; the statements hold the lock of the executions for their duration. The hour is truncated in
-- UTC explicitly: date_trunc on a timestamptz uses the time zone of the session, which on a zone with a
-- half-hour offset would cut at the wrong instant.
ALTER TABLE execution NO FORCE ROW LEVEL SECURITY;
-- The id of a backfilled row is the one the adapter derives from the natural key (orcon_engine.derive_uuid
-- is the SQL form of id.Derive), and its timestamps are the first creation and the last update of the
-- executions it counts.
INSERT INTO execution_hour_count (id, org_id, project_id, hour_started_at, mode, status, execution_count, created_at, updated_at)
SELECT orcon_engine.derive_uuid('execution_hour_count', g.org_id::text, g.project_id::text, (EXTRACT(EPOCH FROM g.hour_started_at) * 1000000)::bigint::text, g.mode, 'ACCEPTED'),
       g.org_id, g.project_id, g.hour_started_at, g.mode, 'ACCEPTED', g.execution_count, g.first_created_at, g.last_updated_at
FROM (
  SELECT org_id, project_id, date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS hour_started_at, mode,
         COUNT(*) AS execution_count, min(created_at) AS first_created_at, max(updated_at) AS last_updated_at
  FROM execution
  GROUP BY org_id, project_id, date_trunc('hour', created_at AT TIME ZONE 'UTC'), mode
) AS g;

INSERT INTO execution_hour_count (id, org_id, project_id, hour_started_at, mode, status, execution_count, created_at, updated_at)
SELECT orcon_engine.derive_uuid('execution_hour_count', g.org_id::text, g.project_id::text, (EXTRACT(EPOCH FROM g.hour_started_at) * 1000000)::bigint::text, g.mode, g.status),
       g.org_id, g.project_id, g.hour_started_at, g.mode, g.status, g.execution_count, g.first_created_at, g.last_updated_at
FROM (
  SELECT org_id, project_id, date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS hour_started_at, mode, status,
         COUNT(*) AS execution_count, min(created_at) AS first_created_at, max(updated_at) AS last_updated_at
  FROM execution
  WHERE status IN ('SUCCEEDED', 'FAILED', 'CANCELED')
  GROUP BY org_id, project_id, date_trunc('hour', created_at AT TIME ZONE 'UTC'), mode, status
) AS g;
ALTER TABLE execution FORCE ROW LEVEL SECURITY;

ALTER TABLE execution_hour_count ENABLE ROW LEVEL SECURITY;
ALTER TABLE execution_hour_count FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON execution_hour_count FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON execution_hour_count TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON execution_hour_count TO orcon_engine;
