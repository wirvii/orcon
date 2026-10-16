// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The ports below are what the flow area needs from durable storage
// (ADR-0015 section 1, architecture-layers.md AL-3). The adapters
// internal/adapter/sqlite and internal/adapter/postgres implement them and
// internal/adapter/storetest proves both behave the same.
//
// Row identities are typed values created by the caller as UUIDv7 (orcon-db
// ID-1); the adapters convert them to the stored form. Documents travel as the
// bytes the flow codec produced; the store keeps them whole and never reads
// inside them (orcon-db JS-1, JS-5). Every method of an organization-scoped
// record takes the [orgscope.Org] first after the context: a row of another
// organization is never visible, and a record that names another organization
// than its scope fails with [identity.ErrScopeMismatch] (ADR-0046).

// Errors the stores return. They are the errors of the identity area, shared so
// that one failure satisfies errors.Is for every area (orcon-db IN-8).
var (
	// ErrNotFound means no row matches, or the row belongs to another organization.
	ErrNotFound = identity.ErrNotFound
	// ErrConflict means a uniqueness rule rejected the write: a duplicate name,
	// slug, version number, content hash or identity.
	ErrConflict = identity.ErrConflict
	// ErrReference means a referenced row does not exist, or a row that is still
	// referenced cannot be removed.
	ErrReference = identity.ErrReference
	// ErrClosed means the store was closed; an operation that was in flight when
	// Close was called finishes first, one that starts after it fails with this.
	ErrClosed = identity.ErrClosed
	// ErrInvalidRecord means a database constraint rejected the values
	// (a CHECK: unknown enumeration value, empty name, wrong format tag).
	ErrInvalidRecord = identity.ErrInvalidRecord
)

// ActivationState says whether the automatic triggers of a flow accept events
// (ADR-0024 section 1). It is separate from publication: publishing creates a
// version, activating switches the triggers on.
type ActivationState string

// Activation states, the spelling the API exposes (ADR-0080 section 3).
const (
	// ActivationActive means the automatic triggers accept events.
	ActivationActive ActivationState = "ACTIVE"
	// ActivationInactive means no automatic trigger accepts events. It is the
	// state of every new flow.
	ActivationInactive ActivationState = "INACTIVE"
)

