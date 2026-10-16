// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"context"
	"testing"

	"github.com/wirvii/orcon/internal/ai/agent"
	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/platform/id"
)

// The cost accounting and the agent runs (SPEC-006 AI2 and AI3; R-ISO-10). The rows of A are invisible to B through
// every method: no run, step, transcript, memory, budget or rollup, and an attempt of B to write into the run or the
// budget of A leaves A's rows as they were.

// aiCases maps each method of the cost and agent stores to its case; addAICases registers them.
var aiCases = map[string]func(context.Context, testing.TB, *Env){
	"PutBudget":    casePutBudget,
	"BudgetStates": caseBudgetStates,
	"RecordUsage":  caseRecordUsage,
	"Commit":       caseAgentCommit,
	"Run":          caseAgentRun,
	"Steps":        caseAgentSteps,
	"Transcript":   caseAgentTranscript,
	"Memory":       caseAgentMemory,
}

// addAICases registers the cases of the cost and agent stores.
func addAICases(r *Registry) error {
	for name, run := range aiCases {
		if err := r.Add(Case{Subject: StoreMethod(name), Run: run}); err != nil {
			return err
		}
	}
	return nil
}

// orgBudget is the monthly budget of the whole organization o.
func orgBudget(o *Org) cost.Budget {
	return cost.Budget{
		Scope: cost.Scope{Kind: cost.ScopeOrg, ID: o.UUID}, Period: cost.PeriodMonth, LimitMicro: 1_000_000,
		AlertPercent: 80, Exceeded: cost.ActionFail, UnknownPrice: cost.UnknownPriceAllow,
	}
}

// modelEntry is one priced model call, labeled with the sentinel of o.
func modelEntry(o *Org) cost.Entry {
	return cost.Entry{
		Purpose: cost.PurposeAgent, Provider: "isolation", Model: o.Sentinel("model"),
		Usage: cost.Usage{Input: 10, Output: 5}, CostMicro: 100, Priced: true,
	}
}

// spentOf returns what organization o has spent in its month, read under its own scope.
func spentOf(ctx context.Context, tb testing.TB, env *Env, o *Org) int64 {
	tb.Helper()
	states, err := env.Stores.BudgetStates(ctx, o.Scope, []cost.Scope{{Kind: cost.ScopeOrg, ID: o.UUID}}, seededAt)
	if err != nil {
		tb.Fatalf("budget states of %s: %v", o.ID, err)
	}
	var total int64
	for i := range states {
		total += states[i].SpentMicro
	}
	return total
}

// casePutBudget: B cannot set the budget of A's organization, and A's budget is stored under A's scope.
func casePutBudget(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if err := env.Stores.PutBudget(ctx, env.B.Scope, orgBudget(&env.A), seededAt); err == nil {
		states, readErr := env.Stores.BudgetStates(ctx, env.A.Scope, []cost.Scope{{Kind: cost.ScopeOrg, ID: env.A.UUID}}, seededAt)
		if readErr != nil || len(states) != 0 {
			tb.Fatalf("B stored a budget that A reads: %d states, %v", len(states), readErr)
		}
	}
	if err := env.Stores.PutBudget(ctx, env.A.Scope, orgBudget(&env.A), seededAt); err != nil {
		tb.Fatalf("A stores its budget: %v", err)
	}
}

// caseBudgetStates: B reads no budget and no spending of A.
func caseBudgetStates(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if err := env.Stores.PutBudget(ctx, env.A.Scope, orgBudget(&env.A), seededAt); err != nil {
		tb.Fatalf("A stores its budget: %v", err)
	}
	if _, err := env.Stores.RecordUsage(ctx, env.A.Scope, []cost.Scope{{Kind: cost.ScopeOrg, ID: env.A.UUID}}, modelEntry(&env.A), seededAt); err != nil {
		tb.Fatalf("A records usage: %v", err)
	}
	if got := spentOf(ctx, tb, env, &env.A); got != 100 {
		tb.Fatalf("A reads its spending = %d; want 100", got)
	}
	states, err := env.Stores.BudgetStates(ctx, env.B.Scope, []cost.Scope{{Kind: cost.ScopeOrg, ID: env.A.UUID}}, seededAt)
	if err != nil || len(states) != 0 {
		tb.Fatalf("B reads the budget of A = %d states, %v; want none", len(states), err)
	}
	noLeak(tb, "BudgetStates of B", states, err, &env.A)
}

// caseRecordUsage: usage that B records against A's scope never reaches A's spending.
func caseRecordUsage(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	if err := env.Stores.PutBudget(ctx, env.A.Scope, orgBudget(&env.A), seededAt); err != nil {
		tb.Fatalf("A stores its budget: %v", err)
	}
	_, _ = env.Stores.RecordUsage(ctx, env.B.Scope, []cost.Scope{{Kind: cost.ScopeOrg, ID: env.A.UUID}}, modelEntry(&env.B), seededAt)
	if got := spentOf(ctx, tb, env, &env.A); got != 0 {
		tb.Fatalf("A's spending after B's attempt = %d; want 0", got)
	}
}

