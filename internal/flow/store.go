// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"errors"
	"time"

	"github.com/wirvii/orcon/internal/platform/id"
)

// The ports below are what the flow area needs from durable storage
// (ADR-0015 section 1, architecture-layers.md AL-3). The adapters
// internal/adapter/sqlite and internal/adapter/postgres implement them and
// internal/adapter/storetest proves both behave the same.
//
// Row identities are [id.UUID] values created by the caller (orcon-db ID-1).
// Documents travel as the bytes the flow codec produced; the store keeps them
// whole and never reads inside them (orcon-db JS-1, JS-5). Every method of an
// organization-scoped record takes the organization first: a row of another
// organization is never visible (ADR-0046).

// Errors the stores return. Adapters map constraint violations to them by
// constraint name (orcon-db IN-8).
var (
	// ErrNotFound means no row matches, or the row belongs to another organization.
	ErrNotFound = errors.New("not found")
	// ErrConflict means a uniqueness rule rejected the write: a duplicate name,
	// slug, version number, content hash or identity.
	ErrConflict = errors.New("conflict")
	// ErrReference means a referenced row does not exist, or a row that is still
	// referenced cannot be removed.
	ErrReference = errors.New("referenced row missing or still in use")
	// ErrInvalidRecord means a database constraint rejected the values
	// (a CHECK: unknown enumeration value, empty name, wrong format tag).
	ErrInvalidRecord = errors.New("record violates a database constraint")
)

