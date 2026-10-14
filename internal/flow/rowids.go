// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import "github.com/wirvii/orcon/internal/platform/id"

// The row identities of the flow tables are distinct types, so a flow
// identity cannot be passed where a version identity is wanted
// (architecture-layers.md AL-4.4). They are not [ID], the identifier a flow
// document carries about itself. The storage adapters convert them to and from
// [id.UUID]; no other package does.

// FlowID identifies a row of flow.
type FlowID id.UUID //nolint:revive // the name is fixed by the storage ruling of SPEC-001 C2

// String returns the lowercase hyphenated form.
func (i FlowID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i FlowID) IsZero() bool { return id.UUID(i).IsZero() }

// FlowVersionID identifies a row of flow_version.
type FlowVersionID id.UUID //nolint:revive // the name is fixed by the storage ruling of SPEC-001 C2

// String returns the lowercase hyphenated form.
func (i FlowVersionID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i FlowVersionID) IsZero() bool { return id.UUID(i).IsZero() }

// FlowSnapshotID identifies a row of flow_snapshot.
type FlowSnapshotID id.UUID //nolint:revive // the name is fixed by the storage ruling of SPEC-001 C2

// String returns the lowercase hyphenated form.
func (i FlowSnapshotID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i FlowSnapshotID) IsZero() bool { return id.UUID(i).IsZero() }

// ConnectorReferenceID identifies a row of connector_reference.
type ConnectorReferenceID id.UUID

// String returns the lowercase hyphenated form.
func (i ConnectorReferenceID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i ConnectorReferenceID) IsZero() bool { return id.UUID(i).IsZero() }
