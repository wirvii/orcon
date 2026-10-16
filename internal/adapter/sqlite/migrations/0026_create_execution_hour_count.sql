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
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  project_id BLOB NOT NULL,
  -- Start of the hour, in microseconds since the Unix epoch (UTC); a whole hour.
  hour_started_at INTEGER NOT NULL,
  mode TEXT NOT NULL,
  status TEXT NOT NULL,
  execution_count INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_execution_hour_count PRIMARY KEY (id),
  -- Columns: org_id, project_id, hour_started_at, mode, status (the name would exceed the identifier limit).
  CONSTRAINT uq_execution_hour_count_bucket UNIQUE (org_id, project_id, hour_started_at, mode, status),
  CONSTRAINT ck_execution_hour_count_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_execution_hour_count_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_execution_hour_count_project_id_length CHECK (length(project_id) = 16),
  CONSTRAINT ck_execution_hour_count_hour_started_at_whole_hour CHECK (hour_started_at >= 0 AND hour_started_at % 3600000000 = 0),
  CONSTRAINT ck_execution_hour_count_mode CHECK (mode IN ('MANUAL', 'TRIGGER', 'TEST', 'CHILD', 'RETRY')),
  CONSTRAINT ck_execution_hour_count_status CHECK (status IN ('ACCEPTED', 'SUCCEEDED', 'FAILED', 'CANCELED')),
  CONSTRAINT ck_execution_hour_count_execution_count_range CHECK (execution_count >= 0),
  CONSTRAINT fk_execution_hour_count_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_execution_hour_count_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE
) STRICT;

-- Backfill of the executions that exist: the accepted count and the terminal counts of every hour. The
-- hours are whole hours of the UTC epoch, so the result does not depend on any session setting.
INSERT INTO execution_hour_count (id, org_id, project_id, hour_started_at, mode, status, execution_count, created_at, updated_at)
SELECT randomblob(16), org_id, project_id, (created_at / 3600000000) * 3600000000, mode, 'ACCEPTED', COUNT(*), min(created_at), max(updated_at)
FROM execution
GROUP BY org_id, project_id, created_at / 3600000000, mode;

INSERT INTO execution_hour_count (id, org_id, project_id, hour_started_at, mode, status, execution_count, created_at, updated_at)
SELECT randomblob(16), org_id, project_id, (created_at / 3600000000) * 3600000000, mode, status, COUNT(*), min(created_at), max(updated_at)
FROM execution
WHERE status IN ('SUCCEEDED', 'FAILED', 'CANCELED')
GROUP BY org_id, project_id, created_at / 3600000000, mode, status;
