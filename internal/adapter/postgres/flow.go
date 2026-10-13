// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

func flowFrom(row *sqlcgen.GetFlowRow) flow.Record {
	return flow.Record{
		ID: fromPGUUID[flow.FlowID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), ProjectID: fromPGUUID[identity.ProjectID](row.ProjectID),
		Name: row.Name, Draft: row.DraftDocument, DraftRevision: row.DraftRevision,
		ActiveVersionID:      fromPGUUID[flow.FlowVersionID](row.ActiveFlowVersionID),
		DeletedAt:            instant(row.DeletedAt),
		DeletedByPrincipalID: fromPGUUID[identity.PrincipalID](row.DeletedByPrincipalID),
		CreatedAt:            instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
}

// CreateFlow implements [flow.Store].
func (s *Store) CreateFlow(ctx context.Context, f flow.Record) error {
	return s.inOrg(ctx, "create flow", f.OrgID, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertFlow(ctx, sqlcgen.InsertFlowParams{
			ID: pgUUID(f.ID), OrgID: pgUUID(f.OrgID), ProjectID: pgUUID(f.ProjectID),
			Name: f.Name, DraftDocument: f.Draft,
			CreatedAt: timestamp(f.CreatedAt), UpdatedAt: timestamp(f.UpdatedAt),
		})
	})
}

// GetFlow implements [flow.Store].
func (s *Store) GetFlow(ctx context.Context, org identity.OrgID, flowID flow.FlowID) (flow.Record, error) {
	var out flow.Record
	err := s.inOrg(ctx, "get flow", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlow(ctx, pgUUID(org), pgUUID(flowID))
		if err != nil {
			return err
		}
		out = flowFrom(&row)
		return nil
	})
	return out, err
}

// GetFlowByName implements [flow.Store].
func (s *Store) GetFlowByName(ctx context.Context, org identity.OrgID, project identity.ProjectID, name string) (flow.Record, error) {
	var out flow.Record
	err := s.inOrg(ctx, "get flow by name", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlowByName(ctx, pgUUID(org), pgUUID(project), name)
		if err != nil {
			return err
		}
		converted := sqlcgen.GetFlowRow(row)
		out = flowFrom(&converted)
		return nil
	})
	return out, err
}

// ListFlows implements [flow.Store].
func (s *Store) ListFlows(ctx context.Context, org identity.OrgID, project identity.ProjectID, after flow.FlowID, limit int) ([]flow.Record, error) {
	var out []flow.Record
	err := s.inOrg(ctx, "list flows", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListFlows(ctx, sqlcgen.ListFlowsParams{
			OrgID: pgUUID(org), ProjectID: pgUUID(project), AfterID: pgUUID(after), PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetFlowRow(rows[i])
			out = append(out, flowFrom(&converted))
		}
		return nil
	})
	return out, err
}

// UpdateFlowDraft implements [flow.Store].
func (s *Store) UpdateFlowDraft(ctx context.Context, org identity.OrgID, flowID flow.FlowID, draft []byte, expectedRevision int64, now time.Time) (int64, error) {
	var revision int64
	err := s.inOrg(ctx, "update flow draft", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpdateFlowDraft(ctx, sqlcgen.UpdateFlowDraftParams{
			DraftDocument: draft, UpdatedAt: timestamp(now), OrgID: pgUUID(org), ID: pgUUID(flowID), ExpectedRevision: expectedRevision,
		})
		if err != nil {
			return err
		}
		if n == 1 {
			revision = expectedRevision + 1
			return nil
		}
		// Either the flow does not exist or its revision moved; tell them apart.
		if _, err := q.GetFlow(ctx, pgUUID(org), pgUUID(flowID)); err != nil {
			return err
		}
		return flow.ErrDraftConflict
	})
	return revision, err
}

// PublishVersion implements [flow.VersionStore].
func (s *Store) PublishVersion(ctx context.Context, v flow.VersionRecord) error {
	return s.inOrg(ctx, "publish flow version", v.OrgID, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		params := sqlcgen.InsertFlowVersionParams{
			ID: pgUUID(v.ID), OrgID: pgUUID(v.OrgID), FlowID: pgUUID(v.FlowID),
			VersionNumber: v.Number, Document: v.Document, ContentHash: v.ContentHash,
			PublishedByPrincipalID: nullPGUUID(v.PublishedByPrincipalID),
			PublishedAt:            timestamp(v.PublishedAt), CreatedAt: timestamp(v.CreatedAt),
		}
		if v.Warnings != nil {
			params.WarningDocument = v.Warnings
			params.WarningDocumentFormat = pgtype.Text{String: "orcon/flow-warnings@1", Valid: true}
		}
		if err := q.InsertFlowVersion(ctx, params); err != nil {
			return err
		}
		n, err := q.UpdateFlowActiveVersion(ctx, sqlcgen.UpdateFlowActiveVersionParams{
			FlowVersionID: pgUUID(v.ID), UpdatedAt: timestamp(v.PublishedAt), OrgID: pgUUID(v.OrgID), ID: pgUUID(v.FlowID),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("flow %s: %w", v.FlowID, flow.ErrNotFound)
		}
		return nil
	})
}

