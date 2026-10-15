-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind rebuild
-- orcon:compatibility-floor self

-- PostgreSQL alters the constraints in place; the SQLite file of this pair rebuilds
-- the same two tables. Both statements take an ACCESS EXCLUSIVE lock on the table
-- for their duration (small tables at this stage; the runner sets lock_timeout).

-- flow_data gains the namespace (ADR-0008 section 4.4). Existing rows become PRODUCTION;
-- the uniqueness of a data key is per namespace, so a test run never shadows a production value.
ALTER TABLE flow_data ADD COLUMN namespace text NOT NULL DEFAULT 'PRODUCTION';
ALTER TABLE flow_data ADD CONSTRAINT ck_flow_data_namespace CHECK (namespace IN ('PRODUCTION', 'TEST'));
ALTER TABLE flow_data DROP CONSTRAINT uq_flow_data_org_id_flow_id_data_key;
ALTER TABLE flow_data ADD CONSTRAINT uq_flow_data_org_id_flow_id_namespace_data_key UNIQUE (org_id, flow_id, namespace, data_key);

-- wait accepts EFFECT_CONFIRMATION (SPEC-014 section 15, ADR-0010 section 8).
ALTER TABLE wait DROP CONSTRAINT ck_wait_kind;
ALTER TABLE wait ADD CONSTRAINT ck_wait_kind CHECK (kind IN ('TIMER', 'SIGNAL', 'HUMAN', 'CHILD', 'EFFECT_CONFIRMATION'));
