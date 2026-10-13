-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE attention_request (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  project_id uuid NOT NULL,
  flow_id uuid NOT NULL,
  flow_version_id uuid,
  node_id text NOT NULL,
  token_id uuid,
  cause text NOT NULL,
  cause_reference text NOT NULL,
  notify_class text NOT NULL,
  status text NOT NULL,
  level bigint NOT NULL,
  group_key text NOT NULL,
  item_count bigint NOT NULL,
  summary text NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  acknowledged_by_principal_id uuid,
  acknowledged_at timestamptz,
  next_reminder_at timestamptz,
  next_escalation_at timestamptz,
  resolved_at timestamptz,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  resolved_by_principal_id uuid,
  resolution text,
  -- ACTION is a reserved word; the column holds the id of the action that closed the request.
  resolution_action text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_attention_request PRIMARY KEY (id),
  CONSTRAINT uq_attention_request_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_attention_request_org_id_cause_reference UNIQUE (org_id, cause_reference),
  CONSTRAINT ck_attention_request_node_id_length CHECK (char_length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_attention_request_cause CHECK (cause IN ('UNCERTAIN_OPERATION', 'POISON_TOKEN', 'COMPENSATION_REVIEW', 'AI_BUDGET_SUSPENDED', 'HUMAN_TASK', 'END_USER_WAIT')),
  CONSTRAINT ck_attention_request_cause_reference_length CHECK (char_length(cause_reference) BETWEEN 1 AND 200),
  CONSTRAINT ck_attention_request_notify_class CHECK (notify_class IN ('NOTIFY', 'SILENT')),
  CONSTRAINT ck_attention_request_status CHECK (status IN ('OPEN', 'RESOLVED', 'EXPIRED', 'CANCELED')),
  CONSTRAINT ck_attention_request_level_range CHECK (level >= 0),
  CONSTRAINT ck_attention_request_group_key_length CHECK (char_length(group_key) BETWEEN 1 AND 500),
  CONSTRAINT ck_attention_request_item_count_range CHECK (item_count >= 0),
  CONSTRAINT ck_attention_request_summary_length CHECK (char_length(summary) BETWEEN 1 AND 2000),
  CONSTRAINT ck_attention_request_resolution CHECK (resolution IN ('ACTION', 'TIMEOUT', 'CANCELLATION')),
  CONSTRAINT ck_attention_request_resolution_action_length CHECK (char_length(resolution_action) BETWEEN 1 AND 64),
  CONSTRAINT fk_attention_request_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_flow_version FOREIGN KEY (org_id, flow_version_id) REFERENCES flow_version (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_token FOREIGN KEY (org_id, token_id) REFERENCES token (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_acknowledged_by_principal FOREIGN KEY (acknowledged_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL,
  CONSTRAINT fk_attention_request_resolved_by_principal FOREIGN KEY (resolved_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports ListAttentionRequests (the inbox, keyset by age).
CREATE INDEX ix_attention_request_org_id_status_created_at_id ON attention_request (org_id, status, created_at, id);

-- Supports fk_attention_request_execution and the delete of its parent row.
CREATE INDEX ix_attention_request_org_id_execution_id ON attention_request (org_id, execution_id);

-- Supports fk_attention_request_project and the delete of its parent row.
CREATE INDEX ix_attention_request_org_id_project_id ON attention_request (org_id, project_id);

-- Supports fk_attention_request_flow and the delete of its parent row.
CREATE INDEX ix_attention_request_org_id_flow_id ON attention_request (org_id, flow_id);

-- Supports fk_attention_request_flow_version and the delete of its parent row.
CREATE INDEX ix_attention_request_org_id_flow_version_id ON attention_request (org_id, flow_version_id) WHERE flow_version_id IS NOT NULL;

-- Supports fk_attention_request_token and the delete of its parent row.
CREATE INDEX ix_attention_request_org_id_token_id ON attention_request (org_id, token_id) WHERE token_id IS NOT NULL;

-- Supports fk_attention_request_acknowledged_by_principal and the delete of its parent row.
CREATE INDEX ix_attention_request_acknowledged_by_principal_id ON attention_request (acknowledged_by_principal_id) WHERE acknowledged_by_principal_id IS NOT NULL;

-- Supports fk_attention_request_resolved_by_principal and the delete of its parent row.
CREATE INDEX ix_attention_request_resolved_by_principal_id ON attention_request (resolved_by_principal_id) WHERE resolved_by_principal_id IS NOT NULL;

ALTER TABLE attention_request ENABLE ROW LEVEL SECURITY;
ALTER TABLE attention_request FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON attention_request FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON attention_request TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON attention_request TO orcon_engine;

CREATE TABLE attention_routing (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  scope_kind text NOT NULL,
  project_id uuid,
  flow_id uuid,
  -- ANY applies to every cause that has no override.
  cause text NOT NULL,
  recipient_document bytea NOT NULL,
  recipient_document_format text NOT NULL,
  escalation_document bytea,
  escalation_document_format text,
  reminder_document bytea,
  reminder_document_format text,
  coalesce_window_milliseconds bigint NOT NULL,
  overdue_after_milliseconds bigint,
  revision bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_attention_routing PRIMARY KEY (id),
  CONSTRAINT uq_attention_routing_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_attention_routing_scope_kind CHECK (scope_kind IN ('ORGANIZATION', 'PROJECT', 'FLOW')),
  CONSTRAINT ck_attention_routing_cause CHECK (cause IN ('UNCERTAIN_OPERATION', 'POISON_TOKEN', 'COMPENSATION_REVIEW', 'AI_BUDGET_SUSPENDED', 'HUMAN_TASK', 'END_USER_WAIT', 'ANY')),
  CONSTRAINT ck_attention_routing_recipient_document_format CHECK (recipient_document_format IN ('orcon/attention-recipients@1')),
  CONSTRAINT ck_attention_routing_escalation_document_pair CHECK ((escalation_document IS NULL) = (escalation_document_format IS NULL)),
  CONSTRAINT ck_attention_routing_escalation_document_format CHECK (escalation_document_format IN ('orcon/attention-escalation@1')),
  CONSTRAINT ck_attention_routing_reminder_document_pair CHECK ((reminder_document IS NULL) = (reminder_document_format IS NULL)),
  CONSTRAINT ck_attention_routing_reminder_document_format CHECK (reminder_document_format IN ('orcon/attention-reminder@1')),
  CONSTRAINT ck_attention_routing_coalesce_window_milliseconds_range CHECK (coalesce_window_milliseconds >= 0),
  CONSTRAINT ck_attention_routing_overdue_after_milliseconds_range CHECK (overdue_after_milliseconds >= 0),
  CONSTRAINT ck_attention_routing_revision_range CHECK (revision >= 0),
  CONSTRAINT ck_attention_routing_scope CHECK ((scope_kind = 'ORGANIZATION' AND project_id IS NULL AND flow_id IS NULL) OR (scope_kind = 'PROJECT' AND project_id IS NOT NULL AND flow_id IS NULL) OR (scope_kind = 'FLOW' AND flow_id IS NOT NULL AND project_id IS NULL)),
  CONSTRAINT fk_attention_routing_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_routing_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_routing_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE
);

-- One routing per cause at organization scope.
CREATE UNIQUE INDEX uq_attention_routing_org_id_cause ON attention_routing (org_id, cause) WHERE scope_kind = 'ORGANIZATION';

-- One routing per cause at project scope.
CREATE UNIQUE INDEX uq_attention_routing_org_id_project_id_cause ON attention_routing (org_id, project_id, cause) WHERE scope_kind = 'PROJECT';

-- One routing per cause at flow scope.
CREATE UNIQUE INDEX uq_attention_routing_org_id_flow_id_cause ON attention_routing (org_id, flow_id, cause) WHERE scope_kind = 'FLOW';

ALTER TABLE attention_routing ENABLE ROW LEVEL SECURITY;
ALTER TABLE attention_routing FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON attention_routing FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON attention_routing TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON attention_routing TO orcon_engine;

CREATE TABLE notification_delivery (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  deduplication_key text NOT NULL,
  channel text NOT NULL,
  recipient text NOT NULL,
  status text NOT NULL,
  attempt_count bigint NOT NULL,
  next_attempt_at timestamptz,
  last_error text,
  finished_at timestamptz,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_notification_delivery PRIMARY KEY (id),
  CONSTRAINT uq_notification_delivery_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_notification_delivery_org_id_deduplication_key UNIQUE (org_id, deduplication_key),
  CONSTRAINT ck_notification_delivery_deduplication_key_length CHECK (char_length(deduplication_key) BETWEEN 1 AND 500),
  CONSTRAINT ck_notification_delivery_channel CHECK (channel IN ('IN_APP', 'EMAIL', 'WEBHOOK')),
  CONSTRAINT ck_notification_delivery_recipient_length CHECK (char_length(recipient) BETWEEN 1 AND 500),
  CONSTRAINT ck_notification_delivery_status CHECK (status IN ('PENDING', 'SENT', 'FAILED', 'DEAD')),
  CONSTRAINT ck_notification_delivery_attempt_count_range CHECK (attempt_count >= 0),
  CONSTRAINT ck_notification_delivery_last_error_length CHECK (char_length(last_error) BETWEEN 1 AND 2000),
  CONSTRAINT fk_notification_delivery_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

ALTER TABLE notification_delivery ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_delivery FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON notification_delivery FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON notification_delivery TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON notification_delivery TO orcon_engine;

CREATE TABLE notification_group (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  group_key text NOT NULL,
  channel text NOT NULL,
  window_ends_at timestamptz NOT NULL,
  pending_count bigint NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_notification_group PRIMARY KEY (id),
  CONSTRAINT uq_notification_group_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_notification_group_org_id_group_key_channel UNIQUE (org_id, group_key, channel),
  CONSTRAINT ck_notification_group_group_key_length CHECK (char_length(group_key) BETWEEN 1 AND 500),
  CONSTRAINT ck_notification_group_channel CHECK (channel IN ('IN_APP', 'EMAIL', 'WEBHOOK')),
  CONSTRAINT ck_notification_group_pending_count_range CHECK (pending_count >= 0),
  CONSTRAINT fk_notification_group_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

ALTER TABLE notification_group ENABLE ROW LEVEL SECURITY;
ALTER TABLE notification_group FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON notification_group FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON notification_group TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON notification_group TO orcon_engine;

CREATE TABLE operational_alert (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  kind text NOT NULL,
  subject_reference text NOT NULL,
  status text NOT NULL,
  level bigint NOT NULL,
  summary text NOT NULL,
  affected_reference_document bytea,
  affected_reference_document_format text,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  acknowledged_by_principal_id uuid,
  acknowledged_at timestamptz,
  next_escalation_at timestamptz,
  closed_at timestamptz,
  closed_reason text,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_operational_alert PRIMARY KEY (id),
  CONSTRAINT uq_operational_alert_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_operational_alert_kind CHECK (kind IN ('CONNECTOR_REVOKED')),
  CONSTRAINT ck_operational_alert_subject_reference_length CHECK (char_length(subject_reference) BETWEEN 1 AND 500),
  CONSTRAINT ck_operational_alert_status CHECK (status IN ('OPEN', 'CLOSED')),
  CONSTRAINT ck_operational_alert_level_range CHECK (level >= 0),
  CONSTRAINT ck_operational_alert_summary_length CHECK (char_length(summary) BETWEEN 1 AND 2000),
  CONSTRAINT ck_operational_alert_affected_reference_document_pair CHECK ((affected_reference_document IS NULL) = (affected_reference_document_format IS NULL)),
  CONSTRAINT ck_operational_alert_affected_reference_document_format CHECK (affected_reference_document_format IN ('orcon/alert-reference@1')),
  CONSTRAINT ck_operational_alert_closed_reason_length CHECK (char_length(closed_reason) BETWEEN 1 AND 500),
  CONSTRAINT fk_operational_alert_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_operational_alert_acknowledged_by_principal FOREIGN KEY (acknowledged_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports fk_operational_alert_acknowledged_by_principal and the delete of its parent row.
CREATE INDEX ix_operational_alert_acknowledged_by_principal_id ON operational_alert (acknowledged_by_principal_id) WHERE acknowledged_by_principal_id IS NOT NULL;

ALTER TABLE operational_alert ENABLE ROW LEVEL SECURITY;
ALTER TABLE operational_alert FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON operational_alert FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON operational_alert TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON operational_alert TO orcon_engine;
