// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

var (
	_ connector.PackageStore = (*Store)(nil)
	_ connector.GrantStore   = (*Store)(nil)
	_ flow.ReferenceStore    = (*Store)(nil)
)

// InsertPackage implements [connector.PackageStore].
func (s *Store) InsertPackage(ctx context.Context, p connector.PackageRecord) error {
	return s.write(ctx, "insert connector package", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertConnectorPackage(ctx, sqlcgen.InsertConnectorPackageParams{
			ID: uuidBytes(p.ID), ConnectorID: p.ConnectorID, Version: p.Version, Digest: p.Digest, Kind: string(p.Kind),
			ManifestDocument: p.Manifest, Status: string(p.Status), InstalledAt: micros(p.InstalledAt),
			InstalledByPrincipalID: nullUUIDBytes(p.InstalledByPrincipalID),
			CreatedAt:              micros(p.CreatedAt), UpdatedAt: micros(p.UpdatedAt),
		})
	})
}

func packageFrom(row *sqlcgen.GetConnectorPackageRow) (connector.PackageRecord, error) {
	var out connector.PackageRecord
	var err error
	if out.ID, err = uuidOf[connector.PackageID](row.ID); err != nil {
		return out, fmt.Errorf("package id: %w", err)
	}
	if out.InstalledByPrincipalID, err = nullUUIDOf[identity.PrincipalID](row.InstalledByPrincipalID); err != nil {
		return out, fmt.Errorf("package installed by principal id: %w", err)
	}
	out.ConnectorID, out.Version, out.Digest = row.ConnectorID, row.Version, row.Digest
	out.Kind, out.Manifest, out.Status = connector.Kind(row.Kind), row.ManifestDocument, connector.PackageStatus(row.Status)
	out.InstalledAt, out.CreatedAt, out.UpdatedAt = instant(row.InstalledAt), instant(row.CreatedAt), instant(row.UpdatedAt)
	return out, nil
}

// GetPackage implements [connector.PackageStore].
func (s *Store) GetPackage(ctx context.Context, connectorID, version string) (connector.PackageRecord, error) {
	var out connector.PackageRecord
	err := s.read(ctx, "get connector package", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetConnectorPackage(ctx, connectorID, version)
		if err != nil {
			return err
		}
		out, err = packageFrom(&row)
		return err
	})
	return out, err
}

// GetPackageByID implements [connector.PackageStore].
func (s *Store) GetPackageByID(ctx context.Context, id connector.PackageID) (connector.PackageRecord, error) {
	var out connector.PackageRecord
	err := s.read(ctx, "get connector package by id", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetConnectorPackageByID(ctx, uuidBytes(id))
		if err != nil {
			return err
		}
		converted := sqlcgen.GetConnectorPackageRow(row)
		out, err = packageFrom(&converted)
		return err
	})
	return out, err
}

// ListPackages implements [connector.PackageStore].
func (s *Store) ListPackages(ctx context.Context, after connector.PackageID, limit int) ([]connector.PackageRecord, error) {
	var out []connector.PackageRecord
	err := s.read(ctx, "list connector packages", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListConnectorPackages(ctx, uuidBytes(after), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetConnectorPackageRow(rows[i])
			rec, err := packageFrom(&converted)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		return nil
	})
	return out, err
}

// ListPackageVersions implements [connector.PackageStore].
func (s *Store) ListPackageVersions(ctx context.Context, connectorID, afterVersion string, limit int) ([]connector.PackageRecord, error) {
	var out []connector.PackageRecord
	err := s.read(ctx, "list connector package versions", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListConnectorPackageVersions(ctx, connectorID, afterVersion, clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			converted := sqlcgen.GetConnectorPackageRow(rows[i])
			rec, err := packageFrom(&converted)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		return nil
	})
	return out, err
}

// InsertGrant implements [connector.GrantStore].
func (s *Store) InsertGrant(ctx context.Context, org orgscope.Org, g connector.GrantRecord) error {
	return s.write(ctx, "insert connector grant", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scopedRecord(org, g.OrgID)
		if err != nil {
			return err
		}
		return q.InsertConnectorGrant(ctx, sqlcgen.InsertConnectorGrantParams{
			ID: uuidBytes(g.ID), OrgID: scope, ConnectorPackageID: uuidBytes(g.PackageID),
			ProjectID: nullUUIDBytes(g.ProjectID), PermissionsDigest: g.PermissionsDigest,
			GrantedByPrincipalID: nullUUIDBytes(g.GrantedByPrincipalID), GrantedAt: micros(g.GrantedAt),
			CreatedAt: micros(g.CreatedAt), UpdatedAt: micros(g.UpdatedAt),
		})
	})
}

