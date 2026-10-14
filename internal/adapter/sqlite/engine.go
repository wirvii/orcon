// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// The durable engine's use of the store (SPEC-001 C4a): opening an execution
// from its rows, committing a step, and the recovery work that moves tokens
// when a lease is lost. SQLite has one process and one writer, so what
// PostgreSQL does in orcon_engine functions is done here in one writer job.

// availableAt is the stored instant at which a token becomes claimable: the one
// given, or the clock of the caller plus the queue order in microseconds (one
// process owns the file, so its clock is the only clock).
func availableAt(given, now time.Time, queueOrder int64) int64 {
	if !given.IsZero() {
		return micros(given)
	}
	return micros(now) + queueOrder
}

// leaseExpiry is the stored expiry of a lease being created.
func leaseExpiry(tok *store.Token, now time.Time) sql.NullInt64 {
	if !tok.LeaseExpiresAt.IsZero() {
		return nullMicros(tok.LeaseExpiresAt)
	}
	if tok.LeaseTTL > 0 {
		return sql.NullInt64{Int64: micros(now.Add(tok.LeaseTTL)), Valid: true}
	}
	return sql.NullInt64{}
}

// livePageSize is the page of live tokens a snapshot reads at a time.
const livePageSize = 500

// DefaultPoisonThreshold is the number of abandonments after which a token is
// suspended as poison (ADR-0013 section 9).
const DefaultPoisonThreshold = store.DefaultPoisonThreshold

func tokenFromLive(row *sqlcgen.ListLiveTokensRow) (store.Token, error) {
	var d decode
	t := store.Token{
		ID: uuidField[store.TokenID](&d, row.ID), OrgID: uuidField[identity.OrgID](&d, row.OrgID),
		ExecutionID: uuidField[store.ExecutionID](&d, row.ExecutionID), Status: row.Status, NodeID: row.NodeID,
		EdgeID: nullString(row.EdgeID), ScopePath: row.ScopePath, ForkLineage: row.ForkLineageDocument,
		PayloadReference: nullString(row.PayloadReference), FairnessKey: row.FairnessKey, EffectivePriority: row.EffectivePriority,
		AvailableAt: instant(row.AvailableAt), LeaseOwner: nullUUIDField[execution.WorkerID](&d, row.LeaseOwner),
		LeaseEpoch: row.LeaseEpoch, LeaseExpiresAt: nullInstant(row.LeaseExpiresAt), ClaimCount: row.ClaimCount,
		AbandonedCount: row.AbandonedCount, SuspendReason: nullString(row.SuspendReason),
	}
	return t, d.err
}

func loadSnapshot(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, exec store.ExecutionID) (store.EngineSnapshot, error) {
	state, err := q.GetEngineState(ctx, uuidBytes(org), uuidBytes(exec))
	if err != nil {
		return store.EngineSnapshot{}, err
	}
	snap := store.EngineSnapshot{EngineState: store.EngineState{
		Status: state.Status, Revision: state.EngineStateRevision, Document: state.EngineStateDocument,
	}}
	after := make([]byte, 16)
	for {
		rows, err := q.ListLiveTokens(ctx, sqlcgen.ListLiveTokensParams{
			OrgID: uuidBytes(org), ExecutionID: uuidBytes(exec), AfterID: after, PageSize: livePageSize,
		})
		if err != nil {
			return store.EngineSnapshot{}, err
		}
		for i := range rows {
			t, err := tokenFromLive(&rows[i])
			if err != nil {
				return store.EngineSnapshot{}, err
			}
			snap.Tokens = append(snap.Tokens, t)
		}
		if len(rows) < livePageSize {
			return snap, nil
		}
		after = rows[len(rows)-1].ID
	}
}

