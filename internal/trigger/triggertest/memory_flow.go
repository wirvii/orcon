// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The flow ports of [Memory]. Each call takes the lock of the store, so a call
// never sees half a transaction of [Memory.Atomic].

var _ Stores = (*Memory)(nil)

// CreateOrganization implements [identity.OrgStore].
func (m *Memory) CreateOrganization(_ context.Context, o identity.OrgRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.s.orgs {
		if e.Slug == o.Slug {
			return fmt.Errorf("organization slug %q: %w", o.Slug, identity.ErrConflict)
		}
	}
	m.s.orgs[o.ID] = &o
	return nil
}

// GetOrganization implements [identity.OrgStore].
func (m *Memory) GetOrganization(_ context.Context, org orgscope.Org) (identity.OrgRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.s.orgs[identity.OrgID(org.UUID())]
	if !ok || org.IsZero() {
		return identity.OrgRecord{}, fmt.Errorf("organization %s: %w", org, identity.ErrNotFound)
	}
	return *o, nil
}

// CreatePrincipal implements [identity.PrincipalStore].
func (m *Memory) CreatePrincipal(_ context.Context, p identity.PrincipalRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.s.principals[p.ID] = &p
	return nil
}

// GetSystemPrincipal implements [identity.PrincipalStore].
func (m *Memory) GetSystemPrincipal(context.Context) (identity.PrincipalRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.s.principals {
		if p.Kind == identity.PrincipalSystem {
			return *p, nil
		}
	}
	return identity.PrincipalRecord{}, identity.ErrNotFound
}

// CreateProject implements [identity.ProjectStore].
func (m *Memory) CreateProject(_ context.Context, org orgscope.Org, p identity.ProjectRecord, _ audit.Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !p.OrgID.Matches(org) {
		return fmt.Errorf("project of organization %s: %w", p.OrgID, identity.ErrScopeMismatch)
	}
	if _, ok := m.s.orgs[p.OrgID]; !ok {
		return fmt.Errorf("organization %s: %w", p.OrgID, identity.ErrReference)
	}
	for _, e := range m.s.projects {
		if e.OrgID == p.OrgID && e.Slug == p.Slug {
			return fmt.Errorf("project slug %q: %w", p.Slug, identity.ErrConflict)
		}
	}
	m.s.projects[p.ID] = &p
	return nil
}

// GetProject implements [identity.ProjectStore] and [trigger.FlowSource].
func (m *Memory) GetProject(_ context.Context, org orgscope.Org, project identity.ProjectID) (identity.ProjectRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.s.projects[project]
	if !ok || !p.OrgID.Matches(org) {
		return identity.ProjectRecord{}, fmt.Errorf("project %s: %w", project, identity.ErrNotFound)
	}
	return *p, nil
}

