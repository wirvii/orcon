// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/ai/agent"
	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// fakeStore keeps runs in memory and enforces the rules of the real store: a
// step is committed once, in order, and transcript rows are only appended.
type fakeStore struct {
	mu     sync.Mutex
	runs   map[id.UUID]agent.RunState
	steps  map[id.UUID][]agent.StepRecord
	rows   map[id.UUID][]agent.TranscriptRow
	conv   map[string][]agent.MemoryRow
	costs  []cost.Entry
	fails  int // commits to refuse before accepting one, to simulate a failed write
	writes int
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		runs:  map[id.UUID]agent.RunState{},
		steps: map[id.UUID][]agent.StepRecord{},
		rows:  map[id.UUID][]agent.TranscriptRow{},
		conv:  map[string][]agent.MemoryRow{},
	}
}

// fakeOrgs issues the scope of every organization of the tests.
type fakeOrgs struct{}

// ScopeOf implements [agent.OrgScopes].
func (fakeOrgs) ScopeOf(org identity.OrgID) (orgscope.Org, error) {
	return issue.New(id.UUID(org))
}

func (s *fakeStore) Run(_ context.Context, _ orgscope.Org, runID id.UUID) (agent.RunState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.runs[runID]
	return st, ok, nil
}

func (s *fakeStore) Steps(_ context.Context, _ orgscope.Org, runID id.UUID) ([]agent.StepRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]agent.StepRecord(nil), s.steps[runID]...), nil
}

func (s *fakeStore) Transcript(_ context.Context, _ orgscope.Org, runID id.UUID) ([]agent.TranscriptRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]agent.TranscriptRow(nil), s.rows[runID]...), nil
}

func (s *fakeStore) Commit(_ context.Context, _ orgscope.Org, c agent.Commit) error { //nolint:gocritic // the port passes the commit by value
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fails > 0 {
		s.fails--
		return errors.New("fake: write refused")
	}
	if c.Step.Number != len(s.steps[c.RunID])+1 {
		return fmt.Errorf("fake: step %d committed out of order", c.Step.Number)
	}
	for i, r := range c.Rows {
		if want := len(s.rows[c.RunID]) + i + 1; r.Sequence != want {
			return fmt.Errorf("fake: transcript row %d written at sequence %d, want %d", i, r.Sequence, want)
		}
	}
	if c.Memory != nil && len(s.rows[c.RunID]) > 0 {
		return errors.New("fake: memory snapshot written after the transcript started")
	}
	st := s.runs[c.RunID]
	st.Status = agent.StatusRunning
	if _, ok := s.runs[c.RunID]; !ok {
		s.runs[c.RunID] = st
	}
	s.steps[c.RunID] = append(s.steps[c.RunID], c.Step)
	s.rows[c.RunID] = append(s.rows[c.RunID], c.Rows...)
	st.StepCount = len(s.steps[c.RunID])
	if c.Step.Kind == agent.StepTool {
		st.ToolCallCount++
	}
	if c.Cost != nil {
		s.costs = append(s.costs, *c.Cost)
		st.SpendMicro += c.Cost.CostMicro
	}
	if c.Final {
		st.Status = agent.StatusSucceeded
		st.FinalText = c.FinalText
		s.conv[c.ConversationKey] = append(s.conv[c.ConversationKey], c.Remember...)
	}
	s.runs[c.RunID] = st
	s.writes++
	return nil
}

func (s *fakeStore) Memory(_ context.Context, _ orgscope.Org, key string, limit int) ([]agent.MemoryRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := s.conv[key]
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	return append([]agent.MemoryRow(nil), all...), nil
}

// reply is one scripted answer of the fake model.
type reply struct {
	resp provider.Response
	err  error
	// wait holds the answer until the context ends, to model a call in flight.
	wait bool
}

// fakeProvider answers from a script and records every request it received.
type fakeProvider struct {
	caps     provider.Capabilities
	mu       sync.Mutex
	script   []reply
	requests []provider.Request
}

