// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cost_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

var orgScope = cost.Scope{Kind: cost.ScopeOrg, ID: id.Derive("org")}

// TestBudgetValidation: a budget needs a positive limit, an alert share inside 1..99 when it names one, and
// a PER_EXECUTION period only on an execution.
func TestBudgetValidation(t *testing.T) {
	t.Parallel()
	good := cost.Budget{Scope: orgScope, Period: cost.PeriodMonth, LimitMicro: 1000, AlertPercent: 80}
	if err := good.Validate(); err != nil {
		t.Fatalf("a valid budget was refused: %v", err)
	}
	for name, mutate := range map[string]func(*cost.Budget){
		"no limit":                func(b *cost.Budget) { b.LimitMicro = 0 },
		"alert share of 100":      func(b *cost.Budget) { b.AlertPercent = 100 },
		"alert share below 1":     func(b *cost.Budget) { b.AlertPercent = -1 },
		"per execution on an org": func(b *cost.Budget) { b.Period = cost.PeriodPerExecution },
	} {
		b := good
		mutate(&b)
		if err := b.Validate(); err == nil {
			t.Errorf("%s: the budget was accepted", name)
		}
	}
	unset := cost.Budget{Scope: orgScope, Period: cost.PeriodMonth, LimitMicro: 1}
	if err := unset.Validate(); err != nil {
		t.Errorf("a budget without an alert share was refused: %v", err)
	}
	if err := (cost.Budget{Scope: orgScope, Period: "WEEK", LimitMicro: 1, AlertPercent: 50}).Validate(); err == nil {
		t.Error("an unknown period was accepted")
	}
}

// TestBudgetDefaults: the optional fields default to an 80 percent alert, FAIL and ALLOW, and a set field stays.
func TestBudgetDefaults(t *testing.T) {
	t.Parallel()
	got := (cost.Budget{Scope: orgScope, Period: cost.PeriodDay, LimitMicro: 5}).WithDefaults()
	if got.AlertPercent != cost.DefaultAlertPercent || got.Exceeded != cost.ActionFail || got.UnknownPrice != cost.UnknownPriceAllow {
		t.Errorf("defaults = %+v", got)
	}
	kept := (cost.Budget{AlertPercent: 50, Exceeded: cost.ActionSuspend, UnknownPrice: cost.UnknownPriceBlock}).WithDefaults()
	if kept.AlertPercent != 50 || kept.Exceeded != cost.ActionSuspend || kept.UnknownPrice != cost.UnknownPriceBlock {
		t.Errorf("a set field was overwritten: %+v", kept)
	}
}

// TestWindowOfAPeriod: DAY and MONTH read their own rows; PER_EXECUTION and TOTAL read every monthly row.
func TestWindowOfAPeriod(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 14, 15, 30, 0, 0, time.FixedZone("x", 3600))
	grain, start, all := cost.Window(cost.Budget{Period: cost.PeriodDay}, at)
	if grain != "DAY" || !start.Equal(time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)) || all {
		t.Errorf("DAY window = %s %s %v", grain, start, all)
	}
	grain, start, all = cost.Window(cost.Budget{Period: cost.PeriodMonth}, at)
	if grain != "MONTH" || !start.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) || all {
		t.Errorf("MONTH window = %s %s %v", grain, start, all)
	}
	for _, p := range []cost.Period{cost.PeriodPerExecution, cost.PeriodTotal, "UNKNOWN"} {
		if grain, _, all := cost.Window(cost.Budget{Period: p}, at); grain != "MONTH" || !all {
			t.Errorf("%s window = %s, all periods %v", p, grain, all)
		}
	}
}

