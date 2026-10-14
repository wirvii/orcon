-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0
-- orcon:kind additive
-- orcon:compatibility-floor self

-- The durable engine (SPEC-001 C4a). An execution keeps, next to its token rows, one document with the
-- completion trackers of its run (which nodes completed, how far each join got, which loop instances are
-- armed): ADR-0010 section 2 persists them where tokens can reach a join or a loop, and this implementation
-- persists them for every node so that a step needs no static analysis. The document is read and written whole
-- (orcon-db JS-1). engine_state_revision counts the commits that wrote it: a worker whose copy is older than the
-- row replays its step on the new state instead of overwriting it. NULL document: no worker has run the execution.
ALTER TABLE execution ADD COLUMN engine_state_document bytea;
ALTER TABLE execution ADD COLUMN engine_state_document_format text;
ALTER TABLE execution ADD COLUMN engine_state_revision bigint NOT NULL DEFAULT 0;
ALTER TABLE execution ADD CONSTRAINT ck_execution_engine_state_document_format CHECK (engine_state_document_format IN ('orcon/engine-state@1'));
ALTER TABLE execution ADD CONSTRAINT ck_execution_engine_state_document_pair CHECK ((engine_state_document IS NULL) = (engine_state_document_format IS NULL));
ALTER TABLE execution ADD CONSTRAINT ck_execution_engine_state_revision_range CHECK (engine_state_revision >= 0);

-- The batch a token starts with: the execution input, the combined input of a join continuation, or of a loop
-- iteration that merged its returns. A node output belongs to a node execution (node_output); these belong to no
-- node execution yet, because the token has not been activated. The id is the payload reference.
CREATE TABLE token_input (
  id uuid NOT NULL,
  org_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  item_count bigint NOT NULL,
  encoding text NOT NULL,
  inline_document bytea NOT NULL,
  inline_document_format text NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CONSTRAINT pk_token_input PRIMARY KEY (id),
  CONSTRAINT uq_token_input_org_id_id UNIQUE (org_id, id),
  CONSTRAINT ck_token_input_item_count_range CHECK (item_count >= 0),
  CONSTRAINT ck_token_input_encoding_length CHECK (char_length(encoding) BETWEEN 1 AND 64),
  CONSTRAINT ck_token_input_inline_document_format CHECK (inline_document_format IN ('orcon/items@1')),
  CONSTRAINT fk_token_input_org FOREIGN KEY (org_id) REFERENCES org (id) ON DELETE CASCADE,
  CONSTRAINT fk_token_input_execution FOREIGN KEY (org_id, execution_id) REFERENCES execution (org_id, id) ON DELETE CASCADE
);

-- Supports fk_token_input_execution and the delete of its parent row.
CREATE INDEX ix_token_input_org_id_execution_id ON token_input (org_id, execution_id);

ALTER TABLE token_input ENABLE ROW LEVEL SECURITY;
ALTER TABLE token_input FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON token_input FOR ALL
  USING (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())))
  WITH CHECK (org_id = (SELECT orcon_context.current_org()) OR current_user = 'orcon_engine'
         OR org_id = ANY ((SELECT orcon_context.current_org_ids())));
GRANT SELECT, INSERT, UPDATE, DELETE ON token_input TO orcon_application;
GRANT SELECT, INSERT, UPDATE, DELETE ON token_input TO orcon_engine;

-- Supports ListLiveTokens (the tokens of one execution that are not terminal, which a worker reads to open the
-- execution) and orcon_engine.refresh_execution (their counts per state).
CREATE INDEX ix_token_org_id_execution_id_id ON token (org_id, execution_id, id)
  WHERE status IN ('RUNNABLE', 'LEASED', 'WAITING', 'DEFERRED', 'SUSPENDED');

-- ---------------------------------------------------------------------------------------------------------------
-- Engine plane, part two. Recovery writes rows of its own (events, an attention request, the counters of the
-- execution) in the same statement group that moves the token, so a crash leaves either all of it or none
-- (ADR-0013 section 9). Row identities are the deterministic ids of ADR-0008 section 6, computed by the same
-- function H the engine uses: the first 128 bits of the SHA-256 of the length-prefixed fields.
-- ---------------------------------------------------------------------------------------------------------------

