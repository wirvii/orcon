// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"slices"

	"github.com/wirvii/orcon/internal/flow"
)

// guard runs a port call and turns a panic into a [PanicError]: the engine
// recovers at the port boundary, never deeper (go-style.md GS-E6).
func guard[T any](call func() (T, error)) (out T, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = &PanicError{Value: p, Stack: debug.Stack()}
		}
	}()
	return call()
}

// step processes a leased token at its node and returns the token that
// continues on this worker, if any (R-REC-01: the worker keeps one branch).
func (r *run) step(ctx context.Context, t *token) (*token, error) {
	n := &r.plan.nodes[t.node]
	if n.join != nil && !t.passed {
		return nil, r.arrive(ctx, t, n)
	}
	if n.kind == kindLoop {
		return r.stepLoop(t, n)
	}
	return r.stepNode(ctx, t, n)
}

// beginNode records the start of a node execution.
func (r *run) beginNode(t *token, n *planNode, scope ScopePath) (*nodeExec, error) {
	id := nodeExecutionID(r.req.Execution, t.id, n.node.ID, scope)
	if _, dup := r.nodeExecs[id]; dup {
		return nil, internalf("node execution %s of node %q exists already", id, n.node.ID)
	}
	ne := &nodeExec{id: id, token: t.id, node: t.node, scope: scope, state: NodeRunning}
	r.nodeExecs[id] = ne
	r.execOrder = append(r.execOrder, ne)
	r.event(Event{Type: EventNodeStarted, Token: t.id, NodeExecution: id, Node: n.node.ID, Scope: scope})
	return ne, nil
}

func (r *run) finishNode(ne *nodeExec, to NodeExecutionState, token TokenID, code string) error {
	next, err := ne.state.Transition(to)
	if err != nil {
		return err
	}
	ne.state = next
	typ := EventNodeSucceeded
	if to == NodeFailed {
		typ = EventNodeFailed
	}
	r.event(Event{Type: typ, Token: token, NodeExecution: ne.id, Node: r.nodeID(ne.node), Scope: ne.scope, Detail: code})
	return nil
}

// stepNode runs a node that is not a loop control: an event, an activity, a
// gateway or an item-routing node.
func (r *run) stepNode(ctx context.Context, t *token, n *planNode) (*token, error) {
	if t.scope.Depth() != n.nesting {
		return nil, internalf("token %s reached node %q in scope %q, which has %d frames; the node lies in %d loops",
			t.id, n.node.ID, t.scope, t.scope.Depth(), n.nesting)
	}
	arrival := nodeKey{t.node, t.scope}
	ne, err := r.beginNode(t, n, t.scope)
	if err != nil {
		return nil, err
	}
	ems, err := r.activateNode(ctx, t, n, ne)
	if err != nil {
		return nil, r.activationError(t, ne, n, err)
	}
	if err := r.finishNode(ne, NodeSucceeded, t.id, ""); err != nil {
		return nil, err
	}
	if n.kind == kindEnd {
		return nil, r.reachEnd(ctx, t, n, arrival)
	}
	return r.route(t, n, ne.id, t.scope, arrival, true, ems)
}

// activationError turns the failure of an activation into a failed execution,
// and lets every other error abort the run.
func (r *run) activationError(t *token, ne *nodeExec, n *planNode, err error) error {
	var fe *failureError
	if !errors.As(err, &fe) {
		return err
	}
	f := fe.failure
	if f.Node == "" {
		f.Node = n.node.ID
		f.Scope = ne.scope
	}
	return r.failNode(f, t, ne)
}

// activate computes what a node sends along its outgoing edges.
func (r *run) activate(ctx context.Context, t *token, n *planNode, ne *nodeExec) ([]emission, error) {
	input, err := r.load(ctx, t.payload)
	if err != nil {
		return nil, err
	}
	switch n.kind {
	case kindStart:
		return r.emitStart(ctx, n, ne, input)
	case kindEnd:
		return nil, nil
	case kindActivity:
		return r.emitActivity(ctx, t, n, ne, input)
	case kindParallel:
		return r.emitEdges(n.out, t.payload), nil
	case kindExclusive, kindInclusive:
		return r.emitGateway(ctx, n, t, ne, input)
	case kindRouting:
		return r.emitRouting(ctx, t, n, ne, input)
	case kindLoop:
		return nil, internalf("loop control %q reached the generic activation", n.node.ID)
	default:
		return nil, internalf("node %q has no kind", n.node.ID)
	}
}

// emitEdges sends the same payload along each of the given edges.
func (r *run) emitEdges(edges []int, payload PayloadRef) []emission {
	ems := make([]emission, 0, len(edges))
	for _, ei := range edges {
		ems = append(ems, emission{edge: ei, payload: payload})
	}
	return ems
}

