-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The ingress lookup of a public request (SPEC-002 R-ADDR-08, ADR-0077 section 6). The organization of a request is
-- not known before this read, so it runs as the engine plane. It answers the organization, the project and the flow
-- that the active production address starts, and nothing else of another organization. The SQLite counterpart is
-- the ResolveTriggerRoute query, which needs no migration.
CREATE FUNCTION orcon_engine.resolve_trigger_route(p_method text, p_path text)
RETURNS TABLE (org_id uuid, project_id uuid, flow_id uuid, node_id text, flow_trigger_id uuid, trigger_address_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  SELECT a.org_id, f.project_id, t.flow_id, t.node_id, a.flow_trigger_id, a.id
  FROM trigger_address_method m
  JOIN trigger_address a ON a.org_id = m.org_id AND a.id = m.trigger_address_id
  JOIN flow_trigger t ON t.org_id = a.org_id AND t.id = a.flow_trigger_id
  JOIN flow f ON f.org_id = t.org_id AND f.id = t.flow_id
  WHERE a.scope = 'PRODUCTION' AND a.status = 'ACTIVE' AND a.pattern = p_path
    AND m.method = p_method AND f.deleted_at IS NULL
  ORDER BY a.id
  LIMIT 1
$$;
ALTER FUNCTION orcon_engine.resolve_trigger_route(text, text) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.resolve_trigger_route(text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.resolve_trigger_route(text, text) TO orcon_application;
