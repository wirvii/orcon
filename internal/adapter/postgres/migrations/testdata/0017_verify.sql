-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- Runs right after migration 0017 and before its seed: the rows of the earlier seeds must survive it.
SELECT 'schema_migration: migration 0017 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 17) <> 1;
SELECT 'instance_key: expected no key before the seed of 0017' WHERE (SELECT count(*) FROM instance_key) <> 0;
SELECT 'org_key: expected no key before the seed of 0017' WHERE (SELECT count(*) FROM org_key) <> 0;
