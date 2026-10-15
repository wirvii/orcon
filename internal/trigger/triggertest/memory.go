// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/trigger"
)

// Stores is everything the battery needs from a storage backend: the flow
// ports the service reads and the ports that create the rows of a scenario.
// The SQLite and PostgreSQL adapters implement it, and so does [Memory].
type Stores interface { //nolint:interfacebloat // the battery exercises every port of a backend through one value
	trigger.FlowSource
	identity.OrgStore
	principalRows
	identity.ProjectStore
	flow.Store
	flow.VersionStore
	flow.SnapshotStore
}

// principalRows is the part of [identity.PrincipalStore] the battery uses: the
// scenario creates its principal and reads the system one. The deletion of a
// principal is not part of the battery, so the in-memory store does not offer it.
type principalRows interface {
	CreatePrincipal(ctx context.Context, p identity.PrincipalRecord) error
	GetSystemPrincipal(ctx context.Context) (identity.PrincipalRecord, error)
}

// Backend is one store under test.
type Backend struct {
	// Triggers is the trigger store.
	Triggers trigger.Store
	// Flows holds the flow ports.
	Flows Stores
	// Scope turns the identity of an organization of the battery into the scope
	// the store methods take. Only the tests that build the backend may create
	// scopes (ADR-0046 section 2), so the battery receives this function instead
	// of creating them itself.
	Scope func(identity.OrgID) orgscope.Org
}

// Memory is an in-memory implementation of the trigger store and of the flow
// ports, for tests of the area that must not touch a database. It applies the
// constraints the lifecycle relies on: unique names among live flows, unique
// version numbers, organization isolation and all-or-nothing transactions.
// It is safe for concurrent use; transactions are serialized like the single
// writer of the SQLite adapter.
type Memory struct {
	// Scopes issues the scope of the organization a route names. The owner of the
	// memory sets it; the store of a deployment gets it from the adapter.
	Scopes store.Scopes

	mu sync.Mutex
	s  memState
}

type memState struct {
	orgs       map[identity.OrgID]*identity.OrgRecord
	principals map[identity.PrincipalID]*identity.PrincipalRecord
	projects   map[identity.ProjectID]*identity.ProjectRecord
	flows      map[flow.FlowID]*flow.Record
	versions   map[flow.FlowVersionID]*flow.VersionRecord
	snapshots  map[flow.FlowSnapshotID]*flow.SnapshotRecord
	triggers   map[trigger.ID]*trigger.Trigger
	addresses  map[trigger.AddressID]*trigger.Address
	events     []trigger.Event
}

// NewMemory returns an empty store.
func NewMemory() *Memory {
	return &Memory{s: memState{
		orgs:       map[identity.OrgID]*identity.OrgRecord{},
		principals: map[identity.PrincipalID]*identity.PrincipalRecord{},
		projects:   map[identity.ProjectID]*identity.ProjectRecord{},
		flows:      map[flow.FlowID]*flow.Record{},
		versions:   map[flow.FlowVersionID]*flow.VersionRecord{},
		snapshots:  map[flow.FlowSnapshotID]*flow.SnapshotRecord{},
		triggers:   map[trigger.ID]*trigger.Trigger{},
		addresses:  map[trigger.AddressID]*trigger.Address{},
	}}
}

// clone copies the state so a failed transaction can be undone. Rows are
// copied one by one; only the slices inside them need their own copy.
func (s *memState) clone() memState {
	out := *s
	out.orgs = cloneRows(s.orgs)
	out.principals = cloneRows(s.principals)
	out.projects = cloneRows(s.projects)
	out.flows = cloneRows(s.flows)
	out.versions = cloneRows(s.versions)
	out.snapshots = cloneRows(s.snapshots)
	out.triggers = cloneRows(s.triggers)
	out.addresses = cloneRows(s.addresses)
	for _, a := range out.addresses {
		a.Methods = slices.Clone(a.Methods)
	}
	out.events = slices.Clone(s.events)
	return out
}

func cloneRows[K comparable, V any](in map[K]*V) map[K]*V {
	out := make(map[K]*V, len(in))
	for k, v := range in {
		c := *v
		out[k] = &c
	}
	return out
}

