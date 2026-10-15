// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"time"

	"github.com/wirvii/orcon/internal/platform/id"
)

// Scope kinds of a role binding (the stored form of SPEC-004 R-AZ-02).
const (
	// RoleScopeOrg is a binding at the organization: it applies to every project.
	RoleScopeOrg = "ORG"
	// RoleScopeProject is a binding in one project.
	RoleScopeProject = "PROJECT"
)

// Predefined role names that the identity area grants. They are the names of the
// authorization catalog (internal/authz, ADR-0041); the authorization area owns the
// permissions of each one, identity only writes the binding.
const (
	// RoleOwner is the organization owner.
	RoleOwner = "Owner"
	// RoleAdmin is the organization administrator.
	RoleAdmin = "Admin"
	// RoleProjectAdmin administers one project; it is granted to the creator of a project (SPEC-016 Q3).
	RoleProjectAdmin = "Project Admin"
)

// RoleBindingRecord is a row of role_binding: one predefined or custom role given to one user,
// at the organization (ProjectID zero) or in one project.
type RoleBindingRecord struct {
	ID        RoleBindingID
	OrgID     OrgID
	ScopeKind string
	// ProjectID is zero for an organization binding.
	ProjectID ProjectID
	UserID    UserID
	RoleName  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RoleBindingID identifies a role binding.
type RoleBindingID id.UUID

// String returns the canonical text form of the identity.
func (r RoleBindingID) String() string { return id.UUID(r).String() }

// IsZero reports whether r is the nil identity.
func (r RoleBindingID) IsZero() bool { return id.UUID(r).IsZero() }
