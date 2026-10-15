// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import "time"

// Organization statuses (ADR-0038 section 7). DELETION_SCHEDULED is the grace
// period before destruction, a lifecycle status and not a soft delete.
const (
	OrgActive            = "ACTIVE"
	OrgSuspended         = "SUSPENDED"
	OrgDeletionScheduled = "DELETION_SCHEDULED"
)

// OrgRecord is a row of org.
type OrgRecord struct {
	ID   OrgID
	Name string
	// Slug is unique across the instance and is the organization prefix of a
	// project handle that needed one (ADR-0038 section 1).
	Slug   string
	Status string
	// DeletionScheduledAt is set exactly when Status is [OrgDeletionScheduled].
	DeletionScheduledAt time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// PrincipalRecord is a row of principal: whoever acts. The kinds are the
// [PrincipalKind] constants of authn.go (ADR-0038 section 3).
type PrincipalRecord struct {
	ID        PrincipalID
	Kind      PrincipalKind
	CreatedAt time.Time
	UpdatedAt time.Time
}

// User statuses.
const (
	UserActive   = "ACTIVE"
	UserDisabled = "DISABLED"
)

// UserRecord is a row of application_user: the account of a person, global to the
// instance and reached by organizations through memberships. Its principal row
// is created and deleted with it.
type UserRecord struct {
	ID          UserID
	PrincipalID PrincipalID
	// EmailNormalized is the normalized address, unique across the instance.
	EmailNormalized string
	DisplayName     string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Membership statuses (ADR-0038 section 2).
const (
	MembershipInvited   = "INVITED"
	MembershipActive    = "ACTIVE"
	MembershipSuspended = "SUSPENDED"
)

// MembershipRecord is a row of membership: the access of one user to one
// organization. A user without an ACTIVE membership cannot observe the
// organization or its existence (SPEC-004 R-ID-02).
type MembershipRecord struct {
	ID     MembershipID
	OrgID  OrgID
	UserID UserID
	Status string
	// InvitedByPrincipalID is the zero value when the inviting principal was deleted.
	InvitedByPrincipalID PrincipalID
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// ProjectRecord is a row of project.
type ProjectRecord struct {
	ID    ProjectID
	OrgID OrgID
	Name  string
	// Slug is unique within the organization.
	Slug string
	// Handle is unique across the instance: the slug, or "<orgSlug>-<slug>" when
	// another organization holds the slug (SPEC-004 R-ID-11). It is the
	// <project> segment of native webhook addresses (ADR-0022 section 1). Empty
	// only for a project written before handles existed.
	Handle string
	// Environment is a free label; empty means the project has none.
	Environment string
	// IsProtected is true for new projects.
	IsProtected bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
