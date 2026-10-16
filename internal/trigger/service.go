// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Config holds the dependencies of a [Service].
type Config struct {
	// Store runs the lifecycle transactions.
	Store Store
	// Flows reads flows, versions and projects and stores snapshots.
	Flows FlowSource
	// Catalog tells what trigger node types declare (rule T-V8).
	Catalog Catalog
	// Credentials tells which credentials a trigger lacks (rule T-V3).
	Credentials Credentials
	// NodeTypes lets the flow validation know the node types; flow.Validate
	// without a catalog when nil.
	NodeTypes flow.NodeTypes
	// Settings are the instance settings the rules read.
	Settings Settings
	// Starter starts manual runs; [Service.RunManually] fails without it.
	Starter Starter
	// Clock supplies the instants the service writes; required.
	Clock clock.Clock
	// IDs generates the identities of new rows; required.
	IDs id.Generator
}

// Service runs the trigger lifecycle: publish, activate, deactivate, roll
// back, delete, restore and purge, plus the acceptance read and the manual
// run. It is safe for concurrent use.
type Service struct {
	store       Store
	flows       FlowSource
	catalog     Catalog
	credentials Credentials
	nodeTypes   flow.NodeTypes
	settings    Settings
	starter     Starter
	clock       clock.Clock
	ids         id.Generator
}

// New builds a service. Store, Flows, Catalog, Credentials, Clock and IDs are
// required.
func New(cfg Config) (*Service, error) {
	switch {
	case cfg.Store == nil:
		return nil, &ConfigError{Field: "Store"}
	case cfg.Flows == nil:
		return nil, &ConfigError{Field: "Flows"}
	case cfg.Catalog == nil:
		return nil, &ConfigError{Field: "Catalog"}
	case cfg.Credentials == nil:
		return nil, &ConfigError{Field: "Credentials"}
	case cfg.Clock == nil:
		return nil, &ConfigError{Field: "Clock"}
	case cfg.IDs == nil:
		return nil, &ConfigError{Field: "IDs"}
	}
	return &Service{
		store: cfg.Store, flows: cfg.Flows, catalog: cfg.Catalog, credentials: cfg.Credentials,
		nodeTypes: cfg.NodeTypes, settings: cfg.Settings, starter: cfg.Starter, clock: cfg.Clock, ids: cfg.IDs,
	}, nil
}

// PublishCommand asks to publish the draft of a flow.
type PublishCommand struct {
	Org  orgscope.Org
	Flow flow.FlowID
	By   identity.PrincipalID
	// ExpectedDraftRevision, when set, makes the publication fail with
	// flow.ErrDraftConflict unless the draft is still at that revision.
	ExpectedDraftRevision *int64
	// Actor is the actor of the audit entry flow.published; the zero Actor is an anonymous caller.
	Actor audit.Actor
	// Note is the release note recorded with the version (at most 500 characters); empty for none.
	Note string
	// AllowImportedAddressChange confirms, for rule T-V4, that the author means
	// to remove or change an address imported from n8n.
	AllowImportedAddressChange bool
}

// RollbackCommand asks to republish the content of an earlier version as the
// next one (R-ACT-04).
type RollbackCommand struct {
	Org  orgscope.Org
	Flow flow.FlowID
	By   identity.PrincipalID
	// ToVersion is the number of the version whose content is republished.
	ToVersion int64
	// ExpectedRevision, when set, makes the rollback fail with flow.ErrRevisionConflict unless the
	// flow row is still at that revision (the ETag of the flow).
	ExpectedRevision *int64
	// Actor is the actor of the audit entry flow.rolled_back; the zero Actor is an anonymous caller.
	Actor audit.Actor
	// Note is the release note recorded with the version (at most 500 characters); empty for none.
	Note string
	// AllowImportedAddressChange is the T-V4 confirmation, as in [PublishCommand].
	AllowImportedAddressChange bool
}

