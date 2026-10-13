-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- Engine plane (ADR-0047 as amended by ADR-0077): work that crosses organizations runs in
-- SECURITY DEFINER functions owned by orcon_engine, the only role the row-level security
-- policies let past the organization context. Every row they return carries org_id. Leases and
-- recovery windows compare with clock_timestamp() so that one clock decides across processes
-- (orcon-db TY-5); every stored instant comes from the caller (p_now). Running the migration
-- as a role that cannot give functions to orcon_engine needs membership in that role.

-- Claims runnable tokens in fairness order with FOR UPDATE SKIP LOCKED (ADR-0013 section 3).
CREATE FUNCTION orcon_engine.claim_tokens(p_worker_id uuid, p_fairness_keys text[], p_limit integer, p_lease_milliseconds bigint, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid, execution_id uuid, node_id text, edge_id text, scope_path text,
               fork_lineage_document bytea, payload_reference text, fairness_key text, lease_epoch bigint, lease_expires_at timestamptz)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  WITH candidate AS (
    SELECT t.id
    FROM token t
    WHERE t.status = 'RUNNABLE'
      AND t.available_at <= clock_timestamp()
      AND (p_fairness_keys IS NULL OR t.fairness_key = ANY (p_fairness_keys))
    ORDER BY t.effective_priority, t.available_at, t.id
    LIMIT p_limit
    FOR UPDATE SKIP LOCKED
  )
  UPDATE token AS t
  SET status = 'LEASED',
      lease_owner = p_worker_id,
      lease_epoch = t.lease_epoch + 1,
      lease_expires_at = clock_timestamp() + p_lease_milliseconds * interval '1 millisecond',
      claim_count = t.claim_count + 1,
      updated_at = p_now
  FROM candidate
  WHERE t.id = candidate.id
  RETURNING t.org_id, t.id, t.execution_id, t.node_id, t.edge_id, t.scope_path,
            t.fork_lineage_document, t.payload_reference, t.fairness_key, t.lease_epoch, t.lease_expires_at
$$;

-- Renews every lease of a worker in one statement and reports the cancellation requests (ADR-0013 section 4).
-- A lease the worker held but that is not returned was lost.
CREATE FUNCTION orcon_engine.renew_leases(p_worker_id uuid, p_lease_milliseconds bigint, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid, lease_epoch bigint, is_cancel_requested boolean)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  UPDATE token AS t
  SET lease_expires_at = clock_timestamp() + p_lease_milliseconds * interval '1 millisecond',
      updated_at = p_now
  WHERE t.lease_owner = p_worker_id
    AND t.status = 'LEASED'
    AND t.lease_expires_at > clock_timestamp()
  RETURNING t.org_id, t.id, t.lease_epoch,
            EXISTS (SELECT 1 FROM execution e
                    WHERE e.org_id = t.org_id AND e.id = t.execution_id AND e.cancel_requested_at IS NOT NULL)
$$;

-- The reaper (ADR-0013 section 4): expired leases return to RUNNABLE, or to SUSPENDED with reason POISON at
-- the threshold (section 9); the started attempts of their node executions become ABANDONED.
CREATE FUNCTION orcon_engine.release_expired_leases(p_limit integer, p_poison_threshold bigint, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid, execution_id uuid, abandoned_count bigint, is_poisoned boolean)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  WITH expired AS (
    SELECT t.id
    FROM token t
    WHERE t.status = 'LEASED' AND t.lease_expires_at < clock_timestamp()
    ORDER BY t.lease_expires_at
    LIMIT p_limit
    FOR UPDATE SKIP LOCKED
  ),
  moved AS (
    UPDATE token AS t
    SET status = CASE WHEN t.abandoned_count + 1 >= p_poison_threshold THEN 'SUSPENDED' ELSE 'RUNNABLE' END,
        suspend_reason = CASE WHEN t.abandoned_count + 1 >= p_poison_threshold THEN 'POISON' ELSE NULL END,
        abandoned_count = t.abandoned_count + 1,
        lease_owner = NULL,
        lease_expires_at = NULL,
        updated_at = p_now
    FROM expired
    WHERE t.id = expired.id
    RETURNING t.org_id, t.id, t.execution_id, t.abandoned_count, (t.status = 'SUSPENDED') AS is_poisoned
  ),
  abandoned AS (
    UPDATE task_attempt AS a
    SET status = 'ABANDONED', finished_at = p_now, updated_at = p_now
    FROM node_execution n, moved m
    WHERE n.org_id = m.org_id AND n.token_id = m.id
      AND a.org_id = n.org_id AND a.node_execution_id = n.id
      AND a.status = 'STARTED'
    RETURNING a.id
  )
  SELECT m.org_id, m.id, m.execution_id, m.abandoned_count, m.is_poisoned FROM moved m
