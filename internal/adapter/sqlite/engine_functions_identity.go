// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// ResolveUserOrganizations implements [identity.OrganizationResolver]: the active
// memberships of active organizations of the user, by name, at most limit rows.
func (s *Store) ResolveUserOrganizations(ctx context.Context, user identity.UserID, limit int) ([]identity.OrganizationChoice, error) {
	out := []identity.OrganizationChoice{}
	err := s.read(ctx, "resolve user organizations", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ResolveUserOrganizations(ctx, uuidBytes(user), int64(clampRows(limit)))
		if err != nil {
			return err
		}
		for _, row := range rows {
			choice, err := organizationChoice(row.ID, row.Slug, row.Name)
			if err != nil {
				return err
			}
			out = append(out, choice)
		}
		return nil
	})
	return out, err
}

// organizationChoice builds the choice of one row, with the scope the adapter issues for it.
func organizationChoice(orgID []byte, slug, name string) (identity.OrganizationChoice, error) {
	oid, err := uuidOf[identity.OrgID](orgID)
	if err != nil {
		return identity.OrganizationChoice{}, fmt.Errorf("organization id: %w", err)
	}
	scope, err := issue.New(id.UUID(oid))
	if err != nil {
		return identity.OrganizationChoice{}, fmt.Errorf("organization scope: %w", err)
	}
	return identity.OrganizationChoice{Scope: scope, ID: oid, Slug: slug, Name: name}, nil
}

// clampRows bounds a listing to the cap of the contract and to at least one row.
func clampRows(limit int) int {
	switch {
	case limit < 1:
		return 1
	case limit > identity.MaxResolvedOrganizations:
		return identity.MaxResolvedOrganizations
	default:
		return limit
	}
}
