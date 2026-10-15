// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// ResolveUserOrganizations implements [identity.OrganizationResolver] through the
// engine function orcon_engine.resolve_user_organizations of migration 0015.
func (s *Store) ResolveUserOrganizations(ctx context.Context, user identity.UserID, limit int) ([]identity.OrganizationChoice, error) {
	out := []identity.OrganizationChoice{}
	err := s.engineRows(ctx, "resolve user organizations",
		"SELECT org_id, org_slug, org_name FROM orcon_engine.resolve_user_organizations($1, $2)",
		[]any{pgUUID(user), clampRows(limit)},
		func(rows pgx.Rows) error {
			var orgID pgtype.UUID
			var slug, name string
			if err := rows.Scan(&orgID, &slug, &name); err != nil {
				return err
			}
			choice, err := organizationChoice(fromPGUUID[identity.OrgID](orgID), slug, name)
			if err != nil {
				return err
			}
			out = append(out, choice)
			return nil
		})
	return out, err
}

// organizationChoice builds the choice of one row, with the scope the adapter issues for it.
func organizationChoice(oid identity.OrgID, slug, name string) (identity.OrganizationChoice, error) {
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