// Published is the outcome of a publication.
type Published struct {
	// Version is the new version.
	Version flow.VersionRecord
	// Report holds the warnings recorded with the version.
	Report flow.Report
	// Live is true when the version is live: the flow was active, so its
	// triggers now accept events on this version.
	Live bool
}

// Publish validates the draft of the flow with the rules of the flow area and
// T-V1..T-V9, and publishes it as the next version. In one transaction it
// inserts the version, points the flow at it, rebuilds the registry of
// triggers and addresses and records the event. Whether the version is live
// depends on the activation state of the flow, which publishing never changes
// (R-ACT-02).
func (s *Service) Publish(ctx context.Context, cmd PublishCommand) (Published, error) {
	rec, err := s.flows.GetFlow(ctx, cmd.Org, cmd.Flow)
	if err != nil {
		return Published{}, fmt.Errorf("publish flow %s: %w", cmd.Flow, err)
	}
	if cmd.ExpectedDraftRevision != nil && *cmd.ExpectedDraftRevision != rec.DraftRevision {
		return Published{}, fmt.Errorf("publish flow %s at draft revision %d: %w", cmd.Flow, *cmd.ExpectedDraftRevision, flow.ErrDraftConflict)
	}
	draft, err := jsonvalue.Parse(rec.Draft)
	if err != nil {
		return Published{}, fmt.Errorf("publish flow %s: the stored draft is not JSON: %w", cmd.Flow, err)
	}
	return s.publish(ctx, &publication{
		org: cmd.Org, flowID: cmd.Flow, by: cmd.By, rec: rec, content: draft,
		draftRevision: rec.DraftRevision, allowImported: cmd.AllowImportedAddressChange,
		event: EventFlowPublished, actor: cmd.Actor, note: cmd.Note,
		guardActive: cmd.ExpectedDraftRevision != nil, activeAtRead: rec.ActiveVersionID,
	})
}

// Rollback republishes the content of version ToVersion as the next version
// and makes the draft that content, so the editor shows what is live. It goes
// through the same validation and the same transaction as [Service.Publish].
func (s *Service) Rollback(ctx context.Context, cmd RollbackCommand) (Published, error) {
	rec, err := s.flows.GetFlow(ctx, cmd.Org, cmd.Flow)
	if err != nil {
		return Published{}, fmt.Errorf("roll back flow %s: %w", cmd.Flow, err)
	}
	old, err := s.flows.GetVersionByNumber(ctx, cmd.Org, cmd.Flow, cmd.ToVersion)
	if err != nil {
		return Published{}, fmt.Errorf("roll back flow %s to version %d: %w", cmd.Flow, cmd.ToVersion, err)
	}
	content, err := jsonvalue.Parse(old.Document)
	if err != nil {
		return Published{}, fmt.Errorf("roll back flow %s to version %d: the stored version is not JSON: %w", cmd.Flow, cmd.ToVersion, err)
	}
	return s.publish(ctx, &publication{
		org: cmd.Org, flowID: cmd.Flow, by: cmd.By, rec: rec, content: content,
		draftRevision: rec.DraftRevision, allowImported: cmd.AllowImportedAddressChange,
		event: EventFlowRolledBack, rolledBackTo: cmd.ToVersion, replaceDraft: true,
		expectedRevision: cmd.ExpectedRevision, actor: cmd.Actor, note: cmd.Note,
	})
}

// publication is one publication in progress.
type publication struct {
	org           orgscope.Org
	flowID        flow.FlowID
	by            identity.PrincipalID
	rec           flow.Record
	content       jsonvalue.Value
	draftRevision int64
	allowImported bool
	event         string
	rolledBackTo  int64
	// replaceDraft makes the content the new draft, in the same transaction.
	replaceDraft bool
	// expectedRevision, when set, is the revision the flow row must be at (rollback).
	expectedRevision *int64
	actor            audit.Actor
	note             string
	// guardActive makes the publication fail with flow.ErrDraftConflict when another publication
	// changed the live version after the flow was read: a conditional publication is consumed by
	// the first one that lands, so two requests with the same If-Match never make two versions.
	guardActive  bool
	activeAtRead flow.FlowVersionID
}

