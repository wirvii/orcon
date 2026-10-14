// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
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
	out.Name = row.Name
	out.Draft = row.DraftDocument
	out.DraftRevision = row.DraftRevision
	out.ActivationState = flow.ActivationState(row.ActivationState)
	out.DeletedAt = nullInstant(row.DeletedAt)
	out.CreatedAt = instant(row.CreatedAt)
	out.UpdatedAt = instant(row.UpdatedAt)
	return out, nil
}

// CreateFlow implements [flow.Store].
func (s *Store) CreateFlow(ctx context.Context, f flow.Record) error {
	return s.write(ctx, "create flow", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertFlow(ctx, sqlcgen.InsertFlowParams{
			ID: uuidBytes(f.ID), OrgID: uuidBytes(f.OrgID), ProjectID: uuidBytes(f.ProjectID),
			Name: f.Name, DraftDocument: f.Draft,
			CreatedAt: micros(f.CreatedAt), UpdatedAt: micros(f.UpdatedAt),
		})
	})
}

// GetFlow implements [flow.Store].
func (s *Store) GetFlow(ctx context.Context, org identity.OrgID, flowID flow.FlowID) (flow.Record, error) {
	var out flow.Record
	err := s.read(ctx, "get flow", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlow(ctx, uuidBytes(org), uuidBytes(flowID))
		if err != nil {
			return err
		}
		out, err = flowFrom(&row)
		return err
	})
	return out, err
}

// GetFlowByName implements [flow.Store].
func (s *Store) GetFlowByName(ctx context.Context, org identity.OrgID, project identity.ProjectID, name string) (flow.Record, error) {
	var out flow.Record
	err := s.read(ctx, "get flow by name", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlowByName(ctx, uuidBytes(org), uuidBytes(project), name)
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
func (s *Store) ListFlows(ctx context.Context, org identity.OrgID, project identity.ProjectID, after flow.FlowID, limit int) ([]flow.Record, error) {
	var out []flow.Record
	err := s.read(ctx, "list flows", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListFlows(ctx, sqlcgen.ListFlowsParams{
			OrgID: uuidBytes(org), ProjectID: uuidBytes(project), AfterID: uuidBytes(after), PageSize: clampPage(limit),
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

// UpdateFlowDraft implements [flow.Store].
func (s *Store) UpdateFlowDraft(ctx context.Context, org identity.OrgID, flowID flow.FlowID, draft []byte, expectedRevision int64, references []flow.ConnectorReference, now time.Time) (int64, error) {
	var revision int64
	err := s.write(ctx, "update flow draft", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpdateFlowDraft(ctx, sqlcgen.UpdateFlowDraftParams{
			DraftDocument: draft, UpdatedAt: micros(now), OrgID: uuidBytes(org), ID: uuidBytes(flowID), ExpectedRevision: expectedRevision,
		})
		if err != nil {
			return err
		}
		if n == 1 {
			revision = expectedRevision + 1
			return replaceDraftReferences(ctx, q, org, flowID, references, now)
		}
		// Either the flow does not exist or its revision moved; tell them apart.
		if _, err := q.GetFlow(ctx, uuidBytes(org), uuidBytes(flowID)); err != nil {
			return err
		}
		return flow.ErrDraftConflict
	})
	return revision, err
}

// PublishVersion implements [flow.VersionStore].
func (s *Store) PublishVersion(ctx context.Context, v flow.VersionRecord) error {
	return s.write(ctx, "publish flow version", func(ctx context.Context, q *sqlcgen.Queries) error {
		params := sqlcgen.InsertFlowVersionParams{
			ID: uuidBytes(v.ID), OrgID: uuidBytes(v.OrgID), FlowID: uuidBytes(v.FlowID),
			VersionNumber: v.Number, Document: v.Document, ContentHash: v.ContentHash,
			PublishedByPrincipalID: nullUUIDBytes(v.PublishedByPrincipalID),
			PublishedAt:            micros(v.PublishedAt), CreatedAt: micros(v.CreatedAt),
		}
		if v.Warnings != nil {
			params.WarningDocument = v.Warnings
			params.WarningDocumentFormat = sql.NullString{String: "orcon/flow-warnings@1", Valid: true}
		}
		if err := q.InsertFlowVersion(ctx, params); err != nil {
			return err
		}
		n, err := q.UpdateFlowActiveVersion(ctx, sqlcgen.UpdateFlowActiveVersionParams{
			FlowVersionID: uuidBytes(v.ID), UpdatedAt: micros(v.PublishedAt), OrgID: uuidBytes(v.OrgID), ID: uuidBytes(v.FlowID),
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
	return out, nil
}

// notFoundAs maps a missing row to the given area error.
func notFoundAs(err, target error) error {
	if err != nil && isNoRows(err) {
		return fmt.Errorf("%w: %w", target, flow.ErrNotFound)
	}
	return err
}

// GetVersion implements [flow.VersionStore].
func (s *Store) GetVersion(ctx context.Context, org identity.OrgID, version flow.FlowVersionID) (flow.VersionRecord, error) {
	var out flow.VersionRecord
	err := s.read(ctx, "get flow version", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlowVersion(ctx, uuidBytes(org), uuidBytes(version))
		if err != nil {
			return notFoundAs(err, flow.ErrVersionNotFound)
		}
		out, err = versionFrom(&row)
		return err
	})
	return out, err
}

// GetVersionByNumber implements [flow.VersionStore].
func (s *Store) GetVersionByNumber(ctx context.Context, org identity.OrgID, flowID flow.FlowID, number int64) (flow.VersionRecord, error) {
	var out flow.VersionRecord
	err := s.read(ctx, "get flow version by number", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlowVersionByNumber(ctx, uuidBytes(org), uuidBytes(flowID), number)
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
func (s *Store) ListVersions(ctx context.Context, org identity.OrgID, flowID flow.FlowID, after int64, limit int) ([]flow.VersionRecord, error) {
	var out []flow.VersionRecord
	err := s.read(ctx, "list flow versions", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListFlowVersions(ctx, sqlcgen.ListFlowVersionsParams{
			OrgID: uuidBytes(org), FlowID: uuidBytes(flowID), AfterNumber: after, PageSize: clampPage(limit),
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
func (s *Store) CreateSnapshot(ctx context.Context, snap flow.SnapshotRecord) (flow.SnapshotRecord, error) {
	var out flow.SnapshotRecord
	err := s.write(ctx, "create flow snapshot", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.InsertFlowSnapshot(ctx, sqlcgen.InsertFlowSnapshotParams{
			ID: uuidBytes(snap.ID), OrgID: uuidBytes(snap.OrgID), FlowID: uuidBytes(snap.FlowID),
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
		row, err := q.GetFlowSnapshotByContentHash(ctx, uuidBytes(snap.OrgID), snap.ContentHash)
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
func (s *Store) GetSnapshot(ctx context.Context, org identity.OrgID, snapshot flow.FlowSnapshotID) (flow.SnapshotRecord, error) {
	var out flow.SnapshotRecord
	err := s.read(ctx, "get flow snapshot", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlowSnapshot(ctx, uuidBytes(org), uuidBytes(snapshot))
		if err != nil {
			return notFoundAs(err, flow.ErrSnapshotNotFound)
		}
		out, err = snapshotFrom(&row)
		return err
	})
	return out, err
}
