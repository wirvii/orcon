// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

var (
	_ store.Reader        = (*Store)(nil)
	_ store.Inbox         = (*Store)(nil)
	_ store.Transactor    = (*Store)(nil)
	_ store.WorkQueue     = (*Store)(nil)
	_ store.LeaseRecovery = (*Store)(nil)
	_ store.Deliveries    = (*Store)(nil)
	_ store.WaitResolver  = (*Store)(nil)
)

// beforeCursor is the position of a newest-first list; the zero cursor is after everything.
func beforeCursor(c store.Cursor) (pgtype.Timestamptz, pgtype.UUID) {
	if c.IsZero() {
		var ceiling [16]byte
		for i := range ceiling {
			ceiling[i] = 0xff
		}
		return pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true}, pgtype.UUID{Bytes: ceiling, Valid: true}
	}
	return timestamp(c.CreatedAt), pgUUID(c.ID)
}

// afterBound and beforeBound are the exclusive creation-time bounds of a list; the zero time opens
// the bound.
func afterBound(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{InfinityModifier: pgtype.NegativeInfinity, Valid: true}
	}
	return timestamp(t)
}

func beforeBound(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true}
	}
	return timestamp(t)
}

// afterCursor is the position of an oldest-first list; the zero cursor is before everything.
func afterCursor(c store.Cursor) (pgtype.Timestamptz, pgtype.UUID) {
	if c.IsZero() {
		return pgtype.Timestamptz{InfinityModifier: pgtype.NegativeInfinity, Valid: true}, pgtype.UUID{Valid: true}
	}
	return timestamp(c.CreatedAt), pgUUID(c.ID)
}

func executionFrom(row *sqlcgen.GetExecutionRow) store.Execution {
	return store.Execution{
		ID: fromPGUUID[store.ExecutionID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID),
		ProjectID: fromPGUUID[identity.ProjectID](row.ProjectID), FlowID: fromPGUUID[flow.FlowID](row.FlowID),
		FlowVersionID:  fromPGUUID[flow.FlowVersionID](row.FlowVersionID),
		FlowSnapshotID: fromPGUUID[flow.FlowSnapshotID](row.FlowSnapshotID),
		Status:         row.Status, Mode: row.Mode, Priority: row.Priority,
		ParentExecutionID: fromPGUUID[store.ExecutionID](row.ParentExecutionID),
		RootExecutionID:   fromPGUUID[store.ExecutionID](row.RootExecutionID),
		CallDepth:         row.CallDepth, RunGeneration: row.RunGeneration,
		CancelRequestedAt: instant(row.CancelRequestedAt),
		RunnableCount:     row.RunnableCount, LeasedCount: row.LeasedCount, WaitingCount: row.WaitingCount, SuspendedCount: row.SuspendedCount,
		StartedAt: instant(row.StartedAt), FinishedAt: instant(row.FinishedAt),
		CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
}

// GetExecution implements [store.Reader].
func (s *Store) GetExecution(ctx context.Context, org orgscope.Org, execID store.ExecutionID) (store.Execution, error) {
	var out store.Execution
	err := s.inScope(ctx, "get execution", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetExecution(ctx, pgUUID(org.UUID()), pgUUID(execID))
		if err != nil {
			return err
		}
		out = executionFrom(&row)
		return nil
	})
	return out, err
}