// prepared is a draft that passed every check that needs no transaction.
type prepared struct {
	doc      flow.Document
	report   flow.Report
	hash     flow.Hash
	document []byte
	specs    []Spec
}

// prepare reads the document and runs the validation that needs no
// transaction: the flow rules and T-V2..T-V9.
func (s *Service) prepare(ctx context.Context, p *publication) (*prepared, error) {
	doc, err := flow.Read(p.content)
	if err != nil {
		return nil, err
	}
	project, err := s.flows.GetProject(ctx, p.org, p.rec.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("read the project: %w", err)
	}
	rc, err := s.ruleContext(ctx, p.org, p.rec, p.allowImported)
	if err != nil {
		return nil, err
	}
	report, err := s.documentReport(ctx, &doc, rc)
	if err != nil {
		return nil, err
	}
	if err := report.Err(); err != nil {
		return nil, err
	}
	normalized := doc.Value()
	hash, err := flow.ContentHashOfValue(normalized)
	if err != nil {
		return nil, err
	}
	document, err := jsonvalue.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode the document: %w", err)
	}
	specs, err := BuildSpecs(&doc, project.Slug)
	if err != nil {
		return nil, err
	}
	return &prepared{doc: doc, report: report, hash: hash, document: document, specs: specs}, nil
}

func (s *Service) publish(ctx context.Context, p *publication) (Published, error) {
	pre, err := s.prepare(ctx, p)
	if err != nil {
		return Published{}, fmt.Errorf("publish flow %s: %w", p.flowID, err)
	}
	var out Published
	err = s.store.Atomic(ctx, p.org, func(ctx context.Context, tx Tx) error {
		var terr error
		out, terr = s.publishTx(ctx, tx, p, pre)
		return terr
	})
	if err != nil {
		return Published{}, fmt.Errorf("publish flow %s: %w", p.flowID, err)
	}
	return out, nil
}

// publishTx is the publication transaction (ADR-0024 section 2): it checks the
// paths, inserts the version, points the flow at it, rebuilds the registry and
// records the event, all or nothing.
func (s *Service) publishTx(ctx context.Context, tx Tx, p *publication, pre *prepared) (Published, error) {
	now := s.clock.Now().UTC()
	state, err := tx.LockFlowState(ctx, p.flowID)
	if err != nil {
		return Published{}, err
	}
	if state.IsDeleted() {
		return Published{}, fmt.Errorf("flow %s: %w", p.flowID, flow.ErrNotFound)
	}
	if p.expectedRevision != nil && *p.expectedRevision != state.Revision {
		return Published{}, fmt.Errorf("flow %s at revision %d: %w", p.flowID, *p.expectedRevision, flow.ErrRevisionConflict)
	}
	if state.DraftRevision != p.draftRevision || (p.guardActive && state.ActiveVersionID != p.activeAtRead) {
		return Published{}, fmt.Errorf("flow %s: %w", p.flowID, flow.ErrDraftConflict)
	}
	routeIssues, err := s.routeIssues(ctx, tx, &routeCheck{flowID: p.flowID, doc: &pre.doc, specs: pre.specs, now: now, lock: true})
	if err != nil {
		return Published{}, err
	}
	full := mergeReports(pre.report, routeIssues)
	if err := full.Err(); err != nil {
		return Published{}, err
	}
	version, err := s.insertVersion(ctx, tx, p, pre, &full, now)
	if err != nil {
		return Published{}, err
	}
	if p.replaceDraft {
		if err := tx.ReplaceDraft(ctx, p.flowID, pre.document, p.draftRevision, s.connectorReferences(&pre.doc), now); err != nil {
			return Published{}, err
		}
	}
	if err := s.syncRegistry(ctx, tx, p.org, p.flowID, state.ActivationState, version.ID, pre.specs, now); err != nil {
		return Published{}, err
	}
	live := state.ActivationState == flow.ActivationActive
	data := map[string]jsonvalue.Value{
		"flowId":        jsonvalue.String(p.flowID.String()),
		"versionNumber": jsonvalue.Int(version.Number),
		"versionId":     jsonvalue.String(version.ID.String()),
		"live":          jsonvalue.Bool(live),
	}
	if p.event == EventFlowRolledBack {
		data["rolledBackToVersion"] = jsonvalue.Int(p.rolledBackTo)
	}
	if err := s.appendEvent(ctx, tx, p.org, p.flowID, "", p.event, now, data); err != nil {
		return Published{}, err
	}
	changed := &changeRecord{org: p.org, actor: p.actor, action: "flow.published", flowID: p.flowID, now: now, version: version.Number, contentHash: version.ContentHash}
	if p.event == EventFlowRolledBack {
		changed.action, changed.rolledBackFrom = "flow.rolled_back", p.rolledBackTo
	}
	if err := s.recordChange(ctx, tx, changed); err != nil {
		return Published{}, err
	}
	if err := tx.NotifyRoutesChanged(ctx, p.flowID); err != nil {
		return Published{}, err
	}
	return Published{Version: version, Report: full, Live: live}, nil
}

