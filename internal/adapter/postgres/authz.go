// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// authzRowLimit bounds the bindings and custom role permissions one read returns.
const authzRowLimit = 10000

// Epoch implements [authz.Store].
func (s *Store) Epoch(ctx context.Context, org orgscope.Org) (int64, error) {
	var out int64
	err := s.inOrg(ctx, "get authorization epoch", identity.OrgIDOf(org), true, func(ctx context.Context, q *sqlcgen.Queries) error {
		var err error
		out, err = q.GetAuthorizationEpoch(ctx, pgUUID(identity.OrgIDOf(org)))
		return err
	})
	return out, err
}

// ActiveMember implements [authz.Store].
func (s *Store) ActiveMember(ctx context.Context, user identity.UserID, org orgscope.Org) (bool, error) {
	var active bool
	err := s.inOrg(ctx, "get membership", identity.OrgIDOf(org), true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetMembershipOfApplicationUser(ctx, pgUUID(identity.OrgIDOf(org)), pgUUID(user))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		active = row.Status == identity.MembershipActive
		return nil
	})
	return active, err
}

// Bindings implements [authz.Store].
func (s *Store) Bindings(ctx context.Context, user identity.UserID, org orgscope.Org) ([]authz.Binding, error) {
	out := []authz.Binding{}
	err := s.inOrg(ctx, "list role bindings", identity.OrgIDOf(org), true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListRoleBindingsOfUser(ctx, pgUUID(identity.OrgIDOf(org)), pgUUID(user), clampPage(authzRowLimit))
		if err != nil {
			return err
		}
		for _, row := range rows {
			b := authz.Binding{Role: row.RoleName}
			if row.ScopeKind == authz.ScopeKindProject {
				if !row.ProjectID.Valid {
					return fmt.Errorf("binding project: %w", identity.ErrInvalidRecord)
				}
				b.Project = id.UUID(row.ProjectID.Bytes).String()
			}
			out = append(out, b)
		}
		return nil
	})
	return out, err
}

// CustomRoles implements [authz.Store].
func (s *Store) CustomRoles(ctx context.Context, org orgscope.Org) (map[string][]authz.Permission, error) {
	out := map[string][]authz.Permission{}
	err := s.inOrg(ctx, "list custom role permissions", identity.OrgIDOf(org), true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListCustomRolePermissions(ctx, pgUUID(identity.OrgIDOf(org)), clampPage(authzRowLimit))
		if err != nil {
			return err
		}
		for _, row := range rows {
			role := id.UUID(row.ID.Bytes).String()
			out[role] = append(out[role], authz.Permission(row.Permission))
		}
		return nil
	})
	return out, err
}

// ProjectOrganization implements [authz.Store]: the organization of a project, across
// organizations, through the engine function orcon_engine.project_organization.
func (s *Store) ProjectOrganization(ctx context.Context, project string) (identity.OrgID, error) {
	pid, err := id.Parse(project)
	if err != nil {
		return identity.OrgID{}, fmt.Errorf("%w: project identifier", identity.ErrNotFound)
	}
	var owner pgtype.UUID
	if err := s.pool.QueryRow(ctx, "SELECT orcon_engine.project_organization($1)", pgtype.UUID{Bytes: pid, Valid: true}).Scan(&owner); err != nil {
		return identity.OrgID{}, mapError("get project organization", err)
	}
	if !owner.Valid {
		return identity.OrgID{}, identity.ErrNotFound
	}
	return identity.OrgID(owner.Bytes), nil
}

// ResourceProject implements [authz.Store]: the organization and the project of a flow, execution,
// external operation or human task, across organizations, through orcon_engine.resource_project.
func (s *Store) ResourceProject(ctx context.Context, kind, resource string) (identity.OrgID, string, error) {
	rid, err := id.Parse(resource)
	if err != nil {
		return identity.OrgID{}, "", fmt.Errorf("%w: %s identifier", identity.ErrNotFound, kind)
	}
	var org, project pgtype.UUID
	err = s.pool.QueryRow(ctx, "SELECT org_id, project_id FROM orcon_engine.resource_project($1, $2)",
		kind, pgtype.UUID{Bytes: rid, Valid: true}).Scan(&org, &project)
	if errors.Is(err, pgx.ErrNoRows) {
		return identity.OrgID{}, "", identity.ErrNotFound
	}
	if err != nil {
		return identity.OrgID{}, "", mapError("get resource project", err)
	}
	return identity.OrgID(org.Bytes), id.UUID(project.Bytes).String(), nil
}