// ListExecutions implements [store.Reader].
func (s *Store) ListExecutions(ctx context.Context, org orgscope.Org, filter store.ExecutionFilter, after store.Cursor, limit int) ([]store.Execution, error) {
	if !filter.Valid() {
		return nil, fmt.Errorf("list executions: the filter must name a project, or exactly one of flow and status: %w", flow.ErrInvalidRecord)
	}
	var out []store.Execution
	err := s.inScope(ctx, "list executions", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := listExecutionRows(ctx, q, org, filter, after, limit)
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, executionFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}

// listExecutionRows runs the list query the filter selects.
func listExecutionRows(ctx context.Context, q *sqlcgen.Queries, org orgscope.Org, filter store.ExecutionFilter, after store.Cursor, limit int) ([]sqlcgen.GetExecutionRow, error) {
	beforeAt, beforeID := beforeCursor(after)
	var rows []sqlcgen.GetExecutionRow
	switch {
	case filter.Oldest:
		afterAt, afterID := afterCursor(after)
		r, err := q.ListProjectExecutionsAsc(ctx, sqlcgen.ListProjectExecutionsAscParams{
			OrgID: pgUUID(org.UUID()), ProjectID: pgUUID(filter.ProjectID),
			AnyFlow: filter.FlowID.IsZero(), FlowID: pgUUID(filter.FlowID),
			AnyStatus: filter.Status == "", Status: filter.Status,
			CreatedAfter: afterBound(filter.CreatedAfter), CreatedBefore: beforeBound(filter.CreatedBefore),
			AfterCreatedAt: afterAt, AfterID: afterID, PageSize: clampPage(limit),
		})
		if err != nil {
			return nil, err
		}
		for i := range r {
			rows = append(rows, sqlcgen.GetExecutionRow(r[i]))
		}
	case filter.Refined() || !filter.ProjectID.IsZero():
		r, err := q.ListProjectExecutions(ctx, sqlcgen.ListProjectExecutionsParams{
			OrgID: pgUUID(org.UUID()), ProjectID: pgUUID(filter.ProjectID),
			AnyFlow: filter.FlowID.IsZero(), FlowID: pgUUID(filter.FlowID),
			AnyStatus: filter.Status == "", Status: filter.Status,
			CreatedAfter: afterBound(filter.CreatedAfter), CreatedBefore: beforeBound(filter.CreatedBefore),
			BeforeCreatedAt: beforeAt, BeforeID: beforeID, PageSize: clampPage(limit),
		})
		if err != nil {
			return nil, err
		}
		for i := range r {
			rows = append(rows, sqlcgen.GetExecutionRow(r[i]))
		}
	case !filter.FlowID.IsZero():
		r, err := q.ListFlowExecutions(ctx, sqlcgen.ListFlowExecutionsParams{OrgID: pgUUID(org.UUID()), FlowID: pgUUID(filter.FlowID), BeforeCreatedAt: beforeAt, BeforeID: beforeID, PageSize: clampPage(limit)})
		if err != nil {
			return nil, err
		}
		for i := range r {
			rows = append(rows, sqlcgen.GetExecutionRow(r[i]))
		}
	default:
		r, err := q.ListExecutionsByStatus(ctx, sqlcgen.ListExecutionsByStatusParams{OrgID: pgUUID(org.UUID()), Status: filter.Status, BeforeCreatedAt: beforeAt, BeforeID: beforeID, PageSize: clampPage(limit)})
		if err != nil {
			return nil, err
		}
		for i := range r {
			rows = append(rows, sqlcgen.GetExecutionRow(r[i]))
		}
	}
	return rows, nil
}

// GetToken implements [store.Reader].
func (s *Store) GetToken(ctx context.Context, org orgscope.Org, token store.TokenID) (store.Token, error) {
	var out store.Token
	err := s.inScope(ctx, "get token", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetToken(ctx, pgUUID(org.UUID()), pgUUID(token))
		if err != nil {
			return err
		}
		out = store.Token{
			ID: fromPGUUID[store.TokenID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID),
			ExecutionID: fromPGUUID[store.ExecutionID](row.ExecutionID), Status: row.Status, NodeID: row.NodeID,
			EdgeID: row.EdgeID.String, ScopePath: row.ScopePath, ForkLineage: row.ForkLineageDocument,
			PayloadReference: row.PayloadReference.String, FairnessKey: row.FairnessKey,
			EffectivePriority: row.EffectivePriority, AvailableAt: instant(row.AvailableAt),
			LeaseOwner: fromPGUUID[execution.WorkerID](row.LeaseOwner), LeaseEpoch: row.LeaseEpoch,
			LeaseExpiresAt: instant(row.LeaseExpiresAt), ClaimCount: row.ClaimCount, AbandonedCount: row.AbandonedCount,
			SuspendReason: row.SuspendReason.String,
		}
		return nil
	})
	return out, err
}

