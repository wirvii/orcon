// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// CreateProjectWithGrant implements [identity.ProjectStore]: the project, the role binding of
// its creator, the authorization epoch increment and the audit entries, in one transaction.
// The idempotency claim of the context, when there is one, is written in the same transaction.
func (s *Store) CreateProjectWithGrant(ctx context.Context, org orgscope.Org, p identity.ProjectRecord, grant identity.RoleBindingRecord, entries []audit.Entry) error {
	return s.inScopeRecord(ctx, "create project", org, p.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		if !grant.OrgID.Matches(org) {
			return fmt.Errorf("role binding of organization %s: %w", grant.OrgID, identity.ErrScopeMismatch)
		}
		if err := claimIdempotency(ctx, q, org, "project:"+id.UUID(p.ID).String(), p.CreatedAt); err != nil {
			return err
		}
		if err := q.InsertProject(ctx, sqlcgen.InsertProjectParams{
			ID: pgUUID(p.ID), OrgID: pgUUID(org.UUID()), Name: p.Name, Slug: p.Slug, Handle: nullText(p.Handle),
			Environment: nullText(p.Environment), IsProtected: p.IsProtected,
			CreatedAt: timestamp(p.CreatedAt), UpdatedAt: timestamp(p.UpdatedAt),
		}); err != nil {
			return fmt.Errorf("insert project: %w", err)
		}
		if err := insertRoleBindingRow(ctx, q, org.UUID(), grant); err != nil {
			return err
		}
		if _, err := q.UpdateAuthorizationEpoch(ctx, pgUUID(org.UUID())); err != nil {
			return fmt.Errorf("bump authorization epoch: %w", err)
		}
		for i := range entries {
			if err := insertPendingOrg(ctx, q, org, entries[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// insertRoleBindingRow writes a role binding of the organization org.
func insertRoleBindingRow(ctx context.Context, q *sqlcgen.Queries, org id.UUID, b identity.RoleBindingRecord) error {
	if err := q.InsertRoleBinding(ctx, sqlcgen.InsertRoleBindingParams{
		ID: pgUUID(b.ID), OrgID: pgUUID(org), ScopeKind: b.ScopeKind, ProjectID: nullPGUUID(b.ProjectID),
		ApplicationUserID: pgUUID(b.UserID), RoleName: b.RoleName,
		CreatedAt: timestamp(b.CreatedAt), UpdatedAt: timestamp(b.UpdatedAt),
	}); err != nil {
		return fmt.Errorf("insert role binding: %w", err)
	}
	return nil
}

// CreateOrganizationWithAdmin implements [identity.OrgStore]: the organization, the membership
// and role binding of its administrator and the audit entries of the creation, in one
// transaction. The transaction runs in the organization being created, so the row-level
// security of its rows admits them.
func (s *Store) CreateOrganizationWithAdmin(ctx context.Context, o identity.OrgRecord, key identity.OrgKeyRecord, m identity.MembershipRecord, grant identity.RoleBindingRecord, entries []audit.Entry) error {
	if m.OrgID != o.ID || grant.OrgID != o.ID {
		return fmt.Errorf("create organization %s: %w", o.ID, identity.ErrScopeMismatch)
	}
	return s.inOrg(ctx, "create organization", o.ID, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		if err := q.InsertOrganization(ctx, sqlcgen.InsertOrganizationParams{
			ID: pgUUID(o.ID), Name: o.Name, Slug: o.Slug, Status: o.Status,
			DeletionScheduledAt: nullTimestamp(o.DeletionScheduledAt),
			CreatedAt:           timestamp(o.CreatedAt), UpdatedAt: timestamp(o.UpdatedAt),
		}); err != nil {
			return fmt.Errorf("insert organization: %w", err)
		}
		if err := q.InsertOrgKey(ctx, sqlcgen.InsertOrgKeyParams{
			OrgID: pgUUID(o.ID), KeyVersion: int64(key.Version), Wrapped: key.Wrapped,
			MasterKeyReference: key.MasterKeyReference, Status: key.Status, Probe: key.Probe,
			CreatedAt: timestamp(key.CreatedAt),
		}); err != nil {
			return fmt.Errorf("insert organization key: %w", err)
		}
		if err := q.InsertMembership(ctx, sqlcgen.InsertMembershipParams{
			ID: pgUUID(m.ID), OrgID: pgUUID(o.ID), ApplicationUserID: pgUUID(m.UserID), Status: m.Status,
			InvitedByPrincipalID: nullPGUUID(m.InvitedByPrincipalID),
			CreatedAt:            timestamp(m.CreatedAt), UpdatedAt: timestamp(m.UpdatedAt),
		}); err != nil {
			return fmt.Errorf("insert membership: %w", err)
		}
		if err := insertRoleBindingRow(ctx, q, id.UUID(o.ID), grant); err != nil {
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

// insertPendingChain writes one pending entry of an organization chain, named by the chain.
func insertPendingChain(ctx context.Context, q *sqlcgen.Queries, org id.UUID, e audit.Entry) error {
	if !e.Chain.Valid() || e.Chain.IsInstance() {
		return fmt.Errorf("audit entry: %w", audit.ErrInvalidEntry)
	}
	if e.Chain.Org() != org {
		return fmt.Errorf("audit entry of another organization: %w", identity.ErrScopeMismatch)
	}
	if err := q.InsertAuditPending(ctx, sqlcgen.InsertAuditPendingParams{
		ID: pgUUID(e.ID), OrgID: pgUUID(e.Chain.Org()), OccurredAt: timestamp(e.OccurredAt),
		ActorPrincipalID: pgPrincipal(e.Actor), ActorKind: pgKind(e.Actor), ActionName: e.Action,
		Outcome: string(e.Outcome), BodyDocument: e.Body, BodyDocumentFormat: audit.Format,
		CreatedAt: timestamp(e.OccurredAt), UpdatedAt: timestamp(e.OccurredAt),
	}); err != nil {
		return fmt.Errorf("insert audit pending: %w", err)
	}
	return nil
}
