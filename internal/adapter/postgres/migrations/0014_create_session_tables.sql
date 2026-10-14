-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The argon2id hash of the password, encoded with its parameters (SPEC-004 R-AUTH-01, ADR-0039 section 1).
-- NULL until the account is given a password; an account without a hash never signs in.
ALTER TABLE application_user ADD COLUMN password_hash text;
ALTER TABLE application_user ADD CONSTRAINT ck_application_user_password_hash_length
  CHECK (password_hash IS NULL OR char_length(password_hash) BETWEEN 1 AND 512);

-- A login session of a browser (SPEC-004 R-ID-06). Only the SHA-256 of the opaque token is stored.
-- session is an instance table: a session is resolved before the organization is known (orcon-db TI-1).
CREATE TABLE session (
  id uuid NOT NULL,
  application_user_id uuid NOT NULL,
  token_hash bytea NOT NULL,
  assurance_level text NOT NULL,
  created_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  idle_expires_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL,
  revoked_at timestamptz,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_session PRIMARY KEY (id),
  CONSTRAINT uq_session_token_hash UNIQUE (token_hash),
  CONSTRAINT ck_session_token_hash_length CHECK (octet_length(token_hash) = 32),
  CONSTRAINT ck_session_assurance_level CHECK (assurance_level IN ('AAL1', 'AAL2')),
  -- application_user is an instance table, so this foreign key is single-column (TI-7). Deleting the user deletes its sessions.
  CONSTRAINT fk_session_application_user FOREIGN KEY (application_user_id) REFERENCES application_user (id) ON DELETE CASCADE
);

-- Supports fk_session_application_user and the delete of its parent row.
CREATE INDEX ix_session_application_user_id ON session (application_user_id);

GRANT SELECT, INSERT, UPDATE, DELETE ON session TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON session TO orcon_engine;

-- Failed sign-in attempts per identifier. id is the keyed identifier of the normalized address: the HMAC-SHA-256
-- with the instance key, truncated to 16 bytes, so the address itself never reaches the table (SPEC-004 R-AUTH-02).
-- locked_until is the instant before which the identifier is refused.
CREATE TABLE login_throttle (
  id bytea NOT NULL,
  failures bigint NOT NULL,
  locked_until timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_login_throttle PRIMARY KEY (id),
  CONSTRAINT ck_login_throttle_id_length CHECK (octet_length(id) = 16),
  CONSTRAINT ck_login_throttle_failures CHECK (failures >= 0)
);

GRANT SELECT, INSERT, UPDATE, DELETE ON login_throttle TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON login_throttle TO orcon_engine;
