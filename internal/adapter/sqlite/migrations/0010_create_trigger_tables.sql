-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The trigger registry of ADR-0020, ADR-0021 section 5 and ADR-0024 (SPEC-002 cut T1). The table
-- the ADR calls "trigger" is flow_trigger: trigger is an SQLite keyword and orcon-db NM-12 forbids it
-- as an identifier.

-- Whether the automatic triggers of a flow accept events (ADR-0024 section 1). New flows are inactive.
ALTER TABLE flow ADD COLUMN activation_state TEXT NOT NULL DEFAULT 'INACTIVE' CONSTRAINT ck_flow_activation_state CHECK (activation_state IN ('ACTIVE', 'INACTIVE'));

CREATE TABLE flow_trigger (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  -- Identifier of the node inside the flow document; stable across versions.
  node_id TEXT NOT NULL,
  family TEXT NOT NULL,
  type TEXT NOT NULL,
  configuration_hash TEXT NOT NULL,
  -- The version this entry reflects.
  flow_version_id BLOB NOT NULL,
  -- ACTIVE accepts events, INACTIVE belongs to an inactive or deleted flow, REMOVED left the live version.
  status TEXT NOT NULL,
  health TEXT NOT NULL,
  last_error TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_flow_trigger PRIMARY KEY (id),
  CONSTRAINT uq_flow_trigger_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_flow_trigger_org_id_flow_id_node_id UNIQUE (org_id, flow_id, node_id),
  CONSTRAINT ck_flow_trigger_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_flow_trigger_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_flow_trigger_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_flow_trigger_flow_version_id_length CHECK (length(flow_version_id) = 16),
  CONSTRAINT ck_flow_trigger_node_id_length CHECK (length(node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_flow_trigger_family CHECK (family IN ('WEBHOOK', 'APP_EVENT', 'SCHEDULE', 'POLL', 'LISTEN', 'INTERNAL')),
  CONSTRAINT ck_flow_trigger_type_length CHECK (length(type) BETWEEN 1 AND 200),
  CONSTRAINT ck_flow_trigger_configuration_hash_length CHECK (length(configuration_hash) = 64),
  CONSTRAINT ck_flow_trigger_status CHECK (status IN ('ACTIVE', 'INACTIVE', 'REMOVED')),
  CONSTRAINT ck_flow_trigger_health CHECK (health IN ('OK', 'PENDING', 'DEGRADED', 'FAILING')),
  CONSTRAINT ck_flow_trigger_last_error_length CHECK (length(last_error) BETWEEN 1 AND 2000),
  CONSTRAINT fk_flow_trigger_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_trigger_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_flow_trigger_flow_version FOREIGN KEY (org_id, flow_version_id) REFERENCES flow_version (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports fk_flow_trigger_flow_version and the delete of its parent row.
CREATE INDEX ix_flow_trigger_org_id_flow_version_id ON flow_trigger (org_id, flow_version_id);

CREATE TABLE trigger_address (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  flow_trigger_id BLOB NOT NULL,
  family TEXT NOT NULL,
  scope TEXT NOT NULL,
  -- The full path with a leading slash; segments that start with a colon are route parameters.
  pattern TEXT NOT NULL,
  -- The part of pattern before the first parameter.
  static_prefix TEXT NOT NULL,
  -- A dedicated ingress host; NULL for the shared one.
  host TEXT,
  -- The unguessable last segment of a native address; NULL when there is none.
  random_suffix TEXT,
  response_profile TEXT NOT NULL,
  status TEXT NOT NULL,
  -- Set exactly when status is RETIRED: the address answers 410 for a grace period, then 404.
  retired_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_trigger_address PRIMARY KEY (id),
  CONSTRAINT uq_trigger_address_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_trigger_address_org_id_flow_trigger_id_scope_pattern UNIQUE (org_id, flow_trigger_id, scope, pattern),
  CONSTRAINT ck_trigger_address_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_trigger_address_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_trigger_address_flow_trigger_id_length CHECK (length(flow_trigger_id) = 16),
  CONSTRAINT ck_trigger_address_family CHECK (family IN ('N8N', 'NATIVE')),
  CONSTRAINT ck_trigger_address_scope CHECK (scope IN ('PRODUCTION', 'TEST')),
  CONSTRAINT ck_trigger_address_pattern_length CHECK (length(pattern) BETWEEN 2 AND 1000),
  CONSTRAINT ck_trigger_address_static_prefix_length CHECK (length(static_prefix) BETWEEN 1 AND 1000),
  CONSTRAINT ck_trigger_address_host_length CHECK (length(host) BETWEEN 1 AND 253),
  CONSTRAINT ck_trigger_address_random_suffix_length CHECK (length(random_suffix) BETWEEN 22 AND 64),
  CONSTRAINT ck_trigger_address_response_profile CHECK (response_profile IN ('NATIVE', 'N8N')),
  CONSTRAINT ck_trigger_address_status CHECK (status IN ('ACTIVE', 'RETIRED')),
  CONSTRAINT ck_trigger_address_retired_at CHECK ((status = 'RETIRED') = (retired_at IS NOT NULL)),
  CONSTRAINT fk_trigger_address_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_trigger_address_flow_trigger FOREIGN KEY (org_id, flow_trigger_id) REFERENCES flow_trigger (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports the collision check inside an organization (ListTriggerRoutes) and the route index of ingress.
CREATE INDEX ix_trigger_address_org_id_scope_pattern ON trigger_address (org_id, scope, pattern);

-- Supports the question across organizations "is this path taken" (ExistRouteTakenByOtherOrganization), which
-- cannot lead with org_id because it looks at every other organization.
CREATE INDEX ix_trigger_address_scope_pattern ON trigger_address (scope, pattern);

CREATE TABLE trigger_address_method (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  trigger_address_id BLOB NOT NULL,
  method TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_trigger_address_method PRIMARY KEY (id),
  CONSTRAINT uq_trigger_address_method_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_trigger_address_method_org_id_trigger_address_id_method UNIQUE (org_id, trigger_address_id, method),
  CONSTRAINT ck_trigger_address_method_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_trigger_address_method_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_trigger_address_method_trigger_address_id_length CHECK (length(trigger_address_id) = 16),
  CONSTRAINT ck_trigger_address_method_method CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD')),
  CONSTRAINT fk_trigger_address_method_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_trigger_address_method_trigger_address FOREIGN KEY (org_id, trigger_address_id) REFERENCES trigger_address (org_id, id) ON DELETE CASCADE
) STRICT;

CREATE TABLE trigger_event (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  occurred_at INTEGER NOT NULL,
  flow_id BLOB NOT NULL,
  -- NULL for an event about the whole flow.
  node_id TEXT,
  -- The event type keeps its dotted form (ADR-0080 section 3), for example flow.published.
  type TEXT NOT NULL,
  schema_version INTEGER NOT NULL,
  data_document BLOB NOT NULL,
  data_document_format TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_trigger_event PRIMARY KEY (id),
  CONSTRAINT uq_trigger_event_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_trigger_event_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_trigger_event_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_trigger_event_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_trigger_event_node_id_length CHECK (length(node_id) BETWEEN 1 AND 64),
  CONSTRAINT ck_trigger_event_type_length CHECK (length(type) BETWEEN 1 AND 100),
  CONSTRAINT ck_trigger_event_schema_version_range CHECK (schema_version >= 1),
  CONSTRAINT ck_trigger_event_data_document_format CHECK (data_document_format IN ('orcon/trigger-event@1')),
  CONSTRAINT fk_trigger_event_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_trigger_event_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
) STRICT;

-- Supports ListTriggerEvents: keyset by id within a flow, and fk_trigger_event_flow.
CREATE INDEX ix_trigger_event_org_id_flow_id_id ON trigger_event (org_id, flow_id, id);
