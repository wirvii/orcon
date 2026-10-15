-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The idempotency records of the API (ADR-0053 section 4, SPEC-005 Amendment 8 section 8-2). A change that takes an
-- Idempotency-Key claims its record in the transaction of the change, so a committed change always has its record;
-- the record is completed with the stored response after the response is built. A record expires after 24 hours.
CREATE TABLE idempotency_record (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  requester text NOT NULL,
  idempotency_key text NOT NULL,
  fingerprint bytea NOT NULL,
  status text NOT NULL,
  resource_reference text,
  response_status integer,
  response_content_type text,
  response_location text,
  response_etag text,
  response_body bytea,
  is_secret_issued boolean NOT NULL DEFAULT false,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_idempotency_record PRIMARY KEY (id),
  CONSTRAINT uq_idempotency_record_org_id_requester_idempotency_key UNIQUE (org_id, requester, idempotency_key),
  CONSTRAINT ck_idempotency_record_requester_length CHECK (char_length(requester) BETWEEN 1 AND 300),
  CONSTRAINT ck_idempotency_record_idempotency_key_length CHECK (char_length(idempotency_key) BETWEEN 1 AND 255),
  CONSTRAINT ck_idempotency_record_fingerprint_length CHECK (octet_length(fingerprint) = 32),
  CONSTRAINT ck_idempotency_record_status CHECK (status IN ('IN_PROGRESS', 'COMPLETED')),
  CONSTRAINT ck_idempotency_record_resource_reference_length CHECK (resource_reference IS NULL OR char_length(resource_reference) BETWEEN 1 AND 300),
  CONSTRAINT ck_idempotency_record_completion CHECK ((status = 'COMPLETED' AND response_status IS NOT NULL) OR (status = 'IN_PROGRESS' AND response_status IS NULL)),
  CONSTRAINT fk_idempotency_record_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

CREATE INDEX ix_idempotency_record_expires_at ON idempotency_record (expires_at);

ALTER TABLE idempotency_record ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_record FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON idempotency_record FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON idempotency_record TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON idempotency_record TO orcon_engine;
