// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package chain runs the single-call AI nodes of SPEC-006 (chapter 14): the
// basic LLM chain orcon.ai.llm@1 today, and the chains that build on it later.
// A node answers one call per item, returns text or a schema-checked JSON
// value, and moves to the next model of its fallback list only when the
// failure is transient (SPEC-006 R-PRV-06).
//
// The package is an area package. The model adapters are reached through the
// [Models] port, so it imports no adapter and no network code.
package chain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// LLMNodeType is the type of the basic LLM chain (SPEC-006 R-CH-01).
const LLMNodeType = "orcon.ai.llm@1"

// Error codes of the chain nodes. They reach the execution as [execution.NodeError]
// codes, so a flow can route them.
const (
	CodeModelUnavailable = "AI_MODEL_UNAVAILABLE"
	CodeProviderRejected = "AI_PROVIDER_REJECTED"
	CodeOutputInvalid    = "AI_OUTPUT_INVALID"
	CodeNodeInvalid      = "AI_NODE_INVALID"
)

// DefaultMaxRetryAfter is the longest Retry-After the node waits for before it
// moves to a fallback instead (SPEC-006 R-PRV-06).
const DefaultMaxRetryAfter = 10 * time.Second

// Models resolves the model adapter of a model reference for an organization.
// The composition root implements it: it chooses the adapter, the endpoint and
// the credential. A nil provider with an error means the model cannot be used.
type Models interface {
	Model(ctx context.Context, org identity.OrgID, ref flow.ModelRef) (provider.ModelProvider, error)
}

// Config are the ports and bounds of the chain runner.
type Config struct {
	Models Models
	// MaxRetryAfter bounds the wait for a 429 before falling back. Zero means
	// [DefaultMaxRetryAfter].
	MaxRetryAfter time.Duration
}

// LLM runs orcon.ai.llm@1.
type LLM struct {
	cfg Config
}

var _ execution.NodeRunner = (*LLM)(nil)

// NewLLM returns the runner of orcon.ai.llm@1.
func NewLLM(cfg Config) (*LLM, error) {
	if cfg.Models == nil {
		return nil, errors.New("chain: a model resolver is required")
	}
	if cfg.MaxRetryAfter <= 0 {
		cfg.MaxRetryAfter = DefaultMaxRetryAfter
	}
	return &LLM{cfg: cfg}, nil
}

// llmParams is the parsed parameter object of one orcon.ai.llm@1 node.
type llmParams struct {
	model          flow.ModelRef
	fallbacks      []flow.ModelRef
	system         string
	inputField     string
	maxOutput      int
	format         provider.ResponseFormat
	repairAttempts int
}

// parseLLM reads the parameters of the node. Tools are not part of this node:
// a flow that needs tool calls uses the agent node (SPEC-006 R-AG-01).
func parseLLM(node *flow.Node) (llmParams, error) {
	var out llmParams
	p := node.Parameters
	m, ok := p.Get("model")
	if !ok {
		return out, errors.New("orcon.ai.llm@1 needs a model")
	}
	if out.model, ok = refOf(m); !ok {
		return out, errors.New("orcon.ai.llm@1 model needs a provider and a name")
	}
	if fb, ok := p.Get("fallbacks"); ok {
		for _, item := range fb.Items() {
			ref, ok := refOf(item)
			if !ok {
				return out, errors.New("orcon.ai.llm@1 fallbacks need a provider and a name each")
			}
			out.fallbacks = append(out.fallbacks, ref)
		}
	}
	if _, ok := p.Get("tools"); ok {
		return out, errors.New("orcon.ai.llm@1 does not call tools; use orcon.ai.agent@1")
	}
	out.system = stringOf(p, "system")
	out.inputField = stringOf(p, "inputField")
	if out.inputField == "" {
		out.inputField = "text"
	}
	if n, ok := p.Get("maxOutputTokens"); ok {
		v, ok := n.Int64()
		if !ok || v <= 0 {
			return out, errors.New("orcon.ai.llm@1 maxOutputTokens must be a positive integer")
		}
		out.maxOutput = int(v)
	}
	out.format = provider.ResponseFormat{Kind: provider.FormatText}
	switch rf := stringOf(p, "responseFormat"); rf {
	case "", "TEXT":
	case "JSON":
		out.format = provider.ResponseFormat{Kind: provider.FormatJSON}
	case "JSON_SCHEMA":
		s, ok := p.Get("schema")
		if !ok {
			return out, errors.New("orcon.ai.llm@1 JSON_SCHEMA needs a schema")
		}
		out.format = provider.ResponseFormat{Kind: provider.FormatJSONSchema, Schema: s, Strict: true}
	default:
		return out, fmt.Errorf("orcon.ai.llm@1 responseFormat %q is not TEXT, JSON or JSON_SCHEMA", rf)
	}
	if n, ok := p.Get("repairAttempts"); ok {
		v, ok := n.Int64()
		if !ok || v < 0 || v > 5 {
			return out, errors.New("orcon.ai.llm@1 repairAttempts must be between 0 and 5")
		}
		out.repairAttempts = int(v)
	} else {
		out.repairAttempts = provider.DefaultRepairAttempts
	}
	return out, nil
}