// ListExecutionEvents implements [store.Reader].
func (s *Store) ListExecutionEvents(ctx context.Context, org orgscope.Org, execID store.ExecutionID, after int64, limit int) ([]store.Event, error) {
	var out []store.Event
	err := s.inScope(ctx, "list execution events", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListExecutionEvents(ctx, sqlcgen.ListExecutionEventsParams{OrgID: pgUUID(org.UUID()), ExecutionID: pgUUID(execID), AfterSequenceNumber: after, PageSize: clampPage(limit)})
		if err != nil {
			return err
		}
		for i := range rows {
			r := &rows[i]
			out = append(out, store.Event{
				ID: fromPGUUID[store.EventID](r.ID), OrgID: fromPGUUID[identity.OrgID](r.OrgID),
				ExecutionID: fromPGUUID[store.ExecutionID](r.ExecutionID), RootExecutionID: fromPGUUID[store.ExecutionID](r.RootExecutionID),
				SequenceNumber: r.SequenceNumber, OccurredAt: instant(r.OccurredAt), Type: r.Type, SchemaVersion: r.SchemaVersion,
				Reference: r.ReferenceDocument, Data: r.DataDocument,
			})
		}
		return nil
	})
	return out, err
}

func operationFrom(row *sqlcgen.GetExternalOperationRow) store.ExternalOperation {
	return store.ExternalOperation{
		ID: fromPGUUID[store.ExternalOperationID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID),
		ExecutionID: fromPGUUID[store.ExecutionID](row.ExecutionID), NodeExecutionID: fromPGUUID[store.NodeExecutionID](row.NodeExecutionID),
		ItemIndex: row.ItemIndex, CorrelationKey: row.CorrelationKey, ContractReference: row.ContractReference, State: row.State,
		BatchKey: row.BatchKey.String, DispatchDeadlineAt: instant(row.DispatchDeadlineAt),
		FirstDispatchedAt: instant(row.FirstDispatchedAt), RetryEpoch: row.RetryEpoch, ResponseReference: row.ResponseReference.String,
	}
}

// ListNodeExecutions implements [store.Reader].
func (s *Store) ListNodeExecutions(ctx context.Context, org orgscope.Org, execID store.ExecutionID, after store.Cursor, limit int) ([]store.NodeExecution, error) {
	afterAt, afterID := afterCursor(after)
	var out []store.NodeExecution
	err := s.inScope(ctx, "list node executions", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListNodeExecutionsByExecution(ctx, sqlcgen.ListNodeExecutionsByExecutionParams{
			OrgID: pgUUID(org.UUID()), ExecutionID: pgUUID(execID), AfterCreatedAt: afterAt, AfterID: afterID, PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			r := &rows[i]
			out = append(out, store.NodeExecution{
				ID: fromPGUUID[store.NodeExecutionID](r.ID), OrgID: fromPGUUID[identity.OrgID](r.OrgID),
				ExecutionID: fromPGUUID[store.ExecutionID](r.ExecutionID), TokenID: fromPGUUID[store.TokenID](r.TokenID),
				NodeID: r.NodeID, ScopePath: r.ScopePath, Status: r.Status, AttemptCount: r.AttemptCount,
				StartedAt: instant(r.StartedAt), FinishedAt: instant(r.FinishedAt),
				CreatedAt: instant(r.CreatedAt), UpdatedAt: instant(r.UpdatedAt),
			})
		}
		return nil
	})
	return out, err
}

// ListNodeOutputs implements [store.Reader].
func (s *Store) ListNodeOutputs(ctx context.Context, org orgscope.Org, execID store.ExecutionID, after store.Cursor, limit int) ([]store.NodeOutputRecord, error) {
	afterAt, afterID := afterCursor(after)
	var out []store.NodeOutputRecord
	err := s.inScope(ctx, "list node outputs", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListNodeOutputsByExecution(ctx, sqlcgen.ListNodeOutputsByExecutionParams{
			OrgID: pgUUID(org.UUID()), ExecutionID: pgUUID(execID), AfterCreatedAt: afterAt, AfterID: afterID, PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			r := &rows[i]
			out = append(out, store.NodeOutputRecord{
				ID: fromPGUUID[store.PayloadID](r.ID), NodeExecutionID: fromPGUUID[store.NodeExecutionID](r.NodeExecutionID),
				NodeID: r.NodeID, Port: r.Port, ItemCount: r.ItemCount, Document: r.InlineDocument, CreatedAt: instant(r.CreatedAt),
			})
		}
		return nil
	})
	return out, err
}

// GetExternalOperation implements [store.Reader].
func (s *Store) GetExternalOperation(ctx context.Context, org orgscope.Org, operation store.ExternalOperationID) (store.ExternalOperation, error) {
	var out store.ExternalOperation
	err := s.inScope(ctx, "get external operation", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetExternalOperation(ctx, pgUUID(org.UUID()), pgUUID(operation))
		if err != nil {
			return err
		}
		out = operationFrom(&row)
		return nil
	})
	return out, err
}

