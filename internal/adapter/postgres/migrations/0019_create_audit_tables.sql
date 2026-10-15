-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- Audit chains (SPEC-004 R-AUD-01..03, ADR-0048). The same shape as the SQLite migration of this number: one chain
-- per organization (org_id NOT NULL, row-level security) and one for the instance. Event and block rows are
-- append-only by grant: the application and engine roles may insert and read them and never update or delete
-- them. The retention design (R-AUD-07) adds the one pruning path when it lands.

CREATE TABLE audit_pending (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  occurred_at timestamptz NOT NULL,
  actor_principal_id uuid,
  actor_kind text,
  action_name text NOT NULL,
  outcome text NOT NULL,
  body_document bytea NOT NULL,
  body_document_format text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_audit_pending PRIMARY KEY (id),
  CONSTRAINT ck_audit_pending_actor_kind CHECK (actor_kind IS NULL OR actor_kind IN ('USER', 'SERVICE_ACCOUNT', 'SCIM_CLIENT', 'API_KEY', 'SYSTEM')),
  CONSTRAINT ck_audit_pending_actor_pair CHECK ((actor_principal_id IS NULL) = (actor_kind IS NULL)),
  CONSTRAINT ck_audit_pending_action_name_length CHECK (char_length(action_name) BETWEEN 1 AND 100),
  CONSTRAINT ck_audit_pending_outcome CHECK (outcome IN ('SUCCESS', 'FAILURE', 'DENIED')),
  CONSTRAINT ck_audit_pending_body_length CHECK (octet_length(body_document) BETWEEN 1 AND 65536),
  CONSTRAINT ck_audit_pending_format CHECK (body_document_format = 'orcon/audit@1'),
  -- org is an instance table, so this foreign key is single-column (TI-7).
  CONSTRAINT fk_audit_pending_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

CREATE INDEX ix_audit_pending_org_id_occurred_at_id ON audit_pending (org_id, occurred_at, id);

ALTER TABLE audit_pending ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_pending FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON audit_pending FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
-- The sealer deletes the rows it has sealed; nothing else changes a pending row.
GRANT SELECT, INSERT, DELETE ON audit_pending TO orcon_application;
GRANT SELECT, INSERT, DELETE ON audit_pending TO orcon_engine;

CREATE TABLE audit_event (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  sequence_number bigint NOT NULL,
  occurred_at timestamptz NOT NULL,
  actor_principal_id uuid,
  actor_kind text,
  action_name text NOT NULL,
  outcome text NOT NULL,
  body_document bytea NOT NULL,
  body_document_format text NOT NULL,
  previous_hash bytea NOT NULL,
  hash bytea NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_audit_event PRIMARY KEY (id),
  CONSTRAINT uq_audit_event_org_id_sequence_number UNIQUE (org_id, sequence_number),
  CONSTRAINT ck_audit_event_sequence CHECK (sequence_number >= 1),
  CONSTRAINT ck_audit_event_actor_kind CHECK (actor_kind IS NULL OR actor_kind IN ('USER', 'SERVICE_ACCOUNT', 'SCIM_CLIENT', 'API_KEY', 'SYSTEM')),
  CONSTRAINT ck_audit_event_actor_pair CHECK ((actor_principal_id IS NULL) = (actor_kind IS NULL)),
  CONSTRAINT ck_audit_event_action_name_length CHECK (char_length(action_name) BETWEEN 1 AND 100),
  CONSTRAINT ck_audit_event_outcome CHECK (outcome IN ('SUCCESS', 'FAILURE', 'DENIED')),
  CONSTRAINT ck_audit_event_body_length CHECK (octet_length(body_document) BETWEEN 1 AND 65536),
  CONSTRAINT ck_audit_event_format CHECK (body_document_format = 'orcon/audit@1'),
  CONSTRAINT ck_audit_event_previous_hash_length CHECK (octet_length(previous_hash) = 32),
  CONSTRAINT ck_audit_event_hash_length CHECK (octet_length(hash) = 32),
  CONSTRAINT fk_audit_event_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE RESTRICT
);

CREATE TABLE audit_block (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  first_sequence_number bigint NOT NULL,
  last_sequence_number bigint NOT NULL,
  head_hash bytea NOT NULL,
  mac bytea NOT NULL,
  key_version bigint NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_audit_block PRIMARY KEY (id),
  CONSTRAINT uq_audit_block_org_id_last_sequence_number UNIQUE (org_id, last_sequence_number),
  CONSTRAINT ck_audit_block_range CHECK (first_sequence_number >= 1 AND last_sequence_number >= first_sequence_number),
  CONSTRAINT ck_audit_block_head_length CHECK (octet_length(head_hash) = 32),
  CONSTRAINT ck_audit_block_mac_length CHECK (octet_length(mac) = 32),
  CONSTRAINT ck_audit_block_key_version CHECK (key_version >= 1),
  CONSTRAINT fk_audit_block_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE RESTRICT
);

ALTER TABLE audit_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_event FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON audit_event FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
ALTER TABLE audit_block ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_block FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON audit_block FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT ON audit_event TO orcon_application;
GRANT SELECT, INSERT ON audit_event TO orcon_engine;
GRANT SELECT, INSERT ON audit_block TO orcon_application;
GRANT SELECT, INSERT ON audit_block TO orcon_engine;

CREATE TABLE instance_audit_pending (
  id uuid NOT NULL,
  occurred_at timestamptz NOT NULL,
  actor_principal_id uuid,
  actor_kind text,
  action_name text NOT NULL,
  outcome text NOT NULL,
  body_document bytea NOT NULL,
  body_document_format text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_instance_audit_pending PRIMARY KEY (id),
  CONSTRAINT ck_instance_audit_pending_actor_kind CHECK (actor_kind IS NULL OR actor_kind IN ('USER', 'SERVICE_ACCOUNT', 'SCIM_CLIENT', 'API_KEY', 'SYSTEM')),
  CONSTRAINT ck_instance_audit_pending_actor_pair CHECK ((actor_principal_id IS NULL) = (actor_kind IS NULL)),
  CONSTRAINT ck_instance_audit_pending_action_name_length CHECK (char_length(action_name) BETWEEN 1 AND 100),
  CONSTRAINT ck_instance_audit_pending_outcome CHECK (outcome IN ('SUCCESS', 'FAILURE', 'DENIED')),
  CONSTRAINT ck_instance_audit_pending_body_length CHECK (octet_length(body_document) BETWEEN 1 AND 65536),
  CONSTRAINT ck_instance_audit_pending_format CHECK (body_document_format = 'orcon/audit@1')
);

CREATE INDEX ix_instance_audit_pending_occurred_at_id ON instance_audit_pending (occurred_at, id);

GRANT SELECT, INSERT, DELETE ON instance_audit_pending TO orcon_application;
GRANT SELECT, INSERT, DELETE ON instance_audit_pending TO orcon_engine;

CREATE TABLE instance_audit_event (
  id uuid NOT NULL,
  sequence_number bigint NOT NULL,
  occurred_at timestamptz NOT NULL,
  actor_principal_id uuid,
  actor_kind text,
  action_name text NOT NULL,
  outcome text NOT NULL,
  body_document bytea NOT NULL,
  body_document_format text NOT NULL,
  previous_hash bytea NOT NULL,
  hash bytea NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_instance_audit_event PRIMARY KEY (id),
  CONSTRAINT uq_instance_audit_event_sequence_number UNIQUE (sequence_number),
  CONSTRAINT ck_instance_audit_event_sequence CHECK (sequence_number >= 1),
  CONSTRAINT ck_instance_audit_event_actor_kind CHECK (actor_kind IS NULL OR actor_kind IN ('USER', 'SERVICE_ACCOUNT', 'SCIM_CLIENT', 'API_KEY', 'SYSTEM')),
  CONSTRAINT ck_instance_audit_event_actor_pair CHECK ((actor_principal_id IS NULL) = (actor_kind IS NULL)),
  CONSTRAINT ck_instance_audit_event_action_name_length CHECK (char_length(action_name) BETWEEN 1 AND 100),
  CONSTRAINT ck_instance_audit_event_outcome CHECK (outcome IN ('SUCCESS', 'FAILURE', 'DENIED')),
  CONSTRAINT ck_instance_audit_event_body_length CHECK (octet_length(body_document) BETWEEN 1 AND 65536),
  CONSTRAINT ck_instance_audit_event_format CHECK (body_document_format = 'orcon/audit@1'),
  CONSTRAINT ck_instance_audit_event_previous_hash_length CHECK (octet_length(previous_hash) = 32),
  CONSTRAINT ck_instance_audit_event_hash_length CHECK (octet_length(hash) = 32)
);

CREATE TABLE instance_audit_block (
  id uuid NOT NULL,
  first_sequence_number bigint NOT NULL,
  last_sequence_number bigint NOT NULL,
  head_hash bytea NOT NULL,
  mac bytea NOT NULL,
  key_version bigint NOT NULL,
  created_at timestamptz NOT NULL,
  CONSTRAINT pk_instance_audit_block PRIMARY KEY (id),
  CONSTRAINT uq_instance_audit_block_last_sequence_number UNIQUE (last_sequence_number),
  CONSTRAINT ck_instance_audit_block_range CHECK (first_sequence_number >= 1 AND last_sequence_number >= first_sequence_number),
  CONSTRAINT ck_instance_audit_block_head_length CHECK (octet_length(head_hash) = 32),
  CONSTRAINT ck_instance_audit_block_mac_length CHECK (octet_length(mac) = 32),
  CONSTRAINT ck_instance_audit_block_key_version CHECK (key_version >= 1)
);

GRANT SELECT, INSERT ON instance_audit_event TO orcon_application;
GRANT SELECT, INSERT ON instance_audit_event TO orcon_engine;
GRANT SELECT, INSERT ON instance_audit_block TO orcon_application;
GRANT SELECT, INSERT ON instance_audit_block TO orcon_engine;

-- The organizations that have pending audit rows, for the sealer (SPEC-004 R-AUD-03). The sealer then seals each
-- chain under its own organization context; this function is the only cross-organization read it needs.
CREATE FUNCTION orcon_engine.list_audit_pending_orgs(p_row_limit integer)
RETURNS TABLE (org_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  SELECT DISTINCT audit_pending.org_id
  FROM audit_pending
  ORDER BY audit_pending.org_id
  LIMIT LEAST(p_row_limit, 1000)
$$;

ALTER FUNCTION orcon_engine.list_audit_pending_orgs(integer) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.list_audit_pending_orgs(integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.list_audit_pending_orgs(integer) TO orcon_application;
