// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// Engine is a process of the durable engine: workers that run tokens, the
// reaper that recovers expired leases and fires timers, and the dispatcher of
// the outbox. Several processes (or, on SQLite, goroutines of one process) share
// one database; correctness never depends on how many there are.
type Engine struct {
	cfg    Config
	store  Store
	plans  *Plans
	interp *execution.Interpreter

	leases   *leaseTable
	claims   *claimState
	draining atomic.Bool
	wake     chan struct{}

	metaMu sync.Mutex
	meta   map[store.ExecutionID]*store.Execution
}

// New returns an engine over a store. The ports of the interpreter (conditions,
// item routing, the node runner) are the ones in deps; deps.Payloads and
// deps.Clock are set by the engine.
func New(cfg *Config, st Store, revisions Revisions, reader DocumentReader, deps execution.Deps, limits PlanLimits) (*Engine, error) {
	if cfg.PoisonThreshold != 0 && cfg.PoisonThreshold < store.MinPoisonThreshold {
		return nil, fmt.Errorf("new durable engine: PoisonThreshold %d is below the minimum %d (with 1 every token would have to run alone)", cfg.PoisonThreshold, store.MinPoisonThreshold)
	}
	cfg.defaults()
	if cfg.Worker.IsZero() {
		return nil, errors.New("new durable engine: Worker is required")
	}
	deps.Payloads = payloadReader{reader: st}
	deps.Clock = cfg.Clock
	e := &Engine{
		cfg: *cfg, store: st, plans: NewPlans(revisions, reader, limits), interp: execution.NewInterpreter(deps),
		leases: newLeaseTable(cfg.LeaseTTL), claims: &claimState{}, wake: make(chan struct{}, cfg.Concurrency), meta: make(map[store.ExecutionID]*store.Execution),
	}
	if router, ok := st.(store.EventRouter); ok {
		router.RouteEvents(cfg.Outbox)
	}
	return e, nil
}

// Worker returns the identity of this process in the registry and in its leases.
func (e *Engine) Worker() execution.WorkerID { return e.cfg.Worker }

// Plans returns the loader of flow revisions, for callers that start executions.
func (e *Engine) Plans() *Plans { return e.plans }

// StartRequest starts an execution of a pinned revision.
type StartRequest struct {
	OrgID     identity.OrgID
	ProjectID identity.ProjectID
	FlowID    flow.FlowID
	// Exactly one of the two names the revision the execution runs.
	FlowVersionID  flow.FlowVersionID
	FlowSnapshotID flow.FlowSnapshotID
	Mode           string
	Priority       int64
	// ID names the execution; the engine makes one when it is the zero value.
	ID store.ExecutionID
	// Start names the start event when the flow has several.
	Start flow.NodeID
	Input execution.Batch
}

// Start creates an execution, its root token (RUNNABLE) and its input in one
// transaction, and returns its identity: from then on any worker may run it
// (ADR-0009 section 1: a trigger creates the execution and the root token
// atomically).
func (e *Engine) Start(ctx context.Context, r StartRequest) (store.ExecutionID, error) {
	mode := r.Mode
	if mode == "" {
		mode = "MANUAL"
	}
	execID := r.ID
	if execID.IsZero() {
		execID = store.ExecutionID(e.cfg.IDs.New())
	}
	now := e.cfg.Clock.Now()
	ex := &store.Execution{
		ID: execID, OrgID: r.OrgID, ProjectID: r.ProjectID, FlowID: r.FlowID, FlowVersionID: r.FlowVersionID, FlowSnapshotID: r.FlowSnapshotID,
		Status: string(execution.ExecQueued), Mode: mode, Priority: r.Priority, RootExecutionID: execID, RunnableCount: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	plan, err := e.plans.For(ctx, ex)
	if err != nil {
		return store.ExecutionID{}, fmt.Errorf("start execution: %w", err)
	}
	sess, err := e.interp.NewSession(ctx, plan, execution.Request{Execution: executionText(execID), Start: r.Start, Input: r.Input})
	if err != nil {
		return store.ExecutionID{}, fmt.Errorf("start execution: %w", err)
	}
	delta, err := sess.Delta()
	if err != nil {
		return store.ExecutionID{}, fmt.Errorf("start execution: %w", err)
	}
	err = e.store.InTx(ctx, r.OrgID, func(ctx context.Context, tx store.Tx) error {
		now := e.cfg.Clock.Now()
		ex.CreatedAt, ex.UpdatedAt = now, now
		if err := tx.InsertExecution(ctx, ex); err != nil {
			return fmt.Errorf("insert execution: %w", err)
		}
		created := execution.Event{Type: "execution.created", Execution: executionText(execID)}
		if err := e.appendEvent(ctx, tx, ex, &created, now); err != nil {
			return err
		}
		_, err = e.apply(ctx, tx, &applyInput{exec: ex, plan: plan, delta: &delta, revision: 0, now: now})
		return err
	})
	if err != nil {
		return store.ExecutionID{}, fmt.Errorf("start execution %s: %w", execID, err)
	}
	e.signal(1)
	return execID, nil
}

// metaOf returns the immutable row of an execution, read once per process.
func (e *Engine) metaOf(ctx context.Context, org identity.OrgID, exec store.ExecutionID) (*store.Execution, error) {
	e.metaMu.Lock()
	if m, ok := e.meta[exec]; ok {
		e.metaMu.Unlock()
		return m, nil
	}
	e.metaMu.Unlock()
	m, err := e.store.GetExecution(ctx, org, exec)
	if err != nil {
		return nil, fmt.Errorf("read execution %s: %w", exec, err)
	}
	e.metaMu.Lock()
	defer e.metaMu.Unlock()
	if len(e.meta) >= 4096 {
		clear(e.meta)
	}
	e.meta[exec] = &m
	return &m, nil
}

// signal wakes up to n idle workers of this process.
func (e *Engine) signal(n int) {
	if e.cfg.DisableWake {
		return
	}
	for range n {
		select {
		case e.wake <- struct{}{}:
		default:
			return
		}
	}
}
