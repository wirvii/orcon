// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector

import (
	"context"
	"errors"
	"fmt"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// ErrDigestConflict means a version of the connector is already installed with
// different content. A published version never changes (ADR-0034 section 5): the
// publisher raises the version.
var ErrDigestConflict = errors.New("connector version already installed with a different digest")

// pageSize is the page the registry reads when it walks a list.
const pageSize = 200

// Policy decides whether an installed connector may be used. It is the hook
// where the trust and signing rules of ADR-0033 plug in (cut CN5); until then
// [OpenPolicy] allows everything.
type Policy interface {
	// Check returns nil when the connector version may be used, or an error whose
	// text says why it may not; level 5 shows the text to the user.
	Check(ctx context.Context, m *Manifest) error
}

// OpenPolicy allows every installed connector.
type OpenPolicy struct{}

// Check implements [Policy].
func (OpenPolicy) Check(context.Context, *Manifest) error { return nil }

// Registry installs connector packages and answers what is installed. It does
// not verify signatures: an installed package carries no seal yet.
type Registry struct {
	packages PackageStore
	grants   GrantStore
	policy   Policy
	clock    clock.Clock
	ids      id.Generator
}

// NewRegistry builds a registry over the stores. The clock and identity
// generator are injected so installation is deterministic in tests.
func NewRegistry(packages PackageStore, grants GrantStore, policy Policy, clk clock.Clock, ids id.Generator) *Registry {
	return &Registry{packages: packages, grants: grants, policy: policy, clock: clk, ids: ids}
}

// Install validates a package archive and records it as an installed package.
// Installing the same archive again returns the existing record; a different
// archive for an installed version fails with [ErrDigestConflict]. An archive
// never uses the reserved "orcon." namespace, because official connectors are
// built into the binary.
func (r *Registry) Install(ctx context.Context, archive []byte, by identity.PrincipalID) (PackageRecord, error) {
	pkg, err := OpenPackage(archive, Options{})
	if err != nil {
		return PackageRecord{}, fmt.Errorf("install connector package: %w", err)
	}
	return r.record(ctx, pkg.Manifest(), pkg.Digest(), by)
}

// RegisterBuiltin records a connector built into the binary, which has no
// archive: its manifest is read with the reserved namespace allowed and its
// digest is the digest of the canonical manifest. It is how the composition
// root makes official connectors visible to flows.
func (r *Registry) RegisterBuiltin(ctx context.Context, manifest []byte) (PackageRecord, error) {
	m, err := Parse(manifest, Options{Official: true})
	if err != nil {
		return PackageRecord{}, fmt.Errorf("register built-in connector: %w", err)
	}
	canonical, err := jsonvalue.Canonical(m.Value())
	if err != nil {
		return PackageRecord{}, fmt.Errorf("register built-in connector %s: %w", m.ID, err)
	}
	return r.record(ctx, &m, DigestOf(canonical), identity.PrincipalID{})
}

func (r *Registry) record(ctx context.Context, m *Manifest, digest Digest, by identity.PrincipalID) (PackageRecord, error) {
	existing, err := r.packages.GetPackage(ctx, m.ID, m.Version)
	switch {
	case err == nil:
		if existing.Digest != string(digest) {
			return PackageRecord{}, fmt.Errorf("install %s %s: %w", m.ID, m.Version, ErrDigestConflict)
		}
		return existing, nil
	case !errors.Is(err, ErrNotFound):
		return PackageRecord{}, fmt.Errorf("install %s %s: %w", m.ID, m.Version, err)
	}
	canonical, err := jsonvalue.Canonical(m.Value())
	if err != nil {
		return PackageRecord{}, fmt.Errorf("install %s %s: canonicalize manifest: %w", m.ID, m.Version, err)
	}
	now := clock.Micros(r.clock.Now())
	rec := PackageRecord{
		ID: PackageID(r.ids.New()), ConnectorID: m.ID, Version: m.Version, Digest: string(digest), Kind: m.Kind,
		Manifest: canonical, Status: StatusInstalled, InstalledAt: now, InstalledByPrincipalID: by, CreatedAt: now, UpdatedAt: now,
	}
	if err := r.packages.InsertPackage(ctx, rec); err != nil {
		return PackageRecord{}, fmt.Errorf("install %s %s: %w", m.ID, m.Version, err)
	}
	return rec, nil
}

// Grant records an organization's approval of an installed package for the
// whole organization, or for one project when project is not the zero value.
// The permissions that were approved are recorded by digest.
func (r *Registry) Grant(ctx context.Context, org identity.OrgID, project identity.ProjectID, pkg PackageRecord, by identity.PrincipalID) (GrantRecord, error) {
	m, err := Parse(pkg.Manifest, Options{Official: true})
	if err != nil {
		return GrantRecord{}, fmt.Errorf("grant %s %s: stored manifest: %w", pkg.ConnectorID, pkg.Version, err)
	}
	digest, err := m.PermissionsDigest()
	if err != nil {
		return GrantRecord{}, fmt.Errorf("grant %s %s: %w", pkg.ConnectorID, pkg.Version, err)
	}
	now := clock.Micros(r.clock.Now())
	g := GrantRecord{
		ID: GrantID(r.ids.New()), OrgID: org, PackageID: pkg.ID, ProjectID: project, PermissionsDigest: digest,
		GrantedByPrincipalID: by, GrantedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := r.grants.InsertGrant(ctx, g); err != nil {
		return GrantRecord{}, fmt.Errorf("grant %s %s: %w", pkg.ConnectorID, pkg.Version, err)
	}
	return g, nil
}

// Catalog returns the node-type catalog for flows of one project. Validation
// level 5 uses it: a connector node is valid when its pinned version is
// installed, granted to the organization or project, and allowed by the policy.
func (r *Registry) Catalog(org identity.OrgID, project identity.ProjectID) *Catalog {
	return &Catalog{registry: r, org: org, project: project}
}

// latest returns the newest installed, not retired version of a connector that
// has the node type, by SemVer precedence.
func (r *Registry) latest(ctx context.Context, connectorID, nodeType string) (PackageRecord, Manifest, bool, error) {
	var best PackageRecord
	var bestManifest Manifest
	found := false
	after := ""
	for {
		page, err := r.packages.ListPackageVersions(ctx, connectorID, after, pageSize)
		if err != nil {
			return PackageRecord{}, Manifest{}, false, fmt.Errorf("list versions of %s: %w", connectorID, err)
		}
		for i := range page {
			rec := &page[i]
			if rec.Status != StatusInstalled || (found && CompareVersions(rec.Version, best.Version) <= 0) {
				continue
			}
			m, err := Parse(rec.Manifest, Options{Official: true})
			if err != nil {
				return PackageRecord{}, Manifest{}, false, fmt.Errorf("stored manifest of %s %s: %w", rec.ConnectorID, rec.Version, err)
			}
			if _, has := m.NodeByType(nodeType); has {
				best, bestManifest, found = *rec, m, true
			}
		}
		if len(page) < pageSize {
			return best, bestManifest, found, nil
		}
		after = page[len(page)-1].Version
	}
}
