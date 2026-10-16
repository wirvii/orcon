// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/trigger"
)

var _ trigger.Store = (*Store)(nil)

// addressPage bounds the rows one address read returns: a trigger has a handful
// of addresses with a handful of methods each.
const addressPage = 1000

// Atomic implements [trigger.Store]: fn runs in the writer, under the group
// commit, and rolls back alone when it fails.
func (s *Store) Atomic(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx trigger.Tx) error) error {
	if org.IsZero() {
		return identity.ErrNoScope
	}
	return s.write(ctx, "run a trigger transaction", func(ctx context.Context, q *sqlcgen.Queries) error {
		return fn(ctx, &triggerTx{q: q, org: org})
	})
}

// Read implements [trigger.Store]: fn runs on one read connection.
func (s *Store) Read(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx trigger.Tx) error) error {
	if org.IsZero() {
		return identity.ErrNoScope
	}
	return s.read(ctx, "run a trigger read", func(ctx context.Context, q *sqlcgen.Queries) error {
		return fn(ctx, &triggerTx{q: q, org: org})
	})
}

// triggerTx is one trigger transaction bound to an organization.
type triggerTx struct {
	q   *sqlcgen.Queries
	org orgscope.Org
}

// triggerTx is not asserted to implement [trigger.Tx] with a variable declaration:
// the orgscope analyzer (rule 3) reads such assertions as store ports, and the
// methods of a transaction act on the organization that Atomic or Read scoped.
// Atomic and Read pass the value as the interface, so the compiler still checks it.

func (t *triggerTx) orgBytes() []byte { return uuidBytes(t.org.UUID()) }

func stateFrom(id flow.FlowID, name string, project, activeVersion []byte, activation string, draftRevision, revision int64, deletedAt sql.NullInt64) (trigger.FlowState, error) {
	var out trigger.FlowState
	var err error
	if out.ProjectID, err = uuidOf[identity.ProjectID](project); err != nil {
		return out, fmt.Errorf("flow project id: %w", err)
	}
	if out.ActiveVersionID, err = nullUUIDOf[flow.FlowVersionID](activeVersion); err != nil {
		return out, fmt.Errorf("flow active version id: %w", err)
	}
	out.ID, out.Name, out.ActivationState = id, name, flow.ActivationState(activation)
	out.DraftRevision, out.Revision, out.DeletedAt = draftRevision, revision, nullInstant(deletedAt)
	return out, nil
}

func (t *triggerTx) GetFlowState(ctx context.Context, flowID flow.FlowID) (trigger.FlowState, error) {
	row, err := t.q.GetFlowState(ctx, t.orgBytes(), uuidBytes(flowID))
	if err != nil {
		return trigger.FlowState{}, notFoundAs(err, flow.ErrNotFound)
	}
	return stateFrom(flowID, row.Name, row.ProjectID, row.ActiveFlowVersionID, row.ActivationState, row.DraftRevision, row.Revision, row.DeletedAt)
}

// LockFlowState needs no lock: the single writer already serializes transactions.
func (t *triggerTx) LockFlowState(ctx context.Context, flowID flow.FlowID) (trigger.FlowState, error) {
	row, err := t.q.LockFlowState(ctx, t.orgBytes(), uuidBytes(flowID))
	if err != nil {
		return trigger.FlowState{}, notFoundAs(err, flow.ErrNotFound)
	}
	return stateFrom(flowID, row.Name, row.ProjectID, row.ActiveFlowVersionID, row.ActivationState, row.DraftRevision, row.Revision, row.DeletedAt)
}

func (t *triggerTx) NextVersionNumber(ctx context.Context, flowID flow.FlowID) (int64, error) {
	return t.q.GetNextFlowVersionNumber(ctx, t.orgBytes(), uuidBytes(flowID))
}

