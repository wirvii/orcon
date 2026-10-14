// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package connector_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/wirvii/orcon/internal/connector"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// memStore is an in-memory implementation of the connector stores. It follows
// the contract the real adapters satisfy (the storetest suite proves those).
type memStore struct {
	mu       sync.Mutex
	packages []connector.PackageRecord
	grants   []connector.GrantRecord
}

func (s *memStore) InsertPackage(_ context.Context, p connector.PackageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.packages {
		q := &s.packages[i]
		if q.ID == p.ID || q.Digest == p.Digest || (q.ConnectorID == p.ConnectorID && q.Version == p.Version) {
			return fmt.Errorf("insert package: %w", connector.ErrConflict)
		}
	}
	s.packages = append(s.packages, p)
	return nil
}

func (s *memStore) GetPackage(_ context.Context, connectorID, version string) (connector.PackageRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.packages {
		if p := &s.packages[i]; p.ConnectorID == connectorID && p.Version == version {
			return *p, nil
		}
	}
	return connector.PackageRecord{}, fmt.Errorf("get package %s %s: %w", connectorID, version, connector.ErrNotFound)
}

func (s *memStore) GetPackageByID(_ context.Context, id connector.PackageID) (connector.PackageRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.packages {
		if p := &s.packages[i]; p.ID == id {
			return *p, nil
		}
	}
	return connector.PackageRecord{}, fmt.Errorf("get package: %w", connector.ErrNotFound)
}

func (s *memStore) ListPackages(_ context.Context, after connector.PackageID, limit int) ([]connector.PackageRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []connector.PackageRecord
	for i := range s.packages {
		if p := &s.packages[i]; strings.Compare(p.ID.String(), after.String()) > 0 {
			out = append(out, *p)
		}
	}
	slices.SortFunc(out, func(a, b connector.PackageRecord) int { return strings.Compare(a.ID.String(), b.ID.String()) })
	return out[:min(limit, len(out))], nil
}

func (s *memStore) ListPackageVersions(_ context.Context, connectorID, afterVersion string, limit int) ([]connector.PackageRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []connector.PackageRecord
	for i := range s.packages {
		if p := &s.packages[i]; p.ConnectorID == connectorID && p.Version > afterVersion {
			out = append(out, *p)
		}
	}
	slices.SortFunc(out, func(a, b connector.PackageRecord) int { return strings.Compare(a.Version, b.Version) })
	return out[:min(limit, len(out))], nil
}

func (s *memStore) InsertGrant(_ context.Context, g connector.GrantRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.grants {
		h := &s.grants[i]
		if h.OrgID == g.OrgID && h.PackageID == g.PackageID && h.ProjectID == g.ProjectID {
			return fmt.Errorf("insert grant: %w", connector.ErrConflict)
		}
	}
	s.grants = append(s.grants, g)
	return nil
}

func (s *memStore) FindGrant(_ context.Context, org identity.OrgID, pkg connector.PackageID, project identity.ProjectID) (connector.GrantRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.grants {
		if g := &s.grants[i]; g.OrgID == org && g.PackageID == pkg && (g.ProjectID == (identity.ProjectID{}) || g.ProjectID == project) {
			return *g, nil
		}
	}
	return connector.GrantRecord{}, fmt.Errorf("find grant: %w", connector.ErrNotFound)
}

// blockingPolicy refuses the connector versions it was told to refuse.
type blockingPolicy struct {
	mu      sync.Mutex
	blocked map[string]string
}

func (p *blockingPolicy) block(connectorID, version, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.blocked == nil {
		p.blocked = map[string]string{}
	}
	p.blocked[connectorID+"@"+version] = reason
}

func (p *blockingPolicy) Check(_ context.Context, m *connector.Manifest) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if reason, ok := p.blocked[m.ID+"@"+m.Version]; ok {
		return errors.New(reason)
	}
	return nil
}

// coreTypes answers for the node types the engine owns, which the connector
// catalog does not know, and delegates the rest.
type coreTypes struct{ *connector.Catalog }

func (c coreTypes) Lookup(ctx context.Context, nodeType string) (flow.NodeType, bool) {
	if strings.HasPrefix(nodeType, "orcon.") {
		return flow.NodeType{Purity: flow.PurityPure}, true
	}
	return c.Catalog.Lookup(ctx, nodeType)
}

func (c coreTypes) ResolveNode(ctx context.Context, nodeType string, pin *flow.ConnectorPin, operation string) flow.NodeResolution {
	if strings.HasPrefix(nodeType, "orcon.") {
		return flow.NodeResolution{Type: flow.NodeType{Purity: flow.PurityPure}, Known: true}
	}
	return c.Catalog.ResolveNode(ctx, nodeType, pin, operation)
}
