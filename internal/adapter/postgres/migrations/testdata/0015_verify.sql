-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0012 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 12) <> 1;
