-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0011 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 11) <> 1;
SELECT 'worker: expected one row per organization' WHERE (SELECT count(*) FROM worker) <> 2;
SELECT 'org: expected one row per organization' WHERE (SELECT count(*) FROM org) <> 2;
SELECT 'principal: expected one row per organization' WHERE (SELECT count(*) FROM principal) <> 2;
SELECT 'project: expected one row per organization' WHERE (SELECT count(*) FROM project) <> 2;
SELECT 'flow: expected one row per organization' WHERE (SELECT count(*) FROM flow) <> 2;
SELECT 'flow_version: expected one row per organization' WHERE (SELECT count(*) FROM flow_version) <> 2;
SELECT 'flow_snapshot: expected one row per organization' WHERE (SELECT count(*) FROM flow_snapshot) <> 2;
