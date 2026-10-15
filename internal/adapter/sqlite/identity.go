// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// CreateOrganization implements [identity.OrgStore].
func (s *Store) CreateOrganization(ctx context.Context, o identity.OrgRecord) error {
	return s.write(ctx, "create organization", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertOrganization(ctx, sqlcgen.InsertOrganizationParams{
			ID: uuidBytes(o.ID), Name: o.Name, Slug: o.Slug, Status: o.Status,
			DeletionScheduledAt: nullMicros(o.DeletionScheduledAt),
			CreatedAt:           micros(o.CreatedAt), UpdatedAt: micros(o.UpdatedAt),
		})
	})
}

// GetOrganization implements [identity.OrgStore].
func (s *Store) GetOrganization(ctx context.Context, org orgscope.Org) (identity.OrgRecord, error) {
	var out identity.OrgRecord
	err := s.read(ctx, "get organization", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		row, err := q.GetOrganization(ctx, scope)
		if err != nil {
			return err
		}
		oid, err := uuidOf[identity.OrgID](row.ID)
		if err != nil {
			return err
		}
		out = identity.OrgRecord{
			ID: oid, Name: row.Name, Slug: row.Slug, Status: row.Status,
			DeletionScheduledAt: nullInstant(row.DeletionScheduledAt),
			CreatedAt:           instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
		}
		return nil
	})
	return out, err
}

// CreatePrincipal implements [identity.PrincipalStore].
func (s *Store) CreatePrincipal(ctx context.Context, p identity.PrincipalRecord) error {
	return s.write(ctx, "create principal", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertPrincipal(ctx, sqlcgen.InsertPrincipalParams{
			ID: uuidBytes(p.ID), Kind: string(p.Kind), CreatedAt: micros(p.CreatedAt), UpdatedAt: micros(p.UpdatedAt),
		})
	})
}

// GetSystemPrincipal implements [identity.PrincipalStore].
func (s *Store) GetSystemPrincipal(ctx context.Context) (identity.PrincipalRecord, error) {
	var out identity.PrincipalRecord
	err := s.read(ctx, "get system principal", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetSystemPrincipal(ctx)
		if err != nil {
			return err
		}
		pid, err := uuidOf[identity.PrincipalID](row.ID)
		if err != nil {
			return err
		}
		out = identity.PrincipalRecord{ID: pid, Kind: identity.PrincipalKind(row.Kind), CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt)}
		return nil
	})
	return out, err
}

// clearer clears the actor column of one table in a batch and returns how many rows it changed.
type clearer struct {
	// column is "table.column", the spelling PrincipalReferenceColumns reports.
	column string
	run    func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error)
}

// principalClearers is every actor column that can name a principal. A migration that adds an
// actor column adds its clearing statement here and to the PostgreSQL function it replaces;
// the contract suite fails until both know the column.
var principalClearers = []clearer{
	{"flow.deleted_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateFlowDeletedByPrincipalToNull(ctx, principal, batch)
	}},
	{"flow_version.published_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateFlowVersionPublishedByPrincipalToNull(ctx, principal, batch)
	}},
	{"membership.invited_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateMembershipInvitedByPrincipalToNull(ctx, principal, batch)
	}},
	{"instance_alert.acknowledged_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateInstanceAlertAcknowledgedByPrincipalToNull(ctx, principal, batch)
	}},
	{"external_operation.resolved_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateExternalOperationResolvedByPrincipalToNull(ctx, principal, batch)
	}},
	{"human_task.resolved_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateHumanTaskResolvedByPrincipalToNull(ctx, principal, batch)
	}},
	// The acknowledged statement of an attention request runs before the resolved one: a row that names the
	// principal in both columns is cleared by the first and left to the next batch by the second, as in
	// the PostgreSQL function.
	{"attention_request.acknowledged_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateAttentionRequestAcknowledgedByPrincipalToNull(ctx, principal, batch)
	}},
	{"attention_request.resolved_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateAttentionRequestResolvedByPrincipalToNull(ctx, principal, batch)
	}},
	{"operational_alert.acknowledged_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateOperationalAlertAcknowledgedByPrincipalToNull(ctx, principal, batch)
	}},
	{"connector_package.installed_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateConnectorPackageInstalledByPrincipalToNull(ctx, principal, batch)
	}},
	{"connector_grant.granted_by_principal_id", func(ctx context.Context, q *sqlcgen.Queries, principal []byte, batch int64) (int64, error) {
		return q.UpdateConnectorGrantGrantedByPrincipalToNull(ctx, principal, batch)
	}},
}

