// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The ports below are what the identity area needs from durable storage
// (ADR-0015 section 1, architecture-layers.md AL-3). The adapters
// internal/adapter/sqlite and internal/adapter/postgres implement them and
// internal/adapter/storetest proves both behave the same.
//
// Row identities are typed values created by the caller as UUIDv7 (orcon-db
// ID-1). A method of organization data takes the [orgscope.Org] first after the
// context and binds it to the org_id of its statement: a row of another
// organization is never visible and a record that names another organization
// than its scope is refused. A method that does not take one declares why in an
// "orcon:scope" directive line of its comment, which the orgscope analyzer of
// tools/orconvet requires (ADR-0046 section 2).

// OrgStore persists organizations.
type OrgStore interface {
	// CreateOrganization inserts an organization. A repeated slug fails with
	// [ErrConflict].
	//
	//orcon:scope instance the organization does not exist before this call, so there is no scope to hold
	CreateOrganization(ctx context.Context, o OrgRecord) error
	// GetOrganization returns the organization of the scope, or [ErrNotFound].
	GetOrganization(ctx context.Context, org orgscope.Org) (OrgRecord, error)
}

// PrincipalStore persists principals, the rows of whoever acts.
type PrincipalStore interface {
	// CreatePrincipal inserts a principal. A second SYSTEM principal fails with
	// [ErrConflict].
	//
	//orcon:scope instance principal is an instance table (orcon-db IN-12)
	CreatePrincipal(ctx context.Context, p PrincipalRecord) error
	// GetSystemPrincipal returns the single SYSTEM principal, or [ErrNotFound]
	// before the instance is bootstrapped.
	//
	//orcon:scope instance principal is an instance table (orcon-db IN-12)
	GetSystemPrincipal(ctx context.Context) (PrincipalRecord, error)
	// DeletePrincipal deletes a principal without deleting any history: in
	// batches of at most batchSize rows it sets to NULL every actor column that
	// names the principal, in every organization, and then deletes the principal
	// row, which takes its user with it. Each batch is its own short transaction.
	// An unknown principal fails with [ErrNotFound].
	//
	//orcon:scope engine the actor columns to clear are in every organization (orcon-db IN-12, QY-8)
	DeletePrincipal(ctx context.Context, principal PrincipalID, batchSize int) error
}

// UserStore persists users.
type UserStore interface {
	// CreateUser inserts the user together with its principal row (kind USER),
	// in one transaction. A repeated address fails with [ErrConflict].
	//
	//orcon:scope instance application_user is an instance table
	CreateUser(ctx context.Context, u UserRecord) error
	// GetUser returns a user by identity, or [ErrNotFound].
	//
	//orcon:scope instance application_user is an instance table
	GetUser(ctx context.Context, user UserID) (UserRecord, error)
	// GetUserByEmail returns the user with that normalized address, or [ErrNotFound].
	//
	//orcon:scope instance application_user is an instance table
	GetUserByEmail(ctx context.Context, emailNormalized string) (UserRecord, error)
	// GetUserCredentialsByEmail returns the user with that normalized address and
	// its stored password hash, or [ErrNotFound]. Only the sign-in path reads the hash.
	//
	//orcon:scope instance application_user is an instance table
	GetUserCredentialsByEmail(ctx context.Context, emailNormalized string) (UserCredentials, error)
	// SetPasswordHash stores the password hash of the user and the instant of the
	// change, or fails with [ErrNotFound].
	//
	//orcon:scope instance application_user is an instance table
	SetPasswordHash(ctx context.Context, user UserID, hash string, now time.Time) error
}

// SessionStore persists login sessions.
type SessionStore interface {
	// CreateSession inserts a session. A repeated token hash fails with [ErrConflict];
	// an unknown user with [ErrReference].
	//
	//orcon:scope instance session is an instance table
	CreateSession(ctx context.Context, s SessionRecord) error
	// GetSessionByTokenHash returns the session with that token hash and its user,
	// revoked or not, or [ErrNotFound].
	//
	//orcon:scope instance session is an instance table
	GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (SessionView, error)
	// GetSession returns one session of the user, or [ErrNotFound].
	//
	//orcon:scope instance session is an instance table
	GetSession(ctx context.Context, user UserID, session SessionID) (SessionRecord, error)
	// UpdateSessionActivity moves the last-seen instant and the idle deadline of a
	// live session, or fails with [ErrNotFound] when it is revoked or unknown.
	//
	//orcon:scope instance session is an instance table
	UpdateSessionActivity(ctx context.Context, session SessionID, lastSeen, idleExpiresAt time.Time) error
	// MarkSessionRevoked sets the revocation instant of a live session of the user,
	// or fails with [ErrNotFound].
	//
	//orcon:scope instance session is an instance table
	MarkSessionRevoked(ctx context.Context, user UserID, session SessionID, at time.Time) error
}