// TestDecideStrictestBudget: a FAIL budget refuses, a SUSPEND budget suspends, a blocked unpriced model is
// refused, and a call inside every limit passes.
func TestDecideStrictestBudget(t *testing.T) {
	t.Parallel()
	state := func(spent, limit int64, action cost.Action, unknown cost.UnknownPriceAction) cost.BudgetState {
		return cost.BudgetState{
			Budget:     cost.Budget{Scope: orgScope, Period: cost.PeriodMonth, LimitMicro: limit, Exceeded: action, UnknownPrice: unknown},
			SpentMicro: spent,
		}
	}
	if d, err := cost.Decide([]cost.BudgetState{state(0, 100, cost.ActionFail, "")}, 100, true); err != nil || d.Suspend {
		t.Errorf("a call that fits = %+v, %v", d, err)
	}
	var exceeded *cost.BudgetExceededError
	_, err := cost.Decide([]cost.BudgetState{state(90, 100, cost.ActionFail, "")}, 20, true)
	if !errors.As(err, &exceeded) || !strings.Contains(err.Error(), "AiBudgetExceeded") || exceeded.LimitMicro != 100 {
		t.Errorf("FAIL budget = %v", err)
	}
	if d, err := cost.Decide([]cost.BudgetState{state(100, 100, cost.ActionSuspend, "")}, 1, true); err != nil || !d.Suspend {
		t.Errorf("SUSPEND budget = %+v, %v", d, err)
	}
	_, err = cost.Decide([]cost.BudgetState{state(0, 100, "OTHER", "")}, 200, true)
	if !errors.As(err, &exceeded) {
		t.Errorf("an unknown action must refuse like FAIL: %v", err)
	}
	if _, err := cost.Decide([]cost.BudgetState{state(0, 100, cost.ActionFail, cost.UnknownPriceBlock)}, 0, false); !errors.Is(err, cost.ErrUnknownPrice) {
		t.Errorf("a blocked unpriced model = %v", err)
	}
	if _, err := cost.Decide([]cost.BudgetState{state(0, 100, cost.ActionFail, cost.UnknownPriceAllow)}, 0, false); err != nil {
		t.Errorf("an allowed unpriced model = %v", err)
	}
}

// TestDueAlerts: the threshold alert fires at the configured share and the exceeded alert at the limit.
func TestDueAlerts(t *testing.T) {
	t.Parallel()
	s := cost.BudgetState{Budget: cost.Budget{Scope: orgScope, Period: cost.PeriodMonth, LimitMicro: 1000, AlertPercent: 50}}
	if got := cost.DueAlerts(s, 499); len(got) != 0 {
		t.Errorf("below the share: %+v", got)
	}
	got := cost.DueAlerts(s, 500)
	if len(got) != 1 || got[0].Kind != cost.AlertThreshold || got[0].ThresholdPct != 50 {
		t.Errorf("at the share: %+v", got)
	}
	got = cost.DueAlerts(s, 1000)
	if len(got) != 2 || got[1].Kind != cost.AlertExceeded || got[1].ThresholdPct != 100 {
		t.Errorf("at the limit: %+v", got)
	}
}

// TestPurposeAndEntryValidation: the purposes are a closed set, and an entry needs a provider, a model and a
// non-negative cost.
func TestPurposeAndEntryValidation(t *testing.T) {
	t.Parallel()
	if !cost.PurposeAgent.Valid() || cost.Purpose("OTHER").Valid() {
		t.Error("Purpose.Valid does not follow the closed set")
	}
	ok := cost.Entry{Purpose: cost.PurposeChain, Provider: "p", Model: "m"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("a valid entry was refused: %v", err)
	}
	for name, mutate := range map[string]func(*cost.Entry){
		"bad purpose": func(e *cost.Entry) { e.Purpose = "X" },
		"no provider": func(e *cost.Entry) { e.Provider = "" },
		"no model":    func(e *cost.Entry) { e.Model = "" },
		"negative":    func(e *cost.Entry) { e.CostMicro = -1 },
	} {
		e := ok
		mutate(&e)
		if err := e.Validate(); err == nil {
			t.Errorf("%s: the entry was accepted", name)
		}
	}
}

// fakeStore answers the budget reads of Admit.
type fakeStore struct {
	states []cost.BudgetState
	err    error
}

func (f fakeStore) PutBudget(context.Context, orgscope.Org, cost.Budget, time.Time) error { return nil }
func (f fakeStore) BudgetStates(context.Context, orgscope.Org, []cost.Scope, time.Time) ([]cost.BudgetState, error) {
	return f.states, f.err
}
func (f fakeStore) RecordUsage(context.Context, orgscope.Org, []cost.Scope, cost.Entry, time.Time) ([]cost.Alert, error) {
	return nil, nil
}

