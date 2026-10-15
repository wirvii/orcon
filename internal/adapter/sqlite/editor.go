// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/flow/editor"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// editorBool stores a boolean as 0 or 1 (orcon-db TY-1).
func editorBool(v bool) int64 {
	if v {
		return 1
	}
	return 0
}

// nullEditorFormat returns the format tag when a document is present and NULL otherwise.
func nullEditorFormat(document []byte, format string) sql.NullString {
	if document == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: format, Valid: true}
}

func nullMicrosOrNull(t time.Time) sql.NullInt64 {
	if t.IsZero() {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: micros(t), Valid: true}
}

func nodeTestDataFrom(row *sqlcgen.NodeTestDatum) (editor.NodeTestData, error) {
	var out editor.NodeTestData
	var err error
	if out.ID, err = uuidOf[id.UUID](row.ID); err != nil {
		return out, fmt.Errorf("node test data id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("node test data org id: %w", err)
	}
	if out.FlowID, err = uuidOf[flow.FlowID](row.FlowID); err != nil {
		return out, fmt.Errorf("node test data flow id: %w", err)
	}
	if out.BlobID, err = nullUUIDOf[id.UUID](row.BlobID); err != nil {
		return out, fmt.Errorf("node test data blob id: %w", err)
	}
	if out.CreatedByPrincipal, err = nullUUIDOf[identity.PrincipalID](row.CreatedByPrincipalID); err != nil {
		return out, fmt.Errorf("node test data principal: %w", err)
	}
	out.NodeID = row.NodeID
	out.SourceKind = row.SourceKind
	out.OutputDocument = row.OutputDocument
	out.ItemCount = row.ItemCount
	out.SizeBytes = row.SizeBytes
	out.ContentSHA256 = row.ContentSha256
	out.HasMaskedValues = row.HasMaskedValues == 1
	out.CreatedAt = instant(row.CreatedAt)
	return out, nil
}

func nodePinFrom(row *sqlcgen.FlowNodePin) (editor.NodePin, error) {
	var out editor.NodePin
	var err error
	if out.ID, err = uuidOf[id.UUID](row.ID); err != nil {
		return out, fmt.Errorf("node pin id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("node pin org id: %w", err)
	}
	if out.FlowID, err = uuidOf[flow.FlowID](row.FlowID); err != nil {
		return out, fmt.Errorf("node pin flow id: %w", err)
	}
	if out.NodeTestDataID, err = uuidOf[id.UUID](row.NodeTestDataID); err != nil {
		return out, fmt.Errorf("node pin output id: %w", err)
	}
	if out.PinnedByPrincipal, err = nullUUIDOf[identity.PrincipalID](row.PinnedByPrincipalID); err != nil {
		return out, fmt.Errorf("node pin principal: %w", err)
	}
	out.NodeID = row.NodeID
	out.Revision = row.Revision
	out.CreatedAt = instant(row.CreatedAt)
	out.UpdatedAt = instant(row.UpdatedAt)
	return out, nil
}

func testRunFrom(row *sqlcgen.TestRun) (editor.TestRun, error) {
	var out editor.TestRun
	var err error
	if out.ID, err = uuidOf[id.UUID](row.ID); err != nil {
		return out, fmt.Errorf("test run id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("test run org id: %w", err)
	}
	if out.FlowID, err = uuidOf[flow.FlowID](row.FlowID); err != nil {
		return out, fmt.Errorf("test run flow id: %w", err)
	}
	if out.ExecutionID, err = uuidOf[store.ExecutionID](row.ExecutionID); err != nil {
		return out, fmt.Errorf("test run execution id: %w", err)
	}
	if out.FlowSnapshotID, err = uuidOf[flow.FlowSnapshotID](row.FlowSnapshotID); err != nil {
		return out, fmt.Errorf("test run snapshot id: %w", err)
	}
	if out.StartedByPrincipal, err = nullUUIDOf[identity.PrincipalID](row.StartedByPrincipalID); err != nil {
		return out, fmt.Errorf("test run principal: %w", err)
	}
	out.TargetKind = row.TargetKind
	out.TargetNodeID = row.TargetNodeID.String
	out.EffectMode = row.EffectMode
	out.PlanDocument = row.PlanDocument
	out.CreatedAt = instant(row.CreatedAt)
	return out, nil
}

func testRunSeedFrom(row *sqlcgen.TestRunSeed) (editor.TestRunSeed, error) {
	var out editor.TestRunSeed
	var err error
	if out.ID, err = uuidOf[id.UUID](row.ID); err != nil {
		return out, fmt.Errorf("test run seed id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("test run seed org id: %w", err)
	}
	if out.TestRunID, err = uuidOf[id.UUID](row.TestRunID); err != nil {
		return out, fmt.Errorf("test run seed run id: %w", err)
	}
	if out.NodeTestDataID, err = uuidOf[id.UUID](row.NodeTestDataID); err != nil {
		return out, fmt.Errorf("test run seed output id: %w", err)
	}
	out.NodeID = row.NodeID
	out.SeedKind = row.SeedKind
	out.CreatedAt = instant(row.CreatedAt)
	return out, nil
}

func effectPreviewFrom(row *sqlcgen.EffectPreview) (editor.EffectPreview, error) {
	var out editor.EffectPreview
	var err error
	if out.ID, err = uuidOf[id.UUID](row.ID); err != nil {
		return out, fmt.Errorf("effect preview id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("effect preview org id: %w", err)
	}
	if out.ExecutionID, err = uuidOf[store.ExecutionID](row.ExecutionID); err != nil {
		return out, fmt.Errorf("effect preview execution id: %w", err)
	}
	if out.NodeExecutionID, err = uuidOf[store.NodeExecutionID](row.NodeExecutionID); err != nil {
		return out, fmt.Errorf("effect preview node execution id: %w", err)
	}
	if out.DecidedByPrincipal, err = nullUUIDOf[identity.PrincipalID](row.DecidedByPrincipalID); err != nil {
		return out, fmt.Errorf("effect preview principal: %w", err)
	}
	out.NodeID = row.NodeID
	out.Status = row.Status
	out.Effect = row.Effect
	out.SafetyLevel = row.SafetyLevel
	out.ItemCount = row.ItemCount
	out.IsPreviewAvailable = row.IsPreviewAvailable == 1
	out.PreviewDocument = row.PreviewDocument
	out.PreviewSHA256 = row.PreviewSha256
	out.DecisionDocument = row.DecisionDocument
	out.ExpiresAt = instant(row.ExpiresAt)
	out.DecidedAt = nullInstant(row.DecidedAt)
	out.Revision = row.Revision
	out.CreatedAt = instant(row.CreatedAt)
	out.UpdatedAt = instant(row.UpdatedAt)
	return out, nil
}

func editingSessionFrom(row *sqlcgen.FlowEditingSession) (editor.EditingSession, error) {
	var out editor.EditingSession
	var err error
	if out.ID, err = uuidOf[id.UUID](row.ID); err != nil {
		return out, fmt.Errorf("editing session id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("editing session org id: %w", err)
	}
	if out.FlowID, err = uuidOf[flow.FlowID](row.FlowID); err != nil {
		return out, fmt.Errorf("editing session flow id: %w", err)
	}
	if out.OpenedByPrincipal, err = nullUUIDOf[identity.PrincipalID](row.OpenedByPrincipalID); err != nil {
		return out, fmt.Errorf("editing session principal: %w", err)
	}
	out.ClientLabel = row.ClientLabel
	out.LastSeenDraftRevision = row.LastSeenDraftRevision
	out.OpenedAt = instant(row.OpenedAt)
	out.LastSeenAt = instant(row.LastSeenAt)
	out.ExpiresAt = instant(row.ExpiresAt)
	out.Revision = row.Revision
	out.CreatedAt = instant(row.CreatedAt)
	out.UpdatedAt = instant(row.UpdatedAt)
	return out, nil
}

// InsertNodeTestData implements [editor.Store].
func (s *Store) InsertNodeTestData(ctx context.Context, d editor.NodeTestData) error {
	return s.write(ctx, "insert node test data", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertNodeTestData(ctx, sqlcgen.InsertNodeTestDataParams{
			ID: uuidBytes(d.ID), OrgID: uuidBytes(d.OrgID), FlowID: uuidBytes(d.FlowID), NodeID: d.NodeID,
			SourceKind: d.SourceKind, OutputDocument: d.OutputDocument,
			OutputDocumentFormat: nullEditorFormat(d.OutputDocument, editor.NodeTestOutputFormat),
			BlobID:               nullUUIDBytes(d.BlobID), ItemCount: d.ItemCount, SizeBytes: d.SizeBytes,
			ContentSha256: d.ContentSHA256, HasMaskedValues: editorBool(d.HasMaskedValues),
			CreatedByPrincipalID: nullUUIDBytes(d.CreatedByPrincipal), CreatedAt: micros(d.CreatedAt),
		})
	})
}

// GetNodeTestData implements [editor.Store].
func (s *Store) GetNodeTestData(ctx context.Context, org identity.OrgID, dataID id.UUID) (editor.NodeTestData, error) {
	var out editor.NodeTestData
	err := s.read(ctx, "get node test data", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetNodeTestData(ctx, uuidBytes(org), uuidBytes(dataID))
		if err != nil {
			return err
		}
		out, err = nodeTestDataFrom(&row)
		return err
	})
	return out, err
}

// ListNodeTestDataForNode implements [editor.Store].
func (s *Store) ListNodeTestDataForNode(ctx context.Context, org identity.OrgID, flowID flow.FlowID, nodeID string, limit int) ([]editor.NodeTestData, error) {
	var out []editor.NodeTestData
	err := s.read(ctx, "list node test data", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListNodeTestDataForNode(ctx, sqlcgen.ListNodeTestDataForNodeParams{
			OrgID: uuidBytes(org), FlowID: uuidBytes(flowID), NodeID: nodeID, PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			d, err := nodeTestDataFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, d)
		}
		return nil
	})
	return out, err
}

// PinNode implements [editor.Store].
func (s *Store) PinNode(ctx context.Context, pin editor.NodePin) error {
	return s.write(ctx, "pin node", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertFlowNodePin(ctx, sqlcgen.UpsertFlowNodePinParams{
			ID: uuidBytes(pin.ID), OrgID: uuidBytes(pin.OrgID), FlowID: uuidBytes(pin.FlowID), NodeID: pin.NodeID,
			NodeTestDataID: uuidBytes(pin.NodeTestDataID), PinnedByPrincipalID: nullUUIDBytes(pin.PinnedByPrincipal),
			CreatedAt: micros(pin.CreatedAt), UpdatedAt: micros(pin.UpdatedAt),
		})
	})
}

// GetNodePin implements [editor.Store].
func (s *Store) GetNodePin(ctx context.Context, org identity.OrgID, flowID flow.FlowID, nodeID string) (editor.NodePin, error) {
	var out editor.NodePin
	err := s.read(ctx, "get node pin", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlowNodePin(ctx, uuidBytes(org), uuidBytes(flowID), nodeID)
		if err != nil {
			return err
		}
		out, err = nodePinFrom(&row)
		return err
	})
	return out, err
}

// ListNodePins implements [editor.Store].
func (s *Store) ListNodePins(ctx context.Context, org identity.OrgID, flowID flow.FlowID, afterNodeID string, limit int) ([]editor.NodePin, error) {
	var out []editor.NodePin
	err := s.read(ctx, "list node pins", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListFlowNodePins(ctx, sqlcgen.ListFlowNodePinsParams{
			OrgID: uuidBytes(org), FlowID: uuidBytes(flowID), AfterNodeID: afterNodeID, AfterID: nil, PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			p, err := nodePinFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

// UnpinNode implements [editor.Store].
func (s *Store) UnpinNode(ctx context.Context, org identity.OrgID, flowID flow.FlowID, nodeID string) error {
	return s.write(ctx, "unpin node", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.DeleteFlowNodePin(ctx, uuidBytes(org), uuidBytes(flowID), nodeID)
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
func (s *Store) InsertTestRun(ctx context.Context, run editor.TestRun) error {
	return s.write(ctx, "insert test run", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertTestRun(ctx, sqlcgen.InsertTestRunParams{
			ID: uuidBytes(run.ID), OrgID: uuidBytes(run.OrgID), FlowID: uuidBytes(run.FlowID),
			ExecutionID: uuidBytes(run.ExecutionID), FlowSnapshotID: uuidBytes(run.FlowSnapshotID),
			TargetKind: run.TargetKind, TargetNodeID: nullText(run.TargetNodeID), EffectMode: run.EffectMode,
			PlanDocument: run.PlanDocument, PlanDocumentFormat: editor.TestPlanFormat,
			StartedByPrincipalID: nullUUIDBytes(run.StartedByPrincipal), CreatedAt: micros(run.CreatedAt),
		})
	})
}

// GetTestRun implements [editor.Store].
func (s *Store) GetTestRun(ctx context.Context, org identity.OrgID, runID id.UUID) (editor.TestRun, error) {
	var out editor.TestRun
	err := s.read(ctx, "get test run", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetTestRun(ctx, uuidBytes(org), uuidBytes(runID))
		if err != nil {
			return err
		}
		out, err = testRunFrom(&row)
		return err
	})
	return out, err
}

// ListTestRuns implements [editor.Store].
func (s *Store) ListTestRuns(ctx context.Context, org identity.OrgID, flowID flow.FlowID, limit int) ([]editor.TestRun, error) {
	var out []editor.TestRun
	err := s.read(ctx, "list test runs", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListTestRuns(ctx, uuidBytes(org), uuidBytes(flowID), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			r, err := testRunFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, r)
		}
		return nil
	})
	return out, err
}

// InsertTestRunSeed implements [editor.Store].
func (s *Store) InsertTestRunSeed(ctx context.Context, seed editor.TestRunSeed) error {
	return s.write(ctx, "insert test run seed", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertTestRunSeed(ctx, sqlcgen.InsertTestRunSeedParams{
			ID: uuidBytes(seed.ID), OrgID: uuidBytes(seed.OrgID), TestRunID: uuidBytes(seed.TestRunID), NodeID: seed.NodeID,
			SeedKind: seed.SeedKind, NodeTestDataID: uuidBytes(seed.NodeTestDataID), CreatedAt: micros(seed.CreatedAt),
		})
	})
}

// ListTestRunSeeds implements [editor.Store].
func (s *Store) ListTestRunSeeds(ctx context.Context, org identity.OrgID, runID id.UUID, limit int) ([]editor.TestRunSeed, error) {
	var out []editor.TestRunSeed
	err := s.read(ctx, "list test run seeds", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListTestRunSeeds(ctx, uuidBytes(org), uuidBytes(runID), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			seed, err := testRunSeedFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, seed)
		}
		return nil
	})
	return out, err
}

