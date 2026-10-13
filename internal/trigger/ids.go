// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import "github.com/wirvii/orcon/internal/platform/id"

// The row identities of the trigger tables are distinct types, so one cannot
// be passed where another is wanted (architecture-layers.md AL-4.4). The
// storage adapters convert them to and from [id.UUID]; no other package does.

// ID identifies a row of flow_trigger.
type ID id.UUID

// String returns the lowercase hyphenated form.
func (i ID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i ID) IsZero() bool { return id.UUID(i).IsZero() }

// AddressID identifies a row of trigger_address.
type AddressID id.UUID

// String returns the lowercase hyphenated form.
func (i AddressID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i AddressID) IsZero() bool { return id.UUID(i).IsZero() }

// MethodID identifies a row of trigger_address_method.
type MethodID id.UUID

// String returns the lowercase hyphenated form.
func (i MethodID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i MethodID) IsZero() bool { return id.UUID(i).IsZero() }

// EventID identifies a row of trigger_event.
type EventID id.UUID

// String returns the lowercase hyphenated form.
func (i EventID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i EventID) IsZero() bool { return id.UUID(i).IsZero() }