// Backend returns the memory store as a [Backend].
func (m *Memory) Backend() Backend { return Backend{Triggers: m, Flows: m} }

// Atomic implements [trigger.Store].
func (m *Memory) Atomic(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx trigger.Tx) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("run a transaction: %w", err)
	}
	if org.IsZero() {
		return identity.ErrNoScope
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	before := m.s.clone()
	if err := fn(ctx, &memTx{m: m, org: identity.OrgID(org.UUID())}); err != nil {
		m.s = before
		return err
	}
	return nil
}

// Read implements [trigger.Store].
func (m *Memory) Read(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx trigger.Tx) error) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("run a read: %w", err)
	}
	if org.IsZero() {
		return identity.ErrNoScope
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	before := m.s.clone()
	err := fn(ctx, &memTx{m: m, org: identity.OrgID(org.UUID()), readOnly: true})
	m.s = before // a read changes nothing, whatever fn did
	return err
}

type memTx struct {
	m        *Memory
	org      identity.OrgID
	readOnly bool
}

func (t *memTx) st() *memState { return &t.m.s }

func (t *memTx) flowRow(flowID flow.FlowID) (*flow.Record, error) {
	f, ok := t.st().flows[flowID]
	if !ok || f.OrgID != t.org {
		return nil, fmt.Errorf("flow %s: %w", flowID, flow.ErrNotFound)
	}
	return f, nil
}

var errReadOnly = errors.New("write in a read-only transaction")

func (t *memTx) write() error {
	if t.readOnly {
		return errReadOnly
	}
	return nil
}

func stateOf(f *flow.Record) trigger.FlowState {
	st := f.ActivationState
	if st == "" {
		st = flow.ActivationInactive
	}
	return trigger.FlowState{
		ID: f.ID, ProjectID: f.ProjectID, Name: f.Name, ActivationState: st,
		ActiveVersionID: f.ActiveVersionID, DraftRevision: f.DraftRevision, DeletedAt: f.DeletedAt,
	}
}

func (t *memTx) GetFlowState(_ context.Context, flowID flow.FlowID) (trigger.FlowState, error) {
	f, err := t.flowRow(flowID)
	if err != nil {
		return trigger.FlowState{}, err
	}
	return stateOf(f), nil
}

func (t *memTx) LockFlowState(ctx context.Context, flowID flow.FlowID) (trigger.FlowState, error) {
	return t.GetFlowState(ctx, flowID)
}

func (t *memTx) NextVersionNumber(_ context.Context, flowID flow.FlowID) (int64, error) {
	next := int64(1)
	for _, v := range t.st().versions {
		if v.OrgID == t.org && v.FlowID == flowID && v.Number >= next {
			next = v.Number + 1
		}
	}
	return next, nil
}

func (t *memTx) InsertVersion(_ context.Context, v flow.VersionRecord) error {
	if err := t.write(); err != nil {
		return err
	}
	if _, err := t.flowRow(v.FlowID); err != nil {
		return err
	}
	for _, o := range t.st().versions {
		if o.OrgID == t.org && o.FlowID == v.FlowID && o.Number == v.Number {
			return fmt.Errorf("version %d of flow %s: %w", v.Number, v.FlowID, flow.ErrConflict)
		}
	}
	v.OrgID = t.org
	t.st().versions[v.ID] = &v
	return nil
}

func (t *memTx) SetActiveVersion(_ context.Context, flowID flow.FlowID, version flow.FlowVersionID, now time.Time) error {
	if err := t.write(); err != nil {
		return err
	}
	f, err := t.flowRow(flowID)
	if err != nil {
		return err
	}
	if v, ok := t.st().versions[version]; !ok || v.OrgID != t.org || v.FlowID != flowID {
		return fmt.Errorf("version %s: %w", version, flow.ErrReference)
	}
	f.ActiveVersionID, f.UpdatedAt = version, now
	return nil
}

func (t *memTx) ReplaceDraft(_ context.Context, flowID flow.FlowID, draft []byte, expected int64, _ []flow.ConnectorReference, now time.Time) error {
	if err := t.write(); err != nil {
		return err
	}
	f, err := t.flowRow(flowID)
	if err != nil {
		return err
	}
	if f.DraftRevision != expected {
		return flow.ErrDraftConflict
	}
	f.Draft, f.DraftRevision, f.UpdatedAt = bytes.Clone(draft), f.DraftRevision+1, now
	return nil
}

