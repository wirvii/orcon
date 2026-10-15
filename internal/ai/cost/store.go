// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cost

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/identity"
)

// Store is the persistence port of cost accounting. The adapters implement it
// in one transaction per call: RecordUsage writes the rollups and the alerts of
// one call together, so a rollup never counts a call the store did not keep.
type Store interface {
	// PutBudget creates or replaces the budget of a scope and period.
	PutBudget(ctx context.Context, org identity.OrgID, b Budget, at time.Time) error
	// BudgetStates returns every budget that covers one of the scopes, with the
	// spending it already has in its window at the instant.
	BudgetStates(ctx context.Context, org identity.OrgID, scopes []Scope, at time.Time) ([]BudgetState, error)
	// RecordUsage adds one call to the rollups of every scope and returns the
	// alerts that this call made due for the first time.
	RecordUsage(ctx context.Context, org identity.OrgID, scopes []Scope, e Entry, at time.Time) ([]Alert, error)
}

// Admit reads the budgets of a call and decides whether it may run. It is the
// check that runs before a model call; the call's cost is recorded afterwards
// with [Store.RecordUsage].
func Admit(ctx context.Context, st Store, org identity.OrgID, scopes []Scope, reservationMicro int64, priced bool, at time.Time) (Decision, error) {
	states, err := st.BudgetStates(ctx, org, scopes, at)
	if err != nil {
		return Decision{}, err
	}
	return Decide(states, reservationMicro, priced)
}
