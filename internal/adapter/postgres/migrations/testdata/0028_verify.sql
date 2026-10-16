-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
SELECT 'flow: expected one row per organization after the editor tables are added' WHERE (SELECT count(*) FROM flow) <> 2;
SELECT 'flow_version: expected one row per organization after the editor tables are added' WHERE (SELECT count(*) FROM flow_version) <> 2;
SELECT 'flow: rows seeded before the migration must have no draft save time' WHERE (SELECT count(*) FROM flow WHERE draft_updated_at IS NOT NULL) <> 0;
