// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cost

import (
	"errors"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/platform/id"
)

// ScopeKind names what a budget or a rollup covers (ADR-0062 §2).
type ScopeKind string

// The scope kinds.
const (
	ScopeExecution ScopeKind = "EXECUTION"
	ScopeFlow      ScopeKind = "FLOW"
	ScopeProject   ScopeKind = "PROJECT"
	ScopeOrg       ScopeKind = "ORG"
)

// Scope is one thing that spending is counted against.
type Scope struct {
	Kind ScopeKind
	ID   id.UUID
}

// Period is the window a budget limit applies to.
type Period string

// The budget periods.
const (
	PeriodPerExecution Period = "PER_EXECUTION"
	PeriodDay          Period = "DAY"
	PeriodMonth        Period = "MONTH"
	PeriodTotal        Period = "TOTAL"
)

// Action is what happens when a call would exceed a budget.
type Action string

// The exceeded actions (SPEC-006 R-COST-04). FAIL is the default.
const (
	ActionFail    Action = "FAIL"
	ActionSuspend Action = "SUSPEND"
)

// UnknownPriceAction says what happens to a call to a model without a price.
type UnknownPriceAction string

// The unknown-price actions. ALLOW is the default: the call runs and its cost
// is recorded as unknown.
const (
	UnknownPriceAllow UnknownPriceAction = "ALLOW"
	UnknownPriceBlock UnknownPriceAction = "BLOCK"
)

// DefaultAlertPercent is the share of the limit at which the first alert fires.
const DefaultAlertPercent = 80

// Budget is a limit on spending within one scope and period.
type Budget struct {
	Scope        Scope
	Period       Period
	LimitMicro   int64
	AlertPercent int
	Exceeded     Action
	UnknownPrice UnknownPriceAction
}

// Validate checks a budget before it is stored.
func (b Budget) Validate() error {
	if b.LimitMicro <= 0 {
		return errors.New("cost: a budget needs a positive limit")
	}
	if b.AlertPercent == 0 {
		return nil
	}
	if b.AlertPercent < 1 || b.AlertPercent > 99 {
		return fmt.Errorf("cost: alert percent %d is outside 1..99", b.AlertPercent)
	}
	if b.Period == PeriodPerExecution && b.Scope.Kind != ScopeExecution {
		return errors.New("cost: a PER_EXECUTION budget belongs to an execution")
	}
	switch b.Period {
	case PeriodPerExecution, PeriodDay, PeriodMonth, PeriodTotal:
	default:
		return fmt.Errorf("cost: period %q is not one of the closed set", b.Period)
	}
	return nil
}

// WithDefaults fills the defaults of the optional fields.
func (b Budget) WithDefaults() Budget {
	if b.AlertPercent == 0 {
		b.AlertPercent = DefaultAlertPercent
	}
	if b.Exceeded == "" {
		b.Exceeded = ActionFail
	}
	if b.UnknownPrice == "" {
		b.UnknownPrice = UnknownPriceAllow
	}
	return b
}

// Window says which rollup rows hold the spending of a budget at an instant.
// DAY and MONTH budgets read the rows of their own period. PER_EXECUTION and
// TOTAL budgets read every monthly row of their scope, since they have no
// period of their own.
func Window(b Budget, at time.Time) (granularity string, start time.Time, allPeriods bool) {
	at = at.UTC()
	switch b.Period {
	case PeriodDay:
		return "DAY", time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC), false
	case PeriodMonth:
		return "MONTH", time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC), false
	case PeriodPerExecution, PeriodTotal:
		return "MONTH", time.Unix(0, 0).UTC(), true
	default:
		return "MONTH", time.Unix(0, 0).UTC(), true
	}
}

// BudgetState is a budget with the spending it already has in its window.
type BudgetState struct {
	Budget      Budget
	PeriodStart time.Time
	SpentMicro  int64
}

// BudgetExceededError reports a call refused because a FAIL budget would be
// exceeded. The flow can route it (SPEC-006 R-COST-04, error AiBudgetExceeded).
type BudgetExceededError struct {
	Scope      Scope
	Period     Period
	LimitMicro int64
	SpentMicro int64
}

// Error implements error.
func (e *BudgetExceededError) Error() string {
	return fmt.Sprintf("AiBudgetExceeded: %s budget for %s has %d micro-dollars left of %d",
		e.Period, e.Scope.Kind, e.LimitMicro-e.SpentMicro, e.LimitMicro)
}

// ErrUnknownPrice reports a call refused because its model has no price and a
// budget blocks unpriced models.
var ErrUnknownPrice = errors.New("cost: the model has no price and a budget blocks unpriced calls")

// Decision is the outcome of checking a call against its budgets.
type Decision struct {
	// Suspend is true when a SUSPEND budget would be exceeded: the run waits
	// until the budget is raised. The call is not made.
	Suspend bool
}

// Decide checks a call against every budget covering it. The strictest budget
// wins: a FAIL budget that would be exceeded refuses the call with a
// [*BudgetExceededError], and a SUSPEND budget suspends it. Every budget is
// checked, so one refusal never hides another. An unpriced model is refused
// only when a budget blocks unpriced calls.
func Decide(states []BudgetState, reservationMicro int64, priced bool) (Decision, error) {
	var d Decision
	for i := range states {
		s := &states[i]
		if !priced && s.Budget.UnknownPrice == UnknownPriceBlock {
			return Decision{}, fmt.Errorf("%w (%s budget)", ErrUnknownPrice, s.Budget.Scope.Kind)
		}
		if s.SpentMicro+reservationMicro <= s.Budget.LimitMicro {
			continue
		}
		switch s.Budget.Exceeded {
		case ActionSuspend:
			d.Suspend = true
		case ActionFail:
			return Decision{}, &BudgetExceededError{
				Scope: s.Budget.Scope, Period: s.Budget.Period, LimitMicro: s.Budget.LimitMicro, SpentMicro: s.SpentMicro,
			}
		default:
			return Decision{}, &BudgetExceededError{
				Scope: s.Budget.Scope, Period: s.Budget.Period, LimitMicro: s.Budget.LimitMicro, SpentMicro: s.SpentMicro,
			}
		}
	}
	return d, nil
}

// Alert is a threshold a budget has crossed in one period. Each alert is
// emitted once per budget, period and threshold (SPEC-006 R-COST-06).
type Alert struct {
	Scope        Scope
	Period       Period
	PeriodStart  time.Time
	Kind         AlertKind
	ThresholdPct int
	SpentMicro   int64
	LimitMicro   int64
}

// AlertKind says which threshold an alert reports.
type AlertKind string

// The alert kinds: the configured share of the limit, and the limit itself.
const (
	AlertThreshold AlertKind = "THRESHOLD"
	AlertExceeded  AlertKind = "EXCEEDED"
)

// DueAlerts returns the alerts a budget state has reached after spending is
// counted. The store records each one once; a repeat is a no-op there.
func DueAlerts(s BudgetState, spentAfter int64) []Alert {
	b := s.Budget.WithDefaults()
	var out []Alert
	base := Alert{
		Scope: b.Scope, Period: b.Period, PeriodStart: s.PeriodStart, SpentMicro: spentAfter, LimitMicro: b.LimitMicro,
	}
	if spentAfter*100 >= b.LimitMicro*int64(b.AlertPercent) {
		a := base
		a.Kind, a.ThresholdPct = AlertThreshold, b.AlertPercent
		out = append(out, a)
	}
	if spentAfter >= b.LimitMicro {
		a := base
		a.Kind, a.ThresholdPct = AlertExceeded, 100
		out = append(out, a)
	}
	return out
}
