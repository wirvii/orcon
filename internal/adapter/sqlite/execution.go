// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

var (
	_ store.Reader        = (*Store)(nil)
	_ store.EngineReader  = (*Store)(nil)
	_ store.Inbox         = (*Store)(nil)
	_ store.Transactor    = (*Store)(nil)
	_ store.WorkQueue     = (*Store)(nil)
	_ store.LeaseRecovery = (*Store)(nil)
	_ store.Deliveries    = (*Store)(nil)
	_ store.WaitResolver  = (*Store)(nil)
)

// decode collects the first error of a sequence of identity conversions.
type decode struct{ err error }

func uuidField[T uuidLike](d *decode, b []byte) T {
	v, err := uuidOf[T](b)
	if err != nil && d.err == nil {
		d.err = err
	}
	return v
}

func nullUUIDField[T uuidLike](d *decode, b []byte) T {
	v, err := nullUUIDOf[T](b)
	if err != nil && d.err == nil {
		d.err = err
	}
	return v
}

func nullString(s sql.NullString) string { return s.String }

// firstPageBefore is the cursor of a newest-first list: after everything.
func beforeCursor(c store.Cursor) (createdAt int64, identity []byte) {
	if c.IsZero() {
		ff := make([]byte, 16)
		for i := range ff {
			ff[i] = 0xff
		}
		return math.MaxInt64, ff
	}
	return micros(c.CreatedAt), uuidBytes(c.ID)
}

// afterCursor is the cursor of an oldest-first list: before everything.
func afterCursor(c store.Cursor) (createdAt int64, identity []byte) {
	if c.IsZero() {
		return math.MinInt64, make([]byte, 16)
	}
	return micros(c.CreatedAt), uuidBytes(c.ID)
}

func executionFrom(row *sqlcgen.GetExecutionRow) (store.Execution, error) {
	var d decode
	e := store.Execution{
		ID: uuidField[store.ExecutionID](&d, row.ID), OrgID: uuidField[identity.OrgID](&d, row.OrgID),
		ProjectID: uuidField[identity.ProjectID](&d, row.ProjectID), FlowID: uuidField[flow.FlowID](&d, row.FlowID),
		FlowVersionID:  nullUUIDField[flow.FlowVersionID](&d, row.FlowVersionID),
		FlowSnapshotID: nullUUIDField[flow.FlowSnapshotID](&d, row.FlowSnapshotID),
		Status:         row.Status, Mode: row.Mode, Priority: row.Priority,
		ParentExecutionID: nullUUIDField[store.ExecutionID](&d, row.ParentExecutionID),
		RootExecutionID:   uuidField[store.ExecutionID](&d, row.RootExecutionID),
		CallDepth:         row.CallDepth, RunGeneration: row.RunGeneration,
		CancelRequestedAt: nullInstant(row.CancelRequestedAt),
		RunnableCount:     row.RunnableCount, LeasedCount: row.LeasedCount, WaitingCount: row.WaitingCount, SuspendedCount: row.SuspendedCount,
		StartedAt: nullInstant(row.StartedAt), FinishedAt: nullInstant(row.FinishedAt),
		CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
	return e, d.err
}

// GetExecution implements [store.Reader].
func (s *Store) GetExecution(ctx context.Context, org orgscope.Org, execID store.ExecutionID) (store.Execution, error) {
	var out store.Execution
	err := s.readOf(ctx, "get execution", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetExecution(ctx, uuidBytes(org.UUID()), uuidBytes(execID))
		if err != nil {
			return err
		}
		out, err = executionFrom(&row)
		return err
	})
	return out, err
}

