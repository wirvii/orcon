// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// The engine plane. sqlc cannot analyze functions that return a table, so
// these methods call the orcon_engine functions of migration 0008 with
// hand-written statements (data-access.md DA-8, orcon-db QY-10). The functions
// are SECURITY DEFINER and carry org_id in every row, so the calls run without an
// organization context; the plans of their bodies are checked through
// auto_explain in the adapter tests. Lease and recovery windows are counted by
// the database clock inside the functions (orcon-db TY-5).

// engineRows runs a statement of the engine plane and hands every row to scan.
func (s *Store) engineRows(ctx context.Context, op, statement string, args []any, scan func(pgx.Rows) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("%s: %w", op, ErrClosed)
	}
	rows, err := s.pool.Query(ctx, statement, args...)
	if err != nil {
		return mapError(op, err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return mapError(op, err)
		}
	}
	return mapError(op, rows.Err())
}

// engineRowsRouted runs a statement of the engine plane that may append events
// and applies the outbox rule to them in the same transaction: the statement
// runs, the events it appended are read back, the rows the rule publishes are
// inserted, and all of it commits together. Without a rule it is [Store.engineRows].
func (s *Store) engineRowsRouted(ctx context.Context, op, statement string, args []any, now time.Time, scan func(pgx.Rows) error) error {
	rule := s.outboxRule()
	if rule == nil {
		return s.engineRows(ctx, op, statement, args, scan)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("%s: %w", op, ErrClosed)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mapError(op, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := queryAll(ctx, tx, statement, args, scan); err != nil {
		return mapError(op, err)
	}
	type appended struct {
		org, exec, event pgtype.UUID
		typ              string
		ref              []byte
		seq              int64
	}
	var events []appended
	err = queryAll(ctx, tx, `SELECT org_id, execution_id, id, type, reference_document, sequence_number FROM orcon_engine.appended_events()`, nil,
		func(rows pgx.Rows) error {
			var a appended
			if err := rows.Scan(&a.org, &a.exec, &a.event, &a.typ, &a.ref, &a.seq); err != nil {
				return err
			}
			events = append(events, a)
			return nil
		})
	if err != nil {
		return mapError(op, err)
	}
	for i := range events {
		a := &events[i]
		org, exec := fromPGUUID[identity.OrgID](a.org), fromPGUUID[store.ExecutionID](a.exec)
		topic, payload, ok := rule(org, exec, engineEvent(a.typ, exec, a.ref, a.seq))
		if !ok {
			continue
		}
		outbox := store.OutboxRowID(fromPGUUID[store.EventID](a.event))
		if _, err := tx.Exec(ctx, `SELECT orcon_engine.enqueue_outbox($1, $2, $3, $4, $5)`, a.org, pgUUID(outbox), topic, payload, timestamp(now)); err != nil {
			return mapError(op, err)
		}
	}
	return mapError(op, tx.Commit(ctx))
}

// queryAll runs a query on a transaction and hands every row to scan.
func queryAll(ctx context.Context, tx pgx.Tx, statement string, args []any, scan func(pgx.Rows) error) error {
	rows, err := tx.Query(ctx, statement, args...)
	if err != nil {
		return err //nolint:wrapcheck // the caller maps the driver error once
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err() //nolint:wrapcheck // the caller maps the driver error once
}

// engineEvent builds the event the outbox rule sees from a stored one: the
// token and the node come from its reference document.
func engineEvent(typ string, exec store.ExecutionID, ref []byte, seq int64) execution.Event {
	var doc struct {
		TokenID string `json:"tokenId"`
		NodeID  string `json:"nodeId"`
	}
	_ = json.Unmarshal(ref, &doc) // a reference that does not parse leaves the fields empty
	return execution.Event{
		Type: execution.EventType(typ), Execution: execution.ID(id.UUID(exec).String()), Sequence: int(seq),
		Token: execution.TokenID(doc.TokenID), Node: flow.NodeID(doc.NodeID),
	}
}

func milliseconds(d time.Duration) int64 { return d.Milliseconds() }

// ClaimTokens implements [store.WorkQueue].
func (s *Store) ClaimTokens(ctx context.Context, r store.ClaimRequest) ([]store.Claimed, error) {
	var out []store.Claimed
	var keys []string
	if len(r.FairnessKeys) > 0 {
		keys = r.FairnessKeys
	}
	err := s.engineRowsRouted(ctx, "claim tokens",
		`SELECT org_id, id, execution_id, node_id, edge_id, scope_path, fork_lineage_document, payload_reference, fairness_key, lease_epoch, lease_expires_at
		 FROM orcon_engine.claim_tokens($1, $2, $3, $4, $5)`,
		[]any{pgUUID(r.Worker), keys, clampPage(r.Limit), milliseconds(r.LeaseTTL), timestamp(r.Now)},
		r.Now,
		func(rows pgx.Rows) error {
			var org, tok, exec pgtype.UUID
			var nodeID, scope, fairness string
			var edge, payload pgtype.Text
			var lineage []byte
			var epoch int64
			var expires pgtype.Timestamptz
			if err := rows.Scan(&org, &tok, &exec, &nodeID, &edge, &scope, &lineage, &payload, &fairness, &epoch, &expires); err != nil {
				return err
			}
			out = append(out, store.Claimed{
				OrgID: fromPGUUID[identity.OrgID](org), TokenID: fromPGUUID[store.TokenID](tok), ExecutionID: fromPGUUID[store.ExecutionID](exec),
				NodeID: nodeID, EdgeID: edge.String, ScopePath: scope, ForkLineage: lineage, PayloadReference: payload.String,
				FairnessKey: fairness, LeaseEpoch: epoch, LeaseExpiresAt: instant(expires),
			})
			return nil
		})
	return out, err
}

// RenewLeases implements [store.WorkQueue].
func (s *Store) RenewLeases(ctx context.Context, worker execution.WorkerID, ttl time.Duration, now time.Time) ([]store.Renewed, error) {
	var out []store.Renewed
	err := s.engineRows(ctx, "renew leases",
		`SELECT org_id, id, lease_epoch, is_cancel_requested FROM orcon_engine.renew_leases($1, $2, $3)`,
		[]any{pgUUID(worker), milliseconds(ttl), timestamp(now)},
		func(rows pgx.Rows) error {
			var org, tok pgtype.UUID
			var epoch int64
			var cancel bool
			if err := rows.Scan(&org, &tok, &epoch, &cancel); err != nil {
				return err
			}
			out = append(out, store.Renewed{OrgID: fromPGUUID[identity.OrgID](org), TokenID: fromPGUUID[store.TokenID](tok), LeaseEpoch: epoch, CancelRequested: cancel})
			return nil
		})
	return out, err
}

// ReleaseExpiredLeases implements [store.WorkQueue].
func (s *Store) ReleaseExpiredLeases(ctx context.Context, limit int, poisonThreshold int64, now time.Time) ([]store.Reaped, error) {
	var out []store.Reaped
	err := s.engineRowsRouted(ctx, "release expired leases",
		`SELECT org_id, id, execution_id, abandoned_count, is_poisoned FROM orcon_engine.release_expired_leases($1, $2, $3)`,
		[]any{clampPage(limit), poisonThreshold, timestamp(now)},
		now,
		func(rows pgx.Rows) error {
			var org, tok, exec pgtype.UUID
			var abandoned int64
			var poisoned bool
			if err := rows.Scan(&org, &tok, &exec, &abandoned, &poisoned); err != nil {
				return err
			}
			out = append(out, store.Reaped{OrgID: fromPGUUID[identity.OrgID](org), TokenID: fromPGUUID[store.TokenID](tok), ExecutionID: fromPGUUID[store.ExecutionID](exec), AbandonedCount: abandoned, Poisoned: poisoned})
			return nil
		})
	return out, err
}

// ReleaseWorkerLeases implements [store.WorkQueue].
func (s *Store) ReleaseWorkerLeases(ctx context.Context, worker execution.WorkerID, poisonThreshold int64, now time.Time) ([]store.Reaped, error) {
	var out []store.Reaped
	err := s.engineRowsRouted(ctx, "release worker leases",
		`SELECT org_id, id, execution_id, abandoned_count, is_poisoned FROM orcon_engine.release_worker_leases($1, $2, $3)`,
		[]any{pgUUID(worker), poisonThreshold, timestamp(now)},
		now,
		func(rows pgx.Rows) error {
			var org, tok, exec pgtype.UUID
			var abandoned int64
			var poisoned bool
			if err := rows.Scan(&org, &tok, &exec, &abandoned, &poisoned); err != nil {
				return err
			}
			out = append(out, store.Reaped{OrgID: fromPGUUID[identity.OrgID](org), TokenID: fromPGUUID[store.TokenID](tok), ExecutionID: fromPGUUID[store.ExecutionID](exec), AbandonedCount: abandoned, Poisoned: poisoned})
			return nil
		})
	return out, err
}

// ReleaseTokenLease implements [store.WorkQueue].
func (s *Store) ReleaseTokenLease(ctx context.Context, f store.Fence, poisonThreshold int64, now time.Time) (store.Reaped, bool, error) {
	var out []store.Reaped
	err := s.engineRowsRouted(ctx, "release token lease",
		`SELECT org_id, id, execution_id, abandoned_count, is_poisoned FROM orcon_engine.release_token_lease($1, $2, $3, $4, $5)`,
		[]any{pgUUID(f.Token), f.Epoch, pgUUID(f.Owner), poisonThreshold, timestamp(now)},
		now,
		func(rows pgx.Rows) error {
			var org, tok, exec pgtype.UUID
			var abandoned int64
			var poisoned bool
			if err := rows.Scan(&org, &tok, &exec, &abandoned, &poisoned); err != nil {
				return err
			}
			out = append(out, store.Reaped{OrgID: fromPGUUID[identity.OrgID](org), TokenID: fromPGUUID[store.TokenID](tok), ExecutionID: fromPGUUID[store.ExecutionID](exec), AbandonedCount: abandoned, Poisoned: poisoned})
			return nil
		})
	if err != nil || len(out) == 0 {
		return store.Reaped{}, false, err
	}
	return out[0], true, nil
}

// FireDueTimers implements [store.WorkQueue].
func (s *Store) FireDueTimers(ctx context.Context, limit int, now time.Time) ([]store.FiredTimer, error) {
	var out []store.FiredTimer
	err := s.engineRowsRouted(ctx, "fire due timers",
		`SELECT org_id, id, execution_id, token_id, kind, due_at FROM orcon_engine.fire_due_timers($1, $2)`,
		[]any{clampPage(limit), timestamp(now)},
		now,
		func(rows pgx.Rows) error {
			var org, tm, exec, tok pgtype.UUID
			var kind string
			var due pgtype.Timestamptz
			if err := rows.Scan(&org, &tm, &exec, &tok, &kind, &due); err != nil {
				return err
			}
			out = append(out, store.FiredTimer{OrgID: fromPGUUID[identity.OrgID](org), TimerID: fromPGUUID[store.TimerID](tm), ExecutionID: fromPGUUID[store.ExecutionID](exec), TokenID: fromPGUUID[store.TokenID](tok), Kind: kind, DueAt: instant(due)})
			return nil
		})
	return out, err
}

// ClaimOutbox implements [store.Deliveries].
func (s *Store) ClaimOutbox(ctx context.Context, worker execution.WorkerID, limit int, ttl time.Duration, now time.Time) ([]store.ClaimedOutbox, error) {
	var out []store.ClaimedOutbox
	err := s.engineRows(ctx, "claim outbox",
		`SELECT org_id, id, topic, payload_document, attempt_count FROM orcon_engine.claim_outbox($1, $2, $3, $4)`,
		[]any{pgUUID(worker), clampPage(limit), milliseconds(ttl), timestamp(now)},
		func(rows pgx.Rows) error {
			var org, ob pgtype.UUID
			var topic string
			var payload []byte
			var attempts int64
			if err := rows.Scan(&org, &ob, &topic, &payload, &attempts); err != nil {
				return err
			}
			out = append(out, store.ClaimedOutbox{OrgID: fromPGUUID[identity.OrgID](org), OutboxID: fromPGUUID[store.OutboxID](ob), Topic: topic, Payload: payload, AttemptCount: attempts})
			return nil
		})
	return out, err
}

// ResolveWaitSecret implements [store.WaitResolver].
func (s *Store) ResolveWaitSecret(ctx context.Context, wait store.WaitID) (store.WaitLookup, error) {
	var out store.WaitLookup
	found := false
	err := s.engineRows(ctx, "resolve wait secret",
		`SELECT org_id, id, secret_hash, status, expires_at FROM orcon_engine.resolve_wait_secret($1)`,
		[]any{pgUUID(wait)},
		func(rows pgx.Rows) error {
			var org, id pgtype.UUID
			var hash []byte
			var status string
			var expires pgtype.Timestamptz
			if err := rows.Scan(&org, &id, &hash, &status, &expires); err != nil {
				return err
			}
			found = true
			out = store.WaitLookup{OrgID: fromPGUUID[identity.OrgID](org), WaitID: fromPGUUID[store.WaitID](id), SecretHash: hash, Status: status, ExpiresAt: instant(expires)}
			return nil
		})
	if err == nil && !found {
		err = pgx.ErrNoRows
	}
	return out, mapError("resolve wait secret", err)
}
