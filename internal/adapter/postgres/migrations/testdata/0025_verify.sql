-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0024 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 24) <> 1;
SELECT 'column: org.time_saved_minutes_per_execution is missing' WHERE NOT EXISTS (
  SELECT 1 FROM information_schema.columns WHERE table_name = 'org' AND column_name = 'time_saved_minutes_per_execution');
SELECT 'index: ix_external_operation_org_id_state_execution_id is missing' WHERE NOT EXISTS (
  SELECT 1 FROM pg_indexes WHERE indexname = 'ix_external_operation_org_id_state_execution_id');