func (t *memTx) SetActivationState(_ context.Context, flowID flow.FlowID, state flow.ActivationState, now time.Time) error {
	if err := t.write(); err != nil {
		return err
	}
	f, err := t.flowRow(flowID)
	if err != nil {
		return err
	}
	f.ActivationState, f.UpdatedAt = state, now
	return nil
}

func (t *memTx) SoftDeleteFlow(_ context.Context, flowID flow.FlowID, by identity.PrincipalID, now time.Time) error {
	if err := t.write(); err != nil {
		return err
	}
	f, err := t.flowRow(flowID)
	if err != nil {
		return err
	}
	if by != (identity.PrincipalID{}) {
		if _, ok := t.st().principals[by]; !ok {
			return fmt.Errorf("principal %s: %w", by, flow.ErrReference)
		}
	}
	f.DeletedAt, f.DeletedByPrincipalID, f.UpdatedAt = now, by, now
	return nil
}

func (t *memTx) RestoreFlow(_ context.Context, flowID flow.FlowID, now time.Time) error {
	if err := t.write(); err != nil {
		return err
	}
	f, err := t.flowRow(flowID)
	if err != nil {
		return err
	}
	for _, o := range t.st().flows {
		if o.OrgID == t.org && o.ID != flowID && o.ProjectID == f.ProjectID && o.Name == f.Name && !o.IsDeleted() {
			return fmt.Errorf("flow name %q in the project: %w", f.Name, flow.ErrConflict)
		}
	}
	f.DeletedAt, f.DeletedByPrincipalID, f.UpdatedAt = time.Time{}, identity.PrincipalID{}, now
	return nil
}

func (t *memTx) PurgeFlow(_ context.Context, flowID flow.FlowID) error {
	if err := t.write(); err != nil {
		return err
	}
	if _, err := t.flowRow(flowID); err != nil {
		return err
	}
	s := t.st()
	delete(s.flows, flowID)
	for id, v := range s.versions {
		if v.FlowID == flowID {
			delete(s.versions, id)
		}
	}
	for id, sn := range s.snapshots {
		if sn.FlowID == flowID {
			delete(s.snapshots, id)
		}
	}
	for id, tr := range s.triggers {
		if tr.FlowID != flowID {
			continue
		}
		for aid, a := range s.addresses {
			if a.TriggerID == id {
				delete(s.addresses, aid)
			}
		}
		delete(s.triggers, id)
	}
	s.events = slices.DeleteFunc(s.events, func(e trigger.Event) bool { return e.FlowID == flowID })
	return nil
}

func (t *memTx) ListTriggers(_ context.Context, flowID flow.FlowID) ([]trigger.Trigger, error) {
	var out []trigger.Trigger
	for _, tr := range t.st().triggers {
		if tr.OrgID == t.org && tr.FlowID == flowID {
			out = append(out, *tr)
		}
	}
	slices.SortFunc(out, func(a, b trigger.Trigger) int { return strings.Compare(string(a.NodeID), string(b.NodeID)) })
	return out, nil
}

func (t *memTx) InsertTrigger(_ context.Context, tr trigger.Trigger) error {
	if err := t.write(); err != nil {
		return err
	}
	if _, err := t.flowRow(tr.FlowID); err != nil {
		return err
	}
	if _, ok := t.st().versions[tr.FlowVersionID]; !ok {
		return fmt.Errorf("version %s: %w", tr.FlowVersionID, flow.ErrReference)
	}
	for _, o := range t.st().triggers {
		if o.OrgID == t.org && o.FlowID == tr.FlowID && o.NodeID == tr.NodeID {
			return fmt.Errorf("trigger of node %q: %w", tr.NodeID, flow.ErrConflict)
		}
	}
	tr.OrgID = t.org
	t.st().triggers[tr.ID] = &tr
	return nil
}

