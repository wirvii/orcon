-- SPDX-FileCopyrightText: The Orcon Authors
-- SPDX-License-Identifier: Apache-2.0

-- The work that crosses organizations (claiming, renewing, reaping, timers, the
-- outbox, wait lookups) is done by the orcon_engine functions of migration 0008.
-- sqlc cannot analyze functions that return a table, so engine.go calls them
-- with hand-written statements (data-access.md DA-8, orcon-db QY-10) and the
-- plans of their bodies are checked through auto_explain in the adapter tests.
-- The SQLite file of the same name holds the equivalent queries.

-- The caller holds the execution row until commit, so concurrent checkpoints of one execution append their events in
-- commit order (ADR-0009 section 7). The lock is FOR NO KEY UPDATE: FOR UPDATE would conflict with the FOR KEY SHARE
-- lock that inserting a child row (token, node execution, join arrival) takes on the execution, and two steps that
-- each insert a child and then append an event would deadlock.
-- name: LockExecution :one
-- orcon:scope org
SELECT id
FROM execution
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id)
FOR NO KEY UPDATE;

-- name: CountJoinArrivals :one
-- orcon:scope org
SELECT count(*) AS arrival_count, count(*) FILTER (WHERE is_live) AS live_count
FROM join_arrival
WHERE org_id = sqlc.arg(org_id) AND execution_id = sqlc.arg(execution_id) AND node_id = sqlc.arg(node_id)
  AND scope_path = sqlc.arg(scope_path) AND round_number = sqlc.arg(round_number);

-- Arrivals at one join run one after the other: a transaction that counts must see the arrivals committed before
-- it, so each takes the row of the join instance first (READ COMMITTED sees them at its next statement).
-- name: LockJoinInstance :one
-- orcon:scope org
SELECT id
FROM join_instance
WHERE org_id = sqlc.arg(org_id) AND execution_id = sqlc.arg(execution_id) AND node_id = sqlc.arg(node_id)
  AND scope_path = sqlc.arg(scope_path) AND round_number = sqlc.arg(round_number)
FOR NO KEY UPDATE;
