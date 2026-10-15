-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0012 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 12) <> 1;
SELECT 'principal: expected the principals of both organizations and both users' WHERE (SELECT count(*) FROM principal) <> 4;
SELECT 'application_user: expected one row per organization' WHERE (SELECT count(*) FROM application_user) <> 2;
SELECT 'membership: expected one row per organization' WHERE (SELECT count(*) FROM membership) <> 2;
SELECT 'project: expected one row per organization' WHERE (SELECT count(*) FROM project) <> 2;
SELECT 'project: a project written before handles has no handle' FROM project WHERE handle IS NULL;
SELECT 'project: the backfilled handle of a unique slug is the slug' FROM project WHERE handle <> slug;