func (t *memTx) UpdateTrigger(_ context.Context, tr trigger.Trigger) error {
	if err := t.write(); err != nil {
		return err
	}
	old, ok := t.st().triggers[tr.ID]
	if !ok || old.OrgID != t.org {
		return fmt.Errorf("trigger %s: %w", tr.ID, flow.ErrNotFound)
	}
	old.Family, old.Type, old.ConfigurationHash, old.FlowVersionID = tr.Family, tr.Type, tr.ConfigurationHash, tr.FlowVersionID
	old.Status, old.Health, old.LastError, old.UpdatedAt = tr.Status, tr.Health, tr.LastError, tr.UpdatedAt
	return nil
}

func (t *memTx) SetTriggerStatuses(_ context.Context, flowID flow.FlowID, status trigger.Status, now time.Time) error {
	if err := t.write(); err != nil {
		return err
	}
	for _, tr := range t.st().triggers {
		if tr.OrgID == t.org && tr.FlowID == flowID && tr.Status != trigger.StatusRemoved {
			tr.Status, tr.UpdatedAt = status, now
		}
	}
	return nil
}

func (t *memTx) ListAddresses(_ context.Context, id trigger.ID) ([]trigger.Address, error) {
	var out []trigger.Address
	for _, a := range t.st().addresses {
		if a.OrgID == t.org && a.TriggerID == id {
			c := *a
			c.Methods = slices.Clone(a.Methods)
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b trigger.Address) int { return strings.Compare(a.Pattern, b.Pattern) })
	return out, nil
}

func (t *memTx) InsertAddress(_ context.Context, a *trigger.Address) error {
	if err := t.write(); err != nil {
		return err
	}
	tr, ok := t.st().triggers[a.TriggerID]
	if !ok || tr.OrgID != t.org {
		return fmt.Errorf("trigger %s: %w", a.TriggerID, flow.ErrReference)
	}
	for _, o := range t.st().addresses {
		if o.OrgID == t.org && o.TriggerID == a.TriggerID && o.Scope == a.Scope && o.Pattern == a.Pattern {
			return fmt.Errorf("address %s: %w", a.Pattern, flow.ErrConflict)
		}
	}
	c := *a
	c.OrgID = t.org
	c.Methods = slices.Clone(a.Methods)
	t.st().addresses[a.ID] = &c
	return nil
}

func (t *memTx) UpdateAddress(_ context.Context, a *trigger.Address) error {
	if err := t.write(); err != nil {
		return err
	}
	old, ok := t.st().addresses[a.ID]
	if !ok || old.OrgID != t.org {
		return fmt.Errorf("address %s: %w", a.ID, flow.ErrNotFound)
	}
	old.Family, old.Status, old.RetiredAt, old.ResponseProfile = a.Family, a.Status, a.RetiredAt, a.ResponseProfile
	old.Host, old.RandomSuffix, old.UpdatedAt = a.Host, a.RandomSuffix, a.UpdatedAt
	return nil
}

func (t *memTx) ReplaceAddressMethods(_ context.Context, id trigger.AddressID, methods []trigger.AddressMethod, now time.Time) error {
	if err := t.write(); err != nil {
		return err
	}
	a, ok := t.st().addresses[id]
	if !ok || a.OrgID != t.org {
		return fmt.Errorf("address %s: %w", id, flow.ErrNotFound)
	}
	a.Methods, a.UpdatedAt = slices.Clone(methods), now
	return nil
}

func (t *memTx) LockRoute(context.Context, trigger.Scope, string) error { return nil }

