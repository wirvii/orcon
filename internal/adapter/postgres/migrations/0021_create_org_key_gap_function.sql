-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The organizations that have no active organization key, for the startup check that gives each one a key
-- (SPEC-004 R-KEY-05, SE3b). An organization is created together with its key; this finds the ones created before
-- that held, so the process can repair them. Organizations scheduled for deletion get no key. The read crosses
-- organizations, so it runs in the engine plane (ADR-0047 section 2): a SECURITY DEFINER function owned by
-- orcon_engine.
CREATE FUNCTION orcon_engine.list_orgs_without_active_key(p_row_limit integer)
RETURNS TABLE (org_id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  SELECT o.id
  FROM org AS o
  WHERE o.status <> 'DELETION_SCHEDULED'
    AND NOT EXISTS (SELECT 1 FROM org_key AS k WHERE k.org_id = o.id AND k.status = 'ACTIVE')
  ORDER BY o.id
  LIMIT LEAST(p_row_limit, 1000)
$$;

ALTER FUNCTION orcon_engine.list_orgs_without_active_key(integer) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.list_orgs_without_active_key(integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.list_orgs_without_active_key(integer) TO orcon_application;
