// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

func flowFrom(row *sqlcgen.GetFlowRow) (flow.Record, error) {
	var out flow.Record
	var err error
	if out.ID, err = uuidOf[flow.FlowID](row.ID); err != nil {
		return out, fmt.Errorf("flow id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("flow org id: %w", err)
	}
	if out.ProjectID, err = uuidOf[identity.ProjectID](row.ProjectID); err != nil {
		return out, fmt.Errorf("flow project id: %w", err)
	}
	if out.ActiveVersionID, err = nullUUIDOf[flow.FlowVersionID](row.ActiveFlowVersionID); err != nil {
		return out, fmt.Errorf("flow active version id: %w", err)
	}
	if out.DeletedByPrincipalID, err = nullUUIDOf[identity.PrincipalID](row.DeletedByPrincipalID); err != nil {
		return out, fmt.Errorf("flow deleted by principal id: %w", err)
	}
	if out.DraftUpdatedBy, err = nullUUIDOf[identity.PrincipalID](row.DraftUpdatedByPrincipalID); err != nil {
		return out, fmt.Errorf("flow draft updated by principal id: %w", err)
	}
	out.Name = row.Name
	out.Draft = row.DraftDocument
	out.DraftRevision = row.DraftRevision
	out.Revision = row.Revision
	out.ActiveVersion = row.ActiveVersionNumber
	out.ActivationState = flow.ActivationState(row.ActivationState)
	out.DeletedAt = nullInstant(row.DeletedAt)
	out.DraftUpdatedAt = nullInstant(row.DraftUpdatedAt)
	out.CreatedAt = instant(row.CreatedAt)
	out.UpdatedAt = instant(row.UpdatedAt)
	return out, nil
}

// CreateFlow implements [flow.Store].
func (s *Store) CreateFlow(ctx context.Context, org orgscope.Org, f flow.Record) error {
	return s.write(ctx, "create flow", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scopedRecord(org, f.OrgID)
		if err != nil {
			return err
		}
		return q.InsertFlow(ctx, sqlcgen.InsertFlowParams{
			ID: uuidBytes(f.ID), OrgID: scope, ProjectID: uuidBytes(f.ProjectID),
			Name: f.Name, DraftDocument: f.Draft,
			CreatedAt: micros(f.CreatedAt), UpdatedAt: micros(f.UpdatedAt),
		})
	})
}

// CreateFlowWithEntry implements [flow.Store]: the flow, the creation entry and the
// idempotency claim of the context, when there is one, in one transaction.
func (s *Store) CreateFlowWithEntry(ctx context.Context, org orgscope.Org, f flow.Record, entry audit.Entry) error {
	return s.write(ctx, "create flow", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scopedRecord(org, f.OrgID)
		if err != nil {
			return err
		}
		if err := claimIdempotency(ctx, q, org, "flow:"+id.UUID(f.ID).String(), f.CreatedAt); err != nil {
			return err
		}
		if err := q.InsertFlow(ctx, sqlcgen.InsertFlowParams{
			ID: uuidBytes(f.ID), OrgID: scope, ProjectID: uuidBytes(f.ProjectID),
			Name: f.Name, DraftDocument: f.Draft,
			CreatedAt: micros(f.CreatedAt), UpdatedAt: micros(f.UpdatedAt),
		}); err != nil {
			return err
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
}

// GetFlow implements [flow.Store].
func (s *Store) GetFlow(ctx context.Context, org orgscope.Org, flowID flow.FlowID) (flow.Record, error) {
	var out flow.Record
	err := s.read(ctx, "get flow", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		row, err := q.GetFlow(ctx, scope, uuidBytes(flowID))
		if err != nil {
			return err
		}
		out, err = flowFrom(&row)
		return err
	})
	return out, err
}

// GetFlowByName implements [flow.Store].
func (s *Store) GetFlowByName(ctx context.Context, org orgscope.Org, project identity.ProjectID, name string) (flow.Record, error) {
	var out flow.Record
	err := s.read(ctx, "get flow by name", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		row, err := q.GetFlowByName(ctx, scope, uuidBytes(project), name)
		if err != nil {
			return err
		}
		converted := sqlcgen.GetFlowRow(row)
		out, err = flowFrom(&converted)
		return err
	})
	return out, err
}