// LoadSnapshot implements [store.Reader]: one read transaction, so the state
// and the tokens belong to one commit.
func (s *Store) LoadSnapshot(ctx context.Context, org identity.OrgID, exec store.ExecutionID) (store.EngineSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return store.EngineSnapshot{}, fmt.Errorf("load snapshot: %w", ErrClosed)
	}
	tx, err := s.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return store.EngineSnapshot{}, annotate("load snapshot", err)
	}
	defer func() { _ = tx.Rollback() }() // a read-only transaction has nothing to lose
	snap, err := loadSnapshot(ctx, sqlcgen.New(tx), org, exec)
	return snap, annotate("load snapshot", err)
}

// GetPayload implements [store.Reader].
func (s *Store) GetPayload(ctx context.Context, org identity.OrgID, payload store.PayloadID) ([]byte, error) {
	var out []byte
	err := s.read(ctx, "get payload", func(ctx context.Context, q *sqlcgen.Queries) error {
		doc, err := q.GetNodeOutputDocument(ctx, uuidBytes(org), uuidBytes(payload))
		if isNoRows(err) {
			doc, err = q.GetTokenInputDocument(ctx, uuidBytes(org), uuidBytes(payload))
		}
		out = doc
		return err
	})
	return out, err
}

// ListTokenNondeterminism implements [store.Reader].
func (s *Store) ListTokenNondeterminism(ctx context.Context, org identity.OrgID, token store.TokenID, limit int) ([]store.Nondeterminism, error) {
	var out []store.Nondeterminism
	err := s.read(ctx, "list token nondeterminism", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListTokenNondeterminism(ctx, uuidBytes(org), uuidBytes(token), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			var d decode
			n := store.Nondeterminism{
				OrgID: org, NodeExecutionID: uuidField[store.NodeExecutionID](&d, rows[i].NodeExecutionID),
				Sequence: rows[i].SequenceNumber, Document: rows[i].ValueDocument,
			}
			if d.err != nil {
				return d.err
			}
			out = append(out, n)
		}
		return nil
	})
	return out, err
}

// --- the writes of a step ---

func (t *sqliteTx) LockExecution(ctx context.Context, org identity.OrgID, exec store.ExecutionID) (store.EngineState, error) {
	if err := t.own(org); err != nil {
		return store.EngineState{}, err
	}
	row, err := t.q.GetEngineState(ctx, uuidBytes(org), uuidBytes(exec))
	if isNoRows(err) {
		return store.EngineState{}, flow.ErrNotFound
	}
	return store.EngineState{Status: row.Status, Revision: row.EngineStateRevision, Document: row.EngineStateDocument}, err
}

func (t *sqliteTx) LoadSnapshot(ctx context.Context, org identity.OrgID, exec store.ExecutionID) (store.EngineSnapshot, error) {
	if err := t.own(org); err != nil {
		return store.EngineSnapshot{}, err
	}
	snap, err := loadSnapshot(ctx, t.q, org, exec)
	if isNoRows(err) {
		return store.EngineSnapshot{}, flow.ErrNotFound
	}
	return snap, err
}

func (t *sqliteTx) GetPayload(ctx context.Context, org identity.OrgID, payload store.PayloadID) ([]byte, error) {
	if err := t.own(org); err != nil {
		return nil, err
	}
	doc, err := t.q.GetNodeOutputDocument(ctx, uuidBytes(org), uuidBytes(payload))
	if isNoRows(err) {
		doc, err = t.q.GetTokenInputDocument(ctx, uuidBytes(org), uuidBytes(payload))
	}
	if isNoRows(err) {
		return nil, flow.ErrNotFound
	}
	return doc, err
}

