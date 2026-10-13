-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- Supports ListApiKeys.
CREATE TABLE api_key (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  token_hash bytea NOT NULL,
  is_active boolean NOT NULL,
  http_status bigint NOT NULL,
  status text NOT NULL,
  authentication_kind text NOT NULL,
  body_document bytea NOT NULL,
  body_document_format text NOT NULL,
  CONSTRAINT pk_api_key PRIMARY KEY (id),
  CONSTRAINT ck_api_key_status CHECK (status IN ('ACTIVE', 'REVOKED')),
  CONSTRAINT ck_api_key_authentication_kind CHECK (authentication_kind IN ('SECRET', 'OAUTH2')),
  CONSTRAINT ck_api_key_body_document_format CHECK (body_document_format IN ('orcon/flow@1')),
  CONSTRAINT fk_api_key_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);
CREATE INDEX ix_api_key_org_id_created_at ON api_key (org_id, created_at);