-- The functions below call each other, so their owner needs the schema they live in.
GRANT USAGE ON SCHEMA orcon_engine TO orcon_engine;

CREATE FUNCTION orcon_engine.derive_uuid(VARIADIC p_fields text[])
RETURNS uuid
LANGUAGE plpgsql IMMUTABLE SET search_path = pg_catalog AS
$$
DECLARE
  v_buffer bytea := ''::bytea;
  v_field text;
  v_bytes bytea;
BEGIN
  FOREACH v_field IN ARRAY p_fields LOOP
    v_bytes := convert_to(v_field, 'UTF8');
    v_buffer := v_buffer || int8send(octet_length(v_bytes)::bigint) || v_bytes;
  END LOOP;
  RETURN encode(substring(sha256(v_buffer) FROM 1 FOR 16), 'hex')::uuid;
END
$$;

-- Appends one event to an execution under the lock of its row, so that concurrent appends take their numbers in
-- commit order (ADR-0009 section 7). The id makes the call idempotent.
CREATE FUNCTION orcon_engine.append_event(p_org_id uuid, p_execution_id uuid, p_id uuid, p_type text,
                                          p_reference text, p_data text, p_now timestamptz)
RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
DECLARE
  v_root uuid;
  v_sequence bigint;
BEGIN
  SELECT e.root_execution_id INTO v_root FROM execution e WHERE e.org_id = p_org_id AND e.id = p_execution_id FOR NO KEY UPDATE;
  IF NOT FOUND THEN
    RETURN;
  END IF;
  SELECT COALESCE(MAX(v.sequence_number), 0) + 1 INTO v_sequence FROM execution_event v WHERE v.org_id = p_org_id AND v.execution_id = p_execution_id;
  INSERT INTO execution_event (id, org_id, execution_id, root_execution_id, sequence_number, occurred_at, type, schema_version,
                               reference_document, reference_document_format, data_document, data_document_format, created_at)
  VALUES (p_id, p_org_id, p_execution_id, v_root, v_sequence, p_now, p_type, 1,
          convert_to(p_reference, 'UTF8'), 'orcon/event-reference@1',
          CASE WHEN p_data IS NULL THEN NULL ELSE convert_to(p_data, 'UTF8') END,
          CASE WHEN p_data IS NULL THEN NULL ELSE 'orcon/event-data@1' END, p_now)
  ON CONFLICT (id) DO NOTHING;
END
$$;

-- Recomputes the token counters of an execution from its live tokens and applies the status moves that follow from
-- them (ADR-0009 section 1): RUNNING or WAITING to SUSPENDED when only suspended tokens are left, WAITING or
-- SUSPENDED to RUNNING when work is runnable again.
CREATE FUNCTION orcon_engine.refresh_execution(p_org_id uuid, p_execution_id uuid, p_now timestamptz)
RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
DECLARE
  v_status text;
  v_runnable bigint;
  v_leased bigint;
  v_waiting bigint;
  v_suspended bigint;
  v_next text;