func (t *sqliteTx) SaveEngineState(ctx context.Context, u store.EngineStateUpdate) error {
	if err := t.own(u.OrgID); err != nil {
		return err
	}
	n, err := t.q.UpdateEngineState(ctx, sqlcgen.UpdateEngineStateParams{
		Status: u.Status, Document: u.Document, StartedAt: nullMicros(u.StartedAt), FinishedAt: nullMicros(u.FinishedAt),
		UpdatedAt: micros(u.Now), OrgID: uuidBytes(u.OrgID), ID: uuidBytes(u.Execution), ExpectedRevision: u.ExpectedRevision,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("save engine state of %s at revision %d: %w", u.Execution, u.ExpectedRevision, flow.ErrConflict)
	}
	return nil
}

func (t *sqliteTx) RefreshCounters(ctx context.Context, org identity.OrgID, exec store.ExecutionID, now time.Time) error {
	if err := t.own(org); err != nil {
		return err
	}
	return t.q.UpdateExecutionCounters(ctx, micros(now), uuidBytes(org), uuidBytes(exec))
}

func (t *sqliteTx) AdvanceToken(ctx context.Context, f store.Fence, u store.TokenUpdate, now time.Time) error {
	if err := t.own(f.OrgID); err != nil {
		return err
	}
	org, token, owner := fenceParams(f)
	n, err := t.q.UpdateAdvancedToken(ctx, sqlcgen.UpdateAdvancedTokenParams{
		NodeID: u.NodeID, EdgeID: nullText(u.EdgeID), ScopePath: u.ScopePath, ForkLineageDocument: u.ForkLineage,
		PayloadReference: nullText(u.PayloadReference), ResetAbandoned: boolToInt(u.ResetAbandoned), UpdatedAt: micros(now),
		OrgID: org, ID: token, LeaseEpoch: f.Epoch, LeaseOwner: owner,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrFenced
	}
	return nil
}

func (t *sqliteTx) MoveHeldToken(ctx context.Context, f store.Fence, u store.TokenUpdate, now time.Time) error {
	if err := t.own(f.OrgID); err != nil {
		return err
	}
	if u.Status == "LEASED" {
		return fmt.Errorf("move held token to LEASED: use AdvanceToken: %w", flow.ErrInvalidRecord)
	}
	org, token, owner := fenceParams(f)
	n, err := t.q.UpdateMovedToken(ctx, sqlcgen.UpdateMovedTokenParams{
		Status: u.Status, NodeID: u.NodeID, EdgeID: nullText(u.EdgeID), ScopePath: u.ScopePath, ForkLineageDocument: u.ForkLineage,
		PayloadReference: nullText(u.PayloadReference), AvailableAt: sql.NullInt64{Int64: availableAt(u.AvailableAt, t.now(), u.QueueOrder), Valid: u.Status == "RUNNABLE"}, ResetAbandoned: boolToInt(u.ResetAbandoned), UpdatedAt: micros(now),
		OrgID: org, ID: token, LeaseEpoch: f.Epoch, LeaseOwner: owner,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrFenced
	}
	return nil
}

func (t *sqliteTx) ReleaseDeferredToken(ctx context.Context, org identity.OrgID, token store.TokenID, queueOrder int64, now time.Time) error {
	if err := t.own(org); err != nil {
		return err
	}
	n, err := t.q.ReleaseDeferredToken(ctx, sqlcgen.ReleaseDeferredTokenParams{
		AvailableAt: availableAt(time.Time{}, t.now(), queueOrder), UpdatedAt: micros(now), OrgID: uuidBytes(org), ID: uuidBytes(token),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("release deferred token: it is not deferred: %w", flow.ErrConflict)
	}
	return nil
}

func (t *sqliteTx) CancelToken(ctx context.Context, org identity.OrgID, token store.TokenID, now time.Time) (bool, error) {
	if err := t.own(org); err != nil {
		return false, err
	}
	n, err := t.q.UpdateCanceledToken(ctx, micros(now), uuidBytes(org), uuidBytes(token))
	return n == 1, err
}

func (t *sqliteTx) PutNodeOutput(ctx context.Context, o store.NodeOutput) error {
	if err := t.own(o.OrgID); err != nil {
		return err
	}
	return t.q.InsertNodeOutput(ctx, sqlcgen.InsertNodeOutputParams{
		ID: uuidBytes(o.ID), OrgID: uuidBytes(o.OrgID), ExecutionID: uuidBytes(o.ExecutionID), NodeExecutionID: uuidBytes(o.NodeExecutionID),
		Port: o.Port, ItemCount: o.ItemCount, Encoding: o.Encoding, InlineDocument: o.Document, CreatedAt: micros(o.CreatedAt),
	})
}

func (t *sqliteTx) PutTokenInput(ctx context.Context, i store.TokenInput) error {
	if err := t.own(i.OrgID); err != nil {
		return err
	}
	return t.q.InsertTokenInput(ctx, sqlcgen.InsertTokenInputParams{
		ID: uuidBytes(i.ID), OrgID: uuidBytes(i.OrgID), ExecutionID: uuidBytes(i.ExecutionID), ItemCount: i.ItemCount,
		Encoding: i.Encoding, InlineDocument: i.Document, CreatedAt: micros(i.CreatedAt),
	})
}

func (t *sqliteTx) RecordNondeterminism(ctx context.Context, n store.Nondeterminism) error {
	if err := t.own(n.OrgID); err != nil {
		return err
	}
	return t.q.InsertNondeterminismRecord(ctx, sqlcgen.InsertNondeterminismRecordParams{
		ID: uuidBytes(n.ID), OrgID: uuidBytes(n.OrgID), ExecutionID: uuidBytes(n.ExecutionID), NodeExecutionID: uuidBytes(n.NodeExecutionID),
		SequenceNumber: n.Sequence, ValueDocument: n.Document, CreatedAt: micros(n.CreatedAt),
	})
}

// --- recovery ---

// abandoned is a token whose lease an engine-plane job gave up.
type abandoned struct {
	org, token, execution []byte
	count, epoch          int64
	node                  string
	poisoned              bool
}

// stamp renders an instant the way the PostgreSQL functions do, for the ids of
// events that depend on it.
func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000000") }

// eventDocument builds a small JSON object from alternating keys and values;
// a value is a string, or an int64 for a number.
func eventDocument(kv ...any) []byte {
	members := make([]jsonvalue.Member, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		key, _ := kv[i].(string)
		switch v := kv[i+1].(type) {
		case string:
			members = append(members, jsonvalue.Member{Key: key, Value: jsonvalue.String(v)})
		case int64:
			members = append(members, jsonvalue.Member{Key: key, Value: jsonvalue.Int(v)})
		}
	}
	obj, err := jsonvalue.Object(members...)
	if err != nil {
		panic("sqlite: event document repeats a key: " + err.Error())
	}
	out, err := jsonvalue.Marshal(obj)
	if err != nil {
		panic("sqlite: event document cannot be written: " + err.Error())
	}
	return out
}

// appendRecoveryEvent appends an event of the engine plane to an execution and
// applies the outbox rule to it in the same writer job.
func appendRecoveryEvent(ctx context.Context, q *sqlcgen.Queries, rule store.OutboxRule, exec *sqlcgen.GetExecutionRow, ev *execution.Event, eventID id.UUID, ref, data []byte, now time.Time) error {
	p := sqlcgen.AppendExecutionEventParams{
		ID: uuidBytes(eventID), OrgID: exec.OrgID, ExecutionID: exec.ID, RootExecutionID: exec.RootExecutionID,
		OccurredAt: micros(now), Type: string(ev.Type), SchemaVersion: 1, ReferenceDocument: ref, DataDocument: data, CreatedAt: micros(now),
	}
	if data != nil {
		p.DataDocumentFormat = sql.NullString{String: "orcon/event-data@1", Valid: true}
	}
	seq, err := q.AppendExecutionEvent(ctx, p)
	if err != nil || rule == nil {
		return err
	}
	var d decode
	org := uuidField[identity.OrgID](&d, exec.OrgID)
	execID := uuidField[store.ExecutionID](&d, exec.ID)
	if d.err != nil {
		return d.err
	}
	ev.Execution = execution.ID(id.UUID(execID).String())
	ev.Sequence = int(seq)
	topic, payload, ok := rule(org, execID, *ev)
	if !ok {
		return nil
	}
	return q.InsertOutbox(ctx, sqlcgen.InsertOutboxParams{
		ID: uuidBytes(id.UUID(store.OutboxRowID(store.EventID(eventID)))), OrgID: exec.OrgID, Topic: topic, PayloadDocument: payload,
		CreatedAt: micros(now), UpdatedAt: micros(now),
	})
}

// refreshExecution recomputes the counters of an execution and applies the
// status moves that follow from them (ADR-0009 section 1): RUNNING or WAITING
// to SUSPENDED when only suspended tokens are left, WAITING or SUSPENDED to
// RUNNING when work is runnable again.
func refreshExecution(ctx context.Context, q *sqlcgen.Queries, rule store.OutboxRule, org, exec []byte, now time.Time) error {
	if err := q.UpdateExecutionCounters(ctx, micros(now), org, exec); err != nil {
		return err
	}
	row, err := q.GetExecution(ctx, org, exec)
	if err != nil {
		return err
	}
	next := row.Status
	switch {
	case (row.Status == "RUNNING" || row.Status == "WAITING") && row.RunnableCount == 0 && row.LeasedCount == 0 && row.SuspendedCount >= 1:
		next = "SUSPENDED"
	case (row.Status == "WAITING" || row.Status == "SUSPENDED") && row.RunnableCount+row.LeasedCount >= 1:
		next = "RUNNING"
	}
	if next == row.Status {
		return nil
	}
	if _, err := q.UpdateExecutionStatus(ctx, sqlcgen.UpdateExecutionStatusParams{Status: next, Now: micros(now), OrgID: org, ID: exec}); err != nil {
		return err
	}
	typ := "execution.resumed"
	if next == "SUSPENDED" {
		typ = "execution.suspended"
	}
	execID, err := uuidOf[id.UUID](exec)
	if err != nil {
		return err
	}
	evID := id.Derive("event", typ, execID.String(), row.Status, stamp(now))
	return appendRecoveryEvent(ctx, q, rule, &row, &execution.Event{Type: execution.EventType(typ)}, evID, []byte("{}"), nil, now)
}

// startExecution starts a queued execution on the claim of its first token.
func startExecution(ctx context.Context, q *sqlcgen.Queries, rule store.OutboxRule, org, exec []byte, now time.Time) error {
	n, err := q.UpdateStartedExecution(ctx, micros(now), org, exec)
	if err != nil || n == 0 {
		return err
	}
	row, err := q.GetExecution(ctx, org, exec)
	if err != nil {
		return err
	}
	execID, err := uuidOf[id.UUID](exec)
	if err != nil {
		return err
	}
	return appendRecoveryEvent(ctx, q, rule, &row, &execution.Event{Type: execution.EventExecutionStarted}, id.Derive("event", "execution.started", execID.String()), []byte("{}"), nil, now)
}

// afterAbandon finishes the release of tokens an update statement moved: the
// attempts of their node executions, the events, the attention request of a
// poisoned token and the counters of the executions, all in the writer job of
// the update (ADR-0013 sections 4 and 9).
func afterAbandon(ctx context.Context, q *sqlcgen.Queries, rule store.OutboxRule, rows []abandoned, counted bool, now time.Time) ([]store.Reaped, error) {
	out := make([]store.Reaped, 0, len(rows))
	for i := range rows {
		a := &rows[i]
		var d decode
		reaped := store.Reaped{
			OrgID: uuidField[identity.OrgID](&d, a.org), TokenID: uuidField[store.TokenID](&d, a.token),
			ExecutionID: uuidField[store.ExecutionID](&d, a.execution), AbandonedCount: a.count, Poisoned: a.poisoned,
		}
		if d.err != nil {
			return nil, d.err
		}
		if _, err := q.MarkTokenAttemptsAbandoned(ctx, sql.NullInt64{Int64: micros(now), Valid: true}, a.org, a.token); err != nil {
			return nil, err
		}
		exec, err := q.GetExecution(ctx, a.org, a.execution)
		if err != nil {
			return nil, err
		}
		if counted {
			tokenText := id.UUID(reaped.TokenID).String()
			epoch := strconv.FormatInt(a.epoch, 10)
			ref := eventDocument("tokenId", tokenText, "nodeId", a.node)
			data := eventDocument("abandonedCount", a.count)
			tokenEvent := execution.Event{Token: execution.TokenID(tokenText), Node: flow.NodeID(a.node)}
			expired := tokenEvent
			expired.Type = execution.EventType("lease.expired")
			if err := appendRecoveryEvent(ctx, q, rule, &exec, &expired, id.Derive("event", "lease.expired", tokenText, epoch), ref, data, now); err != nil {
				return nil, err
			}
			if a.poisoned {
				poisoned := tokenEvent
				poisoned.Type = execution.EventType("token.poisoned")
				if err := appendRecoveryEvent(ctx, q, rule, &exec, &poisoned, id.Derive("event", "token.poisoned", tokenText, epoch), ref, data, now); err != nil {
					return nil, err
				}
				err := q.InsertAttentionRequest(ctx, sqlcgen.InsertAttentionRequestParams{
					ID: uuidBytes(id.Derive("attention", "POISON_TOKEN", tokenText)), OrgID: a.org, ExecutionID: a.execution, ProjectID: exec.ProjectID,
					FlowID: exec.FlowID, NodeID: a.node, TokenID: a.token, Cause: "POISON_TOKEN", CauseReference: "poison/" + tokenText,
					NotifyClass: "NOTIFY", Status: "OPEN", GroupKey: "poison/" + id.UUID(reaped.ExecutionID).String(), ItemCount: 1,
					Summary: fmt.Sprintf("A token was abandoned %d times and is suspended", a.count), CreatedAt: micros(now), UpdatedAt: micros(now),
				})
				if err != nil {
					return nil, err
				}
			}
		}
		if err := refreshExecution(ctx, q, rule, a.org, a.execution, now); err != nil {
			return nil, err
		}
		out = append(out, reaped)
	}
	return out, nil
}

// ReleaseExpiredLeases implements [store.WorkQueue].
func (s *Store) ReleaseExpiredLeases(ctx context.Context, limit int, poisonThreshold int64, _ time.Time) ([]store.Reaped, error) {
	var out []store.Reaped
	now := s.clock.Now()
	err := s.write(ctx, "release expired leases", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ReleaseExpiredLeases(ctx, poisonThreshold, micros(now), clampPage(limit))
		if err != nil {
			return err
		}
		moved := make([]abandoned, len(rows))
		for i, r := range rows {
			moved[i] = abandoned{org: r.OrgID, token: r.ID, execution: r.ExecutionID, count: r.AbandonedCount, epoch: r.LeaseEpoch, node: r.NodeID, poisoned: r.Column7 == 1}
		}
		out, err = afterAbandon(ctx, q, s.outboxRule(), moved, true, now)
		return err
	})
	return out, err
}

// ReleaseWorkerLeases implements [store.WorkQueue]: the owner gives its tokens back.
func (s *Store) ReleaseWorkerLeases(ctx context.Context, worker execution.WorkerID, poisonThreshold int64, now time.Time) ([]store.Reaped, error) {
	return s.releaseWorker(ctx, worker, poisonThreshold, false)
}

// ReleaseLostLeases implements [store.WorkQueue]: nobody returned them, so each counts.
func (s *Store) ReleaseLostLeases(ctx context.Context, worker execution.WorkerID, poisonThreshold int64, now time.Time) ([]store.Reaped, error) {
	return s.releaseWorker(ctx, worker, poisonThreshold, true)
}

func (s *Store) releaseWorker(ctx context.Context, worker execution.WorkerID, poisonThreshold int64, counted bool) ([]store.Reaped, error) {
	var out []store.Reaped
	now := s.clock.Now()
	err := s.write(ctx, "release worker leases", func(ctx context.Context, q *sqlcgen.Queries) error {
		out = nil
		for {
			rows, err := q.ReleaseWorkerLeases(ctx, sqlcgen.ReleaseWorkerLeasesParams{
				Counts: boolToInt(counted), PoisonThreshold: poisonThreshold, Now: micros(now), WorkerID: uuidBytes(worker), PageSize: maxPage,
			})
			if err != nil {
				return err
			}
			moved := make([]abandoned, len(rows))
			for i, r := range rows {
				moved[i] = abandoned{org: r.OrgID, token: r.ID, execution: r.ExecutionID, count: r.AbandonedCount, epoch: r.LeaseEpoch, node: r.NodeID, poisoned: r.Column7 == 1}
			}
			page, err := afterAbandon(ctx, q, s.outboxRule(), moved, counted, now)
			if err != nil {
				return err
			}
			out = append(out, page...)
			if len(rows) < maxPage {
				return nil
			}
		}
	})
	return out, err
}

// ReleaseTokenLease implements [store.WorkQueue].
func (s *Store) ReleaseTokenLease(ctx context.Context, f store.Fence, poisonThreshold int64, _ time.Time) (store.Reaped, bool, error) {
	var out []store.Reaped
	now := s.clock.Now()
	err := s.write(ctx, "release token lease", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ReleaseTokenLease(ctx, sqlcgen.ReleaseTokenLeaseParams{
			PoisonThreshold: poisonThreshold, Now: micros(now), OrgID: uuidBytes(f.OrgID), TokenID: uuidBytes(f.Token),
			LeaseEpoch: f.Epoch, WorkerID: uuidBytes(f.Owner),
		})
		if err != nil {
			return err
		}
		moved := make([]abandoned, len(rows))
		for i, r := range rows {
			moved[i] = abandoned{org: r.OrgID, token: r.ID, execution: r.ExecutionID, count: r.AbandonedCount, epoch: r.LeaseEpoch, node: r.NodeID, poisoned: r.Column7 == 1}
		}
		out, err = afterAbandon(ctx, q, s.outboxRule(), moved, true, now)
		return err
	})
	if err != nil || len(out) == 0 {
		return store.Reaped{}, false, err
	}
	return out[0], true, nil
}

// releaseAllLeases is what a standalone process does at start (ADR-0013
// section 9): the lock file guarantees no other holder exists, so every token
// still LEASED is released, and the release counts as an abandonment so that a
// token that crashes the process every time reaches the poison threshold. It
// reports how many tokens it released.
func (s *Store) releaseAllLeases(ctx context.Context) (int, error) {
	n := 0
	threshold := s.opts.PoisonThreshold
	err := s.write(ctx, "release all leases", func(ctx context.Context, q *sqlcgen.Queries) error {
		n = 0
		now := s.clock.Now()
		for {
			rows, err := q.ReleaseAllLeases(ctx, threshold, micros(now), maxPage)
			if err != nil {
				return err
			}
			moved := make([]abandoned, len(rows))
			for i, r := range rows {
				moved[i] = abandoned{org: r.OrgID, token: r.ID, execution: r.ExecutionID, count: r.AbandonedCount, epoch: r.LeaseEpoch, node: r.NodeID, poisoned: r.Column7 == 1}
			}
			if _, err := afterAbandon(ctx, q, s.outboxRule(), moved, true, now); err != nil {
				return err
			}
			n += len(rows)
			if len(rows) < maxPage {
				return nil
			}
		}
	})
	return n, err
}

// FireDueTimers implements [store.WorkQueue]: the timers fire and the tokens
// that wait for them wake in one writer job, so a crash cannot leave a fired
// timer next to a token nothing wakes.
func (s *Store) FireDueTimers(ctx context.Context, limit int, _ time.Time) ([]store.FiredTimer, error) {
	var out []store.FiredTimer
	now := s.clock.Now()
	err := s.write(ctx, "fire due timers", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.MarkDueTimersFired(ctx, micros(now), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			var d decode
			ft := store.FiredTimer{
				OrgID: uuidField[identity.OrgID](&d, rows[i].OrgID), TimerID: uuidField[store.TimerID](&d, rows[i].ID),
				ExecutionID: uuidField[store.ExecutionID](&d, rows[i].ExecutionID), TokenID: uuidField[store.TokenID](&d, rows[i].TokenID),
				Kind: rows[i].Kind, DueAt: instant(rows[i].DueAt),
			}
			if d.err != nil {
				return d.err
			}
			woken, err := q.UpdateWokenToken(ctx, micros(now), rows[i].OrgID, rows[i].TokenID)
			if err != nil {
				return err
			}
			exec, err := q.GetExecution(ctx, rows[i].OrgID, rows[i].ExecutionID)
			if err != nil {
				return err
			}
			ref := eventDocument("tokenId", id.UUID(ft.TokenID).String())
			data := eventDocument("timerId", id.UUID(ft.TimerID).String(), "kind", ft.Kind)
			fired := execution.Event{Type: execution.EventType("timer.fired"), Token: execution.TokenID(id.UUID(ft.TokenID).String()), Detail: ft.Kind}
			if err := appendRecoveryEvent(ctx, q, s.outboxRule(), &exec, &fired, id.Derive("event", "timer.fired", id.UUID(ft.TimerID).String()), ref, data, now); err != nil {
				return err
			}
			if woken == 1 {
				if err := refreshExecution(ctx, q, s.outboxRule(), rows[i].OrgID, rows[i].ExecutionID, now); err != nil {
					return err
				}
			}
			out = append(out, ft)
		}
		return nil
	})
	return out, err
}

func (t *sqliteTx) InsertAttempt(ctx context.Context, a store.TaskAttempt) error {
	if err := t.own(a.OrgID); err != nil {
		return err
	}
	return t.q.InsertTaskAttempt(ctx, sqlcgen.InsertTaskAttemptParams{
		ID: uuidBytes(a.ID), OrgID: uuidBytes(a.OrgID), ExecutionID: uuidBytes(a.ExecutionID), NodeExecutionID: uuidBytes(a.NodeExecutionID),
		AttemptNumber: a.AttemptNumber, LeaseEpoch: a.LeaseEpoch, WorkerID: nullUUIDBytes(a.Worker), Status: a.Status,
		StartedAt: micros(a.StartedAt), FinishedAt: nullMicros(a.FinishedAt), CreatedAt: micros(a.StartedAt),
	})
}

// ListAttempts implements [store.EngineReader].
func (s *Store) ListAttempts(ctx context.Context, org identity.OrgID, node store.NodeExecutionID, limit int) ([]store.TaskAttempt, error) {
	var out []store.TaskAttempt
	err := s.read(ctx, "list attempts", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListTaskAttempts(ctx, uuidBytes(org), uuidBytes(node), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			var d decode
			a := store.TaskAttempt{
				ID: uuidField[id.UUID](&d, rows[i].ID), OrgID: org, ExecutionID: uuidField[store.ExecutionID](&d, rows[i].ExecutionID),
				NodeExecutionID: node, AttemptNumber: rows[i].AttemptNumber, LeaseEpoch: rows[i].LeaseEpoch,
				Worker: nullUUIDField[execution.WorkerID](&d, rows[i].WorkerID), Status: rows[i].Status,
				StartedAt: instant(rows[i].StartedAt), FinishedAt: nullInstant(rows[i].FinishedAt),
			}
			if d.err != nil {
				return d.err
			}
			out = append(out, a)
		}
		return nil
	})
	return out, err
}