// InsertEffectPreview implements [editor.Store].
func (s *Store) InsertEffectPreview(ctx context.Context, p *editor.EffectPreview) error {
	return s.write(ctx, "insert effect preview", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertEffectPreview(ctx, sqlcgen.InsertEffectPreviewParams{
			ID: uuidBytes(p.ID), OrgID: uuidBytes(p.OrgID), ExecutionID: uuidBytes(p.ExecutionID),
			NodeExecutionID: uuidBytes(p.NodeExecutionID), NodeID: p.NodeID, Status: p.Status, Effect: p.Effect,
			SafetyLevel: p.SafetyLevel, ItemCount: p.ItemCount, IsPreviewAvailable: editorBool(p.IsPreviewAvailable),
			PreviewDocument: p.PreviewDocument, PreviewDocumentFormat: editor.EffectPreviewFormat, PreviewSha256: p.PreviewSHA256,
			DecisionDocument: p.DecisionDocument, DecisionDocumentFormat: nullEditorFormat(p.DecisionDocument, editor.EffectDecisionFormat),
			ExpiresAt: micros(p.ExpiresAt), DecidedByPrincipalID: nullUUIDBytes(p.DecidedByPrincipal),
			DecidedAt: nullMicrosOrNull(p.DecidedAt), Revision: p.Revision, CreatedAt: micros(p.CreatedAt), UpdatedAt: micros(p.UpdatedAt),
		})
	})
}

