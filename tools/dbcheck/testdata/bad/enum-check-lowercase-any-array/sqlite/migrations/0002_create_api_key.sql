-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self
-- Supports ListApiKeys.
CREATE TABLE api_key (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  token_hash BLOB NOT NULL,
  is_active INTEGER NOT NULL,
  http_status INTEGER NOT NULL,
  status TEXT NOT NULL,
  authentication_kind TEXT NOT NULL,
  body_document BLOB NOT NULL,
  body_document_format TEXT NOT NULL,
  CONSTRAINT pk_api_key PRIMARY KEY (id),
  CONSTRAINT ck_api_key_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_api_key_status CHECK (status IN ('ACTIVE', 'REVOKED')),
  CONSTRAINT ck_api_key_authentication_kind CHECK (authentication_kind IN ('SECRET', 'OAUTH2')),
  CONSTRAINT ck_api_key_body_document_format CHECK (body_document_format IN ('orcon/flow@1')),
  CONSTRAINT ck_api_key_is_active CHECK (is_active IN (0, 1)),
  CONSTRAINT fk_api_key_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;
CREATE INDEX ix_api_key_org_id_created_at ON api_key (org_id, created_at);
