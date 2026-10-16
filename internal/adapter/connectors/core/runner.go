// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/wirvii/orcon/internal/execution"
)

// outputPort is the port the core nodes emit on. Every node of this package
// has one output, which is the name every flow uses for it.
const outputPort = "main"

// codeInvalidParameters is the error code of a node whose parameters do not
// parse. Publication rejects such nodes first; this covers flows that reach
// the runner without that check.
const codeInvalidParameters = "INVALID_PARAMETERS"

// Runner executes the activity nodes of this package. It implements
// [execution.NodeRunner] for every type this package defines.
type Runner struct {
	eval Evaluator
}

var _ execution.NodeRunner = (*Runner)(nil)

// NewRunner returns a runner. The evaluator ev resolves the expressions of
// Set values and of Filter conditions; it may be nil, in which case a node
// that needs it fails the execution. An evaluator that also implements
// [ActivationEvaluator] is bound to the recorded environment of each
// activation first.
func NewRunner(ev Evaluator) *Runner { return &Runner{eval: ev} }

// nodeFunc runs one node type for one activation.
type nodeFunc func(ctx context.Context, r *Runner, call execution.NodeCall) (execution.NodeOutput, error)

// nodes is the run function of every node type of this package, by type.
var nodes = map[string]nodeFunc{
	TypeNoOp:         runNoOp,
	TypeSet:          runSet,
	TypeStopAndError: runStopAndError,
	TypeFilter:       runFilter,
	TypeSort:         runSort,
	TypeLimit:        runLimit,
}

// Execute runs one activation of a core node. A node type this package does
// not define is an error, never a silent pass-through.
func (r *Runner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	if call.Node == nil {
		return execution.NodeOutput{}, errors.New("core: node call without a node")
	}
	run, ok := nodes[call.Node.Type]
	if !ok {
		return execution.NodeOutput{}, fmt.Errorf("core: %q is not a core node type of this package", call.Node.Type)
	}
	return run(ctx, r, call)
}

// evaluator returns the evaluator of one activation: the runner's, bound to
// the recorded environment when it can be.
func (r *Runner) evaluator(call execution.NodeCall) Evaluator { //nolint:ireturn // the evaluator is a port; its implementation belongs to the composition root
	if b, ok := r.eval.(ActivationEvaluator); ok {
		return b.Bind(call.Env)
	}
	return r.eval
}

func runNoOp(_ context.Context, _ *Runner, call execution.NodeCall) (execution.NodeOutput, error) {
	return output(call.Input.Items()), nil
}

func runStopAndError(_ context.Context, _ *Runner, call execution.NodeCall) (execution.NodeOutput, error) {
	p, err := ParseStopAndError(call.Node.Parameters)
	if err != nil {
		return execution.NodeOutput{}, invalid(call.Node.Type, err)
	}
	return execution.NodeOutput{}, p.StopError()
}

func runSet(ctx context.Context, r *Runner, call execution.NodeCall) (execution.NodeOutput, error) {
	p, err := ParseSet(call.Node.Parameters)
	if err != nil {
		return execution.NodeOutput{}, invalid(call.Node.Type, err)
	}
	ev := r.evaluator(call)
	in := call.Input.Items()
	out := make([]execution.Item, 0, len(in))
	for i, it := range in {
		v, err := ApplySet(ctx, p, ev, it.JSON)
		if err != nil {
			return execution.NodeOutput{}, failure("SET_FAILED", fmt.Errorf("item %d: %w", i, err))
		}
		out = append(out, execution.Item{JSON: v, Lineage: it.Lineage})
	}
	return output(out), nil
}

func output(items []execution.Item) execution.NodeOutput {
	return execution.NodeOutput{Ports: []execution.PortBatch{{Port: outputPort, Batch: execution.NewBatch(items...)}}}
}

func invalid(nodeType string, err error) *execution.NodeError {
	return &execution.NodeError{
		Code:  codeInvalidParameters,
		Class: execution.ClassPermanent,
		Err:   fmt.Errorf("%s: %w", nodeType, err),
	}
}

// failure is the permanent error of a node whose input or data cannot be
// processed: running it again with the same input fails the same way.
func failure(code string, err error) *execution.NodeError {
	return &execution.NodeError{Code: code, Class: execution.ClassPermanent, Err: err}
}