// Record is a row of flow. Soft delete columns are read here; the operations
// that delete, restore and purge a flow belong to the trigger area, because
// they switch the triggers off in the same transaction (ADR-0024 section 8).
type Record struct {
	ID            FlowID
	OrgID         identity.OrgID
	ProjectID     identity.ProjectID
	Name          string
	Draft         []byte
	DraftRevision int64
	// Revision is the revision of the flow row: it rises with every change of the row (name,
	// activation state, active version, deletion) but not with a draft save. It is the value of
	// the ETag of the flow (SPEC-005 Amendment 8 section 8-9).
	Revision int64
	// ActiveVersionID is the zero value while nothing is published.
	ActiveVersionID FlowVersionID
	// ActiveVersion is the number of the active version, or 0 while nothing is published. Only the
	// reads fill it; the writes ignore it.
	ActiveVersion int64
	// HasUnpublishedChanges says whether the draft differs from the active version. Only
	// [Manager.Get] fills it, from the content hashes; the stores leave it false.
	HasUnpublishedChanges bool
	// ActivationState is [ActivationInactive] until the flow is activated.
	// CreateFlow ignores it: a new flow is always inactive.
	ActivationState ActivationState
	// DeletedAt is the zero time while the flow is active.
	DeletedAt time.Time
	// DeletedByPrincipalID is the zero value unless a principal deleted the flow.
	DeletedByPrincipalID identity.PrincipalID
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// IsDeleted reports whether the flow is soft-deleted.
func (r Record) IsDeleted() bool { return !r.DeletedAt.IsZero() }

// Store persists flows. Reads never return a soft-deleted flow unless the
// method says so.
type Store interface { //nolint:interfacebloat // the flow port: the flows, their drafts and their activity are one area
	// CreateFlow inserts a flow with draft revision 0. A second non-deleted
	// flow with the same name in the project fails with [ErrConflict].
	CreateFlow(ctx context.Context, org orgscope.Org, f Record) error
	// CreateFlowWithEntry is [Store.CreateFlow] with the audit entry of the creation in the same
	// transaction (SPEC-004 R-AUD-02). When the context carries an idempotency claim, the record
	// of the claim is written in that transaction too (see internal/platform/idempotencyclaim).
	CreateFlowWithEntry(ctx context.Context, org orgscope.Org, f Record, entry audit.Entry) error
	// GetFlow returns a non-deleted flow, or [ErrNotFound].
	GetFlow(ctx context.Context, org orgscope.Org, flow FlowID) (Record, error)
	// GetFlowByName returns the non-deleted flow of the project with that name.
	GetFlowByName(ctx context.Context, org orgscope.Org, project identity.ProjectID, name string) (Record, error)
	// ListFlows returns at most limit non-deleted flows of the project with an
	// identity greater than after, ordered by identity.
	ListFlows(ctx context.Context, org orgscope.Org, project identity.ProjectID, after FlowID, limit int) ([]Record, error)
	// ListFlowActivity returns at most limit non-deleted flows of the project with an identity
	// greater than after, ordered by identity, each with its latest execution and the number of
	// executions accepted at or after since (SPEC-019 R-UX-BE-01). It is one read: no query per flow.
	ListFlowActivity(ctx context.Context, org orgscope.Org, project identity.ProjectID, since time.Time, after FlowID, limit int) ([]Activity, error)
	// CountFlows returns the number of non-deleted flows of the project.
	CountFlows(ctx context.Context, org orgscope.Org, project identity.ProjectID) (int64, error)
	// UpdateFlowDraft replaces the draft document when the stored revision still
	// equals expectedRevision, increments the revision and returns it. In the
	// same transaction it replaces the connector references recorded for the
	// draft with references (nil clears them), so a retirement never reads
	// inside a document (ADR-0034 section 6). A different stored revision fails
	// with [ErrDraftConflict] and changes nothing.
	UpdateFlowDraft(ctx context.Context, org orgscope.Org, flow FlowID, draft []byte, expectedRevision int64, references []ConnectorReference, now time.Time) (int64, error)
}

// VersionRecord is a row of flow_version: an immutable published version.
type VersionRecord struct {
	ID          FlowVersionID
	OrgID       identity.OrgID
	FlowID      FlowID
	Number      int64
	Document    []byte
	ContentHash string
	// Warnings is nil when publication produced none.
	Warnings []byte
	// ConnectorReferences are the connector versions the document pins; they
	// are stored with the version in the same transaction.
	ConnectorReferences    []ConnectorReference
	PublishedByPrincipalID identity.PrincipalID
	PublishedAt            time.Time
	CreatedAt              time.Time
	// Note is the release note of the publication, empty when there is none (SPEC-014 A7).
	Note string
	// RolledBackFromVersion is the number of the version whose content a rollback published
	// again, or 0 for a publication of the draft.
	RolledBackFromVersion int64
}

// VersionStore persists published versions.
type VersionStore interface {
	// PublishVersion inserts the version, records its connector references and
	// makes it the flow's active version in one transaction. A repeated version
	// number fails with [ErrConflict].
	PublishVersion(ctx context.Context, org orgscope.Org, v VersionRecord) error
	// GetVersion returns a version by identity, or [ErrVersionNotFound].
	GetVersion(ctx context.Context, org orgscope.Org, version FlowVersionID) (VersionRecord, error)
	// GetVersionByNumber returns a version of the flow by its number, or
	// [ErrVersionNotFound].
	GetVersionByNumber(ctx context.Context, org orgscope.Org, flow FlowID, number int64) (VersionRecord, error)
	// ListVersions returns at most limit versions of the flow with a number
	// greater than after, ordered by number.
	ListVersions(ctx context.Context, org orgscope.Org, flow FlowID, after int64, limit int) ([]VersionRecord, error)
}

// ConnectorReferenceRecord is a row of connector_reference as it is read.
type ConnectorReferenceRecord struct {
	ConnectorReference
	OrgID  identity.OrgID
	FlowID FlowID
	// FlowVersionID is the zero value for a reference of the draft.
	FlowVersionID FlowVersionID
	CreatedAt     time.Time
}

// ReferenceStore reads the connector references recorded with drafts and
// published versions.
type ReferenceStore interface {
	// ListDraftConnectorReferences returns at most limit references of the
	// flow's draft with an identity greater than after, ordered by identity.
	ListDraftConnectorReferences(ctx context.Context, org orgscope.Org, flow FlowID, after ConnectorReferenceID, limit int) ([]ConnectorReferenceRecord, error)
	// ListVersionConnectorReferences returns at most limit references of a
	// published version with an identity greater than after, ordered by identity.
	ListVersionConnectorReferences(ctx context.Context, org orgscope.Org, version FlowVersionID, after ConnectorReferenceID, limit int) ([]ConnectorReferenceRecord, error)
}

// SnapshotRecord is a row of flow_snapshot: the content-addressed document of
// a test run.
type SnapshotRecord struct {
	ID          FlowSnapshotID
	OrgID       identity.OrgID
	FlowID      FlowID
	ContentHash string
	Document    []byte
	CreatedAt   time.Time
}

// SnapshotStore persists snapshots.
type SnapshotStore interface {
	// CreateSnapshot inserts the snapshot, or returns the stored one when the
	// organization already has a snapshot with the same content hash.
	CreateSnapshot(ctx context.Context, org orgscope.Org, s SnapshotRecord) (SnapshotRecord, error)
	// GetSnapshot returns a snapshot by identity, or [ErrSnapshotNotFound].
	GetSnapshot(ctx context.Context, org orgscope.Org, snapshot FlowSnapshotID) (SnapshotRecord, error)
}

// DraftStore is what the draft service needs of the storage: the flow and the atomic save of a
// draft with its audit entry.
type DraftStore interface {
	// GetFlow returns a non-deleted flow, or [ErrNotFound].
	GetFlow(ctx context.Context, org orgscope.Org, flow FlowID) (Record, error)
	// UpdateFlowDraftWithEntry is [Store.UpdateFlowDraft] with the audit entry of the change in the
	// same transaction (SPEC-004 R-AUD-02): a conflict or a missing flow writes neither.
	UpdateFlowDraftWithEntry(ctx context.Context, org orgscope.Org, flow FlowID, draft []byte, expectedRevision int64, references []ConnectorReference, now time.Time, entry audit.Entry) (int64, error)
}

// ManageStore is what the flow management service needs of the storage: the flow and the atomic
// change of its row with its audit entry.
type ManageStore interface {
	// GetFlow returns a non-deleted flow, or [ErrNotFound].
	GetFlow(ctx context.Context, org orgscope.Org, flow FlowID) (Record, error)
	// GetVersion returns a version by identity, or [ErrVersionNotFound].
	GetVersion(ctx context.Context, org orgscope.Org, version FlowVersionID) (VersionRecord, error)
	// UpdateFlowNameWithEntry renames the flow when its revision still equals expectedRevision,
	// increments the revision and writes the audit entry in the same transaction. A different
	// revision fails with [ErrRevisionConflict], a missing flow with [ErrNotFound] and a name the
	// project already uses with [ErrConflict]; none of them writes anything.
	UpdateFlowNameWithEntry(ctx context.Context, org orgscope.Org, flow FlowID, name string, expectedRevision int64, now time.Time, entry audit.Entry) error
}
