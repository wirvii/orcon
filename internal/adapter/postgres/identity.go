// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// CreateOrganization implements [identity.OrgStore]. The row-level security
// policy of org compares the row to the organization context, so the insert
// runs in the context of the organization it creates.
func (s *Store) CreateOrganization(ctx context.Context, o identity.OrgRecord) error {
	return s.inOrg(ctx, "create organization", o.ID, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertOrganization(ctx, sqlcgen.InsertOrganizationParams{
			ID: pgUUID(o.ID), Name: o.Name, Slug: o.Slug, Status: o.Status,
			DeletionScheduledAt: nullTimestamp(o.DeletionScheduledAt),
			CreatedAt:           timestamp(o.CreatedAt), UpdatedAt: timestamp(o.UpdatedAt),
		})
	})
}

// GetOrganization implements [identity.OrgStore].
func (s *Store) GetOrganization(ctx context.Context, org orgscope.Org) (identity.OrgRecord, error) {
	var out identity.OrgRecord
	err := s.inScope(ctx, "get organization", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetOrganization(ctx, pgUUID(org.UUID()))
		if err != nil {
			return err
		}
		out = identity.OrgRecord{
			ID: fromPGUUID[identity.OrgID](row.ID), Name: row.Name, Slug: row.Slug, Status: row.Status,
			DeletionScheduledAt: instant(row.DeletionScheduledAt),
			CreatedAt:           instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
		}
		return nil
	})
	return out, err
}

// CreatePrincipal implements [identity.PrincipalStore].
func (s *Store) CreatePrincipal(ctx context.Context, p identity.PrincipalRecord) error {
	return s.instance(ctx, "create principal", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertPrincipal(ctx, sqlcgen.InsertPrincipalParams{
			ID: pgUUID(p.ID), Kind: string(p.Kind), CreatedAt: timestamp(p.CreatedAt), UpdatedAt: timestamp(p.UpdatedAt),
		})
	})
}

// GetSystemPrincipal implements [identity.PrincipalStore].
func (s *Store) GetSystemPrincipal(ctx context.Context) (identity.PrincipalRecord, error) {
	var out identity.PrincipalRecord
	err := s.instance(ctx, "get system principal", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetSystemPrincipal(ctx)
		if err != nil {
			return err
		}
		out = identity.PrincipalRecord{ID: fromPGUUID[identity.PrincipalID](row.ID), Kind: identity.PrincipalKind(row.Kind), CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt)}
		return nil
	})
	return out, err
}

// DeletePrincipal implements [identity.PrincipalStore]: bounded batches that set
// the actor columns to NULL through the engine function
// orcon_engine.clear_principal_references (each call is one statement, hence
// one short transaction), then the delete of the principal row (orcon-db IN-12).
func (s *Store) DeletePrincipal(ctx context.Context, principal identity.PrincipalID, batchSize int) error {
	batch := clampPage(batchSize)
	for {
		changed, err := s.clearPrincipalReferences(ctx, principal, batch)
		if err != nil {
			return err
		}
		if changed == 0 {
			break
		}
	}
	return s.instance(ctx, "delete principal", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.DeletePrincipal(ctx, pgUUID(principal))
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("principal %s: %w", principal, identity.ErrNotFound)
		}
		return nil
	})
}

// clearPrincipalReferences runs one batch of the engine function and returns how many rows it cleared.
// The call is hand-written because sqlc cannot analyze a function with a SECURITY DEFINER body
// (data-access.md DA-8, orcon-db QY-10); the contract tests cover it.
func (s *Store) clearPrincipalReferences(ctx context.Context, principal identity.PrincipalID, batch int32) (int32, error) {
	const op = "clear principal references"
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return 0, fmt.Errorf("%s: %w", op, ErrClosed)
	}
	var changed int32
	err := s.pool.QueryRow(ctx, "SELECT orcon_engine.clear_principal_references($1, $2)", pgUUID(principal), batch).Scan(&changed)
	return changed, mapError(op, err)
}

func userFrom(id, principal pgtype.UUID, email, displayName, status string, created, updated pgtype.Timestamptz) identity.UserRecord {
	return identity.UserRecord{
		ID: fromPGUUID[identity.UserID](id), PrincipalID: fromPGUUID[identity.PrincipalID](principal),
		EmailNormalized: email, DisplayName: displayName, Status: status,
		CreatedAt: instant(created), UpdatedAt: instant(updated),
	}
}

