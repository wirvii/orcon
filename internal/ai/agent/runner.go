// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/wirvii/orcon/internal/ai/chain"
	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// NodeType is the type of the agent node (SPEC-006 R-AG-01).
const NodeType = "orcon.ai.agent@1"

// Error codes of the agent node. They reach the execution as node error codes.
const (
	CodeLimitExceeded    = "AI_AGENT_LIMIT_EXCEEDED"
	CodeArgumentsInvalid = "AI_TOOL_ARGUMENTS_INVALID"
	CodeBudgetExceeded   = "AI_BUDGET_EXCEEDED"
	CodeBudgetSuspended  = "AI_BUDGET_SUSPENDED"
	CodeModelUnavailable = "AI_MODEL_UNAVAILABLE"
	CodeProviderRejected = "AI_PROVIDER_REJECTED"
	CodeNodeInvalid      = "AI_NODE_INVALID"
)

// Default limits of an agent run (SPEC-006 R-AG-06).
const (
	DefaultMaxSteps         = 10
	DefaultMaxToolCalls     = 50
	DefaultMaxParallelTools = 4
	DefaultMemoryWindow     = 20
	defaultMaxOutputTokens  = 1024
)

// Prices resolves the price of a model. The boolean is false when the model has
// no price, so its cost is unknown.
type Prices interface {
	Price(ctx context.Context, org identity.OrgID, ref flow.ModelRef) (cost.Price, string, bool)
}

// Config are the ports of the agent runner.
type Config struct {
	Models chain.Models
	Store  Store
	// Prices may be nil: every model is then unpriced.
	Prices Prices
	// Budgets is read before each model call. It may be nil: no budget applies.
	Budgets cost.Store
	// Runners run the tools of the agent, by the node type each tool declares.
	Runners map[string]execution.NodeRunner
	// Scopes says which scopes a call's cost counts against. Nil means the
	// organization scope only.
	Scopes func(org identity.OrgID) []cost.Scope
	Clock  clock.Clock
}

// Runner runs orcon.ai.agent@1.
type Runner struct {
	cfg Config
}

var _ execution.NodeRunner = (*Runner)(nil)

// New returns the runner of the agent node. It validates the ports it needs.
func New(cfg Config) (*Runner, error) {
	if cfg.Models == nil || cfg.Store == nil {
		return nil, errors.New("agent: a model resolver and a store are required")
	}
	if cfg.Clock == nil {
		cfg.Clock = clock.System{}
	}
	if cfg.Scopes == nil {
		cfg.Scopes = func(org identity.OrgID) []cost.Scope {
			return []cost.Scope{{Kind: cost.ScopeOrg, ID: id.UUID(org)}}
		}
	}
	return &Runner{cfg: cfg}, nil
}

// toolDecl is one tool the node declares.
type toolDecl struct {
	name        string
	description string
	schema      jsonvalue.Value
	nodeType    string
	parameters  jsonvalue.Value
}

// params is the parsed parameter object of one agent node.
type params struct {
	model      flow.ModelRef
	system     string
	inputField string
	// conversationField is the item field that keys the conversation memory.
	conversationField string
	tools             []toolDecl
	maxSteps          int
	maxToolCalls      int
	maxParallel       int
	memoryWindow      int
	maxSpend          int64
	maxOutput         int
}

func parseParams(node *flow.Node) (params, error) {
	var p params
	raw := node.Parameters
	m, ok := raw.Get("model")
	if !ok {
		return p, errors.New("orcon.ai.agent@1 needs a model")
	}
	if p.model, ok = refOf(m); !ok {
		return p, errors.New("orcon.ai.agent@1 model needs a provider and a name")
	}
	p.system = stringOf(raw, "system")
	p.inputField = stringOf(raw, "inputField")
	if p.inputField == "" {
		p.inputField = "text"
	}
	// conversationField names the item field that keys the memory of the
	// conversation: a messaging flow sets it to the sender, so each sender keeps
	// its own memory.
	p.conversationField = stringOf(raw, "conversationField")
	if p.conversationField == "" {
		p.conversationField = "conversationKey"
	}
	var err error
	if p.maxSteps, err = intParam(raw, "maxSteps", DefaultMaxSteps); err != nil {
		return p, err
	}
	if p.maxToolCalls, err = intParam(raw, "maxToolCalls", DefaultMaxToolCalls); err != nil {
		return p, err
	}
	if p.maxParallel, err = intParam(raw, "maxParallelToolCalls", DefaultMaxParallelTools); err != nil {
		return p, err
	}
	if p.memoryWindow, err = intParam(raw, "memoryWindow", DefaultMemoryWindow); err != nil {
		return p, err
	}
	if p.maxOutput, err = intParam(raw, "maxOutputTokens", defaultMaxOutputTokens); err != nil {
		return p, err
	}
	if v, ok := raw.Get("maxSpend"); ok {
		n, ok := v.Int64()
		if !ok || n <= 0 {
			return p, errors.New("orcon.ai.agent@1 maxSpend must be a positive number of micro-dollars")
		}
		p.maxSpend = n
	}
	if p.maxParallel < 1 {
		return p, errors.New("orcon.ai.agent@1 maxParallelToolCalls must be at least 1")
	}
	if tools, ok := raw.Get("tools"); ok {
		for i, t := range tools.Items() {
			d, err := parseTool(t)
			if err != nil {
				return p, fmt.Errorf("orcon.ai.agent@1 tools[%d]: %w", i, err)
			}
			p.tools = append(p.tools, d)
		}
	}
	return p, nil
}