// ListExternalOperations implements [store.Reader].
func (s *Store) ListExternalOperations(ctx context.Context, org orgscope.Org, node store.NodeExecutionID, fromItem int64, fromID store.ExternalOperationID, limit int) ([]store.ExternalOperation, error) {
	var out []store.ExternalOperation
	err := s.inScope(ctx, "list external operations", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListExternalOperations(ctx, sqlcgen.ListExternalOperationsParams{OrgID: pgUUID(org.UUID()), NodeExecutionID: pgUUID(node), FromItemIndex: fromItem, FromID: pgUUID(fromID), PageSize: clampPage(limit)})
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetExternalOperationRow(rows[i])
			out = append(out, operationFrom(&converted))
		}
		return nil
	})
	return out, err
}

// ListAttentionRequests implements [store.Inbox].
func (s *Store) ListAttentionRequests(ctx context.Context, org orgscope.Org, status string, after store.Cursor, limit int) ([]store.AttentionRequest, error) {
	afterAt, afterID := afterCursor(after)
	var out []store.AttentionRequest
	err := s.inScope(ctx, "list attention requests", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListAttentionRequests(ctx, sqlcgen.ListAttentionRequestsParams{OrgID: pgUUID(org.UUID()), Status: status, AfterCreatedAt: afterAt, AfterID: afterID, PageSize: clampPage(limit)})
		if err != nil {
			return err
		}
		for i := range rows {
			r := &rows[i]
			out = append(out, store.AttentionRequest{
				ID: fromPGUUID[store.AttentionRequestID](r.ID), OrgID: fromPGUUID[identity.OrgID](r.OrgID),
				ExecutionID: fromPGUUID[store.ExecutionID](r.ExecutionID), ProjectID: fromPGUUID[identity.ProjectID](r.ProjectID),
				FlowID: fromPGUUID[flow.FlowID](r.FlowID), NodeID: r.NodeID, TokenID: fromPGUUID[store.TokenID](r.TokenID),
				Cause: r.Cause, CauseReference: r.CauseReference, NotifyClass: r.NotifyClass, Status: r.Status, Level: r.Level,
				GroupKey: r.GroupKey, ItemCount: r.ItemCount, Summary: r.Summary, CreatedAt: instant(r.CreatedAt), UpdatedAt: instant(r.UpdatedAt),
			})
		}
		return nil
	})
	return out, err
}

// GetTimer implements [store.Inbox].
func (s *Store) GetTimer(ctx context.Context, org orgscope.Org, timer store.TimerID) (store.Timer, error) {
	var out store.Timer
	err := s.inScope(ctx, "get timer", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetTimer(ctx, pgUUID(org.UUID()), pgUUID(timer))
		if err != nil {
			return err
		}
		out = store.Timer{
			ID: fromPGUUID[store.TimerID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID),
			ExecutionID: fromPGUUID[store.ExecutionID](row.ExecutionID), TokenID: fromPGUUID[store.TokenID](row.TokenID),
			Kind: row.Kind, DueAt: instant(row.DueAt), Status: row.Status,
		}
		return nil
	})
	return out, err
}

// GetJoinInstance implements [store.Inbox].
func (s *Store) GetJoinInstance(ctx context.Context, key store.JoinKey) (store.JoinInstance, error) {
	var out store.JoinInstance
	err := s.inOrg(ctx, "get join instance", key.OrgID, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetJoinInstance(ctx, sqlcgen.GetJoinInstanceParams{
			OrgID: pgUUID(key.OrgID), ExecutionID: pgUUID(key.ExecutionID), NodeID: key.NodeID, ScopePath: key.ScopePath, RoundNumber: key.RoundNumber,
		})
		if err != nil {
			return err
		}
		out = store.JoinInstance{
			ID: fromPGUUID[store.JoinInstanceID](row.ID), Key: key, Status: row.Status,
			ContinuationTokenID: fromPGUUID[store.TokenID](row.ContinuationTokenID),
		}
		return nil
	})
	return out, err
}

// InTx implements [store.Transactor]: one READ COMMITTED transaction with the
// organization context set.
func (s *Store) InTx(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx store.Tx) error) error {
	return s.inScope(ctx, "transaction", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		return fn(ctx, &pgTx{q: q, org: identity.OrgID(org.UUID())})
	})
}

