// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/id"
)

var (
	_ flow.InstanceStore = (*Store)(nil)
	_ flow.ProjectStore  = (*Store)(nil)
	_ flow.Store         = (*Store)(nil)
	_ flow.VersionStore  = (*Store)(nil)
	_ flow.SnapshotStore = (*Store)(nil)
)

// CreateOrganization implements [flow.InstanceStore].
func (s *Store) CreateOrganization(ctx context.Context, o flow.OrgRecord) error {
	return s.write(ctx, "create organization", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertOrganization(ctx, sqlcgen.InsertOrganizationParams{
			ID: uuidBytes(o.ID), Name: o.Name, Status: o.Status,
			CreatedAt: micros(o.CreatedAt), UpdatedAt: micros(o.UpdatedAt),
		})
	})
}

// GetOrganization implements [flow.InstanceStore].
func (s *Store) GetOrganization(ctx context.Context, org id.UUID) (flow.OrgRecord, error) {
	var out flow.OrgRecord
	err := s.read(ctx, "get organization", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetOrganization(ctx, uuidBytes(org))
		if err != nil {
			return err
		}
		oid, err := id.FromBytes(row.ID)
		if err != nil {
			return err
		}
		out = flow.OrgRecord{ID: oid, Name: row.Name, Status: row.Status, CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt)}
		return nil
	})
	return out, err
}

// CreatePrincipal implements [flow.InstanceStore].
func (s *Store) CreatePrincipal(ctx context.Context, p flow.PrincipalRecord) error {
	return s.write(ctx, "create principal", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertPrincipal(ctx, sqlcgen.InsertPrincipalParams{
			ID: uuidBytes(p.ID), Kind: p.Kind, CreatedAt: micros(p.CreatedAt), UpdatedAt: micros(p.UpdatedAt),
		})
	})
}

// GetSystemPrincipal implements [flow.InstanceStore].
func (s *Store) GetSystemPrincipal(ctx context.Context) (flow.PrincipalRecord, error) {
	var out flow.PrincipalRecord
	err := s.read(ctx, "get system principal", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetSystemPrincipal(ctx)
		if err != nil {
			return err
		}
		pid, err := id.FromBytes(row.ID)
		if err != nil {
			return err
		}
		out = flow.PrincipalRecord{ID: pid, Kind: row.Kind, CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt)}
		return nil
	})
	return out, err
}

// RegisterWorker implements [flow.InstanceStore].
func (s *Store) RegisterWorker(ctx context.Context, w flow.WorkerRecord) error {
	now := s.clock.Now()
	return s.write(ctx, "register worker", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertWorker(ctx, sqlcgen.UpsertWorkerParams{
			ID: uuidBytes(w.ID), Role: w.Role, Host: w.Host, Version: w.Version,
			StartedAt: micros(w.StartedAt), HeartbeatAt: micros(w.HeartbeatAt),
			CreatedAt: micros(now), UpdatedAt: micros(now),
		})
	})
}

// CreateProject implements [flow.ProjectStore].
func (s *Store) CreateProject(ctx context.Context, p flow.ProjectRecord) error {
	return s.write(ctx, "create project", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertProject(ctx, sqlcgen.InsertProjectParams{
			ID: uuidBytes(p.ID), OrgID: uuidBytes(p.OrgID), Name: p.Name, Slug: p.Slug,
			Environment: p.Environment, IsProtected: boolInt(p.IsProtected),
			CreatedAt: micros(p.CreatedAt), UpdatedAt: micros(p.UpdatedAt),
		})
	})
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func projectFrom(row sqlcgen.Project) (flow.ProjectRecord, error) {
	pid, err := id.FromBytes(row.ID)
	if err != nil {
		return flow.ProjectRecord{}, fmt.Errorf("project id: %w", err)
	}
	oid, err := id.FromBytes(row.OrgID)
	if err != nil {
		return flow.ProjectRecord{}, fmt.Errorf("project org id: %w", err)
	}
	return flow.ProjectRecord{
		ID: pid, OrgID: oid, Name: row.Name, Slug: row.Slug, Environment: row.Environment,
		IsProtected: row.IsProtected == 1, CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}, nil
}

// GetProject implements [flow.ProjectStore].
func (s *Store) GetProject(ctx context.Context, org, project id.UUID) (flow.ProjectRecord, error) {
	var out flow.ProjectRecord
	err := s.read(ctx, "get project", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetProject(ctx, uuidBytes(org), uuidBytes(project))
		if err != nil {
			return err
		}
		out, err = projectFrom(row)
		return err
	})
	return out, err
}

// ListProjects implements [flow.ProjectStore].
func (s *Store) ListProjects(ctx context.Context, org, after id.UUID, limit int) ([]flow.ProjectRecord, error) {
	var out []flow.ProjectRecord
	err := s.read(ctx, "list projects", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListProjects(ctx, uuidBytes(org), uuidBytes(after), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			row := &rows[i]
			p, err := projectFrom(*row)
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		return nil
	})
	return out, err
}

// maxPage is the largest page a store returns (orcon-db QY-6).
const maxPage = 1000

func clampPage(limit int) int64 {
	switch {
	case limit < 1:
		return 1
	case limit > maxPage:
		return maxPage
	default:
		return int64(limit)
	}
}
