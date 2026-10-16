-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0024 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 24) <> 1;
SELECT 'column: org.time_saved_minutes_per_execution is missing' WHERE (SELECT count(*) FROM pragma_table_info('org') WHERE name = 'time_saved_minutes_per_execution') <> 1;
SELECT 'index: ix_external_operation_org_id_state_execution_id is missing' WHERE NOT EXISTS (
  SELECT 1 FROM sqlite_master WHERE type = 'index' AND name = 'ix_external_operation_org_id_state_execution_id');