// ListFlows implements [flow.Store].
func (s *Store) ListFlows(ctx context.Context, org orgscope.Org, project identity.ProjectID, after flow.FlowID, limit int) ([]flow.Record, error) {
	var out []flow.Record
	err := s.read(ctx, "list flows", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		rows, err := q.ListFlows(ctx, sqlcgen.ListFlowsParams{
			OrgID: scope, ProjectID: uuidBytes(project), AfterID: uuidBytes(after), PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetFlowRow(rows[i])
			f, err := flowFrom(&converted)
			if err != nil {
				return err
			}
			out = append(out, f)
		}
		return nil
	})
	return out, err
}

// ListFlowActivity implements [flow.Store] (SPEC-019 R-UX-BE-01): the flows of the page with their
// latest execution and the executions of the last day, in one query.
func (s *Store) ListFlowActivity(ctx context.Context, org orgscope.Org, project identity.ProjectID, since time.Time, after flow.FlowID, limit int) ([]flow.Activity, error) {
	var out []flow.Activity
	err := s.read(ctx, "list flow activity", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		rows, err := q.ListFlowActivity(ctx, sqlcgen.ListFlowActivityParams{
			OrgID: scope, ProjectID: uuidBytes(project), SinceAt: micros(since), AfterID: uuidBytes(after), PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			a, err := activityFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, a)
		}
		return nil
	})
	return out, err
}

// activityFrom converts a row of ListFlowActivity: the flow, and its latest execution when it ran.
func activityFrom(row *sqlcgen.ListFlowActivityRow) (flow.Activity, error) {
	var out flow.Activity
	f, err := flowFrom(&sqlcgen.GetFlowRow{
		ID: row.ID, OrgID: row.OrgID, ProjectID: row.ProjectID, Name: row.Name, DraftRevision: row.DraftRevision, Revision: row.Revision, ActiveVersionNumber: row.ActiveVersionNumber,
		ActiveFlowVersionID: row.ActiveFlowVersionID, ActivationState: row.ActivationState,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	if err != nil {
		return out, err
	}
	out.Flow = f
	out.ExecutionCountLast24h = row.ExecutionCountLast24h
	if len(row.LastExecutionID) == 0 {
		return out, nil
	}
	executionID, err := uuidOf[id.UUID](row.LastExecutionID)
	if err != nil {
		return out, fmt.Errorf("last execution id: %w", err)
	}
	out.LastExecution = &flow.LastExecution{
		ID: executionID, Status: row.LastExecutionStatus.String,
		HasUncertainOperation: row.LastExecutionHasUncertainOperation != 0,
		CreatedAt:             nullInstant(row.LastExecutionCreatedAt),
		FinishedAt:            nullInstant(row.LastExecutionFinishedAt),
	}
	return out, nil
}

// CountFlows implements [flow.Store].
func (s *Store) CountFlows(ctx context.Context, org orgscope.Org, project identity.ProjectID) (int64, error) {
	var n int64
	err := s.read(ctx, "count flows", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		n, err = q.CountFlows(ctx, scope, uuidBytes(project))
		return err
	})
	return n, err
}

// UpdateFlowDraft implements [flow.Store].
func (s *Store) UpdateFlowDraft(ctx context.Context, org orgscope.Org, flowID flow.FlowID, draft []byte, expectedRevision int64, references []flow.ConnectorReference, now time.Time) (int64, error) {
	return s.updateFlowDraft(ctx, org, flowID, draft, expectedRevision, references, identity.PrincipalID{}, now, nil)
}

// UpdateFlowDraftWithEntry implements [flow.Store]: the new draft, its connector references and
// the audit entry of the change in one transaction.
func (s *Store) UpdateFlowDraftWithEntry(ctx context.Context, org orgscope.Org, flowID flow.FlowID, draft []byte, expectedRevision int64, references []flow.ConnectorReference, now time.Time, entry audit.Entry) (int64, error) {
	return s.updateFlowDraft(ctx, org, flowID, draft, expectedRevision, references, flow.SavingPrincipal(entry.Actor), now, &entry)
}

func (s *Store) updateFlowDraft(ctx context.Context, org orgscope.Org, flowID flow.FlowID, draft []byte, expectedRevision int64, references []flow.ConnectorReference, by identity.PrincipalID, now time.Time, entry *audit.Entry) (int64, error) {
	var revision int64
	err := s.write(ctx, "update flow draft", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		n, err := q.UpdateFlowDraft(ctx, sqlcgen.UpdateFlowDraftParams{
			DraftDocument: draft, UpdatedAt: micros(now), UpdatedByPrincipalID: nullUUIDBytes(by), OrgID: scope, ID: uuidBytes(flowID), ExpectedRevision: expectedRevision,
		})
		if err != nil {
			return err
		}
		if n == 1 {
			revision = expectedRevision + 1
			if err := replaceDraftReferences(ctx, q, org, flowID, references, now); err != nil {
				return err
			}
			if entry != nil {
				return insertPendingOrg(ctx, q, org, *entry)
			}
			return nil
		}
		// Either the flow does not exist or its revision moved; tell them apart.
		if _, err := q.GetFlow(ctx, scope, uuidBytes(flowID)); err != nil {
			return err
		}
		return flow.ErrDraftConflict
	})
	return revision, err
}