type pgTx struct {
	q   *sqlcgen.Queries
	org identity.OrgID
}

var errOtherOrganization = errors.New("the record belongs to another organization than the transaction")

func (t *pgTx) own(org identity.OrgID) error {
	if org != t.org {
		return fmt.Errorf("%w: %w", flow.ErrInvalidRecord, errOtherOrganization)
	}
	return nil
}

func (t *pgTx) AssertFence(ctx context.Context, f store.Fence, now time.Time) error {
	if err := t.own(f.OrgID); err != nil {
		return err
	}
	n, err := t.q.MarkTokenFenced(ctx, sqlcgen.MarkTokenFencedParams{UpdatedAt: timestamp(now), OrgID: pgUUID(f.OrgID), ID: pgUUID(f.Token), LeaseEpoch: f.Epoch, LeaseOwner: pgUUID(f.Owner)})
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrFenced
	}
	return nil
}

func (t *pgTx) MoveToken(ctx context.Context, f store.Fence, to string, now time.Time) error {
	if err := t.own(f.OrgID); err != nil {
		return err
	}
	if to == "LEASED" {
		return fmt.Errorf("move token to LEASED: only a claim leases a token: %w", flow.ErrInvalidRecord)
	}
	n, err := t.q.UpdateTokenFenced(ctx, sqlcgen.UpdateTokenFencedParams{Status: to, UpdatedAt: timestamp(now), OrgID: pgUUID(f.OrgID), ID: pgUUID(f.Token), LeaseEpoch: f.Epoch, LeaseOwner: pgUUID(f.Owner)})
	if err != nil {
		return err
	}
	if n == 0 {
		return store.ErrFenced
	}
	return nil
}

func (t *pgTx) InsertExecution(ctx context.Context, e *store.Execution) error {
	if err := t.own(e.OrgID); err != nil {
		return err
	}
	root := e.RootExecutionID
	if root.IsZero() {
		root = e.ID
	}
	if err := t.q.InsertExecution(ctx, sqlcgen.InsertExecutionParams{
		ID: pgUUID(e.ID), OrgID: pgUUID(e.OrgID), ProjectID: pgUUID(e.ProjectID), FlowID: pgUUID(e.FlowID),
		FlowVersionID: nullPGUUID(e.FlowVersionID), FlowSnapshotID: nullPGUUID(e.FlowSnapshotID),
		Status: e.Status, Mode: e.Mode, Priority: e.Priority, ParentExecutionID: nullPGUUID(e.ParentExecutionID),
		RootExecutionID: pgUUID(root), CallDepth: e.CallDepth, RunnableCount: e.RunnableCount,
		CreatedAt: timestamp(e.CreatedAt), UpdatedAt: timestamp(e.UpdatedAt),
	}); err != nil {
		return err
	}
	return countAcceptance(ctx, t.q, e)
}

func (t *pgTx) InsertToken(ctx context.Context, tok *store.Token) error {
	if err := t.own(tok.OrgID); err != nil {
		return err
	}
	return t.q.InsertToken(ctx, sqlcgen.InsertTokenParams{
		ID: pgUUID(tok.ID), OrgID: pgUUID(tok.OrgID), ExecutionID: pgUUID(tok.ExecutionID), Status: tok.Status,
		NodeID: tok.NodeID, EdgeID: nullText(tok.EdgeID), ScopePath: tok.ScopePath, ForkLineageDocument: tok.ForkLineage,
		PayloadReference: nullText(tok.PayloadReference), FairnessKey: tok.FairnessKey, EffectivePriority: tok.EffectivePriority,
		AvailableAt: nullTimestamp(tok.AvailableAt), QueueOrder: tok.QueueOrder, LeaseOwner: nullPGUUID(tok.LeaseOwner), LeaseEpoch: tok.LeaseEpoch,
		LeaseExpiresAt: nullTimestamp(tok.LeaseExpiresAt), LeaseMilliseconds: tok.LeaseTTL.Milliseconds(),
		CreatedAt: timestamp(tok.CreatedAt), UpdatedAt: timestamp(tok.UpdatedAt),
	})
}