func parseTool(v jsonvalue.Value) (toolDecl, error) {
	d := toolDecl{
		name:        stringOf(v, "name"),
		description: stringOf(v, "description"),
		nodeType:    stringOf(v, "nodeType"),
	}
	if d.name == "" || d.nodeType == "" {
		return d, errors.New("a tool needs a name and a nodeType")
	}
	if s, ok := v.Get("schema"); ok {
		d.schema = s
	}
	if p, ok := v.Get("parameters"); ok {
		d.parameters = p
	}
	return d, nil
}

func intParam(p jsonvalue.Value, key string, def int) (int, error) {
	v, ok := p.Get(key)
	if !ok {
		return def, nil
	}
	n, ok := v.Int64()
	if !ok || n < 0 || n > 1_000_000 {
		return 0, fmt.Errorf("orcon.ai.agent@1 %s must be a whole number from 0 to 1000000", key)
	}
	return int(n), nil
}

// Execute implements [execution.NodeRunner]. Each item is one run.
func (r *Runner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	if call.Node.Type != NodeType {
		return execution.NodeOutput{}, fmt.Errorf("agent: node type %q is not %s", call.Node.Type, NodeType)
	}
	p, err := parseParams(call.Node)
	if err != nil {
		return execution.NodeOutput{}, &execution.NodeError{Code: CodeNodeInvalid, Class: execution.ClassPermanent, Err: err}
	}
	items := make([]execution.Item, 0, call.Input.Len())
	for i := range call.Input.Len() {
		out, err := r.runItem(ctx, call, p, i)
		if err != nil {
			return execution.NodeOutput{}, err
		}
		items = append(items, execution.Item{JSON: out})
	}
	if len(items) == 0 {
		return execution.NodeOutput{}, nil
	}
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: execution.NewBatch(items...)}}}, nil
}

// runItem drives one run to its answer. A run that is already recorded as
// finished returns its answer; otherwise the steps it committed are replayed
// and the loop continues from the first step that is missing.
func (r *Runner) runItem(ctx context.Context, call execution.NodeCall, p params, i int) (jsonvalue.Value, error) {
	org := call.Org
	item := call.Input.Item(i)
	runID := RunID(org, string(call.Execution), string(call.NodeExecution), i)

	state, found, err := r.cfg.Store.Run(ctx, org, runID)
	if err != nil {
		return jsonvalue.Value{}, transient(err)
	}
	if found && state.Status == StatusSucceeded {
		return outputOf(item, p, runID, state.FinalText, state.StepCount)
	}
	if found && state.Status == StatusFailed {
		return jsonvalue.Value{}, &execution.NodeError{Code: state.ErrorCode, Class: execution.ClassPermanent,
			Err: errors.New("the agent run already failed")}
	}
	userText, ok := textField(item.JSON, p.inputField)
	if !ok {
		return jsonvalue.Value{}, &execution.NodeError{Code: CodeNodeInvalid, Class: execution.ClassPermanent,
			Err: fmt.Errorf("the item has no text field %q", p.inputField)}
	}
	conversation := stringOf(item.JSON, p.conversationField)

	steps, err := r.cfg.Store.Steps(ctx, org, runID)
	if err != nil {
		return jsonvalue.Value{}, transient(err)
	}
	transcript, err := r.cfg.Store.Transcript(ctx, org, runID)
	if err != nil {
		return jsonvalue.Value{}, transient(err)
	}
	run := &runState{
		call: call, p: p, runID: runID, org: org, user: userText, conversation: conversation,
		steps: steps, transcript: transcript,
	}
	if len(transcript) == 0 {
		run.first = true
		if conversation != "" {
			mem, err := r.cfg.Store.Memory(ctx, org, conversation, p.memoryWindow)
			if err != nil {
				return jsonvalue.Value{}, transient(err)
			}
			run.memory = mem
		}
		run.pending = initialRows(run.memory, userText, len(transcript))
	}
	text, stepCount, err := r.loop(ctx, run)
	if err != nil {
		return jsonvalue.Value{}, err
	}
	return outputOf(item, p, runID, text, stepCount)
}

