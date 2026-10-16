// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/flow/editor"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// editorFormat returns the format tag when a document is present and NULL otherwise.
func editorFormat(document []byte, format string) pgtype.Text {
	if document == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: format, Valid: true}
}

// editorTimestamp converts an optional instant: the zero time becomes NULL.
func editorTimestamp(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return timestamp(t)
}

func nodeTestDataFrom(row *sqlcgen.NodeTestDatum) editor.NodeTestData {
	return editor.NodeTestData{
		ID: fromPGUUID[id.UUID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), FlowID: fromPGUUID[flow.FlowID](row.FlowID),
		NodeID: row.NodeID, SourceKind: row.SourceKind, OutputDocument: row.OutputDocument,
		BlobID: fromPGUUID[id.UUID](row.BlobID), ItemCount: row.ItemCount, SizeBytes: row.SizeBytes,
		ContentSHA256: row.ContentSha256, HasMaskedValues: row.HasMaskedValues,
		CreatedByPrincipal: fromPGUUID[identity.PrincipalID](row.CreatedByPrincipalID), CreatedAt: instant(row.CreatedAt),
	}
}

func nodePinFrom(row *sqlcgen.FlowNodePin) editor.NodePin {
	return editor.NodePin{
		ID: fromPGUUID[id.UUID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), FlowID: fromPGUUID[flow.FlowID](row.FlowID),
		NodeID: row.NodeID, NodeTestDataID: fromPGUUID[id.UUID](row.NodeTestDataID), Revision: row.Revision,
		PinnedByPrincipal: fromPGUUID[identity.PrincipalID](row.PinnedByPrincipalID),
		CreatedAt:         instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
}

func testRunFrom(row *sqlcgen.TestRun) editor.TestRun {
	return editor.TestRun{
		ID: fromPGUUID[id.UUID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), FlowID: fromPGUUID[flow.FlowID](row.FlowID),
		ExecutionID:    fromPGUUID[store.ExecutionID](row.ExecutionID),
		FlowSnapshotID: fromPGUUID[flow.FlowSnapshotID](row.FlowSnapshotID),
		TargetKind:     row.TargetKind, TargetNodeID: row.TargetNodeID.String, EffectMode: row.EffectMode,
		PlanDocument:       row.PlanDocument,
		StartedByPrincipal: fromPGUUID[identity.PrincipalID](row.StartedByPrincipalID),
		CreatedAt:          instant(row.CreatedAt),
	}
}

func testRunSeedFrom(row *sqlcgen.TestRunSeed) editor.TestRunSeed {
	return editor.TestRunSeed{
		ID: fromPGUUID[id.UUID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), TestRunID: fromPGUUID[id.UUID](row.TestRunID),
		NodeID: row.NodeID, SeedKind: row.SeedKind, NodeTestDataID: fromPGUUID[id.UUID](row.NodeTestDataID),
		CreatedAt: instant(row.CreatedAt),
	}
}

func effectPreviewFrom(row *sqlcgen.EffectPreview) editor.EffectPreview {
	return editor.EffectPreview{
		ID: fromPGUUID[id.UUID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID),
		ExecutionID:     fromPGUUID[store.ExecutionID](row.ExecutionID),
		NodeExecutionID: fromPGUUID[store.NodeExecutionID](row.NodeExecutionID),
		NodeID:          row.NodeID, Status: row.Status, Effect: row.Effect, SafetyLevel: row.SafetyLevel,
		ItemCount: row.ItemCount, IsPreviewAvailable: row.IsPreviewAvailable,
		PreviewDocument: row.PreviewDocument, PreviewSHA256: row.PreviewSha256, DecisionDocument: row.DecisionDocument,
		ExpiresAt:          instant(row.ExpiresAt),
		DecidedByPrincipal: fromPGUUID[identity.PrincipalID](row.DecidedByPrincipalID),
		DecidedAt:          instant(row.DecidedAt),
		Revision:           row.Revision, CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
}

func editingSessionFrom(row *sqlcgen.FlowEditingSession) editor.EditingSession {
	return editor.EditingSession{
		ID: fromPGUUID[id.UUID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), FlowID: fromPGUUID[flow.FlowID](row.FlowID),
		OpenedByPrincipal: fromPGUUID[identity.PrincipalID](row.OpenedByPrincipalID), ClientLabel: row.ClientLabel,
		LastSeenDraftRevision: row.LastSeenDraftRevision, OpenedAt: instant(row.OpenedAt), LastSeenAt: instant(row.LastSeenAt),
		ExpiresAt: instant(row.ExpiresAt), Revision: row.Revision, CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
}

// InsertNodeTestData implements [editor.Store].
func (s *Store) InsertNodeTestData(ctx context.Context, org orgscope.Org, d editor.NodeTestData) error {
	return s.inScopeRecord(ctx, "insert node test data", org, d.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertNodeTestData(ctx, sqlcgen.InsertNodeTestDataParams{
			ID: pgUUID(d.ID), OrgID: pgUUID(d.OrgID), FlowID: pgUUID(d.FlowID), NodeID: d.NodeID,
			SourceKind: d.SourceKind, OutputDocument: d.OutputDocument,
			OutputDocumentFormat: editorFormat(d.OutputDocument, editor.NodeTestOutputFormat),
			BlobID:               nullPGUUID(d.BlobID), ItemCount: d.ItemCount, SizeBytes: d.SizeBytes,
			ContentSha256: d.ContentSHA256, HasMaskedValues: d.HasMaskedValues,
			CreatedByPrincipalID: nullPGUUID(d.CreatedByPrincipal), CreatedAt: timestamp(d.CreatedAt),
		})
	})
}

// GetNodeTestData implements [editor.Store].
func (s *Store) GetNodeTestData(ctx context.Context, org orgscope.Org, dataID id.UUID) (editor.NodeTestData, error) {
	var out editor.NodeTestData
	err := s.inScope(ctx, "get node test data", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetNodeTestData(ctx, pgUUID(org.UUID()), pgUUID(dataID))
		if err != nil {
			return err
		}
		out = nodeTestDataFrom(&row)
		return nil
	})
	return out, err
}

// ListNodeTestDataForNode implements [editor.Store].
func (s *Store) ListNodeTestDataForNode(ctx context.Context, org orgscope.Org, flowID flow.FlowID, nodeID string, limit int) ([]editor.NodeTestData, error) {
	var out []editor.NodeTestData
	err := s.inScope(ctx, "list node test data", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListNodeTestDataForNode(ctx, sqlcgen.ListNodeTestDataForNodeParams{
			OrgID: pgUUID(org.UUID()), FlowID: pgUUID(flowID), NodeID: nodeID, PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, nodeTestDataFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}

// PinNode implements [editor.Store].
func (s *Store) PinNode(ctx context.Context, org orgscope.Org, pin editor.NodePin) error {
	return s.inScopeRecord(ctx, "pin node", org, pin.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertFlowNodePin(ctx, sqlcgen.UpsertFlowNodePinParams{
			ID: pgUUID(pin.ID), OrgID: pgUUID(pin.OrgID), FlowID: pgUUID(pin.FlowID), NodeID: pin.NodeID,
			NodeTestDataID: pgUUID(pin.NodeTestDataID), PinnedByPrincipalID: nullPGUUID(pin.PinnedByPrincipal),
			CreatedAt: timestamp(pin.CreatedAt), UpdatedAt: timestamp(pin.UpdatedAt),
		})
	})
}

// GetNodePin implements [editor.Store].
func (s *Store) GetNodePin(ctx context.Context, org orgscope.Org, flowID flow.FlowID, nodeID string) (editor.NodePin, error) {
	var out editor.NodePin
	err := s.inScope(ctx, "get node pin", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlowNodePin(ctx, pgUUID(org.UUID()), pgUUID(flowID), nodeID)
		if err != nil {
			return err
		}
		out = nodePinFrom(&row)
		return nil
	})
	return out, err
}

// ListNodePins implements [editor.Store].
func (s *Store) ListNodePins(ctx context.Context, org orgscope.Org, flowID flow.FlowID, afterNodeID string, limit int) ([]editor.NodePin, error) {
	var out []editor.NodePin
	err := s.inScope(ctx, "list node pins", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListFlowNodePins(ctx, sqlcgen.ListFlowNodePinsParams{
			OrgID: pgUUID(org.UUID()), FlowID: pgUUID(flowID), AfterNodeID: afterNodeID, AfterID: pgtype.UUID{}, PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, nodePinFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}

// UnpinNode implements [editor.Store].
func (s *Store) UnpinNode(ctx context.Context, org orgscope.Org, flowID flow.FlowID, nodeID string) error {
	return s.inScope(ctx, "unpin node", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.DeleteFlowNodePin(ctx, pgUUID(org.UUID()), pgUUID(flowID), nodeID)
		if err != nil {
			return err
		}
		if n == 0 {
			return flow.ErrNotFound
		}
		return nil
	})
}

// InsertTestRun implements [editor.Store].
func (s *Store) InsertTestRun(ctx context.Context, org orgscope.Org, run editor.TestRun) error {
	return s.inScopeRecord(ctx, "insert test run", org, run.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertTestRun(ctx, sqlcgen.InsertTestRunParams{
			ID: pgUUID(run.ID), OrgID: pgUUID(run.OrgID), FlowID: pgUUID(run.FlowID),
			ExecutionID: pgUUID(run.ExecutionID), FlowSnapshotID: pgUUID(run.FlowSnapshotID),
			TargetKind: run.TargetKind, TargetNodeID: nullText(run.TargetNodeID), EffectMode: run.EffectMode,
			PlanDocument: run.PlanDocument, PlanDocumentFormat: editor.TestPlanFormat,
			StartedByPrincipalID: nullPGUUID(run.StartedByPrincipal), CreatedAt: timestamp(run.CreatedAt),
		})
	})
}

// GetTestRun implements [editor.Store].
func (s *Store) GetTestRun(ctx context.Context, org orgscope.Org, runID id.UUID) (editor.TestRun, error) {
	var out editor.TestRun
	err := s.inScope(ctx, "get test run", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetTestRun(ctx, pgUUID(org.UUID()), pgUUID(runID))
		if err != nil {
			return err
		}
		out = testRunFrom(&row)
		return nil
	})
	return out, err
}

// ListTestRuns implements [editor.Store].
func (s *Store) ListTestRuns(ctx context.Context, org orgscope.Org, flowID flow.FlowID, limit int) ([]editor.TestRun, error) {
	var out []editor.TestRun
	err := s.inScope(ctx, "list test runs", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListTestRuns(ctx, pgUUID(org.UUID()), pgUUID(flowID), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, testRunFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}

// InsertTestRunSeed implements [editor.Store].
func (s *Store) InsertTestRunSeed(ctx context.Context, org orgscope.Org, seed editor.TestRunSeed) error {
	return s.inScopeRecord(ctx, "insert test run seed", org, seed.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertTestRunSeed(ctx, sqlcgen.InsertTestRunSeedParams{
			ID: pgUUID(seed.ID), OrgID: pgUUID(seed.OrgID), TestRunID: pgUUID(seed.TestRunID), NodeID: seed.NodeID,
			SeedKind: seed.SeedKind, NodeTestDataID: pgUUID(seed.NodeTestDataID), CreatedAt: timestamp(seed.CreatedAt),
		})
	})
}

// ListTestRunSeeds implements [editor.Store].
func (s *Store) ListTestRunSeeds(ctx context.Context, org orgscope.Org, runID id.UUID, limit int) ([]editor.TestRunSeed, error) {
	var out []editor.TestRunSeed
	err := s.inScope(ctx, "list test run seeds", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListTestRunSeeds(ctx, pgUUID(org.UUID()), pgUUID(runID), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, testRunSeedFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}

// InsertEffectPreview implements [editor.Store].
func (s *Store) InsertEffectPreview(ctx context.Context, org orgscope.Org, p *editor.EffectPreview) error {
	return s.inScopeRecord(ctx, "insert effect preview", org, p.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertEffectPreview(ctx, sqlcgen.InsertEffectPreviewParams{
			ID: pgUUID(p.ID), OrgID: pgUUID(p.OrgID), ExecutionID: pgUUID(p.ExecutionID),
			NodeExecutionID: pgUUID(p.NodeExecutionID), NodeID: p.NodeID, Status: p.Status, Effect: p.Effect,
			SafetyLevel: p.SafetyLevel, ItemCount: p.ItemCount, IsPreviewAvailable: p.IsPreviewAvailable,
			PreviewDocument: p.PreviewDocument, PreviewDocumentFormat: editor.EffectPreviewFormat, PreviewSha256: p.PreviewSHA256,
			DecisionDocument: p.DecisionDocument, DecisionDocumentFormat: editorFormat(p.DecisionDocument, editor.EffectDecisionFormat),
			ExpiresAt: timestamp(p.ExpiresAt), DecidedByPrincipalID: nullPGUUID(p.DecidedByPrincipal),
			DecidedAt: editorTimestamp(p.DecidedAt), Revision: p.Revision, CreatedAt: timestamp(p.CreatedAt), UpdatedAt: timestamp(p.UpdatedAt),
		})
	})
}

// GetEffectPreview implements [editor.Store].
func (s *Store) GetEffectPreview(ctx context.Context, org orgscope.Org, previewID id.UUID) (editor.EffectPreview, error) {
	var out editor.EffectPreview
	err := s.inScope(ctx, "get effect preview", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetEffectPreview(ctx, pgUUID(org.UUID()), pgUUID(previewID))
		if err != nil {
			return err
		}
		out = effectPreviewFrom(&row)
		return nil
	})
	return out, err
}

// DecideEffectPreview implements [editor.Store]. A stale revision or a preview that is not open is a conflict.
func (s *Store) DecideEffectPreview(ctx context.Context, org orgscope.Org, previewID id.UUID, expectedRevision int64, status string, decision []byte, decidedBy identity.PrincipalID, now time.Time) error {
	return s.inScope(ctx, "decide effect preview", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.MarkEffectPreviewDecided(ctx, sqlcgen.MarkEffectPreviewDecidedParams{
			Status: status, DecisionDocument: decision, DecisionDocumentFormat: editorFormat(decision, editor.EffectDecisionFormat),
			DecidedByPrincipalID: nullPGUUID(decidedBy), DecidedAt: timestamp(now), UpdatedAt: timestamp(now),
			OrgID: pgUUID(org.UUID()), ID: pgUUID(previewID), ExpectedRevision: expectedRevision,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return flow.ErrConflict
		}
		return nil
	})
}

// ListEffectPreviews implements [editor.Store].
func (s *Store) ListEffectPreviews(ctx context.Context, org orgscope.Org, execID store.ExecutionID, limit int) ([]editor.EffectPreview, error) {
	var out []editor.EffectPreview
	err := s.inScope(ctx, "list effect previews", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListEffectPreviews(ctx, pgUUID(org.UUID()), pgUUID(execID), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, effectPreviewFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}

// UpsertEditingSession implements [editor.Store].
func (s *Store) UpsertEditingSession(ctx context.Context, org orgscope.Org, e editor.EditingSession) error {
	return s.inScopeRecord(ctx, "upsert editing session", org, e.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertFlowEditingSession(ctx, sqlcgen.UpsertFlowEditingSessionParams{
			ID: pgUUID(e.ID), OrgID: pgUUID(e.OrgID), FlowID: pgUUID(e.FlowID),
			OpenedByPrincipalID: nullPGUUID(e.OpenedByPrincipal), ClientLabel: e.ClientLabel,
			LastSeenDraftRevision: e.LastSeenDraftRevision, OpenedAt: timestamp(e.OpenedAt), LastSeenAt: timestamp(e.LastSeenAt),
			ExpiresAt: timestamp(e.ExpiresAt), CreatedAt: timestamp(e.CreatedAt), UpdatedAt: timestamp(e.UpdatedAt),
		})
	})
}

// ListEditingSessions implements [editor.Store].
func (s *Store) ListEditingSessions(ctx context.Context, org orgscope.Org, flowID flow.FlowID, now time.Time, limit int) ([]editor.EditingSession, error) {
	var out []editor.EditingSession
	err := s.inScope(ctx, "list editing sessions", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListFlowEditingSessions(ctx, sqlcgen.ListFlowEditingSessionsParams{
			OrgID: pgUUID(org.UUID()), FlowID: pgUUID(flowID), Now: timestamp(now), PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, editingSessionFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}

// PurgeEditorTestData implements [editor.Store] through the orcon_engine function prune_batch.
func (s *Store) PurgeEditorTestData(ctx context.Context, now time.Time, limit int) (int64, error) {
	var removed int64
	err := s.instance(ctx, "purge editor test data", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.PurgeEditorTestData(ctx, clampPage(limit), timestamp(now))
		removed = n
		return err
	})
	return removed, err
}

// PurgeEditorPresence implements [editor.Store] through the orcon_engine function prune_batch.
func (s *Store) PurgeEditorPresence(ctx context.Context, now time.Time, limit int) (int64, error) {
	var removed int64
	err := s.instance(ctx, "purge editor presence", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.PurgeEditorPresence(ctx, clampPage(limit), timestamp(now))
		removed = n
		return err
	})
	return removed, err
}
