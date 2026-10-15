-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- Audit chains (SPEC-004 R-AUD-01..03, ADR-0048). Two chains: one per organization (org_id NOT NULL) and one for
-- the instance (no org_id, ADR-0077 section 5). Actions insert a pending row in their own transaction; the sealer
-- turns pending rows into events with a sequence number and a hash link, and seals each batch with a MAC derived
-- from the chain's key. Event rows are append-only: the application role may insert and read them, never change
-- or delete them (append_only_tables.txt).
--
-- Times are microseconds since the Unix epoch, as in the other tables. actor_principal_id has no foreign key: the
-- audit history outlives a deleted principal (ADR-0077 section 8). actor_kind is NULL only for an anonymous
-- attempt (a failed sign-in naming no known user); the five kinds of orcon/audit@1 are the only other values.
-- action_name is the action of orcon/audit@1 (the name "action" is reserved in SQLite and PostgreSQL).

CREATE TABLE audit_pending (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  occurred_at INTEGER NOT NULL,
  actor_principal_id BLOB,
  actor_kind TEXT,
  action_name TEXT NOT NULL,
  outcome TEXT NOT NULL,
  body_document BLOB NOT NULL,
  body_document_format TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_audit_pending PRIMARY KEY (id),
  CONSTRAINT ck_audit_pending_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_audit_pending_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_audit_pending_actor_principal_length CHECK (actor_principal_id IS NULL OR length(actor_principal_id) = 16),
  CONSTRAINT ck_audit_pending_actor_kind CHECK (actor_kind IS NULL OR actor_kind IN ('USER', 'SERVICE_ACCOUNT', 'SCIM_CLIENT', 'API_KEY', 'SYSTEM')),
  CONSTRAINT ck_audit_pending_actor_pair CHECK ((actor_principal_id IS NULL) = (actor_kind IS NULL)),
  CONSTRAINT ck_audit_pending_action_name_length CHECK (length(action_name) BETWEEN 1 AND 100),
  CONSTRAINT ck_audit_pending_outcome CHECK (outcome IN ('SUCCESS', 'FAILURE', 'DENIED')),
  CONSTRAINT ck_audit_pending_body_length CHECK (length(body_document) BETWEEN 1 AND 65536),
  CONSTRAINT ck_audit_pending_format CHECK (body_document_format = 'orcon/audit@1'),
  CONSTRAINT fk_audit_pending_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

CREATE INDEX ix_audit_pending_org_id_occurred_at_id ON audit_pending (org_id, occurred_at, id);

CREATE TABLE audit_event (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  sequence_number INTEGER NOT NULL,
  occurred_at INTEGER NOT NULL,
  actor_principal_id BLOB,
  actor_kind TEXT,
  action_name TEXT NOT NULL,
  outcome TEXT NOT NULL,
  body_document BLOB NOT NULL,
  body_document_format TEXT NOT NULL,
  previous_hash BLOB NOT NULL,
  hash BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_audit_event PRIMARY KEY (id),
  CONSTRAINT uq_audit_event_org_id_sequence_number UNIQUE (org_id, sequence_number),
  CONSTRAINT ck_audit_event_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_audit_event_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_audit_event_sequence CHECK (sequence_number >= 1),
  CONSTRAINT ck_audit_event_actor_principal_length CHECK (actor_principal_id IS NULL OR length(actor_principal_id) = 16),
  CONSTRAINT ck_audit_event_actor_kind CHECK (actor_kind IS NULL OR actor_kind IN ('USER', 'SERVICE_ACCOUNT', 'SCIM_CLIENT', 'API_KEY', 'SYSTEM')),
  CONSTRAINT ck_audit_event_actor_pair CHECK ((actor_principal_id IS NULL) = (actor_kind IS NULL)),
  CONSTRAINT ck_audit_event_action_name_length CHECK (length(action_name) BETWEEN 1 AND 100),
  CONSTRAINT ck_audit_event_outcome CHECK (outcome IN ('SUCCESS', 'FAILURE', 'DENIED')),
  CONSTRAINT ck_audit_event_body_length CHECK (length(body_document) BETWEEN 1 AND 65536),
  CONSTRAINT ck_audit_event_format CHECK (body_document_format = 'orcon/audit@1'),
  CONSTRAINT ck_audit_event_previous_hash_length CHECK (length(previous_hash) = 32),
  CONSTRAINT ck_audit_event_hash_length CHECK (length(hash) = 32),
  CONSTRAINT fk_audit_event_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE RESTRICT
) STRICT;

-- A block records one sealed batch: its sequence range, the hash that ends it and the MAC over that head.
-- key_version names the organization key version whose derived key made the MAC.
CREATE TABLE audit_block (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  first_sequence_number INTEGER NOT NULL,
  last_sequence_number INTEGER NOT NULL,
  head_hash BLOB NOT NULL,
  mac BLOB NOT NULL,
  key_version INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_audit_block PRIMARY KEY (id),
  CONSTRAINT uq_audit_block_org_id_last_sequence_number UNIQUE (org_id, last_sequence_number),
  CONSTRAINT ck_audit_block_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_audit_block_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_audit_block_range CHECK (first_sequence_number >= 1 AND last_sequence_number >= first_sequence_number),
  CONSTRAINT ck_audit_block_head_length CHECK (length(head_hash) = 32),
  CONSTRAINT ck_audit_block_mac_length CHECK (length(mac) = 32),
  CONSTRAINT ck_audit_block_key_version CHECK (key_version >= 1),
  CONSTRAINT fk_audit_block_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE RESTRICT
) STRICT;

CREATE TABLE instance_audit_pending (
  id BLOB NOT NULL,
  occurred_at INTEGER NOT NULL,
  actor_principal_id BLOB,
  actor_kind TEXT,
  action_name TEXT NOT NULL,
  outcome TEXT NOT NULL,
  body_document BLOB NOT NULL,
  body_document_format TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_instance_audit_pending PRIMARY KEY (id),
  CONSTRAINT ck_instance_audit_pending_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_instance_audit_pending_actor_principal_length CHECK (actor_principal_id IS NULL OR length(actor_principal_id) = 16),
  CONSTRAINT ck_instance_audit_pending_actor_kind CHECK (actor_kind IS NULL OR actor_kind IN ('USER', 'SERVICE_ACCOUNT', 'SCIM_CLIENT', 'API_KEY', 'SYSTEM')),
  CONSTRAINT ck_instance_audit_pending_actor_pair CHECK ((actor_principal_id IS NULL) = (actor_kind IS NULL)),
  CONSTRAINT ck_instance_audit_pending_action_name_length CHECK (length(action_name) BETWEEN 1 AND 100),
  CONSTRAINT ck_instance_audit_pending_outcome CHECK (outcome IN ('SUCCESS', 'FAILURE', 'DENIED')),
  CONSTRAINT ck_instance_audit_pending_body_length CHECK (length(body_document) BETWEEN 1 AND 65536),
  CONSTRAINT ck_instance_audit_pending_format CHECK (body_document_format = 'orcon/audit@1')
) STRICT;

CREATE INDEX ix_instance_audit_pending_occurred_at_id ON instance_audit_pending (occurred_at, id);

CREATE TABLE instance_audit_event (
  id BLOB NOT NULL,
  sequence_number INTEGER NOT NULL,
  occurred_at INTEGER NOT NULL,
  actor_principal_id BLOB,
  actor_kind TEXT,
  action_name TEXT NOT NULL,
  outcome TEXT NOT NULL,
  body_document BLOB NOT NULL,
  body_document_format TEXT NOT NULL,
  previous_hash BLOB NOT NULL,
  hash BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_instance_audit_event PRIMARY KEY (id),
  CONSTRAINT uq_instance_audit_event_sequence_number UNIQUE (sequence_number),
  CONSTRAINT ck_instance_audit_event_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_instance_audit_event_sequence CHECK (sequence_number >= 1),
  CONSTRAINT ck_instance_audit_event_actor_principal_length CHECK (actor_principal_id IS NULL OR length(actor_principal_id) = 16),
  CONSTRAINT ck_instance_audit_event_actor_kind CHECK (actor_kind IS NULL OR actor_kind IN ('USER', 'SERVICE_ACCOUNT', 'SCIM_CLIENT', 'API_KEY', 'SYSTEM')),
  CONSTRAINT ck_instance_audit_event_actor_pair CHECK ((actor_principal_id IS NULL) = (actor_kind IS NULL)),
  CONSTRAINT ck_instance_audit_event_action_name_length CHECK (length(action_name) BETWEEN 1 AND 100),
  CONSTRAINT ck_instance_audit_event_outcome CHECK (outcome IN ('SUCCESS', 'FAILURE', 'DENIED')),
  CONSTRAINT ck_instance_audit_event_body_length CHECK (length(body_document) BETWEEN 1 AND 65536),
  CONSTRAINT ck_instance_audit_event_format CHECK (body_document_format = 'orcon/audit@1'),
  CONSTRAINT ck_instance_audit_event_previous_hash_length CHECK (length(previous_hash) = 32),
  CONSTRAINT ck_instance_audit_event_hash_length CHECK (length(hash) = 32)
) STRICT;

CREATE TABLE instance_audit_block (
  id BLOB NOT NULL,
  first_sequence_number INTEGER NOT NULL,
  last_sequence_number INTEGER NOT NULL,
  head_hash BLOB NOT NULL,
  mac BLOB NOT NULL,
  key_version INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT pk_instance_audit_block PRIMARY KEY (id),
  CONSTRAINT uq_instance_audit_block_last_sequence_number UNIQUE (last_sequence_number),
  CONSTRAINT ck_instance_audit_block_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_instance_audit_block_range CHECK (first_sequence_number >= 1 AND last_sequence_number >= first_sequence_number),
  CONSTRAINT ck_instance_audit_block_head_length CHECK (length(head_hash) = 32),
  CONSTRAINT ck_instance_audit_block_mac_length CHECK (length(mac) = 32),
  CONSTRAINT ck_instance_audit_block_key_version CHECK (key_version >= 1)
) STRICT;
