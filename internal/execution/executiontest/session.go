// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package executiontest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
)

// Engine runs a compiled plan to its end with the given ports and returns what
// happened in the shape the in-memory interpreter returns it. The conformance
// suite runs its flows through one, so the same cases judge the in-memory
// interpreter, the session driver below and the durable engines over the
// databases.
type Engine func(tb testing.TB, doc *flow.Document, plan *execution.Plan, deps execution.Deps, input execution.Batch) execution.Result

var engine Engine = MemoryEngine

// UseEngine sets the engine [Run] uses, and returns a function that restores
// the previous one. Call it from TestMain or a test that owns the package
// state; the engine is not meant to be switched while tests run in parallel.
func UseEngine(e Engine) (restore func()) {
	previous := engine
	engine = e
	return func() { engine = previous }
}

// Diff describes how two results of the same flow differ, or returns "" when
// they are the same run: status and failure, events (without their numbering,
// which a store assigns), tokens, node executions and the items that reached an
// end event.
func Diff(want, got execution.Result) string {
	var out []string
	if want.Status != got.Status {
		out = append(out, fmt.Sprintf("status = %s, want %s", got.Status, want.Status))
	}
	if (want.Failure == nil) != (got.Failure == nil) || (want.Failure != nil && (want.Failure.Code != got.Failure.Code || want.Failure.Node != got.Failure.Node)) {
		out = append(out, fmt.Sprintf("failure = %v, want %v", got.Failure, want.Failure))
	}
	if d := cmp.Diff(withoutSequence(want.Events), withoutSequence(got.Events)); d != "" {
		out = append(out, "events differ (-want +got):\n"+d)
	}
	if d := cmp.Diff(want.Tokens, got.Tokens, cmpopts.EquateEmpty()); d != "" {
		out = append(out, "tokens differ (-want +got):\n"+d)
	}
	if d := cmp.Diff(want.NodeExecutions, got.NodeExecutions); d != "" {
		out = append(out, "node executions differ (-want +got):\n"+d)
	}
	if d := cmp.Diff(Paths(want), Paths(got)); d != "" {
		out = append(out, "end items differ (-want +got):\n"+d)
	}
	return strings.Join(out, "\n")
}

func withoutSequence(events []execution.Event) []execution.Event {
	out := slices.Clone(events)
	for i := range out {
		out[i].Sequence = 0
	}
	return out
}

// MemoryEngine is the in-memory interpreter.
func MemoryEngine(tb testing.TB, _ *flow.Document, plan *execution.Plan, deps execution.Deps, input execution.Batch) execution.Result {
	tb.Helper()
	res, err := execution.NewInterpreter(deps).Run(tb.Context(), plan, execution.Request{Execution: "exec-1", Input: input})
	if err != nil {
		tb.Fatalf("Run() = %v, want no error", err)
	}
	return res
}

// dbRow is a token row with the order in which it became available.
type dbRow struct {
	row  execution.TokenRow
	seq  int
	born int
}

// fakeDB keeps what a durable store would, in memory: the rows, the tracker
// document, the batches, the events. It applies a [execution.Delta] the way a
// transaction does.
type fakeDB struct {
	status   execution.State
	trackers []byte
	rows     map[execution.TokenID]*dbRow
	born     int
	seq      int
	payloads map[execution.PayloadRef][]byte

	events    []execution.Event
	nodeExecs []execution.NodeExecutionRecord
	ends      []execution.EndArrival
	failure   *execution.Failure
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		status: execution.ExecQueued, rows: make(map[execution.TokenID]*dbRow),
		payloads: make(map[execution.PayloadRef][]byte),
	}
}

// Get implements [execution.Payloads].
func (db *fakeDB) Get(_ context.Context, ref execution.PayloadRef) (execution.Batch, error) {
	doc, ok := db.payloads[ref]
	if !ok {
		return execution.Batch{}, fmt.Errorf("get payload %s: %w", ref, execution.ErrPayloadNotFound)
	}
	return execution.DecodeBatch(doc)
}

// Put implements [execution.Payloads].
func (db *fakeDB) Put(context.Context, execution.PayloadRef, execution.Batch) error {
	return errors.New("executiontest: the fake database stores payloads through deltas")
}