func (t *memTx) ListRoutes(_ context.Context, scope trigger.Scope, after trigger.MethodID, limit int) ([]trigger.RoutePage, error) {
	var out []trigger.RoutePage
	for _, a := range t.st().addresses {
		if a.OrgID != t.org || a.Scope != scope {
			continue
		}
		tr := t.st().triggers[a.TriggerID]
		f := t.st().flows[tr.FlowID]
		for _, m := range a.Methods {
			if string(m.ID[:]) <= string(after[:]) {
				continue
			}
			out = append(out, trigger.RoutePage{MethodID: m.ID, Route: trigger.Route{
				AddressID: a.ID, TriggerID: a.TriggerID, FlowID: tr.FlowID, FlowName: f.Name, NodeID: tr.NodeID,
				Pattern: a.Pattern, Method: m.Method, Status: a.Status, RetiredAt: a.RetiredAt,
			}})
		}
	}
	slices.SortFunc(out, func(a, b trigger.RoutePage) int { return bytes.Compare(a.MethodID[:], b.MethodID[:]) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (t *memTx) RouteTakenByOtherOrganization(_ context.Context, scope trigger.Scope, pattern, method string, retiredAfter time.Time) (bool, error) {
	for _, a := range t.st().addresses {
		if a.OrgID == t.org || a.Scope != scope || a.Pattern != pattern {
			continue
		}
		if a.Status == trigger.AddressRetired && !a.RetiredAt.After(retiredAfter) {
			continue
		}
		for _, m := range a.Methods {
			if m.Method == method {
				return true, nil
			}
		}
	}
	return false, nil
}

func (t *memTx) NotifyRoutesChanged(context.Context, flow.FlowID) error { return nil }

func (t *memTx) AppendEvent(_ context.Context, e trigger.Event) error {
	if err := t.write(); err != nil {
		return err
	}
	if _, err := t.flowRow(e.FlowID); err != nil {
		return err
	}
	e.OrgID = t.org
	e.Data = bytes.Clone(e.Data)
	t.st().events = append(t.st().events, e)
	return nil
}

func (t *memTx) ListEvents(_ context.Context, flowID flow.FlowID, after trigger.EventID, limit int) ([]trigger.Event, error) {
	var out []trigger.Event
	for i := range t.st().events {
		e := &t.st().events[i]
		if e.OrgID == t.org && e.FlowID == flowID && string(e.ID[:]) > string(after[:]) {
			out = append(out, *e)
		}
	}
	slices.SortFunc(out, func(a, b trigger.Event) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (t *memTx) ResolveAcceptance(_ context.Context, flowID flow.FlowID, node flow.NodeID) (trigger.Acceptance, error) {
	f, err := t.flowRow(flowID)
	if err != nil {
		return trigger.Acceptance{}, err
	}
	for _, tr := range t.st().triggers {
		if tr.OrgID != t.org || tr.FlowID != flowID || tr.NodeID != node {
			continue
		}
		v := t.st().versions[tr.FlowVersionID]
		return trigger.Acceptance{
			TriggerID: tr.ID, Family: tr.Family, Type: tr.Type, NodeID: tr.NodeID,
			FlowVersionID: tr.FlowVersionID, VersionNumber: v.Number, ConfigurationHash: tr.ConfigurationHash,
			Accepting: !f.IsDeleted() && f.ActivationState == flow.ActivationActive && tr.Status == trigger.StatusActive,
		}, nil
	}
	return trigger.Acceptance{}, fmt.Errorf("node %q of flow %s: %w", node, flowID, flow.ErrNotFound)
}

// ResolveRoute implements [trigger.Store]: the active production address with
// the path and the method, smallest identity first, as the SQL statements order it.
func (m *Memory) ResolveRoute(_ context.Context, method, path string) (trigger.Resolved, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Scopes == nil {
		return trigger.Resolved{}, errors.New("triggertest: Memory.Scopes is not set")
	}
	var best *trigger.Address
	for _, a := range m.s.addresses {
		if a.Scope != trigger.ScopeProduction || a.Status != trigger.AddressActive || a.Pattern != path || !hasMethod(a, method) {
			continue
		}
		if best == nil || bytes.Compare(a.ID[:], best.ID[:]) < 0 {
			best = a
		}
	}
	if best == nil {
		return trigger.Resolved{}, trigger.ErrNoRoute
	}
	tr := m.s.triggers[best.TriggerID]
	f := m.s.flows[tr.FlowID]
	if f == nil || !f.DeletedAt.IsZero() {
		return trigger.Resolved{}, trigger.ErrNoRoute
	}
	scope, err := m.Scopes.ScopeOf(best.OrgID)
	if err != nil {
		return trigger.Resolved{}, err
	}
	return trigger.Resolved{
		Org: best.OrgID, Scope: scope, ProjectID: f.ProjectID, FlowID: tr.FlowID, NodeID: tr.NodeID,
		TriggerID: best.TriggerID, AddressID: best.ID,
	}, nil
}

func hasMethod(a *trigger.Address, method string) bool {
	for _, m := range a.Methods {
		if m.Method == method {
			return true
		}
	}
	return false
}
