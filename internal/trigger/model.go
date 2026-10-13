// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"slices"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// Status is the state of a trigger in the registry.
type Status string

// Trigger statuses, the spelling the API exposes (ADR-0080 section 3).
const (
	// StatusActive means the trigger accepts events: its flow is active and
	// the node is part of the live version.
	StatusActive Status = "ACTIVE"
	// StatusInactive means the flow is inactive or deleted.
	StatusInactive Status = "INACTIVE"
	// StatusRemoved means the node is no longer a trigger of the live version.
	// The row stays so that its removed addresses can answer 410 for the grace
	// period (ADR-0022 section 2).
	StatusRemoved Status = "REMOVED"
)

// Health is the health of a trigger (R-OBS-03). T1 only ever writes OK; the
// families that degrade a trigger arrive with their cuts.
type Health string

// Trigger health values.
const (
	// HealthOK means nothing is wrong.
	HealthOK Health = "OK"
	// HealthPending means the trigger waits for something, such as an
	// external registration.
	HealthPending Health = "PENDING"
	// HealthDegraded means the trigger works with failures.
	HealthDegraded Health = "DEGRADED"
	// HealthFailing means the trigger does not work.
	HealthFailing Health = "FAILING"
)

// Scope tells whether an address serves production or the test session of a
// draft (ADR-0022 section 1).
type Scope string

// Address scopes.
const (
	// ScopeProduction is the stable address of the live version.
	ScopeProduction Scope = "PRODUCTION"
	// ScopeTest is the address of an open test session.
	ScopeTest Scope = "TEST"
)

// AddressStatus is the state of an address.
type AddressStatus string

// Address statuses.
const (
	// AddressActive means the address routes to its trigger.
	AddressActive AddressStatus = "ACTIVE"
	// AddressRetired means the address was removed by a publication; it
	// answers 410 during the grace period and 404 afterwards.
	AddressRetired AddressStatus = "RETIRED"
)

// RetiredGrace is how long a retired address keeps answering 410 and keeps its
// path reserved (ADR-0022 section 2).
const RetiredGrace = 30 * 24 * time.Hour

// RecoveryWindow is how long a deleted flow can be restored before retention
// purges it (ADR-0077 section 4).
const RecoveryWindow = 30 * 24 * time.Hour

// Trigger is a row of flow_trigger: the registry entry of one trigger node of
// a flow. Its key is the pair (flow, node); node ids are stable across
// versions, so the entry survives them (ADR-0020 section 1).
type Trigger struct {
	ID    ID
	OrgID identity.OrgID
	// FlowID is the row identity of the flow.
	FlowID flow.FlowID
	NodeID flow.NodeID
	Family flow.TriggerFamily
	// Type is the node type, for example "orcon.webhook@1".
	Type string
	// ConfigurationHash decides whether the state of the trigger survives a
	// publication (ADR-0024 section 3).
	ConfigurationHash string
	// FlowVersionID is the version the entry reflects.
	FlowVersionID flow.FlowVersionID
	Status        Status
	Health        Health
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AddressMethod is a row of trigger_address_method: one HTTP method an
// address accepts.
type AddressMethod struct {
	ID     MethodID
	Method string
}

// Address is a row of trigger_address with its methods.
type Address struct {
	ID        AddressID
	OrgID     identity.OrgID
	TriggerID ID
	Family    flow.AddressFamily
	Scope     Scope
	// Pattern is the full path with a leading slash. Segments that start with
	// a colon are route parameters.
	Pattern string
	// StaticPrefix is the part of Pattern before the first parameter.
	StaticPrefix string
	// Host is the dedicated ingress host of the address; empty for the shared one.
	Host string
	// RandomSuffix is the unguessable last segment of a native address.
	RandomSuffix    string
	ResponseProfile flow.ResponseProfile
	Methods         []AddressMethod
	Status          AddressStatus
	// RetiredAt is the zero time unless Status is [AddressRetired].
	RetiredAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MethodNames returns the accepted methods in a stable order.
func (a *Address) MethodNames() []string {
	out := make([]string, len(a.Methods))
	for i, m := range a.Methods {
		out[i] = m.Method
	}
	slices.Sort(out)
	return out
}

// FlowState is what the lifecycle needs to know about the flow row.
type FlowState struct {
	ID        flow.FlowID
	ProjectID identity.ProjectID
	Name      string
	// ActivationState is the state the flow is in.
	ActivationState flow.ActivationState
	// ActiveVersionID is the zero value while nothing is published.
	ActiveVersionID flow.FlowVersionID
	DraftRevision   int64
	// DeletedAt is the zero time while the flow is not deleted.
	DeletedAt time.Time
}

// IsDeleted reports whether the flow is soft-deleted.
func (s FlowState) IsDeleted() bool { return !s.DeletedAt.IsZero() }

// Route is one (address, method) pair of an organization, as the collision
// check reads it.
type Route struct {
	AddressID AddressID
	TriggerID ID
	FlowID    flow.FlowID
	// FlowName is the name of the flow that owns the address.
	FlowName string
	NodeID   flow.NodeID
	Pattern  string
	Method   string
	Status   AddressStatus
	// RetiredAt is the zero time unless Status is [AddressRetired].
	RetiredAt time.Time
}

// Acceptance is what an ingress acceptance transaction learns about a trigger:
// the version to pin an event to, or the reason it is refused.
type Acceptance struct {
	TriggerID ID
	Family    flow.TriggerFamily
	Type      string
	NodeID    flow.NodeID
	// FlowVersionID and VersionNumber name the version the event is pinned to.
	FlowVersionID flow.FlowVersionID
	VersionNumber int64
	// ConfigurationHash is the hash of the trigger the version carries.
	ConfigurationHash string
	// Accepting is true when the flow is active, not deleted, and the trigger
	// is part of the live version. A refused event carries the reason
	// ErrInactive.
	Accepting bool
}

// Event types of the trigger event stream that T1 writes (ADR-0020 section 7).
const (
	// EventFlowPublished is written when a version is published.
	EventFlowPublished = "flow.published"
	// EventFlowActivated is written when a flow is activated.
	EventFlowActivated = "flow.activated"
	// EventFlowDeactivated is written when a flow is deactivated or deleted.
	EventFlowDeactivated = "flow.deactivated"
	// EventFlowRolledBack is written when an earlier version is republished.
	EventFlowRolledBack = "flow.rolled_back"
)

// EventFormat is the format tag of the data document of a trigger event.
const EventFormat = "orcon/trigger-event@1"

// EventSchemaVersion is the schema version of the events T1 writes.
const EventSchemaVersion = 1

// Event is a row of trigger_event. The stream is append-only and kept 30 days.
type Event struct {
	ID         EventID
	OrgID      identity.OrgID
	OccurredAt time.Time
	FlowID     flow.FlowID
	// NodeID is empty for an event about the whole flow.
	NodeID flow.NodeID
	Type   string
	// Data is the data document: a JSON object in [EventFormat].
	Data []byte
}
