// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"fmt"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// ItemsFormat tags the document of a stored batch.
const ItemsFormat = "orcon/items@1"

// ItemsEncoding is the encoding column of a batch stored as ItemsFormat: the
// JSON text of the document. Compression arrives with the blob cut
// (ADR-0008 section 3).
const ItemsEncoding = "JSON"

// EncodeBatch writes the document of a batch: {"items":[{"json":…,"lineage":
// [{"port":…,"index":…}]}]}. Numbers keep their text, so a batch decodes to the
// items it was made of (ADR-0008 section 3).
func EncodeBatch(b Batch) ([]byte, error) {
	items := make([]jsonvalue.Value, b.Len())
	for i, it := range b.items {
		var w docWriter
		w.put("json", it.JSON)
		lineage := make([]jsonvalue.Value, len(it.Lineage))
		for j, l := range it.Lineage {
			var lw docWriter
			lw.str("port", l.Port)
			lw.put("index", jsonvalue.Int(int64(l.Index)))
			lineage[j] = lw.value()
		}
		w.list("lineage", lineage)
		items[i] = w.value()
	}
	var root docWriter
	root.put("items", jsonvalue.Array(items...))
	out, err := jsonvalue.Marshal(root.value())
	if err != nil {
		return nil, fmt.Errorf("encode batch: %w", err)
	}
	return out, nil
}

// DecodeBatch reads the document EncodeBatch writes.
func DecodeBatch(data []byte) (Batch, error) {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return Batch{}, fmt.Errorf("decode batch: %w", err)
	}
	var d docReader
	if !d.object(v, "the batch document") {
		return Batch{}, d.err
	}
	raw := d.list(v, "items")
	items := make([]Item, len(raw))
	for i, rv := range raw {
		if !d.object(rv, "an item") {
			return Batch{}, d.err
		}
		js, ok := rv.Get("json")
		if !ok {
			d.failf("item %d has no json member", i)
			return Batch{}, d.err
		}
		it := Item{JSON: js}
		for _, lv := range d.list(rv, "lineage") {
			it.Lineage = append(it.Lineage, LineageRef{Port: d.str(lv, "port"), Index: int(d.num(lv, "index"))})
		}
		items[i] = it
	}
	if d.err != nil {
		return Batch{}, d.err
	}
	return Batch{items: items}, nil
}
