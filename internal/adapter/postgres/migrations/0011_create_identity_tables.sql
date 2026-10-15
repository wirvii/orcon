-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self


CREATE TABLE application_user (
  id uuid NOT NULL,
  principal_id uuid NOT NULL,
  email_normalized text NOT NULL,
  display_name text NOT NULL,
  status text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_application_user PRIMARY KEY (id),
  CONSTRAINT uq_application_user_principal_id UNIQUE (principal_id),
  CONSTRAINT uq_application_user_email_normalized UNIQUE (email_normalized),
  CONSTRAINT ck_application_user_email_normalized_length CHECK (char_length(email_normalized) BETWEEN 3 AND 320),
  CONSTRAINT ck_application_user_display_name_length CHECK (char_length(display_name) BETWEEN 1 AND 200),
  CONSTRAINT ck_application_user_status CHECK (status IN ('ACTIVE', 'DISABLED')),
  -- principal is an instance table, so this foreign key is single-column (TI-7). Deleting the principal deletes the user (IN-12).
  CONSTRAINT fk_application_user_principal FOREIGN KEY (principal_id) REFERENCES principal (id) ON DELETE CASCADE
);

GRANT SELECT, INSERT, UPDATE, DELETE ON application_user TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON application_user TO orcon_engine;

CREATE TABLE membership (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  application_user_id uuid NOT NULL,
  status text NOT NULL,
  -- NULL when the principal was deleted; the audit log keeps the identity (IN-12).
  invited_by_principal_id uuid,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_membership PRIMARY KEY (id),
  CONSTRAINT uq_membership_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_membership_org_id_application_user_id UNIQUE (org_id, application_user_id),
  CONSTRAINT ck_membership_status CHECK (status IN ('INVITED', 'ACTIVE', 'SUSPENDED')),
  -- org, application_user and principal are instance tables, so these foreign keys are single-column (TI-7).
  CONSTRAINT fk_membership_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_membership_application_user FOREIGN KEY (application_user_id) REFERENCES application_user (id) ON DELETE CASCADE,
  CONSTRAINT fk_membership_invited_by_principal FOREIGN KEY (invited_by_principal_id) REFERENCES principal (id) ON DELETE SET NULL
);

-- Supports ListMemberships (keyset by id within an organization) and the organization foreign key.
-- The unique constraint uq_membership_org_id_id above is the index.

-- Supports fk_membership_application_user and the delete of its parent row.
CREATE INDEX ix_membership_application_user_id ON membership (application_user_id);

-- Supports fk_membership_invited_by_principal and the batched clearing of the column when a principal is deleted (IN-12).
CREATE INDEX ix_membership_invited_by_principal_id ON membership (invited_by_principal_id) WHERE invited_by_principal_id IS NOT NULL;

ALTER TABLE membership ENABLE ROW LEVEL SECURITY;
ALTER TABLE membership FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON membership FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON membership TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON membership TO orcon_engine;
