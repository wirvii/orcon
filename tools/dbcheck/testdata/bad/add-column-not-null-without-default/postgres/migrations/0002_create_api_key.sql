-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self
-- Supports ListApiKeys.
CREATE TABLE api_key (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  token_hash bytea NOT NULL,
  is_active boolean NOT NULL,
  http_status bigint NOT NULL,
  CONSTRAINT pk_api_key PRIMARY KEY (id),
  CONSTRAINT fk_api_key_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);
CREATE INDEX ix_api_key_org_id_created_at ON api_key (org_id, created_at);
ALTER TABLE api_key ADD COLUMN label text NOT NULL;
