// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// The durable engine's use of the store (SPEC-001 C4a): opening an execution
// from its rows and committing a step. The recovery work that moves tokens when
// a lease is lost runs in the orcon_engine functions of migrations 0008 and
// 0014 (engine.go).

// livePageSize is the page of live tokens a snapshot reads at a time.
const livePageSize = 500

func tokenFromLive(row *sqlcgen.ListLiveTokensRow) store.Token {
	return store.Token{
		ID: fromPGUUID[store.TokenID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID),
		ExecutionID: fromPGUUID[store.ExecutionID](row.ExecutionID), Status: row.Status, NodeID: row.NodeID,
		EdgeID: row.EdgeID.String, ScopePath: row.ScopePath, ForkLineage: row.ForkLineageDocument,
		PayloadReference: row.PayloadReference.String, FairnessKey: row.FairnessKey, EffectivePriority: row.EffectivePriority,
		AvailableAt: instant(row.AvailableAt), LeaseOwner: fromPGUUID[execution.WorkerID](row.LeaseOwner),
		LeaseEpoch: row.LeaseEpoch, LeaseExpiresAt: instant(row.LeaseExpiresAt), ClaimCount: row.ClaimCount,
		AbandonedCount: row.AbandonedCount, SuspendReason: row.SuspendReason.String,
	}
}

func loadSnapshot(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, exec store.ExecutionID) (store.EngineSnapshot, error) {
	state, err := q.GetEngineState(ctx, pgUUID(org), pgUUID(exec))
	if err != nil {
		return store.EngineSnapshot{}, err
	}
	snap := store.EngineSnapshot{EngineState: store.EngineState{
		Status: state.Status, Revision: state.EngineStateRevision, Document: state.EngineStateDocument,
	}}
	after := pgtype.UUID{Bytes: [16]byte{}, Valid: true}
	for {
		rows, err := q.ListLiveTokens(ctx, sqlcgen.ListLiveTokensParams{
			OrgID: pgUUID(org), ExecutionID: pgUUID(exec), AfterID: after, PageSize: livePageSize,
		})
		if err != nil {
			return store.EngineSnapshot{}, err
		}
		for i := range rows {
			snap.Tokens = append(snap.Tokens, tokenFromLive(&rows[i]))
		}
		if len(rows) < livePageSize {
			return snap, nil
		}
		after = rows[len(rows)-1].ID
	}
}

// LoadSnapshot implements [store.Reader]: one REPEATABLE READ read, so the state
// and the tokens belong to one commit.
func (s *Store) LoadSnapshot(ctx context.Context, org identity.OrgID, exec store.ExecutionID) (store.EngineSnapshot, error) {
	var out store.EngineSnapshot
	err := s.inOrgIso(ctx, "load snapshot", org, pgx.RepeatableRead, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		var err error
		out, err = loadSnapshot(ctx, q, org, exec)
		return err
	})
	return out, err
}

// GetPayload implements [store.Reader].
func (s *Store) GetPayload(ctx context.Context, org identity.OrgID, payload store.PayloadID) ([]byte, error) {
	var out []byte
	err := s.inOrg(ctx, "get payload", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		doc, err := q.GetNodeOutputDocument(ctx, pgUUID(org), pgUUID(payload))
		if err != nil && errorsIsNoRows(err) {
			doc, err = q.GetTokenInputDocument(ctx, pgUUID(org), pgUUID(payload))
		}
		out = doc
		return err
	})
	return out, err
}

// ListTokenNondeterminism implements [store.Reader].
func (s *Store) ListTokenNondeterminism(ctx context.Context, org identity.OrgID, token store.TokenID, limit int) ([]store.Nondeterminism, error) {
	var out []store.Nondeterminism
	err := s.inOrg(ctx, "list token nondeterminism", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListTokenNondeterminism(ctx, pgUUID(org), pgUUID(token), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, store.Nondeterminism{
				OrgID: org, NodeExecutionID: fromPGUUID[store.NodeExecutionID](rows[i].NodeExecutionID),
				Sequence: rows[i].SequenceNumber, Document: rows[i].ValueDocument,
			})
		}
		return nil
	})
	return out, err
}

// --- the writes of a step ---

func notFound(err error) error {
	if errorsIsNoRows(err) {
		return flow.ErrNotFound
	}
	return err
}

func (t *pgTx) LockExecution(ctx context.Context, org identity.OrgID, exec store.ExecutionID) (store.EngineState, error) {
	if err := t.own(org); err != nil {
		return store.EngineState{}, err
	}
	if _, err := t.q.LockExecution(ctx, pgUUID(org), pgUUID(exec)); err != nil {
		return store.EngineState{}, notFound(err)
	}
	row, err := t.q.GetEngineState(ctx, pgUUID(org), pgUUID(exec))
	if err != nil {
		return store.EngineState{}, notFound(err)
	}
	return store.EngineState{Status: row.Status, Revision: row.EngineStateRevision, Document: row.EngineStateDocument}, nil
}