// runOf commits the first step of a run of organization o, in a conversation of o, and returns the run.
func runOf(ctx context.Context, tb testing.TB, env *Env, o *Org) (id.UUID, string) {
	tb.Helper()
	run := fresh[id.UUID](env)
	conversation := "conversation-" + o.Sentinel("conversation")
	c := agent.Commit{
		RunID: run, ConversationKey: conversation,
		Step: agent.StepRecord{Number: 1, Kind: agent.StepModel, Output: `{"text":"` + o.Sentinel("answer") + `","toolCalls":[]}`,
			Model: agent.ModelUse{Provider: "isolation", Model: "model", CostStatus: "UNKNOWN", FinishReason: provider.FinishStop}},
		Rows:  []agent.TranscriptRow{{Sequence: 1, Role: provider.RoleAssistant, Part: agent.PartText, Text: o.Sentinel("answer")}},
		Final: true, FinalText: o.Sentinel("answer"),
		Remember: []agent.MemoryRow{{Role: provider.RoleUser, Text: o.Sentinel("question")}, {Role: provider.RoleAssistant, Text: o.Sentinel("answer")}},
		At:       seededAt,
	}
	if err := env.Stores.Commit(ctx, o.Scope, c); err != nil {
		tb.Fatalf("commit a step of %s: %v", o.ID, err)
	}
	return run, conversation
}

// caseAgentCommit: B cannot add a step to A's run, and A's run keeps its one step.
func caseAgentCommit(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	run, _ := runOf(ctx, tb, env, &env.A)
	_ = env.Stores.Commit(ctx, env.B.Scope, agent.Commit{
		RunID: run, Step: agent.StepRecord{Number: 2, Kind: agent.StepModel, Output: `{"text":"intruder","toolCalls":[]}`,
			Model: agent.ModelUse{Provider: "isolation", Model: "model", CostStatus: "UNKNOWN", FinishReason: provider.FinishStop}},
		Rows: []agent.TranscriptRow{{Sequence: 2, Role: provider.RoleAssistant, Part: agent.PartText, Text: "intruder"}}, At: seededAt,
	})
	steps, err := env.Stores.Steps(ctx, env.A.Scope, run)
	if err != nil || len(steps) != 1 {
		tb.Fatalf("the steps of A's run after B's attempt = %d, %v; want 1", len(steps), err)
	}
}

// caseAgentRun: the owner reads its run; B finds no such run.
func caseAgentRun(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	run, _ := runOf(ctx, tb, env, &env.A)
	state, ok, err := env.Stores.Run(ctx, env.A.Scope, run)
	if err != nil || !ok || state.Status != agent.StatusSucceeded {
		tb.Fatalf("A reads its run = %+v, %v, %v", state, ok, err)
	}
	other, ok, err := env.Stores.Run(ctx, env.B.Scope, run)
	if ok {
		tb.Fatalf("B reads the run of A: %+v", other)
	}
	noLeak(tb, "Run of B", other, err, &env.A)
}

// caseAgentSteps: B lists no step of A's run.
func caseAgentSteps(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	run, _ := runOf(ctx, tb, env, &env.A)
	steps, err := env.Stores.Steps(ctx, env.B.Scope, run)
	if len(steps) != 0 {
		tb.Fatalf("B lists %d steps of the run of A", len(steps))
	}
	noLeak(tb, "Steps of B", steps, err, &env.A)
}

// caseAgentTranscript: B reads no transcript row of A's run.
func caseAgentTranscript(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	run, _ := runOf(ctx, tb, env, &env.A)
	rows, err := env.Stores.Transcript(ctx, env.B.Scope, run)
	if len(rows) != 0 {
		tb.Fatalf("B reads %d transcript rows of the run of A", len(rows))
	}
	noLeak(tb, "Transcript of B", rows, err, &env.A)
}

// caseAgentMemory: B reads nothing of a conversation of A, even with the same key.
func caseAgentMemory(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	_, conversation := runOf(ctx, tb, env, &env.A)
	mine, err := env.Stores.Memory(ctx, env.A.Scope, conversation, 10)
	if err != nil || len(mine) != 2 {
		tb.Fatalf("A reads its conversation = %d rows, %v; want 2", len(mine), err)
	}
	rows, err := env.Stores.Memory(ctx, env.B.Scope, conversation, 10)
	if len(rows) != 0 {
		tb.Fatalf("B reads %d messages of the conversation of A", len(rows))
	}
	noLeak(tb, "Memory of B", rows, err, &env.A)
}