// ProjectRecord is a row of project.
type ProjectRecord struct {
	ID          id.UUID
	OrgID       id.UUID
	Name        string
	Slug        string
	Environment string
	IsProtected bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProjectStore persists projects.
type ProjectStore interface {
	// CreateProject inserts a project. A second project with the same slug in
	// the organization fails with [ErrConflict]; an unknown organization with
	// [ErrReference].
	CreateProject(ctx context.Context, p ProjectRecord) error
	// GetProject returns one project of the organization, or [ErrNotFound].
	GetProject(ctx context.Context, org, project id.UUID) (ProjectRecord, error)
	// ListProjects returns at most limit projects of the organization with an
	// identity greater than after, ordered by identity (keyset pagination).
	ListProjects(ctx context.Context, org, after id.UUID, limit int) ([]ProjectRecord, error)
}

// Record is a row of flow. Soft delete columns are read here; the delete,
// restore and purge operations belong to the retention cut (ADR-0077 section 4).
type Record struct {
	ID            id.UUID
	OrgID         id.UUID
	ProjectID     id.UUID
	Name          string
	Draft         []byte
	DraftRevision int64
	// ActiveVersionID is the zero UUID while nothing is published.
	ActiveVersionID id.UUID
	// DeletedAt is the zero time while the flow is active.
	DeletedAt time.Time
	// DeletedByPrincipalID is the zero UUID unless a principal deleted the flow.
	DeletedByPrincipalID id.UUID
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// IsDeleted reports whether the flow is soft-deleted.
func (r Record) IsDeleted() bool { return !r.DeletedAt.IsZero() }

// Store persists flows. Reads never return a soft-deleted flow unless the
// method says so.
type Store interface {
	// CreateFlow inserts a flow with draft revision 0. A second non-deleted
	// flow with the same name in the project fails with [ErrConflict].
	CreateFlow(ctx context.Context, f Record) error
	// GetFlow returns a non-deleted flow, or [ErrNotFound].
	GetFlow(ctx context.Context, org, flow id.UUID) (Record, error)
	// GetFlowByName returns the non-deleted flow of the project with that name.
	GetFlowByName(ctx context.Context, org, project id.UUID, name string) (Record, error)
	// ListFlows returns at most limit non-deleted flows of the project with an
	// identity greater than after, ordered by identity.
	ListFlows(ctx context.Context, org, project, after id.UUID, limit int) ([]Record, error)
	// UpdateFlowDraft replaces the draft document when the stored revision still
	// equals expectedRevision, increments the revision and returns it. A
	// different stored revision fails with [ErrDraftConflict] and changes
	// nothing.
	UpdateFlowDraft(ctx context.Context, org, flow id.UUID, draft []byte, expectedRevision int64, now time.Time) (int64, error)
}

// VersionRecord is a row of flow_version: an immutable published version.
type VersionRecord struct {
	ID          id.UUID
	OrgID       id.UUID
	FlowID      id.UUID
	Number      int64
	Document    []byte
	ContentHash string
	// Warnings is nil when publication produced none.
	Warnings               []byte
	PublishedByPrincipalID id.UUID
	PublishedAt            time.Time
	CreatedAt              time.Time
}

// VersionStore persists published versions.
type VersionStore interface {
	// PublishVersion inserts the version and makes it the flow's active version
	// in one transaction. A repeated version number fails with [ErrConflict].
	PublishVersion(ctx context.Context, v VersionRecord) error
	// GetVersion returns a version by identity, or [ErrVersionNotFound].
	GetVersion(ctx context.Context, org, version id.UUID) (VersionRecord, error)
	// GetVersionByNumber returns a version of the flow by its number, or
	// [ErrVersionNotFound].
	GetVersionByNumber(ctx context.Context, org, flow id.UUID, number int64) (VersionRecord, error)
	// ListVersions returns at most limit versions of the flow with a number
	// greater than after, ordered by number.
	ListVersions(ctx context.Context, org, flow id.UUID, after int64, limit int) ([]VersionRecord, error)
}

// SnapshotRecord is a row of flow_snapshot: the content-addressed document of
// a test run.
type SnapshotRecord struct {
	ID          id.UUID
	OrgID       id.UUID
	FlowID      id.UUID
	ContentHash string
	Document    []byte
	CreatedAt   time.Time
}

// SnapshotStore persists snapshots.
type SnapshotStore interface {
	// CreateSnapshot inserts the snapshot, or returns the stored one when the
	// organization already has a snapshot with the same content hash.
	CreateSnapshot(ctx context.Context, s SnapshotRecord) (SnapshotRecord, error)
	// GetSnapshot returns a snapshot by identity, or [ErrSnapshotNotFound].
	GetSnapshot(ctx context.Context, org, snapshot id.UUID) (SnapshotRecord, error)
}

// OrgRecord is a row of org.
type OrgRecord struct {
	ID        id.UUID
	Name      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Organization statuses.
const (
	OrgActive    = "ACTIVE"
	OrgSuspended = "SUSPENDED"
	OrgDeleting  = "DELETING"
)

// PrincipalRecord is a row of principal: whoever acts.
type PrincipalRecord struct {
	ID        id.UUID
	Kind      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Principal kinds, the spelling the API exposes.
const (
	PrincipalUser           = "USER"
	PrincipalServiceAccount = "SERVICE_ACCOUNT"
	PrincipalSystem         = "SYSTEM"
)

// WorkerRecord is a row of worker: an informational registry of processes.
type WorkerRecord struct {
	ID          id.UUID
	Role        string
	Host        string
	Version     string
	StartedAt   time.Time
	HeartbeatAt time.Time
}

// InstanceStore persists the instance tables an installation bootstraps with.
// The identity area (SPEC-004) takes organizations and principals over; this
// port is their interim home so the storage layer and its tests have a way to
// create them.
type InstanceStore interface {
	// CreateOrganization inserts an organization.
	CreateOrganization(ctx context.Context, o OrgRecord) error
	// GetOrganization returns one organization, or [ErrNotFound].
	GetOrganization(ctx context.Context, org id.UUID) (OrgRecord, error)
	// CreatePrincipal inserts a principal. A second SYSTEM principal fails with
	// [ErrConflict].
	CreatePrincipal(ctx context.Context, p PrincipalRecord) error
	// GetSystemPrincipal returns the single SYSTEM principal, or [ErrNotFound]
	// before the instance is bootstrapped.
	GetSystemPrincipal(ctx context.Context) (PrincipalRecord, error)
	// RegisterWorker inserts the worker or refreshes its heartbeat and version.
	RegisterWorker(ctx context.Context, w WorkerRecord) error
}