// TestAdmitReadsTheBudgetsOfACall: Admit decides from the stored states and passes a store failure on.
func TestAdmitReadsTheBudgetsOfACall(t *testing.T) {
	t.Parallel()
	over := fakeStore{states: []cost.BudgetState{{
		Budget: cost.Budget{Scope: orgScope, Period: cost.PeriodMonth, LimitMicro: 10, Exceeded: cost.ActionFail}, SpentMicro: 10,
	}}}
	var exceeded *cost.BudgetExceededError
	if _, err := cost.Admit(t.Context(), over, orgscope.Org{}, []cost.Scope{orgScope}, 1, true, time.Now()); !errors.As(err, &exceeded) {
		t.Errorf("Admit over the limit = %v", err)
	}
	if _, err := cost.Admit(t.Context(), fakeStore{}, orgscope.Org{}, nil, 1, true, time.Now()); err != nil {
		t.Errorf("Admit without budgets = %v", err)
	}
	boom := errors.New("store down")
	if _, err := cost.Admit(t.Context(), fakeStore{err: boom}, orgscope.Org{}, nil, 1, true, time.Now()); !errors.Is(err, boom) {
		t.Errorf("Admit with a failing store = %v", err)
	}
}

// TestPriceTableParsing: a document in the orcon/ai-prices@1 format reads back, and each malformed shape is refused.
func TestPriceTableParsing(t *testing.T) {
	t.Parallel()
	doc := `{"format":"orcon/ai-prices@1","version":"v1","issuedAt":"2026-10-01T00:00:00Z","expiresAt":"2027-01-01T00:00:00Z",
	  "models":[{"provider":"openai","model":"gpt-x","inputPerMillion":100,"outputPerMillion":200}]}`
	tb, err := cost.ParseTable([]byte(doc))
	if err != nil || tb.Len() != 1 || tb.ExpiresAt.IsZero() {
		t.Fatalf("ParseTable = %+v, %v", tb, err)
	}
	if p, ok := tb.Price(cost.ModelKey{Provider: "openai", Model: "gpt-x"}); !ok || p.Output != 200 {
		t.Errorf("price = %+v, %v", p, ok)
	}
	bad := map[string]string{
		"not json":       `{`,
		"wrong format":   `{"format":"x"}`,
		"no issuedAt":    `{"format":"orcon/ai-prices@1","version":"v","models":[]}`,
		"bad issuedAt":   `{"format":"orcon/ai-prices@1","version":"v","issuedAt":"yesterday","models":[]}`,
		"bad expiresAt":  `{"format":"orcon/ai-prices@1","version":"v","issuedAt":"2026-10-01T00:00:00Z","expiresAt":"soon","models":[]}`,
		"no models":      `{"format":"orcon/ai-prices@1","version":"v","issuedAt":"2026-10-01T00:00:00Z"}`,
		"no model name":  `{"format":"orcon/ai-prices@1","version":"v","issuedAt":"2026-10-01T00:00:00Z","models":[{"provider":"p"}]}`,
		"fractional":     `{"format":"orcon/ai-prices@1","version":"v","issuedAt":"2026-10-01T00:00:00Z","models":[{"provider":"p","model":"m","inputPerMillion":1.5}]}`,
		"negative":       `{"format":"orcon/ai-prices@1","version":"v","issuedAt":"2026-10-01T00:00:00Z","models":[{"provider":"p","model":"m","inputPerMillion":-1}]}`,
		"listed twice":   `{"format":"orcon/ai-prices@1","version":"v","issuedAt":"2026-10-01T00:00:00Z","models":[{"provider":"p","model":"m"},{"provider":"p","model":"m"}]}`,
		"no version row": `{"format":"orcon/ai-prices@1","issuedAt":"2026-10-01T00:00:00Z","models":[]}`,
	}
	for name, d := range bad {
		if _, err := cost.ParseTable([]byte(d)); err == nil {
			t.Errorf("%s: the document was accepted", name)
		}
	}
	if cost.Embedded().Len() != 0 {
		t.Error("the embedded table must ship without prices")
	}
}