// changeRecord describes the audit entry of a change of a flow.
type changeRecord struct {
	org    orgscope.Org
	actor  audit.Actor
	action string
	flowID flow.FlowID
	now    time.Time
	// version and contentHash describe a publication or rollback; zero for the other changes.
	version        int64
	contentHash    string
	rolledBackFrom int64
}

// recordChange writes the audit entry of a change, and the idempotency claim of the request, in
// the transaction of the change (SPEC-004 R-AUD-02).
func (s *Service) recordChange(ctx context.Context, tx Tx, c *changeRecord) error {
	members := []jsonvalue.Member{{Key: "flowId", Value: jsonvalue.String(c.flowID.String())}}
	if c.version > 0 {
		members = append(members,
			jsonvalue.Member{Key: "version", Value: jsonvalue.Int(c.version)},
			jsonvalue.Member{Key: "contentHash", Value: jsonvalue.String(c.contentHash)})
	}
	if c.rolledBackFrom > 0 {
		members = append(members, jsonvalue.Member{Key: "rolledBackFromVersion", Value: jsonvalue.Int(c.rolledBackFrom)})
	}
	details, err := jsonvalue.Object(members...)
	if err != nil {
		return fmt.Errorf("audit %s: %w", c.action, err)
	}
	entry, err := audit.NewEntry(audit.EntryParams{
		ID: s.ids.New(), Chain: audit.OrganizationChain(id.UUID(identity.OrgIDOf(c.org))), OccurredAt: c.now, Actor: c.actor,
		Action: c.action, Outcome: audit.OutcomeSuccess, Details: details,
	})
	if err != nil {
		return fmt.Errorf("audit %s: %w", c.action, err)
	}
	return tx.RecordChange(ctx, entry, "flow:"+c.flowID.String(), c.now)
}

// insertVersion inserts the next version of the flow and makes it the one the
// flow points at.
func (s *Service) insertVersion(ctx context.Context, tx Tx, p *publication, pre *prepared, report *flow.Report, now time.Time) (flow.VersionRecord, error) {
	number, err := tx.NextVersionNumber(ctx, p.flowID)
	if err != nil {
		return flow.VersionRecord{}, err
	}
	version := flow.VersionRecord{
		ID: flow.FlowVersionID(s.ids.New()), OrgID: identity.OrgID(p.org.UUID()), FlowID: p.flowID, Number: number,
		Document: pre.document, ContentHash: pre.hash.String(),
		PublishedByPrincipalID: p.by, PublishedAt: now, CreatedAt: now,
		ConnectorReferences: s.connectorReferences(&pre.doc),
		Note:                p.note, RolledBackFromVersion: p.rolledBackTo,
	}
	if w := report.WarningsValue(); !w.IsZero() {
		if version.Warnings, err = jsonvalue.Marshal(w); err != nil {
			return flow.VersionRecord{}, fmt.Errorf("encode the warnings: %w", err)
		}
	}
	if err := tx.InsertVersion(ctx, version); err != nil {
		return flow.VersionRecord{}, err
	}
	if err := tx.SetActiveVersion(ctx, p.flowID, version.ID, now); err != nil {
		return flow.VersionRecord{}, err
	}
	return version, nil
}