// runState is what the loop of one run carries between steps.
type runState struct {
	call         execution.NodeCall
	p            params
	runID        id.UUID
	org          identity.OrgID
	user         string
	conversation string
	steps        []StepRecord
	transcript   []TranscriptRow
	// pending are the rows of the first commit: the memory and the user message.
	pending []TranscriptRow
	memory  []MemoryRow
	first   bool
}

func initialRows(memory []MemoryRow, user string, start int) []TranscriptRow {
	rows := make([]TranscriptRow, 0, len(memory)+1)
	seq := start
	for _, m := range memory {
		seq++
		rows = append(rows, TranscriptRow{Sequence: seq, Role: m.Role, Part: PartText, Text: m.Text})
	}
	seq++
	return append(rows, TranscriptRow{Sequence: seq, Role: provider.RoleUser, Part: PartText, Text: user})
}

// loop alternates model calls and tool calls until the model answers without
// a tool call. Replay is the same loop: a step already committed is read from
// the store rather than made again.
func (r *Runner) loop(ctx context.Context, run *runState) (string, int, error) {
	for {
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		pending := pendingCalls(run.steps)
		if len(pending) > 0 {
			if err := r.runTools(ctx, run, pending); err != nil {
				return "", 0, err
			}
			continue
		}
		if err := r.checkLimits(run); err != nil {
			return "", 0, err
		}
		messages := r.messagesFor(run)
		resp, use, err := r.callModel(ctx, run, messages)
		if err != nil {
			return "", 0, err
		}
		doc, err := encodeTurn(resp)
		if err != nil {
			return "", 0, transient(err)
		}
		final := len(resp.ToolCalls) == 0
		step := StepRecord{Number: len(run.steps) + 1, Kind: StepModel, Output: doc, Model: use}
		rows := rowsOfTurn(resp, len(run.transcript)+len(run.pending)+1)
		commit := Commit{
			RunID: run.runID, ConversationKey: run.conversation, Step: step,
			Rows: append(run.pending, rows...), Final: final, FinalText: resp.Text,
			Memory: run.memoryForFirst(), Cost: use.entry(), Scopes: r.cfg.Scopes(run.org), At: r.cfg.Clock.Now(),
		}
		if final {
			commit.Remember = []MemoryRow{{Role: provider.RoleUser, Text: run.user}, {Role: provider.RoleAssistant, Text: resp.Text}}
		}
		if err := r.cfg.Store.Commit(ctx, run.org, commit); err != nil {
			return "", 0, transient(err)
		}
		run.steps = append(run.steps, step)
		run.transcript = append(run.transcript, run.pending...)
		run.transcript = append(run.transcript, rows...)
		run.pending = nil
		run.first = false
		if final {
			return resp.Text, len(run.steps), nil
		}
	}
}

func (run *runState) memoryForFirst() []MemoryRow {
	if !run.first {
		return nil
	}
	return run.memory
}

// messagesFor builds the request messages: the system instruction, then the
// transcript as committed, then the pending rows of the first commit.
func (r *Runner) messagesFor(run *runState) []provider.Message {
	rows := append(append([]TranscriptRow(nil), run.transcript...), run.pending...)
	msgs := messagesOf(rows)
	if run.p.system != "" {
		msgs = append([]provider.Message{{Role: provider.RoleSystem, Parts: []provider.Part{{Kind: provider.PartText, Text: run.p.system}}}}, msgs...)
	}
	return msgs
}

