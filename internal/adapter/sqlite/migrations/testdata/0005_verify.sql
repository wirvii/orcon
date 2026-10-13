-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'schema_migration: migration 0005 is not recorded exactly once' WHERE (SELECT count(*) FROM schema_migration WHERE version = 5) <> 1;
SELECT 'worker: expected one row per organization' WHERE (SELECT count(*) FROM worker) <> 2;
SELECT 'org: expected one row per organization' WHERE (SELECT count(*) FROM org) <> 2;
SELECT 'principal: expected one row per organization' WHERE (SELECT count(*) FROM principal) <> 2;
SELECT 'project: expected one row per organization' WHERE (SELECT count(*) FROM project) <> 2;
SELECT 'flow: expected one row per organization' WHERE (SELECT count(*) FROM flow) <> 2;
SELECT 'flow_version: expected one row per organization' WHERE (SELECT count(*) FROM flow_version) <> 2;
SELECT 'flow_snapshot: expected one row per organization' WHERE (SELECT count(*) FROM flow_snapshot) <> 2;
SELECT 'instance_alert: expected one row per organization' WHERE (SELECT count(*) FROM instance_alert) <> 2;
SELECT 'call_edge: expected one row per organization' WHERE (SELECT count(*) FROM call_edge) <> 2;
SELECT 'execution: expected one row per organization' WHERE (SELECT count(*) FROM execution) <> 2;
SELECT 'token: expected one row per organization' WHERE (SELECT count(*) FROM token) <> 2;
SELECT 'node_execution: expected one row per organization' WHERE (SELECT count(*) FROM node_execution) <> 2;
SELECT 'task_attempt: expected one row per organization' WHERE (SELECT count(*) FROM task_attempt) <> 2;
SELECT 'blob: expected one row per organization' WHERE (SELECT count(*) FROM blob) <> 2;
SELECT 'blob_chunk: expected one row per organization' WHERE (SELECT count(*) FROM blob_chunk) <> 2;
SELECT 'node_output: expected one row per organization' WHERE (SELECT count(*) FROM node_output) <> 2;
SELECT 'external_operation: expected one row per organization' WHERE (SELECT count(*) FROM external_operation) <> 2;
