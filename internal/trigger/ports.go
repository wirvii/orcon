// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Store is the storage port of the trigger area. It runs a unit of work in one
// transaction, so the lifecycle rules live here once and each adapter only
// implements the small operations of [Tx] (architecture-layers.md AL-3).
//
// The function passed to Atomic and Read must only call the methods of its
// [Tx] and pure code: it runs inside a database transaction, which must never
// stay open across a network or connector call (orcon-db QY-8).
type Store interface {
	// Atomic runs fn in one read-write transaction scoped to org. When fn
	// returns an error the transaction is rolled back and Atomic returns that
	// error; storage errors are mapped to the errors of the flow area.
	Atomic(ctx context.Context, org identity.OrgID, fn func(ctx context.Context, tx Tx) error) error
	// Read runs fn on one consistent read-only view scoped to org.
	Read(ctx context.Context, org identity.OrgID, fn func(ctx context.Context, tx Tx) error) error
}

// Tx is one transaction of a [Store], bound to an organization. Every method
// of an organization-scoped row acts on that organization only (ADR-0046).
//
//nolint:interfacebloat // one method per storage operation; the lifecycle composes them
type Tx interface {
	// GetFlowState returns the flow row, including a soft-deleted one, or
	// flow.ErrNotFound.
	GetFlowState(ctx context.Context, flowID flow.FlowID) (FlowState, error)
	// LockFlowState is GetFlowState that also locks the row against concurrent
	// lifecycle changes until the transaction ends. Only Atomic may call it.
	LockFlowState(ctx context.Context, flowID flow.FlowID) (FlowState, error)
	// NextVersionNumber returns the number of the next version of the flow.
	NextVersionNumber(ctx context.Context, flowID flow.FlowID) (int64, error)
	// InsertVersion inserts a published version.
	InsertVersion(ctx context.Context, v flow.VersionRecord) error
	// SetActiveVersion points the flow at its live version.
	SetActiveVersion(ctx context.Context, flowID flow.FlowID, version flow.FlowVersionID, now time.Time) error
	// ReplaceDraft replaces the draft when its revision is still
	// expectedRevision, or fails with flow.ErrDraftConflict.
	ReplaceDraft(ctx context.Context, flowID flow.FlowID, draft []byte, expectedRevision int64, now time.Time) error
	// SetActivationState writes the activation state of the flow.
	SetActivationState(ctx context.Context, flowID flow.FlowID, state flow.ActivationState, now time.Time) error
	// SoftDeleteFlow marks the flow deleted and records who did it.
	SoftDeleteFlow(ctx context.Context, flowID flow.FlowID, by identity.PrincipalID, now time.Time) error
	// RestoreFlow clears the deletion marks. A flow with the same name in the
	// project fails with flow.ErrConflict and nothing changes.
	RestoreFlow(ctx context.Context, flowID flow.FlowID, now time.Time) error
	// PurgeFlow deletes the flow row for good, with its versions, snapshots,
	// triggers, addresses and trigger events.
	PurgeFlow(ctx context.Context, flowID flow.FlowID) error

	// ListTriggers returns every registry entry of the flow, removed ones
	// included, ordered by node id.
	ListTriggers(ctx context.Context, flowID flow.FlowID) ([]Trigger, error)
	// InsertTrigger inserts a registry entry.
	InsertTrigger(ctx context.Context, t Trigger) error
	// UpdateTrigger rewrites the type, family, configuration hash, version,
	// status, health and last error of an entry.
	UpdateTrigger(ctx context.Context, t Trigger) error
	// SetTriggerStatuses sets the status of every entry of the flow that is not
	// removed.
	SetTriggerStatuses(ctx context.Context, flowID flow.FlowID, status Status, now time.Time) error

	// ListAddresses returns the addresses of an entry with their methods.
	ListAddresses(ctx context.Context, trigger ID) ([]Address, error)
	// InsertAddress inserts an address and its methods.
	InsertAddress(ctx context.Context, a *Address) error
	// UpdateAddress rewrites the status, retirement, response profile, host and
	// random suffix of an address.
	UpdateAddress(ctx context.Context, a *Address) error
	// ReplaceAddressMethods replaces the methods of an address.
	ReplaceAddressMethods(ctx context.Context, address AddressID, methods []AddressMethod, now time.Time) error

	// LockRoute serializes the claims of one path across organizations for the
	// rest of the transaction, so two publications cannot both take it. Stores
	// with a single writer have nothing to do.
	LockRoute(ctx context.Context, scope Scope, pattern string) error
	// ListRoutes returns the (address, method) pairs of the organization in
	// the scope with a method identity greater than after, ordered by method
	// identity. after is the zero value for the first page.
	ListRoutes(ctx context.Context, scope Scope, after MethodID, limit int) ([]RoutePage, error)
	// RouteTakenByOtherOrganization reports whether an address of another
	// organization holds the path with the method: active, or retired after
	// retiredAfter. It answers yes or no and nothing else: no row of another
	// organization is ever read in the caller's context (ADR-0022 section 3).
	RouteTakenByOtherOrganization(ctx context.Context, scope Scope, pattern, method string, retiredAfter time.Time) (bool, error)
	// NotifyRoutesChanged tells the other processes that the route index must
	// be rebuilt. Stores without a notification channel do nothing.
	NotifyRoutesChanged(ctx context.Context, flowID flow.FlowID) error

	// AppendEvent appends an event to the trigger event stream.
	AppendEvent(ctx context.Context, e Event) error
	// ListEvents returns at most limit events of the flow with an identity
	// greater than after, ordered by identity.
	ListEvents(ctx context.Context, flowID flow.FlowID, after EventID, limit int) ([]Event, error)

	// ResolveAcceptance reads, in one statement, what an ingress acceptance
	// needs about the trigger of a node: the pinned version and whether the
	// trigger accepts events. A flow or node that does not exist fails with
	// flow.ErrNotFound; a deleted flow answers with Accepting false.
	ResolveAcceptance(ctx context.Context, flowID flow.FlowID, node flow.NodeID) (Acceptance, error)
}