// pendingCalls returns the tool calls of the last model answer that have no
// committed result yet, in the order the model asked for them.
func pendingCalls(steps []StepRecord) []provider.ToolCall {
	var last *StepRecord
	answered := map[string]bool{}
	for i := range steps {
		switch steps[i].Kind {
		case StepModel:
			last = &steps[i]
			answered = map[string]bool{}
		case StepTool:
			answered[steps[i].ToolCallID] = true
		}
	}
	if last == nil {
		return nil
	}
	t, err := decodeTurn(last.Output)
	if err != nil {
		return nil
	}
	var out []provider.ToolCall
	for i := range t.Calls {
		c := t.Calls[i]
		if !answered[c.ID] {
			out = append(out, c)
		}
	}
	return out
}

// checkLimits enforces the caps of the run before a model call.
func (r *Runner) checkLimits(run *runState) error {
	models, tools := 0, 0
	var spent int64
	for i := range run.steps {
		s := run.steps[i]
		switch s.Kind {
		case StepModel:
			models++
			spent += s.Model.CostMicro
		case StepTool:
			tools++
		}
	}
	if models >= run.p.maxSteps {
		return limit("steps", models, run.p.maxSteps)
	}
	if tools > run.p.maxToolCalls {
		return limit("toolCalls", tools, run.p.maxToolCalls)
	}
	if run.p.maxSpend > 0 && spent >= run.p.maxSpend {
		return limit("spend", int(spent), int(run.p.maxSpend))
	}
	return nil
}

func limit(kind string, got, bound int) error {
	return &execution.NodeError{Code: CodeLimitExceeded, Class: execution.ClassPermanent,
		Err: fmt.Errorf("AgentLimitExceeded{kind=%s}: %d of %d", kind, got, bound)}
}

// runTools runs the tool calls of one model answer, at most maxParallel at a
// time, and commits each result as its own step in the order the model asked.
func (r *Runner) runTools(ctx context.Context, run *runState, calls []provider.ToolCall) error {
	toolCount := 0
	for i := range run.steps {
		if run.steps[i].Kind == StepTool {
			toolCount++
		}
	}
	if toolCount+len(calls) > run.p.maxToolCalls {
		return limit("toolCalls", toolCount+len(calls), run.p.maxToolCalls)
	}
	results := make([]StepRecord, len(calls))
	sem := make(chan struct{}, run.p.maxParallel)
	var wg sync.WaitGroup
	for i := range calls {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = r.runOneTool(ctx, run, calls[i], i)
		}(i)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	for i := range results {
		s := results[i]
		if s.ArgumentsInvalid && consecutiveInvalid(run.steps, s.ToolName) >= 1 {
			return &execution.NodeError{Code: CodeArgumentsInvalid, Class: execution.ClassPermanent,
				Err: fmt.Errorf("tool %s received invalid arguments twice in a row", s.ToolName)}
		}
		s.Number = len(run.steps) + 1
		row := TranscriptRow{
			Sequence: len(run.transcript) + len(run.pending) + 1, Role: provider.RoleTool, Part: PartToolResult,
			CallID: s.ToolCallID, ToolName: s.ToolName, Text: s.Output, IsError: s.IsError,
		}
		commit := Commit{RunID: run.runID, ConversationKey: run.conversation, Step: s, Rows: append(run.pending, row), At: r.cfg.Clock.Now()}
		if err := r.cfg.Store.Commit(ctx, run.org, commit); err != nil {
			return transient(err)
		}
		run.steps = append(run.steps, s)
		run.transcript = append(run.transcript, run.pending...)
		run.transcript = append(run.transcript, row)
		run.pending = nil
		run.first = false
	}
	return nil
}

// consecutiveInvalid counts the invalid-argument results at the end of the
// steps for one tool, without a valid call in between.
func consecutiveInvalid(steps []StepRecord, tool string) int {
	n := 0
	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		if s.Kind != StepTool {
			continue
		}
		if s.ToolName != tool {
			continue
		}
		if !s.ArgumentsInvalid {
			break
		}
		n++
	}
	return n
}

