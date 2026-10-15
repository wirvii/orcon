// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/trigger"
)

var _ trigger.Store = (*Store)(nil)

// addressPage bounds the rows one address read returns: a trigger has a handful
// of addresses with a handful of methods each.
const addressPage = 1000

// Atomic implements [trigger.Store]: fn runs in one READ COMMITTED
// transaction with the organization context set, and rolls back when it fails.
func (s *Store) Atomic(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx trigger.Tx) error) error {
	return s.inScope(ctx, "run a trigger transaction", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		return fn(ctx, &triggerTx{q: q, org: org})
	})
}

// Read implements [trigger.Store]: fn runs in one read-only transaction.
func (s *Store) Read(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx trigger.Tx) error) error {
	return s.inScope(ctx, "run a trigger read", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
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

func (t *triggerTx) orgID() pgtype.UUID { return pgUUID(t.org.UUID()) }

func stateFrom(id flow.FlowID, name string, project, activeVersion pgtype.UUID, activation string, revision int64, deletedAt pgtype.Timestamptz) trigger.FlowState {
	return trigger.FlowState{
		ID: id, ProjectID: fromPGUUID[identity.ProjectID](project), Name: name,
		ActivationState: flow.ActivationState(activation), ActiveVersionID: fromPGUUID[flow.FlowVersionID](activeVersion),
		DraftRevision: revision, DeletedAt: instant(deletedAt),
	}
}

func (t *triggerTx) GetFlowState(ctx context.Context, flowID flow.FlowID) (trigger.FlowState, error) {
	row, err := t.q.GetFlowState(ctx, t.orgID(), pgUUID(flowID))
	if err != nil {
		return trigger.FlowState{}, err
	}
	return stateFrom(flowID, row.Name, row.ProjectID, row.ActiveFlowVersionID, row.ActivationState, row.DraftRevision, row.DeletedAt), nil
}

func (t *triggerTx) LockFlowState(ctx context.Context, flowID flow.FlowID) (trigger.FlowState, error) {
	row, err := t.q.LockFlowState(ctx, t.orgID(), pgUUID(flowID))
	if err != nil {
		return trigger.FlowState{}, err
	}
	return stateFrom(flowID, row.Name, row.ProjectID, row.ActiveFlowVersionID, row.ActivationState, row.DraftRevision, row.DeletedAt), nil
}

func (t *triggerTx) NextVersionNumber(ctx context.Context, flowID flow.FlowID) (int64, error) {
	return t.q.GetNextFlowVersionNumber(ctx, t.orgID(), pgUUID(flowID))
}

func (t *triggerTx) InsertVersion(ctx context.Context, v flow.VersionRecord) error {
	params := sqlcgen.InsertFlowVersionParams{
		ID: pgUUID(v.ID), OrgID: t.orgID(), FlowID: pgUUID(v.FlowID),
		VersionNumber: v.Number, Document: v.Document, ContentHash: v.ContentHash,
		PublishedByPrincipalID: nullPGUUID(v.PublishedByPrincipalID),
		PublishedAt:            timestamp(v.PublishedAt), CreatedAt: timestamp(v.CreatedAt),
	}
	if v.Warnings != nil {
		params.WarningDocument = v.Warnings
		params.WarningDocumentFormat = pgtype.Text{String: flow.WarningsFormat, Valid: true}
	}
	if err := t.q.InsertFlowVersion(ctx, params); err != nil {
		return err
	}
	return insertVersionReferences(ctx, t.q, v)
}

func (t *triggerTx) SetActiveVersion(ctx context.Context, flowID flow.FlowID, version flow.FlowVersionID, now time.Time) error {
	n, err := t.q.UpdateFlowActiveVersion(ctx, sqlcgen.UpdateFlowActiveVersionParams{
		FlowVersionID: pgUUID(version), UpdatedAt: timestamp(now), OrgID: t.orgID(), ID: pgUUID(flowID),
	})
	return expectOne(n, err, "flow", flowID)
}

func (t *triggerTx) ReplaceDraft(ctx context.Context, flowID flow.FlowID, draft []byte, expectedRevision int64, references []flow.ConnectorReference, now time.Time) error {
	n, err := t.q.UpdateFlowDraft(ctx, sqlcgen.UpdateFlowDraftParams{
		DraftDocument: draft, UpdatedAt: timestamp(now), OrgID: t.orgID(), ID: pgUUID(flowID), ExpectedRevision: expectedRevision,
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
		ActivationState: string(state), UpdatedAt: timestamp(now), OrgID: t.orgID(), ID: pgUUID(flowID),
	})
	return expectOne(n, err, "flow", flowID)
}

func (t *triggerTx) SoftDeleteFlow(ctx context.Context, flowID flow.FlowID, by identity.PrincipalID, now time.Time) error {
	n, err := t.q.MarkFlowDeleted(ctx, sqlcgen.MarkFlowDeletedParams{
		DeletedAt: timestamp(now), DeletedByPrincipalID: nullPGUUID(by), UpdatedAt: timestamp(now), OrgID: t.orgID(), ID: pgUUID(flowID),
	})
	return expectOne(n, err, "flow", flowID)
}

func (t *triggerTx) RestoreFlow(ctx context.Context, flowID flow.FlowID, now time.Time) error {
	n, err := t.q.RestoreFlow(ctx, timestamp(now), t.orgID(), pgUUID(flowID))
	return expectOne(n, err, "flow", flowID)
}

func (t *triggerTx) PurgeFlow(ctx context.Context, flowID flow.FlowID) error {
	n, err := t.q.PurgeFlow(ctx, t.orgID(), pgUUID(flowID))
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

func triggerFrom(row *sqlcgen.FlowTrigger) trigger.Trigger {
	return trigger.Trigger{
		ID: fromPGUUID[trigger.ID](row.ID), OrgID: fromPGUUID[identity.OrgID](row.OrgID), FlowID: fromPGUUID[flow.FlowID](row.FlowID),
		NodeID: flow.NodeID(row.NodeID), Family: flow.TriggerFamily(row.Family), Type: row.Type,
		ConfigurationHash: row.ConfigurationHash, FlowVersionID: fromPGUUID[flow.FlowVersionID](row.FlowVersionID),
		Status: trigger.Status(row.Status), Health: trigger.Health(row.Health), LastError: row.LastError.String,
		CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
	}
}

func (t *triggerTx) ListTriggers(ctx context.Context, flowID flow.FlowID) ([]trigger.Trigger, error) {
	var out []trigger.Trigger
	var after pgtype.UUID
	after.Valid = true
	for {
		rows, err := t.q.ListFlowTriggers(ctx, sqlcgen.ListFlowTriggersParams{
			OrgID: t.orgID(), FlowID: pgUUID(flowID), AfterID: after, PageSize: maxPage,
		})
		if err != nil {
			return nil, err
		}
		for i := range rows {
			tr := triggerFrom(&rows[i])
			out = append(out, tr)
			after = pgUUID(tr.ID)
		}
		if len(rows) < maxPage {
			break
		}
	}
	slices.SortFunc(out, func(a, b trigger.Trigger) int { return strings.Compare(string(a.NodeID), string(b.NodeID)) })
	return out, nil
}

func (t *triggerTx) InsertTrigger(ctx context.Context, tr trigger.Trigger) error {
	return t.q.InsertFlowTrigger(ctx, sqlcgen.InsertFlowTriggerParams{
		ID: pgUUID(tr.ID), OrgID: t.orgID(), FlowID: pgUUID(tr.FlowID), NodeID: string(tr.NodeID),
		Family: string(tr.Family), Type: tr.Type, ConfigurationHash: tr.ConfigurationHash,
		FlowVersionID: pgUUID(tr.FlowVersionID), Status: string(tr.Status), Health: string(tr.Health),
		LastError: nullText(tr.LastError), CreatedAt: timestamp(tr.CreatedAt), UpdatedAt: timestamp(tr.UpdatedAt),
	})
}

func (t *triggerTx) UpdateTrigger(ctx context.Context, tr trigger.Trigger) error {
	n, err := t.q.UpdateFlowTrigger(ctx, sqlcgen.UpdateFlowTriggerParams{
		Family: string(tr.Family), Type: tr.Type, ConfigurationHash: tr.ConfigurationHash,
		FlowVersionID: pgUUID(tr.FlowVersionID), Status: string(tr.Status), Health: string(tr.Health),
		LastError: nullText(tr.LastError), UpdatedAt: timestamp(tr.UpdatedAt), OrgID: t.orgID(), ID: pgUUID(tr.ID),
	})
	return expectOne(n, err, "trigger", tr.ID)
}

func (t *triggerTx) SetTriggerStatuses(ctx context.Context, flowID flow.FlowID, status trigger.Status, now time.Time) error {
	return t.q.UpdateFlowTriggerStatuses(ctx, sqlcgen.UpdateFlowTriggerStatusesParams{
		Status: string(status), UpdatedAt: timestamp(now), OrgID: t.orgID(), FlowID: pgUUID(flowID),
	})
}

func (t *triggerTx) ListAddresses(ctx context.Context, id trigger.ID) ([]trigger.Address, error) {
	rows, err := t.q.ListTriggerAddresses(ctx, t.orgID(), pgUUID(id), addressPage)
	if err != nil {
		return nil, err
	}
	var out []trigger.Address
	index := map[[16]byte]int{}
	for i := range rows {
		r := &rows[i]
		at, seen := index[r.ID.Bytes]
		if !seen {
			out = append(out, addressFrom(r))
			at = len(out) - 1
			index[r.ID.Bytes] = at
		}
		if r.MethodRowID.Valid {
			out[at].Methods = append(out[at].Methods, trigger.AddressMethod{ID: fromPGUUID[trigger.MethodID](r.MethodRowID), Method: r.Method.String})
		}
	}
	return out, nil
}

func addressFrom(r *sqlcgen.ListTriggerAddressesRow) trigger.Address {
	return trigger.Address{
		ID: fromPGUUID[trigger.AddressID](r.ID), OrgID: fromPGUUID[identity.OrgID](r.OrgID), TriggerID: fromPGUUID[trigger.ID](r.FlowTriggerID),
		Family: flow.AddressFamily(r.Family), Scope: trigger.Scope(r.Scope), Pattern: r.Pattern, StaticPrefix: r.StaticPrefix,
		Host: r.Host.String, RandomSuffix: r.RandomSuffix.String, ResponseProfile: flow.ResponseProfile(r.ResponseProfile),
		Status: trigger.AddressStatus(r.Status), RetiredAt: instant(r.RetiredAt),
		CreatedAt: instant(r.CreatedAt), UpdatedAt: instant(r.UpdatedAt),
	}
}

func (t *triggerTx) InsertAddress(ctx context.Context, a *trigger.Address) error {
	err := t.q.InsertTriggerAddress(ctx, sqlcgen.InsertTriggerAddressParams{
		ID: pgUUID(a.ID), OrgID: t.orgID(), FlowTriggerID: pgUUID(a.TriggerID),
		Family: string(a.Family), Scope: string(a.Scope), Pattern: a.Pattern, StaticPrefix: a.StaticPrefix,
		Host: nullText(a.Host), RandomSuffix: nullText(a.RandomSuffix), ResponseProfile: string(a.ResponseProfile),
		Status: string(a.Status), RetiredAt: nullTimestamp(a.RetiredAt), CreatedAt: timestamp(a.CreatedAt), UpdatedAt: timestamp(a.UpdatedAt),
	})
	if err != nil {
		return err
	}
	return t.insertMethods(ctx, a.ID, a.Methods, a.CreatedAt)
}

func (t *triggerTx) insertMethods(ctx context.Context, address trigger.AddressID, methods []trigger.AddressMethod, now time.Time) error {
	for _, m := range methods {
		if err := t.q.InsertTriggerAddressMethod(ctx, sqlcgen.InsertTriggerAddressMethodParams{
			ID: pgUUID(m.ID), OrgID: t.orgID(), TriggerAddressID: pgUUID(address), Method: m.Method,
			CreatedAt: timestamp(now), UpdatedAt: timestamp(now),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (t *triggerTx) UpdateAddress(ctx context.Context, a *trigger.Address) error {
	n, err := t.q.UpdateTriggerAddress(ctx, sqlcgen.UpdateTriggerAddressParams{
		Family: string(a.Family), Host: nullText(a.Host), RandomSuffix: nullText(a.RandomSuffix),
		ResponseProfile: string(a.ResponseProfile), Status: string(a.Status), RetiredAt: nullTimestamp(a.RetiredAt),
		UpdatedAt: timestamp(a.UpdatedAt), OrgID: t.orgID(), ID: pgUUID(a.ID),
	})
	return expectOne(n, err, "address", a.ID)
}

func (t *triggerTx) ReplaceAddressMethods(ctx context.Context, address trigger.AddressID, methods []trigger.AddressMethod, now time.Time) error {
	if err := t.q.DeleteTriggerAddressMethods(ctx, t.orgID(), pgUUID(address)); err != nil {
		return err
	}
	return t.insertMethods(ctx, address, methods, now)
}

// LockRoute takes a transaction-scoped advisory lock on the path, so two
// publications in different organizations cannot both take it between the
// check and the commit (ADR-0022 section 3).
func (t *triggerTx) LockRoute(ctx context.Context, scope trigger.Scope, pattern string) error {
	return t.q.LockTriggerRoute(ctx, string(scope)+" "+pattern)
}

func (t *triggerTx) ListRoutes(ctx context.Context, scope trigger.Scope, after trigger.MethodID, limit int) ([]trigger.RoutePage, error) {
	rows, err := t.q.ListTriggerRoutes(ctx, sqlcgen.ListTriggerRoutesParams{
		OrgID: t.orgID(), Scope: string(scope), AfterID: pgUUID(after), PageSize: clampPage(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]trigger.RoutePage, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, trigger.RoutePage{
			MethodID: fromPGUUID[trigger.MethodID](r.MethodRowID),
			Route: trigger.Route{
				AddressID: fromPGUUID[trigger.AddressID](r.AddressID), TriggerID: fromPGUUID[trigger.ID](r.FlowTriggerID),
				FlowID: fromPGUUID[flow.FlowID](r.FlowID), FlowName: r.FlowName, NodeID: flow.NodeID(r.NodeID),
				Pattern: r.Pattern, Method: r.Method, Status: trigger.AddressStatus(r.Status), RetiredAt: instant(r.RetiredAt),
			},
		})
	}
	return out, nil
}

func (t *triggerTx) RouteTakenByOtherOrganization(ctx context.Context, scope trigger.Scope, pattern, method string, retiredAfter time.Time) (bool, error) {
	return t.q.ExistsRouteTakenByOtherOrganization(ctx, sqlcgen.ExistsRouteTakenByOtherOrganizationParams{
		Scope: string(scope), Pattern: pattern, Method: method, RetiredAfter: timestamp(retiredAfter),
	})
}

// NotifyRoutesChanged sends orcon_routes to the api processes that listen
// (ADR-0022 section 3). The notification is delivered when the transaction
// commits and not at all when it rolls back.
func (t *triggerTx) NotifyRoutesChanged(ctx context.Context, flowID flow.FlowID) error {
	return t.q.MarkTriggerRoutesChanged(ctx, flowID.String())
}

func (t *triggerTx) AppendEvent(ctx context.Context, e trigger.Event) error {
	return t.q.AppendTriggerEvent(ctx, sqlcgen.AppendTriggerEventParams{
		ID: pgUUID(e.ID), OrgID: t.orgID(), OccurredAt: timestamp(e.OccurredAt), FlowID: pgUUID(e.FlowID),
		NodeID: nullText(string(e.NodeID)), Type: e.Type, SchemaVersion: trigger.EventSchemaVersion,
		DataDocument: e.Data, CreatedAt: timestamp(e.OccurredAt),
	})
}

func (t *triggerTx) ListEvents(ctx context.Context, flowID flow.FlowID, after trigger.EventID, limit int) ([]trigger.Event, error) {
	rows, err := t.q.ListTriggerEvents(ctx, sqlcgen.ListTriggerEventsParams{
		OrgID: t.orgID(), FlowID: pgUUID(flowID), AfterID: pgUUID(after), PageSize: clampPage(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]trigger.Event, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		out = append(out, trigger.Event{
			ID: fromPGUUID[trigger.EventID](r.ID), OrgID: fromPGUUID[identity.OrgID](r.OrgID), OccurredAt: instant(r.OccurredAt),
			FlowID: flowID, NodeID: flow.NodeID(r.NodeID.String), Type: r.Type, Data: r.DataDocument,
		})
	}
	return out, nil
}

func (t *triggerTx) ResolveAcceptance(ctx context.Context, flowID flow.FlowID, node flow.NodeID) (trigger.Acceptance, error) {
	row, err := t.q.GetTriggerAcceptance(ctx, t.orgID(), pgUUID(flowID), string(node))
	if err != nil {
		return trigger.Acceptance{}, err
	}
	return trigger.Acceptance{
		TriggerID: fromPGUUID[trigger.ID](row.ID), Family: flow.TriggerFamily(row.Family), Type: row.Type,
		NodeID: flow.NodeID(row.NodeID), FlowVersionID: fromPGUUID[flow.FlowVersionID](row.FlowVersionID),
		VersionNumber: row.VersionNumber, ConfigurationHash: row.ConfigurationHash,
		Accepting: !row.DeletedAt.Valid && flow.ActivationState(row.ActivationState) == flow.ActivationActive &&
			trigger.Status(row.Status) == trigger.StatusActive,
	}, nil
}
