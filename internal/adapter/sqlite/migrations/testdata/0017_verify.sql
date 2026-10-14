-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- Runs right after migration 0017 and before its seed: the rows of the earlier seeds must survive it.
SELECT 'schema_migration: migration 0017 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 17) <> 1;
SELECT 'role_binding: expected no binding before the seed of 0017' WHERE (SELECT count(*) FROM role_binding) <> 0;
SELECT 'custom_role: expected no custom role before the seed of 0017' WHERE (SELECT count(*) FROM custom_role) <> 0;
SELECT 'org: expected every organization at epoch zero' WHERE (SELECT count(*) FROM org WHERE authorization_epoch <> 0) <> 0;
