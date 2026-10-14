-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0009 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 9) <> 1;
SELECT 'project: expected one row per organization' WHERE (SELECT count(*) FROM project) <> 2;
SELECT 'flow: expected one row per organization' WHERE (SELECT count(*) FROM flow) <> 2;
SELECT 'flow_version: expected one row per organization' WHERE (SELECT count(*) FROM flow_version) <> 2;
SELECT 'flow_snapshot: expected one row per organization' WHERE (SELECT count(*) FROM flow_snapshot) <> 2;