// FindGrant implements [connector.GrantStore].
func (s *Store) FindGrant(ctx context.Context, org orgscope.Org, pkg connector.PackageID, project identity.ProjectID) (connector.GrantRecord, error) {
	var out connector.GrantRecord
	err := s.readOf(ctx, "find connector grant", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetConnectorGrant(ctx, uuidBytes(org.UUID()), uuidBytes(pkg), nullUUIDBytes(project))
		if err != nil {
			return err
		}
		if out.ID, err = uuidOf[connector.GrantID](row.ID); err != nil {
			return fmt.Errorf("grant id: %w", err)
		}
		if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
			return fmt.Errorf("grant org id: %w", err)
		}
		if out.PackageID, err = uuidOf[connector.PackageID](row.ConnectorPackageID); err != nil {
			return fmt.Errorf("grant package id: %w", err)
		}
		if out.ProjectID, err = nullUUIDOf[identity.ProjectID](row.ProjectID); err != nil {
			return fmt.Errorf("grant project id: %w", err)
		}
		if out.GrantedByPrincipalID, err = nullUUIDOf[identity.PrincipalID](row.GrantedByPrincipalID); err != nil {
			return fmt.Errorf("grant principal id: %w", err)
		}
		out.PermissionsDigest = row.PermissionsDigest
		out.GrantedAt, out.CreatedAt, out.UpdatedAt = instant(row.GrantedAt), instant(row.CreatedAt), instant(row.UpdatedAt)
		return nil
	})
	return out, err
}

// replaceDraftReferences stores the connector references of a draft, in the
// transaction of the caller, replacing the ones recorded before.
func replaceDraftReferences(ctx context.Context, q *sqlcgen.Queries, org orgscope.Org, flowID flow.FlowID, refs []flow.ConnectorReference, now time.Time) error {
	if err := q.DeleteDraftConnectorReferences(ctx, uuidBytes(org.UUID()), uuidBytes(flowID)); err != nil {
		return err
	}
	for _, ref := range refs {
		if err := q.InsertConnectorReference(ctx, sqlcgen.InsertConnectorReferenceParams{
			ID: uuidBytes(ref.ID), OrgID: uuidBytes(org.UUID()), FlowID: uuidBytes(flowID),
			ConnectorID: ref.ConnectorID, ConnectorVersion: ref.ConnectorVersion, CreatedAt: micros(now),
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
			ID: uuidBytes(ref.ID), OrgID: uuidBytes(v.OrgID), FlowID: uuidBytes(v.FlowID), FlowVersionID: uuidBytes(v.ID),
			ConnectorID: ref.ConnectorID, ConnectorVersion: ref.ConnectorVersion, CreatedAt: micros(v.CreatedAt),
		}); err != nil {
			return err
		}
	}
	return nil
}

func referenceFrom(row *sqlcgen.ConnectorReference) (flow.ConnectorReferenceRecord, error) {
	var out flow.ConnectorReferenceRecord
	var err error
	if out.ID, err = uuidOf[flow.ConnectorReferenceID](row.ID); err != nil {
		return out, fmt.Errorf("reference id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("reference org id: %w", err)
	}
	if out.FlowID, err = uuidOf[flow.FlowID](row.FlowID); err != nil {
		return out, fmt.Errorf("reference flow id: %w", err)
	}
	if out.FlowVersionID, err = nullUUIDOf[flow.FlowVersionID](row.FlowVersionID); err != nil {
		return out, fmt.Errorf("reference flow version id: %w", err)
	}
	out.ConnectorID, out.ConnectorVersion, out.CreatedAt = row.ConnectorID, row.ConnectorVersion, instant(row.CreatedAt)
	return out, nil
}

// ListDraftConnectorReferences implements [flow.ReferenceStore].
func (s *Store) ListDraftConnectorReferences(ctx context.Context, org orgscope.Org, flowID flow.FlowID, after flow.ConnectorReferenceID, limit int) ([]flow.ConnectorReferenceRecord, error) {
	var out []flow.ConnectorReferenceRecord
	err := s.readOf(ctx, "list draft connector references", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListDraftConnectorReferences(ctx, sqlcgen.ListDraftConnectorReferencesParams{
			OrgID: uuidBytes(org.UUID()), FlowID: uuidBytes(flowID), AfterID: uuidBytes(after), PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			rec, err := referenceFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		return nil
	})
	return out, err
}

// ListVersionConnectorReferences implements [flow.ReferenceStore].
func (s *Store) ListVersionConnectorReferences(ctx context.Context, org orgscope.Org, version flow.FlowVersionID, after flow.ConnectorReferenceID, limit int) ([]flow.ConnectorReferenceRecord, error) {
	var out []flow.ConnectorReferenceRecord
	err := s.readOf(ctx, "list version connector references", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListVersionConnectorReferences(ctx, sqlcgen.ListVersionConnectorReferencesParams{
			OrgID: uuidBytes(org.UUID()), FlowVersionID: uuidBytes(version), AfterID: uuidBytes(after), PageSize: clampPage(limit),
		})
		if err != nil {
			return err
		}
		for i := range rows {
			rec, err := referenceFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		return nil
	})
	return out, err
}
