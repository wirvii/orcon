// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// maxBudgetsPerScope bounds the budgets one scope can have: one per period.
const maxBudgetsPerScope = 8

var _ cost.Store = (*Store)(nil)

// rollupGranularities are the granularities every call is rolled up into.
var rollupGranularities = []string{"DAY", "MONTH"}

// PutBudget implements [cost.Store]: the budget of a scope and period is
// created, or its limit and actions replaced.
func (s *Store) PutBudget(ctx context.Context, org identity.OrgID, b cost.Budget, at time.Time) error {
	b = b.WithDefaults()
	if err := b.Validate(); err != nil {
		return err
	}
	return s.write(ctx, "put ai budget", func(ctx context.Context, q *sqlcgen.Queries) error {
		execution, flow, project := scopeColumns(b.Scope)
		return q.UpsertAIBudget(ctx, sqlcgen.UpsertAIBudgetParams{
			ID:                    budgetID(org, b).Bytes(),
			OrgID:                 uuidBytes(org),
			ScopeKey:              scopeKeyOf(org, b.Scope).Bytes(),
			ScopeKind:             string(b.Scope.Kind),
			ExecutionID:           execution,
			FlowID:                flow,
			ProjectID:             project,
			Period:                string(b.Period),
			LimitMicrodollars:     b.LimitMicro,
			AlertThresholdPercent: int64(b.AlertPercent),
			ExceededAction:        string(b.Exceeded),
			UnknownPriceAction:    string(b.UnknownPrice),
			CreatedAt:             micros(at),
			UpdatedAt:             micros(at),
		})
	})
}

// BudgetStates implements [cost.Store].
func (s *Store) BudgetStates(ctx context.Context, org identity.OrgID, scopes []cost.Scope, at time.Time) ([]cost.BudgetState, error) {
	var out []cost.BudgetState
	err := s.read(ctx, "read ai budgets", func(ctx context.Context, q *sqlcgen.Queries) error {
		for _, sc := range scopes {
			states, err := budgetStatesOf(ctx, q, org, sc, at)
			if err != nil {
				return err
			}
			out = append(out, states...)
		}
		return nil
	})
	return out, err
}

// RecordUsage implements [cost.Store]. The rollups of every scope, the budget
// states after this call and the alerts it made due are written in one
// transaction, so a rollup never counts a call that is not kept.
func (s *Store) RecordUsage(ctx context.Context, org identity.OrgID, scopes []cost.Scope, e cost.Entry, at time.Time) ([]cost.Alert, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	var alerts []cost.Alert
	err := s.write(ctx, "record ai usage", func(ctx context.Context, q *sqlcgen.Queries) error {
		for _, sc := range scopes {
			for _, gran := range rollupGranularities {
				if err := upsertRollup(ctx, q, org, sc, gran, e, at); err != nil {
					return err
				}
			}
		}
		var err error
		alerts, err = recordAlerts(ctx, q, org, scopes, at)
		return err
	})
	return alerts, err
}

// recordAlerts inserts the alerts that the spending of the scopes made due. An
// alert already recorded for its period and kind is not inserted again.
func recordAlerts(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, scopes []cost.Scope, at time.Time) ([]cost.Alert, error) {
	var alerts []cost.Alert
	for _, sc := range scopes {
		states, err := budgetStatesOf(ctx, q, org, sc, at)
		if err != nil {
			return nil, err
		}
		for i := range states {
			st := &states[i]
			for _, a := range cost.DueAlerts(*st, st.SpentMicro) {
				n, err := q.InsertAIBudgetAlertIfAbsent(ctx, sqlcgen.InsertAIBudgetAlertIfAbsentParams{
					ID:                alertID(org, *st, a).Bytes(),
					OrgID:             uuidBytes(org),
					AiBudgetID:        budgetID(org, st.Budget).Bytes(),
					PeriodStartedAt:   micros(st.PeriodStart),
					Kind:              string(a.Kind),
					ThresholdPercent:  int64(a.ThresholdPct),
					SpentMicrodollars: a.SpentMicro,
					CreatedAt:         micros(at),
					UpdatedAt:         micros(at),
				})
				if err != nil {
					return nil, err
				}
				if n > 0 {
					alerts = append(alerts, a)
				}
			}
		}
	}
	return alerts, nil
}