// runOneTool validates the arguments and runs the tool node. A failure of the
// tool is an error result the model sees, not a failure of the run.
func (r *Runner) runOneTool(ctx context.Context, run *runState, c provider.ToolCall, idx int) StepRecord {
	step := StepRecord{Kind: StepTool, ToolCallID: c.ID, ToolName: c.Name}
	decl, ok := findTool(run.p.tools, c.Name)
	if !ok {
		step.IsError = true
		step.Output = "the tool " + c.Name + " is not declared by this agent"
		return step
	}
	if c.ParseError != "" {
		step.IsError, step.ArgumentsInvalid = true, true
		step.Output = "the arguments are not valid JSON: " + c.ParseError
		return step
	}
	if err := provider.ValidateArguments(decl.schema, c.Arguments); err != nil {
		step.IsError, step.ArgumentsInvalid = true, true
		step.Output = "the arguments do not match the schema: " + err.Error()
		return step
	}
	runner, ok := r.cfg.Runners[decl.nodeType]
	if !ok {
		step.IsError = true
		step.Output = "no runner is installed for " + decl.nodeType
		return step
	}
	node := &flow.Node{ID: flow.NodeID(decl.name), Name: decl.name, Type: decl.nodeType, Parameters: decl.parameters}
	out, err := runner.Execute(ctx, execution.NodeCall{
		Org: run.org, Execution: run.call.Execution, NodeExecution: execution.NodeExecutionID(run.runID.String() + ":" + strconv.Itoa(idx)),
		Node: node, Input: execution.NewBatch(execution.Item{JSON: c.Arguments}),
	})
	if err != nil {
		step.IsError = true
		step.Output = "the tool failed: " + err.Error()
		return step
	}
	step.Output = resultText(out)
	return step
}

func findTool(tools []toolDecl, name string) (toolDecl, bool) {
	for i := range tools {
		if tools[i].name == name {
			return tools[i], true
		}
	}
	return toolDecl{}, false
}

// resultText is the text the model sees for a tool: the first item of the
// first output port, as JSON.
func resultText(out execution.NodeOutput) string {
	for i := range out.Ports {
		pb := &out.Ports[i]
		if pb.Batch.Len() == 0 {
			continue
		}
		b, err := jsonvalue.Marshal(pb.Batch.Item(0).JSON)
		if err != nil {
			return ""
		}
		return string(b)
	}
	return ""
}

// callModel asks the model for its next answer, after the budget admits the call.
func (r *Runner) callModel(ctx context.Context, run *runState, msgs []provider.Message) (provider.Response, ModelUse, error) {
	prov, err := r.cfg.Models.Model(ctx, run.org, run.p.model)
	if err != nil {
		return provider.Response{}, ModelUse{}, transient(err)
	}
	price, version, priced := r.price(ctx, run.org, run.p.model)
	if err := r.admit(ctx, run, msgs, price, priced); err != nil {
		return provider.Response{}, ModelUse{}, err
	}
	tools := make([]provider.Tool, 0, len(run.p.tools))
	for i := range run.p.tools {
		t := &run.p.tools[i]
		tools = append(tools, provider.Tool{Name: t.name, Description: t.description, Schema: t.schema})
	}
	if len(tools) > 0 && !prov.Capabilities(run.p.model.Name).Tools {
		return provider.Response{}, ModelUse{}, &execution.NodeError{Code: CodeNodeInvalid, Class: execution.ClassPermanent,
			Err: fmt.Errorf("model %s/%s does not support tool calling", run.p.model.Provider, run.p.model.Name)}
	}
	resp, err := prov.Generate(ctx, provider.Request{
		Model: run.p.model.Name, Messages: msgs, Tools: tools,
		ToolChoice: provider.ToolChoice{Mode: provider.ToolChoiceAuto}, MaxOutputTokens: run.p.maxOutput,
	})
	if err != nil {
		if ctx.Err() != nil {
			return provider.Response{}, ModelUse{}, ctx.Err()
		}
		if provider.IsTransient(err) || !isProviderError(err) {
			return provider.Response{}, ModelUse{}, &execution.NodeError{Code: CodeModelUnavailable, Class: execution.ClassTransient, Err: err}
		}
		return provider.Response{}, ModelUse{}, &execution.NodeError{Code: CodeProviderRejected, Class: execution.ClassPermanent, Err: err}
	}
	usage := cost.Usage{
		Input: resp.Usage.Input, Output: resp.Usage.Output, CacheRead: resp.Usage.CacheRead,
		CacheWrite: resp.Usage.CacheWrite, Reasoning: resp.Usage.Reasoning,
	}
	use := ModelUse{
		Provider: run.p.model.Provider, Model: run.p.model.Name, Usage: usage, Priced: priced,
		PriceVersion: version, FinishReason: resp.FinishReason, CostStatus: "UNKNOWN",
	}
	if priced {
		use.CostMicro = cost.UsageCost(price, usage)
		use.CostStatus = "ESTIMATED"
	}
	return resp, use, nil
}

func isProviderError(err error) bool {
	var pe *provider.Error
	return errors.As(err, &pe)
}

