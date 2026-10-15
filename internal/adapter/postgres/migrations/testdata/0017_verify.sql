-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0017 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 17) <> 1;
SELECT 'clear_principal_references: does not clear an actor column of the editor tables' WHERE (SELECT count(*) FROM (VALUES ('node_test_data'), ('flow_node_pin'), ('test_run'), ('effect_preview'), ('flow_editing_session'), ('flow')) AS t(tbl) WHERE position(tbl IN pg_get_functiondef('orcon_engine.clear_principal_references(uuid, integer)'::regprocedure)) = 0) > 0;
SELECT 'clear_principal_references: the engine role cannot update node_test_data.created_by_principal_id' WHERE NOT has_column_privilege('orcon_engine', 'node_test_data', 'created_by_principal_id', 'UPDATE');
SELECT 'clear_principal_references: the engine role cannot update test_run.started_by_principal_id' WHERE NOT has_column_privilege('orcon_engine', 'test_run', 'started_by_principal_id', 'UPDATE');
