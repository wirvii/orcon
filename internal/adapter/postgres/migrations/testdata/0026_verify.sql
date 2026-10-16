-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- Runs right after migration 0025 and before its seed: the backfill counted the executions of the earlier
-- seeds (0003: one QUEUED MANUAL execution in each of two organizations at 2026-10-09T08:53:20Z; 0024: a
-- SUCCEEDED TRIGGER one at 08:15:00Z and a FAILED MANUAL one at 23:45:00Z in the first organization).
SELECT 'schema_migration: migration 0025 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 25) <> 1;
SELECT 'table: execution_hour_count is missing' WHERE NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'execution_hour_count');
SELECT 'backfill: expected 6 hourly rows, found ' || count(*) FROM execution_hour_count HAVING count(*) <> 6;
SELECT 'backfill: the total of the counts is not 6' WHERE (SELECT COALESCE(sum(execution_count), 0) FROM execution_hour_count) <> 6;
SELECT 'backfill: every execution must be counted once as ACCEPTED' WHERE (SELECT COALESCE(sum(execution_count), 0) FROM execution_hour_count WHERE status = 'ACCEPTED') <> (SELECT count(*) FROM execution);
SELECT 'backfill: the 08:00Z hour of MANUAL is not counted in UTC' WHERE (
  SELECT count(*) FROM execution_hour_count WHERE hour_started_at = timestamptz '2026-10-09T08:00:00Z' AND mode = 'MANUAL' AND status = 'ACCEPTED' AND execution_count = 1) <> 2;
SELECT 'backfill: the 08:00Z hour of TRIGGER is not counted in UTC' WHERE (
  SELECT count(*) FROM execution_hour_count WHERE hour_started_at = timestamptz '2026-10-09T08:00:00Z' AND mode = 'TRIGGER' AND status IN ('ACCEPTED', 'SUCCEEDED') AND execution_count = 1) <> 2;
SELECT 'backfill: the 23:00Z hour of MANUAL is not counted in UTC' WHERE (
  SELECT count(*) FROM execution_hour_count WHERE hour_started_at = timestamptz '2026-10-09T23:00:00Z' AND mode = 'MANUAL' AND status IN ('ACCEPTED', 'FAILED') AND execution_count = 1) <> 2;
SELECT 'backfill: a row has no id or timestamps' WHERE EXISTS (SELECT 1 FROM execution_hour_count WHERE id IS NULL OR created_at IS NULL OR updated_at IS NULL);
