// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestBatchIsImmutable(t *testing.T) {
	t.Parallel()
	items := []execution.Item{
		{JSON: jsonvalue.Int(1), Lineage: []execution.LineageRef{{Port: "main", Index: 0}}},
		{JSON: jsonvalue.Int(2)},
	}
	b := execution.NewBatch(items...)
	items[0].JSON = jsonvalue.Int(99)
	items[0].Lineage[0].Index = 99
	if b.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", b.Len())
	}
	if got, _ := b.Item(0).JSON.Int64(); got != 1 {
		t.Errorf("item 0 = %d after the caller changed its slice, want 1", got)
	}
	if got := b.Item(0).Lineage[0].Index; got != 0 {
		t.Errorf("lineage index = %d after the caller changed it, want 0", got)
	}
	out := b.Items()
	out[1].JSON = jsonvalue.Int(77)
	b.Item(0).Lineage[0].Index = 55
	if got, _ := b.Item(1).JSON.Int64(); got != 2 {
		t.Errorf("item 1 = %d after changing a copy, want 2", got)
	}
	if got := b.Item(0).Lineage[0].Index; got != 0 {
		t.Errorf("lineage index = %d after changing a copy, want 0", got)
	}
	var zero execution.Batch
	if zero.Len() != 0 || len(zero.Items()) != 0 {
		t.Error("the zero Batch is not empty")
	}
}

func TestMemoryPayloads(t *testing.T) {
	t.Parallel()
	store := execution.NewMemoryPayloads()
	ctx := t.Context()
	first := executiontest.Batch(`{"v":1}`)
	if err := store.Put(ctx, "ref", first); err != nil {
		t.Fatalf("Put() = %v", err)
	}
	// A deterministic ref carries the same data, so a second Put is a no-op.
	if err := store.Put(ctx, "ref", executiontest.Batch(`{"v":2}`)); err != nil {
		t.Fatalf("second Put() = %v", err)
	}
	got, err := store.Get(ctx, "ref")
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if diff := cmp.Diff(executiontest.Texts(first), executiontest.Texts(got)); diff != "" {
		t.Errorf("stored batch mismatch (-want +got): the first Put wins\n%s", diff)
	}
	if store.Len() != 1 {
		t.Errorf("Len() = %d, want 1", store.Len())
	}
	if _, err := store.Get(ctx, "missing"); !errors.Is(err, execution.ErrPayloadNotFound) {
		t.Errorf("Get(missing) = %v, want ErrPayloadNotFound", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.Put(canceled, "x", first); !errors.Is(err, context.Canceled) {
		t.Errorf("Put() with a canceled context = %v, want context.Canceled", err)
	}
	if _, err := store.Get(canceled, "ref"); !errors.Is(err, context.Canceled) {
		t.Errorf("Get() with a canceled context = %v, want context.Canceled", err)
	}
}