func versionFrom(row *sqlcgen.GetFlowVersionRow) flow.VersionRecord {
	return flow.VersionRecord{
		ID: fromPGUUID[flow.FlowVersionID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), FlowID: fromPGUUID[flow.FlowID](row.FlowID),
		Number: row.VersionNumber, Document: row.Document, ContentHash: row.ContentHash,
		Warnings:               row.WarningDocument,
		PublishedByPrincipalID: fromPGUUID[identity.PrincipalID](row.PublishedByPrincipalID),
		PublishedAt:            instant(row.PublishedAt), CreatedAt: instant(row.CreatedAt),
	}
}

// GetVersion implements [flow.VersionStore].
func (s *Store) GetVersion(ctx context.Context, org identity.OrgID, version flow.FlowVersionID) (flow.VersionRecord, error) {
	var out flow.VersionRecord
	err := s.inOrg(ctx, "get flow version", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlowVersion(ctx, pgUUID(org), pgUUID(version))
		if err != nil {
			return notFoundAs(err, flow.ErrVersionNotFound)
		}
		out = versionFrom(&row)
		return nil
	})
	return out, err
}

// GetVersionByNumber implements [flow.VersionStore].
func (s *Store) GetVersionByNumber(ctx context.Context, org identity.OrgID, flowID flow.FlowID, number int64) (flow.VersionRecord, error) {
	var out flow.VersionRecord
	err := s.inOrg(ctx, "get flow version by number", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlowVersionByNumber(ctx, pgUUID(org), pgUUID(flowID), number)
		if err != nil {
			return notFoundAs(err, flow.ErrVersionNotFound)
		}
		converted := sqlcgen.GetFlowVersionRow(row)
		out = versionFrom(&converted)
		return nil
	})
	return out, err
}

// ListVersions implements [flow.VersionStore].
func (s *Store) ListVersions(ctx context.Context, org identity.OrgID, flowID flow.FlowID, after int64, limit int) ([]flow.VersionRecord, error) {
	var out []flow.VersionRecord
	err := s.inOrg(ctx, "list flow versions", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListFlowVersions(ctx, sqlcgen.ListFlowVersionsParams{
			OrgID: pgUUID(org), FlowID: pgUUID(flowID), AfterNumber: after, PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetFlowVersionRow(rows[i])
			out = append(out, versionFrom(&converted))
		}
		return nil
	})
	return out, err
}

func snapshotFrom(row *sqlcgen.GetFlowSnapshotRow) flow.SnapshotRecord {
	return flow.SnapshotRecord{
		ID: fromPGUUID[flow.FlowSnapshotID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), FlowID: fromPGUUID[flow.FlowID](row.FlowID),
		ContentHash: row.ContentHash, Document: row.Document, CreatedAt: instant(row.CreatedAt),
	}
}

// CreateSnapshot implements [flow.SnapshotStore].
func (s *Store) CreateSnapshot(ctx context.Context, snap flow.SnapshotRecord) (flow.SnapshotRecord, error) {
	var out flow.SnapshotRecord
	err := s.inOrg(ctx, "create flow snapshot", snap.OrgID, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.InsertFlowSnapshot(ctx, sqlcgen.InsertFlowSnapshotParams{
			ID: pgUUID(snap.ID), OrgID: pgUUID(snap.OrgID), FlowID: pgUUID(snap.FlowID),
			ContentHash: snap.ContentHash, Document: snap.Document, CreatedAt: timestamp(snap.CreatedAt),
		})
		if err != nil {
			return err
		}
		if n == 1 {
			out = snap
			out.CreatedAt = snap.CreatedAt.UTC().Truncate(time.Microsecond)
			return nil
		}
		row, err := q.GetFlowSnapshotByContentHash(ctx, pgUUID(snap.OrgID), snap.ContentHash)
		if err != nil {
			return err
		}
		converted := sqlcgen.GetFlowSnapshotRow(row)
		out = snapshotFrom(&converted)
		return nil
	})
	return out, err
}

// GetSnapshot implements [flow.SnapshotStore].
func (s *Store) GetSnapshot(ctx context.Context, org identity.OrgID, snapshot flow.FlowSnapshotID) (flow.SnapshotRecord, error) {
	var out flow.SnapshotRecord
	err := s.inOrg(ctx, "get flow snapshot", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetFlowSnapshot(ctx, pgUUID(org), pgUUID(snapshot))
		if err != nil {
			return notFoundAs(err, flow.ErrSnapshotNotFound)
		}
		out = snapshotFrom(&row)
		return nil
	})
	return out, err
}