BEGIN
  SELECT e.status INTO v_status FROM execution e WHERE e.org_id = p_org_id AND e.id = p_execution_id FOR NO KEY UPDATE;
  IF NOT FOUND THEN
    RETURN;
  END IF;
  SELECT count(*) FILTER (WHERE t.status = 'RUNNABLE'), count(*) FILTER (WHERE t.status = 'LEASED'),
         count(*) FILTER (WHERE t.status = 'WAITING'), count(*) FILTER (WHERE t.status = 'SUSPENDED')
    INTO v_runnable, v_leased, v_waiting, v_suspended
  FROM token t
  WHERE t.org_id = p_org_id AND t.execution_id = p_execution_id
    AND t.status IN ('RUNNABLE', 'LEASED', 'WAITING', 'DEFERRED', 'SUSPENDED');
  v_next := v_status;
  IF v_status IN ('RUNNING', 'WAITING') AND v_runnable = 0 AND v_leased = 0 AND v_suspended >= 1 THEN
    v_next := 'SUSPENDED';
  ELSIF v_status IN ('WAITING', 'SUSPENDED') AND v_runnable + v_leased >= 1 THEN
    v_next := 'RUNNING';
  END IF;
  UPDATE execution e
  SET runnable_count = v_runnable, leased_count = v_leased, waiting_count = v_waiting, suspended_count = v_suspended,
      status = v_next, updated_at = p_now
  WHERE e.org_id = p_org_id AND e.id = p_execution_id;
  IF v_next <> v_status THEN
    PERFORM orcon_engine.append_event(p_org_id, p_execution_id,
      orcon_engine.derive_uuid('event', CASE WHEN v_next = 'SUSPENDED' THEN 'execution.suspended' ELSE 'execution.resumed' END,
                               p_execution_id::text, v_status, to_char(p_now AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US')),
      CASE WHEN v_next = 'SUSPENDED' THEN 'execution.suspended' ELSE 'execution.resumed' END,
      '{}', NULL, p_now);
  END IF;
END
$$;

-- Gives up the leases of the given tokens: each goes back to RUNNABLE, or to SUSPENDED with reason POISON once it
-- was abandoned p_poison_threshold times (ADR-0013 section 9). The started attempts of its node executions become
-- ABANDONED, and the events, the attention request and the counters follow in the same transaction. The callers
-- (expired leases, a worker that shuts down) choose the tokens.
CREATE FUNCTION orcon_engine.abandon_tokens(p_ids uuid[], p_poison_threshold bigint, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid, execution_id uuid, abandoned_count bigint, is_poisoned boolean)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
#variable_conflict use_column
DECLARE
  r record;
BEGIN
  FOR r IN
    UPDATE token AS t
    SET status = CASE WHEN t.abandoned_count + 1 >= p_poison_threshold THEN 'SUSPENDED' ELSE 'RUNNABLE' END,
        suspend_reason = CASE WHEN t.abandoned_count + 1 >= p_poison_threshold THEN 'POISON' ELSE NULL END,
        abandoned_count = t.abandoned_count + 1,
        lease_owner = NULL,
        lease_expires_at = NULL,
        available_at = CASE WHEN t.abandoned_count + 1 >= p_poison_threshold THEN t.available_at ELSE p_now END,
        updated_at = p_now
    WHERE t.id = ANY (p_ids) AND t.status = 'LEASED'
    RETURNING t.org_id AS r_org, t.id AS r_token, t.execution_id AS r_execution, t.abandoned_count AS r_abandoned,
              t.lease_epoch AS r_epoch, t.node_id AS r_node, (t.status = 'SUSPENDED') AS r_poisoned
  LOOP
    UPDATE task_attempt AS a
    SET status = 'ABANDONED', finished_at = p_now, updated_at = p_now
    FROM node_execution n
    WHERE n.org_id = r.r_org AND n.token_id = r.r_token
      AND a.org_id = n.org_id AND a.node_execution_id = n.id AND a.status = 'STARTED';
    PERFORM orcon_engine.append_event(r.r_org, r.r_execution,
      orcon_engine.derive_uuid('event', 'lease.expired', r.r_token::text, r.r_epoch::text),
      'lease.expired', '{"tokenId":"' || r.r_token::text || '","nodeId":' || to_json(r.r_node)::text || '}',
      '{"abandonedCount":' || r.r_abandoned::text || '}', p_now);
    IF r.r_poisoned THEN
      PERFORM orcon_engine.append_event(r.r_org, r.r_execution,
        orcon_engine.derive_uuid('event', 'token.poisoned', r.r_token::text, r.r_epoch::text),
        'token.poisoned', '{"tokenId":"' || r.r_token::text || '","nodeId":' || to_json(r.r_node)::text || '}',
        '{"abandonedCount":' || r.r_abandoned::text || '}', p_now);
      INSERT INTO attention_request (id, org_id, execution_id, project_id, flow_id, flow_version_id, node_id, token_id, cause,
                                     cause_reference, notify_class, status, level, group_key, item_count, summary, created_at, updated_at)
      SELECT orcon_engine.derive_uuid('attention', 'POISON_TOKEN', r.r_token::text), e.org_id, e.id, e.project_id, e.flow_id,
             e.flow_version_id, r.r_node, r.r_token, 'POISON_TOKEN', 'poison/' || r.r_token::text, 'NOTIFY', 'OPEN', 0,
             'poison/' || e.id::text, 1,
             'A token was abandoned ' || r.r_abandoned::text || ' times and is suspended', p_now, p_now
      FROM execution e
      WHERE e.org_id = r.r_org AND e.id = r.r_execution
      ON CONFLICT (org_id, cause_reference) DO NOTHING;
    END IF;
    PERFORM orcon_engine.refresh_execution(r.r_org, r.r_execution, p_now);
    org_id := r.r_org;
    id := r.r_token;
    execution_id := r.r_execution;
    abandoned_count := r.r_abandoned;
    is_poisoned := r.r_poisoned;
    RETURN NEXT;
  END LOOP;
END
$$;

-- The reaper (ADR-0013 section 4): expired leases are abandoned. The database clock decides what expired.
CREATE OR REPLACE FUNCTION orcon_engine.release_expired_leases(p_limit integer, p_poison_threshold bigint, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid, execution_id uuid, abandoned_count bigint, is_poisoned boolean)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
DECLARE
  v_ids uuid[];
BEGIN
  SELECT COALESCE(array_agg(x.id), ARRAY[]::uuid[]) INTO v_ids
  FROM (
    SELECT t.id
    FROM token t
    WHERE t.status = 'LEASED' AND t.lease_expires_at < clock_timestamp()
    ORDER BY t.lease_expires_at
    LIMIT p_limit
    FOR UPDATE SKIP LOCKED
  ) x;
  RETURN QUERY SELECT a.org_id, a.id, a.execution_id, a.abandoned_count, a.is_poisoned
               FROM orcon_engine.abandon_tokens(v_ids, p_poison_threshold, p_now) a;
END
$$;

-- Gives up every lease of a worker at once (graceful shutdown, ADR-0013 section 9). It counts as an abandonment
-- like an expired lease does, so a token that keeps ending its worker reaches the poison threshold.
DROP FUNCTION orcon_engine.release_worker_leases(uuid, timestamptz);
CREATE FUNCTION orcon_engine.release_worker_leases(p_worker_id uuid, p_poison_threshold bigint, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid, execution_id uuid, abandoned_count bigint, is_poisoned boolean)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
DECLARE
  v_ids uuid[];
BEGIN
  SELECT COALESCE(array_agg(x.id), ARRAY[]::uuid[]) INTO v_ids
  FROM (
    SELECT t.id
    FROM token t
    WHERE t.lease_owner = p_worker_id AND t.status = 'LEASED'
    FOR UPDATE SKIP LOCKED
  ) x;
  RETURN QUERY SELECT a.org_id, a.id, a.execution_id, a.abandoned_count, a.is_poisoned
               FROM orcon_engine.abandon_tokens(v_ids, p_poison_threshold, p_now) a;
END
$$;

-- Fires due timers with FOR UPDATE SKIP LOCKED (ADR-0010 section 8) and wakes the token that waits for each in the
-- same transaction: a crash can no longer leave a FIRED timer next to a WAITING token that nothing wakes.
CREATE OR REPLACE FUNCTION orcon_engine.fire_due_timers(p_limit integer, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid, execution_id uuid, token_id uuid, kind text, due_at timestamptz, payload_document bytea)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
#variable_conflict use_column
DECLARE
  r record;
  v_woken boolean;
BEGIN
  FOR r IN
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
    RETURNING tm.org_id AS r_org, tm.id AS r_timer, tm.execution_id AS r_execution, tm.token_id AS r_token,
              tm.kind AS r_kind, tm.due_at AS r_due, tm.payload_document AS r_payload
  LOOP
    UPDATE token AS t
    SET status = 'RUNNABLE', available_at = p_now, updated_at = p_now
    WHERE t.org_id = r.r_org AND t.id = r.r_token AND t.status = 'WAITING';
    v_woken := FOUND;
    PERFORM orcon_engine.append_event(r.r_org, r.r_execution,
      orcon_engine.derive_uuid('event', 'timer.fired', r.r_timer::text),
      'timer.fired', '{"tokenId":"' || r.r_token::text || '"}', '{"timerId":"' || r.r_timer::text || '","kind":"' || r.r_kind || '"}', p_now);
    IF v_woken THEN
      PERFORM orcon_engine.refresh_execution(r.r_org, r.r_execution, p_now);
    END IF;
    org_id := r.r_org;
    id := r.r_timer;
    execution_id := r.r_execution;
    token_id := r.r_token;
    kind := r.r_kind;
    due_at := r.r_due;
    payload_document := r.r_payload;
    RETURN NEXT;
  END LOOP;
END
$$;

-- Claims runnable tokens in fairness order with FOR UPDATE SKIP LOCKED (ADR-0013 section 3). The first claim of
-- a token of an execution starts it, with its event (ADR-0009 section 1: QUEUED to RUNNING when the first token is
-- claimed). The counters of the execution are refreshed by the next commit of a worker or by the reaper.
CREATE OR REPLACE FUNCTION orcon_engine.claim_tokens(p_worker_id uuid, p_fairness_keys text[], p_limit integer, p_lease_milliseconds bigint, p_now timestamptz)
RETURNS TABLE (org_id uuid, id uuid, execution_id uuid, node_id text, edge_id text, scope_path text,
               fork_lineage_document bytea, payload_reference text, fairness_key text, lease_epoch bigint, lease_expires_at timestamptz)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = orcon_engine, public AS
$$
#variable_conflict use_column
DECLARE
  r record;
BEGIN
  FOR r IN
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
    RETURNING t.org_id AS r_org, t.id AS r_id, t.execution_id AS r_execution, t.node_id AS r_node, t.edge_id AS r_edge,
              t.scope_path AS r_scope, t.fork_lineage_document AS r_lineage, t.payload_reference AS r_payload,
              t.fairness_key AS r_fairness, t.lease_epoch AS r_epoch, t.lease_expires_at AS r_expires
  LOOP
    UPDATE execution e
    SET status = 'RUNNING', started_at = COALESCE(e.started_at, p_now), updated_at = p_now
    WHERE e.org_id = r.r_org AND e.id = r.r_execution AND e.status = 'QUEUED';
    IF FOUND THEN
      PERFORM orcon_engine.append_event(r.r_org, r.r_execution,
        orcon_engine.derive_uuid('event', 'execution.started', r.r_execution::text), 'execution.started', '{}', NULL, p_now);
    END IF;
    org_id := r.r_org;
    id := r.r_id;
    execution_id := r.r_execution;
    node_id := r.r_node;
    edge_id := r.r_edge;
    scope_path := r.r_scope;
    fork_lineage_document := r.r_lineage;
    payload_reference := r.r_payload;
    fairness_key := r.r_fairness;
    lease_epoch := r.r_epoch;
    lease_expires_at := r.r_expires;
    RETURN NEXT;
  END LOOP;
END
$$;

ALTER FUNCTION orcon_engine.derive_uuid(text[]) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.derive_uuid(text[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.derive_uuid(text[]) TO orcon_application;
ALTER FUNCTION orcon_engine.append_event(uuid, uuid, uuid, text, text, text, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.append_event(uuid, uuid, uuid, text, text, text, timestamptz) FROM PUBLIC;
ALTER FUNCTION orcon_engine.refresh_execution(uuid, uuid, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.refresh_execution(uuid, uuid, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.refresh_execution(uuid, uuid, timestamptz) TO orcon_application;
ALTER FUNCTION orcon_engine.abandon_tokens(uuid[], bigint, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.abandon_tokens(uuid[], bigint, timestamptz) FROM PUBLIC;
ALTER FUNCTION orcon_engine.claim_tokens(uuid, text[], integer, bigint, timestamptz) OWNER TO orcon_engine;
ALTER FUNCTION orcon_engine.release_expired_leases(integer, bigint, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.release_expired_leases(integer, bigint, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.release_expired_leases(integer, bigint, timestamptz) TO orcon_application;
ALTER FUNCTION orcon_engine.release_worker_leases(uuid, bigint, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.release_worker_leases(uuid, bigint, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.release_worker_leases(uuid, bigint, timestamptz) TO orcon_application;
ALTER FUNCTION orcon_engine.fire_due_timers(integer, timestamptz) OWNER TO orcon_engine;
REVOKE ALL ON FUNCTION orcon_engine.fire_due_timers(integer, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION orcon_engine.fire_due_timers(integer, timestamptz) TO orcon_application;