func (t *pgTx) InsertNodeExecution(ctx context.Context, n store.NodeExecution) error {
	if err := t.own(n.OrgID); err != nil {
		return err
	}
	return t.q.InsertNodeExecution(ctx, sqlcgen.InsertNodeExecutionParams{
		ID: pgUUID(n.ID), OrgID: pgUUID(n.OrgID), ExecutionID: pgUUID(n.ExecutionID), TokenID: pgUUID(n.TokenID),
		NodeID: n.NodeID, ScopePath: n.ScopePath, Status: n.Status, StartedAt: nullTimestamp(n.StartedAt), FinishedAt: nullTimestamp(n.FinishedAt),
		CreatedAt: timestamp(n.CreatedAt), UpdatedAt: timestamp(n.UpdatedAt),
	})
}

func (t *pgTx) AppendEvent(ctx context.Context, e store.Event) (int64, error) {
	if err := t.own(e.OrgID); err != nil {
		return 0, err
	}
	// Hold the execution until commit: events of one execution then get their numbers in commit order.
	if _, err := t.q.LockExecution(ctx, pgUUID(e.OrgID), pgUUID(e.ExecutionID)); err != nil {
		return 0, err
	}
	root := e.RootExecutionID
	if root.IsZero() {
		root = e.ExecutionID
	}
	p := sqlcgen.AppendExecutionEventParams{
		ID: pgUUID(e.ID), OrgID: pgUUID(e.OrgID), ExecutionID: pgUUID(e.ExecutionID), RootExecutionID: pgUUID(root),
		OccurredAt: timestamp(e.OccurredAt), Type: e.Type, SchemaVersion: e.SchemaVersion, ReferenceDocument: e.Reference,
		DataDocument: e.Data, CreatedAt: timestamp(e.CreatedAt),
	}
	if e.Data != nil {
		p.DataDocumentFormat = pgtype.Text{String: "orcon/event-data@1", Valid: true}
	}
	return t.q.AppendExecutionEvent(ctx, p)
}

