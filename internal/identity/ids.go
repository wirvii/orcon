// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package identity is the business area of principals, organizations and
// projects. Today it holds only their row identities, so the ports of other
// areas can name them with distinct types (architecture-layers.md AL-4.4); the
// rules and the ports of the area arrive with the identity cut (SPEC-004).
// It is an area package: it imports no adapter.
package identity

import "github.com/wirvii/orcon/internal/platform/id"

// OrgID identifies an organization. The adapters convert it to and from
// [id.UUID]; no other package does.
type OrgID id.UUID

// String returns the lowercase hyphenated form.
func (i OrgID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i OrgID) IsZero() bool { return id.UUID(i).IsZero() }

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