// Execute implements [execution.NodeRunner]. Items are answered one at a time;
// an item whose every model failed transiently fails the activation with
// [CodeModelUnavailable], and the step is retried later by the engine.
func (l *LLM) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	if call.Node.Type != LLMNodeType {
		return execution.NodeOutput{}, fmt.Errorf("chain: node type %q is not %s", call.Node.Type, LLMNodeType)
	}
	params, err := parseLLM(call.Node)
	if err != nil {
		return execution.NodeOutput{}, &execution.NodeError{Code: CodeNodeInvalid, Class: execution.ClassPermanent, Err: err}
	}
	items := make([]execution.Item, 0, call.Input.Len())
	for i := range call.Input.Len() {
		out, err := l.answer(ctx, call.Org, params, call.Input.Item(i))
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

// answer runs one item against the primary model and then the fallbacks.
func (l *LLM) answer(ctx context.Context, org identity.OrgID, params llmParams, item execution.Item) (jsonvalue.Value, error) {
	text, ok := item.JSON.Get(params.inputField)
	prompt, isText := text.Str()
	if !ok || !isText {
		return jsonvalue.Value{}, &execution.NodeError{
			Code:  CodeNodeInvalid,
			Class: execution.ClassPermanent,
			Err:   fmt.Errorf("the item has no text field %q", params.inputField),
		}
	}
	refs := append([]flow.ModelRef{params.model}, params.fallbacks...)
	var lastErr error
	for _, ref := range refs {
		res, model, err := l.call(ctx, org, ref, params, prompt)
		if err == nil {
			return l.output(item, params, res, model)
		}
		if ctx.Err() != nil {
			return jsonvalue.Value{}, ctx.Err()
		}
		if !fallbackable(err) {
			return jsonvalue.Value{}, classify(err)
		}
		lastErr = err
	}
	return jsonvalue.Value{}, &execution.NodeError{
		Code:  CodeModelUnavailable,
		Class: execution.ClassTransient,
		Err:   fmt.Errorf("every model failed; last error: %w", lastErr),
	}
}

// call performs one answer with one model, structured or plain.
func (l *LLM) call(ctx context.Context, org identity.OrgID, ref flow.ModelRef, params llmParams, prompt string) (provider.StructuredResult, flow.ModelRef, error) {
	p, err := l.cfg.Models.Model(ctx, org, ref)
	if err != nil {
		return provider.StructuredResult{}, ref, err
	}
	req := provider.Request{
		Model:           ref.Name,
		Messages:        messages(params.system, prompt),
		MaxOutputTokens: params.maxOutput,
		ResponseFormat:  params.format,
	}
	if params.format.Kind == provider.FormatJSONSchema {
		res, err := provider.Structured(ctx, p, req, provider.StructuredOptions{RepairAttempts: params.repairAttempts})
		return res, ref, err
	}
	resp, err := p.Generate(ctx, req)
	if err != nil {
		return provider.StructuredResult{}, ref, err
	}
	res := provider.StructuredResult{Mode: provider.StructuredRepaired, Calls: 1, Value: jsonvalue.String(resp.Text)}
	if params.format.Kind == provider.FormatJSON {
		v, perr := jsonvalue.Parse([]byte(resp.Text))
		if perr != nil {
			return res, ref, fmt.Errorf("%w: the answer is not JSON", provider.ErrOutputInvalid)
		}
		res.Value = v
	}
	return res, ref, nil
}

func messages(system, prompt string) []provider.Message {
	var msgs []provider.Message
	if system != "" {
		msgs = append(msgs, provider.Message{Role: provider.RoleSystem, Parts: []provider.Part{{Kind: provider.PartText, Text: system}}})
	}
	return append(msgs, provider.Message{Role: provider.RoleUser, Parts: []provider.Part{{Kind: provider.PartText, Text: prompt}}})
}

// output builds the item the node emits: the input members plus the answer,
// the model that answered and the repair count (SPEC-006 R-PRV-06: the model
// that answered is recorded with the step).
func (l *LLM) output(item execution.Item, params llmParams, res provider.StructuredResult, ref flow.ModelRef) (jsonvalue.Value, error) {
	key := "text"
	if params.format.Kind != provider.FormatText {
		key = "json"
	}
	members := []jsonvalue.Member{}
	if item.JSON.Kind() == jsonvalue.KindObject {
		members = append(members, item.JSON.Members()...)
	}
	members = withMember(members, key, res.Value)
	members = withMember(members, "model", jsonvalue.String(ref.Provider+"/"+ref.Name))
	members = withMember(members, "repairs", jsonvalue.Int(int64(res.Repairs)))
	out, err := jsonvalue.Object(members...)
	if err != nil {
		return jsonvalue.Value{}, &execution.NodeError{Code: CodeNodeInvalid, Class: execution.ClassPermanent, Err: err}
	}
	return out, nil
}

// withMember sets key to v, replacing an existing member of the same name.
func withMember(members []jsonvalue.Member, key string, v jsonvalue.Value) []jsonvalue.Member {
	for i := range members {
		if members[i].Key == key {
			members[i].Value = v
			return members
		}
	}
	return append(members, jsonvalue.Member{Key: key, Value: v})
}

// fallbackable reports whether the next model in the list may be tried.
// Transient provider failures and transport failures qualify; validation,
// refusal and credential errors, and unsupported features, do not (R-PRV-06).
func fallbackable(err error) bool {
	if errors.Is(err, provider.ErrUnsupported) {
		return false
	}
	var pe *provider.Error
	if errors.As(err, &pe) {
		return pe.Class == provider.ClassTransient
	}
	if errors.Is(err, provider.ErrOutputInvalid) {
		return false
	}
	return true
}

// classify turns a failure of a model that cannot fall back into a node
// error with its class.
func classify(err error) error {
	var ne *execution.NodeError
	if errors.As(err, &ne) {
		return ne
	}
	if errors.Is(err, provider.ErrOutputInvalid) {
		return &execution.NodeError{Code: CodeOutputInvalid, Class: execution.ClassPermanent, Err: err}
	}
	return &execution.NodeError{Code: CodeProviderRejected, Class: execution.ClassPermanent, Err: err}
}

// refOf reads a {"provider", "name"} object.
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

func stringOf(p jsonvalue.Value, key string) string {
	v, ok := p.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.Str()
	return s
}
