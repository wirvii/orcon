// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package triggertest holds what the tests of the trigger area share: an
// in-memory store ([Memory]), fakes for the catalog and the credential store,
// a builder of flow documents, and the scenario battery ([Battery]) that the
// in-memory store and the SQLite and PostgreSQL adapters must all pass.
package triggertest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/trigger"
)

// Factory returns a backend on a fresh, migrated database and registers its
// cleanup with t.
type Factory func(t *testing.T) Backend

// Start is the instant the fake clock of a scenario starts at: whole seconds,
// so stores that keep microseconds read back what was written.
var Start = time.Date(2026, 10, 13, 9, 30, 15, 0, time.UTC)

// env is one scenario: a backend, a service over it, and two organizations
// with a project each. The organizations are named twice: by identity, for the
// rows the scenario writes, and by scope, which every store method and every
// service command takes.
type env struct {
	t       *testing.T
	b       Backend
	clock   *clock.Fake
	ids     *id.Sequence
	creds   *Credentials
	starter *RecordingStarter
	svc     *trigger.Service
	orgA    identity.OrgID
	orgB    identity.OrgID
	scopeA  orgscope.Org
	scopeB  orgscope.Org
	projA   identity.ProjectID
	projB   identity.ProjectID
	user    identity.PrincipalID
}

func newEnv(t *testing.T, f Factory, settings trigger.Settings) *env {
	t.Helper()
	e := &env{t: t, b: f(t), clock: clock.NewFake(Start), ids: id.NewSequence(), creds: &Credentials{}, starter: &RecordingStarter{}}
	if e.b.Scope == nil {
		t.Fatal("the backend has no Scope: the battery cannot make the scopes of its organizations")
	}
	svc, err := trigger.New(trigger.Config{
		Store: e.b.Triggers, Flows: e.b.Flows, Catalog: DefaultCatalog(), Credentials: e.creds,
		Settings: settings, Starter: e.starter, Clock: e.clock, IDs: e.ids,
	})
	if err != nil {
		t.Fatalf("build the trigger service: %v", err)
	}
	e.svc = svc
	ctx := e.ctx()
	e.user = identity.PrincipalID(e.ids.New())
	must(t, "create the principal", e.b.Flows.CreatePrincipal(ctx, identity.PrincipalRecord{ID: e.user, Kind: identity.PrincipalUser, CreatedAt: Start, UpdatedAt: Start}))
	e.orgA, e.projA = e.organization("org-a")
	e.orgB, e.projB = e.organization("org-b")
	e.scopeA, e.scopeB = e.b.Scope(e.orgA), e.b.Scope(e.orgB)
	return e
}

func (e *env) ctx() context.Context { return e.t.Context() }

func (e *env) organization(slug string) (identity.OrgID, identity.ProjectID) {
	e.t.Helper()
	org := identity.OrgID(e.ids.New())
	must(e.t, "create organization "+slug, e.b.Flows.CreateOrganization(e.ctx(), identity.OrgRecord{
		ID: org, Name: slug, Slug: slug, Status: identity.OrgActive, CreatedAt: Start, UpdatedAt: Start,
	}))
	project := identity.ProjectID(e.ids.New())
	must(e.t, "create the project of "+slug, e.b.Flows.CreateProject(e.ctx(), e.b.Scope(org), identity.ProjectRecord{
		ID: project, OrgID: org, Name: "Alpha", Slug: "alpha", Environment: "development", IsProtected: true, CreatedAt: Start, UpdatedAt: Start,
	}, audittest.Org(e.t, id.UUID(org), "project.created")))
	return org, project
}

// newFlow creates a flow whose draft is the document.
func (e *env) newFlow(org orgscope.Org, project identity.ProjectID, name string, draft []byte) flow.FlowID {
	e.t.Helper()
	rec := flow.Record{ID: flow.FlowID(e.ids.New()), OrgID: identity.OrgID(org.UUID()), ProjectID: project, Name: name, Draft: draft, CreatedAt: Start, UpdatedAt: Start}
	must(e.t, "create flow "+name, e.b.Flows.CreateFlow(e.ctx(), org, rec))
	return rec.ID
}

// saveDraft replaces the draft at its current revision.
func (e *env) saveDraft(org orgscope.Org, flowID flow.FlowID, draft []byte) {
	e.t.Helper()
	rec, err := e.b.Flows.GetFlow(e.ctx(), org, flowID)
	must(e.t, "read the flow", err)
	_, err = e.b.Flows.UpdateFlowDraft(e.ctx(), org, flowID, draft, rec.DraftRevision, nil, e.clock.Now())
	must(e.t, "save the draft", err)
}

func (e *env) publish(org orgscope.Org, flowID flow.FlowID) trigger.Published {
	e.t.Helper()
	p, err := e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: org, Flow: flowID, By: e.user})
	must(e.t, "publish", err)
	return p
}

func (e *env) triggers(org orgscope.Org, flowID flow.FlowID) []trigger.Trigger {
	e.t.Helper()
	st, err := e.svc.Status(e.ctx(), org, flowID)
	must(e.t, "status", err)
	return st.Triggers
}

func (e *env) addresses(org orgscope.Org, id trigger.ID) []trigger.Address {
	e.t.Helper()
	var out []trigger.Address
	must(e.t, "list addresses", e.b.Triggers.Read(e.ctx(), org, func(ctx context.Context, tx trigger.Tx) error {
		var err error
		out, err = tx.ListAddresses(ctx, id)
		return err
	}))
	return out
}

// addressesByPattern indexes the addresses of an entry by their path.
func (e *env) addressesByPattern(org orgscope.Org, id trigger.ID) map[string]*trigger.Address {
	e.t.Helper()
	list := e.addresses(org, id)
	out := make(map[string]*trigger.Address, len(list))
	for i := range list {
		out[list[i].Pattern] = &list[i]
	}
	return out
}

func (e *env) events(org orgscope.Org, flowID flow.FlowID) []trigger.Event {
	e.t.Helper()
	var out []trigger.Event
	must(e.t, "list events", e.b.Triggers.Read(e.ctx(), org, func(ctx context.Context, tx trigger.Tx) error {
		var err error
		out, err = tx.ListEvents(ctx, flowID, trigger.EventID{}, 100)
		return err
	}))
	return out
}

func eventTypes(events []trigger.Event) []string {
	out := make([]string, len(events))
	for i := range events {
		out[i] = events[i].Type
	}
	return out
}

func must(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

func wantErr(t *testing.T, what string, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("%s: error %v, want %v", what, got, want)
	}
}

// validationIssue returns the first issue of the validation error in err with
// the code, or fails the test.
func validationIssue(t *testing.T, what string, err error, code string) flow.Issue {
	t.Helper()
	var ve *flow.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("%s: error %v, want a validation error with %s", what, err, code)
	}
	for _, i := range ve.Report.Issues {
		if i.Code == code && i.Severity == flow.SeverityError {
			return i
		}
	}
	t.Fatalf("%s: validation error without an error %s: %v", what, code, ve.Report.Issues)
	return flow.Issue{}
}

func contains(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
