-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0003 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 3) <> 1;
SELECT 'flow: a flow seeded before activation_state existed is not INACTIVE' WHERE (SELECT count(*) FROM flow WHERE activation_state <> 'INACTIVE') <> 0;
SELECT 'project: expected one row per organization' WHERE (SELECT count(*) FROM project) <> 2;
SELECT 'flow: expected one row per organization' WHERE (SELECT count(*) FROM flow) <> 2;
SELECT 'flow_version: expected one row per organization' WHERE (SELECT count(*) FROM flow_version) <> 2;