// PrincipalReferenceColumns returns the actor columns, as "table.column", that DeletePrincipal
// clears in batches. The contract suite compares them with the actor columns of the schema.
func PrincipalReferenceColumns() []string {
	out := make([]string, len(principalClearers))
	for i := range principalClearers {
		out[i] = principalClearers[i].column
	}
	return out
}

// DeletePrincipal implements [identity.PrincipalStore]: bounded batches that
// set the actor columns to NULL, each its own transaction, then the delete of
// the principal row (orcon-db IN-12).
func (s *Store) DeletePrincipal(ctx context.Context, principal identity.PrincipalID, batchSize int) error {
	batch := clampPage(batchSize)
	for i := range principalClearers {
		step := &principalClearers[i]
		for {
			var changed int64
			err := s.write(ctx, "clear principal references", func(ctx context.Context, q *sqlcgen.Queries) error {
				var err error
				changed, err = step.run(ctx, q, uuidBytes(principal), batch)
				return err
			})
			if err != nil {
				return err
			}
			if changed < batch {
				break
			}
		}
	}
	return s.write(ctx, "delete principal", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.DeletePrincipal(ctx, uuidBytes(principal))
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("principal %s: %w", principal, identity.ErrNotFound)
		}
		return nil
	})
}

func userFrom(id, principal []byte, email, displayName, status string, created, updated int64) (identity.UserRecord, error) {
	uid, err := uuidOf[identity.UserID](id)
	if err != nil {
		return identity.UserRecord{}, fmt.Errorf("user id: %w", err)
	}
	pid, err := uuidOf[identity.PrincipalID](principal)
	if err != nil {
		return identity.UserRecord{}, fmt.Errorf("user principal id: %w", err)
	}
	return identity.UserRecord{
		ID: uid, PrincipalID: pid, EmailNormalized: email, DisplayName: displayName, Status: status,
		CreatedAt: instant(created), UpdatedAt: instant(updated),
	}, nil
}

// CreateUser implements [identity.UserStore]: the principal and the user in one transaction.
func (s *Store) CreateUser(ctx context.Context, u identity.UserRecord) error {
	return s.write(ctx, "create user", func(ctx context.Context, q *sqlcgen.Queries) error {
		return insertUser(ctx, q, u)
	})
}

// CreateUserWithEntry implements [identity.UserStore]: the principal, the user and the audit
// entry of the creation in one transaction.
func (s *Store) CreateUserWithEntry(ctx context.Context, u identity.UserRecord, passwordHash string, entry audit.Entry) error {
	return s.write(ctx, "create user", func(ctx context.Context, q *sqlcgen.Queries) error {
		if err := insertUser(ctx, q, u); err != nil {
			return err
		}
		if _, err := q.UpdateApplicationUserPasswordHash(ctx, sql.NullString{String: passwordHash, Valid: true}, micros(u.CreatedAt), uuidBytes(u.ID)); err != nil {
			return err
		}
		return insertPendingInstance(ctx, q, entry)
	})
}