// connectorReferences lists the connector versions the document pins, as the
// rows the store records with a version or a draft (ADR-0034 section 6).
func (s *Service) connectorReferences(doc *flow.Document) []flow.ConnectorReference {
	var refs []flow.ConnectorReference
	for _, use := range flow.ConnectorUses(doc) {
		refs = append(refs, flow.ConnectorReference{
			ID: flow.ConnectorReferenceID(s.ids.New()), ConnectorID: use.ConnectorID, ConnectorVersion: use.ConnectorVersion,
		})
	}
	return refs
}

func mergeReports(r flow.Report, extra []flow.Issue) flow.Report {
	if len(extra) == 0 {
		return r
	}
	return flow.Report{Issues: append(append([]flow.Issue{}, r.Issues...), extra...)}
}

// ruleContext reads what rule T-V4 compares the draft with: the live version.
func (s *Service) ruleContext(ctx context.Context, org orgscope.Org, rec flow.Record, allowImported bool) (*ruleContext, error) {
	rc := &ruleContext{org: org, project: rec.ProjectID, allowImportedAddressChange: allowImported}
	if rec.ActiveVersionID.IsZero() {
		return rc, nil
	}
	v, err := s.flows.GetVersion(ctx, org, rec.ActiveVersionID)
	if err != nil {
		return nil, fmt.Errorf("read the live version: %w", err)
	}
	value, err := jsonvalue.Parse(v.Document)
	if err != nil {
		return nil, fmt.Errorf("read the live version: %w", err)
	}
	prev, err := flow.FromValue(value)
	if err != nil {
		return nil, fmt.Errorf("read the live version: %w", err)
	}
	rc.previous = &prev
	return rc, nil
}

// documentReport validates a document with the rules of the flow area and the
// document-level trigger rules.
func (s *Service) documentReport(ctx context.Context, doc *flow.Document, rc *ruleContext) (flow.Report, error) {
	var report flow.Report
	if s.nodeTypes != nil {
		report = flow.ValidateWithTypes(ctx, doc, s.nodeTypes)
	} else {
		report = flow.Validate(doc)
	}
	issues, err := s.documentIssues(ctx, doc, rc)
	if err != nil {
		return flow.Report{}, err
	}
	return mergeReports(report, issues), nil
}

// Validate runs, without publishing, every check publication would run on the
// draft, including whether its addresses are free. The report is the answer;
// the error is only for a failure to check.
func (s *Service) Validate(ctx context.Context, org orgscope.Org, flowID flow.FlowID) (flow.Report, error) {
	rec, err := s.flows.GetFlow(ctx, org, flowID)
	if err != nil {
		return flow.Report{}, fmt.Errorf("validate flow %s: %w", flowID, err)
	}
	value, err := jsonvalue.Parse(rec.Draft)
	if err != nil {
		return flow.Report{}, fmt.Errorf("validate flow %s: the stored draft is not JSON: %w", flowID, err)
	}
	doc, err := flow.FromValue(value)
	if err != nil {
		var ve *flow.ValidationError
		if errors.As(err, &ve) {
			return ve.Report, nil
		}
		return flow.Report{}, fmt.Errorf("validate flow %s: %w", flowID, err)
	}
	project, err := s.flows.GetProject(ctx, org, rec.ProjectID)
	if err != nil {
		return flow.Report{}, fmt.Errorf("validate flow %s: read the project: %w", flowID, err)
	}
	rc, err := s.ruleContext(ctx, org, rec, false)
	if err != nil {
		return flow.Report{}, fmt.Errorf("validate flow %s: %w", flowID, err)
	}
	report, err := s.documentReport(ctx, &doc, rc)
	if err != nil {
		return flow.Report{}, fmt.Errorf("validate flow %s: %w", flowID, err)
	}
	if report.HasErrors() {
		return report, nil
	}
	specs, err := BuildSpecs(&doc, project.Slug)
	if err != nil {
		return flow.Report{}, fmt.Errorf("validate flow %s: %w", flowID, err)
	}
	var extra []flow.Issue
	err = s.store.Read(ctx, org, func(ctx context.Context, tx Tx) error {
		var rerr error
		extra, rerr = s.routeIssues(ctx, tx, &routeCheck{flowID: flowID, doc: &doc, specs: specs, now: s.clock.Now().UTC()})
		return rerr
	})
	if err != nil {
		return flow.Report{}, fmt.Errorf("validate flow %s: %w", flowID, err)
	}
	return mergeReports(report, extra), nil
}

