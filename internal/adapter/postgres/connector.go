// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

var (
	_ connector.PackageStore = (*Store)(nil)
	_ connector.GrantStore   = (*Store)(nil)
	_ flow.ReferenceStore    = (*Store)(nil)
)

// InsertPackage implements [connector.PackageStore].
func (s *Store) InsertPackage(ctx context.Context, p connector.PackageRecord) error {
	return s.instance(ctx, "insert connector package", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertConnectorPackage(ctx, sqlcgen.InsertConnectorPackageParams{
			ID: pgUUID(p.ID), ConnectorID: p.ConnectorID, Version: p.Version, Digest: p.Digest, Kind: string(p.Kind),
			ManifestDocument: p.Manifest, Status: string(p.Status), InstalledAt: timestamp(p.InstalledAt),
			InstalledByPrincipalID: nullPGUUID(p.InstalledByPrincipalID),
			CreatedAt:              timestamp(p.CreatedAt), UpdatedAt: timestamp(p.UpdatedAt),
		})
	})
}

func packageFrom(row *sqlcgen.GetConnectorPackageRow) connector.PackageRecord {
	return connector.PackageRecord{
		ID: fromPGUUID[connector.PackageID](row.ID), ConnectorID: row.ConnectorID, Version: row.Version, Digest: row.Digest,
		Kind: connector.Kind(row.Kind), Manifest: row.ManifestDocument, Status: connector.PackageStatus(row.Status),
		InstalledAt: instant(row.InstalledAt), InstalledByPrincipalID: fromPGUUID[identity.PrincipalID](row.InstalledByPrincipalID),
		CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
}

// GetPackage implements [connector.PackageStore].
func (s *Store) GetPackage(ctx context.Context, connectorID, version string) (connector.PackageRecord, error) {
	var out connector.PackageRecord
	err := s.instance(ctx, "get connector package", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetConnectorPackage(ctx, connectorID, version)
		if err != nil {
			return err
		}
		out = packageFrom(&row)
		return nil
	})
	return out, err
}

// GetPackageByID implements [connector.PackageStore].
func (s *Store) GetPackageByID(ctx context.Context, id connector.PackageID) (connector.PackageRecord, error) {
	var out connector.PackageRecord
	err := s.instance(ctx, "get connector package by id", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetConnectorPackageByID(ctx, pgUUID(id))
		if err != nil {
			return err
		}
		converted := sqlcgen.GetConnectorPackageRow(row)
		out = packageFrom(&converted)
		return nil
	})
	return out, err
}

// ListPackages implements [connector.PackageStore].
func (s *Store) ListPackages(ctx context.Context, after connector.PackageID, limit int) ([]connector.PackageRecord, error) {
	var out []connector.PackageRecord
	err := s.instance(ctx, "list connector packages", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListConnectorPackages(ctx, pgUUID(after), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetConnectorPackageRow(rows[i])
			out = append(out, packageFrom(&converted))
		}
		return nil
	})
	return out, err
}

// ListPackageVersions implements [connector.PackageStore].
func (s *Store) ListPackageVersions(ctx context.Context, connectorID, afterVersion string, limit int) ([]connector.PackageRecord, error) {
	var out []connector.PackageRecord
	err := s.instance(ctx, "list connector package versions", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListConnectorPackageVersions(ctx, connectorID, afterVersion, clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetConnectorPackageRow(rows[i])
			out = append(out, packageFrom(&converted))
		}
		return nil
	})
	return out, err
}

// InsertGrant implements [connector.GrantStore].
func (s *Store) InsertGrant(ctx context.Context, g connector.GrantRecord) error {
	return s.inOrg(ctx, "insert connector grant", g.OrgID, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertConnectorGrant(ctx, sqlcgen.InsertConnectorGrantParams{
			ID: pgUUID(g.ID), OrgID: pgUUID(g.OrgID), ConnectorPackageID: pgUUID(g.PackageID),
			ProjectID: nullPGUUID(g.ProjectID), PermissionsDigest: g.PermissionsDigest,
			GrantedByPrincipalID: nullPGUUID(g.GrantedByPrincipalID), GrantedAt: timestamp(g.GrantedAt),
			CreatedAt: timestamp(g.CreatedAt), UpdatedAt: timestamp(g.UpdatedAt),
		})
	})
}

