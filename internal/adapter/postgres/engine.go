// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
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

func milliseconds(d time.Duration) int64 { return d.Milliseconds() }

// ClaimTokens implements [store.WorkQueue].
func (s *Store) ClaimTokens(ctx context.Context, r store.ClaimRequest) ([]store.Claimed, error) {
	var out []store.Claimed
	var keys []string
	if len(r.FairnessKeys) > 0 {
		keys = r.FairnessKeys
	}
	err := s.engineRows(ctx, "claim tokens",
		`SELECT org_id, id, execution_id, node_id, edge_id, scope_path, fork_lineage_document, payload_reference, fairness_key, lease_epoch, lease_expires_at
		 FROM orcon_engine.claim_tokens($1, $2, $3, $4, $5)`,
		[]any{pgUUID(r.Worker), keys, clampPage(r.Limit), milliseconds(r.LeaseTTL), timestamp(r.Now)},
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
	err := s.engineRows(ctx, "release expired leases",
		`SELECT org_id, id, execution_id, abandoned_count, is_poisoned FROM orcon_engine.release_expired_leases($1, $2, $3)`,
		[]any{clampPage(limit), poisonThreshold, timestamp(now)},
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
func (s *Store) ReleaseWorkerLeases(ctx context.Context, worker execution.WorkerID, now time.Time) ([]store.TokenRef, error) {
	var out []store.TokenRef
	err := s.engineRows(ctx, "release worker leases",
		`SELECT org_id, id FROM orcon_engine.release_worker_leases($1, $2)`,
		[]any{pgUUID(worker), timestamp(now)},
		func(rows pgx.Rows) error {
			var org, tok pgtype.UUID
			if err := rows.Scan(&org, &tok); err != nil {
				return err
			}
			out = append(out, store.TokenRef{OrgID: fromPGUUID[identity.OrgID](org), TokenID: fromPGUUID[store.TokenID](tok)})
			return nil
		})
	return out, err
}

// FireDueTimers implements [store.WorkQueue].
func (s *Store) FireDueTimers(ctx context.Context, limit int, now time.Time) ([]store.FiredTimer, error) {
	var out []store.FiredTimer
	err := s.engineRows(ctx, "fire due timers",
		`SELECT org_id, id, execution_id, token_id, kind, due_at FROM orcon_engine.fire_due_timers($1, $2)`,
		[]any{clampPage(limit), timestamp(now)},
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
