// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package identity is the business area of principals, users, organizations,
// memberships and projects (ADR-0038, SPEC-004 R-ID-01 to R-ID-04 and R-ID-11).
// It owns the rows of those tables and the ports its stores implement; the
// adapters internal/adapter/sqlite and internal/adapter/postgres persist them.
// Sessions, authentication, authorization and the instance roles arrive with
// the later cuts of SPEC-004. It is an area package: it imports no adapter.
package identity

import (
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// OrgID identifies an organization. The adapters convert it to and from
// [id.UUID]; no other package does.
type OrgID id.UUID

// String returns the lowercase hyphenated form.
func (i OrgID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i OrgID) IsZero() bool { return id.UUID(i).IsZero() }

// OrgIDOf returns the identity of the organization a scope stands for.
func OrgIDOf(o orgscope.Org) OrgID { return OrgID(o.UUID()) }

// Matches reports whether o is the scope of the organization i. A record that
// names its organization is written only under the scope of that organization.
func (i OrgID) Matches(o orgscope.Org) bool { return !o.IsZero() && id.UUID(i) == o.UUID() }

// PrincipalID identifies a principal: whoever acts.
type PrincipalID id.UUID

// String returns the lowercase hyphenated form.
func (i PrincipalID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i PrincipalID) IsZero() bool { return id.UUID(i).IsZero() }

// ProjectID identifies a project.
type ProjectID id.UUID

// String returns the lowercase hyphenated form.
func (i ProjectID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i ProjectID) IsZero() bool { return id.UUID(i).IsZero() }

// UserID identifies a user: the instance-wide account of a person.
type UserID id.UUID

// String returns the lowercase hyphenated form.
func (i UserID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i UserID) IsZero() bool { return id.UUID(i).IsZero() }

// MembershipID identifies a membership: the access of one user to one organization.
type MembershipID id.UUID

// String returns the lowercase hyphenated form.
func (i MembershipID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i MembershipID) IsZero() bool { return id.UUID(i).IsZero() }
