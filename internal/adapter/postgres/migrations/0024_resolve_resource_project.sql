-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The organization and the project of a resource that belongs to a project (a flow, an execution, an external
-- operation or a human task), across organizations: a resource of another organization must be hidden, so the lookup
-- runs in the engine plane like project_organization (ADR-0041 section 6). It answers no row for an unknown kind or
-- identifier. The SQLite counterpart is the GetResourceProject query, which needs no migration.
CREATE FUNCTION orcon_engine.resource_project(p_kind text, p_id uuid)
RETURNS TABLE (org_id uuid, project_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  SELECT f.org_id, f.project_id FROM flow f WHERE p_kind = 'flow' AND f.id = p_id AND f.deleted_at IS NULL
  UNION ALL
  SELECT e.org_id, e.project_id FROM execution e WHERE p_kind = 'execution' AND e.id = p_id
  UNION ALL
  SELECT e.org_id, e.project_id FROM external_operation o JOIN execution e ON e.org_id = o.org_id AND e.id = o.execution_id
    WHERE p_kind = 'externalOperation' AND o.id = p_id
  UNION ALL
  SELECT e.org_id, e.project_id FROM human_task h JOIN execution e ON e.org_id = h.org_id AND e.id = h.execution_id
    WHERE p_kind = 'humanTask' AND h.id = p_id
  LIMIT 1
$$;
ALTER FUNCTION orcon_engine.resource_project(text, uuid) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.resource_project(text, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.resource_project(text, uuid) TO orcon_application;