// ListExecutions implements [store.Reader].
func (s *Store) ListExecutions(ctx context.Context, org orgscope.Org, filter store.ExecutionFilter, after store.Cursor, limit int) ([]store.Execution, error) {
	set := 0
	if !filter.ProjectID.IsZero() {
		set++
	}
	if !filter.FlowID.IsZero() {
		set++
	}
	if filter.Status != "" {
		set++
	}
	if set != 1 {
		return nil, fmt.Errorf("list executions: exactly one filter field must be set, got %d: %w", set, flow.ErrInvalidRecord)
	}
	beforeAt, beforeID := beforeCursor(after)
	var out []store.Execution
	err := s.readOf(ctx, "list executions", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		var rows []sqlcgen.GetExecutionRow
		switch {
		case !filter.ProjectID.IsZero():
			r, err := q.ListExecutions(ctx, sqlcgen.ListExecutionsParams{OrgID: uuidBytes(org.UUID()), ProjectID: uuidBytes(filter.ProjectID), BeforeCreatedAt: beforeAt, BeforeID: beforeID, PageSize: clampPage(limit)})
			if err != nil {
				return err
			}
			for i := range r {
				rows = append(rows, sqlcgen.GetExecutionRow(r[i]))
			}
		case !filter.FlowID.IsZero():
			r, err := q.ListFlowExecutions(ctx, sqlcgen.ListFlowExecutionsParams{OrgID: uuidBytes(org.UUID()), FlowID: uuidBytes(filter.FlowID), BeforeCreatedAt: beforeAt, BeforeID: beforeID, PageSize: clampPage(limit)})
			if err != nil {
				return err
			}
			for i := range r {
				rows = append(rows, sqlcgen.GetExecutionRow(r[i]))
			}
		default:
			r, err := q.ListExecutionsByStatus(ctx, sqlcgen.ListExecutionsByStatusParams{OrgID: uuidBytes(org.UUID()), Status: filter.Status, BeforeCreatedAt: beforeAt, BeforeID: beforeID, PageSize: clampPage(limit)})
			if err != nil {
				return err
			}
			for i := range r {
				rows = append(rows, sqlcgen.GetExecutionRow(r[i]))
			}
		}
		for i := range rows {
			e, err := executionFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return nil
	})
	return out, err
}

// GetToken implements [store.Reader].
func (s *Store) GetToken(ctx context.Context, org orgscope.Org, token store.TokenID) (store.Token, error) {
	var out store.Token
	err := s.readOf(ctx, "get token", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetToken(ctx, uuidBytes(org.UUID()), uuidBytes(token))
		if err != nil {
			return err
		}
		var d decode
		out = store.Token{
			ID: uuidField[store.TokenID](&d, row.ID), OrgID: uuidField[identity.OrgID](&d, row.OrgID),
			ExecutionID: uuidField[store.ExecutionID](&d, row.ExecutionID), Status: row.Status, NodeID: row.NodeID,
			EdgeID: nullString(row.EdgeID), ScopePath: row.ScopePath, ForkLineage: row.ForkLineageDocument,
			PayloadReference: nullString(row.PayloadReference), FairnessKey: row.FairnessKey,
			EffectivePriority: row.EffectivePriority, AvailableAt: instant(row.AvailableAt),
			LeaseOwner: nullUUIDField[execution.WorkerID](&d, row.LeaseOwner), LeaseEpoch: row.LeaseEpoch,
			LeaseExpiresAt: nullInstant(row.LeaseExpiresAt), ClaimCount: row.ClaimCount, AbandonedCount: row.AbandonedCount,
			SuspendReason: nullString(row.SuspendReason),
		}
		return d.err
	})
	return out, err
}

// ListExecutionEvents implements [store.Reader].
func (s *Store) ListExecutionEvents(ctx context.Context, org orgscope.Org, execID store.ExecutionID, after int64, limit int) ([]store.Event, error) {
	var out []store.Event
	err := s.readOf(ctx, "list execution events", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListExecutionEvents(ctx, sqlcgen.ListExecutionEventsParams{OrgID: uuidBytes(org.UUID()), ExecutionID: uuidBytes(execID), AfterSequenceNumber: after, PageSize: clampPage(limit)})
		if err != nil {
			return err
		}
		for i := range rows {
			var d decode
			e := store.Event{
				ID: uuidField[store.EventID](&d, rows[i].ID), OrgID: uuidField[identity.OrgID](&d, rows[i].OrgID),
				ExecutionID: uuidField[store.ExecutionID](&d, rows[i].ExecutionID), RootExecutionID: uuidField[store.ExecutionID](&d, rows[i].RootExecutionID),
				SequenceNumber: rows[i].SequenceNumber, OccurredAt: instant(rows[i].OccurredAt), Type: rows[i].Type,
				SchemaVersion: rows[i].SchemaVersion, Reference: rows[i].ReferenceDocument, Data: rows[i].DataDocument,
			}
			if d.err != nil {
				return d.err
			}
			out = append(out, e)
		}
		return nil
	})
	return out, err
}

