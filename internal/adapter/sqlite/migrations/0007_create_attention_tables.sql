-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE attention_request (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  execution_id BLOB NOT NULL,
  project_id BLOB NOT NULL,
  flow_id BLOB NOT NULL,
  flow_version_id BLOB,
  node_id TEXT NOT NULL,
  token_id BLOB,
  cause TEXT NOT NULL,
  cause_reference TEXT NOT NULL,
  notify_class TEXT NOT NULL,
  status TEXT NOT NULL,
  level INTEGER NOT NULL,
  group_key TEXT NOT NULL,
  item_count INTEGER NOT NULL,
  summary TEXT NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  acknowledged_by_principal_id BLOB,
  acknowledged_at INTEGER,
  next_reminder_at INTEGER,
  next_escalation_at INTEGER,
  resolved_at INTEGER,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  resolved_by_principal_id BLOB,
  resolution TEXT,
  -- ACTION is a reserved word; the column holds the id of the action that closed the request.
  resolution_action TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_attention_request PRIMARY KEY (id),
  CONSTRAINT uq_attention_request_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_attention_request_org_id_cause_reference UNIQUE (org_id, cause_reference),
  CONSTRAINT ck_attention_request_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_attention_request_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_attention_request_execution_id_length CHECK (length(execution_id) = 16),
  CONSTRAINT ck_attention_request_project_id_length CHECK (length(project_id) = 16),
  CONSTRAINT ck_attention_request_flow_id_length CHECK (length(flow_id) = 16),
  CONSTRAINT ck_attention_request_flow_version_id_length CHECK (length(flow_version_id) = 16),
  CONSTRAINT ck_attention_request_node_id_length CHECK (length(node_id) BETWEEN 1 AND 200),
  CONSTRAINT ck_attention_request_token_id_length CHECK (length(token_id) = 16),
  CONSTRAINT ck_attention_request_cause CHECK (cause IN ('UNCERTAIN_OPERATION', 'POISON_TOKEN', 'COMPENSATION_REVIEW', 'AI_BUDGET_SUSPENDED', 'HUMAN_TASK', 'END_USER_WAIT')),
  CONSTRAINT ck_attention_request_cause_reference_length CHECK (length(cause_reference) BETWEEN 1 AND 200),
  CONSTRAINT ck_attention_request_notify_class CHECK (notify_class IN ('NOTIFY', 'SILENT')),
  CONSTRAINT ck_attention_request_status CHECK (status IN ('OPEN', 'RESOLVED', 'EXPIRED', 'CANCELED')),
  CONSTRAINT ck_attention_request_level_range CHECK (level >= 0),
  CONSTRAINT ck_attention_request_group_key_length CHECK (length(group_key) BETWEEN 1 AND 500),
  CONSTRAINT ck_attention_request_item_count_range CHECK (item_count >= 0),
  CONSTRAINT ck_attention_request_summary_length CHECK (length(summary) BETWEEN 1 AND 2000),
  CONSTRAINT ck_attention_request_acknowledged_by_principal_id_length CHECK (length(acknowledged_by_principal_id) = 16),
  CONSTRAINT ck_attention_request_resolved_by_principal_id_length CHECK (length(resolved_by_principal_id) = 16),
  CONSTRAINT ck_attention_request_resolution CHECK (resolution IN ('ACTION', 'TIMEOUT', 'CANCELLATION')),
  CONSTRAINT ck_attention_request_resolution_action_length CHECK (length(resolution_action) BETWEEN 1 AND 64),
  CONSTRAINT fk_attention_request_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_flow FOREIGN KEY (org_id, flow_id) REFERENCES flow (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_flow_version FOREIGN KEY (org_id, flow_version_id) REFERENCES flow_version (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_token FOREIGN KEY (org_id, token_id) REFERENCES token (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_attention_request_acknowledged_by_principal FOREIGN KEY (acknowledged_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL,
  CONSTRAINT fk_attention_request_resolved_by_principal FOREIGN KEY (resolved_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

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

CREATE TABLE attention_routing (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  scope_kind TEXT NOT NULL,
  project_id BLOB,
  flow_id BLOB,
  -- ANY applies to every cause that has no override.
  cause TEXT NOT NULL,
  recipient_document BLOB NOT NULL,
  recipient_document_format TEXT NOT NULL,
  escalation_document BLOB,
  escalation_document_format TEXT,
  reminder_document BLOB,
  reminder_document_format TEXT,
  coalesce_window_milliseconds INTEGER NOT NULL,
  overdue_after_milliseconds INTEGER,
  revision INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_attention_routing PRIMARY KEY (id),
  CONSTRAINT uq_attention_routing_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_attention_routing_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_attention_routing_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_attention_routing_scope_kind CHECK (scope_kind IN ('ORGANIZATION', 'PROJECT', 'FLOW')),
  CONSTRAINT ck_attention_routing_project_id_length CHECK (length(project_id) = 16),
  CONSTRAINT ck_attention_routing_flow_id_length CHECK (length(flow_id) = 16),
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
) STRICT;

-- One routing per cause at organization scope.
CREATE UNIQUE INDEX uq_attention_routing_org_id_cause ON attention_routing (org_id, cause) WHERE scope_kind = 'ORGANIZATION';

-- One routing per cause at project scope.
CREATE UNIQUE INDEX uq_attention_routing_org_id_project_id_cause ON attention_routing (org_id, project_id, cause) WHERE scope_kind = 'PROJECT';

-- One routing per cause at flow scope.
CREATE UNIQUE INDEX uq_attention_routing_org_id_flow_id_cause ON attention_routing (org_id, flow_id, cause) WHERE scope_kind = 'FLOW';

CREATE TABLE notification_delivery (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  deduplication_key TEXT NOT NULL,
  channel TEXT NOT NULL,
  recipient TEXT NOT NULL,
  status TEXT NOT NULL,
  attempt_count INTEGER NOT NULL,
  next_attempt_at INTEGER,
  last_error TEXT,
  finished_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_notification_delivery PRIMARY KEY (id),
  CONSTRAINT uq_notification_delivery_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_notification_delivery_org_id_deduplication_key UNIQUE (org_id, deduplication_key),
  CONSTRAINT ck_notification_delivery_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_notification_delivery_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_notification_delivery_deduplication_key_length CHECK (length(deduplication_key) BETWEEN 1 AND 500),
  CONSTRAINT ck_notification_delivery_channel CHECK (channel IN ('IN_APP', 'EMAIL', 'WEBHOOK')),
  CONSTRAINT ck_notification_delivery_recipient_length CHECK (length(recipient) BETWEEN 1 AND 500),
  CONSTRAINT ck_notification_delivery_status CHECK (status IN ('PENDING', 'SENT', 'FAILED', 'DEAD')),
  CONSTRAINT ck_notification_delivery_attempt_count_range CHECK (attempt_count >= 0),
  CONSTRAINT ck_notification_delivery_last_error_length CHECK (length(last_error) BETWEEN 1 AND 2000),
  CONSTRAINT fk_notification_delivery_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;


CREATE TABLE notification_group (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  group_key TEXT NOT NULL,
  channel TEXT NOT NULL,
  window_ends_at INTEGER NOT NULL,
  pending_count INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_notification_group PRIMARY KEY (id),
  CONSTRAINT uq_notification_group_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_notification_group_org_id_group_key_channel UNIQUE (org_id, group_key, channel),
  CONSTRAINT ck_notification_group_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_notification_group_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_notification_group_group_key_length CHECK (length(group_key) BETWEEN 1 AND 500),
  CONSTRAINT ck_notification_group_channel CHECK (channel IN ('IN_APP', 'EMAIL', 'WEBHOOK')),
  CONSTRAINT ck_notification_group_pending_count_range CHECK (pending_count >= 0),
  CONSTRAINT fk_notification_group_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;


CREATE TABLE operational_alert (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  kind TEXT NOT NULL,
  subject_reference TEXT NOT NULL,
  status TEXT NOT NULL,
  level INTEGER NOT NULL,
  summary TEXT NOT NULL,
  affected_reference_document BLOB,
  affected_reference_document_format TEXT,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  acknowledged_by_principal_id BLOB,
  acknowledged_at INTEGER,
  next_escalation_at INTEGER,
  closed_at INTEGER,
  closed_reason TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_operational_alert PRIMARY KEY (id),
  CONSTRAINT uq_operational_alert_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_operational_alert_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_operational_alert_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_operational_alert_kind CHECK (kind IN ('CONNECTOR_REVOKED')),
  CONSTRAINT ck_operational_alert_subject_reference_length CHECK (length(subject_reference) BETWEEN 1 AND 500),
  CONSTRAINT ck_operational_alert_status CHECK (status IN ('OPEN', 'CLOSED')),
  CONSTRAINT ck_operational_alert_level_range CHECK (level >= 0),
  CONSTRAINT ck_operational_alert_summary_length CHECK (length(summary) BETWEEN 1 AND 2000),
  CONSTRAINT ck_operational_alert_affected_reference_document_pair CHECK ((affected_reference_document IS NULL) = (affected_reference_document_format IS NULL)),
  CONSTRAINT ck_operational_alert_affected_reference_document_format CHECK (affected_reference_document_format IN ('orcon/alert-reference@1')),
  CONSTRAINT ck_operational_alert_acknowledged_by_principal_id_length CHECK (length(acknowledged_by_principal_id) = 16),
  CONSTRAINT ck_operational_alert_closed_reason_length CHECK (length(closed_reason) BETWEEN 1 AND 500),
  CONSTRAINT fk_operational_alert_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_operational_alert_acknowledged_by_principal FOREIGN KEY (acknowledged_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
) STRICT;

-- Supports fk_operational_alert_acknowledged_by_principal and the delete of its parent row.
CREATE INDEX ix_operational_alert_acknowledged_by_principal_id ON operational_alert (acknowledged_by_principal_id) WHERE acknowledged_by_principal_id IS NOT NULL;
