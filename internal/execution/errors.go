// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/flow"
)

// Sentinel errors of the area. Callers match them with [errors.Is].
var (
	// ErrInvalidTransition marks a state change that is not in the closed
	// transition table of its state machine (R-SM-01). It signals a bug.
	ErrInvalidTransition = errors.New("invalid state transition")
	// ErrInvalidFlow marks a flow document that fails static validation.
	ErrInvalidFlow = errors.New("invalid flow")
	// ErrUnsupported marks a flow that uses a feature a later implementation
	// cut provides. The error names the feature and the cut.
	ErrUnsupported = errors.New("flow uses a feature the interpreter does not support yet")
	// ErrMissingPort marks a flow that needs a port the interpreter was built
	// without, for example conditions without a condition evaluator.
	ErrMissingPort = errors.New("interpreter port is not configured")
	// ErrInvalidRequest marks a request that cannot start an execution.
	ErrInvalidRequest = errors.New("invalid execution request")
	// ErrPayloadNotFound marks a payload reference the store does not hold.
	ErrPayloadNotFound = errors.New("payload not found")
	// ErrPort marks a port implementation that broke its contract, for
	// example an item router that answered for the wrong number of items.
	ErrPort = errors.New("port contract violated")
)

// Failure codes. Engine-raised codes are identifiers, not enumerated values,
// so they keep their CamelCase form (ADR-0009 section 9, ADR-0011 section 1).
// The list is closed: another code needs an amendment of ADR-0011 section 1.
const (
	// CodeNoPathTaken is raised by an exclusive or inclusive split, or an item
	// that matched no port, when nothing is chosen and there is no default.
	CodeNoPathTaken = "NoPathTaken"
	// CodeJoinRequirementNotMet is raised by an ALL join when an incoming edge
	// completes with no live token.
	CodeJoinRequirementNotMet = "JoinRequirementNotMet"
	// CodeLoopLimitExceeded is raised when a loop would start an iteration
	// beyond its maxIterations.
	CodeLoopLimitExceeded = "LoopLimitExceeded"
	// CodeExpressionError is raised when evaluating a condition fails.
	CodeExpressionError = "ExpressionError"
	// CodePanic is raised when a port implementation panics; the interpreter
	// recovers at the port boundary (go-style.md GS-E6).
	CodePanic = "Panic"
)

// The code NodeError is the only code of this interpreter outside the closed
// list above: it marks the failure of a node whose runner returned an error
// that carries no code of its own. A failure carrying it reports Internal, and
// it is never emitted in an event nor persisted.
const codeNodeError = "NodeError"

// ErrorClass is the class a node type gives its errors (ADR-0011 section 1).
type ErrorClass string

// Error classes.
const (
	// ClassTransient marks an error worth retrying.
	ClassTransient ErrorClass = "TRANSIENT"
	// ClassPermanent marks an error retrying does not fix.
	ClassPermanent ErrorClass = "PERMANENT"
	// ClassUncertain marks a write whose outcome is unknown.
	ClassUncertain ErrorClass = "UNCERTAIN"
)

// NodeError is the error a [NodeRunner] returns to give a failure the code and
// class its node type's classifier assigned (ADR-0011 section 1). Any other
// error a runner returns fails the node as an internal failure.
type NodeError struct {
	// Code identifies the error, for example a connector error code.
	Code  string
	Class ErrorClass
	Err   error
}

// Error implements error.
func (e *NodeError) Error() string { return fmt.Sprintf("%s: %v", e.Code, e.Err) }

// Unwrap returns the underlying error.
func (e *NodeError) Unwrap() error { return e.Err }

// FailureOrigin tells whether the engine or a node raised a failure.
type FailureOrigin string

// Failure origins.
const (
	// FailureEngine marks a failure raised by the engine itself.
	FailureEngine FailureOrigin = "ENGINE"
	// FailureNode marks a failure returned by a node.
	FailureNode FailureOrigin = "NODE"
)

// UnsupportedIssue is one feature of a flow the interpreter cannot run yet.
type UnsupportedIssue struct {
	// Node is the node that uses the feature; empty for a flow-level setting.
	Node flow.NodeID
	// Edge is the edge that uses the feature, when it is an edge.
	Edge flow.EdgeID
	// Feature says what is used, for example "multiInstance".
	Feature string
	// Cut names the implementation cut that provides it, for example "C9".
	Cut string
}

// String renders the issue on one line.
func (i UnsupportedIssue) String() string {
	where := "flow"
	switch {
	case i.Node != "":
		where = fmt.Sprintf("node %q", i.Node)
	case i.Edge != "":
		where = fmt.Sprintf("edge %q", i.Edge)
	}
	return fmt.Sprintf("%s uses %s (cut %s)", where, i.Feature, i.Cut)
}

// UnsupportedError lists every unsupported feature [Compile] found. It matches
// [ErrUnsupported] with [errors.Is].
type UnsupportedError struct {
	Issues []UnsupportedIssue
}

// Error implements error.
func (e *UnsupportedError) Error() string {
	parts := make([]string, len(e.Issues))
	for i, issue := range e.Issues {
		parts[i] = issue.String()
	}
	return "execution: " + ErrUnsupported.Error() + ": " + strings.Join(parts, "; ")
}

// Is makes the error match [ErrUnsupported].
func (e *UnsupportedError) Is(target error) bool { return target == ErrUnsupported }

// PanicError wraps a value recovered from a port implementation.
type PanicError struct {
	// Value is what the port panicked with.
	Value any
	// Stack is the goroutine stack at the panic.
	Stack []byte
}

// Error implements error.
func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v", e.Value) }

// Failure describes why an execution ended in FAILED.
type Failure struct {
	// Code is one of the Code constants for an engine failure, or the code the
	// node's runner gave for a node failure.
	Code string
	// Origin says whether the engine or a node raised the failure.
	Origin FailureOrigin
	// Class is the error class of a node failure, when the runner gave one.
	Class ErrorClass
	// Internal marks a failure with a code outside the closed list of
	// ADR-0011 section 1. It is never emitted in an event nor persisted.
	Internal bool
	// Node is the node whose execution failed, when there is one.
	Node flow.NodeID
	// NodeExecution is the node execution that failed, when there is one.
	NodeExecution NodeExecutionID
	// Scope is the scope the failure happened in.
	Scope ScopePath
	// Message is a human-readable explanation.
	Message string
	// Cause is the underlying error of a port, if any; a recovered panic is a
	// [*PanicError].
	Cause error
}

// String renders the failure on one line.
func (f *Failure) String() string {
	if f.Node == "" {
		return fmt.Sprintf("%s: %s", f.Code, f.Message)
	}
	return fmt.Sprintf("%s at node %q: %s", f.Code, f.Node, f.Message)
}

// failureError carries a [Failure] up from the code that detects it to the
// step that fails the execution.
type failureError struct {
	failure *Failure
}

func (e *failureError) Error() string { return e.failure.String() }

func (e *failureError) Unwrap() error { return e.failure.Cause }

// raise returns the failure as an error.
func raise(f *Failure) error { return &failureError{failure: f} }
