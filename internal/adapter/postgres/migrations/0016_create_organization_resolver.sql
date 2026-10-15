-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The organizations a user belongs to: active memberships of active organizations only (SPEC-004 R-ID-02, R-ID-03).
-- membership is an organization table, so the read crosses organizations and runs in the engine plane (ADR-0047):
-- a SECURITY DEFINER function owned by orcon_engine. The caller passes the user of its own principal, never a value
-- sent by a request, and the function returns the organizations in the order the listing shows them.
CREATE FUNCTION orcon_engine.resolve_user_organizations(p_application_user_id uuid, p_row_limit integer)
RETURNS TABLE (org_id uuid, org_slug text, org_name text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  SELECT org.id, org.slug, org.name
  FROM membership
  JOIN org ON org.id = membership.org_id
  WHERE membership.application_user_id = p_application_user_id
    AND membership.status = 'ACTIVE'
    AND org.status = 'ACTIVE'
  ORDER BY org.name, org.id
  LIMIT LEAST(p_row_limit, 1000)
$$;

ALTER FUNCTION orcon_engine.resolve_user_organizations(uuid, integer) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.resolve_user_organizations(uuid, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.resolve_user_organizations(uuid, integer) TO orcon_application;
