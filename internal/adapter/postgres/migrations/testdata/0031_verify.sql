-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- Runs right after migration 0031 and before its seed: the credential tables start empty.
SELECT 'schema_migration: migration 0031 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 31) <> 1;
SELECT 'credential: expected no credential before the seed of 0031' WHERE (SELECT count(*) FROM credential) <> 0;
SELECT 'credential_version: expected no version before the seed of 0031' WHERE (SELECT count(*) FROM credential_version) <> 0;
SELECT 'credential_share: expected no share before the seed of 0031' WHERE (SELECT count(*) FROM credential_share) <> 0;
