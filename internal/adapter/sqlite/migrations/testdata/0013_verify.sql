-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0013 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 13) <> 1;
SELECT 'project: expected two rows per organization' WHERE (SELECT count(*) FROM project) <> 4;
SELECT 'membership: expected one row per organization' WHERE (SELECT count(*) FROM membership) <> 2;
SELECT 'application_user: expected one row per organization' WHERE (SELECT count(*) FROM application_user) <> 2;
