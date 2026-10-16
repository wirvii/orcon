// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/ai/agent"
	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The agent battery runs the durable agent (SPEC-006 AI3) on the stores the -store
// flag selects: a run commits its steps, its transcript and its memory, and a run
// that failed in flight resumes from the step it stopped at.

// storeOf is what the agent battery needs from a store.
type storeOf interface {
	agent.Store
	CreateOrganization(ctx context.Context, rec identity.OrgRecord) error
}

// storeOrgs issues the scope of the organizations the agent battery creates.
type storeOrgs struct{}

// ScopeOf implements [agent.OrgScopes].
func (storeOrgs) ScopeOf(org identity.OrgID) (orgscope.Org, error) {
	return storetest.ScopeOf(org), nil
}

// scripted answers from a fixed list; a nil resp with an err fails the call.
type scripted struct {
	mu       sync.Mutex
	script   []scriptedReply
	requests int
}

type scriptedReply struct {
	resp provider.Response
	err  error
}

func (s *scripted) Capabilities(string) provider.Capabilities {
	return provider.Capabilities{Tools: true}
}

func (s *scripted) Generate(context.Context, provider.Request) (provider.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests++
	if len(s.script) == 0 {
		return provider.Response{}, errors.New("scripted: no reply left")
	}
	r := s.script[0]
	s.script = s.script[1:]
	return r.resp, r.err
}

type resolver struct{ p provider.ModelProvider }

func (r resolver) Model(context.Context, identity.OrgID, identity.ProjectID, flow.ModelRef) (provider.ModelProvider, error) { //nolint:ireturn // implements the chain.Models port
	return r.p, nil
}

// answerTool returns a fixed answer for every call.
type answerTool struct {
	mu    sync.Mutex
	calls int
}

func (a *answerTool) Execute(context.Context, execution.NodeCall) (execution.NodeOutput, error) {
	a.mu.Lock()
	a.calls++
	a.mu.Unlock()
	time.Sleep(time.Millisecond)
	out, err := jsonvalue.Object(jsonvalue.Member{Key: "status", Value: jsonvalue.String("ships today")})
	if err != nil {
		return execution.NodeOutput{}, err
	}
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: execution.NewBatch(execution.Item{JSON: out})}}}, nil
}

