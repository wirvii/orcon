-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

CREATE TABLE thing (
  id bigint NOT NULL,
  org_id bigint NOT NULL,
  name text NOT NULL,
  created_at bigint NOT NULL,
  CONSTRAINT pk_thing PRIMARY KEY (id)
);
CREATE INDEX ix_thing_org_id_created_at_id ON thing (org_id, created_at, id);

CREATE SCHEMA orcon_engine;
CREATE FUNCTION orcon_engine.list_things(p_org_id bigint, p_limit bigint) RETURNS SETOF bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path = orcon_engine, public AS $$
BEGIN
  RETURN QUERY SELECT id FROM thing WHERE org_id = p_org_id ORDER BY created_at, id LIMIT p_limit;
END
$$;
