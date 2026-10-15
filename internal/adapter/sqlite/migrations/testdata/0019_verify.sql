-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- Runs right after migration 0019 and before its seed: the audit tables start empty.
SELECT 'schema_migration: migration 0019 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 19) <> 1;
SELECT 'audit_event: expected no event before the seed of 0019' WHERE (SELECT count(*) FROM audit_event) <> 0;
SELECT 'instance_audit_pending: expected no pending entry before the seed of 0019' WHERE (SELECT count(*) FROM instance_audit_pending) <> 0;
