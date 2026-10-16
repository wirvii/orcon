-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0027 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 27) <> 1;
SELECT 'flow: every flow that existed before starts at revision 0' WHERE EXISTS (SELECT 1 FROM flow WHERE revision <> 0);
SELECT 'flow_version: a version that existed before has no note and no rollback origin' WHERE EXISTS (SELECT 1 FROM flow_version WHERE publication_note IS NOT NULL OR rolled_back_from_version_number IS NOT NULL);
