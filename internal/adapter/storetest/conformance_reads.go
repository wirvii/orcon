// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/platform/id"
)

// nodeReadsCase checks the reads the API serves for an execution: its node executions with their
// attempt counts and its stored output batches with the node of each, paged oldest first and
// invisible from another organization.
func nodeReadsCase(t *testing.T, newStore Factory) { //nolint:gocognit // a conformance case reads as one scenario; splitting it would hide the order of its steps
	t.Helper()
	e := newExecFixture(t, newStore)
	ex := e.execution()
	other := e.execution()
	w := worker(e)
	if err := e.s.RegisterWorker(e.ctx(), execution.WorkerRecord{ID: w, Role: execution.WorkerWorker, Host: "host", Version: "1.0.0", StartedAt: base, HeartbeatAt: base}); err != nil {
		t.Fatal(err)
	}
	const nodes = 5
	var runs []store.NodeExecution
	for i := range nodes {
		tok := e.token(ex, fmt.Sprintf("n%d", i), 0, base)
		run := store.NodeExecution{
			ID: next[store.NodeExecutionID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, TokenID: tok.ID, NodeID: fmt.Sprintf("n%d", i),
			Status: "SUCCEEDED", StartedAt: base, FinishedAt: base.Add(time.Second), CreatedAt: base.Add(time.Duration(i) * time.Second), UpdatedAt: base,
		}
		runs = append(runs, run)
		mustTx(t, e.tx(e.orgA, func(ctx context.Context, tx store.Tx) error {
			if err := tx.InsertNodeExecution(ctx, run); err != nil {
				return err
			}
			if err := tx.InsertAttempt(ctx, store.TaskAttempt{
				ID: next[id.UUID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, NodeExecutionID: run.ID,
				AttemptNumber: 1, LeaseEpoch: 1, Worker: w, Status: "SUCCEEDED", StartedAt: base, FinishedAt: base,
			}); err != nil {
				return err
			}
			return tx.PutNodeOutput(ctx, store.NodeOutput{
				ID: next[store.PayloadID](e.ids), OrgID: e.orgA, ExecutionID: ex.ID, NodeExecutionID: run.ID, Port: "main", ItemCount: 1,
				Encoding: execution.ItemsEncoding, Document: []byte(`{"items":[{"json":{"i":1},"lineage":[]}]}`), CreatedAt: base.Add(time.Duration(i) * time.Second),
			})
		}), "write a node run")
	}

	var got []store.NodeExecution
	var cursor store.Cursor
	for {
		rows, err := e.s.ListNodeExecutions(e.ctx(), e.scopeA, ex.ID, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			break
		}
		if len(rows) > 2 {
			t.Fatalf("page of %d, limit 2", len(rows))
		}
		got = append(got, rows...)
		last := rows[len(rows)-1]
		cursor = store.Cursor{CreatedAt: last.CreatedAt, ID: [16]byte(last.ID)}
	}
	if len(got) != nodes {
		t.Fatalf("listed %d node executions, want %d", len(got), nodes)
	}
	for i := range got {
		if got[i].ID != runs[i].ID || got[i].NodeID != runs[i].NodeID || got[i].Status != "SUCCEEDED" || got[i].FinishedAt.IsZero() || got[i].AttemptCount != 1 {
			t.Fatalf("node execution %d = %+v, want %+v (oldest first)", i, got[i], runs[i])
		}
	}

	var outs []store.NodeOutputRecord
	cursor = store.Cursor{}
	for {
		rows, err := e.s.ListNodeOutputs(e.ctx(), e.scopeA, ex.ID, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			break
		}
		outs = append(outs, rows...)
		last := rows[len(rows)-1]
		cursor = store.Cursor{CreatedAt: last.CreatedAt, ID: [16]byte(last.ID)}
	}
	if len(outs) != nodes {
		t.Fatalf("listed %d outputs, want %d", len(outs), nodes)
	}
	for i := range outs {
		if outs[i].NodeExecutionID != runs[i].ID || outs[i].NodeID != runs[i].NodeID || outs[i].Port != "main" || outs[i].ItemCount != 1 || len(outs[i].Document) == 0 {
			t.Fatalf("output %d = %+v", i, outs[i])
		}
	}

	// Another execution of the same organization lists nothing of this one, and another
	// organization lists nothing at all.
	if rows, err := e.s.ListNodeExecutions(e.ctx(), e.scopeA, other.ID, store.Cursor{}, 10); err != nil || len(rows) != 0 {
		t.Errorf("node executions of a sibling execution: %d, %v", len(rows), err)
	}
	if rows, err := e.s.ListNodeExecutions(e.ctx(), e.scopeB, ex.ID, store.Cursor{}, 10); err != nil || len(rows) != 0 {
		t.Errorf("node executions of another organization: %d, %v", len(rows), err)
	}
	if rows, err := e.s.ListNodeOutputs(e.ctx(), e.scopeB, ex.ID, store.Cursor{}, 10); err != nil || len(rows) != 0 {
		t.Errorf("outputs of another organization: %d, %v", len(rows), err)
	}
}
