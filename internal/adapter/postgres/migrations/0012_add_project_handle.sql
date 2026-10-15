-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind expand
-- orcon:compatibility-floor self

-- The project handle is unique across the instance (ADR-0038 section 1, SPEC-004 R-ID-11): the slug, or
-- "<org slug>-<slug>" when another project holds the slug. NULL only for a project written by a binary that
-- predates handles; the code of this release always writes one, and a later contract migration makes the
-- column required. Existing projects are given a handle below.
ALTER TABLE project ADD COLUMN handle text;
ALTER TABLE project ADD CONSTRAINT ck_project_handle_length CHECK (char_length(handle) BETWEEN 1 AND 201);

-- Backfill of the rows of 0002. The project table is small (one row per project of the instance), so the
-- single statement holds its lock for a negligible time (DA-5.2). Row-level security is forced on the table,
-- which binds even its owner, so it is lifted for this statement (and for org, which it reads) and restored;
-- the migration is one transaction.
ALTER TABLE org NO FORCE ROW LEVEL SECURITY;
ALTER TABLE project NO FORCE ROW LEVEL SECURITY;
UPDATE project SET handle = CASE
  WHEN (SELECT count(*) FROM project AS other WHERE other.slug = project.slug) = 1 THEN project.slug
  ELSE (SELECT org.slug FROM org WHERE org.id = project.org_id) || '-' || project.slug
END;
ALTER TABLE project FORCE ROW LEVEL SECURITY;
ALTER TABLE org FORCE ROW LEVEL SECURITY;

-- Business uniqueness of the handle across organizations; NULLs do not collide. Supports the check of CreateProject.
CREATE UNIQUE INDEX uq_project_handle ON project (handle);