func (t *pgTx) AppendJournal(ctx context.Context, operations []store.ExternalOperation) error {
	for i := range operations {
		o := &operations[i]
		if err := t.own(o.OrgID); err != nil {
			return err
		}
		if err := t.q.InsertExternalOperation(ctx, sqlcgen.InsertExternalOperationParams{
			ID: pgUUID(o.ID), OrgID: pgUUID(o.OrgID), ExecutionID: pgUUID(o.ExecutionID), NodeExecutionID: pgUUID(o.NodeExecutionID),
			ItemIndex: o.ItemIndex, CorrelationKey: o.CorrelationKey, ContractReference: o.ContractReference, BatchKey: nullText(o.BatchKey),
			CreatedAt: timestamp(o.CreatedAt), UpdatedAt: timestamp(o.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (t *pgTx) UpsertNodeExecution(ctx context.Context, n store.NodeExecution) error {
	if err := t.own(n.OrgID); err != nil {
		return err
	}
	return t.q.UpsertNodeExecution(ctx, sqlcgen.UpsertNodeExecutionParams{
		ID: pgUUID(n.ID), OrgID: pgUUID(n.OrgID), ExecutionID: pgUUID(n.ExecutionID), TokenID: pgUUID(n.TokenID),
		NodeID: n.NodeID, ScopePath: n.ScopePath, Status: n.Status, StartedAt: nullTimestamp(n.StartedAt), FinishedAt: nullTimestamp(n.FinishedAt),
		CreatedAt: timestamp(n.CreatedAt), UpdatedAt: timestamp(n.UpdatedAt),
	})
}

func (t *pgTx) InsertJournalIfAbsent(ctx context.Context, operations []store.ExternalOperation) error {
	for i := range operations {
		o := &operations[i]
		if err := t.own(o.OrgID); err != nil {
			return err
		}
		if _, err := t.q.InsertExternalOperationIfAbsent(ctx, sqlcgen.InsertExternalOperationIfAbsentParams{
			ID: pgUUID(o.ID), OrgID: pgUUID(o.OrgID), ExecutionID: pgUUID(o.ExecutionID), NodeExecutionID: pgUUID(o.NodeExecutionID),
			ItemIndex: o.ItemIndex, CorrelationKey: o.CorrelationKey, ContractReference: o.ContractReference, BatchKey: nullText(o.BatchKey),
			CreatedAt: timestamp(o.CreatedAt), UpdatedAt: timestamp(o.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (t *pgTx) MarkDispatching(ctx context.Context, operations []store.ExternalOperationID, attemptTimeout time.Duration) (int64, error) {
	// The deadline is the database clock of this transaction plus the attempt timeout (ADR-0012 section 5).
	now, err := t.DatabaseNow(ctx)
	if err != nil {
		return 0, err
	}
	deadline := now.Add(attemptTimeout)
	var moved int64
	for _, op := range operations {
		n, err := t.q.MarkExternalOperationDispatching(ctx, sqlcgen.MarkExternalOperationDispatchingParams{
			DispatchDeadlineAt: nullTimestamp(deadline), DispatchedAt: timestamp(now), UpdatedAt: timestamp(now), OrgID: pgUUID(t.org), ID: pgUUID(op),
		})
		if err != nil {
			return moved, err
		}
		moved += n
	}
	return moved, nil
}

// DatabaseNow implements [store.Tx] with the PostgreSQL clock_timestamp(), so
// that every process decides recovery windows with one clock (ADR-0013 section 4).
func (t *pgTx) DatabaseNow(ctx context.Context) (time.Time, error) {
	ts, err := t.q.GetDatabaseTime(ctx)
	if err != nil {
		return time.Time{}, err
	}
	return ts.Time.UTC(), nil
}

func (t *pgTx) ResolveOperation(ctx context.Context, operation store.ExternalOperationID, from, to, responseReference string, now time.Time) (bool, error) {
	n, err := t.q.MarkExternalOperationOutcome(ctx, sqlcgen.MarkExternalOperationOutcomeParams{
		NextState: to, ResponseReference: nullText(responseReference), UpdatedAt: timestamp(now), OrgID: pgUUID(t.org), ID: pgUUID(operation), ExpectedState: from,
	})
	return n == 1, err
}

func (t *pgTx) RecordJoinArrival(ctx context.Context, a store.JoinArrival, now time.Time) (store.JoinProgress, error) {
	if err := t.own(a.Key.OrgID); err != nil {
		return store.JoinProgress{}, err
	}
	k := a.Key
	if _, err := t.q.InsertJoinInstance(ctx, sqlcgen.InsertJoinInstanceParams{
		ID: pgUUID(a.InstanceID), OrgID: pgUUID(k.OrgID), ExecutionID: pgUUID(k.ExecutionID), NodeID: k.NodeID, ScopePath: k.ScopePath,
		RoundNumber: k.RoundNumber, SplitNodeExecutionID: nullPGUUID(a.SplitNodeExecutionID), EdgeStateDocument: []byte("{}"),
		CreatedAt: timestamp(now), UpdatedAt: timestamp(now),
	}); err != nil {
		return store.JoinProgress{}, err
	}
	// Serialize with the other arrivals at this join before inserting and counting.
	if _, err := t.q.LockJoinInstance(ctx, sqlcgen.LockJoinInstanceParams{
		OrgID: pgUUID(k.OrgID), ExecutionID: pgUUID(k.ExecutionID), NodeID: k.NodeID, ScopePath: k.ScopePath, RoundNumber: k.RoundNumber,
	}); err != nil {
		return store.JoinProgress{}, err
	}
	n, err := t.q.InsertJoinArrival(ctx, sqlcgen.InsertJoinArrivalParams{
		ID: pgUUID(a.ArrivalID), OrgID: pgUUID(k.OrgID), ExecutionID: pgUUID(k.ExecutionID), NodeID: k.NodeID, ScopePath: k.ScopePath,
		RoundNumber: k.RoundNumber, EdgeIndex: a.EdgeIndex, TokenID: pgUUID(a.TokenID), PayloadReference: nullText(a.PayloadReference),
		IsLive: a.IsLive, ArrivedAt: timestamp(a.ArrivedAt), CreatedAt: timestamp(now), UpdatedAt: timestamp(now),
	})
	if err != nil {
		return store.JoinProgress{}, err
	}
	counts, err := t.q.CountJoinArrivals(ctx, sqlcgen.CountJoinArrivalsParams{
		OrgID: pgUUID(k.OrgID), ExecutionID: pgUUID(k.ExecutionID), NodeID: k.NodeID, ScopePath: k.ScopePath, RoundNumber: k.RoundNumber,
	})
	if err != nil {
		return store.JoinProgress{}, err
	}
	return store.JoinProgress{Recorded: n == 1, Arrivals: counts.ArrivalCount, Live: counts.LiveCount}, nil
}

func (t *pgTx) MarkJoinFired(ctx context.Context, key store.JoinKey, continuation store.TokenID, now time.Time) (bool, error) {
	if err := t.own(key.OrgID); err != nil {
		return false, err
	}
	n, err := t.q.MarkJoinFired(ctx, sqlcgen.MarkJoinFiredParams{
		ContinuationTokenID: pgUUID(continuation), UpdatedAt: timestamp(now), OrgID: pgUUID(key.OrgID), ExecutionID: pgUUID(key.ExecutionID),
		NodeID: key.NodeID, ScopePath: key.ScopePath, RoundNumber: key.RoundNumber,
	})
	return n == 1, err
}

func (t *pgTx) InsertTimer(ctx context.Context, tm store.Timer) error {
	if err := t.own(tm.OrgID); err != nil {
		return err
	}
	return t.q.InsertTimer(ctx, sqlcgen.InsertTimerParams{
		ID: pgUUID(tm.ID), OrgID: pgUUID(tm.OrgID), ExecutionID: pgUUID(tm.ExecutionID), TokenID: pgUUID(tm.TokenID),
		Kind: tm.Kind, DueAt: timestamp(tm.DueAt), CreatedAt: timestamp(tm.CreatedAt), UpdatedAt: timestamp(tm.CreatedAt),
	})
}

func (t *pgTx) InsertWait(ctx context.Context, w store.Wait) error {
	if err := t.own(w.OrgID); err != nil {
		return err
	}
	return t.q.InsertWait(ctx, sqlcgen.InsertWaitParams{
		ID: pgUUID(w.ID), OrgID: pgUUID(w.OrgID), ExecutionID: pgUUID(w.ExecutionID), TokenID: pgUUID(w.TokenID),
		NodeExecutionID: pgUUID(w.NodeExecutionID), Kind: w.Kind, WaitGroupKey: nullText(w.WaitGroupKey), SecretHash: w.SecretHash,
		ExpiresAt: nullTimestamp(w.ExpiresAt), CreatedAt: timestamp(w.CreatedAt), UpdatedAt: timestamp(w.CreatedAt),
	})
}

func (t *pgTx) InsertOutbox(ctx context.Context, o store.OutboxRow) error {
	if err := t.own(o.OrgID); err != nil {
		return err
	}
	return t.q.InsertOutbox(ctx, sqlcgen.InsertOutboxParams{
		ID: pgUUID(o.ID), OrgID: pgUUID(o.OrgID), Topic: o.Topic, PayloadDocument: o.Payload,
		CreatedAt: timestamp(o.CreatedAt), UpdatedAt: timestamp(o.CreatedAt),
	})
}

func (t *pgTx) MarkOutboxDelivered(ctx context.Context, outbox store.OutboxID, holder execution.WorkerID, deliveredAt time.Time) (bool, error) {
	n, err := t.q.MarkOutboxDelivered(ctx, sqlcgen.MarkOutboxDeliveredParams{
		DeliveredAt: nullTimestamp(deliveredAt), UpdatedAt: timestamp(deliveredAt), OrgID: pgUUID(t.org), ID: pgUUID(outbox), LeaseOwner: pgUUID(holder),
	})
	return n == 1, err
}

func (t *pgTx) InsertAttentionRequest(ctx context.Context, a *store.AttentionRequest) error {
	if err := t.own(a.OrgID); err != nil {
		return err
	}
	return t.q.InsertAttentionRequest(ctx, sqlcgen.InsertAttentionRequestParams{
		ID: pgUUID(a.ID), OrgID: pgUUID(a.OrgID), ExecutionID: pgUUID(a.ExecutionID), ProjectID: pgUUID(a.ProjectID),
		FlowID: pgUUID(a.FlowID), NodeID: a.NodeID, TokenID: nullPGUUID(a.TokenID), Cause: a.Cause, CauseReference: a.CauseReference,
		NotifyClass: defaultText(a.NotifyClass, "NOTIFY"), Status: defaultText(a.Status, "OPEN"), Level: a.Level,
		GroupKey: a.GroupKey, ItemCount: a.ItemCount, Summary: a.Summary, CreatedAt: timestamp(a.CreatedAt), UpdatedAt: timestamp(a.CreatedAt),
	})
}

// pgTx is not asserted to implement [store.Tx] with a variable declaration: the
// orgscope analyzer (rule 3) reads such assertions as store ports, and the
// methods of a transaction act on the organization that InTx scoped. InTx passes
// the value as the interface, so the compiler still checks the implementation.

// defaultText returns v, or fallback when v is empty.
func defaultText(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
