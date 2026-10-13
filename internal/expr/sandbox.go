// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"context"
	"errors"
	"time"
)

// Sandbox is the isolation boundary for JavaScript that is not trusted
// (ADR-0018 section 3). Its implementation lives in an adapter; this package
// only describes what it needs.
type Sandbox interface {
	// Open creates a runtime, loads the scripts of cfg into its global scope in
	// order and binds cfg.Entry. The runtime is bound to one execution: it is
	// never shared across executions.
	Open(ctx context.Context, cfg SandboxConfig) (Runtime, error)
}

// SandboxConfig describes one runtime.
type SandboxConfig struct {
	// Scripts are evaluated in the global scope, in order, before the entry
	// point is looked up.
	Scripts []Script
	// Entry is the name of the global function the runtime calls for every
	// [Runtime.Call]. It takes one string and returns one string.
	Entry string
	// MemoryLimit is the hard cap on the memory of the runtime, in bytes.
	MemoryLimit uint32
	// StackLimit is the cap on the JavaScript call stack, in bytes.
	StackLimit uint32
	// Setup serves the host calls the scripts make while they load, such as the
	// zone conversions of the warm-up. It must be deterministic: the runtime
	// built from the scripts is kept as a snapshot and reused. Nil means no data.
	Setup Guest
}

// Script is JavaScript source loaded into a runtime.
type Script struct {
	// Name appears in error messages.
	Name   string
	Source string
}

// Runtime is one isolated JavaScript runtime.
//
// A runtime serves one call at a time. After a call fails with
// [ErrTimeout], [ErrMemory] or [ErrSandbox] it must not be used again: the
// caller closes it and opens a new one.
type Runtime interface {
	// Call runs the entry point with request and returns its answer. The time
	// limit of the call is the context deadline, or the duration that
	// [TimeoutFrom] reads from the context, which the runtime starts to count
	// when the call begins to run (a runtime that queues calls does not count
	// the wait). guest serves what the code asks of the host while the call runs.
	Call(ctx context.Context, request []byte, guest Guest) ([]byte, error)
	// MemoryUsage reports the bytes the JavaScript heap currently holds.
	MemoryUsage(ctx context.Context) int64
	// Close releases the runtime.
	Close(ctx context.Context) error
}

// Guest is what the code in a runtime may ask of the host. It is the only
// window to the outside: the runtime has no filesystem, network or clock of
// its own.
type Guest interface {
	// HostCall answers a data request of the code (one of the Op constants).
	// An empty answer means "no data".
	HostCall(ctx context.Context, op int32, payload []byte) ([]byte, error)
	// Now returns the recorded activation time, the value of the runtime clock.
	Now() time.Time
	// Read fills p with recorded random bytes.
	Read(p []byte)
}

// Host operations of the sandbox protocol between the prelude scripts and
// [Session]. They are part of the prelude's contract; changing a value
// requires changing prelude/runtime.js in the same change.
const (
	// OpInputAll returns the input items of the activation as a JSON array of
	// {json, binary}.
	OpInputAll int32 = 1
	// OpNodeMeta takes a node name and returns {exists, executed, params, count}.
	OpNodeMeta int32 = 2
	// OpNodeItems takes a node name and returns its output items as a JSON array.
	OpNodeItems int32 = 3
	// OpNodePaired takes {n: node name, i: item index} and returns the item of
	// that node paired with the item (lineage), or nothing.
	OpNodePaired int32 = 4
	// OpRandom returns the next random number in [0, 1) as decimal text.
	OpRandom int32 = 5
	// OpRandomBytes takes a count and returns that many random bytes as hex.
	OpRandomBytes int32 = 6
	// OpZoneFields takes "<zone>\t<epoch milliseconds>" and returns
	// "year month day hour minute second" in that zone, or nothing when the
	// zone is unknown.
	OpZoneFields int32 = 7
	// OpHash takes "<algorithm>\t<text>" and returns the hex digest of the
	// text, or nothing for an unknown algorithm.
	OpHash int32 = 8
)

// Errors of the sandbox. A failure of the code itself (a thrown error) is an
// [*EvalError]; these report that the sandbox stopped the evaluation or
// failed, and the runtime that raised them is discarded.
var (
	// ErrTimeout means the evaluation exceeded its time limit.
	ErrTimeout = errors.New("expr: evaluation exceeded its time limit")
	// ErrMemory means the evaluation exceeded its memory limit.
	ErrMemory = errors.New("expr: evaluation exceeded its memory limit")
	// ErrStack means the evaluation recursed past the stack limit.
	ErrStack = errors.New("expr: evaluation exceeded the stack limit")
	// ErrSandbox means the sandbox failed in a way the code did not cause
	// directly, for example a trap of the WebAssembly module.
	ErrSandbox = errors.New("expr: sandbox failure")
	// ErrSandboxSaturated means the process holds as many abandoned
	// evaluations still running as it allows, so it starts no new sandbox
	// call (ADR-0018 section 3, amendment 10). It is an infrastructure error,
	// not an error of the expression: it is not an [*EvalError], it does not
	// count against the node's retries, and the activation that met it must
	// not be committed, so it is recovered like the work of a worker that
	// exits (ADR-0013).
	ErrSandboxSaturated = errors.New("expr: sandbox saturated by abandoned evaluations")
)

// RestartReason is the reason a process gives when it shuts down because its
// sandbox is saturated or holds a stuck call.
const RestartReason = "sandbox_abandoned"

// Health lets the composition root know that the sandbox can no longer be
// trusted to serve this process, so the process must restart (ADR-0018
// section 3, amendment 10).
type Health interface {
	// Degraded returns a channel that is closed when the sandbox is saturated
	// or holds a stuck abandoned call. It never reopens.
	Degraded() <-chan struct{}
	// HealthReport describes the state, for the log line of the restart.
	HealthReport() HealthReport
}

// HealthReport counts the abandoned evaluations of a sandbox.
type HealthReport struct {
	// AbandonedRunning and Stuck are the abandoned calls still running now and
	// the part of them that has run longer than the stuck limit.
	AbandonedRunning, Stuck int
	// MaxAbandoned is the limit of abandoned calls still running, and Slots
	// the number of calls that may execute guest code at once.
	MaxAbandoned, Slots int
	// Abandoned and Saturated count, since the process started, the calls
	// abandoned at their deadline and the calls refused because of the limit.
	Abandoned, Saturated uint64
}

// CallIdentity names the execution a sandbox call belongs to, for the log
// line of an abandoned call. It never holds the expression or its data.
type CallIdentity struct {
	OrganizationID, ExecutionID, NodeID string
}

type callIdentityKey struct{}

type callTimeoutKey struct{}

// WithTimeout returns a context that asks the runtime for a time limit of d
// for the call, counted from the moment the call begins to run.
func WithTimeout(ctx context.Context, d time.Duration) context.Context {
	return context.WithValue(ctx, callTimeoutKey{}, d)
}

// TimeoutFrom returns the duration [WithTimeout] stored, or zero.
func TimeoutFrom(ctx context.Context) time.Duration {
	d, _ := ctx.Value(callTimeoutKey{}).(time.Duration)
	return d
}

// WithCallIdentity returns a context that carries id to the sandbox adapter.
func WithCallIdentity(ctx context.Context, id CallIdentity) context.Context {
	return context.WithValue(ctx, callIdentityKey{}, id)
}

// CallIdentityFrom returns the identity [WithCallIdentity] stored, or the zero
// value.
func CallIdentityFrom(ctx context.Context) CallIdentity {
	id, _ := ctx.Value(callIdentityKey{}).(CallIdentity)
	return id
}
