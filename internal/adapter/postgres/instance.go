// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// CreateOrganization implements [flow.InstanceStore]. The row-level security
// policy of org compares the row to the organization context, so the insert
// runs in the context of the organization it creates.
func (s *Store) CreateOrganization(ctx context.Context, o flow.OrgRecord) error {
	return s.inOrg(ctx, "create organization", o.ID, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertOrganization(ctx, sqlcgen.InsertOrganizationParams{
			ID: pgUUID(o.ID), Name: o.Name, Slug: o.Slug, Status: o.Status,
			DeletionScheduledAt: nullTimestamp(o.DeletionScheduledAt),
			CreatedAt:           timestamp(o.CreatedAt), UpdatedAt: timestamp(o.UpdatedAt),
		})
	})
}

// GetOrganization implements [flow.InstanceStore].
func (s *Store) GetOrganization(ctx context.Context, org identity.OrgID) (flow.OrgRecord, error) {
	var out flow.OrgRecord
	err := s.inOrg(ctx, "get organization", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetOrganization(ctx, pgUUID(org))
		if err != nil {
			return err
		}
		out = flow.OrgRecord{
			ID: fromPGUUID[identity.OrgID](row.ID), Name: row.Name, Slug: row.Slug, Status: row.Status,
			DeletionScheduledAt: instant(row.DeletionScheduledAt),
			CreatedAt:           instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
		}
		return nil
	})
	return out, err
}

// CreatePrincipal implements [flow.InstanceStore].
func (s *Store) CreatePrincipal(ctx context.Context, p flow.PrincipalRecord) error {
	return s.instance(ctx, "create principal", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertPrincipal(ctx, sqlcgen.InsertPrincipalParams{
			ID: pgUUID(p.ID), Kind: p.Kind, CreatedAt: timestamp(p.CreatedAt), UpdatedAt: timestamp(p.UpdatedAt),
		})
	})
}

// GetSystemPrincipal implements [flow.InstanceStore].
func (s *Store) GetSystemPrincipal(ctx context.Context) (flow.PrincipalRecord, error) {
	var out flow.PrincipalRecord
	err := s.instance(ctx, "get system principal", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetSystemPrincipal(ctx)
		if err != nil {
			return err
		}
		out = flow.PrincipalRecord{ID: fromPGUUID[identity.PrincipalID](row.ID), Kind: row.Kind, CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt)}
		return nil
	})
	return out, err
}

// RegisterWorker implements [execution.WorkerStore].
func (s *Store) RegisterWorker(ctx context.Context, w execution.WorkerRecord) error {
	now := s.clock.Now()
	return s.instance(ctx, "register worker", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertWorker(ctx, sqlcgen.UpsertWorkerParams{
			ID: pgUUID(w.ID), Role: w.Role, Host: w.Host, Version: w.Version,
			StartedAt: timestamp(w.StartedAt), HeartbeatAt: timestamp(w.HeartbeatAt),
			CreatedAt: timestamp(now), UpdatedAt: timestamp(now),
		})
	})
}

// CreateProject implements [flow.ProjectStore].
func (s *Store) CreateProject(ctx context.Context, p flow.ProjectRecord) error {
	return s.inOrg(ctx, "create project", p.OrgID, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertProject(ctx, sqlcgen.InsertProjectParams{
			ID: pgUUID(p.ID), OrgID: pgUUID(p.OrgID), Name: p.Name, Slug: p.Slug,
			Environment: nullText(p.Environment), IsProtected: p.IsProtected,
			CreatedAt: timestamp(p.CreatedAt), UpdatedAt: timestamp(p.UpdatedAt),
		})
	})
}

func projectFrom(row *sqlcgen.Project) flow.ProjectRecord {
	return flow.ProjectRecord{
		ID: fromPGUUID[identity.ProjectID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), Name: row.Name, Slug: row.Slug,
		Environment: row.Environment.String, IsProtected: row.IsProtected,
		CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
}

// GetProject implements [flow.ProjectStore].
func (s *Store) GetProject(ctx context.Context, org identity.OrgID, project identity.ProjectID) (flow.ProjectRecord, error) {
	var out flow.ProjectRecord
	err := s.inOrg(ctx, "get project", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetProject(ctx, pgUUID(org), pgUUID(project))
		if err != nil {
			return err
		}
		out = projectFrom(&row)
		return nil
	})
	return out, err
}

// ListProjects implements [flow.ProjectStore].
func (s *Store) ListProjects(ctx context.Context, org identity.OrgID, after identity.ProjectID, limit int) ([]flow.ProjectRecord, error) {
	var out []flow.ProjectRecord
	err := s.inOrg(ctx, "list projects", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListProjects(ctx, pgUUID(org), pgUUID(after), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, projectFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}
