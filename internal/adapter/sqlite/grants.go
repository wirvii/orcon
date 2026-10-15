// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// CreateProjectWithGrant implements [identity.ProjectStore]: the project, the role binding of
// its creator, the authorization epoch increment and the audit entries, in one transaction.
// The idempotency claim of the context, when there is one, is written in the same transaction.
func (s *Store) CreateProjectWithGrant(ctx context.Context, org orgscope.Org, p identity.ProjectRecord, grant identity.RoleBindingRecord, entries []audit.Entry) error {
	return s.write(ctx, "create project", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scopedRecord(org, p.OrgID)
		if err != nil {
			return err
		}
		if !grant.OrgID.Matches(org) {
			return fmt.Errorf("role binding of organization %s: %w", grant.OrgID, identity.ErrScopeMismatch)
		}
		if err := claimIdempotency(ctx, q, org, "project:"+id.UUID(p.ID).String(), p.CreatedAt); err != nil {
			return err
		}
		if err := insertProjectRow(ctx, q, scope, p); err != nil {
			return err
		}
		if err := insertRoleBindingRow(ctx, q, scope, grant); err != nil {
			return err
		}
		if err := bumpAuthorizationEpoch(ctx, q, scope); err != nil {
			return err
		}
		for i := range entries {
			if err := insertPendingOrg(ctx, q, org, entries[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// insertProjectRow writes the row of a project.
func insertProjectRow(ctx context.Context, q *sqlcgen.Queries, scope []byte, p identity.ProjectRecord) error {
	if err := q.InsertProject(ctx, sqlcgen.InsertProjectParams{
		ID: uuidBytes(p.ID), OrgID: scope, Name: p.Name, Slug: p.Slug, Handle: nullText(p.Handle),
		Environment: nullText(p.Environment), IsProtected: boolInt(p.IsProtected),
		CreatedAt: micros(p.CreatedAt), UpdatedAt: micros(p.UpdatedAt),
	}); err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

// insertRoleBindingRow writes a role binding of the organization whose scope is given.
func insertRoleBindingRow(ctx context.Context, q *sqlcgen.Queries, scope []byte, b identity.RoleBindingRecord) error {
	var project []byte
	if b.ScopeKind == identity.RoleScopeProject {
		project = uuidBytes(b.ProjectID)
	}
	if err := q.InsertRoleBinding(ctx, sqlcgen.InsertRoleBindingParams{
		ID: uuidBytes(b.ID), OrgID: scope, ScopeKind: b.ScopeKind, ProjectID: project,
		ApplicationUserID: uuidBytes(b.UserID), RoleName: b.RoleName,
		CreatedAt: micros(b.CreatedAt), UpdatedAt: micros(b.UpdatedAt),
	}); err != nil {
		return fmt.Errorf("insert role binding: %w", err)
	}
	return nil
}

// bumpAuthorizationEpoch moves the authorization epoch of the organization, so cached grants
// are recomputed on the next request (SPEC-004 R-AZ-06).
func bumpAuthorizationEpoch(ctx context.Context, q *sqlcgen.Queries, scope []byte) error {
	n, err := q.UpdateAuthorizationEpoch(ctx, scope)
	if err != nil {
		return fmt.Errorf("bump authorization epoch: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("bump authorization epoch: %w", identity.ErrNotFound)
	}
	return nil
}

// CreateOrganizationWithAdmin implements [identity.OrgStore]: the organization, the membership
// and role binding of its administrator and the audit entries of the creation, in one
// transaction. The organization is new, so its authorization epoch starts at zero.
func (s *Store) CreateOrganizationWithAdmin(ctx context.Context, o identity.OrgRecord, m identity.MembershipRecord, grant identity.RoleBindingRecord, entries []audit.Entry) error {
	if m.OrgID != o.ID || grant.OrgID != o.ID {
		return fmt.Errorf("create organization %s: %w", o.ID, identity.ErrScopeMismatch)
	}
	return s.write(ctx, "create organization", func(ctx context.Context, q *sqlcgen.Queries) error {
		if err := q.InsertOrganization(ctx, sqlcgen.InsertOrganizationParams{
			ID: uuidBytes(o.ID), Name: o.Name, Slug: o.Slug, Status: o.Status,
			DeletionScheduledAt: nullMicros(o.DeletionScheduledAt),
			CreatedAt:           micros(o.CreatedAt), UpdatedAt: micros(o.UpdatedAt),
		}); err != nil {
			return fmt.Errorf("insert organization: %w", err)
		}
		scope := uuidBytes(o.ID)
		if err := q.InsertMembership(ctx, sqlcgen.InsertMembershipParams{
			ID: uuidBytes(m.ID), OrgID: scope, ApplicationUserID: uuidBytes(m.UserID), Status: m.Status,
			InvitedByPrincipalID: nullUUIDBytes(m.InvitedByPrincipalID),
			CreatedAt:            micros(m.CreatedAt), UpdatedAt: micros(m.UpdatedAt),
		}); err != nil {
			return fmt.Errorf("insert membership: %w", err)
		}
		if err := insertRoleBindingRow(ctx, q, scope, grant); err != nil {
			return err
		}
		for i := range entries {
			if err := insertPendingChain(ctx, q, id.UUID(o.ID), entries[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// insertPendingChain writes one pending entry of an organization chain; the chain names the
// organization, so the entry needs no scope value (the organization may be new).
func insertPendingChain(ctx context.Context, q *sqlcgen.Queries, org id.UUID, e audit.Entry) error {
	if !e.Chain.Valid() || e.Chain.IsInstance() {
		return fmt.Errorf("audit entry: %w", audit.ErrInvalidEntry)
	}
	if e.Chain.Org() != org {
		return fmt.Errorf("audit entry of another organization: %w", identity.ErrScopeMismatch)
	}
	return insertPendingRow(ctx, q, uuidBytes(org), e)
}

// insertPendingRow writes the pending row of an organization entry under the given organization.
func insertPendingRow(ctx context.Context, q *sqlcgen.Queries, orgID []byte, e audit.Entry) error {
	if err := q.InsertAuditPending(ctx, pendingParams(orgID, e)); err != nil {
		return fmt.Errorf("insert audit pending: %w", err)
	}
	return nil
}