// emitStart passes the execution input on the main port.
func (r *run) emitStart(ctx context.Context, n *planNode, ne *nodeExec, input Batch) ([]emission, error) {
	return r.emitPorts(ctx, n, ne, []PortBatch{{Port: flow.PortMain, Batch: input}})
}

// emitPorts stores the batches a node produced and sends each along the edges
// of its port. A port with an empty batch emitted nothing.
func (r *run) emitPorts(ctx context.Context, n *planNode, ne *nodeExec, ports []PortBatch) ([]emission, error) {
	refs := make(map[string]PayloadRef, len(ports))
	for _, pb := range ports {
		if _, dup := refs[pb.Port]; dup {
			return nil, fmt.Errorf("%w: node %q emitted port %q twice", ErrPort, n.node.ID, pb.Port)
		}
		ref := outputRef(ne.id, pb.Port)
		if err := r.put(ctx, PayloadWrite{Ref: ref, Kind: PayloadOutput, NodeExecution: ne.id, Port: pb.Port, Batch: pb.Batch}); err != nil {
			return nil, fmt.Errorf("store output of node %q port %q: %w", n.node.ID, pb.Port, err)
		}
		if pb.Batch.Len() > 0 {
			refs[pb.Port] = ref
		}
	}
	var ems []emission
	for _, ei := range n.out {
		if ref, ok := refs[r.plan.edges[ei].edge.From.Port]; ok {
			ems = append(ems, emission{edge: ei, payload: ref})
		}
	}
	return ems, nil
}

// emitActivity runs the node through the NodeRunner port.
func (r *run) emitActivity(ctx context.Context, t *token, n *planNode, ne *nodeExec, input Batch) ([]emission, error) {
	call := NodeCall{
		Org:           r.req.Org,
		Execution:     r.req.Execution,
		Token:         t.id,
		NodeExecution: ne.id,
		Node:          n.node,
		Scope:         t.scope,
		Input:         input,
		Env:           r.envOf(ne.id),
	}
	out, err := guard(func() (NodeOutput, error) { return r.in.deps.Runner.Execute(ctx, call) })
	if err != nil {
		return nil, r.portFailure(ctx, "", n, "node failed", err)
	}
	return r.emitPorts(ctx, n, ne, out.Ports)
}

// emitGateway decides the outgoing edges of an exclusive or inclusive gateway
// (ADR-0010 section 4): conditions are evaluated in declared order on the
// whole batch. An exclusive gateway takes the first true edge, an inclusive
// one every true edge; none true takes the default edge, or raises NoPathTaken.
// An edge without a condition is always true, and the default edge is not
// evaluated, whatever condition it carries.
func (r *run) emitGateway(ctx context.Context, n *planNode, t *token, ne *nodeExec, input Batch) ([]emission, error) {
	var chosen []int
	for _, ei := range n.out {
		if ei == n.defaultEdge {
			continue
		}
		e := &r.plan.edges[ei]
		ok := true
		if cond := e.edge.Condition; cond != "" {
			env := EvalEnv{Node: n.node.ID, Edge: e.edge.ID, Scope: t.scope, Items: input, Env: r.envOf(ne.id)}
			verdict, err := guard(func() (bool, error) { return r.in.deps.Conditions.EvalCondition(ctx, cond, env) })
			if err != nil {
				return nil, r.portFailure(ctx, CodeExpressionError, n, fmt.Sprintf("condition of edge %q failed", e.edge.ID), err)
			}
			ok = verdict
		}
		if !ok {
			continue
		}
		chosen = append(chosen, ei)
		if n.kind == kindExclusive {
			break
		}
	}
	if len(chosen) == 0 {
		if n.defaultEdge < 0 {
			return nil, raise(&Failure{Code: CodeNoPathTaken, Origin: FailureEngine, Message: "no outgoing edge condition is true and the gateway has no default edge"})
		}
		chosen = append(chosen, n.defaultEdge)
	}
	return r.emitEdges(chosen, t.payload), nil
}

// portFailure classifies the error of a port call. A context error aborts the
// run; anything else fails the execution under the given code, or Panic.
func (r *run) portFailure(ctx context.Context, code string, n *planNode, what string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("execution %s: %w", r.req.Execution, ctxErr)
	}
	f := &Failure{Code: code, Origin: FailureEngine, Node: n.node.ID, Message: fmt.Sprintf("%s: %v", what, err), Cause: err}
	var pe *PanicError
	var ne *NodeError
	switch {
	case errors.As(err, &pe):
		f.Code = CodePanic
	case code == "" && errors.As(err, &ne):
		f.Code, f.Class, f.Origin = ne.Code, ne.Class, FailureNode
	case code == "":
		f.Code, f.Internal, f.Origin = codeNodeError, true, FailureNode
	}
	return raise(f)
}

