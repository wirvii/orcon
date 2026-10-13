// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// LineageRef says which output item produced an item: the port of the
// producing node and the item's index in that port's batch (ADR-0008 section 1).
type LineageRef struct {
	Port  string
	Index int
}

// Item is one element of a batch: JSON data and its lineage. Binary
// attachments arrive with the blob cut (ADR-0016).
type Item struct {
	JSON    jsonvalue.Value
	Lineage []LineageRef
}

// Batch is an ordered, immutable list of items. Nodes receive one batch per
// activation and emit one batch per output port. The zero Batch is empty.
type Batch struct {
	items []Item
}

// NewBatch returns a batch of the given items. It copies its argument.
func NewBatch(items ...Item) Batch {
	return Batch{items: cloneItems(items)}
}

// Len returns the number of items.
func (b Batch) Len() int { return len(b.items) }

// Item returns the item at index i. It panics when i is out of range, like a
// slice index.
func (b Batch) Item(i int) Item { return cloneItem(b.items[i]) }

// Items returns a copy of the items.
func (b Batch) Items() []Item { return cloneItems(b.items) }

// concat returns the items of the batches in order.
func concat(batches []Batch) Batch {
	n := 0
	for _, b := range batches {
		n += len(b.items)
	}
	items := make([]Item, 0, n)
	for _, b := range batches {
		items = append(items, b.items...)
	}
	return Batch{items: items}
}

// pick returns the batch holding the items at the given positions, in the
// order given. Items are immutable, so the subset shares them.
func (b Batch) pick(positions []int) Batch {
	items := make([]Item, 0, len(positions))
	for _, p := range positions {
		items = append(items, b.items[p])
	}
	return Batch{items: items}
}

func cloneItem(it Item) Item {
	it.Lineage = slices.Clone(it.Lineage)
	return it
}

func cloneItems(items []Item) []Item {
	out := make([]Item, len(items))
	for i, it := range items {
		out[i] = cloneItem(it)
	}
	return out
}

// PayloadRef names an immutable batch in a [Payloads] store. Refs are derived
// from deterministic ids (ADR-0008 section 6), so writing the same batch twice
// is idempotent. A token carries a ref, never a copy of the data.
type PayloadRef string

// Payloads is the port through which the interpreter stores and reads
// immutable batches: one document per node execution and output port, plus the
// execution input and the combined input of a join continuation. The storage
// cut (C2) implements it over the database and, past the size threshold, the
// blob store (ADR-0008 section 3, ADR-0016).
type Payloads interface {
	// Put stores a batch under ref. Storing the same ref again is a no-op:
	// refs are deterministic, so a repeated Put carries the same data.
	Put(ctx context.Context, ref PayloadRef, batch Batch) error
	// Get returns the batch stored under ref, or an error wrapping
	// [ErrPayloadNotFound].
	Get(ctx context.Context, ref PayloadRef) (Batch, error)
}

// MemoryPayloads is a [Payloads] kept in memory. It is safe for concurrent use.
type MemoryPayloads struct {
	mu      sync.Mutex
	batches map[PayloadRef]Batch
}

// NewMemoryPayloads returns an empty in-memory payload store.
func NewMemoryPayloads() *MemoryPayloads {
	return &MemoryPayloads{batches: make(map[PayloadRef]Batch)}
}

// Put implements [Payloads].
func (m *MemoryPayloads) Put(ctx context.Context, ref PayloadRef, batch Batch) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("put payload %s: %w", ref, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.batches[ref]; !ok {
		m.batches[ref] = batch
	}
	return nil
}

// Get implements [Payloads].
func (m *MemoryPayloads) Get(ctx context.Context, ref PayloadRef) (Batch, error) {
	if err := ctx.Err(); err != nil {
		return Batch{}, fmt.Errorf("get payload %s: %w", ref, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.batches[ref]
	if !ok {
		return Batch{}, fmt.Errorf("get payload %s: %w", ref, ErrPayloadNotFound)
	}
	return b, nil
}

// Len returns the number of stored batches.
func (m *MemoryPayloads) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.batches)
}