// CreateUser implements [identity.UserStore]: the principal and the user in one transaction.
func (s *Store) CreateUser(ctx context.Context, u identity.UserRecord) error {
	return s.instanceTx(ctx, "create user", func(ctx context.Context, q *sqlcgen.Queries) error {
		return insertUser(ctx, q, u)
	})
}

// CreateUserWithEntry implements [identity.UserStore]: the principal, the user and the audit
// entry of the creation in one transaction.
func (s *Store) CreateUserWithEntry(ctx context.Context, u identity.UserRecord, passwordHash string, entry audit.Entry) error {
	return s.instanceTx(ctx, "create user", func(ctx context.Context, q *sqlcgen.Queries) error {
		if err := insertUser(ctx, q, u); err != nil {
			return err
		}
		if _, err := q.UpdateApplicationUserPasswordHash(ctx, pgtype.Text{String: passwordHash, Valid: true}, timestamp(u.CreatedAt), pgUUID(u.ID)); err != nil {
			return err
		}
		return insertPendingInstance(ctx, q, entry)
	})
}

// insertUser writes the principal row and the user row of a new user.
func insertUser(ctx context.Context, q *sqlcgen.Queries, u identity.UserRecord) error {
	if err := q.InsertPrincipal(ctx, sqlcgen.InsertPrincipalParams{
		ID: pgUUID(u.PrincipalID), Kind: string(identity.PrincipalUser), CreatedAt: timestamp(u.CreatedAt), UpdatedAt: timestamp(u.UpdatedAt),
	}); err != nil {
		return err
	}
	return q.InsertApplicationUser(ctx, sqlcgen.InsertApplicationUserParams{
		ID: pgUUID(u.ID), PrincipalID: pgUUID(u.PrincipalID), EmailNormalized: u.EmailNormalized,
		DisplayName: u.DisplayName, Status: u.Status, CreatedAt: timestamp(u.CreatedAt), UpdatedAt: timestamp(u.UpdatedAt),
	})
}

// GetUser implements [identity.UserStore].
func (s *Store) GetUser(ctx context.Context, user identity.UserID) (identity.UserRecord, error) {
	var out identity.UserRecord
	err := s.instance(ctx, "get user", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetApplicationUser(ctx, pgUUID(user))
		if err != nil {
			return err
		}
		out = userFrom(row.ID, row.PrincipalID, row.EmailNormalized, row.DisplayName, row.Status, row.CreatedAt, row.UpdatedAt)
		return nil
	})
	return out, err
}

// GetUserByEmail implements [identity.UserStore].
func (s *Store) GetUserByEmail(ctx context.Context, emailNormalized string) (identity.UserRecord, error) {
	var out identity.UserRecord
	err := s.instance(ctx, "get user by email", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetApplicationUserByEmail(ctx, emailNormalized)
		if err != nil {
			return err
		}
		out = userFrom(row.ID, row.PrincipalID, row.EmailNormalized, row.DisplayName, row.Status, row.CreatedAt, row.UpdatedAt)
		return nil
	})
	return out, err
}

func membershipFrom(row *sqlcgen.Membership) identity.MembershipRecord {
	return identity.MembershipRecord{
		ID: fromPGUUID[identity.MembershipID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID),
		UserID: fromPGUUID[identity.UserID](row.ApplicationUserID), Status: row.Status,
		InvitedByPrincipalID: fromPGUUID[identity.PrincipalID](row.InvitedByPrincipalID),
		CreatedAt:            instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
}

// CreateMembership implements [identity.MembershipStore].
func (s *Store) CreateMembership(ctx context.Context, org orgscope.Org, m identity.MembershipRecord) error {
	return s.inScopeRecord(ctx, "create membership", org, m.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertMembership(ctx, sqlcgen.InsertMembershipParams{
			ID: pgUUID(m.ID), OrgID: pgUUID(org.UUID()), ApplicationUserID: pgUUID(m.UserID), Status: m.Status,
			InvitedByPrincipalID: nullPGUUID(m.InvitedByPrincipalID),
			CreatedAt:            timestamp(m.CreatedAt), UpdatedAt: timestamp(m.UpdatedAt),
		})
	})
}