// syncRegistry brings the registry entries of the flow in line with the specs
// of the version just published (ADR-0024 sections 2 and 3).
func (s *Service) syncRegistry(ctx context.Context, tx Tx, org orgscope.Org, flowID flow.FlowID, activation flow.ActivationState, version flow.FlowVersionID, specs []Spec, now time.Time) error {
	existing, err := tx.ListTriggers(ctx, flowID)
	if err != nil {
		return err
	}
	byNode := make(map[flow.NodeID]int, len(existing))
	for i := range existing {
		byNode[existing[i].NodeID] = i
	}
	status := StatusInactive
	if activation == flow.ActivationActive {
		status = StatusActive
	}
	wanted := make(map[flow.NodeID]bool, len(specs))
	for i := range specs {
		spec := &specs[i]
		wanted[spec.NodeID] = true
		var t *Trigger
		if at, ok := byNode[spec.NodeID]; ok {
			t = &existing[at]
		}
		t, err := s.upsertTrigger(ctx, tx, &triggerWrite{org: org, flowID: flowID, version: version, status: status, now: now}, t, spec)
		if err != nil {
			return err
		}
		if err := s.syncAddresses(ctx, tx, org, t, spec.Addresses, now); err != nil {
			return err
		}
	}
	for i := range existing {
		t := &existing[i]
		if wanted[t.NodeID] || t.Status == StatusRemoved {
			continue
		}
		t.Status, t.UpdatedAt = StatusRemoved, now
		if err := tx.UpdateTrigger(ctx, *t); err != nil {
			return err
		}
		if err := s.syncAddresses(ctx, tx, org, t, nil, now); err != nil {
			return err
		}
	}
	return nil
}

// triggerWrite carries what every entry written by one publication shares.
type triggerWrite struct {
	org     orgscope.Org
	flowID  flow.FlowID
	version flow.FlowVersionID
	status  Status
	now     time.Time
}

// upsertTrigger inserts the entry of a node, or points the stored one at the
// new version. The health of an entry whose configuration changed starts over.
func (s *Service) upsertTrigger(ctx context.Context, tx Tx, w *triggerWrite, have *Trigger, spec *Spec) (*Trigger, error) {
	if have == nil {
		t := Trigger{
			ID: ID(s.ids.New()), OrgID: identity.OrgID(w.org.UUID()), FlowID: w.flowID, NodeID: spec.NodeID,
			Family: spec.Family, Type: spec.Type, ConfigurationHash: spec.ConfigurationHash,
			FlowVersionID: w.version, Status: w.status, Health: HealthOK, CreatedAt: w.now, UpdatedAt: w.now,
		}
		return &t, tx.InsertTrigger(ctx, t)
	}
	if have.ConfigurationHash != spec.ConfigurationHash {
		have.Health, have.LastError = HealthOK, ""
	}
	have.Family, have.Type, have.ConfigurationHash = spec.Family, spec.Type, spec.ConfigurationHash
	have.FlowVersionID, have.Status, have.UpdatedAt = w.version, w.status, w.now
	return have, tx.UpdateTrigger(ctx, *have)
}

