// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package executiontest

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// JSONItem parses one JSON text into an item. It panics on invalid JSON, which
// is a bug in the test.
func JSONItem(text string) execution.Item {
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		panic(fmt.Sprintf("executiontest: bad item %q: %v", text, err))
	}
	return execution.Item{JSON: v}
}

// Batch builds a batch from JSON texts, one item each.
func Batch(texts ...string) execution.Batch {
	items := make([]execution.Item, len(texts))
	for i, t := range texts {
		items[i] = JSONItem(t)
	}
	return execution.NewBatch(items...)
}

// Texts renders the items of a batch as canonical JSON texts.
func Texts(b execution.Batch) []string {
	out := make([]string, 0, b.Len())
	for _, it := range b.Items() {
		raw, err := jsonvalue.Canonical(it.JSON)
		if err != nil {
			panic(fmt.Sprintf("executiontest: encode item: %v", err))
		}
		out = append(out, string(raw))
	}
	return out
}

// Conditions is a [execution.ConditionEvaluator] that looks the expression up
// in a table. An expression that is not in the table is an error, so a test
// cannot depend on a condition it forgot to declare.
type Conditions map[string]bool

// EvalCondition implements [execution.ConditionEvaluator].
func (c Conditions) EvalCondition(_ context.Context, expression string, _ execution.EvalEnv) (bool, error) {
	v, ok := c[expression]
	if !ok {
		return false, fmt.Errorf("executiontest: no verdict for condition %q", expression)
	}
	return v, nil
}

// ConditionFunc adapts a function to [execution.ConditionEvaluator].
type ConditionFunc func(expression string, env execution.EvalEnv) (bool, error)

// EvalCondition implements [execution.ConditionEvaluator].
func (f ConditionFunc) EvalCondition(_ context.Context, expression string, env execution.EvalEnv) (bool, error) {
	return f(expression, env)
}

// RouterFunc is an [execution.ItemRouter] that decides the ports of one item
// at a time. The function gets the node, so one router serves a whole flow.
type RouterFunc func(node *flow.Node, item execution.Item) []string

// RouteItems implements [execution.ItemRouter].
func (f RouterFunc) RouteItems(_ context.Context, call execution.RouteCall) ([][]string, error) {
	items := call.Items.Items()
	out := make([][]string, len(items))
	for i, it := range items {
		out[i] = f(call.Node, it)
	}
	return out, nil
}

// FieldRouter returns a [RouterFunc] that sends an item to the port named by
// the string value of its JSON field, or to no port when the field is absent
// or not a string.
func FieldRouter(field string) RouterFunc {
	return func(_ *flow.Node, it execution.Item) []string {
		v, ok := it.JSON.Get(field)
		if !ok {
			return nil
		}
		s, ok := v.Str()
		if !ok {
			return nil
		}
		return []string{s}
	}
}

// TraceRunner is an [execution.NodeRunner] that passes the items through on
// the main port after appending the node id to the "path" array of every item.
// The path of an item therefore shows which nodes it went through, which makes
// data flow visible in assertions. It records the nodes it ran, in order.
//
// A node listed in Behaviors runs that function instead. TraceRunner is safe
// for concurrent use.
type TraceRunner struct {
	Behaviors map[flow.NodeID]func(execution.NodeCall) (execution.NodeOutput, error)

	mu    sync.Mutex
	calls []flow.NodeID
}

// Execute implements [execution.NodeRunner].
func (r *TraceRunner) Execute(_ context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	r.mu.Lock()
	r.calls = append(r.calls, call.Node.ID)
	r.mu.Unlock()
	if fn, ok := r.Behaviors[call.Node.ID]; ok {
		return fn(call)
	}
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: AppendPath(call.Input, string(call.Node.ID))}}}, nil
}

// Calls returns the ids of the nodes the runner ran, in order.
func (r *TraceRunner) Calls() []flow.NodeID {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]flow.NodeID(nil), r.calls...)
}

// AppendPath returns the batch with node appended to the "path" array of every
// item. An item that is not an object is returned unchanged.
func AppendPath(in execution.Batch, node string) execution.Batch {
	items := in.Items()
	for i, it := range items {
		if it.JSON.Kind() != jsonvalue.KindObject {
			continue
		}
		var path []jsonvalue.Value
		if cur, ok := it.JSON.Get("path"); ok {
			path = cur.Items()
		}
		path = append(path, jsonvalue.String(node))
		items[i].JSON = it.JSON.With("path", jsonvalue.Array(path...))
	}
	return execution.NewBatch(items...)
}

// Increment returns a [TraceRunner] behavior that adds one to the integer
// field of every item (a missing field counts as zero) and appends the node to
// the item path.
func Increment(field string) func(execution.NodeCall) (execution.NodeOutput, error) {
	return func(call execution.NodeCall) (execution.NodeOutput, error) {
		items := AppendPath(call.Input, string(call.Node.ID)).Items()
		for i, it := range items {
			n := int64(0)
			if v, ok := it.JSON.Get(field); ok {
				n, _ = v.Int64()
			}
			items[i].JSON = it.JSON.With(field, jsonvalue.Int(n+1))
		}
		return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: execution.NewBatch(items...)}}}, nil
	}
}

// SetField returns a [TraceRunner] behavior that sets the integer field of
// every item to value and appends the node to the item path.
func SetField(field string, value int64) func(execution.NodeCall) (execution.NodeOutput, error) {
	return func(call execution.NodeCall) (execution.NodeOutput, error) {
		items := AppendPath(call.Input, string(call.Node.ID)).Items()
		for i, it := range items {
			items[i].JSON = it.JSON.With(field, jsonvalue.Int(value))
		}
		return execution.NodeOutput{Ports: []execution.PortBatch{{Port: flow.PortMain, Batch: execution.NewBatch(items...)}}}, nil
	}
}

// WhileLess returns a [RouterFunc] that sends an item to port "true" while the
// integer field is below limit and to port "false" from then on.
func WhileLess(field string, limit int64) RouterFunc {
	return func(_ *flow.Node, it execution.Item) []string {
		n := int64(0)
		if v, ok := it.JSON.Get(field); ok {
			n, _ = v.Int64()
		}
		if n < limit {
			return []string{"true"}
		}
		return []string{"false"}
	}
}

// ErrBoom is the error the failing fakes return.
var ErrBoom = errors.New("executiontest: boom")

// Recorder is an [execution.EventSink] that keeps the events it receives. It
// fails with Err, when set, on the event with sequence number FailAt.
type Recorder struct {
	FailAt int
	Err    error

	mu     sync.Mutex
	events []execution.Event
}

// Emit implements [execution.EventSink].
func (r *Recorder) Emit(_ context.Context, e execution.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.FailAt != 0 && e.Sequence == r.FailAt {
		return r.Err
	}
	r.events = append(r.events, e)
	return nil
}

// Events returns the events received.
func (r *Recorder) Events() []execution.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]execution.Event(nil), r.events...)
}
