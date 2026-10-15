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
// [execution.NodeRunner] for orcon.noop@1, orcon.set@1 and orcon.stopAndError@1.
type Runner struct {
	eval Evaluator
}

var _ execution.NodeRunner = (*Runner)(nil)

// NewRunner returns a runner. The evaluator ev resolves expressions in Set
// values; it may be nil, in which case a Set node with an expression fails the execution.
func NewRunner(ev Evaluator) *Runner { return &Runner{eval: ev} }

// Execute runs one activation of a core node. A node type this package does
// not define is an error, never a silent pass-through.
func (r *Runner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	if call.Node == nil {
		return execution.NodeOutput{}, errors.New("core: node call without a node")
	}
	switch call.Node.Type {
	case TypeNoOp:
		return output(call.Input.Items()), nil
	case TypeSet:
		return r.runSet(ctx, call)
	case TypeStopAndError:
		p, err := ParseStopAndError(call.Node.Parameters)
		if err != nil {
			return execution.NodeOutput{}, invalid(call.Node.Type, err)
		}
		return execution.NodeOutput{}, p.StopError()
	default:
		return execution.NodeOutput{}, fmt.Errorf("core: %q is not a core node type of this package", call.Node.Type)
	}
}

func (r *Runner) runSet(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	p, err := ParseSet(call.Node.Parameters)
	if err != nil {
		return execution.NodeOutput{}, invalid(call.Node.Type, err)
	}
	in := call.Input.Items()
	out := make([]execution.Item, 0, len(in))
	for i, it := range in {
		v, err := ApplySet(ctx, p, r.eval, it.JSON)
		if err != nil {
			return execution.NodeOutput{}, &execution.NodeError{
				Code:  "SET_FAILED",
				Class: execution.ClassPermanent,
				Err:   fmt.Errorf("item %d: %w", i, err),
			}
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
