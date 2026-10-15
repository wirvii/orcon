// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package costtest is the battery of the cost accounting (SPEC-006 AI2,
// criterion E-COST-2). The scenarios run against any [cost.Store]; the storetest
// package runs them against SQLite and PostgreSQL, so both engines give the same
// answers for budgets, rollups and alerts.
package costtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// Start is the instant every scenario runs at: mid-month, mid-day, in UTC.
var Start = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// Organizations creates the rows the cost tables reference: an organization,
// a project of it and a flow of that project. Execution scopes need a running
// flow and are covered by the agent battery of SPEC-006 AI3 instead.
type Organizations interface {
	CreateOrganization(ctx context.Context, rec flow.OrgRecord) error
	CreateProject(ctx context.Context, p flow.ProjectRecord) error
	CreateFlow(ctx context.Context, f flow.Record) error
}

// Backend is one store under test.
type Backend struct {
	Cost cost.Store
	Orgs Organizations
}

// Factory returns a fresh backend for one scenario.
type Factory func(t *testing.T) Backend

// env is one scenario's organization and project, with a deterministic id source.
type env struct {
	t       *testing.T
	ctx     context.Context //nolint:containedctx // scoped to one scenario, never stored beyond it
	b       Backend
	ids     *id.Sequence
	org     identity.OrgID
	project identity.ProjectID
}

func newEnv(t *testing.T, f Factory) *env {
	t.Helper()
	b := f(t)
	e := &env{t: t, ctx: t.Context(), b: b, ids: id.NewSequence()}
	e.org = identity.OrgID(e.ids.New())
	must(t, b.Orgs.CreateOrganization(e.ctx, flow.OrgRecord{
		ID: e.org, Name: "cost", Slug: "cost", Status: flow.OrgActive, CreatedAt: Start, UpdatedAt: Start,
	}))
	e.project = identity.ProjectID(e.ids.New())
	must(t, b.Orgs.CreateProject(e.ctx, flow.ProjectRecord{
		ID: e.project, OrgID: e.org, Name: "Alpha", Slug: "alpha", Environment: "development", IsProtected: true,
		CreatedAt: Start, UpdatedAt: Start,
	}))
	return e
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// scope returns a scope of the given kind with a fresh id. A flow scope gets a
// real flow of the project, so the foreign keys of the cost tables hold.
func (e *env) scope(kind cost.ScopeKind) cost.Scope {
	e.t.Helper()
	switch kind {
	case cost.ScopeOrg:
		return cost.Scope{Kind: kind, ID: id.UUID(e.org)}
	case cost.ScopeProject:
		return cost.Scope{Kind: kind, ID: id.UUID(e.project)}
	case cost.ScopeFlow:
		fid := e.ids.New()
		must(e.t, e.b.Orgs.CreateFlow(e.ctx, flow.Record{
			ID: flow.FlowID(fid), OrgID: e.org, ProjectID: e.project, Name: "flow " + fid.String(),
			Draft: []byte("{}"), CreatedAt: Start, UpdatedAt: Start,
		}))
		return cost.Scope{Kind: kind, ID: fid}
	case cost.ScopeExecution:
		e.t.Fatalf("scope %s is covered by the agent battery, not this one", kind)
		return cost.Scope{}
	default:
		e.t.Fatalf("scope %s is not used by this battery", kind)
		return cost.Scope{}
	}
}

func (e *env) budget(sc cost.Scope, period cost.Period, limit int64, exceeded cost.Action) cost.Budget {
	e.t.Helper()
	return cost.Budget{Scope: sc, Period: period, LimitMicro: limit, Exceeded: exceeded}
}

func (e *env) put(b cost.Budget) {
	e.t.Helper()
	if err := e.b.Cost.PutBudget(e.ctx, e.org, b, Start); err != nil {
		e.t.Fatalf("put budget: %v", err)
	}
}

// record adds one priced call of cost micro-dollars to the scopes.
func (e *env) record(scopes []cost.Scope, micro int64) []cost.Alert {
	e.t.Helper()
	alerts, err := e.b.Cost.RecordUsage(e.ctx, e.org, scopes, cost.Entry{
		Purpose: cost.PurposeAgent, Provider: "openai", Model: "gpt-x",
		Usage: cost.Usage{Input: 10, Output: 5}, CostMicro: micro, Priced: true, PriceVersion: "test",
	}, Start)
	if err != nil {
		e.t.Fatalf("record usage: %v", err)
	}
	return alerts
}

// admit runs the pre-call check for a call that would cost reservation.
func (e *env) admit(scopes []cost.Scope, reservation int64) (cost.Decision, error) {
	e.t.Helper()
	return cost.Admit(e.ctx, e.b.Cost, e.org, scopes, reservation, true, Start)
}

// BudgetsStrictestWins checks that when several budgets cover a call, the one
// closest to its limit decides, whatever its scope.
func BudgetsStrictestWins(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f)
	exec := e.scope(cost.ScopeFlow)
	org := e.scope(cost.ScopeOrg)
	e.put(e.budget(exec, cost.PeriodMonth, 500, cost.ActionFail))
	e.put(e.budget(org, cost.PeriodMonth, 1000, cost.ActionFail))
	scopes := []cost.Scope{exec, org}

	if _, err := e.admit(scopes, 400); err != nil {
		t.Fatalf("a call inside both limits was refused: %v", err)
	}
	var exceeded *cost.BudgetExceededError
	if _, err := e.admit(scopes, 600); !errors.As(err, &exceeded) || exceeded.Scope.Kind != cost.ScopeFlow {
		t.Fatalf("a call over the flow limit = %v, want the flow budget to refuse it", err)
	}

	e.record(scopes, 450)
	if _, err := e.admit(scopes, 100); !errors.As(err, &exceeded) || exceeded.Scope.Kind != cost.ScopeFlow {
		t.Fatalf("after 450 spent, a 100 call = %v, want the 500 flow budget to refuse it", err)
	}
	if _, err := e.admit([]cost.Scope{org}, 100); err != nil {
		t.Errorf("the organization alone still has room: %v", err)
	}
}