// insertUser writes the principal row and the user row of a new user.
func insertUser(ctx context.Context, q *sqlcgen.Queries, u identity.UserRecord) error {
	if err := q.InsertPrincipal(ctx, sqlcgen.InsertPrincipalParams{
		ID: uuidBytes(u.PrincipalID), Kind: string(identity.PrincipalUser), CreatedAt: micros(u.CreatedAt), UpdatedAt: micros(u.UpdatedAt),
	}); err != nil {
		return err
	}
	return q.InsertApplicationUser(ctx, sqlcgen.InsertApplicationUserParams{
		ID: uuidBytes(u.ID), PrincipalID: uuidBytes(u.PrincipalID), EmailNormalized: u.EmailNormalized,
		DisplayName: u.DisplayName, Status: u.Status, CreatedAt: micros(u.CreatedAt), UpdatedAt: micros(u.UpdatedAt),
	})
}

// GetUser implements [identity.UserStore].
func (s *Store) GetUser(ctx context.Context, user identity.UserID) (identity.UserRecord, error) {
	var out identity.UserRecord
	err := s.read(ctx, "get user", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetApplicationUser(ctx, uuidBytes(user))
		if err != nil {
			return err
		}
		out, err = userFrom(row.ID, row.PrincipalID, row.EmailNormalized, row.DisplayName, row.Status, row.CreatedAt, row.UpdatedAt)
		return err
	})
	return out, err
}

// GetUserByEmail implements [identity.UserStore].
func (s *Store) GetUserByEmail(ctx context.Context, emailNormalized string) (identity.UserRecord, error) {
	var out identity.UserRecord
	err := s.read(ctx, "get user by email", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetApplicationUserByEmail(ctx, emailNormalized)
		if err != nil {
			return err
		}
		out, err = userFrom(row.ID, row.PrincipalID, row.EmailNormalized, row.DisplayName, row.Status, row.CreatedAt, row.UpdatedAt)
		return err
	})
	return out, err
}

func membershipFrom(row *sqlcgen.Membership) (identity.MembershipRecord, error) {
	var out identity.MembershipRecord
	var err error
	if out.ID, err = uuidOf[identity.MembershipID](row.ID); err != nil {
		return out, fmt.Errorf("membership id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("membership org id: %w", err)
	}
	if out.UserID, err = uuidOf[identity.UserID](row.ApplicationUserID); err != nil {
		return out, fmt.Errorf("membership user id: %w", err)
	}
	if out.InvitedByPrincipalID, err = nullUUIDOf[identity.PrincipalID](row.InvitedByPrincipalID); err != nil {
		return out, fmt.Errorf("membership invited by principal id: %w", err)
	}
	out.Status = row.Status
	out.CreatedAt = instant(row.CreatedAt)
	out.UpdatedAt = instant(row.UpdatedAt)
	return out, nil
}

// CreateMembership implements [identity.MembershipStore].
func (s *Store) CreateMembership(ctx context.Context, org orgscope.Org, m identity.MembershipRecord) error {
	return s.write(ctx, "create membership", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scopedRecord(org, m.OrgID)
		if err != nil {
			return err
		}
		return q.InsertMembership(ctx, sqlcgen.InsertMembershipParams{
			ID: uuidBytes(m.ID), OrgID: scope, ApplicationUserID: uuidBytes(m.UserID), Status: m.Status,
			InvitedByPrincipalID: nullUUIDBytes(m.InvitedByPrincipalID),
			CreatedAt:            micros(m.CreatedAt), UpdatedAt: micros(m.UpdatedAt),
		})
	})
}

// GetMembership implements [identity.MembershipStore].
func (s *Store) GetMembership(ctx context.Context, org orgscope.Org, membership identity.MembershipID) (identity.MembershipRecord, error) {
	var out identity.MembershipRecord
	err := s.read(ctx, "get membership", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		row, err := q.GetMembership(ctx, scope, uuidBytes(membership))
		if err != nil {
			return err
		}
		out, err = membershipFrom(&row)
		return err
	})
	return out, err
}

