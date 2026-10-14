-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0015 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 15) <> 1;
SELECT 'function: orcon_engine.resolve_user_organizations is missing' WHERE NOT EXISTS (
  SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
  WHERE n.nspname = 'orcon_engine' AND p.proname = 'resolve_user_organizations');
