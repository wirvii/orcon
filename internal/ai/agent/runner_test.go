// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/ai/agent"
	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
)

const toolNodeType = "orcon.tool@1"

// agentParams builds the parameters of an agent node with one tool, "lookup",
// that requires the string field q. Extra is spliced into the object.
func agentParams(extra string) string {
	base := `"model":{"provider":"p","name":"m"},` +
		`"tools":[{"name":"lookup","description":"find","nodeType":"` + toolNodeType + `",` +
		`"schema":{"type":"object","required":["q"],"properties":{"q":{"type":"string"}}}}]`
	if extra == "" {
		return "{" + base + "}"
	}
	return "{" + base + "," + extra + "}"
}

type harness struct {
	t      *testing.T
	store  *fakeStore
	prov   *fakeProvider
	tool   *fakeTool
	runner *agent.Runner
}

// newHarness builds a runner over the store and a scripted provider.
func newHarness(t *testing.T, script []reply, budgets cost.Store, prices agent.Prices, tool *fakeTool) harness {
	t.Helper()
	if tool == nil {
		tool = &fakeTool{answer: "found"}
	}
	store := newFakeStore()
	prov := &fakeProvider{caps: provider.Capabilities{Tools: true, JSONSchema: true}, script: script}
	r, err := agent.New(agent.Config{
		Models:  fakeModels{p: prov},
		Store:   store,
		Prices:  prices,
		Budgets: budgets,
		Runners: map[string]execution.NodeRunner{toolNodeType: tool},
	})
	if err != nil {
		t.Fatal(err)
	}
	return harness{t: t, store: store, prov: prov, tool: tool, runner: r}
}

// run executes one item of one node execution. The execution and node ids are
// fixed, so every call of one test names the same run.
func (h harness) run(ctx context.Context, params, input, nodeExecution string) (execution.NodeOutput, error) {
	h.t.Helper()
	return h.runner.Execute(ctx, execution.NodeCall{
		Org:           testOrg(),
		Execution:     "exec-1",
		NodeExecution: execution.NodeExecutionID(nodeExecution),
		Node:          &flow.Node{ID: "agent", Name: "agent", Type: agent.NodeType, Parameters: mustParse(h.t, params)},
		Input:         execution.NewBatch(execution.Item{JSON: mustParse(h.t, input)}),
	})
}

func outText(t *testing.T, out execution.NodeOutput) string {
	t.Helper()
	if len(out.Ports) != 1 || out.Ports[0].Batch.Len() != 1 {
		t.Fatalf("output = %+v, want one item", out)
	}
	v, _ := out.Ports[0].Batch.Item(0).JSON.Get("text")
	s, _ := v.Str()
	return s
}

func nodeCode(err error) string {
	var ne *execution.NodeError
	if errors.As(err, &ne) {
		return ne.Code
	}
	return ""
}

const userInput = `{"text":"where is my order?"}`

// TestAgentRunSteps: a tool call, its result and the final answer run in order,
// each as one committed step.
func TestAgentRunSteps(t *testing.T) {
	t.Parallel()
	h := newHarness(t, []reply{{resp: toolReply("lookup")}, {resp: textReply("it ships today")}}, nil, nil, nil)
	out, err := h.run(t.Context(), agentParams(""), userInput, "ne-1")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := outText(t, out); got != "it ships today" {
		t.Errorf("answer = %q", got)
	}
	steps, _ := h.store.Steps(t.Context(), testOrg(), agent.RunID(testOrg(), "exec-1", "ne-1", 0))
	kinds := []agent.StepKind{}
	for _, s := range steps {
		kinds = append(kinds, s.Kind)
	}
	if len(kinds) != 3 || kinds[0] != agent.StepModel || kinds[1] != agent.StepTool || kinds[2] != agent.StepModel {
		t.Errorf("step kinds = %v, want MODEL TOOL MODEL", kinds)
	}
	if h.tool.calls.Load() != 1 {
		t.Errorf("tool ran %d times, want once", h.tool.calls.Load())
	}
	if n := h.prov.requestCount(); n != 2 {
		t.Errorf("model asked %d times, want 2", n)
	}
}