// GetMembershipOfUser implements [identity.MembershipStore].
func (s *Store) GetMembershipOfUser(ctx context.Context, org orgscope.Org, user identity.UserID) (identity.MembershipRecord, error) {
	var out identity.MembershipRecord
	err := s.read(ctx, "get membership of user", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		row, err := q.GetMembershipOfApplicationUser(ctx, scope, uuidBytes(user))
		if err != nil {
			return err
		}
		out, err = membershipFrom(&row)
		return err
	})
	return out, err
}

// ListMemberships implements [identity.MembershipStore].
func (s *Store) ListMemberships(ctx context.Context, org orgscope.Org, after identity.MembershipID, limit int) ([]identity.MembershipRecord, error) {
	var out []identity.MembershipRecord
	err := s.read(ctx, "list memberships", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		rows, err := q.ListMemberships(ctx, scope, uuidBytes(after), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			m, err := membershipFrom(&rows[i])
			if err != nil {
				return err
			}
			out = append(out, m)
		}
		return nil
	})
	return out, err
}

// UpdateMembershipStatus implements [identity.MembershipStore].
func (s *Store) UpdateMembershipStatus(ctx context.Context, org orgscope.Org, membership identity.MembershipID, status string, now time.Time) error {
	return s.write(ctx, "update membership status", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		n, err := q.UpdateMembershipStatus(ctx, sqlcgen.UpdateMembershipStatusParams{
			Status: status, UpdatedAt: micros(now), OrgID: scope, ID: uuidBytes(membership),
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
	return s.write(ctx, "create project", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scopedRecord(org, p.OrgID)
		if err != nil {
			return err
		}
		if err := q.InsertProject(ctx, sqlcgen.InsertProjectParams{
			ID: uuidBytes(p.ID), OrgID: scope, Name: p.Name, Slug: p.Slug, Handle: nullText(p.Handle),
			Environment: nullText(p.Environment), IsProtected: boolInt(p.IsProtected),
			CreatedAt: micros(p.CreatedAt), UpdatedAt: micros(p.UpdatedAt),
		}); err != nil {
			return err
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// projectRow is the columns of a project row, whichever query read it.
type projectRow = sqlcgen.GetProjectRow

func projectFrom(row *projectRow) (identity.ProjectRecord, error) {
	pid, err := uuidOf[identity.ProjectID](row.ID)
	if err != nil {
		return identity.ProjectRecord{}, fmt.Errorf("project id: %w", err)
	}
	oid, err := uuidOf[identity.OrgID](row.OrgID)
	if err != nil {
		return identity.ProjectRecord{}, fmt.Errorf("project org id: %w", err)
	}
	return identity.ProjectRecord{
		ID: pid, OrgID: oid, Name: row.Name, Slug: row.Slug, Handle: row.Handle.String, Environment: row.Environment.String,
		IsProtected: row.IsProtected == 1, CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}, nil
}

// GetProject implements [identity.ProjectStore].
func (s *Store) GetProject(ctx context.Context, org orgscope.Org, project identity.ProjectID) (identity.ProjectRecord, error) {
	var out identity.ProjectRecord
	err := s.read(ctx, "get project", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		row, err := q.GetProject(ctx, scope, uuidBytes(project))
		if err != nil {
			return err
		}
		out, err = projectFrom(&row)
		return err
	})
	return out, err
}

// ListProjects implements [identity.ProjectStore].
func (s *Store) ListProjects(ctx context.Context, org orgscope.Org, after identity.ProjectID, limit int) ([]identity.ProjectRecord, error) {
	var out []identity.ProjectRecord
	err := s.read(ctx, "list projects", func(ctx context.Context, q *sqlcgen.Queries) error {
		scope, err := scoped(org)
		if err != nil {
			return err
		}
		rows, err := q.ListProjects(ctx, scope, uuidBytes(after), clampPage(limit))
		if err != nil {
			return err
		}
		for i := range rows {
			converted := projectRow(rows[i])
			p, err := projectFrom(&converted)
			if err != nil {
				return err
			}
			out = append(out, p)
		}
		return nil
	})
	return out, err
}