// admit refuses or suspends the call when a budget would be exceeded by its
// reservation: the counted input at the input price plus the largest output.
func (r *Runner) admit(ctx context.Context, run *runState, msgs []provider.Message, price cost.Price, priced bool) error {
	if r.cfg.Budgets == nil {
		return nil
	}
	var inputChars int64
	for i := range msgs {
		for j := range msgs[i].Parts {
			part := &msgs[i].Parts[j]
			inputChars += int64(len(part.Text) + len(part.ToolResult.Output) + len(part.ToolCall.RawArguments))
		}
	}
	reservation := cost.Reservation(price, inputChars/4+1, int64(run.p.maxOutput))
	d, err := cost.Admit(ctx, r.cfg.Budgets, run.org, r.cfg.Scopes(run.org), reservation, priced, r.cfg.Clock.Now())
	if err != nil {
		var exceeded *cost.BudgetExceededError
		if errors.As(err, &exceeded) {
			return &execution.NodeError{Code: CodeBudgetExceeded, Class: execution.ClassPermanent, Err: err}
		}
		if errors.Is(err, cost.ErrUnknownPrice) {
			return &execution.NodeError{Code: CodeBudgetExceeded, Class: execution.ClassPermanent, Err: err}
		}
		return transient(err)
	}
	if d.Suspend {
		return &execution.NodeError{Code: CodeBudgetSuspended, Class: execution.ClassTransient,
			Err: errors.New("a budget is exhausted; the run waits until it is raised")}
	}
	return nil
}

func (r *Runner) price(ctx context.Context, org identity.OrgID, ref flow.ModelRef) (cost.Price, string, bool) {
	if r.cfg.Prices == nil {
		return cost.Price{}, "", false
	}
	p, v, ok := r.cfg.Prices.Price(ctx, org, ref)
	return p, v, ok
}

// entry turns the use of one model call into the cost entry the rollups record.
func (u ModelUse) entry() *cost.Entry {
	return &cost.Entry{
		Purpose:      cost.PurposeAgent,
		Provider:     u.Provider,
		Model:        u.Model,
		Usage:        u.Usage,
		CostMicro:    u.CostMicro,
		Priced:       u.Priced,
		PriceVersion: u.PriceVersion,
	}
}

// transient marks an infrastructure failure as retryable.
func transient(err error) error {
	var ne *execution.NodeError
	if errors.As(err, &ne) {
		return err
	}
	return &execution.NodeError{Code: CodeModelUnavailable, Class: execution.ClassTransient, Err: err}
}

func textField(v jsonvalue.Value, key string) (string, bool) {
	f, ok := v.Get(key)
	if !ok {
		return "", false
	}
	return f.Str()
}

func stringOf(p jsonvalue.Value, key string) string {
	v, ok := p.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.Str()
	return s
}

func refOf(v jsonvalue.Value) (flow.ModelRef, bool) {
	p, okP := v.Get("provider")
	n, okN := v.Get("name")
	if !okP || !okN {
		return flow.ModelRef{}, false
	}
	prov, _ := p.Str()
	name, _ := n.Str()
	if prov == "" || name == "" {
		return flow.ModelRef{}, false
	}
	return flow.ModelRef{Provider: prov, Name: name}, true
}

// outputOf builds the item the node emits: the input members, the answer, the
// run identity and the number of steps the run took.
func outputOf(item execution.Item, p params, runID id.UUID, text string, steps int) (jsonvalue.Value, error) {
	members := []jsonvalue.Member{}
	if item.JSON.Kind() == jsonvalue.KindObject {
		members = append(members, item.JSON.Members()...)
	}
	members = withMember(members, "text", jsonvalue.String(text))
	members = withMember(members, "model", jsonvalue.String(p.model.Provider+"/"+p.model.Name))
	members = withMember(members, "steps", jsonvalue.Int(int64(steps)))
	members = withMember(members, "runId", jsonvalue.String(runID.String()))
	out, err := jsonvalue.Object(members...)
	if err != nil {
		return jsonvalue.Value{}, &execution.NodeError{Code: CodeNodeInvalid, Class: execution.ClassPermanent, Err: err}
	}
	return out, nil
}

func withMember(members []jsonvalue.Member, key string, v jsonvalue.Value) []jsonvalue.Member {
	for i := range members {
		if members[i].Key == key {
			members[i].Value = v
			return members
		}
	}
	return append(members, jsonvalue.Member{Key: key, Value: v})
}
