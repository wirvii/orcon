// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package issue is the only place an organization scope is created
// (ADR-0046 section 2, SPEC-004 R-ISO-01). The type itself is declared here so
// that its field is unexported and no composite literal outside this package
// can fill it; [github.com/wirvii/orcon/internal/platform/orgscope] re-exports
// the type without the constructor, which is what every store port names.
//
// Importing this package is the act of creating a scope, so it is confined by
// the orgscope-issue-confined depguard rule to the places ADR-0046 names: the
// authorization context (internal/authz), the engine (internal/execution and
// the adapter code that turns the rows of an engine function into scopes) and
// tests. A handler never builds a scope from a request value.
package issue

import (
	"errors"

	"github.com/wirvii/orcon/internal/platform/id"
)

// ErrNilOrg reports an attempt to scope to the nil identity.
var ErrNilOrg = errors.New("orgscope: the nil identity is not an organization")

// Org is a proof that the caller acts for one organization. Every store method
// of organization data takes it and binds it to the org_id of its statement; the
// zero value scopes to nothing and every store rejects it.
type Org struct {
	uuid id.UUID
}

// New returns the scope of the organization u. It fails for the nil identity.
func New(u id.UUID) (Org, error) {
	if u.IsZero() {
		return Org{}, ErrNilOrg
	}
	return Org{uuid: u}, nil
}

// UUID returns the identity of the organization.
func (o Org) UUID() id.UUID { return o.uuid }

// IsZero reports whether o is the zero scope, which scopes to nothing.
func (o Org) IsZero() bool { return o.uuid.IsZero() }

// String returns the lowercase hyphenated identity.
func (o Org) String() string { return o.uuid.String() }