func operationFrom(row *sqlcgen.GetExternalOperationRow) (store.ExternalOperation, error) {
	var d decode
	o := store.ExternalOperation{
		ID: uuidField[store.ExternalOperationID](&d, row.ID), OrgID: uuidField[identity.OrgID](&d, row.OrgID),
		ExecutionID: uuidField[store.ExecutionID](&d, row.ExecutionID), NodeExecutionID: uuidField[store.NodeExecutionID](&d, row.NodeExecutionID),
		ItemIndex: row.ItemIndex, CorrelationKey: row.CorrelationKey, ContractReference: row.ContractReference, State: row.State,
		BatchKey: nullString(row.BatchKey), DispatchDeadlineAt: nullInstant(row.DispatchDeadlineAt),
		FirstDispatchedAt: nullInstant(row.FirstDispatchedAt), RetryEpoch: row.RetryEpoch, ResponseReference: nullString(row.ResponseReference),
	}
	return o, d.err
}

// GetExternalOperation implements [store.Reader].
func (s *Store) GetExternalOperation(ctx context.Context, org orgscope.Org, operation store.ExternalOperationID) (store.ExternalOperation, error) {
	var out store.ExternalOperation
	err := s.readOf(ctx, "get external operation", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetExternalOperation(ctx, uuidBytes(org.UUID()), uuidBytes(operation))
		if err != nil {
			return err
		}
		out, err = operationFrom(&row)
		return err
	})
	return out, err
}

// ListExternalOperations implements [store.Reader].
func (s *Store) ListExternalOperations(ctx context.Context, org orgscope.Org, node store.NodeExecutionID, fromItem int64, limit int) ([]store.ExternalOperation, error) {
	var out []store.ExternalOperation
	err := s.readOf(ctx, "list external operations", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListExternalOperations(ctx, sqlcgen.ListExternalOperationsParams{OrgID: uuidBytes(org.UUID()), NodeExecutionID: uuidBytes(node), FromItemIndex: fromItem, PageSize: clampPage(limit)})
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetExternalOperationRow(rows[i])
			o, err := operationFrom(&converted)
			if err != nil {
				return err
			}
			out = append(out, o)
		}
		return nil
	})
	return out, err
}

// ListAttentionRequests implements [store.Inbox].
func (s *Store) ListAttentionRequests(ctx context.Context, org orgscope.Org, status string, after store.Cursor, limit int) ([]store.AttentionRequest, error) {
	afterAt, afterID := afterCursor(after)
	var out []store.AttentionRequest
	err := s.readOf(ctx, "list attention requests", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListAttentionRequests(ctx, sqlcgen.ListAttentionRequestsParams{OrgID: uuidBytes(org.UUID()), Status: status, AfterCreatedAt: afterAt, AfterID: afterID, PageSize: clampPage(limit)})
		if err != nil {
			return err
		}
		for i := range rows {
			var d decode
			r := &rows[i]
			a := store.AttentionRequest{
				ID: uuidField[store.AttentionRequestID](&d, r.ID), OrgID: uuidField[identity.OrgID](&d, r.OrgID),
				ExecutionID: uuidField[store.ExecutionID](&d, r.ExecutionID), ProjectID: uuidField[identity.ProjectID](&d, r.ProjectID),
				FlowID: uuidField[flow.FlowID](&d, r.FlowID), NodeID: r.NodeID, TokenID: nullUUIDField[store.TokenID](&d, r.TokenID),
				Cause: r.Cause, CauseReference: r.CauseReference, NotifyClass: r.NotifyClass, Status: r.Status, Level: r.Level,
				GroupKey: r.GroupKey, ItemCount: r.ItemCount, Summary: r.Summary, CreatedAt: instant(r.CreatedAt), UpdatedAt: instant(r.UpdatedAt),
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

// GetTimer implements [store.Inbox].
func (s *Store) GetTimer(ctx context.Context, org orgscope.Org, timer store.TimerID) (store.Timer, error) {
	var out store.Timer
	err := s.readOf(ctx, "get timer", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetTimer(ctx, uuidBytes(org.UUID()), uuidBytes(timer))
		if err != nil {
			return err
		}
		var d decode
		out = store.Timer{
			ID: uuidField[store.TimerID](&d, row.ID), OrgID: uuidField[identity.OrgID](&d, row.OrgID),
			ExecutionID: uuidField[store.ExecutionID](&d, row.ExecutionID), TokenID: uuidField[store.TokenID](&d, row.TokenID),
			Kind: row.Kind, DueAt: instant(row.DueAt), Status: row.Status,
		}
		return d.err
	})
	return out, err
}

// GetJoinInstance implements [store.Inbox].
func (s *Store) GetJoinInstance(ctx context.Context, key store.JoinKey) (store.JoinInstance, error) {
	var out store.JoinInstance
	err := s.read(ctx, "get join instance", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetJoinInstance(ctx, sqlcgen.GetJoinInstanceParams{
			OrgID: uuidBytes(key.OrgID), ExecutionID: uuidBytes(key.ExecutionID), NodeID: key.NodeID, ScopePath: key.ScopePath, RoundNumber: key.RoundNumber,
		})
		if err != nil {
			return err
		}
		var d decode
		out = store.JoinInstance{
			ID: uuidField[store.JoinInstanceID](&d, row.ID), Key: key, Status: row.Status,
			ContinuationTokenID: nullUUIDField[store.TokenID](&d, row.ContinuationTokenID),
		}
		return d.err
	})
	return out, err
}

// InTx implements [store.Transactor]: the function runs in the single writer,
// under a savepoint of the group commit, so an error rolls back its writes only.
func (s *Store) InTx(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx store.Tx) error) error {
	if org.IsZero() {
		return identity.ErrNoScope
	}
	return s.write(ctx, "transaction", func(ctx context.Context, q *sqlcgen.Queries) error {
		return fn(ctx, &sqliteTx{q: q, org: identity.OrgID(org.UUID()), now: s.clock.Now})
	})
}

type sqliteTx struct {
	q   *sqlcgen.Queries
	org identity.OrgID
	// now is the clock of the store: the instants a lease and the availability of a
	// token are counted from.
	now func() time.Time
}

var errOtherOrganization = errors.New("the record belongs to another organization than the transaction")

func (t *sqliteTx) own(org identity.OrgID) error {
	if org != t.org {
		return fmt.Errorf("%w: %w", flow.ErrInvalidRecord, errOtherOrganization)
	}
	return nil
}

func fenceParams(f store.Fence) (org, token, owner []byte) {
	return uuidBytes(f.OrgID), uuidBytes(f.Token), uuidBytes(f.Owner)
}

func (t *sqliteTx) AssertFence(ctx context.Context, f store.Fence, now time.Time) error {
	if err := t.own(f.OrgID); err != nil {
		return err
	}
	org, token, owner := fenceParams(f)
	n, err := t.q.MarkTokenFenced(ctx, sqlcgen.MarkTokenFencedParams{UpdatedAt: micros(now), OrgID: org, ID: token, LeaseEpoch: f.Epoch, LeaseOwner: owner})
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrFenced
	}
	return nil
}