$$;

-- Gives up every lease of a worker at once (graceful shutdown, ADR-0013 section 9).
CREATE FUNCTION orcon_engine.release_worker_leases(p_worker_id uuid, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  UPDATE token AS t
  SET status = 'RUNNABLE', lease_owner = NULL, lease_expires_at = NULL, updated_at = p_now
  WHERE t.lease_owner = p_worker_id AND t.status = 'LEASED'
  RETURNING t.org_id, t.id
$$;

-- Fires due timers with FOR UPDATE SKIP LOCKED (ADR-0010 section 8); waking the token is the engine's step.
CREATE FUNCTION orcon_engine.fire_due_timers(p_limit integer, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid, execution_id uuid, token_id uuid, kind text, due_at timestamptz, payload_document bytea)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  WITH due AS (
    SELECT tm.id
    FROM timer tm
    WHERE tm.status = 'SCHEDULED' AND tm.due_at <= clock_timestamp()
    ORDER BY tm.due_at, tm.id
    LIMIT p_limit
    FOR UPDATE SKIP LOCKED
  )
  UPDATE timer AS tm
  SET status = 'FIRED', updated_at = p_now
  FROM due
  WHERE tm.id = due.id
  RETURNING tm.org_id, tm.id, tm.execution_id, tm.token_id, tm.kind, tm.due_at, tm.payload_document
$$;

-- Leases undelivered outbox rows to a dispatcher (ADR-0013 section 8).
CREATE FUNCTION orcon_engine.claim_outbox(p_worker_id uuid, p_limit integer, p_lease_milliseconds bigint, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid, topic text, payload_document bytea, attempt_count bigint)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  WITH candidate AS (
    SELECT o.id
    FROM outbox o
    WHERE o.delivered_at IS NULL
      AND (o.lease_expires_at IS NULL OR o.lease_expires_at < clock_timestamp())
    ORDER BY o.created_at, o.id
    LIMIT p_limit
    FOR UPDATE SKIP LOCKED
  )
  UPDATE outbox AS o
  SET lease_owner = p_worker_id,
      lease_expires_at = clock_timestamp() + p_lease_milliseconds * interval '1 millisecond',
      attempt_count = o.attempt_count + 1,
      updated_at = p_now
  FROM candidate
  WHERE o.id = candidate.id
  RETURNING o.org_id, o.id, o.topic, o.payload_document, o.attempt_count
$$;

-- Finds the organization of a signal wait before the organization is known (ADR-0010 section 8, ADR-0077
-- section 6). The caller compares the secret hash in constant time and then continues in that organization.
CREATE FUNCTION orcon_engine.resolve_wait_secret(p_wait_id uuid)
RETURNS TABLE (org_id uuid, id uuid, secret_hash bytea, status text, expires_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  SELECT w.org_id, w.id, w.secret_hash, w.status, w.expires_at
  FROM wait w
  WHERE w.id = p_wait_id AND w.kind = 'SIGNAL'
$$;

ALTER FUNCTION orcon_engine.claim_tokens(uuid, text[], integer, bigint, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.claim_tokens(uuid, text[], integer, bigint, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.claim_tokens(uuid, text[], integer, bigint, timestamptz) TO orcon_application;
ALTER FUNCTION orcon_engine.renew_leases(uuid, bigint, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.renew_leases(uuid, bigint, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.renew_leases(uuid, bigint, timestamptz) TO orcon_application;
ALTER FUNCTION orcon_engine.release_expired_leases(integer, bigint, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.release_expired_leases(integer, bigint, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.release_expired_leases(integer, bigint, timestamptz) TO orcon_application;
ALTER FUNCTION orcon_engine.release_worker_leases(uuid, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.release_worker_leases(uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.release_worker_leases(uuid, timestamptz) TO orcon_application;
ALTER FUNCTION orcon_engine.fire_due_timers(integer, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.fire_due_timers(integer, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.fire_due_timers(integer, timestamptz) TO orcon_application;
ALTER FUNCTION orcon_engine.claim_outbox(uuid, integer, bigint, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.claim_outbox(uuid, integer, bigint, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.claim_outbox(uuid, integer, bigint, timestamptz) TO orcon_application;
ALTER FUNCTION orcon_engine.resolve_wait_secret(uuid) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.resolve_wait_secret(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.resolve_wait_secret(uuid) TO orcon_application;