func agentRunner(t *testing.T, st agent.Store, p provider.ModelProvider, tool execution.NodeRunner) *agent.Runner {
	t.Helper()
	r, err := agent.New(agent.Config{
		Models:  resolver{p: p},
		Store:   st,
		Orgs:    storeOrgs{},
		Runners: map[string]execution.NodeRunner{"orcon.tool@1": tool},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func agentNode(t *testing.T) *flow.Node {
	t.Helper()
	params, err := jsonvalue.Parse([]byte(`{"model":{"provider":"p","name":"m"},"tools":[{"name":"lookup","description":"order status","nodeType":"orcon.tool@1","schema":{"type":"object","required":["q"],"properties":{"q":{"type":"string"}}}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	return &flow.Node{ID: "agent", Name: "agent", Type: agent.NodeType, Parameters: params}
}

func agentCall(t *testing.T, org identity.OrgID, nodeExecution, input string) execution.NodeCall {
	t.Helper()
	item, err := jsonvalue.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	return execution.NodeCall{
		Org: org, Execution: "exec-store", NodeExecution: execution.NodeExecutionID(nodeExecution),
		Node: agentNode(t), Input: execution.NewBatch(execution.Item{JSON: item}),
	}
}

func toolAnswer(t *testing.T) provider.Response {
	t.Helper()
	args, err := jsonvalue.Parse([]byte(`{"q":"order 42"}`))
	if err != nil {
		t.Fatal(err)
	}
	return provider.Response{ToolCalls: []provider.ToolCall{{ID: "call-1", Name: "lookup", Arguments: args}}, FinishReason: provider.FinishToolCalls}
}

func textAnswer(text string) provider.Response {
	return provider.Response{Text: text, FinishReason: provider.FinishStop}
}

func newAgentOrg(t *testing.T, s storeOf) identity.OrgID {
	t.Helper()
	org := identity.OrgID(id.Derive("agent-battery-org", t.Name()))
	if err := s.CreateOrganization(t.Context(), identity.OrgRecord{
		ID: org, Name: "agent", Slug: "agent-" + id.Derive(t.Name()).String()[:8], Status: identity.OrgActive,
		CreatedAt: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("create organization: %v", err)
	}
	return org
}

// runAgentBattery runs one scenario once per store the -store flag selects.
func runAgentBattery(t *testing.T, scenario func(*testing.T, storeOf)) {
	t.Helper()
	ran := 0
	if wantSQLite() {
		ran++
		t.Run("sqlite", func(t *testing.T) {
			scenario(t, storetest.OpenSQLite(t, filepath.Join(t.TempDir(), "orcon.db")))
		})
	}
	if wantPostgres() {
		ran++
		t.Run("postgres", func(t *testing.T) {
			scenario(t, storetest.OpenPostgres(t, newPostgresDatabase(t)))
		})
	}
	if ran == 0 {
		t.Fatal("no store selected")
	}
}

// TestAgentRunOnStore: a run with a tool call commits its steps, its transcript
// and its memory, and the next run of the conversation starts from that memory.
func TestAgentRunOnStore(t *testing.T) {
	runAgentBattery(t, func(t *testing.T, s storeOf) {
		t.Helper()
		org := newAgentOrg(t, s)
		p := &scripted{script: []scriptedReply{{resp: toolAnswer(t)}, {resp: textAnswer("llega hoy")}, {resp: textAnswer("claro")}}}
		tool := &answerTool{}
		r := agentRunner(t, s, p, tool)

		out, err := r.Execute(t.Context(), agentCall(t, org, "ne-store-1", `{"text":"donde esta mi pedido","conversationKey":"wa:+573001"}`))
		if err != nil {
			t.Fatalf("first run: %v", err)
		}
		if n := len(out.Ports[0].Batch.Items()); n != 1 {
			t.Fatalf("first run emitted %d items", n)
		}
		runID := agent.RunID(org, "exec-store", "ne-store-1", 0)
		steps, err := s.Steps(t.Context(), storetest.ScopeOf(org), runID)
		if err != nil || len(steps) != 3 {
			t.Fatalf("steps = %d, %v; want 3 committed steps", len(steps), err)
		}
		if steps[0].Model.Provider != "p" || steps[0].Model.Model != "m" {
			t.Errorf("model step does not record the model used: %+v", steps[0].Model)
		}
		rows, _ := s.Transcript(t.Context(), storetest.ScopeOf(org), runID)
		if len(rows) != 4 {
			t.Errorf("transcript rows = %d, want user, tool call, tool result, answer", len(rows))
		}
		for i, row := range rows {
			if row.Sequence != i+1 {
				t.Errorf("row %d sequence = %d", i, row.Sequence)
			}
		}

		if _, err := r.Execute(t.Context(), agentCall(t, org, "ne-store-2", `{"text":"y el de ayer","conversationKey":"wa:+573001"}`)); err != nil {
			t.Fatalf("second run: %v", err)
		}
		mem, err := s.Memory(t.Context(), storetest.ScopeOf(org), "wa:+573001", 10)
		if err != nil || len(mem) != 4 {
			t.Errorf("conversation memory = %d messages, %v; want the two exchanges", len(mem), err)
		}
		if tool.calls != 1 {
			t.Errorf("tool ran %d times, want once", tool.calls)
		}
	})
}

// TestAgentRerunOnStore: a model call that fails in flight is retried; the steps
// committed before it are read back, so the tool does not run again.
func TestAgentRerunOnStore(t *testing.T) {
	runAgentBattery(t, func(t *testing.T, s storeOf) {
		t.Helper()
		org := newAgentOrg(t, s)
		transient := &provider.Error{Class: provider.ClassTransient, Status: 503, Code: "overloaded", Message: "busy"}
		p := &scripted{script: []scriptedReply{{resp: toolAnswer(t)}, {err: transient}}}
		tool := &answerTool{}
		r := agentRunner(t, s, p, tool)
		call := agentCall(t, org, "ne-rerun-store", `{"text":"donde esta mi pedido"}`)

		_, err := r.Execute(t.Context(), call)
		var ne *execution.NodeError
		if !errors.As(err, &ne) || ne.Class != execution.ClassTransient {
			t.Fatalf("first attempt = %v, want a transient failure", err)
		}
		p.mu.Lock()
		p.script = append(p.script, scriptedReply{resp: textAnswer("ya")})
		p.mu.Unlock()

		out, err := r.Execute(t.Context(), call)
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		if n := len(out.Ports[0].Batch.Items()); n != 1 {
			t.Fatalf("retry emitted %d items", n)
		}
		if tool.calls != 1 {
			t.Errorf("tool ran %d times across the attempts, want once", tool.calls)
		}
		if p.requests != 3 {
			t.Errorf("model asked %d times, want 3", p.requests)
		}
	})
}