func (t *sqliteTx) MoveToken(ctx context.Context, f store.Fence, to string, now time.Time) error {
	if err := t.own(f.OrgID); err != nil {
		return err
	}
	if to == "LEASED" {
		return fmt.Errorf("move token to LEASED: only a claim leases a token: %w", flow.ErrInvalidRecord)
	}
	org, token, owner := fenceParams(f)
	n, err := t.q.UpdateTokenFenced(ctx, sqlcgen.UpdateTokenFencedParams{Status: to, UpdatedAt: micros(now), OrgID: org, ID: token, LeaseEpoch: f.Epoch, LeaseOwner: owner})
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrFenced
	}
	return nil
}

func (t *sqliteTx) InsertExecution(ctx context.Context, e *store.Execution) error {
	if err := t.own(e.OrgID); err != nil {
		return err
	}
	root := e.RootExecutionID
	if root.IsZero() {
		root = e.ID
	}
	return t.q.InsertExecution(ctx, sqlcgen.InsertExecutionParams{
		ID: uuidBytes(e.ID), OrgID: uuidBytes(e.OrgID), ProjectID: uuidBytes(e.ProjectID), FlowID: uuidBytes(e.FlowID),
		FlowVersionID: nullUUIDBytes(e.FlowVersionID), FlowSnapshotID: nullUUIDBytes(e.FlowSnapshotID),
		Status: e.Status, Mode: e.Mode, Priority: e.Priority, ParentExecutionID: nullUUIDBytes(e.ParentExecutionID),
		RootExecutionID: uuidBytes(root), CallDepth: e.CallDepth, RunnableCount: e.RunnableCount,
		CreatedAt: micros(e.CreatedAt), UpdatedAt: micros(e.UpdatedAt),
	})
}

func (t *sqliteTx) InsertToken(ctx context.Context, tok *store.Token) error {
	if err := t.own(tok.OrgID); err != nil {
		return err
	}
	return t.q.InsertToken(ctx, sqlcgen.InsertTokenParams{
		ID: uuidBytes(tok.ID), OrgID: uuidBytes(tok.OrgID), ExecutionID: uuidBytes(tok.ExecutionID), Status: tok.Status,
		NodeID: tok.NodeID, EdgeID: nullText(tok.EdgeID), ScopePath: tok.ScopePath, ForkLineageDocument: tok.ForkLineage,
		PayloadReference: nullText(tok.PayloadReference), FairnessKey: tok.FairnessKey, EffectivePriority: tok.EffectivePriority,
		AvailableAt: availableAt(tok.AvailableAt, t.now(), tok.QueueOrder), LeaseOwner: nullUUIDBytes(tok.LeaseOwner), LeaseEpoch: tok.LeaseEpoch,
		LeaseExpiresAt: leaseExpiry(tok, t.now()), CreatedAt: micros(tok.CreatedAt), UpdatedAt: micros(tok.UpdatedAt),
	})
}

