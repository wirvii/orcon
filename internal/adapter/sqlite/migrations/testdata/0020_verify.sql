-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- Runs right after migration 0020 and before its seed: the records of the earlier seeds must survive it.
SELECT 'schema_migration: migration 0020 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 20) <> 1;
SELECT 'idempotency_record: expected no record before the seed of 0020' WHERE (SELECT count(*) FROM idempotency_record) <> 0;
