// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// The flow ports of [Memory]. Each call takes the lock of the store, so a call
// never sees half a transaction of [Memory.Atomic].

var _ Stores = (*Memory)(nil)

// CreateOrganization implements [flow.InstanceStore].
func (m *Memory) CreateOrganization(_ context.Context, o flow.OrgRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.s.orgs {
		if e.Slug == o.Slug {
			return fmt.Errorf("organization slug %q: %w", o.Slug, flow.ErrConflict)
		}
	}
	m.s.orgs[o.ID] = &o
	return nil
}

// GetOrganization implements [flow.InstanceStore].
func (m *Memory) GetOrganization(_ context.Context, org identity.OrgID) (flow.OrgRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.s.orgs[org]
	if !ok {
		return flow.OrgRecord{}, fmt.Errorf("organization %s: %w", org, flow.ErrNotFound)
	}
	return *o, nil
}

// CreatePrincipal implements [flow.InstanceStore].
func (m *Memory) CreatePrincipal(_ context.Context, p flow.PrincipalRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.s.principals[p.ID] = &p
	return nil
}

// GetSystemPrincipal implements [flow.InstanceStore].
func (m *Memory) GetSystemPrincipal(context.Context) (flow.PrincipalRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.s.principals {
		if p.Kind == flow.PrincipalSystem {
			return *p, nil
		}
	}
	return flow.PrincipalRecord{}, flow.ErrNotFound
}

// CreateProject implements [flow.ProjectStore].
func (m *Memory) CreateProject(_ context.Context, p flow.ProjectRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.s.orgs[p.OrgID]; !ok {
		return fmt.Errorf("organization %s: %w", p.OrgID, flow.ErrReference)
	}
	for _, e := range m.s.projects {
		if e.OrgID == p.OrgID && e.Slug == p.Slug {
			return fmt.Errorf("project slug %q: %w", p.Slug, flow.ErrConflict)
		}
	}
	m.s.projects[p.ID] = &p
	return nil
}

// GetProject implements [flow.ProjectStore] and [trigger.FlowSource].
func (m *Memory) GetProject(_ context.Context, org identity.OrgID, project identity.ProjectID) (flow.ProjectRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.s.projects[project]
	if !ok || p.OrgID != org {
		return flow.ProjectRecord{}, fmt.Errorf("project %s: %w", project, flow.ErrNotFound)
	}
	return *p, nil
}

