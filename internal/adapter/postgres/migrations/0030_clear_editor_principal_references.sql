-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The editor tables (migration 0028) and the draft save columns of flow add actor columns that name a
-- principal; this replaces the clearing function of migration 0013 to include them (orcon-db IN-12, QY-8).
CREATE OR REPLACE FUNCTION orcon_engine.clear_principal_references(p_principal_id uuid, p_limit integer)
RETURNS integer
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  WITH cleared_flow AS (
    UPDATE flow SET deleted_by_principal_id = NULL
    WHERE id IN (SELECT id FROM flow WHERE deleted_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_flow_version AS (
    UPDATE flow_version SET published_by_principal_id = NULL
    WHERE id IN (SELECT id FROM flow_version WHERE published_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_membership AS (
    UPDATE membership SET invited_by_principal_id = NULL
    WHERE id IN (SELECT id FROM membership WHERE invited_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_instance_alert AS (
    UPDATE instance_alert SET acknowledged_by_principal_id = NULL
    WHERE id IN (SELECT id FROM instance_alert WHERE acknowledged_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_external_operation AS (
    UPDATE external_operation SET resolved_by_principal_id = NULL
    WHERE id IN (SELECT id FROM external_operation WHERE resolved_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_human_task AS (
    UPDATE human_task SET resolved_by_principal_id = NULL
    WHERE id IN (SELECT id FROM human_task WHERE resolved_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_attention_request_acknowledged AS (
    UPDATE attention_request SET acknowledged_by_principal_id = NULL
    WHERE id IN (SELECT id FROM attention_request WHERE acknowledged_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  -- A row that names the principal in both columns is cleared by the first statement; this one leaves it
  -- for the next call, so no row is updated twice by the same statement.
  cleared_attention_request_resolved AS (
    UPDATE attention_request SET resolved_by_principal_id = NULL
    WHERE id IN (
      SELECT id FROM attention_request
      WHERE resolved_by_principal_id = p_principal_id AND acknowledged_by_principal_id IS DISTINCT FROM p_principal_id
      ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_operational_alert AS (
    UPDATE operational_alert SET acknowledged_by_principal_id = NULL
    WHERE id IN (SELECT id FROM operational_alert WHERE acknowledged_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_connector_package AS (
    UPDATE connector_package SET installed_by_principal_id = NULL
    WHERE id IN (SELECT id FROM connector_package WHERE installed_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_connector_grant AS (
    UPDATE connector_grant SET granted_by_principal_id = NULL
    WHERE id IN (SELECT id FROM connector_grant WHERE granted_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  -- A row that names the principal in both actor columns of flow is cleared by the first statement; this one
  -- leaves it for the next call, so no row is updated twice by the same statement.
  cleared_flow_draft_updated AS (
    UPDATE flow SET draft_updated_by_principal_id = NULL
    WHERE id IN (
      SELECT id FROM flow
      WHERE draft_updated_by_principal_id = p_principal_id AND deleted_by_principal_id IS DISTINCT FROM p_principal_id
      ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_node_test_data AS (
    UPDATE node_test_data SET created_by_principal_id = NULL
    WHERE id IN (SELECT id FROM node_test_data WHERE created_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_flow_node_pin AS (
    UPDATE flow_node_pin SET pinned_by_principal_id = NULL
    WHERE id IN (SELECT id FROM flow_node_pin WHERE pinned_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_test_run AS (
    UPDATE test_run SET started_by_principal_id = NULL
    WHERE id IN (SELECT id FROM test_run WHERE started_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_effect_preview AS (
    UPDATE effect_preview SET decided_by_principal_id = NULL
    WHERE id IN (SELECT id FROM effect_preview WHERE decided_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  ),
  cleared_flow_editing_session AS (
    UPDATE flow_editing_session SET opened_by_principal_id = NULL
    WHERE id IN (SELECT id FROM flow_editing_session WHERE opened_by_principal_id = p_principal_id ORDER BY id LIMIT p_limit)
    RETURNING 1
  )
  SELECT ((SELECT count(*) FROM cleared_flow)
    + (SELECT count(*) FROM cleared_flow_version)
    + (SELECT count(*) FROM cleared_membership)
    + (SELECT count(*) FROM cleared_instance_alert)
    + (SELECT count(*) FROM cleared_external_operation)
    + (SELECT count(*) FROM cleared_human_task)
    + (SELECT count(*) FROM cleared_attention_request_acknowledged)
    + (SELECT count(*) FROM cleared_attention_request_resolved)
    + (SELECT count(*) FROM cleared_operational_alert)
    + (SELECT count(*) FROM cleared_connector_package)
    + (SELECT count(*) FROM cleared_connector_grant)
    + (SELECT count(*) FROM cleared_flow_draft_updated)
    + (SELECT count(*) FROM cleared_node_test_data)
    + (SELECT count(*) FROM cleared_flow_node_pin)
    + (SELECT count(*) FROM cleared_test_run)
    + (SELECT count(*) FROM cleared_effect_preview)
    + (SELECT count(*) FROM cleared_flow_editing_session))::integer
$$;

ALTER FUNCTION orcon_engine.clear_principal_references(uuid, integer) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.clear_principal_references(uuid, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.clear_principal_references(uuid, integer) TO orcon_application;
