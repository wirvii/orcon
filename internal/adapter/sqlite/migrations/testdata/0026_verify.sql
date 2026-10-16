-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- Runs right after migration 0025 and before its seed: the backfill counted the executions of the earlier
-- seeds (0003: one QUEUED MANUAL execution in each of two organizations at 1760000000000000 microseconds;
-- 0024: a SUCCEEDED TRIGGER one at 1759997700000000 and a FAILED MANUAL one at 1760053500000000 in the
-- first organization). 1759996800000000 is the start of the first hour and 1760050800000000 of the second.
SELECT 'schema_migration: migration 0025 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 25) <> 1;
SELECT 'table: execution_hour_count is missing' WHERE NOT EXISTS (SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'execution_hour_count');
SELECT 'backfill: expected 6 hourly rows, found ' || count(*) FROM execution_hour_count HAVING count(*) <> 6;
SELECT 'backfill: the total of the counts is not 6' WHERE (SELECT COALESCE(sum(execution_count), 0) FROM execution_hour_count) <> 6;
SELECT 'backfill: every execution must be counted once as ACCEPTED' WHERE (SELECT COALESCE(sum(execution_count), 0) FROM execution_hour_count WHERE status = 'ACCEPTED') <> (SELECT count(*) FROM execution);
SELECT 'backfill: the first hour of MANUAL is wrong' WHERE (
  SELECT count(*) FROM execution_hour_count WHERE hour_started_at = 1759996800000000 AND mode = 'MANUAL' AND status = 'ACCEPTED' AND execution_count = 1) <> 2;
SELECT 'backfill: the first hour of TRIGGER is wrong' WHERE (
  SELECT count(*) FROM execution_hour_count WHERE hour_started_at = 1759996800000000 AND mode = 'TRIGGER' AND status IN ('ACCEPTED', 'SUCCEEDED') AND execution_count = 1) <> 2;
SELECT 'backfill: the second hour of MANUAL is wrong' WHERE (
  SELECT count(*) FROM execution_hour_count WHERE hour_started_at = 1760050800000000 AND mode = 'MANUAL' AND status IN ('ACCEPTED', 'FAILED') AND execution_count = 1) <> 2;
