// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
)

// Deps are the ports an [Interpreter] consumes. Each is needed only by the
// flows that use it: a flow without edge conditions does not need Conditions.
// Run reports [ErrMissingPort] before it starts anything when a flow needs a
// port that is nil.
type Deps struct {
	// Conditions decides the edge conditions of exclusive and inclusive gateways.
	Conditions ConditionEvaluator
	// Router routes the items of orcon.if and orcon.switch nodes.
	Router ItemRouter
	// Runner runs the activity nodes.
	Runner NodeRunner
	// Payloads stores the immutable batches. When nil, every run keeps its
	// batches in memory and drops them when the run ends.
	Payloads Payloads
	// Events receives the execution events in order. When nil the events are
	// only returned in [Result].
	Events EventSink
	// Clock gives each activation its start instant (ADR-0013 section 7). When
	// nil, activations carry a zero Time.
	Clock clock.Clock
}

// Interpreter runs compiled flows as token interpreters: tokens move through
// the graph node by node, fork at splits, are absorbed by joins, and the
// execution ends when no live token is left (ADR-0010, R-REC-01).
//
// This is the in-memory interpreter of the implementation cut C3. It keeps
// every table of a run in memory and runs one token at a time in a fixed
// order, so the same flow and input always produce the same trace. It has no
// storage, leases, expression sandbox or effects journal: those arrive through
// the ports of [Deps] and the durable driver of cut C4, which reuses the state
// machines, deterministic ids, join rules and plan of this package.
//
// An Interpreter holds no run state; one value can run many executions at the
// same time as long as its ports allow it.
type Interpreter struct {
	deps Deps
}

// NewInterpreter returns an interpreter over the given ports.
func NewInterpreter(deps Deps) *Interpreter {
	return &Interpreter{deps: deps}
}

// Request starts one execution.
type Request struct {
	// Execution identifies the execution. Every id the run derives (tokens,
	// node executions, payloads) is a function of it, so it must be unique.
	Execution ID
	// Org is the organization the execution belongs to; nodes that act on
	// external systems use it for their journal and their rate buckets.
	Org identity.OrgID
	// Start is the start event to begin at. It may be empty when the flow has
	// exactly one start event.
	Start flow.NodeID
	// Input is the batch the start event emits.
	Input Batch
}

// EndArrival records a token that reached an end event.
type EndArrival struct {
	Node  flow.NodeID
	Scope ScopePath
	Token TokenID
	// Batch is the data the token carried.
	Batch Batch
}

// TokenRecord is the final state of a token.
type TokenRecord struct {
	ID      TokenID
	State   TokenState
	Node    flow.NodeID
	Scope   ScopePath
	Lineage []ForkRef
	Payload PayloadRef
}

// NodeExecutionRecord is the final state of a node execution. Nodes on dead
// paths have none (ADR-0009 section 3).
type NodeExecutionRecord struct {
	ID    NodeExecutionID
	Token TokenID
	Node  flow.NodeID
	Scope ScopePath
	State NodeExecutionState
}

// Result is what a run produced.
type Result struct {
	Execution ID
	// Status is the final execution state: SUCCEEDED or FAILED, or RUNNING when
	// the run was interrupted by an error.
	Status State
	// Failure explains a FAILED execution.
	Failure *Failure
	// Events is the full trace, in emission order.
	Events []Event
	// Tokens lists every token ever created, in creation order.
	Tokens []TokenRecord
	// NodeExecutions lists every node execution, in creation order.
	NodeExecutions []NodeExecutionRecord
	// Ends lists the tokens that reached an end event, in arrival order.
	Ends []EndArrival
}

// Run executes the plan once and returns what happened.
//
// A flow-level failure (a node error, NoPathTaken, JoinRequirementNotMet,
// LoopLimitExceeded) is not an error of Run: it ends the execution in FAILED
// and is described by [Result.Failure]. Run returns an error when the request
// is invalid, a port is missing, the context ends, a port breaks its contract,
// or an internal invariant breaks; the returned Result then holds what
// happened until that point.
func (in *Interpreter) Run(ctx context.Context, plan *Plan, req Request) (Result, error) {
	if plan == nil {
		return Result{}, fmt.Errorf("%w: no plan", ErrInvalidRequest)
	}
	if req.Execution == "" {
		return Result{}, fmt.Errorf("%w: no execution id", ErrInvalidRequest)
	}
	start, err := resolveStart(plan, req.Start)
	if err != nil {
		return Result{Execution: req.Execution}, err
	}
	if err := in.checkPorts(plan); err != nil {
		return Result{Execution: req.Execution}, err
	}
	r := newRun(in, plan, req)
	err = r.execute(ctx, start)
	return r.result(), err
}

func resolveStart(plan *Plan, id flow.NodeID) (int, error) {
	if id != "" {
		i, ok := plan.index[id]
		if !ok || plan.nodes[i].kind != kindStart {
			return 0, fmt.Errorf("%w: %q is not a start event of the flow", ErrInvalidRequest, id)
		}
		return i, nil
	}
	switch len(plan.starts) {
	case 0:
		return 0, fmt.Errorf("%w: the flow has no start event", ErrInvalidRequest)
	case 1:
		return plan.starts[0], nil
	default:
		return 0, fmt.Errorf("%w: the flow has %d start events; name one in the request", ErrInvalidRequest, len(plan.starts))
	}
}

func (in *Interpreter) checkPorts(plan *Plan) error {
	switch {
	case plan.usesConditions && in.deps.Conditions == nil:
		return fmt.Errorf("%w: the flow has edge conditions and no ConditionEvaluator", ErrMissingPort)
	case plan.usesRouting && in.deps.Router == nil:
		return fmt.Errorf("%w: the flow has item-routing nodes and no ItemRouter", ErrMissingPort)
	case plan.usesActivities && in.deps.Runner == nil:
		return fmt.Errorf("%w: the flow has activity nodes and no NodeRunner", ErrMissingPort)
	default:
		return nil
	}
}