func (db *fakeDB) snapshot(exec execution.ID) execution.Snapshot {
	snap := execution.Snapshot{Execution: exec, Status: db.status, Trackers: slices.Clone(db.trackers)}
	for _, r := range db.rows {
		if !r.row.State.IsTerminal() {
			snap.Tokens = append(snap.Tokens, r.row)
		}
	}
	// A real store returns rows in an order of its own; shuffling here would hide
	// nothing, so keep it deterministic and independent of the creation order.
	slices.SortFunc(snap.Tokens, func(a, b execution.TokenRow) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return snap
}

// apply commits a delta, round-tripping the token documents the way a store
// does, and reports the events to the sink.
func (db *fakeDB) apply(d *execution.Delta) error {
	for i := range d.Tokens {
		row := d.Tokens[i]
		doc, err := execution.EncodeTokenDocument(&row)
		if err != nil {
			return err
		}
		var back execution.TokenRow
		if err := execution.DecodeTokenDocument(doc, &back); err != nil {
			return err
		}
		row.Lineage, row.Passed, row.Merged = back.Lineage, back.Passed, back.Merged
		db.born++
		db.rows[row.ID] = &dbRow{row: row, born: db.born}
	}
	for i := range d.Changed {
		c := &d.Changed[i]
		r, ok := db.rows[c.Row.ID]
		if !ok {
			return fmt.Errorf("executiontest: delta changes unknown token %s", c.Row.ID)
		}
		if r.row.State != c.From && (c.Row.State != execution.TokenCanceled || r.row.State.IsTerminal()) {
			return fmt.Errorf("executiontest: token %s is %s, the delta expected %s", c.Row.ID, r.row.State, c.From)
		}
		r.row = c.Row
	}
	for _, id := range d.Runnable {
		if r := db.rows[id]; r != nil && r.row.State == execution.TokenRunnable {
			db.seq++
			r.seq = db.seq
		}
	}
	for _, w := range d.Payloads {
		if _, ok := db.payloads[w.Ref]; !ok {
			doc, err := execution.EncodeBatch(w.Batch)
			if err != nil {
				return err
			}
			db.payloads[w.Ref] = doc
		}
	}
	db.nodeExecs = append(db.nodeExecs, d.NodeExecutions...)
	db.ends = append(db.ends, d.Ends...)
	if d.Failure != nil {
		db.failure = d.Failure
	}
	for i := range d.Events {
		e := d.Events[i]
		e.Sequence = len(db.events) + 1
		db.events = append(db.events, e)
	}
	db.status = d.State
	if d.TrackersChanged {
		db.trackers = slices.Clone(d.Trackers)
	}
	return nil
}

// nextRunnable returns the token that became available first.
func (db *fakeDB) nextRunnable() (*dbRow, bool) {
	var best *dbRow
	for _, r := range db.rows {
		if r.row.State == execution.TokenRunnable && (best == nil || r.seq < best.seq) {
			best = r
		}
	}
	return best, best != nil
}

// SessionEngine runs the plan through [execution.Session] over a fake
// database, one worker, the way a durable worker would: a session per segment,
// opened from the rows and discarded after its delta is applied. Its results
// must equal the in-memory interpreter's; that is what makes it a check of the
// snapshot, the tracker document and the delta.
func SessionEngine(tb testing.TB, _ *flow.Document, plan *execution.Plan, deps execution.Deps, input execution.Batch) execution.Result {
	tb.Helper()
	ctx := tb.Context()
	const exec = execution.ID("exec-1")
	db := newFakeDB()
	deps.Payloads = db
	in := execution.NewInterpreter(deps)
	sess, err := in.NewSession(ctx, plan, execution.Request{Execution: exec, Input: input})
	if err != nil {
		tb.Fatalf("NewSession() = %v", err)
	}
	commit := func(s *execution.Session) execution.Delta {
		d, err := s.Delta()
		if err != nil {
			tb.Fatalf("Delta() = %v", err)
		}
		if err := db.apply(&d); err != nil {
			tb.Fatalf("apply delta: %v", err)
		}
		return d
	}
	commit(sess)
	for {
		row, ok := db.nextRunnable()
		if !ok {
			break
		}
		row.row.State = execution.TokenLeased
		held := row.row.ID
		for held != "" {
			s, err := in.OpenSession(plan, db.snapshot(exec), execution.SessionOptions{})
			if err != nil {
				tb.Fatalf("OpenSession() = %v", err)
			}
			res, err := s.Work(ctx, held, execution.WorkOptions{StopAtFork: true})
			if err != nil {
				tb.Fatalf("Work() = %v", err)
			}
			d := commit(s)
			if deps.Events != nil {
				for i := range d.Events {
					if err := deps.Events.Emit(ctx, d.Events[i]); err != nil {
						tb.Fatalf("Emit() = %v", err)
					}
				}
			}
			held = res.Held
		}
	}
	return db.result(exec)
}

func (db *fakeDB) result(exec execution.ID) execution.Result {
	res := execution.Result{
		Execution: exec, Status: db.status, Failure: db.failure, Events: slices.Clone(db.events),
		NodeExecutions: slices.Clone(db.nodeExecs), Ends: slices.Clone(db.ends),
	}
	rows := make([]*dbRow, 0, len(db.rows))
	for _, r := range db.rows {
		rows = append(rows, r)
	}
	slices.SortFunc(rows, func(a, b *dbRow) int { return a.born - b.born })
	for _, r := range rows {
		res.Tokens = append(res.Tokens, execution.TokenRecord{
			ID: r.row.ID, State: r.row.State, Node: r.row.Node, Scope: r.row.Scope,
			Lineage: slices.Clone(r.row.Lineage), Payload: r.row.Payload,
		})
	}
	return res
}
