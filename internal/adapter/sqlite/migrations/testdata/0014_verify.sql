-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- Runs right after migration 0014 and before its seed: the rows of the earlier seeds must survive it.
SELECT 'schema_migration: migration 0014 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 14) <> 1;
SELECT 'session: expected no session before the seed of 0014' WHERE (SELECT count(*) FROM session) <> 0;
SELECT 'login_throttle: expected no identifier before the seed of 0014' WHERE (SELECT count(*) FROM login_throttle) <> 0;
SELECT 'application_user: expected no password hash before the seed of 0014' WHERE (SELECT count(*) FROM application_user WHERE password_hash IS NOT NULL) <> 0;