// emitRouting routes the items of an orcon.if or orcon.switch node (ADR-0010
// section 4). Each item goes to the first port it matches (EXCLUSIVE) or to
// every port it matches (INCLUSIVE); an item that matched no port goes to the
// port of the default edge, or raises NoPathTaken. Each port that receives
// items emits one live token carrying its subset, in item order. A port with
// no edge drops the items it receives: the node matched them, they lead
// nowhere.
func (r *run) emitRouting(ctx context.Context, t *token, n *planNode, ne *nodeExec, input Batch) ([]emission, error) {
	call := RouteCall{Execution: r.req.Execution, NodeExecution: ne.id, Node: n.node, Scope: t.scope, Items: input, Env: r.envOf(ne.id)}
	matches, err := guard(func() ([][]string, error) { return r.in.deps.Router.RouteItems(ctx, call) })
	if err != nil {
		return nil, r.portFailure(ctx, "", n, "routing failed", err)
	}
	if len(matches) != input.Len() {
		return nil, fmt.Errorf("%w: item router answered for %d items of node %q, which received %d",
			ErrPort, len(matches), n.node.ID, input.Len())
	}
	defaultPort := ""
	if n.defaultEdge >= 0 {
		defaultPort = r.plan.edges[n.defaultEdge].edge.From.Port
	}
	var order []string
	positions := make(map[string][]int)
	for i, ports := range matches {
		chosen, err := r.chosenPorts(n, ports)
		if err != nil {
			return nil, err
		}
		if len(chosen) == 0 {
			if n.defaultEdge < 0 {
				return nil, raise(&Failure{Code: CodeNoPathTaken, Origin: FailureEngine, Message: fmt.Sprintf("item %d matched no port and the node has no default edge", i)})
			}
			chosen = []string{defaultPort}
		}
		for _, p := range chosen {
			if _, seen := positions[p]; !seen {
				order = append(order, p)
			}
			positions[p] = append(positions[p], i)
		}
	}
	ports := make([]PortBatch, 0, len(order))
	for _, p := range order {
		ports = append(ports, PortBatch{Port: p, Batch: input.pick(positions[p])})
	}
	return r.emitPorts(ctx, n, ne, ports)
}

// chosenPorts checks the ports an item matched against the routing mode. An
// EXCLUSIVE node must answer with at most one port; more is a violation of the
// ItemRouter contract, a programming error and not a failure of the execution.
func (r *run) chosenPorts(n *planNode, matched []string) ([]string, error) {
	var chosen []string
	for _, p := range matched {
		if !slices.Contains(chosen, p) {
			chosen = append(chosen, p)
		}
	}
	if n.routing == flow.RoutingExclusive && len(chosen) > 1 {
		return nil, fmt.Errorf("%w: the router answered %d ports for one item of EXCLUSIVE node %q", ErrPort, len(chosen), n.node.ID)
	}
	return chosen, nil
}

// reachEnd completes the token at an end event.
func (r *run) reachEnd(ctx context.Context, t *token, n *planNode, arrival nodeKey) error {
	batch, err := r.load(ctx, t.payload)
	if err != nil {
		return err
	}
	r.ends = append(r.ends, EndArrival{Node: n.node.ID, Scope: t.scope, Token: t.id, Batch: batch})
	if err := r.setToken(t, TokenCompleted); err != nil {
		return err
	}
	r.nodeOf(arrival).inflight--
	r.checkComplete(arrival)
	return nil
}

// failNode ends the execution in FAILED because of a failure at a node.
func (r *run) failNode(f *Failure, t *token, ne *nodeExec) error {
	if ne != nil {
		if err := r.finishNode(ne, NodeFailed, t.id, ""); err != nil {
			return err
		}
	}
	return r.fail(f, t)
}

// fail ends the execution in FAILED. Under STOP_EXECUTION an unhandled error
// stops the execution: the failing token fails, every other live token is
// canceled (ADR-0009 section 1). Error routing arrives with cut C7.
func (r *run) fail(f *Failure, failing *token) error {
	if r.failure != nil {
		return nil
	}
	r.failure = f
	if failing != nil {
		if err := r.setToken(failing, TokenFailed); err != nil {
			return err
		}
	}
	for _, t := range r.order {
		if t.state.isActive() {
			if err := r.setToken(t, TokenCanceled); err != nil {
				return err
			}
		}
	}
	if err := r.setExec(ExecFailed); err != nil {
		return err
	}
	detail := f.Code
	if f.Internal {
		detail = "" // internal codes are never emitted
	}
	r.event(Event{Type: EventExecutionFailed, Node: f.Node, Scope: f.Scope, Detail: detail})
	return nil
}