// syncAddresses makes the production addresses of an entry the desired ones:
// new ones are inserted, kept ones are left alone, changed ones are updated,
// and the ones no longer wanted are retired with a grace period.
func (s *Service) syncAddresses(ctx context.Context, tx Tx, org orgscope.Org, t *Trigger, desired []AddressSpec, now time.Time) error {
	current, err := tx.ListAddresses(ctx, t.ID)
	if err != nil {
		return err
	}
	byPattern := map[string]*Address{}
	for i := range current {
		if current[i].Scope == ScopeProduction {
			byPattern[current[i].Pattern] = &current[i]
		}
	}
	wanted := map[string]bool{}
	for i := range desired {
		wanted[desired[i].Pattern] = true
		if err := s.reconcileAddress(ctx, tx, org, t, byPattern[desired[i].Pattern], &desired[i], now); err != nil {
			return err
		}
	}
	for pattern, a := range byPattern {
		if wanted[pattern] || a.Status == AddressRetired {
			continue
		}
		a.Status, a.RetiredAt, a.UpdatedAt = AddressRetired, now, now
		if err := tx.UpdateAddress(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

// reconcileAddress inserts the wanted address when there is none, and
// otherwise brings the stored one in line with it.
func (s *Service) reconcileAddress(ctx context.Context, tx Tx, org orgscope.Org, t *Trigger, have *Address, want *AddressSpec, now time.Time) error {
	if have == nil {
		a := Address{
			ID: AddressID(s.ids.New()), OrgID: identity.OrgID(org.UUID()), TriggerID: t.ID, Family: want.Family, Scope: ScopeProduction,
			Pattern: want.Pattern, StaticPrefix: want.StaticPrefix, RandomSuffix: want.RandomSuffix,
			ResponseProfile: want.ResponseProfile, Status: AddressActive, CreatedAt: now, UpdatedAt: now,
			Methods: s.newMethods(want.Methods),
		}
		return tx.InsertAddress(ctx, &a)
	}
	if have.Status != AddressActive || have.Family != want.Family || have.ResponseProfile != want.ResponseProfile || have.RandomSuffix != want.RandomSuffix {
		have.Status, have.RetiredAt, have.Family = AddressActive, time.Time{}, want.Family
		have.ResponseProfile, have.RandomSuffix, have.UpdatedAt = want.ResponseProfile, want.RandomSuffix, now
		if err := tx.UpdateAddress(ctx, have); err != nil {
			return err
		}
	}
	if !sameStrings(have.MethodNames(), want.Methods) {
		return tx.ReplaceAddressMethods(ctx, have.ID, s.newMethods(want.Methods), now)
	}
	return nil
}

func (s *Service) newMethods(names []string) []AddressMethod {
	out := make([]AddressMethod, len(names))
	for i, m := range names {
		out[i] = AddressMethod{ID: MethodID(s.ids.New()), Method: m}
	}
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// appendEvent writes one event of the trigger stream; the members of its data
// document come from data, and the id and the instant are the service's.
func (s *Service) appendEvent(ctx context.Context, tx Tx, org orgscope.Org, flowID flow.FlowID, node flow.NodeID, eventType string, now time.Time, data map[string]jsonvalue.Value) error {
	members := make([]jsonvalue.Member, 0, len(data))
	for _, key := range sortedKeys(data) {
		members = append(members, jsonvalue.Member{Key: key, Value: data[key]})
	}
	doc, err := jsonvalue.Object(members...)
	if err != nil {
		return fmt.Errorf("build the data of event %s: %w", eventType, err)
	}
	raw, err := jsonvalue.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode the data of event %s: %w", eventType, err)
	}
	return tx.AppendEvent(ctx, Event{
		ID: EventID(s.ids.New()), OrgID: identity.OrgID(org.UUID()), OccurredAt: now, FlowID: flowID, NodeID: node, Type: eventType, Data: raw,
	})
}
