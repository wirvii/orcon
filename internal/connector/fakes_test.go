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

// fakeCore is a CoreTypes fake: orcon.http@1 takes its contract from the
// literal method (GET and HEAD read, anything else is an unsafe effect), and
// every other core type is pure. It records how it was asked.
type fakeCore struct {
	mu       sync.Mutex
	lookups  []string
	resolved []string
}

var _ connector.CoreTypes = (*fakeCore)(nil)

func (c *fakeCore) Lookup(_ context.Context, nodeType string) (flow.NodeType, bool) {
	c.mu.Lock()
	c.lookups = append(c.lookups, nodeType)
	c.mu.Unlock()
	switch {
	case nodeType == "orcon.http@1":
		return unsafeHTTP, true
	case strings.HasPrefix(nodeType, "orcon."):
		return flow.NodeType{Purity: flow.PurityPure}, true
	}
	return flow.NodeType{}, false
}

func (c *fakeCore) ResolveCore(_ context.Context, n *flow.Node) (flow.NodeType, bool) {
	c.mu.Lock()
	c.resolved = append(c.resolved, n.Type)
	c.mu.Unlock()
	switch {
	case n.Type == "orcon.http@1":
		if m, ok := n.Parameters.Get("method"); ok {
			if s, ok := m.Str(); ok && (s == "GET" || s == "HEAD") {
				return flow.NodeType{Purity: flow.PurityRead}, true
			}
		}
		return unsafeHTTP, true
	case strings.HasPrefix(n.Type, "orcon."):
		return flow.NodeType{Purity: flow.PurityPure}, true
	}
	return flow.NodeType{}, false
}

var unsafeHTTP = flow.NodeType{Purity: flow.PurityEffect, Contract: &flow.Contract{
	Effect: flow.EffectUnknown, Level: flow.SafetyUnsafe, OnUncertain: flow.OnUncertainSuspend,
}}
