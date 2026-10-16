-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The authorization epoch of an organization (SPEC-004 R-AZ-06): every change of roles, bindings, groups or
-- memberships increments it in the same transaction; cached grants are valid while it does not move.
ALTER TABLE org ADD COLUMN authorization_epoch INTEGER NOT NULL DEFAULT 0
  CONSTRAINT ck_org_authorization_epoch CHECK (authorization_epoch >= 0);

-- A custom role of an organization (ADR-0041 section 4): a named set of catalog permissions of one scope.
CREATE TABLE custom_role (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  name TEXT NOT NULL,
  scope_kind TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_custom_role PRIMARY KEY (id),
  CONSTRAINT uq_custom_role_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_custom_role_org_id_name UNIQUE (org_id, name),
  CONSTRAINT ck_custom_role_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_custom_role_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_custom_role_name_length CHECK (length(name) BETWEEN 1 AND 100),
  CONSTRAINT ck_custom_role_scope_kind CHECK (scope_kind IN ('ORG', 'PROJECT')),
  CONSTRAINT fk_custom_role_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
) STRICT;

-- The permissions a custom role grants; each name once per role.
CREATE TABLE custom_role_permission (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  custom_role_id BLOB NOT NULL,
  permission TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_custom_role_permission PRIMARY KEY (id),
  CONSTRAINT uq_custom_role_permission_org_id_custom_role_id_permission UNIQUE (org_id, custom_role_id, permission),
  CONSTRAINT ck_custom_role_permission_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_custom_role_permission_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_custom_role_permission_custom_role_id_length CHECK (length(custom_role_id) = 16),
  CONSTRAINT ck_custom_role_permission_permission_length CHECK (length(permission) BETWEEN 3 AND 100),
  CONSTRAINT fk_custom_role_permission_custom_role FOREIGN KEY (org_id, custom_role_id) REFERENCES custom_role (org_id, id) ON DELETE CASCADE
) STRICT;

-- A role given to a user: at the organization (applies to every project) or in one project. role_name is a
-- predefined role name or the identifier of a custom role of the organization.
CREATE TABLE role_binding (
  id BLOB NOT NULL,
  org_id BLOB NOT NULL,
  scope_kind TEXT NOT NULL,
  project_id BLOB,
  application_user_id BLOB NOT NULL,
  role_name TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  CONSTRAINT pk_role_binding PRIMARY KEY (id),
  CONSTRAINT uq_role_binding_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_role_binding_id_length CHECK (length(id) = 16),
  CONSTRAINT ck_role_binding_org_id_length CHECK (length(org_id) = 16),
  CONSTRAINT ck_role_binding_application_user_id_length CHECK (length(application_user_id) = 16),
  CONSTRAINT ck_role_binding_project_id_length CHECK (project_id IS NULL OR length(project_id) = 16),
  CONSTRAINT ck_role_binding_scope_kind CHECK (scope_kind IN ('ORG', 'PROJECT')),
  CONSTRAINT ck_role_binding_scope_project CHECK ((scope_kind = 'ORG' AND project_id IS NULL) OR (scope_kind = 'PROJECT' AND project_id IS NOT NULL)),
  CONSTRAINT ck_role_binding_role_name_length CHECK (length(role_name) BETWEEN 1 AND 100),
  CONSTRAINT fk_role_binding_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_role_binding_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_role_binding_application_user FOREIGN KEY (application_user_id) REFERENCES application_user (id) ON DELETE CASCADE
) STRICT;

-- Supports the lookup of the bindings of a user in an organization.
CREATE INDEX ix_role_binding_org_id_application_user_id ON role_binding (org_id, application_user_id);
-- Supports fk_role_binding_project and the delete of the project row.
CREATE INDEX ix_role_binding_org_id_project_id ON role_binding (org_id, project_id);
-- Supports fk_role_binding_application_user and the delete of the user row.
CREATE INDEX ix_role_binding_application_user_id ON role_binding (application_user_id);