// GetMembership implements [identity.MembershipStore].
func (s *Store) GetMembership(ctx context.Context, org orgscope.Org, membership identity.MembershipID) (identity.MembershipRecord, error) {
	var out identity.MembershipRecord
	err := s.inScope(ctx, "get membership", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetMembership(ctx, pgUUID(org.UUID()), pgUUID(membership))
		if err != nil {
			return err
		}
		out = membershipFrom(&row)
		return nil
	})
	return out, err
}

// GetMembershipOfUser implements [identity.MembershipStore].
func (s *Store) GetMembershipOfUser(ctx context.Context, org orgscope.Org, user identity.UserID) (identity.MembershipRecord, error) {
	var out identity.MembershipRecord
	err := s.inScope(ctx, "get membership of user", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetMembershipOfApplicationUser(ctx, pgUUID(org.UUID()), pgUUID(user))
		if err != nil {
			return err
		}
		out = membershipFrom(&row)
		return nil
	})
	return out, err
}

// ListMemberships implements [identity.MembershipStore].
func (s *Store) ListMemberships(ctx context.Context, org orgscope.Org, after identity.MembershipID, limit int) ([]identity.MembershipRecord, error) {
	var out []identity.MembershipRecord
	err := s.inScope(ctx, "list memberships", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListMemberships(ctx, pgUUID(org.UUID()), pgUUID(after), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			out = append(out, membershipFrom(&rows[i]))
		}
		return nil
	})
	return out, err
}

// UpdateMembershipStatus implements [identity.MembershipStore].
func (s *Store) UpdateMembershipStatus(ctx context.Context, org orgscope.Org, membership identity.MembershipID, status string, now time.Time) error {
	return s.inScope(ctx, "update membership status", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpdateMembershipStatus(ctx, sqlcgen.UpdateMembershipStatusParams{
			Status: status, UpdatedAt: timestamp(now), OrgID: pgUUID(org.UUID()), ID: pgUUID(membership),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("membership %s: %w", membership, identity.ErrNotFound)
		}
		return nil
	})
}

// CreateProject implements [identity.ProjectStore]. The creation entry joins the
// organization chain in the same transaction (R-AUD-02).
func (s *Store) CreateProject(ctx context.Context, org orgscope.Org, p identity.ProjectRecord, entry audit.Entry) error {
	return s.inScopeRecord(ctx, "create project", org, p.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		if err := q.InsertProject(ctx, sqlcgen.InsertProjectParams{
			ID: pgUUID(p.ID), OrgID: pgUUID(org.UUID()), Name: p.Name, Slug: p.Slug, Handle: nullText(p.Handle),
			Environment: nullText(p.Environment), IsProtected: p.IsProtected,
			CreatedAt: timestamp(p.CreatedAt), UpdatedAt: timestamp(p.UpdatedAt),
		}); err != nil {
			return err
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
}

// projectRow is the columns of a project row, whichever query read it.
type projectRow = sqlcgen.GetProjectRow

func projectFrom(row *projectRow) identity.ProjectRecord {
	return identity.ProjectRecord{
		ID: fromPGUUID[identity.ProjectID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), Name: row.Name, Slug: row.Slug,
		Handle: row.Handle.String, Environment: row.Environment.String, IsProtected: row.IsProtected,
		CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
}

// GetProject implements [identity.ProjectStore].
func (s *Store) GetProject(ctx context.Context, org orgscope.Org, project identity.ProjectID) (identity.ProjectRecord, error) {
	var out identity.ProjectRecord
	err := s.inScope(ctx, "get project", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetProject(ctx, pgUUID(org.UUID()), pgUUID(project))
		if err != nil {
			return err
		}
		out = projectFrom(&row)
		return nil
	})
	return out, err
}

// ListProjects implements [identity.ProjectStore].
func (s *Store) ListProjects(ctx context.Context, org orgscope.Org, after identity.ProjectID, limit int) ([]identity.ProjectRecord, error) {
	var out []identity.ProjectRecord
	err := s.inScope(ctx, "list projects", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListProjects(ctx, pgUUID(org.UUID()), pgUUID(after), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			converted := projectRow(rows[i])
			out = append(out, projectFrom(&converted))
		}
		return nil
	})
	return out, err
}