// ListProjects implements [identity.ProjectStore].
func (m *Memory) ListProjects(_ context.Context, org orgscope.Org, after identity.ProjectID, limit int) ([]identity.ProjectRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []identity.ProjectRecord
	for _, p := range m.s.projects {
		if p.OrgID.Matches(org) && string(p.ID[:]) > string(after[:]) {
			out = append(out, *p)
		}
	}
	slices.SortFunc(out, func(a, b identity.ProjectRecord) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// CreateProjectWithGrant implements [identity.ProjectStore]. The fake keeps no role bindings.
func (m *Memory) CreateProjectWithGrant(ctx context.Context, org orgscope.Org, p identity.ProjectRecord, _ identity.RoleBindingRecord, entries []audit.Entry) error {
	return m.CreateProject(ctx, org, p, entries[0])
}

// CreateOrganizationWithAdmin implements [identity.OrgStore]. The fake keeps no memberships,
// role bindings or audit entries.
func (m *Memory) CreateOrganizationWithAdmin(ctx context.Context, o identity.OrgRecord, _ identity.OrgKeyRecord, _ identity.MembershipRecord, _ identity.RoleBindingRecord, _ []audit.Entry) error {
	return m.CreateOrganization(ctx, o)
}

// CreateFlowWithEntry implements [flow.Store]. The in-memory fake keeps no audit chain, so
// the entry is accepted and dropped; the storage adapters write it.
func (m *Memory) CreateFlowWithEntry(ctx context.Context, org orgscope.Org, f flow.Record, _ audit.Entry) error {
	return m.CreateFlow(ctx, org, f)
}

// CreateFlow implements [flow.Store].
func (m *Memory) CreateFlow(_ context.Context, org orgscope.Org, f flow.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !f.OrgID.Matches(org) {
		return fmt.Errorf("flow of organization %s: %w", f.OrgID, identity.ErrScopeMismatch)
	}
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
func (m *Memory) GetFlow(_ context.Context, org orgscope.Org, id flow.FlowID) (flow.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.s.flows[id]
	if !ok || !f.OrgID.Matches(org) || f.IsDeleted() {
		return flow.Record{}, fmt.Errorf("flow %s: %w", id, flow.ErrNotFound)
	}
	out := *f
	out.Draft = bytes.Clone(f.Draft)
	return out, nil
}

// GetFlowByName implements [flow.Store].
func (m *Memory) GetFlowByName(_ context.Context, org orgscope.Org, project identity.ProjectID, name string) (flow.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, f := range m.s.flows {
		if f.OrgID.Matches(org) && f.ProjectID == project && f.Name == name && !f.IsDeleted() {
			return *f, nil
		}
	}
	return flow.Record{}, fmt.Errorf("flow %q: %w", name, flow.ErrNotFound)
}

// ListFlows implements [flow.Store].
func (m *Memory) ListFlows(_ context.Context, org orgscope.Org, project identity.ProjectID, after flow.FlowID, limit int) ([]flow.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []flow.Record
	for _, f := range m.s.flows {
		if f.OrgID.Matches(org) && f.ProjectID == project && !f.IsDeleted() && string(f.ID[:]) > string(after[:]) {
			out = append(out, *f)
		}
	}
	slices.SortFunc(out, func(a, b flow.Record) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// UpdateFlowDraft implements [flow.Store]. The memory store keeps no connector
// references of drafts: the references argument is accepted and ignored.
func (m *Memory) UpdateFlowDraft(_ context.Context, org orgscope.Org, id flow.FlowID, draft []byte, expected int64, _ []flow.ConnectorReference, now time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.s.flows[id]
	if !ok || !f.OrgID.Matches(org) || f.IsDeleted() {
		return 0, fmt.Errorf("flow %s: %w", id, flow.ErrNotFound)
	}
	if f.DraftRevision != expected {
		return 0, flow.ErrDraftConflict
	}
	f.Draft, f.DraftRevision, f.UpdatedAt = bytes.Clone(draft), expected+1, now
	return f.DraftRevision, nil
}

// PublishVersion implements [flow.VersionStore].
func (m *Memory) PublishVersion(_ context.Context, org orgscope.Org, v flow.VersionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !v.OrgID.Matches(org) {
		return fmt.Errorf("version of organization %s: %w", v.OrgID, identity.ErrScopeMismatch)
	}
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
func (m *Memory) GetVersion(_ context.Context, org orgscope.Org, id flow.FlowVersionID) (flow.VersionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.s.versions[id]
	if !ok || !v.OrgID.Matches(org) {
		return flow.VersionRecord{}, fmt.Errorf("version %s: %w", id, flow.ErrVersionNotFound)
	}
	return *v, nil
}

// GetVersionByNumber implements [flow.VersionStore] and [trigger.FlowSource].
func (m *Memory) GetVersionByNumber(_ context.Context, org orgscope.Org, id flow.FlowID, number int64) (flow.VersionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, v := range m.s.versions {
		if v.OrgID.Matches(org) && v.FlowID == id && v.Number == number {
			return *v, nil
		}
	}
	return flow.VersionRecord{}, fmt.Errorf("version %d of flow %s: %w", number, id, flow.ErrVersionNotFound)
}

// ListVersions implements [flow.VersionStore].
func (m *Memory) ListVersions(_ context.Context, org orgscope.Org, id flow.FlowID, after int64, limit int) ([]flow.VersionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []flow.VersionRecord
	for _, v := range m.s.versions {
		if v.OrgID.Matches(org) && v.FlowID == id && v.Number > after {
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
func (m *Memory) CreateSnapshot(_ context.Context, org orgscope.Org, s flow.SnapshotRecord) (flow.SnapshotRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !s.OrgID.Matches(org) {
		return flow.SnapshotRecord{}, fmt.Errorf("snapshot of organization %s: %w", s.OrgID, identity.ErrScopeMismatch)
	}
	for _, e := range m.s.snapshots {
		if e.OrgID == s.OrgID && e.ContentHash == s.ContentHash {
			return *e, nil
		}
	}
	m.s.snapshots[s.ID] = &s
	return s, nil
}

// GetSnapshot implements [flow.SnapshotStore].
func (m *Memory) GetSnapshot(_ context.Context, org orgscope.Org, id flow.FlowSnapshotID) (flow.SnapshotRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.s.snapshots[id]
	if !ok || !s.OrgID.Matches(org) {
		return flow.SnapshotRecord{}, fmt.Errorf("snapshot %s: %w", id, flow.ErrSnapshotNotFound)
	}
	return *s, nil
}
