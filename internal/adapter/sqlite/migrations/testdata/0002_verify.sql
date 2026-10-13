-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0002 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 2) <> 1;
SELECT 'worker: expected one row per organization' WHERE (SELECT count(*) FROM worker) <> 2;
SELECT 'org: expected one row per organization' WHERE (SELECT count(*) FROM org) <> 2;
SELECT 'principal: expected one row per organization' WHERE (SELECT count(*) FROM principal) <> 2;
