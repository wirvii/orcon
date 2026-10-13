// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"context"

	"github.com/wirvii/orcon/internal/flow"
)

// The interfaces below are the ports the interpreter consumes (go-style.md
// AL-3). Each is small and declared here, next to its caller. The cuts that
// follow provide the real implementations:
//
//   - [ConditionEvaluator] and [ItemRouter]: the expression cut (C5) on top of
//     the sandbox, and the node types the format itself defines.
//   - [NodeRunner]: connectors and the code node; the journal cut (C6) wraps it
//     for effect nodes, keyed by [NodeCall.NodeExecution].
//   - [Payloads] and [EventSink]: the storage cut (C2) and the durability cut
//     (C4), which write them in the transaction that commits a step.

// EvalEnv is the data a condition is evaluated against.
type EvalEnv struct {
	// Node is the gateway whose outgoing edge carries the condition.
	Node flow.NodeID
	// Edge is the edge the condition belongs to.
	Edge flow.EdgeID
	// Scope is the scope path of the token being routed.
	Scope ScopePath
	// Items is the batch the token carries. Conditions of exclusive and
	// inclusive gateways are token-level: they see the whole batch.
	Items Batch
}

// ConditionEvaluator decides edge conditions (ADR-0010 section 4). The
// expression language (ADR-0018) decides what counts as true; the interpreter
// only needs the verdict. A returned error fails the execution with
// [CodeExpressionError].
type ConditionEvaluator interface {
	EvalCondition(ctx context.Context, expression string, env EvalEnv) (bool, error)
}

// RouteCall is what an [ItemRouter] receives.
type RouteCall struct {
	Execution     ID
	NodeExecution NodeExecutionID
	// Node is the item-routing node (orcon.if or orcon.switch). It is shared
	// with the flow document and must not be modified.
	Node  *flow.Node
	Scope ScopePath
	Items Batch
}

// ItemRouter tells which output ports each item of a batch matches (ADR-0010
// section 4). The parameters of orcon.if and orcon.switch belong to the node
// types, so the router owns reading them; the interpreter owns the semantics
// around them: EXCLUSIVE keeps the first matched port, INCLUSIVE keeps all,
// and an item that matched no port goes to the node's default edge or fails
// with [CodeNoPathTaken].
type ItemRouter interface {
	// RouteItems returns one list of port names per item, in item order and in
	// the node's declared port order.
	RouteItems(ctx context.Context, call RouteCall) ([][]string, error)
}

// NodeCall is what a [NodeRunner] receives for one activation.
type NodeCall struct {
	Execution     ID
	Token         TokenID
	NodeExecution NodeExecutionID
	// Node is shared with the flow document and must not be modified.
	Node  *flow.Node
	Scope ScopePath
	// Input is the batch the token carries.
	Input Batch
}

// PortBatch is the batch a node emitted on one output port.
type PortBatch struct {
	Port  string
	Batch Batch
}

// NodeOutput is the result of one activation: the batches per output port. A
// port that is absent, or whose batch is empty, emitted nothing, and the edges
// on it complete with no live token.
type NodeOutput struct {
	Ports []PortBatch
}

// NodeRunner runs the activity nodes: the element classes task, serviceTask,
// scriptTask and sendTask. The interpreter itself runs the control nodes
// (events, gateways, item routing, loop control). A returned error fails the
// execution with [CodeNodeError] until the errors cut adds error routing.
type NodeRunner interface {
	Execute(ctx context.Context, call NodeCall) (NodeOutput, error)
}

// EventSink receives the execution events in order, after the step that
// produced them. A returned error aborts the run: an event that cannot be
// recorded must not be lost silently.
type EventSink interface {
	Emit(ctx context.Context, event Event) error
}