// FindGrant implements [connector.GrantStore].
func (s *Store) FindGrant(ctx context.Context, org identity.OrgID, pkg connector.PackageID, project identity.ProjectID) (connector.GrantRecord, error) {
	var out connector.GrantRecord
	err := s.inOrg(ctx, "find connector grant", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetConnectorGrant(ctx, pgUUID(org), pgUUID(pkg), nullPGUUID(project))
		if err != nil {
			return err
		}
		out = connector.GrantRecord{
			ID: fromPGUUID[connector.GrantID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID),
			PackageID: fromPGUUID[connector.PackageID](row.ConnectorPackageID), ProjectID: fromPGUUID[identity.ProjectID](row.ProjectID),
			PermissionsDigest:    row.PermissionsDigest,
			GrantedByPrincipalID: fromPGUUID[identity.PrincipalID](row.GrantedByPrincipalID),
			GrantedAt:            instant(row.GrantedAt), CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
		}
		return nil
	})
	return out, err
}

// replaceDraftReferences stores the connector references of a draft, in the
// transaction of the caller, replacing the ones recorded before.
func replaceDraftReferences(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, flowID flow.FlowID, refs []flow.ConnectorReference, now time.Time) error {
	if err := q.DeleteDraftConnectorReferences(ctx, pgUUID(org), pgUUID(flowID)); err != nil {
		return err
	}
	for _, ref := range refs {
		if err := q.InsertConnectorReference(ctx, sqlcgen.InsertConnectorReferenceParams{
			ID: pgUUID(ref.ID), OrgID: pgUUID(org), FlowID: pgUUID(flowID),
			ConnectorID: ref.ConnectorID, ConnectorVersion: ref.ConnectorVersion, CreatedAt: timestamp(now),
		}); err != nil {
			return err
		}
	}
	return nil
}

// insertVersionReferences stores the connector references of a published
// version, in the transaction of the caller.
func insertVersionReferences(ctx context.Context, q *sqlcgen.Queries, v flow.VersionRecord) error {
	for _, ref := range v.ConnectorReferences {
		if err := q.InsertConnectorReference(ctx, sqlcgen.InsertConnectorReferenceParams{
			ID: pgUUID(ref.ID), OrgID: pgUUID(v.OrgID), FlowID: pgUUID(v.FlowID), FlowVersionID: pgUUID(v.ID),
			ConnectorID: ref.ConnectorID, ConnectorVersion: ref.ConnectorVersion, CreatedAt: timestamp(v.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return nil
}

func referenceFrom(row *sqlcgen.ConnectorReference) flow.ConnectorReferenceRecord {
	return flow.ConnectorReferenceRecord{
		ConnectorReference: flow.ConnectorReference{
			ID: fromPGUUID[flow.ConnectorReferenceID](row.ID), ConnectorID: row.ConnectorID, ConnectorVersion: row.ConnectorVersion,
		},
		OrgID: fromPGUUID[identity.OrgID](row.OrgID), FlowID: fromPGUUID[flow.FlowID](row.FlowID),
		FlowVersionID: fromPGUUID[flow.FlowVersionID](row.FlowVersionID), CreatedAt: instant(row.CreatedAt),
	}
}

// ListDraftConnectorReferences implements [flow.ReferenceStore].
func (s *Store) ListDraftConnectorReferences(ctx context.Context, org identity.OrgID, flowID flow.FlowID, after flow.ConnectorReferenceID, limit int) ([]flow.ConnectorReferenceRecord, error) {
	var out []flow.ConnectorReferenceRecord
	err := s.inOrg(ctx, "list draft connector references", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListDraftConnectorReferences(ctx, sqlcgen.ListDraftConnectorReferencesParams{
			OrgID: pgUUID(org), FlowID: pgUUID(flowID), AfterID: pgUUID(after), PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, referenceFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}

// ListVersionConnectorReferences implements [flow.ReferenceStore].
func (s *Store) ListVersionConnectorReferences(ctx context.Context, org identity.OrgID, version flow.FlowVersionID, after flow.ConnectorReferenceID, limit int) ([]flow.ConnectorReferenceRecord, error) {
	var out []flow.ConnectorReferenceRecord
	err := s.inOrg(ctx, "list version connector references", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListVersionConnectorReferences(ctx, sqlcgen.ListVersionConnectorReferencesParams{
			OrgID: pgUUID(org), FlowVersionID: pgUUID(version), AfterID: pgUUID(after), PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, referenceFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}
