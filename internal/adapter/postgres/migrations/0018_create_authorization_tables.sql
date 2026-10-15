-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The authorization epoch of an organization (SPEC-004 R-AZ-06): every change of roles, bindings, groups or
-- memberships increments it in the same transaction; cached grants are valid while it does not move.
ALTER TABLE org ADD COLUMN authorization_epoch bigint NOT NULL DEFAULT 0;
ALTER TABLE org ADD CONSTRAINT ck_org_authorization_epoch CHECK (authorization_epoch >= 0);

-- A custom role of an organization (ADR-0041 section 4): a named set of catalog permissions of one scope.
CREATE TABLE custom_role (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  name text NOT NULL,
  scope_kind text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_custom_role PRIMARY KEY (id),
  CONSTRAINT uq_custom_role_org_id_id UNIQUE (org_id, id),
  CONSTRAINT uq_custom_role_org_id_name UNIQUE (org_id, name),
  CONSTRAINT ck_custom_role_name_length CHECK (char_length(name) BETWEEN 1 AND 100),
  CONSTRAINT ck_custom_role_scope_kind CHECK (scope_kind IN ('ORG', 'PROJECT')),
  CONSTRAINT fk_custom_role_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE
);

ALTER TABLE custom_role ENABLE ROW LEVEL SECURITY;
ALTER TABLE custom_role FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON custom_role FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON custom_role TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON custom_role TO orcon_engine;

-- The permissions a custom role grants; each name once per role.
CREATE TABLE custom_role_permission (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  custom_role_id uuid NOT NULL,
  permission text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_custom_role_permission PRIMARY KEY (id),
  CONSTRAINT uq_custom_role_permission_org_id_custom_role_id_permission UNIQUE (org_id, custom_role_id, permission),
  CONSTRAINT ck_custom_role_permission_permission_length CHECK (char_length(permission) BETWEEN 3 AND 100),
  CONSTRAINT fk_custom_role_permission_custom_role FOREIGN KEY (org_id, custom_role_id) REFERENCES custom_role (org_id, id) ON DELETE CASCADE
);

ALTER TABLE custom_role_permission ENABLE ROW LEVEL SECURITY;
ALTER TABLE custom_role_permission FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON custom_role_permission FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON custom_role_permission TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON custom_role_permission TO orcon_engine;

-- A role given to a user: at the organization (applies to every project) or in one project. role_name is a
-- predefined role name or the identifier of a custom role of the organization.
CREATE TABLE role_binding (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  scope_kind text NOT NULL,
  project_id uuid,
  application_user_id uuid NOT NULL,
  role_name text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_role_binding PRIMARY KEY (id),
  CONSTRAINT uq_role_binding_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_role_binding_scope_kind CHECK (scope_kind IN ('ORG', 'PROJECT')),
  CONSTRAINT ck_role_binding_scope_project CHECK ((scope_kind = 'ORG' AND project_id IS NULL) OR (scope_kind = 'PROJECT' AND project_id IS NOT NULL)),
  CONSTRAINT ck_role_binding_role_name_length CHECK (char_length(role_name) BETWEEN 1 AND 100),
  CONSTRAINT fk_role_binding_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_role_binding_project FOREIGN KEY (org_id, project_id) REFERENCES project (org_id, id) ON DELETE CASCADE,
  CONSTRAINT fk_role_binding_application_user FOREIGN KEY (application_user_id) REFERENCES application_user (id) ON DELETE CASCADE
);

-- Supports the lookup of the bindings of a user in an organization.
CREATE INDEX ix_role_binding_org_id_application_user_id ON role_binding (org_id, application_user_id);
-- Supports fk_role_binding_project and the delete of the project row.
CREATE INDEX ix_role_binding_org_id_project_id ON role_binding (org_id, project_id);
-- Supports fk_role_binding_application_user and the delete of the user row.
CREATE INDEX ix_role_binding_application_user_id ON role_binding (application_user_id);

ALTER TABLE role_binding ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_binding FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON role_binding FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON role_binding TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON role_binding TO orcon_engine;

-- The organization of a project, across organizations: a project of another organization must be hidden, so the
-- lookup runs in the engine plane (ADR-0041 section 6).
CREATE FUNCTION orcon_engine.project_organization(p_project_id uuid)
RETURNS uuid
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  SELECT org_id FROM project WHERE id = p_project_id
$$;

ALTER FUNCTION orcon_engine.project_organization(uuid) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.project_organization(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.project_organization(uuid) TO orcon_application;
