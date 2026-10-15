// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package exprflow decides the conditions of a flow with the expression
// language of internal/expr (ADR-0010 section 4, ADR-0018). It implements the
// two ports the interpreter asks for control decisions: the verdict of an edge
// condition of an exclusive or inclusive gateway, and the output ports of the
// item-routing node orcon.if.
//
// The adapter is stateless apart from a bounded cache of compiled programs, so
// one value serves every execution of the process.
package exprflow

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/flow"
)

// IfNodeType is the item-routing node that routes items by a condition.
const IfNodeType = "orcon.if@1"

// ParamCondition is the parameter of orcon.if that holds the condition.
const ParamCondition = "condition"

// PortTrue and PortFalse are the output ports of orcon.if.
const (
	PortTrue  = "true"
	PortFalse = "false"
)

// ErrNotBoolean means a condition evaluated to something other than true or
// false. The interpreter fails the execution with the expression error code.
var ErrNotBoolean = errors.New("the condition did not evaluate to a boolean")

// ErrUnsupportedNode means the router was asked to route a node type it does
// not know.
var ErrUnsupportedNode = errors.New("item routing for this node type is not available")

// maxPrograms bounds the compiled-program cache; a flow has few conditions.
const maxPrograms = 1024

// Routing is the condition evaluator and item router of the engine.
type Routing struct {
	ev *expr.Evaluator

	mu       sync.Mutex
	programs map[string]*expr.Program
}

// Compile-time checks of the ports.
var (
	_ execution.ConditionEvaluator = (*Routing)(nil)
	_ execution.ItemRouter         = (*Routing)(nil)
)

// New returns the routing over an expression evaluator.
func New(ev *expr.Evaluator) *Routing {
	return &Routing{ev: ev, programs: map[string]*expr.Program{}}
}

// EvalCondition implements [execution.ConditionEvaluator]. A condition written
// as plain text is evaluated as the code of one segment; a template (a string
// starting with "=") is evaluated as written. The whole batch of the token is
// visible as $input.all(), and $json is its first item.
func (r *Routing) EvalCondition(ctx context.Context, expression string, env execution.EvalEnv) (bool, error) {
	prog, err := r.program(expression)
	if err != nil {
		return false, err
	}
	items := toExprItems(env.Items)
	return r.verdict(ctx, prog, &expr.Env{
		Input: items, ItemIndex: 0,
		Activation: expr.NewActivation(env.Env.Time, seedOf(env.Env.Seed)),
	})
}

// RouteItems implements [execution.ItemRouter] for orcon.if: each item goes to
// the port "true" or "false" by the value of its condition. The interpreter
// applies the routing mode (EXCLUSIVE or INCLUSIVE) on top of these answers.
func (r *Routing) RouteItems(ctx context.Context, call execution.RouteCall) ([][]string, error) {
	if call.Node == nil || call.Node.Type != IfNodeType {
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedNode, nodeType(call.Node))
	}
	source, err := conditionOf(call.Node)
	if err != nil {
		return nil, err
	}
	prog, err := r.program(source)
	if err != nil {
		return nil, err
	}
	items := toExprItems(call.Items)
	out := make([][]string, len(items))
	for i := range items {
		ok, err := r.verdict(ctx, prog, &expr.Env{
			Input: items, ItemIndex: i,
			Activation: expr.NewActivation(call.Env.Time, seedOf(call.Env.Seed)),
		})
		if err != nil {
			return nil, err
		}
		if ok {
			out[i] = []string{PortTrue}
		} else {
			out[i] = []string{PortFalse}
		}
	}
	return out, nil
}

// verdict evaluates one program in a fresh session and reads its boolean.
func (r *Routing) verdict(ctx context.Context, prog *expr.Program, env *expr.Env) (bool, error) {
	sess := r.ev.Open()
	defer func() { _ = sess.Close(ctx) }()
	res, err := sess.Eval(ctx, prog, env)
	if err != nil {
		return false, err
	}
	if res.Undefined {
		return false, ErrNotBoolean
	}
	b, ok := res.Value.Bool()
	if !ok {
		return false, ErrNotBoolean
	}
	return b, nil
}

// program compiles a condition once and keeps it for the next evaluations.
func (r *Routing) program(source string) (*expr.Program, error) {
	if !expr.IsTemplate(source) {
		source = "={{ " + source + " }}"
	}
	r.mu.Lock()
	if p, ok := r.programs[source]; ok {
		r.mu.Unlock()
		return p, nil
	}
	r.mu.Unlock()
	p, err := r.ev.Compile(source)
	if err != nil {
		return nil, fmt.Errorf("compile condition: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.programs) >= maxPrograms {
		clear(r.programs)
	}
	r.programs[source] = p
	return p, nil
}

// conditionOf reads the condition parameter of an orcon.if node.
func conditionOf(n *flow.Node) (string, error) {
	v, ok := n.Parameters.Get(ParamCondition)
	if !ok {
		return "", fmt.Errorf("node %q has no %s parameter", n.ID, ParamCondition)
	}
	s, ok := v.Str()
	if !ok {
		return "", fmt.Errorf("node %q: %s must be a string", n.ID, ParamCondition)
	}
	return s, nil
}

func toExprItems(b execution.Batch) []expr.Item {
	items := b.Items()
	out := make([]expr.Item, len(items))
	for i := range items {
		out[i] = expr.Item{JSON: items[i].JSON}
	}
	return out
}

// seedOf derives the 32-byte activation seed from the recorded 64-bit seed, so the
// same activation always draws the same stream.
func seedOf(s uint64) [expr.SeedSize]byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], s)
	return sha256.Sum256(b[:])
}

func nodeType(n *flow.Node) string {
	if n == nil {
		return "<nil>"
	}
	return n.Type
}