// TestAgentParallelTools: the tool calls of one answer run together, never more
// than maxParallelToolCalls at a time, and each one is answered.
func TestAgentParallelTools(t *testing.T) {
	t.Parallel()
	tool := &fakeTool{answer: "ok", delay: 30 * time.Millisecond}
	h := newHarness(t, []reply{{resp: toolReply("lookup", "lookup", "lookup", "lookup")}, {resp: textReply("done")}}, nil, nil, tool)
	if _, err := h.run(t.Context(), agentParams(`"maxParallelToolCalls":2`), userInput, "ne-par"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := tool.calls.Load(); got != 4 {
		t.Errorf("tool ran %d times, want 4", got)
	}
	if got := tool.maxSeen.Load(); got > 2 {
		t.Errorf("up to %d tool calls ran at once, want at most 2", got)
	}
	if got := tool.maxSeen.Load(); got < 2 {
		t.Errorf("tool calls did not run in parallel (max %d)", got)
	}
}

// TestAgentFinalAnswer: an answer without a tool call ends the run in one step.
func TestAgentFinalAnswer(t *testing.T) {
	t.Parallel()
	h := newHarness(t, []reply{{resp: textReply("hola")}}, nil, nil, nil)
	out, err := h.run(t.Context(), agentParams(""), userInput, "ne-final")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := outText(t, out); got != "hola" {
		t.Errorf("answer = %q", got)
	}
	if steps, _ := h.store.Steps(t.Context(), testOrg(), agent.RunID(testOrg(), "exec-1", "ne-final", 0)); len(steps) != 1 {
		t.Errorf("steps = %d, want 1", len(steps))
	}
}

// TestAgentTranscriptAppendOnly: the transcript numbers its rows 1..n with no
// gap, and a finished run answers again from its record without asking the model.
func TestAgentTranscriptAppendOnly(t *testing.T) {
	t.Parallel()
	h := newHarness(t, []reply{{resp: toolReply("lookup")}, {resp: textReply("listo")}}, nil, nil, nil)
	if _, err := h.run(t.Context(), agentParams(""), userInput, "ne-tr"); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	rows, _ := h.store.Transcript(t.Context(), testOrg(), agent.RunID(testOrg(), "exec-1", "ne-tr", 0))
	for i, r := range rows {
		if r.Sequence != i+1 {
			t.Fatalf("row %d has sequence %d, want %d", i, r.Sequence, i+1)
		}
	}
	if len(rows) != 4 {
		t.Errorf("transcript has %d rows, want user, tool call, tool result, answer", len(rows))
	}
	again, err := h.run(t.Context(), agentParams(""), userInput, "ne-tr")
	if err != nil || outText(t, again) != "listo" {
		t.Fatalf("second Execute() = %v, %v; want the recorded answer", again, err)
	}
	if h.prov.requestCount() != 2 {
		t.Errorf("a finished run asked the model again (%d requests)", h.prov.requestCount())
	}
}

// TestAgentRerunFromStep: a model call that fails in flight is retried, and the
// steps committed before it are read back, not made again.
func TestAgentRerunFromStep(t *testing.T) {
	t.Parallel()
	transient := &provider.Error{Class: provider.ClassTransient, Status: 503, Code: "overloaded", Message: "busy"}
	h := newHarness(t, []reply{{resp: toolReply("lookup")}, {err: transient}}, nil, nil, nil)
	_, err := h.run(t.Context(), agentParams(""), userInput, "ne-rerun")
	if got := nodeCode(err); got != agent.CodeModelUnavailable {
		t.Fatalf("first attempt error code = %q, want %s (err %v)", got, agent.CodeModelUnavailable, err)
	}
	h.prov.script = append(h.prov.script, reply{resp: textReply("ya")})

	out, err := h.run(t.Context(), agentParams(""), userInput, "ne-rerun")
	if err != nil {
		t.Fatalf("retry error = %v", err)
	}
	if got := outText(t, out); got != "ya" {
		t.Errorf("answer = %q", got)
	}
	if h.tool.calls.Load() != 1 {
		t.Errorf("tool ran %d times across the attempts, want once", h.tool.calls.Load())
	}
	if n := h.prov.requestCount(); n != 3 {
		t.Errorf("model asked %d times, want 3: the answer, the failed call, the retry", n)
	}
	last := h.prov.requests[len(h.prov.requests)-1]
	if !hasRole(last.Messages, provider.RoleTool) {
		t.Errorf("the retry does not carry the recorded tool result")
	}
}

// TestAgentCapsSteps: the model may not be asked past maxSteps.
func TestAgentCapsSteps(t *testing.T) {
	t.Parallel()
	h := newHarness(t, []reply{{resp: toolReply("lookup")}, {resp: textReply("nunca")}}, nil, nil, nil)
	_, err := h.run(t.Context(), agentParams(`"maxSteps":1`), userInput, "ne-steps")
	if got := nodeCode(err); got != agent.CodeLimitExceeded {
		t.Errorf("error code = %q, want %s (err %v)", got, agent.CodeLimitExceeded, err)
	}
	if n := h.prov.requestCount(); n != 1 {
		t.Errorf("model asked %d times, want 1", n)
	}
}

// TestAgentCapsSpend: once the run has spent its maxSpend, it asks no more.
func TestAgentCapsSpend(t *testing.T) {
	t.Parallel()
	prices := fixedPrices{price: cost.Price{Input: 1_000_000}}
	h := newHarness(t, []reply{{resp: toolReply("lookup")}, {resp: textReply("nunca")}}, nil, prices, nil)
	_, err := h.run(t.Context(), agentParams(`"maxSpend":50`), userInput, "ne-spend")
	if got := nodeCode(err); got != agent.CodeLimitExceeded || !strings.Contains(err.Error(), "spend") {
		t.Errorf("error = %v (code %q), want a spend limit", err, got)
	}
}

// TestAgentInvalidArgumentsTwice: a second invalid call to the same tool in a
// row fails the run; the first one only tells the model what was wrong.
func TestAgentInvalidArgumentsTwice(t *testing.T) {
	t.Parallel()
	empty := provider.Response{ToolCalls: []provider.ToolCall{{ID: "c1", Name: "lookup", Arguments: mustParse(t, `{}`)}}, FinishReason: provider.FinishToolCalls}
	again := provider.Response{ToolCalls: []provider.ToolCall{{ID: "c2", Name: "lookup", Arguments: mustParse(t, `{}`)}}, FinishReason: provider.FinishToolCalls}
	h := newHarness(t, []reply{{resp: empty}, {resp: again}, {resp: textReply("nunca")}}, nil, nil, nil)
	_, err := h.run(t.Context(), agentParams(""), userInput, "ne-args")
	if got := nodeCode(err); got != agent.CodeArgumentsInvalid {
		t.Errorf("error code = %q, want %s (err %v)", got, agent.CodeArgumentsInvalid, err)
	}
	if h.tool.calls.Load() != 0 {
		t.Errorf("the tool ran %d times with invalid arguments", h.tool.calls.Load())
	}
}

// TestAgentMemory: the second run of a conversation starts with the first run's
// user message and answer, and the memory is written once per run.
func TestAgentMemory(t *testing.T) {
	t.Parallel()
	h := newHarness(t, []reply{{resp: textReply("¡hola!")}, {resp: textReply("claro")}}, nil, nil, nil)
	conv := `{"text":"hola","conversationKey":"wa:+57300"}`
	if _, err := h.run(t.Context(), agentParams(""), conv, "ne-m1"); err != nil {
		t.Fatalf("first run error = %v", err)
	}
	if _, err := h.run(t.Context(), agentParams(""), `{"text":"otra cosa","conversationKey":"wa:+57300"}`, "ne-m2"); err != nil {
		t.Fatalf("second run error = %v", err)
	}
	msgs := h.prov.requests[1].Messages
	if !containsText(msgs, "hola") || !containsText(msgs, "¡hola!") || !containsText(msgs, "otra cosa") {
		t.Errorf("second run messages = %v, want the first exchange before the new message", texts(msgs))
	}
}

// TestAgentBudgetRefusesBeforeCall: a budget already at its limit stops the run
// before the model is asked.
func TestAgentBudgetRefusesBeforeCall(t *testing.T) {
	t.Parallel()
	h := newHarness(t, []reply{{resp: textReply("nunca")}}, fakeBudgets{}, nil, nil)
	_, err := h.run(t.Context(), agentParams(""), userInput, "ne-budget")
	if got := nodeCode(err); got != agent.CodeBudgetExceeded {
		t.Errorf("error code = %q, want %s (err %v)", got, agent.CodeBudgetExceeded, err)
	}
	if n := h.prov.requestCount(); n != 0 {
		t.Errorf("model asked %d times past the budget", n)
	}
}

// TestAgentCancelAbortsCall: canceling a run aborts the call in flight and
// commits nothing for it.
func TestAgentCancelAbortsCall(t *testing.T) {
	t.Parallel()
	h := newHarness(t, []reply{{wait: true}}, nil, nil, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	_, err := h.run(ctx, agentParams(""), userInput, "ne-cancel")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want the context deadline", err)
	}
	if steps, _ := h.store.Steps(t.Context(), testOrg(), agent.RunID(testOrg(), "exec-1", "ne-cancel", 0)); len(steps) != 0 {
		t.Errorf("a canceled call left %d steps", len(steps))
	}
}

func hasRole(msgs []provider.Message, role provider.Role) bool {
	for _, m := range msgs {
		if m.Role == role {
			return true
		}
	}
	return false
}

func containsText(msgs []provider.Message, text string) bool {
	for i := range msgs {
		for j := range msgs[i].Parts {
			if strings.Contains(msgs[i].Parts[j].Text, text) {
				return true
			}
		}
	}
	return false
}

func texts(msgs []provider.Message) []string {
	var out []string
	for i := range msgs {
		for j := range msgs[i].Parts {
			if text := msgs[i].Parts[j].Text; text != "" {
				out = append(out, text)
			}
		}
	}
	return out
}