func (t *pgTx) LoadSnapshot(ctx context.Context, org identity.OrgID, exec store.ExecutionID) (store.EngineSnapshot, error) {
	if err := t.own(org); err != nil {
		return store.EngineSnapshot{}, err
	}
	snap, err := loadSnapshot(ctx, t.q, org, exec)
	return snap, notFound(err)
}

func (t *pgTx) GetPayload(ctx context.Context, org identity.OrgID, payload store.PayloadID) ([]byte, error) {
	if err := t.own(org); err != nil {
		return nil, err
	}
	doc, err := t.q.GetNodeOutputDocument(ctx, pgUUID(org), pgUUID(payload))
	if err != nil && errorsIsNoRows(err) {
		doc, err = t.q.GetTokenInputDocument(ctx, pgUUID(org), pgUUID(payload))
	}
	if err != nil && errorsIsNoRows(err) {
		return nil, flow.ErrNotFound
	}
	return doc, err
}

func (t *pgTx) SaveEngineState(ctx context.Context, u store.EngineStateUpdate) error {
	if err := t.own(u.OrgID); err != nil {
		return err
	}
	// The status before the change decides which hourly counters move (SPEC-019 R-UX-BE-02): the counter
	// of a final status is raised when the execution reaches it and lowered when it leaves it, in this
	// transaction. Only the key of the execution is read, not the row.
	prior, found, err := readCountKey(ctx, t.q, u.OrgID, u.Execution)
	if err != nil {
		return err
	}
	if !found {
		return conflictf("save engine state of %s at revision %d", u.Execution, u.ExpectedRevision)
	}
	n, err := t.q.UpdateEngineState(ctx, sqlcgen.UpdateEngineStateParams{
		Status: u.Status, Document: u.Document, StartedAt: nullTimestamp(u.StartedAt), FinishedAt: nullTimestamp(u.FinishedAt),
		UpdatedAt: timestamp(u.Now), OrgID: pgUUID(u.OrgID), ID: pgUUID(u.Execution), ExpectedRevision: u.ExpectedRevision,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return conflictf("save engine state of %s at revision %d", u.Execution, u.ExpectedRevision)
	}
	return countTransition(ctx, t.q, u.OrgID, prior, u.Status, u.Now)
}

func (t *pgTx) RefreshCounters(ctx context.Context, org identity.OrgID, exec store.ExecutionID, now time.Time) error {
	if err := t.own(org); err != nil {
		return err
	}
	return t.q.UpdateExecutionCounters(ctx, timestamp(now), pgUUID(org), pgUUID(exec))
}

func (t *pgTx) AdvanceToken(ctx context.Context, f store.Fence, u store.TokenUpdate, now time.Time) error {
	if err := t.own(f.OrgID); err != nil {
		return err
	}
	n, err := t.q.UpdateAdvancedToken(ctx, sqlcgen.UpdateAdvancedTokenParams{
		NodeID: u.NodeID, EdgeID: nullText(u.EdgeID), ScopePath: u.ScopePath, ForkLineageDocument: u.ForkLineage,
		PayloadReference: nullText(u.PayloadReference), ResetAbandoned: u.ResetAbandoned, UpdatedAt: timestamp(now), OrgID: pgUUID(f.OrgID), ID: pgUUID(f.Token),
		LeaseEpoch: f.Epoch, LeaseOwner: pgUUID(f.Owner),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrFenced
	}
	return nil
}

func (t *pgTx) MoveHeldToken(ctx context.Context, f store.Fence, u store.TokenUpdate, now time.Time) error {
	if err := t.own(f.OrgID); err != nil {
		return err
	}
	if u.Status == "LEASED" {
		return invalidf("move held token to LEASED: use AdvanceToken")
	}
	n, err := t.q.UpdateMovedToken(ctx, sqlcgen.UpdateMovedTokenParams{
		Status: u.Status, NodeID: u.NodeID, EdgeID: nullText(u.EdgeID), ScopePath: u.ScopePath, ForkLineageDocument: u.ForkLineage,
		PayloadReference: nullText(u.PayloadReference), AvailableAt: nullTimestamp(u.AvailableAt), QueueOrder: u.QueueOrder, ResetAbandoned: u.ResetAbandoned, UpdatedAt: timestamp(now),
		OrgID: pgUUID(f.OrgID), ID: pgUUID(f.Token), LeaseEpoch: f.Epoch, LeaseOwner: pgUUID(f.Owner),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrFenced
	}
	return nil
}

func (t *pgTx) ReleaseDeferredToken(ctx context.Context, org identity.OrgID, token store.TokenID, queueOrder int64, now time.Time) error {
	if err := t.own(org); err != nil {
		return err
	}
	n, err := t.q.ReleaseDeferredToken(ctx, sqlcgen.ReleaseDeferredTokenParams{
		QueueOrder: queueOrder, UpdatedAt: timestamp(now), OrgID: pgUUID(org), ID: pgUUID(token),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return conflictf("release deferred token %s: it is not deferred", token)
	}
	return nil
}

func (t *pgTx) CancelToken(ctx context.Context, org identity.OrgID, token store.TokenID, now time.Time) (bool, error) {
	if err := t.own(org); err != nil {
		return false, err
	}
	n, err := t.q.UpdateCanceledToken(ctx, timestamp(now), pgUUID(org), pgUUID(token))
	return n == 1, err
}

func (t *pgTx) PutNodeOutput(ctx context.Context, o store.NodeOutput) error {
	if err := t.own(o.OrgID); err != nil {
		return err
	}
	return t.q.InsertNodeOutput(ctx, sqlcgen.InsertNodeOutputParams{
		ID: pgUUID(o.ID), OrgID: pgUUID(o.OrgID), ExecutionID: pgUUID(o.ExecutionID), NodeExecutionID: pgUUID(o.NodeExecutionID),
		Port: o.Port, ItemCount: o.ItemCount, Encoding: o.Encoding, InlineDocument: o.Document, CreatedAt: timestamp(o.CreatedAt),
	})
}

func (t *pgTx) PutTokenInput(ctx context.Context, i store.TokenInput) error {
	if err := t.own(i.OrgID); err != nil {
		return err
	}
	return t.q.InsertTokenInput(ctx, sqlcgen.InsertTokenInputParams{
		ID: pgUUID(i.ID), OrgID: pgUUID(i.OrgID), ExecutionID: pgUUID(i.ExecutionID), ItemCount: i.ItemCount,
		Encoding: i.Encoding, InlineDocument: i.Document, CreatedAt: timestamp(i.CreatedAt),
	})
}

func (t *pgTx) RecordNondeterminism(ctx context.Context, n store.Nondeterminism) error {
	if err := t.own(n.OrgID); err != nil {
		return err
	}
	return t.q.InsertNondeterminismRecord(ctx, sqlcgen.InsertNondeterminismRecordParams{
		ID: pgUUID(n.ID), OrgID: pgUUID(n.OrgID), ExecutionID: pgUUID(n.ExecutionID), NodeExecutionID: pgUUID(n.NodeExecutionID),
		SequenceNumber: n.Sequence, ValueDocument: n.Document, CreatedAt: timestamp(n.CreatedAt),
	})
}

func (t *pgTx) InsertAttempt(ctx context.Context, a store.TaskAttempt) error {
	if err := t.own(a.OrgID); err != nil {
		return err
	}
	return t.q.InsertTaskAttempt(ctx, sqlcgen.InsertTaskAttemptParams{
		ID: pgUUID(a.ID), OrgID: pgUUID(a.OrgID), ExecutionID: pgUUID(a.ExecutionID), NodeExecutionID: pgUUID(a.NodeExecutionID),
		AttemptNumber: a.AttemptNumber, LeaseEpoch: a.LeaseEpoch, WorkerID: nullPGUUID(a.Worker), Status: a.Status,
		StartedAt: timestamp(a.StartedAt), FinishedAt: nullTimestamp(a.FinishedAt), CreatedAt: timestamp(a.StartedAt),
	})
}

// ListAttempts implements [store.EngineReader].
func (s *Store) ListAttempts(ctx context.Context, org identity.OrgID, node store.NodeExecutionID, limit int) ([]store.TaskAttempt, error) {
	var out []store.TaskAttempt
	err := s.inOrg(ctx, "list attempts", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListTaskAttempts(ctx, pgUUID(org), pgUUID(node), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, store.TaskAttempt{
				ID: fromPGUUID[id.UUID](rows[i].ID), OrgID: org, ExecutionID: fromPGUUID[store.ExecutionID](rows[i].ExecutionID),
				NodeExecutionID: node, AttemptNumber: rows[i].AttemptNumber, LeaseEpoch: rows[i].LeaseEpoch,
				Worker: fromPGUUID[execution.WorkerID](rows[i].WorkerID), Status: rows[i].Status,
				StartedAt: instant(rows[i].StartedAt), FinishedAt: instant(rows[i].FinishedAt),
			})
		}
		return nil
	})
	return out, err
}