// LoginThrottleStore persists the failed sign-in attempts per keyed identifier.
type LoginThrottleStore interface {
	// GetLoginThrottle returns the state of the identifier, or [ErrNotFound].
	//
	//orcon:scope instance login_throttle is an instance table
	GetLoginThrottle(ctx context.Context, identifier []byte) (LoginThrottle, error)
	// PutLoginThrottle stores the state of the identifier, replacing any previous one.
	//
	//orcon:scope instance login_throttle is an instance table
	PutLoginThrottle(ctx context.Context, identifier []byte, t LoginThrottle) error
	// ClearLoginThrottle removes the state of the identifier; an unknown one is not an error.
	//
	//orcon:scope instance login_throttle is an instance table
	ClearLoginThrottle(ctx context.Context, identifier []byte) error
	// IncrementLoginThrottle counts one failed attempt in one atomic statement and returns the
	// count after it, so parallel failures are never lost (SPEC-004 R-AUTH-02).
	//
	//orcon:scope instance login_throttle is an instance table
	IncrementLoginThrottle(ctx context.Context, identifier []byte, now time.Time) (int, error)
	// LockLoginThrottle sets the instant before which the identifier is refused, or fails with [ErrNotFound].
	//
	//orcon:scope instance login_throttle is an instance table
	LockLoginThrottle(ctx context.Context, identifier []byte, until, now time.Time) error
}

// MembershipStore persists the memberships of an organization.
type MembershipStore interface {
	// CreateMembership inserts a membership. A second membership of the same user
	// in the organization fails with [ErrConflict]; an unknown user with [ErrReference].
	CreateMembership(ctx context.Context, org orgscope.Org, m MembershipRecord) error
	// GetMembership returns one membership of the organization, or [ErrNotFound].
	GetMembership(ctx context.Context, org orgscope.Org, membership MembershipID) (MembershipRecord, error)
	// GetMembershipOfUser returns the membership of the user in the organization,
	// or [ErrNotFound].
	GetMembershipOfUser(ctx context.Context, org orgscope.Org, user UserID) (MembershipRecord, error)
	// ListMemberships returns at most limit memberships of the organization with
	// an identity greater than after, ordered by identity (keyset pagination).
	ListMemberships(ctx context.Context, org orgscope.Org, after MembershipID, limit int) ([]MembershipRecord, error)
	// UpdateMembershipStatus sets the status and the update instant of one
	// membership of the organization, or fails with [ErrNotFound].
	UpdateMembershipStatus(ctx context.Context, org orgscope.Org, membership MembershipID, status string, now time.Time) error
}

// ProjectStore persists projects.
type ProjectStore interface {
	// CreateProject inserts a project. A second project with the same slug in the
	// organization fails with [ErrConflict]; a handle held by any project of the
	// instance fails with [ErrHandleTaken]; an unknown organization with
	// [ErrReference].
	CreateProject(ctx context.Context, org orgscope.Org, p ProjectRecord) error
	// GetProject returns one project of the organization, or [ErrNotFound].
	GetProject(ctx context.Context, org orgscope.Org, project ProjectID) (ProjectRecord, error)
	// ListProjects returns at most limit projects of the organization with an
	// identity greater than after, ordered by identity (keyset pagination).
	ListProjects(ctx context.Context, org orgscope.Org, after ProjectID, limit int) ([]ProjectRecord, error)
}

// OrganizationResolver lists the organizations a user belongs to through an active
// membership of an active organization (SPEC-004 R-ID-02, R-ID-03). Membership rows
// belong to many organizations, so the read crosses them in the engine plane.
type OrganizationResolver interface {
	// ResolveUserOrganizations returns at most limit organizations of the user, ordered
	// by name and then id. The adapters cap limit at [MaxResolvedOrganizations].
	//
	//orcon:scope engine membership rows of every organization are read for one user (SPEC-004 section 7-3)
	ResolveUserOrganizations(ctx context.Context, user UserID, limit int) ([]OrganizationChoice, error)
}