// ListProjects implements [flow.ProjectStore].
func (m *Memory) ListProjects(_ context.Context, org identity.OrgID, after identity.ProjectID, limit int) ([]flow.ProjectRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []flow.ProjectRecord
	for _, p := range m.s.projects {
		if p.OrgID == org && string(p.ID[:]) > string(after[:]) {
			out = append(out, *p)
		}
	}
	slices.SortFunc(out, func(a, b flow.ProjectRecord) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// CreateFlow implements [flow.Store].
func (m *Memory) CreateFlow(_ context.Context, f flow.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.s.projects[f.ProjectID]; !ok || p.OrgID != f.OrgID {
		return fmt.Errorf("project %s: %w", f.ProjectID, flow.ErrReference)
	}
	for _, e := range m.s.flows {
		if e.OrgID == f.OrgID && e.ProjectID == f.ProjectID && e.Name == f.Name && !e.IsDeleted() {
			return fmt.Errorf("flow name %q: %w", f.Name, flow.ErrConflict)
		}
	}
	f.DraftRevision = 0
	f.ActiveVersionID = flow.FlowVersionID{}
	f.ActivationState = flow.ActivationInactive
	f.DeletedAt, f.DeletedByPrincipalID = time.Time{}, identity.PrincipalID{}
	f.Draft = bytes.Clone(f.Draft)
	m.s.flows[f.ID] = &f
	return nil
}

// GetFlow implements [flow.Store] and [trigger.FlowSource].
func (m *Memory) GetFlow(_ context.Context, org identity.OrgID, id flow.FlowID) (flow.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.s.flows[id]
	if !ok || f.OrgID != org || f.IsDeleted() {
		return flow.Record{}, fmt.Errorf("flow %s: %w", id, flow.ErrNotFound)
	}
	out := *f
	out.Draft = bytes.Clone(f.Draft)
	return out, nil
}

// GetFlowByName implements [flow.Store].
func (m *Memory) GetFlowByName(_ context.Context, org identity.OrgID, project identity.ProjectID, name string) (flow.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, f := range m.s.flows {
		if f.OrgID == org && f.ProjectID == project && f.Name == name && !f.IsDeleted() {
			return *f, nil
		}
	}
	return flow.Record{}, fmt.Errorf("flow %q: %w", name, flow.ErrNotFound)
}

// ListFlows implements [flow.Store].
func (m *Memory) ListFlows(_ context.Context, org identity.OrgID, project identity.ProjectID, after flow.FlowID, limit int) ([]flow.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []flow.Record
	for _, f := range m.s.flows {
		if f.OrgID == org && f.ProjectID == project && !f.IsDeleted() && string(f.ID[:]) > string(after[:]) {
			out = append(out, *f)
		}
	}
	slices.SortFunc(out, func(a, b flow.Record) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// UpdateFlowDraft implements [flow.Store].
func (m *Memory) UpdateFlowDraft(_ context.Context, org identity.OrgID, id flow.FlowID, draft []byte, expected int64, _ []flow.ConnectorReference, now time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.s.flows[id]
	if !ok || f.OrgID != org || f.IsDeleted() {
		return 0, fmt.Errorf("flow %s: %w", id, flow.ErrNotFound)
	}
	if f.DraftRevision != expected {
		return 0, flow.ErrDraftConflict
	}
	f.Draft, f.DraftRevision, f.UpdatedAt = bytes.Clone(draft), expected+1, now
	return f.DraftRevision, nil
}

// PublishVersion implements [flow.VersionStore].
func (m *Memory) PublishVersion(_ context.Context, v flow.VersionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.s.flows[v.FlowID]
	if !ok || f.OrgID != v.OrgID || f.IsDeleted() {
		return fmt.Errorf("flow %s: %w", v.FlowID, flow.ErrNotFound)
	}
	for _, e := range m.s.versions {
		if e.FlowID == v.FlowID && e.Number == v.Number {
			return fmt.Errorf("version %d: %w", v.Number, flow.ErrConflict)
		}
	}
	m.s.versions[v.ID] = &v
	f.ActiveVersionID, f.UpdatedAt = v.ID, v.PublishedAt
	return nil
}

// GetVersion implements [flow.VersionStore] and [trigger.FlowSource].
func (m *Memory) GetVersion(_ context.Context, org identity.OrgID, id flow.FlowVersionID) (flow.VersionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.s.versions[id]
	if !ok || v.OrgID != org {
		return flow.VersionRecord{}, fmt.Errorf("version %s: %w", id, flow.ErrVersionNotFound)
	}
	return *v, nil
}

// GetVersionByNumber implements [flow.VersionStore] and [trigger.FlowSource].
func (m *Memory) GetVersionByNumber(_ context.Context, org identity.OrgID, id flow.FlowID, number int64) (flow.VersionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.s.versions {
		if v.OrgID == org && v.FlowID == id && v.Number == number {
			return *v, nil
		}
	}
	return flow.VersionRecord{}, fmt.Errorf("version %d of flow %s: %w", number, id, flow.ErrVersionNotFound)
}

// ListVersions implements [flow.VersionStore].
func (m *Memory) ListVersions(_ context.Context, org identity.OrgID, id flow.FlowID, after int64, limit int) ([]flow.VersionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []flow.VersionRecord
	for _, v := range m.s.versions {
		if v.OrgID == org && v.FlowID == id && v.Number > after {
			out = append(out, *v)
		}
	}
	slices.SortFunc(out, func(a, b flow.VersionRecord) int { return int(a.Number - b.Number) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// CreateSnapshot implements [flow.SnapshotStore] and [trigger.FlowSource].
func (m *Memory) CreateSnapshot(_ context.Context, s flow.SnapshotRecord) (flow.SnapshotRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.s.snapshots {
		if e.OrgID == s.OrgID && e.ContentHash == s.ContentHash {
			return *e, nil
		}
	}
	m.s.snapshots[s.ID] = &s
	return s, nil
}

// GetSnapshot implements [flow.SnapshotStore].
func (m *Memory) GetSnapshot(_ context.Context, org identity.OrgID, id flow.FlowSnapshotID) (flow.SnapshotRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.s.snapshots[id]
	if !ok || s.OrgID != org {
		return flow.SnapshotRecord{}, fmt.Errorf("snapshot %s: %w", id, flow.ErrSnapshotNotFound)
	}
	return *s, nil
}
