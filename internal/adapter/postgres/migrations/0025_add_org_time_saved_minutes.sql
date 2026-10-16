-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- Minutes a person saves per successful automated execution (SPEC-019 R-UX-BE-02). The activity summary
-- multiplies them by the executions of the last week. Every existing organization gets the default of 3;
-- the update of the value arrives with updateOrganization (SPEC-005, F-05).
ALTER TABLE org ADD COLUMN time_saved_minutes_per_execution integer NOT NULL DEFAULT 3;
ALTER TABLE org ADD CONSTRAINT ck_org_time_saved_minutes_per_execution_range CHECK (time_saved_minutes_per_execution BETWEEN 0 AND 600);

-- Supports the count of executions with an uncertain operation in the activity summary and in the
-- uncertain filter. Only the UNCERTAIN rows are indexed; they are few compared with all external operations.
-- The index of (org_id, execution_id) already exists, so this one adds the state to the key and keeps the
-- partial predicate.
CREATE INDEX ix_external_operation_org_id_state_execution_id ON external_operation (org_id, state, execution_id) WHERE state = 'UNCERTAIN';