func (t *sqliteTx) InsertNodeExecution(ctx context.Context, n store.NodeExecution) error {
	if err := t.own(n.OrgID); err != nil {
		return err
	}
	return t.q.InsertNodeExecution(ctx, sqlcgen.InsertNodeExecutionParams{
		ID: uuidBytes(n.ID), OrgID: uuidBytes(n.OrgID), ExecutionID: uuidBytes(n.ExecutionID), TokenID: uuidBytes(n.TokenID),
		NodeID: n.NodeID, ScopePath: n.ScopePath, Status: n.Status, StartedAt: nullMicros(n.StartedAt), FinishedAt: nullMicros(n.FinishedAt),
		CreatedAt: micros(n.CreatedAt), UpdatedAt: micros(n.UpdatedAt),
	})
}

func (t *sqliteTx) AppendEvent(ctx context.Context, e store.Event) (int64, error) {
	if err := t.own(e.OrgID); err != nil {
		return 0, err
	}
	root := e.RootExecutionID
	if root.IsZero() {
		root = e.ExecutionID
	}
	p := sqlcgen.AppendExecutionEventParams{
		ID: uuidBytes(e.ID), OrgID: uuidBytes(e.OrgID), ExecutionID: uuidBytes(e.ExecutionID), RootExecutionID: uuidBytes(root),
		OccurredAt: micros(e.OccurredAt), Type: e.Type, SchemaVersion: e.SchemaVersion, ReferenceDocument: e.Reference,
		DataDocument: e.Data, CreatedAt: micros(e.CreatedAt),
	}
	if e.Data != nil {
		p.DataDocumentFormat = sql.NullString{String: "orcon/event-data@1", Valid: true}
	}
	return t.q.AppendExecutionEvent(ctx, p)
}

