-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0024 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 24) <> 1;