// UpdateFlowNameWithEntry implements [flow.ManageStore]: the new name, the new revision of the flow
// row and the audit entry in one transaction.
func (s *Store) UpdateFlowNameWithEntry(ctx context.Context, org orgscope.Org, flowID flow.FlowID, name string, expectedRevision int64, now time.Time, entry audit.Entry) error {
	return s.write(ctx, "rename flow", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		n, err := q.UpdateFlowName(ctx, sqlcgen.UpdateFlowNameParams{
			Name: name, UpdatedAt: micros(now), OrgID: scope, ID: uuidBytes(flowID), ExpectedRevision: expectedRevision,
		})
		if err != nil {
			return err
		}
		if n == 1 {
			return insertPendingOrg(ctx, q, org, entry)
		}
		// Either the flow does not exist or its revision moved; tell them apart.
		if _, err := q.GetFlow(ctx, scope, uuidBytes(flowID)); err != nil {
			return err
		}
		return flow.ErrRevisionConflict
	})
}

// PublishVersion implements [flow.VersionStore].
func (s *Store) PublishVersion(ctx context.Context, org orgscope.Org, v flow.VersionRecord) error {
	return s.write(ctx, "publish flow version", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scopedRecord(org, v.OrgID)
		if err != nil {
			return err
		}
		params := sqlcgen.InsertFlowVersionParams{
			ID: uuidBytes(v.ID), OrgID: scope, FlowID: uuidBytes(v.FlowID),
			VersionNumber: v.Number, Document: v.Document, ContentHash: v.ContentHash,
			PublishedByPrincipalID: nullUUIDBytes(v.PublishedByPrincipalID),
			PublishedAt:            micros(v.PublishedAt), CreatedAt: micros(v.CreatedAt),
			PublicationNote: nullText(v.Note), RolledBackFromVersionNumber: nullPositive(v.RolledBackFromVersion),
		}
		if v.Warnings != nil {
			params.WarningDocument = v.Warnings
			params.WarningDocumentFormat = sql.NullString{String: "orcon/flow-warnings@1", Valid: true}
		}
		if err := q.InsertFlowVersion(ctx, params); err != nil {
			return err
		}
		n, err := q.UpdateFlowActiveVersion(ctx, sqlcgen.UpdateFlowActiveVersionParams{
			FlowVersionID: uuidBytes(v.ID), UpdatedAt: micros(v.PublishedAt), OrgID: scope, ID: uuidBytes(v.FlowID),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("flow %s: %w", v.FlowID, flow.ErrNotFound)
		}
		return insertVersionReferences(ctx, q, v)
	})
}

func versionFrom(row *sqlcgen.GetFlowVersionRow) (flow.VersionRecord, error) {
	var out flow.VersionRecord
	var err error
	if out.ID, err = uuidOf[flow.FlowVersionID](row.ID); err != nil {
		return out, fmt.Errorf("version id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("version org id: %w", err)
	}
	if out.FlowID, err = uuidOf[flow.FlowID](row.FlowID); err != nil {
		return out, fmt.Errorf("version flow id: %w", err)
	}
	if out.PublishedByPrincipalID, err = nullUUIDOf[identity.PrincipalID](row.PublishedByPrincipalID); err != nil {
		return out, fmt.Errorf("version published by principal id: %w", err)
	}
	out.Number = row.VersionNumber
	out.Document = row.Document
	out.ContentHash = row.ContentHash
	out.Warnings = row.WarningDocument
	out.PublishedAt = instant(row.PublishedAt)
	out.CreatedAt = instant(row.CreatedAt)
	out.Note = row.PublicationNote.String
	out.RolledBackFromVersion = row.RolledBackFromVersionNumber.Int64
	return out, nil
}

// nullPositive is the nullable form of a number that is absent when it is 0.
func nullPositive(n int64) sql.NullInt64 { return sql.NullInt64{Int64: n, Valid: n > 0} }

// notFoundAs maps a missing row to the given area error.
func notFoundAs(err, target error) error {
	if err != nil && isNoRows(err) {
		return fmt.Errorf("%w: %w", target, flow.ErrNotFound)
	}
	return err
}

// GetVersion implements [flow.VersionStore].
func (s *Store) GetVersion(ctx context.Context, org orgscope.Org, version flow.FlowVersionID) (flow.VersionRecord, error) {
	var out flow.VersionRecord
	err := s.read(ctx, "get flow version", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		row, err := q.GetFlowVersion(ctx, scope, uuidBytes(version))
		if err != nil {
			return notFoundAs(err, flow.ErrVersionNotFound)
		}
		out, err = versionFrom(&row)
		return err
	})
	return out, err
}

