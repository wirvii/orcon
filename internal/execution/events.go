// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
)

// EventType is a type of the event catalog of ADR-0009 section 8. Event types
// are identifiers and keep their dotted lowercase form.
type EventType string

// The events the in-memory interpreter produces. The names are those of the
// catalog; the interpreter adds none.
const (
	// EventExecutionStarted marks the move from QUEUED to RUNNING.
	EventExecutionStarted EventType = "execution.started"
	// EventExecutionSucceeded marks an execution that ended with no live token.
	EventExecutionSucceeded EventType = "execution.succeeded"
	// EventExecutionFailed marks an execution that ended with an error.
	EventExecutionFailed EventType = "execution.failed"
	// EventTokenForked marks a token that handed out several child tokens.
	EventTokenForked EventType = "token.forked"
	// EventTokenDeferred marks a child token held back by sequential branch mode.
	EventTokenDeferred EventType = "token.deferred"
	// EventTokenConsumed marks a token absorbed by a join, a fork or a dead end.
	EventTokenConsumed EventType = "token.consumed"
	// EventEdgeClosed marks the completion signal of an edge in a scope.
	EventEdgeClosed EventType = "edge.closed"
	// EventJoinFired marks a join that created its continuation token.
	EventJoinFired EventType = "join.fired"
	// EventJoinFailed marks a join that failed with JoinRequirementNotMet.
	EventJoinFailed EventType = "join.failed"
	// EventLoopIterationStarted marks the start of a loop iteration.
	EventLoopIterationStarted EventType = "loop.iteration_started"
	// EventLoopLimitExceeded marks a loop that would exceed maxIterations.
	EventLoopLimitExceeded EventType = "loop.limit_exceeded"
	// EventNodeStarted marks the start of a node execution.
	EventNodeStarted EventType = "node.started"
	// EventNodeSucceeded marks a node execution that finished.
	EventNodeSucceeded EventType = "node.succeeded"
	// EventNodeFailed marks a node execution that failed.
	EventNodeFailed EventType = "node.failed"
)

// Event is one entry of the execution history, shaped after the envelope of
// ADR-0009 section 7. Fields that do not apply to a type stay empty.
type Event struct {
	// Sequence numbers the events of one run from 1, in emission order. A
	// durable store replaces it with the database sequence number.
	Sequence      int
	Type          EventType
	Execution     ID
	Token         TokenID
	NodeExecution NodeExecutionID
	// Node is the node the event is about. For edge.closed it is the node the
	// edge leads to, whose input the signal completes.
	Node  flow.NodeID
	Edge  flow.EdgeID
	Scope ScopePath
	// Live is set on edge.closed when at least one live token traveled the edge
	// in the scope.
	Live bool
	// Iteration is the loop iteration of loop.* events.
	Iteration int
	// Tokens lists the children of token.forked, or the tokens a discriminator
	// canceled in join.fired.
	Tokens []TokenID
	// Detail is a short UPPER_SNAKE qualifier: the join mode of join.fired, the
	// reason of token.consumed, the failure code of *.failed.
	Detail string
}

// String renders the event on one line, with the unset fields left out. The
// form is stable, so tests can compare whole traces.
func (e Event) String() string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(e.Sequence))
	b.WriteByte(' ')
	b.WriteString(string(e.Type))
	if e.Node != "" {
		b.WriteString(" node=")
		b.WriteString(string(e.Node))
	}
	if e.Edge != "" {
		b.WriteString(" edge=")
		b.WriteString(string(e.Edge))
	}
	if e.Scope != "" {
		b.WriteString(" scope=")
		b.WriteString(string(e.Scope))
	}
	if e.Type == EventEdgeClosed {
		b.WriteString(" live=")
		b.WriteString(strconv.FormatBool(e.Live))
	}
	if e.Type == EventLoopIterationStarted || e.Type == EventLoopLimitExceeded {
		b.WriteString(" iteration=")
		b.WriteString(strconv.Itoa(e.Iteration))
	}
	if e.Detail != "" {
		b.WriteString(" detail=")
		b.WriteString(e.Detail)
	}
	if len(e.Tokens) > 0 {
		b.WriteString(" tokens=")
		b.WriteString(strconv.Itoa(len(e.Tokens)))
	}
	return b.String()
}
