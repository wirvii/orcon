// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import "github.com/wirvii/orcon/internal/platform/id"

// The row identities of the connector tables are distinct types, so a package
// identity cannot be passed where a grant identity is wanted
// (architecture-layers.md AL-4.4). The storage adapters convert them to and
// from [id.UUID]; no other package does.

// PackageID identifies a row of connector_package.
type PackageID id.UUID

// String returns the lowercase hyphenated form.
func (i PackageID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i PackageID) IsZero() bool { return id.UUID(i).IsZero() }

// GrantID identifies a row of connector_grant.
type GrantID id.UUID

// String returns the lowercase hyphenated form.
func (i GrantID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i GrantID) IsZero() bool { return id.UUID(i).IsZero() }
