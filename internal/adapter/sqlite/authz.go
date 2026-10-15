// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
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
	err := s.read(ctx, "get authorization epoch", func(ctx context.Context, q *sqlcgen.Queries) error {
		var err error
		out, err = q.GetAuthorizationEpoch(ctx, uuidBytes(identity.OrgIDOf(org)))
		return err
	})
	return out, err
}

// ActiveMember implements [authz.Store].
func (s *Store) ActiveMember(ctx context.Context, user identity.UserID, org orgscope.Org) (bool, error) {
	var active bool
	err := s.readOf(ctx, "get membership", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetMembershipOfApplicationUser(ctx, uuidBytes(identity.OrgIDOf(org)), uuidBytes(user))
		if isNoRows(err) {
			return nil
		}
		if err != nil {
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
	err := s.readOf(ctx, "list role bindings", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListRoleBindingsOfUser(ctx, uuidBytes(identity.OrgIDOf(org)), uuidBytes(user), authzRowLimit)
		if err != nil {
			return err
		}
		for _, row := range rows {
			b := authz.Binding{Role: row.RoleName}
			if row.ScopeKind == authz.ScopeKindProject {
				if len(row.ProjectID) != 16 {
					return fmt.Errorf("binding project: %w", identity.ErrInvalidRecord)
				}
				b.Project = bytesToUUID(row.ProjectID).String()
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
	err := s.readOf(ctx, "list custom role permissions", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListCustomRolePermissions(ctx, uuidBytes(identity.OrgIDOf(org)), authzRowLimit)
		if err != nil {
			return err
		}
		for _, row := range rows {
			role := bytesToUUID(row.ID).String()
			out[role] = append(out[role], authz.Permission(row.Permission))
		}
		return nil
	})
	return out, err
}

// ProjectOrganization implements [authz.Store]: the organization of a project, across
// organizations (the engine plane, see the query's policy entry).
func (s *Store) ProjectOrganization(ctx context.Context, project string) (identity.OrgID, error) {
	pid, err := id.Parse(project)
	if err != nil {
		return identity.OrgID{}, fmt.Errorf("%w: project identifier", identity.ErrNotFound)
	}
	var out identity.OrgID
	err = s.read(ctx, "get project organization", func(ctx context.Context, q *sqlcgen.Queries) error {
		raw, err := q.GetProjectOrganization(ctx, pid.Bytes())
		if err != nil {
			return err
		}
		out, err = uuidOf[identity.OrgID](raw)
		return err
	})
	return out, err
}

// bytesToUUID copies 16 stored bytes into a UUID value.
func bytesToUUID(b []byte) id.UUID {
	var u id.UUID
	copy(u[:], b)
	return u
}