func (t *sqliteTx) AppendJournal(ctx context.Context, operations []store.ExternalOperation) error {
	for i := range operations {
		o := &operations[i]
		if err := t.own(o.OrgID); err != nil {
			return err
		}
		if err := t.q.InsertExternalOperation(ctx, sqlcgen.InsertExternalOperationParams{
			ID: uuidBytes(o.ID), OrgID: uuidBytes(o.OrgID), ExecutionID: uuidBytes(o.ExecutionID), NodeExecutionID: uuidBytes(o.NodeExecutionID),
			ItemIndex: o.ItemIndex, CorrelationKey: o.CorrelationKey, ContractReference: o.ContractReference, BatchKey: nullText(o.BatchKey),
			CreatedAt: micros(o.CreatedAt), UpdatedAt: micros(o.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (t *sqliteTx) MarkDispatching(ctx context.Context, operations []store.ExternalOperationID, deadline, now time.Time) (int64, error) {
	var moved int64
	for _, op := range operations {
		n, err := t.q.MarkExternalOperationDispatching(ctx, sqlcgen.MarkExternalOperationDispatchingParams{
			DispatchDeadlineAt: nullMicros(deadline), DispatchedAt: nullMicros(now), UpdatedAt: micros(now), OrgID: uuidBytes(t.org), ID: uuidBytes(op),
		})
		if err != nil {
			return moved, err
		}
		moved += n
	}
	return moved, nil
}

func (t *sqliteTx) ResolveOperation(ctx context.Context, operation store.ExternalOperationID, from, to, responseReference string, now time.Time) (bool, error) {
	n, err := t.q.MarkExternalOperationOutcome(ctx, sqlcgen.MarkExternalOperationOutcomeParams{
		NextState: to, ResponseReference: nullText(responseReference), UpdatedAt: micros(now), OrgID: uuidBytes(t.org), ID: uuidBytes(operation), ExpectedState: from,
	})
	return n == 1, err
}

func (t *sqliteTx) RecordJoinArrival(ctx context.Context, a store.JoinArrival, now time.Time) (store.JoinProgress, error) {
	if err := t.own(a.Key.OrgID); err != nil {
		return store.JoinProgress{}, err
	}
	k := a.Key
	if _, err := t.q.InsertJoinInstance(ctx, sqlcgen.InsertJoinInstanceParams{
		ID: uuidBytes(a.InstanceID), OrgID: uuidBytes(k.OrgID), ExecutionID: uuidBytes(k.ExecutionID), NodeID: k.NodeID, ScopePath: k.ScopePath,
		RoundNumber: k.RoundNumber, SplitNodeExecutionID: nullUUIDBytes(a.SplitNodeExecutionID), EdgeStateDocument: []byte("{}"),
		CreatedAt: micros(now), UpdatedAt: micros(now),
	}); err != nil {
		return store.JoinProgress{}, err
	}
	live := int64(0)
	if a.IsLive {
		live = 1
	}
	n, err := t.q.InsertJoinArrival(ctx, sqlcgen.InsertJoinArrivalParams{
		ID: uuidBytes(a.ArrivalID), OrgID: uuidBytes(k.OrgID), ExecutionID: uuidBytes(k.ExecutionID), NodeID: k.NodeID, ScopePath: k.ScopePath,
		RoundNumber: k.RoundNumber, EdgeIndex: a.EdgeIndex, TokenID: uuidBytes(a.TokenID), PayloadReference: nullText(a.PayloadReference),
		IsLive: live, ArrivedAt: micros(a.ArrivedAt), CreatedAt: micros(now), UpdatedAt: micros(now),
	})
	if err != nil {
		return store.JoinProgress{}, err
	}
	counts, err := t.q.CountJoinArrivals(ctx, sqlcgen.CountJoinArrivalsParams{
		OrgID: uuidBytes(k.OrgID), ExecutionID: uuidBytes(k.ExecutionID), NodeID: k.NodeID, ScopePath: k.ScopePath, RoundNumber: k.RoundNumber,
	})
	if err != nil {
		return store.JoinProgress{}, err
	}
	return store.JoinProgress{Recorded: n == 1, Arrivals: counts.ArrivalCount, Live: counts.LiveCount}, nil
}

func (t *sqliteTx) MarkJoinFired(ctx context.Context, key store.JoinKey, continuation store.TokenID, now time.Time) (bool, error) {
	if err := t.own(key.OrgID); err != nil {
		return false, err
	}
	n, err := t.q.MarkJoinFired(ctx, sqlcgen.MarkJoinFiredParams{
		ContinuationTokenID: uuidBytes(continuation), UpdatedAt: micros(now), OrgID: uuidBytes(key.OrgID), ExecutionID: uuidBytes(key.ExecutionID),
		NodeID: key.NodeID, ScopePath: key.ScopePath, RoundNumber: key.RoundNumber,
	})
	return n == 1, err
}

func (t *sqliteTx) InsertTimer(ctx context.Context, tm store.Timer) error {
	if err := t.own(tm.OrgID); err != nil {
		return err
	}
	return t.q.InsertTimer(ctx, sqlcgen.InsertTimerParams{
		ID: uuidBytes(tm.ID), OrgID: uuidBytes(tm.OrgID), ExecutionID: uuidBytes(tm.ExecutionID), TokenID: uuidBytes(tm.TokenID),
		Kind: tm.Kind, DueAt: micros(tm.DueAt), CreatedAt: micros(tm.CreatedAt), UpdatedAt: micros(tm.CreatedAt),
	})
}

func (t *sqliteTx) InsertWait(ctx context.Context, w store.Wait) error {
	if err := t.own(w.OrgID); err != nil {
		return err
	}
	return t.q.InsertWait(ctx, sqlcgen.InsertWaitParams{
		ID: uuidBytes(w.ID), OrgID: uuidBytes(w.OrgID), ExecutionID: uuidBytes(w.ExecutionID), TokenID: uuidBytes(w.TokenID),
		NodeExecutionID: uuidBytes(w.NodeExecutionID), Kind: w.Kind, WaitGroupKey: nullText(w.WaitGroupKey), SecretHash: w.SecretHash,
		ExpiresAt: nullMicros(w.ExpiresAt), CreatedAt: micros(w.CreatedAt), UpdatedAt: micros(w.CreatedAt),
	})
}

func (t *sqliteTx) InsertOutbox(ctx context.Context, o store.OutboxRow) error {
	if err := t.own(o.OrgID); err != nil {
		return err
	}
	return t.q.InsertOutbox(ctx, sqlcgen.InsertOutboxParams{
		ID: uuidBytes(o.ID), OrgID: uuidBytes(o.OrgID), Topic: o.Topic, PayloadDocument: o.Payload,
		CreatedAt: micros(o.CreatedAt), UpdatedAt: micros(o.CreatedAt),
	})
}

func (t *sqliteTx) MarkOutboxDelivered(ctx context.Context, outbox store.OutboxID, holder execution.WorkerID, deliveredAt time.Time) (bool, error) {
	n, err := t.q.MarkOutboxDelivered(ctx, sqlcgen.MarkOutboxDeliveredParams{
		DeliveredAt: nullMicros(deliveredAt), UpdatedAt: micros(deliveredAt), OrgID: uuidBytes(t.org), ID: uuidBytes(outbox), LeaseOwner: uuidBytes(holder),
	})
	return n == 1, err
}

func (t *sqliteTx) InsertAttentionRequest(ctx context.Context, a *store.AttentionRequest) error {
	if err := t.own(a.OrgID); err != nil {
		return err
	}
	return t.q.InsertAttentionRequest(ctx, sqlcgen.InsertAttentionRequestParams{
		ID: uuidBytes(a.ID), OrgID: uuidBytes(a.OrgID), ExecutionID: uuidBytes(a.ExecutionID), ProjectID: uuidBytes(a.ProjectID),
		FlowID: uuidBytes(a.FlowID), NodeID: a.NodeID, TokenID: nullUUIDBytes(a.TokenID), Cause: a.Cause, CauseReference: a.CauseReference,
		NotifyClass: defaultText(a.NotifyClass, "NOTIFY"), Status: defaultText(a.Status, "OPEN"), Level: a.Level,
		GroupKey: a.GroupKey, ItemCount: a.ItemCount, Summary: a.Summary, CreatedAt: micros(a.CreatedAt), UpdatedAt: micros(a.CreatedAt),
	})
}

// sqliteTx is not asserted to implement [store.Tx] with a variable declaration:
// the orgscope analyzer (rule 3) reads such assertions as store ports, and the
// methods of a transaction act on the organization that InTx scoped. InTx passes
// the value as the interface, so the compiler still checks the implementation.

// The engine plane. SQLite has one process and one writer, so these run as ordinary writer jobs.

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// ClaimTokens implements [store.WorkQueue].
func (s *Store) ClaimTokens(ctx context.Context, r store.ClaimRequest) ([]store.Claimed, error) {
	// One process owns the file, so its clock is the database clock: the instant of the
	// caller is not used, and a worker whose clock is off cannot make a lease
	// expire early or late (ADR-0013 section 4).
	now := s.clock.Now()
	keys := r.FairnessKeys
	if len(keys) == 0 {
		keys = []string{""} // the slice is never empty; AnyFairnessKey decides
	}
	var out []store.Claimed
	err := s.write(ctx, "claim tokens", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ClaimTokens(ctx, sqlcgen.ClaimTokensParams{
			WorkerID: uuidBytes(r.Worker), LeaseExpiresAt: sql.NullInt64{Int64: micros(now.Add(r.LeaseTTL)), Valid: true}, Now: micros(now),
			AnyFairnessKey: boolToInt(len(r.FairnessKeys) == 0), FairnessKeys: keys, PoisonThreshold: thresholdOf(r), PageSize: clampPage(r.Limit),
		})
		if err != nil {
			return err
		}
		out, err = s.afterClaim(ctx, q, rows, now)
		return err
	})
	return out, err
}

// ClaimLastChanceToken implements [store.WorkQueue].
func (s *Store) ClaimLastChanceToken(ctx context.Context, r store.ClaimRequest) ([]store.Claimed, error) {
	now := s.clock.Now()
	var out []store.Claimed
	err := s.write(ctx, "claim last chance token", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ClaimLastChanceToken(ctx, sqlcgen.ClaimLastChanceTokenParams{
			WorkerID: uuidBytes(r.Worker), LeaseExpiresAt: sql.NullInt64{Int64: micros(now.Add(r.LeaseTTL)), Valid: true}, Now: micros(now),
			PoisonThreshold: thresholdOf(r),
		})
		if err != nil {
			return err
		}
		claimed := make([]claimedRow, len(rows))
		for i := range rows {
			claimed[i] = claimedRow(rows[i])
		}
		out, err = s.afterClaim(ctx, q, claimed, now)
		return err
	})
	return out, err
}

// LastChanceWaiting implements [store.WorkQueue].
func (s *Store) LastChanceWaiting(ctx context.Context, poisonThreshold int64) (time.Time, bool, error) {
	var since time.Time
	var found bool
	err := s.read(ctx, "read the oldest last chance token", func(ctx context.Context, q *sqlcgen.Queries) error {
		v, err := q.GetOldestLastChance(ctx, s.clock.Now().UnixMicro(), poisonThreshold)
		if err != nil {
			return err
		}
		if m, ok := v.(int64); ok {
			since, found = time.UnixMicro(m).UTC(), true
		}
		return nil
	})
	return since, found, err
}

// thresholdOf returns the poison threshold of a claim.
func thresholdOf(r store.ClaimRequest) int64 {
	if r.PoisonThreshold <= 0 {
		return store.DefaultPoisonThreshold
	}
	return r.PoisonThreshold
}

// claimedRow is the row both claim statements return.
type claimedRow = sqlcgen.ClaimTokensRow

// afterClaim turns the rows of a claim into claims and starts the executions they belong to.
func (s *Store) afterClaim(ctx context.Context, q *sqlcgen.Queries, rows []claimedRow, now time.Time) ([]store.Claimed, error) {
	out := make([]store.Claimed, 0, len(rows))
	for i := range rows {
		var d decode
		row := &rows[i]
		out = append(out, store.Claimed{
			OrgID: uuidField[identity.OrgID](&d, row.OrgID), TokenID: uuidField[store.TokenID](&d, row.ID),
			ExecutionID: uuidField[store.ExecutionID](&d, row.ExecutionID), NodeID: row.NodeID, EdgeID: nullString(row.EdgeID),
			ScopePath: row.ScopePath, ForkLineage: row.ForkLineageDocument, PayloadReference: nullString(row.PayloadReference),
			FairnessKey: row.FairnessKey, LeaseEpoch: row.LeaseEpoch, LeaseExpiresAt: nullInstant(row.LeaseExpiresAt),
		})
		if d.err != nil {
			return nil, d.err
		}
		if err := startExecution(ctx, q, s.outboxRule(), row.OrgID, row.ExecutionID, now); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// RenewLeases implements [store.WorkQueue].
func (s *Store) RenewLeases(ctx context.Context, worker execution.WorkerID, ttl time.Duration, _ time.Time) ([]store.Renewed, error) {
	var out []store.Renewed
	now := s.clock.Now()
	err := s.write(ctx, "renew leases", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.RenewLeases(ctx, sql.NullInt64{Int64: micros(now.Add(ttl)), Valid: true}, micros(now), uuidBytes(worker))
		if err != nil {
			return err
		}
		for i := range rows {
			var d decode
			out = append(out, store.Renewed{
				OrgID: uuidField[identity.OrgID](&d, rows[i].OrgID), TokenID: uuidField[store.TokenID](&d, rows[i].ID),
				LeaseEpoch: rows[i].LeaseEpoch, CancelRequested: rows[i].Column4 == 1,
			})
			if d.err != nil {
				return d.err
			}
		}
		return nil
	})
	return out, err
}

// ClaimOutbox implements [store.Deliveries].
func (s *Store) ClaimOutbox(ctx context.Context, worker execution.WorkerID, limit int, ttl time.Duration, _ time.Time) ([]store.ClaimedOutbox, error) {
	var out []store.ClaimedOutbox
	now := s.clock.Now()
	err := s.write(ctx, "claim outbox", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ClaimOutbox(ctx, sqlcgen.ClaimOutboxParams{
			WorkerID: uuidBytes(worker), LeaseExpiresAt: sql.NullInt64{Int64: micros(now.Add(ttl)), Valid: true}, Now: micros(now), PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			var d decode
			out = append(out, store.ClaimedOutbox{
				OrgID: uuidField[identity.OrgID](&d, rows[i].OrgID), OutboxID: uuidField[store.OutboxID](&d, rows[i].ID),
				Topic: rows[i].Topic, Payload: rows[i].PayloadDocument, AttemptCount: rows[i].AttemptCount,
			})
			if d.err != nil {
				return d.err
			}
		}
		return nil
	})
	return out, err
}

// ResolveWaitSecret implements [store.WaitResolver].
func (s *Store) ResolveWaitSecret(ctx context.Context, wait store.WaitID) (store.WaitLookup, error) {
	var out store.WaitLookup
	err := s.read(ctx, "resolve wait secret", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.ResolveWaitSecret(ctx, uuidBytes(wait))
		if err != nil {
			return err
		}
		var d decode
		out = store.WaitLookup{
			OrgID: uuidField[identity.OrgID](&d, row.OrgID), WaitID: uuidField[store.WaitID](&d, row.ID),
			SecretHash: row.SecretHash, Status: row.Status, ExpiresAt: nullInstant(row.ExpiresAt),
		}
		return d.err
	})
	return out, err
}

// defaultText returns v, or fallback when v is empty.
func defaultText(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
