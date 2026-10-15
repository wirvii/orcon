-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The execution level of the concurrency limits (SPEC-001 C11, ADR-0014 section 2.2): a claim leases at most
-- p_execution_limit tokens per execution. The execution row is locked before its leased tokens are counted, so two
-- workers claiming for one execution serialize and cannot both pass the check. The runnable tokens are read in index
-- order and the claim stops after p_limit tokens, not after p_limit candidates: a token whose execution is at its cap
-- is passed over without using up the claim, so a capped execution cannot starve the others (BL-120). The SQLite
-- counterpart is the ClaimTokens query, which runs under the single writer and needs no migration.
DROP FUNCTION orcon_engine.claim_tokens(uuid, text[], integer, bigint, timestamptz, bigint);
CREATE FUNCTION orcon_engine.claim_tokens(p_worker_id uuid, p_fairness_keys text[], p_limit integer, p_lease_milliseconds bigint,
                                          p_now timestamptz, p_poison_threshold bigint, p_execution_limit integer)
RETURNS TABLE (org_id uuid, id uuid, execution_id uuid, node_id text, edge_id text, scope_path text,
               fork_lineage_document bytea, payload_reference text, fairness_key text, lease_epoch bigint, lease_expires_at timestamptz)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
#variable_conflict use_column
DECLARE
  r record;
  claimed record;
  leased bigint;
  taken integer := 0;
  -- The executions found at their cap during this claim, so that their other tokens cost no further count.
  full_executions uuid[] := ARRAY[]::uuid[];
BEGIN
  -- Lock the executions of the candidates first, in one fixed order, and only then the tokens (which are skipped
  -- when locked, never waited for). Every claim takes the locks in this order, so two claims cannot deadlock.
  PERFORM 1 FROM execution e
    WHERE (e.org_id, e.id) IN (
      SELECT t.org_id, t.execution_id FROM token t
      WHERE t.status = 'RUNNABLE'
        AND t.available_at <= clock_timestamp()
        AND (p_fairness_keys IS NULL OR t.fairness_key = ANY (p_fairness_keys))
        AND t.abandoned_count < p_poison_threshold - 1)
    ORDER BY e.org_id, e.id
    FOR UPDATE;
  FOR r IN
    SELECT t.id, t.org_id, t.execution_id
    FROM token t
    WHERE t.status = 'RUNNABLE'
      AND t.available_at <= clock_timestamp()
      AND (p_fairness_keys IS NULL OR t.fairness_key = ANY (p_fairness_keys))
      AND t.abandoned_count < p_poison_threshold - 1
    ORDER BY t.effective_priority, t.available_at, t.id
  LOOP
    EXIT WHEN taken >= p_limit;
    CONTINUE WHEN r.execution_id = ANY (full_executions);
    SELECT count(*) INTO leased FROM token l
      WHERE l.org_id = r.org_id AND l.execution_id = r.execution_id AND l.status = 'LEASED';
    IF leased >= p_execution_limit THEN
      full_executions := full_executions || r.execution_id;
      CONTINUE;
    END IF;
    -- A token another transaction holds is skipped, never waited for.
    PERFORM 1 FROM token k WHERE k.org_id = r.org_id AND k.id = r.id AND k.status = 'RUNNABLE' FOR UPDATE SKIP LOCKED;
    CONTINUE WHEN NOT FOUND;

    UPDATE token AS t
    SET status = 'LEASED',
        lease_owner = p_worker_id,
        lease_epoch = t.lease_epoch + 1,
        lease_expires_at = clock_timestamp() + p_lease_milliseconds * interval '1 millisecond',
        claim_count = t.claim_count + 1,
        updated_at = p_now
    WHERE t.org_id = r.org_id AND t.id = r.id AND t.status = 'RUNNABLE'
    RETURNING t.* INTO claimed;
    IF NOT FOUND THEN
      CONTINUE;
    END IF;

    taken := taken + 1;
    PERFORM orcon_engine.start_claimed_execution(claimed.org_id, claimed.execution_id, p_now);
    org_id := claimed.org_id;
    id := claimed.id;
    execution_id := claimed.execution_id;
    node_id := claimed.node_id;
    edge_id := claimed.edge_id;
    scope_path := claimed.scope_path;
    fork_lineage_document := claimed.fork_lineage_document;
    payload_reference := claimed.payload_reference;
    fairness_key := claimed.fairness_key;
    lease_epoch := claimed.lease_epoch;
    lease_expires_at := claimed.lease_expires_at;
    RETURN NEXT;
  END LOOP;
END
$$;
ALTER FUNCTION orcon_engine.claim_tokens(uuid, text[], integer, bigint, timestamptz, bigint, integer) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.claim_tokens(uuid, text[], integer, bigint, timestamptz, bigint, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.claim_tokens(uuid, text[], integer, bigint, timestamptz, bigint, integer) TO orcon_application;

-- The claimable tokens grouped by fairness key and priority, the snapshot a worker plans its claims from (ADR-0014
-- section 1). The planner needs every organization, so it reads through the engine role, as the claim does.
CREATE FUNCTION orcon_engine.runnable_counts(p_now timestamptz, p_row_limit integer)
RETURNS TABLE (fairness_key text, effective_priority bigint, runnable_count bigint, oldest_available_at timestamptz)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
  SELECT t.fairness_key, t.effective_priority, count(*), min(t.available_at)
  FROM token t
  WHERE t.status = 'RUNNABLE' AND t.available_at <= p_now
  GROUP BY t.fairness_key, t.effective_priority
  ORDER BY t.fairness_key, t.effective_priority
  LIMIT p_row_limit;
$$;
ALTER FUNCTION orcon_engine.runnable_counts(timestamptz, integer) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.runnable_counts(timestamptz, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.runnable_counts(timestamptz, integer) TO orcon_application;