func upsertRollup(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, sc cost.Scope, gran string, e cost.Entry, at time.Time) error {
	_, start, _ := cost.Window(cost.Budget{Period: periodOfGranularity(gran)}, at)
	execution, flow, project := scopeColumns(sc)
	return q.UpsertAIUsageRollup(ctx, sqlcgen.UpsertAIUsageRollupParams{
		ID:                   rollupID(org, sc, gran, start, e.Purpose).Bytes(),
		OrgID:                uuidBytes(org),
		ScopeKey:             scopeKeyOf(org, sc).Bytes(),
		ScopeKind:            string(sc.Kind),
		ExecutionID:          execution,
		FlowID:               flow,
		ProjectID:            project,
		Grain:                gran,
		PeriodStartedAt:      micros(start),
		Purpose:              string(e.Purpose),
		InputTokenCount:      e.Usage.Input,
		OutputTokenCount:     e.Usage.Output,
		CacheReadTokenCount:  e.Usage.CacheRead,
		CacheWriteTokenCount: e.Usage.CacheWrite,
		ReasoningTokenCount:  e.Usage.Reasoning,
		CostMicrodollars:     e.CostMicro,
		UnpricedCallCount:    unpriced(e),
		CallCount:            1,
		CreatedAt:            micros(at),
		UpdatedAt:            micros(at),
	})
}

// budgetStatesOf reads the budgets of one scope with the spending in their window.
func budgetStatesOf(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, sc cost.Scope, at time.Time) ([]cost.BudgetState, error) {
	rows, err := q.ListAIBudgetsForScope(ctx, uuidBytes(org), scopeKeyOf(org, sc).Bytes(), maxBudgetsPerScope)
	if err != nil {
		return nil, err
	}
	out := make([]cost.BudgetState, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		b := cost.Budget{
			Scope:        sc,
			Period:       cost.Period(r.Period),
			LimitMicro:   r.LimitMicrodollars,
			AlertPercent: int(r.AlertThresholdPercent),
			Exceeded:     cost.Action(r.ExceededAction),
			UnknownPrice: cost.UnknownPriceAction(r.UnknownPriceAction),
		}
		gran, start, all := cost.Window(b, at)
		spent, err := spentIn(ctx, q, org, sc, gran, start, all)
		if err != nil {
			return nil, fmt.Errorf("read spending of %s budget: %w", b.Period, err)
		}
		out = append(out, cost.BudgetState{Budget: b, PeriodStart: start, SpentMicro: spent})
	}
	return out, nil
}

// spentIn sums the rollups of a scope in a window: one period, or every monthly row.
func spentIn(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, sc cost.Scope, gran string, start time.Time, all bool) (int64, error) {
	if all {
		return q.GetAIUsageSpendAllPeriods(ctx, uuidBytes(org), scopeKeyOf(org, sc).Bytes())
	}
	return q.GetAIUsageSpend(ctx, sqlcgen.GetAIUsageSpendParams{
		OrgID: uuidBytes(org), ScopeKey: scopeKeyOf(org, sc).Bytes(), Grain: gran, PeriodStartedAt: micros(start),
	})
}

// scopeColumns returns the foreign key column of each scope kind: the scope's
// own id in its column and nil in the others. The organization scope has none.
func scopeColumns(sc cost.Scope) (execution, flow, project []byte) {
	switch sc.Kind {
	case cost.ScopeExecution:
		execution = uuidBytes(sc.ID)
	case cost.ScopeFlow:
		flow = uuidBytes(sc.ID)
	case cost.ScopeProject:
		project = uuidBytes(sc.ID)
	case cost.ScopeOrg:
	}
	return execution, flow, project
}

func periodOfGranularity(gran string) cost.Period {
	if gran == "DAY" {
		return cost.PeriodDay
	}
	return cost.PeriodMonth
}

func unpriced(e cost.Entry) int64 {
	if e.Priced {
		return 0
	}
	return 1
}

// scopeKeyOf identifies a scope across organizations; the unique keys of the
// rollups and budgets use it.
func scopeKeyOf(org identity.OrgID, sc cost.Scope) id.UUID {
	return id.Derive("ai_scope", org.String(), string(sc.Kind), sc.ID.String())
}

func budgetID(org identity.OrgID, b cost.Budget) id.UUID {
	return id.Derive("ai_budget", org.String(), string(b.Scope.Kind), b.Scope.ID.String(), string(b.Period))
}

func rollupID(org identity.OrgID, sc cost.Scope, gran string, start time.Time, purpose cost.Purpose) id.UUID {
	return id.Derive("ai_usage_rollup", org.String(), string(sc.Kind), sc.ID.String(), gran,
		start.Format(time.RFC3339Nano), string(purpose))
}

func alertID(org identity.OrgID, st cost.BudgetState, a cost.Alert) id.UUID {
	return id.Derive("ai_budget_alert", org.String(), string(st.Budget.Scope.Kind), st.Budget.Scope.ID.String(),
		string(st.Budget.Period), st.PeriodStart.Format(time.RFC3339Nano), string(a.Kind), strconv.Itoa(a.ThresholdPct))
}
