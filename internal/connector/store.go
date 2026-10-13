// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// The ports below are what the connector area needs from durable storage
// (ADR-0029 section 8, architecture-layers.md AL-3). The adapters
// internal/adapter/sqlite and internal/adapter/postgres implement them and
// internal/adapter/storetest proves both behave the same.
//
// Row identities are typed values created by the caller as UUIDv7 (orcon-db
// ID-1). Documents travel as the bytes the registry produced; the store keeps
// them whole and never reads inside them (orcon-db JS-1, JS-5).

// Storage errors. They are the ones of the flow area, which the adapters map
// constraint violations to by name (orcon-db IN-8), so a caller can match them
// with either package.
var (
	// ErrNotFound means no row matches, or the row belongs to another organization.
	ErrNotFound = flow.ErrNotFound
	// ErrConflict means a uniqueness rule rejected the write.
	ErrConflict = flow.ErrConflict
	// ErrReference means a referenced row does not exist.
	ErrReference = flow.ErrReference
	// ErrInvalidRecord means a database constraint rejected the values.
	ErrInvalidRecord = flow.ErrInvalidRecord
)

// PackageRecord is a row of connector_package: one installed version of a
// connector. It is an instance record; every organization sees the same set,
// and approves it with a grant.
type PackageRecord struct {
	ID          PackageID
	ConnectorID string
	Version     string
	// Digest is "sha256:" and the hexadecimal SHA-256 of the package archive.
	Digest string
	Kind   Kind
	// Manifest is the canonical JSON text of the manifest.
	Manifest []byte
	Status   PackageStatus
	// InstalledAt is when the package was installed.
	InstalledAt time.Time
	// InstalledByPrincipalID is the zero value when the principal was deleted.
	InstalledByPrincipalID identity.PrincipalID
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// PackageStore persists installed packages.
type PackageStore interface {
	// InsertPackage stores a package. A second row for the same connector id and
	// version, or the same digest, fails with [ErrConflict].
	InsertPackage(ctx context.Context, p PackageRecord) error
	// GetPackage returns one version of a connector, or [ErrNotFound].
	GetPackage(ctx context.Context, connectorID, version string) (PackageRecord, error)
	// GetPackageByID returns a package by row identity, or [ErrNotFound].
	GetPackageByID(ctx context.Context, id PackageID) (PackageRecord, error)
	// ListPackages returns at most limit packages with an identity greater than
	// after, ordered by identity (keyset pagination).
	ListPackages(ctx context.Context, after PackageID, limit int) ([]PackageRecord, error)
	// ListPackageVersions returns at most limit versions of a connector whose
	// version text sorts after afterVersion, in that order. The order is the
	// text order of the store, not SemVer precedence; callers compare versions
	// with [CompareVersions].
	ListPackageVersions(ctx context.Context, connectorID, afterVersion string, limit int) ([]PackageRecord, error)
}

// GrantRecord is a row of connector_grant: an organization's approval of an
// installed package (ADR-0029 section 6), for the whole organization or one
// project.
type GrantRecord struct {
	ID    GrantID
	OrgID identity.OrgID
	// PackageID is the approved package version.
	PackageID PackageID
	// ProjectID is the zero value for a grant that covers the organization.
	ProjectID identity.ProjectID
	// PermissionsDigest is "sha256:" over the canonical permissions block that
	// was approved.
	PermissionsDigest string
	// GrantedByPrincipalID is the zero value when the principal was deleted.
	GrantedByPrincipalID identity.PrincipalID
	GrantedAt            time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// GrantStore persists grants. Every method takes the organization first: a
// grant of another organization is never visible (ADR-0046).
type GrantStore interface {
	// InsertGrant stores a grant. A second grant of the same package for the
	// same scope fails with [ErrConflict]; an unknown package or project with
	// [ErrReference].
	InsertGrant(ctx context.Context, g GrantRecord) error
	// FindGrant returns the grant that lets the project use the package: the
	// project's own grant or the organization-wide one. It fails with
	// [ErrNotFound] when neither exists.
	FindGrant(ctx context.Context, org identity.OrgID, pkg PackageID, project identity.ProjectID) (GrantRecord, error)
}
