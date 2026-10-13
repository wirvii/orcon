// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"fmt"
	"slices"
)

// The state machines of ADR-0009 are closed transition tables: a transition
// that is not in its table is a programming error, reported as an error
// wrapping [ErrInvalidTransition] and caught by tests (R-SM-01). Every state
// value is UPPER_SNAKE (ADR-0009 section 9).

// transitions maps each state to the states it may move to.
type transitions[S ~string] map[S][]S

func (t transitions[S]) allows(from, to S) bool {
	return slices.Contains(t[from], to)
}

// states returns every state of the table, sorted, including states that
// only appear as a target.
func (t transitions[S]) states() []S {
	seen := make(map[S]struct{}, len(t))
	for from, tos := range t {
		seen[from] = struct{}{}
		for _, to := range tos {
			seen[to] = struct{}{}
		}
	}
	out := make([]S, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	slices.Sort(out)
	return out
}

func (t transitions[S]) check(machine string, from, to S) error {
	if t.allows(from, to) {
		return nil
	}
	return &TransitionError{Machine: machine, From: string(from), To: string(to)}
}

// State is the state of a workflow execution (ADR-0009 section 1).
type State string

// Execution states.
const (
	// ExecQueued is an execution created and not yet claimed by a worker.
	ExecQueued State = "QUEUED"
	// ExecRunning is an execution with at least one token being worked.
	ExecRunning State = "RUNNING"
	// ExecWaiting is an execution whose live tokens are all waiting.
	ExecWaiting State = "WAITING"
	// ExecSuspended is an execution parked for a supervised resolution.
	ExecSuspended State = "SUSPENDED"
	// ExecCanceling is an execution draining after a cancel request.
	ExecCanceling State = "CANCELING"
	// ExecCompensating is an execution running its compensations.
	ExecCompensating State = "COMPENSATING"
	// ExecSucceeded is an execution that ended with no live tokens left.
	ExecSucceeded State = "SUCCEEDED"
	// ExecFailed is an execution that ended with an unhandled error.
	ExecFailed State = "FAILED"
	// ExecCanceled is an execution that was canceled.
	ExecCanceled State = "CANCELED"
)

var executionTable = transitions[State]{
	ExecQueued:       {ExecRunning, ExecCanceling},
	ExecRunning:      {ExecWaiting, ExecSuspended, ExecSucceeded, ExecFailed, ExecCanceling},
	ExecWaiting:      {ExecRunning, ExecSuspended, ExecFailed, ExecCanceling},
	ExecSuspended:    {ExecRunning, ExecCanceling},
	ExecFailed:       {ExecRunning},
	ExecCanceling:    {ExecCanceled, ExecCompensating},
	ExecCompensating: {ExecCanceled, ExecSuspended},
}

// States returns every execution state, sorted.
func States() []State { return executionTable.states() }

// CanTransitionTo reports whether the table allows moving from s to to.
func (s State) CanTransitionTo(to State) bool { return executionTable.allows(s, to) }

// Transition returns to when the table allows the move, and an error wrapping
// [ErrInvalidTransition] otherwise.
func (s State) Transition(to State) (State, error) {
	if err := executionTable.check("execution", s, to); err != nil {
		return s, err
	}
	return to, nil
}

// IsInitial reports whether an execution may be created in this state.
func (s State) IsInitial() bool { return s == ExecQueued }

// TokenState is the state of a token (ADR-0009 section 2).
type TokenState string

// Token states.
const (
	// TokenRunnable is a token a worker may claim.
	TokenRunnable TokenState = "RUNNABLE"
	// TokenLeased is a token a worker holds and is working on.
	TokenLeased TokenState = "LEASED"
	// TokenWaiting is a token parked on a durable wait.
	TokenWaiting TokenState = "WAITING"
	// TokenDeferred is a token held back by sequential branch mode.
	TokenDeferred TokenState = "DEFERRED"
	// TokenSuspended is a token parked for a supervised resolution.
	TokenSuspended TokenState = "SUSPENDED"
	// TokenConsumed is a token absorbed by a join, a merge or a fork.
	TokenConsumed TokenState = "CONSUMED"
	// TokenCompleted is a token that reached an end event.
	TokenCompleted TokenState = "COMPLETED"
	// TokenCanceled is a token canceled by a cancellation, a discriminator or
	// a failing execution.
	TokenCanceled TokenState = "CANCELED"
	// TokenFailed is a token whose node failed.
	TokenFailed TokenState = "FAILED"
)

// The table is ADR-0009 section 2 line by line, with one reading: a token
// that forks becomes CONSUMED, because its identity continues in the child
// tokens it hands out (R-REC-01); the ADR lists CONSUMED for "absorbed by a
// join or merge tracker" and a fork absorbs its parent the same way.
var tokenTable = transitions[TokenState]{
	TokenRunnable:  {TokenLeased, TokenCanceled},
	TokenLeased:    {TokenRunnable, TokenWaiting, TokenConsumed, TokenCompleted, TokenFailed, TokenSuspended, TokenCanceled},
	TokenWaiting:   {TokenRunnable, TokenCanceled},
	TokenDeferred:  {TokenRunnable, TokenCanceled},
	TokenSuspended: {TokenRunnable, TokenCanceled},
}

// TokenStates returns every token state, sorted.
func TokenStates() []TokenState { return tokenTable.states() }

// CanTransitionTo reports whether the table allows moving from s to to.
func (s TokenState) CanTransitionTo(to TokenState) bool { return tokenTable.allows(s, to) }

// Transition returns to when the table allows the move, and an error wrapping
// [ErrInvalidTransition] otherwise.
func (s TokenState) Transition(to TokenState) (TokenState, error) {
	if err := tokenTable.check("token", s, to); err != nil {
		return s, err
	}
	return to, nil
}

// IsInitial reports whether a token may be created in this state: runnable, or
// deferred by sequential branch mode, or already leased by the worker that
// forked it and keeps one branch (R-REC-01).
func (s TokenState) IsInitial() bool {
	return s == TokenRunnable || s == TokenDeferred || s == TokenLeased
}

// IsTerminal reports whether no transition leaves the state.
func (s TokenState) IsTerminal() bool { return len(tokenTable[s]) == 0 }

// isActive reports whether a token in this state still has work to do or is
// held for a resolution: it counts as live for loop scopes and for sequential
// branch release (ADR-0010 section 5 releases the next sibling when every
// descendant is consumed, completed, waiting or terminal).
func (s TokenState) isActive() bool {
	switch s {
	case TokenRunnable, TokenLeased, TokenDeferred, TokenSuspended:
		return true
	case TokenWaiting, TokenConsumed, TokenCompleted, TokenCanceled, TokenFailed:
		return false
	default:
		return false
	}
}

// NodeExecutionState is the state of a node execution (ADR-0009 section 3).
type NodeExecutionState string

// Node execution states.
const (
	// NodeRunning is a node execution in progress.
	NodeRunning NodeExecutionState = "RUNNING"
	// NodeWaiting is a node execution parked on a wait.
	NodeWaiting NodeExecutionState = "WAITING"
	// NodeSucceeded is a node execution that finished without errors.
	NodeSucceeded NodeExecutionState = "SUCCEEDED"
	// NodeSucceededWithErrors is a node execution where some items were routed
	// through error handling.
	NodeSucceededWithErrors NodeExecutionState = "SUCCEEDED_WITH_ERRORS"
	// NodeFailed is a node execution that failed.
	NodeFailed NodeExecutionState = "FAILED"
	// NodeSuspended is a node execution parked for a supervised resolution.
	NodeSuspended NodeExecutionState = "SUSPENDED"
	// NodeCanceled is a node execution that was canceled.
	NodeCanceled NodeExecutionState = "CANCELED"
)

// ADR-0009 section 3 lists the states but not the moves between them. The
// table is derived: a running node ends in any final state, may park on a wait
// or for a resolution, and returns to RUNNING from both (the RUNNING and
// WAITING cycle of stepped nodes is the SPEC-006 amendment); a parked node
// can be canceled. The cuts that implement waits (C8), errors (C7) and
// suspension may extend the table by an amendment of ADR-0009.
var nodeExecutionTable = transitions[NodeExecutionState]{
	NodeRunning:   {NodeWaiting, NodeSucceeded, NodeSucceededWithErrors, NodeFailed, NodeSuspended, NodeCanceled},
	NodeWaiting:   {NodeRunning, NodeCanceled},
	NodeSuspended: {NodeRunning, NodeCanceled},
}

// NodeExecutionStates returns every node execution state, sorted.
func NodeExecutionStates() []NodeExecutionState { return nodeExecutionTable.states() }

// CanTransitionTo reports whether the table allows moving from s to to.
func (s NodeExecutionState) CanTransitionTo(to NodeExecutionState) bool {
	return nodeExecutionTable.allows(s, to)
}

// Transition returns to when the table allows the move, and an error wrapping
// [ErrInvalidTransition] otherwise.
func (s NodeExecutionState) Transition(to NodeExecutionState) (NodeExecutionState, error) {
	if err := nodeExecutionTable.check("node execution", s, to); err != nil {
		return s, err
	}
	return to, nil
}

// IsInitial reports whether a node execution may be created in this state.
func (s NodeExecutionState) IsInitial() bool { return s == NodeRunning }

// AttemptState is the state of a task attempt (ADR-0009 section 4).
type AttemptState string

// Task attempt states.
const (
	// AttemptStarted is an attempt in progress.
	AttemptStarted AttemptState = "STARTED"
	// AttemptSucceeded is an attempt that finished.
	AttemptSucceeded AttemptState = "SUCCEEDED"
	// AttemptFailed is an attempt that failed with an error class.
	AttemptFailed AttemptState = "FAILED"
	// AttemptTimedOut is an attempt that exceeded its time limit.
	AttemptTimedOut AttemptState = "TIMED_OUT"
	// AttemptAbandoned is an attempt whose lease epoch recovery superseded.
	AttemptAbandoned AttemptState = "ABANDONED"
	// AttemptCanceled is an attempt that was canceled.
	AttemptCanceled AttemptState = "CANCELED"
)

var attemptTable = transitions[AttemptState]{
	AttemptStarted: {AttemptSucceeded, AttemptFailed, AttemptTimedOut, AttemptAbandoned, AttemptCanceled},
}

// AttemptStates returns every task attempt state, sorted.
func AttemptStates() []AttemptState { return attemptTable.states() }

// CanTransitionTo reports whether the table allows moving from s to to.
func (s AttemptState) CanTransitionTo(to AttemptState) bool { return attemptTable.allows(s, to) }

// Transition returns to when the table allows the move, and an error wrapping
// [ErrInvalidTransition] otherwise.
func (s AttemptState) Transition(to AttemptState) (AttemptState, error) {
	if err := attemptTable.check("task attempt", s, to); err != nil {
		return s, err
	}
	return to, nil
}

// IsInitial reports whether an attempt may be created in this state.
func (s AttemptState) IsInitial() bool { return s == AttemptStarted }

// TimerState is the state of a timer (ADR-0009 section 6).
type TimerState string

// Timer states.
const (
	// TimerScheduled is a timer waiting for its due time.
	TimerScheduled TimerState = "SCHEDULED"
	// TimerFired is a timer that fired.
	TimerFired TimerState = "FIRED"
	// TimerCanceled is a timer that was canceled before firing.
	TimerCanceled TimerState = "CANCELED"
)

var timerTable = transitions[TimerState]{
	TimerScheduled: {TimerFired, TimerCanceled},
}

// TimerStates returns every timer state, sorted.
func TimerStates() []TimerState { return timerTable.states() }

// CanTransitionTo reports whether the table allows moving from s to to.
func (s TimerState) CanTransitionTo(to TimerState) bool { return timerTable.allows(s, to) }

// Transition returns to when the table allows the move, and an error wrapping
// [ErrInvalidTransition] otherwise.
func (s TimerState) Transition(to TimerState) (TimerState, error) {
	if err := timerTable.check("timer", s, to); err != nil {
		return s, err
	}
	return to, nil
}

// IsInitial reports whether a timer may be created in this state.
func (s TimerState) IsInitial() bool { return s == TimerScheduled }

// WaitState is the state of a wait (ADR-0009 section 6).
type WaitState string

// Wait states.
const (
	// WaitOpen is a wait still waiting for its resolution.
	WaitOpen WaitState = "OPEN"
	// WaitResolved is a wait resolved by a signal, a person, a timer or a child.
	WaitResolved WaitState = "RESOLVED"
	// WaitExpired is a wait whose timeout path was taken.
	WaitExpired WaitState = "EXPIRED"
	// WaitCanceled is a wait that was canceled.
	WaitCanceled WaitState = "CANCELED"
)

var waitTable = transitions[WaitState]{
	WaitOpen: {WaitResolved, WaitExpired, WaitCanceled},
}

// WaitStates returns every wait state, sorted.
func WaitStates() []WaitState { return waitTable.states() }

// CanTransitionTo reports whether the table allows moving from s to to.
func (s WaitState) CanTransitionTo(to WaitState) bool { return waitTable.allows(s, to) }

// Transition returns to when the table allows the move, and an error wrapping
// [ErrInvalidTransition] otherwise.
func (s WaitState) Transition(to WaitState) (WaitState, error) {
	if err := waitTable.check("wait", s, to); err != nil {
		return s, err
	}
	return to, nil
}

// IsInitial reports whether a wait may be created in this state.
func (s WaitState) IsInitial() bool { return s == WaitOpen }

// TransitionError reports a state change the closed table of a state machine
// does not contain. It matches [ErrInvalidTransition] with [errors.Is].
type TransitionError struct {
	// Machine names the state machine, for example "token".
	Machine string
	From    string
	To      string
}

// Error implements error.
func (e *TransitionError) Error() string {
	return fmt.Sprintf("%s: transition %s -> %s is not in the state table", e.Machine, e.From, e.To)
}

// Is makes the error match [ErrInvalidTransition].
func (e *TransitionError) Is(target error) bool { return target == ErrInvalidTransition }