// GetEffectPreview implements [editor.Store].
func (s *Store) GetEffectPreview(ctx context.Context, org identity.OrgID, previewID id.UUID) (editor.EffectPreview, error) {
	var out editor.EffectPreview
	err := s.read(ctx, "get effect preview", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetEffectPreview(ctx, uuidBytes(org), uuidBytes(previewID))
		if err != nil {
			return err
		}
		out, err = effectPreviewFrom(&row)
		return err
	})
	return out, err
}

// DecideEffectPreview implements [editor.Store]. A stale revision or a preview that is not open is a conflict.
func (s *Store) DecideEffectPreview(ctx context.Context, org identity.OrgID, previewID id.UUID, expectedRevision int64, status string, decision []byte, decidedBy identity.PrincipalID, now time.Time) error {
	return s.write(ctx, "decide effect preview", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.MarkEffectPreviewDecided(ctx, sqlcgen.MarkEffectPreviewDecidedParams{
			Status: status, DecisionDocument: decision, DecisionDocumentFormat: nullEditorFormat(decision, editor.EffectDecisionFormat),
			DecidedByPrincipalID: nullUUIDBytes(decidedBy), DecidedAt: nullMicrosOrNull(now), UpdatedAt: micros(now),
			OrgID: uuidBytes(org), ID: uuidBytes(previewID), ExpectedRevision: expectedRevision,
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
func (s *Store) ListEffectPreviews(ctx context.Context, org identity.OrgID, execID store.ExecutionID, limit int) ([]editor.EffectPreview, error) {
	var out []editor.EffectPreview
	err := s.read(ctx, "list effect previews", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListEffectPreviews(ctx, uuidBytes(org), uuidBytes(execID), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			p, err := effectPreviewFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

// UpsertEditingSession implements [editor.Store].
func (s *Store) UpsertEditingSession(ctx context.Context, e editor.EditingSession) error {
	return s.write(ctx, "upsert editing session", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertFlowEditingSession(ctx, sqlcgen.UpsertFlowEditingSessionParams{
			ID: uuidBytes(e.ID), OrgID: uuidBytes(e.OrgID), FlowID: uuidBytes(e.FlowID),
			OpenedByPrincipalID: nullUUIDBytes(e.OpenedByPrincipal), ClientLabel: e.ClientLabel,
			LastSeenDraftRevision: e.LastSeenDraftRevision, OpenedAt: micros(e.OpenedAt), LastSeenAt: micros(e.LastSeenAt),
			ExpiresAt: micros(e.ExpiresAt), CreatedAt: micros(e.CreatedAt), UpdatedAt: micros(e.UpdatedAt),
		})
	})
}

// ListEditingSessions implements [editor.Store].
func (s *Store) ListEditingSessions(ctx context.Context, org identity.OrgID, flowID flow.FlowID, now time.Time, limit int) ([]editor.EditingSession, error) {
	var out []editor.EditingSession
	err := s.read(ctx, "list editing sessions", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListFlowEditingSessions(ctx, sqlcgen.ListFlowEditingSessionsParams{
			OrgID: uuidBytes(org), FlowID: uuidBytes(flowID), Now: micros(now), PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			e, err := editingSessionFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return nil
	})
	return out, err
}

// PurgeEditorTestData implements [editor.Store]. The SQLite adapter performs the retention that the
// PostgreSQL engine runs in prune_batch: a row created before now minus 24 hours is removed when no pin
// and no test run seed refers to it.
func (s *Store) PurgeEditorTestData(ctx context.Context, now time.Time, limit int) (int64, error) {
	var removed int64
	err := s.write(ctx, "purge editor test data", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.PurgeEditorTestData(ctx, micros(now.Add(-24*time.Hour)), clampPage(limit))
		removed = n
		return err
	})
	return removed, err
}

// PurgeEditorPresence implements [editor.Store].
func (s *Store) PurgeEditorPresence(ctx context.Context, now time.Time, limit int) (int64, error) {
	var removed int64
	err := s.write(ctx, "purge editor presence", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.PurgeEditorPresence(ctx, micros(now), clampPage(limit))
		removed = n
		return err
	})
	return removed, err
}