// BudgetFailAndSuspend checks that a FAIL budget refuses the call with
// AiBudgetExceeded, and that a SUSPEND budget suspends the run until the limit
// is raised.
func BudgetFailAndSuspend(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f)
	failing := e.scope(cost.ScopeProject)
	suspending := e.scope(cost.ScopeFlow)
	e.put(e.budget(failing, cost.PeriodMonth, 100, cost.ActionFail))
	e.put(e.budget(suspending, cost.PeriodMonth, 100, cost.ActionSuspend))

	var exceeded *cost.BudgetExceededError
	if _, err := e.admit([]cost.Scope{failing}, 101); !errors.As(err, &exceeded) {
		t.Fatalf("FAIL budget over its limit = %v, want a BudgetExceededError", err)
	}
	d, err := e.admit([]cost.Scope{suspending}, 101)
	if err != nil || !d.Suspend {
		t.Fatalf("SUSPEND budget over its limit = %+v, %v; want the run suspended", d, err)
	}

	e.put(e.budget(suspending, cost.PeriodMonth, 1000, cost.ActionSuspend))
	if d, err := e.admit([]cost.Scope{suspending}, 101); err != nil || d.Suspend {
		t.Errorf("after raising the limit = %+v, %v; want the run to resume", d, err)
	}
}

// RollupsInSameTransaction checks that a recorded call is visible in every
// scope's spending at once, and that a budget read after it sees the same total.
func RollupsInSameTransaction(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f)
	project := e.scope(cost.ScopeProject)
	org := e.scope(cost.ScopeOrg)
	e.put(e.budget(project, cost.PeriodMonth, 10_000, cost.ActionFail))
	e.put(e.budget(org, cost.PeriodTotal, 10_000, cost.ActionFail))

	e.record([]cost.Scope{project, org}, 300)
	e.record([]cost.Scope{project, org}, 200)

	for _, sc := range []cost.Scope{project, org} {
		states, err := e.b.Cost.BudgetStates(e.ctx, e.org, []cost.Scope{sc}, Start)
		if err != nil || len(states) != 1 {
			t.Fatalf("budget states of %s = %v, %v", sc.Kind, states, err)
		}
		if states[0].SpentMicro != 500 {
			t.Errorf("%s spent %d micro-dollars after two calls, want 500", sc.Kind, states[0].SpentMicro)
		}
	}
}

// BudgetAlertOncePerPeriod checks that each threshold of a budget alerts once
// per period: the threshold alert fires when crossed, the limit alert fires once
// when reached, and repeated calls over the same thresholds alert nothing new.
func BudgetAlertOncePerPeriod(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f)
	sc := e.scope(cost.ScopeProject)
	e.put(e.budget(sc, cost.PeriodMonth, 1000, cost.ActionFail))
	scopes := []cost.Scope{sc}

	if a := e.record(scopes, 700); len(a) != 0 {
		t.Fatalf("70%% spent raised alerts %v, want none", a)
	}
	first := e.record(scopes, 150)
	if len(first) != 1 || first[0].Kind != cost.AlertThreshold || first[0].ThresholdPct != cost.DefaultAlertPercent {
		t.Fatalf("crossing 80%% raised %v, want one threshold alert", first)
	}
	if again := e.record(scopes, 10); len(again) != 0 {
		t.Errorf("a repeat over the same threshold raised %v, want none", again)
	}
	reached := e.record(scopes, 200)
	if len(reached) != 1 || reached[0].Kind != cost.AlertExceeded {
		t.Fatalf("reaching the limit raised %v, want one exceeded alert", reached)
	}
	if again := e.record(scopes, 5); len(again) != 0 {
		t.Errorf("a call past the limit raised %v, want no repeat", again)
	}
}