func (t *triggerTx) InsertVersion(ctx context.Context, v flow.VersionRecord) error {
	params := sqlcgen.InsertFlowVersionParams{
		ID: uuidBytes(v.ID), OrgID: t.orgBytes(), FlowID: uuidBytes(v.FlowID),
		VersionNumber: v.Number, Document: v.Document, ContentHash: v.ContentHash,
		PublishedByPrincipalID: nullUUIDBytes(v.PublishedByPrincipalID),
		PublishedAt:            micros(v.PublishedAt), CreatedAt: micros(v.CreatedAt),
		PublicationNote: nullText(v.Note), RolledBackFromVersionNumber: nullPositive(v.RolledBackFromVersion),
	}
	if v.Warnings != nil {
		params.WarningDocument = v.Warnings
		params.WarningDocumentFormat = sql.NullString{String: flow.WarningsFormat, Valid: true}
	}
	if err := t.q.InsertFlowVersion(ctx, params); err != nil {
		return err
	}
	return insertVersionReferences(ctx, t.q, v)
}

func (t *triggerTx) SetActiveVersion(ctx context.Context, flowID flow.FlowID, version flow.FlowVersionID, now time.Time) error {
	n, err := t.q.UpdateFlowActiveVersion(ctx, sqlcgen.UpdateFlowActiveVersionParams{
		FlowVersionID: uuidBytes(version), UpdatedAt: micros(now), OrgID: t.orgBytes(), ID: uuidBytes(flowID),
	})
	return expectOne(n, err, "flow", flowID)
}

func (t *triggerTx) ReplaceDraft(ctx context.Context, flowID flow.FlowID, draft []byte, expectedRevision int64, references []flow.ConnectorReference, now time.Time) error {
	n, err := t.q.UpdateFlowDraft(ctx, sqlcgen.UpdateFlowDraftParams{
		DraftDocument: draft, UpdatedAt: micros(now), OrgID: t.orgBytes(), ID: uuidBytes(flowID), ExpectedRevision: expectedRevision,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return flow.ErrDraftConflict
	}
	return replaceDraftReferences(ctx, t.q, t.org, flowID, references, now)
}

func (t *triggerTx) SetActivationState(ctx context.Context, flowID flow.FlowID, state flow.ActivationState, now time.Time) error {
	n, err := t.q.UpdateFlowActivationState(ctx, sqlcgen.UpdateFlowActivationStateParams{
		ActivationState: string(state), UpdatedAt: micros(now), OrgID: t.orgBytes(), ID: uuidBytes(flowID),
	})
	return expectOne(n, err, "flow", flowID)
}

func (t *triggerTx) SoftDeleteFlow(ctx context.Context, flowID flow.FlowID, by identity.PrincipalID, now time.Time) error {
	n, err := t.q.MarkFlowDeleted(ctx, sqlcgen.MarkFlowDeletedParams{
		DeletedAt: nullMicros(now), DeletedByPrincipalID: nullUUIDBytes(by), UpdatedAt: micros(now), OrgID: t.orgBytes(), ID: uuidBytes(flowID),
	})
	return expectOne(n, err, "flow", flowID)
}

func (t *triggerTx) RestoreFlow(ctx context.Context, flowID flow.FlowID, now time.Time) error {
	n, err := t.q.RestoreFlow(ctx, micros(now), t.orgBytes(), uuidBytes(flowID))
	return expectOne(n, err, "flow", flowID)
}

// RecordChange implements [trigger.Tx]: the idempotency claim of the context, when there is one,
// and the audit entry, in the transaction Atomic opened.
func (t *triggerTx) RecordChange(ctx context.Context, entry audit.Entry, resourceRef string, now time.Time) error {
	if err := claimIdempotency(ctx, t.q, t.org, resourceRef, now); err != nil {
		return err
	}
	return insertPendingOrg(ctx, t.q, t.org, entry)
}

func (t *triggerTx) PurgeFlow(ctx context.Context, flowID flow.FlowID) error {
	n, err := t.q.PurgeFlow(ctx, t.orgBytes(), uuidBytes(flowID))
	return expectOne(n, err, "flow", flowID)
}

// expectOne turns "no row changed" into flow.ErrNotFound.
func expectOne(n int64, err error, what string, id fmt.Stringer) error {
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%s %s: %w", what, id, flow.ErrNotFound)
	}
	return nil
}