// RoutePage is one row of [Tx.ListRoutes]: a [Route] and the identity that
// orders the pages.
type RoutePage struct {
	Route
	// MethodID is the key of the page.
	MethodID MethodID
}

// FlowSource is the part of the flow storage the service reads and writes
// outside a lifecycle transaction. The adapters of the flow area implement it.
type FlowSource interface {
	// GetFlow returns a non-deleted flow, or flow.ErrNotFound.
	GetFlow(ctx context.Context, org identity.OrgID, flowID flow.FlowID) (flow.Record, error)
	// GetVersion returns a version by identity, or flow.ErrVersionNotFound.
	GetVersion(ctx context.Context, org identity.OrgID, version flow.FlowVersionID) (flow.VersionRecord, error)
	// GetVersionByNumber returns a version of the flow by its number, or
	// flow.ErrVersionNotFound.
	GetVersionByNumber(ctx context.Context, org identity.OrgID, flowID flow.FlowID, number int64) (flow.VersionRecord, error)
	// CreateSnapshot stores a snapshot, or returns the one with the same hash.
	CreateSnapshot(ctx context.Context, s flow.SnapshotRecord) (flow.SnapshotRecord, error)
	// GetProject returns one project of the organization, or flow.ErrNotFound.
	GetProject(ctx context.Context, org identity.OrgID, project identity.ProjectID) (flow.ProjectRecord, error)
}

// Delivery is the guarantee a source gives about delivering an event
// (ADR-0020 section 3).
type Delivery string

// Delivery guarantees.
const (
	// DeliveryAtLeastOnce redelivers until acknowledged; duplicates are possible.
	DeliveryAtLeastOnce Delivery = "AT_LEAST_ONCE"
	// DeliveryAtMostOnce never redelivers; loss is possible.
	DeliveryAtMostOnce Delivery = "AT_MOST_ONCE"
	// DeliveryCursorExact is delivery tracked by a cursor stored with the acceptance.
	DeliveryCursorExact Delivery = "CURSOR_EXACT"
)

// Deliveries returns every valid [Delivery].
func Deliveries() []Delivery {
	return []Delivery{DeliveryAtLeastOnce, DeliveryAtMostOnce, DeliveryCursorExact}
}

// Declaration is what a trigger node type declares about itself (ADR-0020
// section 3). A node type without a valid declaration cannot be published.
type Declaration struct {
	Family flow.TriggerFamily
	// Delivery is the guarantee of the source.
	Delivery Delivery
	// DedupeKeySource says where the deduplication key comes from when the
	// node type fixes it; empty if the author chooses.
	DedupeKeySource flow.DedupeSource
	// SensitiveFields lists the fields of the event to mask.
	SensitiveFields []string
	// MaxPayloadBytes is the largest event the type accepts.
	MaxPayloadBytes int64
	// N8NTypes lists the n8n node types this type replaces.
	N8NTypes []string
}

// Catalog tells what a trigger node type declares. The connector catalog
// implements it (SPEC-003); tests use a fake.
type Catalog interface {
	// LookupTrigger reports false for a node type that is not a trigger type.
	LookupTrigger(ctx context.Context, nodeType string) (Declaration, bool)
}

// Credentials tells which credentials a trigger node needs and lacks (rule
// T-V3). The credential store implements it.
type Credentials interface {
	// Missing returns the credentials the node refers to that the project
	// cannot use, as names fit to show a person; empty if none.
	Missing(ctx context.Context, org identity.OrgID, project identity.ProjectID, nodeType string, parameters jsonvalue.Value) ([]string, error)
}

// Settings are the instance settings the trigger rules read.
type Settings struct {
	// LocalFileEnabled turns on the local-file trigger. It is off by default
	// (R-LSN-07).
	LocalFileEnabled bool
	// LocalFileAllowedPaths lists the absolute directories a local-file
	// trigger may watch.
	LocalFileAllowedPaths []string
}

// RunMode is the mode of an execution started by a trigger (ADR-0009).
type RunMode string

// Execution modes of the triggers of this area.
const (
	// ModeManual is a person's run of the draft or of a published version.
	ModeManual RunMode = "MANUAL"
)

// ManualRun is a manual run ready to start: the revision to execute, in mode
// [ModeManual], without deduplication or cap (R-INT-01).
type ManualRun struct {
	Org    identity.OrgID
	FlowID flow.FlowID
	// Node is the start event the run enters through.
	Node flow.NodeID
	// Revision pins exactly one revision of the flow.
	Revision flow.Revision
	// Version is set for a run of a published version.
	Version flow.FlowVersionID
	// Snapshot is set for a run of the draft.
	Snapshot flow.FlowSnapshotID
	Mode     RunMode
}

// Starter starts an execution. The engine implements it; the durable engine
// arrives with the execution storage (SPEC-001 C2b and C4).
type Starter interface {
	// StartManual starts the run with the given input and returns the
	// identity of the execution.
	StartManual(ctx context.Context, run ManualRun, input []byte) (executionID string, err error)
}
