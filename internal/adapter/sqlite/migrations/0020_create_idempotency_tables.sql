-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The idempotency records of the API (ADR-0053 section 4, SPEC-005 Amendment 8 section 8-2). A change that takes an
-- Idempotency-Key claims its record in the transaction of the change, so a committed change always has its record;
-- the record is completed with the stored response after the response is built. A record expires after 24 hours.
CREATE TABLE idempotency_record (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  requester TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  fingerprint BLOB NOT NULL,
  status TEXT NOT NULL,
  resource_reference TEXT,
  response_status INTEGER,
  response_content_type TEXT,
  response_location TEXT,
  response_etag TEXT,
  response_body BLOB,
  is_secret_issued INTEGER NOT NULL DEFAULT 0,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_idempotency_record PRIMARY KEY (id),
  CONSTRAINT uq_idempotency_record_org_id_requester_idempotency_key UNIQUE (org_id, requester, idempotency_key),
  CONSTRAINT ck_idempotency_record_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_idempotency_record_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_idempotency_record_requester_length CHECK (length(requester) BETWEEN 1 AND 300),
  CONSTRAINT ck_idempotency_record_idempotency_key_length CHECK (length(idempotency_key) BETWEEN 1 AND 255),
  CONSTRAINT ck_idempotency_record_fingerprint_length CHECK (length(fingerprint) = 32),
  CONSTRAINT ck_idempotency_record_status CHECK (status IN ('IN_PROGRESS', 'COMPLETED')),
  CONSTRAINT ck_idempotency_record_resource_reference_length CHECK (resource_reference IS NULL OR length(resource_reference) BETWEEN 1 AND 300),
  CONSTRAINT ck_idempotency_record_completion CHECK ((status = 'COMPLETED' AND response_status IS NOT NULL) OR (status = 'IN_PROGRESS' AND response_status IS NULL)),
  CONSTRAINT ck_idempotency_record_is_secret_issued CHECK (is_secret_issued IN (0, 1)),
  CONSTRAINT fk_idempotency_record_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

CREATE INDEX ix_idempotency_record_expires_at ON idempotency_record (expires_at);