func (f *fakeProvider) Capabilities(string) provider.Capabilities { return f.caps }

func (f *fakeProvider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	if len(f.script) == 0 {
		f.mu.Unlock()
		return provider.Response{}, errors.New("fake: no scripted reply left")
	}
	r := f.script[0]
	f.script = f.script[1:]
	f.mu.Unlock()
	if r.wait {
		<-ctx.Done()
		return provider.Response{}, ctx.Err()
	}
	return r.resp, r.err
}

func (f *fakeProvider) requestCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

// fakeModels resolves every model to one fake provider.
type fakeModels struct{ p provider.ModelProvider }

func (m fakeModels) Model(context.Context, identity.OrgID, identity.ProjectID, flow.ModelRef) (provider.ModelProvider, error) { //nolint:ireturn // implements the chain.Models port
	return m.p, nil
}

// fakeTool is a tool node: it records its calls and tracks how many run at once.
type fakeTool struct {
	calls    atomic.Int64
	inFlight atomic.Int64
	maxSeen  atomic.Int64
	delay    time.Duration
	answer   string
}

func (f *fakeTool) Execute(_ context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	f.calls.Add(1)
	n := f.inFlight.Add(1)
	defer f.inFlight.Add(-1)
	for {
		m := f.maxSeen.Load()
		if n <= m || f.maxSeen.CompareAndSwap(m, n) {
			break
		}
	}
	time.Sleep(f.delay)
	members := []jsonvalue.Member{}
	if f.answer != "" {
		members = append(members, jsonvalue.Member{Key: "answer", Value: jsonvalue.String(f.answer)})
	}
	out, err := jsonvalue.Object(members...)
	if err != nil {
		return execution.NodeOutput{}, err
	}
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: execution.NewBatch(execution.Item{JSON: out})}}}, nil
}

// fixedPrices prices every model at the same rate.
type fixedPrices struct{ price cost.Price }

func (p fixedPrices) Price(context.Context, identity.OrgID, flow.ModelRef) (cost.Price, string, bool) {
	return p.price, "test-table", true
}

// fakeBudgets reports one budget that is already at its limit.
type fakeBudgets struct{ cost.Store }

func (fakeBudgets) BudgetStates(context.Context, orgscope.Org, []cost.Scope, time.Time) ([]cost.BudgetState, error) {
	return []cost.BudgetState{{
		Budget:     cost.Budget{Scope: cost.Scope{Kind: cost.ScopeOrg}, Period: cost.PeriodMonth, LimitMicro: 100, Exceeded: cost.ActionFail},
		SpentMicro: 100,
	}}, nil
}

// testOrg is the organization every test run belongs to.
func testOrg() identity.OrgID { return identity.OrgID(id.Derive("org-test")) }

// testScope is the scope of testOrg, as the store takes it.
func testScope() orgscope.Org {
	o, err := issue.New(id.UUID(testOrg()))
	if err != nil {
		panic(err)
	}
	return o
}

func mustParse(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	return v
}

// toolReply asks for one call to each named tool, with arguments {"q": "for <name>"}.
func toolReply(calls ...string) provider.Response {
	var tcs []provider.ToolCall
	for i, c := range calls {
		tcs = append(tcs, provider.ToolCall{ID: fmt.Sprintf("call-%d", i+1), Name: c, Arguments: mustObject(c)})
	}
	return provider.Response{ToolCalls: tcs, FinishReason: provider.FinishToolCalls, Usage: provider.Usage{Input: 100, Output: 10}}
}

func textReply(text string) provider.Response {
	return provider.Response{Text: text, FinishReason: provider.FinishStop, Usage: provider.Usage{Input: 100, Output: 10}}
}

// mustObject is the arguments a scripted tool call carries: one string field.
func mustObject(name string) jsonvalue.Value {
	v, err := jsonvalue.Object(jsonvalue.Member{Key: "q", Value: jsonvalue.String("for " + name)})
	if err != nil {
		panic(err)
	}
	return v
}