// GetVersionByNumber implements [flow.VersionStore].
func (s *Store) GetVersionByNumber(ctx context.Context, org orgscope.Org, flowID flow.FlowID, number int64) (flow.VersionRecord, error) {
	var out flow.VersionRecord
	err := s.read(ctx, "get flow version by number", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		row, err := q.GetFlowVersionByNumber(ctx, scope, uuidBytes(flowID), number)
		if err != nil {
			return notFoundAs(err, flow.ErrVersionNotFound)
		}
		converted := sqlcgen.GetFlowVersionRow(row)
		out, err = versionFrom(&converted)
		return err
	})
	return out, err
}

// ListVersions implements [flow.VersionStore].
func (s *Store) ListVersions(ctx context.Context, org orgscope.Org, flowID flow.FlowID, after int64, limit int) ([]flow.VersionRecord, error) {
	var out []flow.VersionRecord
	err := s.read(ctx, "list flow versions", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		rows, err := q.ListFlowVersions(ctx, sqlcgen.ListFlowVersionsParams{
			OrgID: scope, FlowID: uuidBytes(flowID), AfterNumber: after, PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetFlowVersionRow(rows[i])
			v, err := versionFrom(&converted)
			if err != nil {
				return err
			}
			out = append(out, v)
		}
		return nil
	})
	return out, err
}

func snapshotFrom(row *sqlcgen.GetFlowSnapshotRow) (flow.SnapshotRecord, error) {
	var out flow.SnapshotRecord
	var err error
	if out.ID, err = uuidOf[flow.FlowSnapshotID](row.ID); err != nil {
		return out, fmt.Errorf("snapshot id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("snapshot org id: %w", err)
	}
	if out.FlowID, err = uuidOf[flow.FlowID](row.FlowID); err != nil {
		return out, fmt.Errorf("snapshot flow id: %w", err)
	}
	out.ContentHash = row.ContentHash
	out.Document = row.Document
	out.CreatedAt = instant(row.CreatedAt)
	return out, nil
}

// CreateSnapshot implements [flow.SnapshotStore].
func (s *Store) CreateSnapshot(ctx context.Context, org orgscope.Org, snap flow.SnapshotRecord) (flow.SnapshotRecord, error) {
	var out flow.SnapshotRecord
	err := s.write(ctx, "create flow snapshot", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scopedRecord(org, snap.OrgID)
		if err != nil {
			return err
		}
		n, err := q.InsertFlowSnapshot(ctx, sqlcgen.InsertFlowSnapshotParams{
			ID: uuidBytes(snap.ID), OrgID: scope, FlowID: uuidBytes(snap.FlowID),
			ContentHash: snap.ContentHash, Document: snap.Document, CreatedAt: micros(snap.CreatedAt),
		})
		if err != nil {
			return err
		}
		if n == 1 {
			out = snap
			out.CreatedAt = snap.CreatedAt.UTC().Truncate(time.Microsecond)
			return nil
		}
		row, err := q.GetFlowSnapshotByContentHash(ctx, scope, snap.ContentHash)
		if err != nil {
			return err
		}
		converted := sqlcgen.GetFlowSnapshotRow(row)
		out, err = snapshotFrom(&converted)
		return err
	})
	return out, err
}

// GetSnapshot implements [flow.SnapshotStore].
func (s *Store) GetSnapshot(ctx context.Context, org orgscope.Org, snapshot flow.FlowSnapshotID) (flow.SnapshotRecord, error) {
	var out flow.SnapshotRecord
	err := s.read(ctx, "get flow snapshot", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		row, err := q.GetFlowSnapshot(ctx, scope, uuidBytes(snapshot))
		if err != nil {
			return notFoundAs(err, flow.ErrSnapshotNotFound)
		}
		out, err = snapshotFrom(&row)
		return err
	})
	return out, err
}