func triggerFrom(row *sqlcgen.FlowTrigger) (trigger.Trigger, error) {
	var out trigger.Trigger
	var err error
	if out.ID, err = uuidOf[trigger.ID](row.ID); err != nil {
		return out, fmt.Errorf("trigger id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return out, fmt.Errorf("trigger org id: %w", err)
	}
	if out.FlowID, err = uuidOf[flow.FlowID](row.FlowID); err != nil {
		return out, fmt.Errorf("trigger flow id: %w", err)
	}
	if out.FlowVersionID, err = uuidOf[flow.FlowVersionID](row.FlowVersionID); err != nil {
		return out, fmt.Errorf("trigger flow version id: %w", err)
	}
	out.NodeID, out.Family, out.Type = flow.NodeID(row.NodeID), flow.TriggerFamily(row.Family), row.Type
	out.ConfigurationHash, out.Status, out.Health = row.ConfigurationHash, trigger.Status(row.Status), trigger.Health(row.Health)
	out.LastError = row.LastError.String
	out.CreatedAt, out.UpdatedAt = instant(row.CreatedAt), instant(row.UpdatedAt)
	return out, nil
}

func (t *triggerTx) ListTriggers(ctx context.Context, flowID flow.FlowID) ([]trigger.Trigger, error) {
	var out []trigger.Trigger
	var after [16]byte
	for {
		rows, err := t.q.ListFlowTriggers(ctx, sqlcgen.ListFlowTriggersParams{
			OrgID: t.orgBytes(), FlowID: uuidBytes(flowID), AfterID: after[:], PageSize: maxPage,
		})
		if err != nil {
			return nil, err
		}
		for i := range rows {
			tr, err := triggerFrom(&rows[i])
			if err != nil {
				return nil, err
			}
			out = append(out, tr)
			after = [16]byte(tr.ID)
		}
		if int64(len(rows)) < maxPage {
			break
		}
	}
	slices.SortFunc(out, func(a, b trigger.Trigger) int { return strings.Compare(string(a.NodeID), string(b.NodeID)) })
	return out, nil
}

func (t *triggerTx) InsertTrigger(ctx context.Context, tr trigger.Trigger) error {
	return t.q.InsertFlowTrigger(ctx, sqlcgen.InsertFlowTriggerParams{
		ID: uuidBytes(tr.ID), OrgID: t.orgBytes(), FlowID: uuidBytes(tr.FlowID), NodeID: string(tr.NodeID),
		Family: string(tr.Family), Type: tr.Type, ConfigurationHash: tr.ConfigurationHash,
		FlowVersionID: uuidBytes(tr.FlowVersionID), Status: string(tr.Status), Health: string(tr.Health),
		LastError: nullText(tr.LastError), CreatedAt: micros(tr.CreatedAt), UpdatedAt: micros(tr.UpdatedAt),
	})
}

func (t *triggerTx) UpdateTrigger(ctx context.Context, tr trigger.Trigger) error {
	n, err := t.q.UpdateFlowTrigger(ctx, sqlcgen.UpdateFlowTriggerParams{
		Family: string(tr.Family), Type: tr.Type, ConfigurationHash: tr.ConfigurationHash,
		FlowVersionID: uuidBytes(tr.FlowVersionID), Status: string(tr.Status), Health: string(tr.Health),
		LastError: nullText(tr.LastError), UpdatedAt: micros(tr.UpdatedAt), OrgID: t.orgBytes(), ID: uuidBytes(tr.ID),
	})
	return expectOne(n, err, "trigger", tr.ID)
}

func (t *triggerTx) SetTriggerStatuses(ctx context.Context, flowID flow.FlowID, status trigger.Status, now time.Time) error {
	return t.q.UpdateFlowTriggerStatuses(ctx, sqlcgen.UpdateFlowTriggerStatusesParams{
		Status: string(status), UpdatedAt: micros(now), OrgID: t.orgBytes(), FlowID: uuidBytes(flowID),
	})
}

func (t *triggerTx) ListAddresses(ctx context.Context, id trigger.ID) ([]trigger.Address, error) {
	rows, err := t.q.ListTriggerAddresses(ctx, t.orgBytes(), uuidBytes(id), addressPage)
	if err != nil {
		return nil, err
	}
	var out []trigger.Address
	index := map[[16]byte]int{}
	for i := range rows {
		r := &rows[i]
		key := [16]byte(r.ID)
		at, seen := index[key]
		if !seen {
			a, err := addressFrom(r)
			if err != nil {
				return nil, err
			}
			out = append(out, a)
			at = len(out) - 1
			index[key] = at
		}
		if r.MethodRowID != nil {
			mid, err := uuidOf[trigger.MethodID](r.MethodRowID)
			if err != nil {
				return nil, fmt.Errorf("address method id: %w", err)
			}
			out[at].Methods = append(out[at].Methods, trigger.AddressMethod{ID: mid, Method: r.Method.String})
		}
	}
	return out, nil
}

func addressFrom(r *sqlcgen.ListTriggerAddressesRow) (trigger.Address, error) {
	var out trigger.Address
	var err error
	if out.ID, err = uuidOf[trigger.AddressID](r.ID); err != nil {
		return out, fmt.Errorf("address id: %w", err)
	}
	if out.OrgID, err = uuidOf[identity.OrgID](r.OrgID); err != nil {
		return out, fmt.Errorf("address org id: %w", err)
	}
	if out.TriggerID, err = uuidOf[trigger.ID](r.FlowTriggerID); err != nil {
		return out, fmt.Errorf("address trigger id: %w", err)
	}
	out.Family, out.Scope, out.Pattern = flow.AddressFamily(r.Family), trigger.Scope(r.Scope), r.Pattern
	out.StaticPrefix, out.Host, out.RandomSuffix = r.StaticPrefix, r.Host.String, r.RandomSuffix.String
	out.ResponseProfile, out.Status = flow.ResponseProfile(r.ResponseProfile), trigger.AddressStatus(r.Status)
	out.RetiredAt, out.CreatedAt, out.UpdatedAt = nullInstant(r.RetiredAt), instant(r.CreatedAt), instant(r.UpdatedAt)
	return out, nil
}

func (t *triggerTx) InsertAddress(ctx context.Context, a *trigger.Address) error {
	err := t.q.InsertTriggerAddress(ctx, sqlcgen.InsertTriggerAddressParams{
		ID: uuidBytes(a.ID), OrgID: t.orgBytes(), FlowTriggerID: uuidBytes(a.TriggerID),
		Family: string(a.Family), Scope: string(a.Scope), Pattern: a.Pattern, StaticPrefix: a.StaticPrefix,
		Host: nullText(a.Host), RandomSuffix: nullText(a.RandomSuffix), ResponseProfile: string(a.ResponseProfile),
		Status: string(a.Status), RetiredAt: nullMicros(a.RetiredAt), CreatedAt: micros(a.CreatedAt), UpdatedAt: micros(a.UpdatedAt),
	})
	if err != nil {
		return err
	}
	return t.insertMethods(ctx, a.ID, a.Methods, a.CreatedAt)
}

func (t *triggerTx) insertMethods(ctx context.Context, address trigger.AddressID, methods []trigger.AddressMethod, now time.Time) error {
	for _, m := range methods {
		if err := t.q.InsertTriggerAddressMethod(ctx, sqlcgen.InsertTriggerAddressMethodParams{
			ID: uuidBytes(m.ID), OrgID: t.orgBytes(), TriggerAddressID: uuidBytes(address), Method: m.Method,
			CreatedAt: micros(now), UpdatedAt: micros(now),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (t *triggerTx) UpdateAddress(ctx context.Context, a *trigger.Address) error {
	n, err := t.q.UpdateTriggerAddress(ctx, sqlcgen.UpdateTriggerAddressParams{
		Family: string(a.Family), Host: nullText(a.Host), RandomSuffix: nullText(a.RandomSuffix),
		ResponseProfile: string(a.ResponseProfile), Status: string(a.Status), RetiredAt: nullMicros(a.RetiredAt),
		UpdatedAt: micros(a.UpdatedAt), OrgID: t.orgBytes(), ID: uuidBytes(a.ID),
	})
	return expectOne(n, err, "address", a.ID)
}

func (t *triggerTx) ReplaceAddressMethods(ctx context.Context, address trigger.AddressID, methods []trigger.AddressMethod, now time.Time) error {
	if err := t.q.DeleteTriggerAddressMethods(ctx, t.orgBytes(), uuidBytes(address)); err != nil {
		return err
	}
	return t.insertMethods(ctx, address, methods, now)
}

// ResolveRoute implements [trigger.Store]. It reads the one store without an
// organization scope on purpose: see the query ResolveTriggerRoute.
func (s *Store) ResolveRoute(ctx context.Context, method, path string) (trigger.Resolved, error) {
	var out trigger.Resolved
	err := s.read(ctx, "resolve route", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.ResolveTriggerRoute(ctx, path, method)
		if errors.Is(err, sql.ErrNoRows) {
			return trigger.ErrNoRoute
		}
		if err != nil {
			return err
		}
		return resolvedInto(&out, &row)
	})
	if err != nil {
		return out, err
	}
	out.Scope, err = s.ScopeOf(out.Org)
	return out, err
}

func resolvedInto(out *trigger.Resolved, row *sqlcgen.ResolveTriggerRouteRow) error {
	var err error
	if out.Org, err = uuidOf[identity.OrgID](row.OrgID); err != nil {
		return fmt.Errorf("route org id: %w", err)
	}
	if out.ProjectID, err = uuidOf[identity.ProjectID](row.ProjectID); err != nil {
		return fmt.Errorf("route project id: %w", err)
	}
	if out.FlowID, err = uuidOf[flow.FlowID](row.FlowID); err != nil {
		return fmt.Errorf("route flow id: %w", err)
	}
	out.NodeID = flow.NodeID(row.NodeID)
	if out.TriggerID, err = uuidOf[trigger.ID](row.FlowTriggerID); err != nil {
		return fmt.Errorf("route trigger id: %w", err)
	}
	if out.AddressID, err = uuidOf[trigger.AddressID](row.TriggerAddressID); err != nil {
		return fmt.Errorf("route address id: %w", err)
	}
	return nil
}

// LockRoute has no SQL on SQLite: one writer applies every transaction in turn,
// so two claims of a path never interleave. PostgreSQL takes an advisory lock.
func (t *triggerTx) LockRoute(context.Context, trigger.Scope, string) error { return nil }

func (t *triggerTx) ListRoutes(ctx context.Context, scope trigger.Scope, after trigger.MethodID, limit int) ([]trigger.RoutePage, error) {
	rows, err := t.q.ListTriggerRoutes(ctx, sqlcgen.ListTriggerRoutesParams{
		OrgID: t.orgBytes(), Scope: string(scope), AfterID: uuidBytes(after), PageSize: clampPage(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]trigger.RoutePage, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		var p trigger.RoutePage
		if p.MethodID, err = uuidOf[trigger.MethodID](r.MethodRowID); err != nil {
			return nil, fmt.Errorf("route method id: %w", err)
		}
		if p.AddressID, err = uuidOf[trigger.AddressID](r.AddressID); err != nil {
			return nil, fmt.Errorf("route address id: %w", err)
		}
		if p.TriggerID, err = uuidOf[trigger.ID](r.FlowTriggerID); err != nil {
			return nil, fmt.Errorf("route trigger id: %w", err)
		}
		if p.FlowID, err = uuidOf[flow.FlowID](r.FlowID); err != nil {
			return nil, fmt.Errorf("route flow id: %w", err)
		}
		p.FlowName, p.NodeID, p.Pattern, p.Method = r.FlowName, flow.NodeID(r.NodeID), r.Pattern, r.Method
		p.Status, p.RetiredAt = trigger.AddressStatus(r.Status), nullInstant(r.RetiredAt)
		out = append(out, p)
	}
	return out, nil
}

func (t *triggerTx) RouteTakenByOtherOrganization(ctx context.Context, scope trigger.Scope, pattern, method string, retiredAfter time.Time) (bool, error) {
	return t.q.ExistsRouteTakenByOtherOrganization(ctx, sqlcgen.ExistsRouteTakenByOtherOrganizationParams{
		Scope: string(scope), Pattern: pattern, Method: method, OrgID: t.orgBytes(), RetiredAfter: nullMicros(retiredAfter),
	})
}

// NotifyRoutesChanged has no SQL on SQLite and no channel to signal: the route index of a SQLite
// instance lives in its own process and rebuilds there.
func (t *triggerTx) NotifyRoutesChanged(context.Context, flow.FlowID) error { return nil }

func (t *triggerTx) AppendEvent(ctx context.Context, e trigger.Event) error {
	return t.q.AppendTriggerEvent(ctx, sqlcgen.AppendTriggerEventParams{
		ID: uuidBytes(e.ID), OrgID: t.orgBytes(), OccurredAt: micros(e.OccurredAt), FlowID: uuidBytes(e.FlowID),
		NodeID: nullText(string(e.NodeID)), Type: e.Type, SchemaVersion: trigger.EventSchemaVersion,
		DataDocument: e.Data, CreatedAt: micros(e.OccurredAt),
	})
}

func (t *triggerTx) ListEvents(ctx context.Context, flowID flow.FlowID, after trigger.EventID, limit int) ([]trigger.Event, error) {
	rows, err := t.q.ListTriggerEvents(ctx, sqlcgen.ListTriggerEventsParams{
		OrgID: t.orgBytes(), FlowID: uuidBytes(flowID), AfterID: uuidBytes(after), PageSize: clampPage(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]trigger.Event, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		e := trigger.Event{OccurredAt: instant(r.OccurredAt), NodeID: flow.NodeID(r.NodeID.String), Type: r.Type, Data: r.DataDocument, FlowID: flowID}
		if e.ID, err = uuidOf[trigger.EventID](r.ID); err != nil {
			return nil, fmt.Errorf("event id: %w", err)
		}
		if e.OrgID, err = uuidOf[identity.OrgID](r.OrgID); err != nil {
			return nil, fmt.Errorf("event org id: %w", err)
		}
		out = append(out, e)
	}
	return out, nil
}

func (t *triggerTx) ResolveAcceptance(ctx context.Context, flowID flow.FlowID, node flow.NodeID) (trigger.Acceptance, error) {
	row, err := t.q.GetTriggerAcceptance(ctx, t.orgBytes(), uuidBytes(flowID), string(node))
	if err != nil {
		return trigger.Acceptance{}, notFoundAs(err, flow.ErrNotFound)
	}
	var out trigger.Acceptance
	if out.TriggerID, err = uuidOf[trigger.ID](row.ID); err != nil {
		return out, fmt.Errorf("trigger id: %w", err)
	}
	if out.FlowVersionID, err = uuidOf[flow.FlowVersionID](row.FlowVersionID); err != nil {
		return out, fmt.Errorf("trigger flow version id: %w", err)
	}
	out.Family, out.Type, out.NodeID = flow.TriggerFamily(row.Family), row.Type, flow.NodeID(row.NodeID)
	out.VersionNumber, out.ConfigurationHash = row.VersionNumber, row.ConfigurationHash
	out.Accepting = !row.DeletedAt.Valid && flow.ActivationState(row.ActivationState) == flow.ActivationActive &&
		trigger.Status(row.Status) == trigger.StatusActive
	return out, nil
}
