// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Parameter names and values of orcon.sort@1.
const (
	paramSortMode   = "mode"
	paramSortFields = "fields"
	paramSortField  = "field"
	paramSortOrder  = "order"

	// SortSimple orders the items by one or more fields.
	SortSimple = "SIMPLE"
	// SortRandom puts the items in a random order that the recorded seed of
	// the activation fixes, so a recomputed activation gives the same order.
	SortRandom = "RANDOM"

	// OrderAscending is the default order of a sort field.
	OrderAscending = "ASCENDING"
	// OrderDescending reverses the order of a sort field.
	OrderDescending = "DESCENDING"
)

// SortField is one field the items are ordered by.
type SortField struct {
	// Field is a field name or a path through nested objects.
	Field      string
	Descending bool
}

// SortParams is the parsed parameters of orcon.sort@1.
type SortParams struct {
	Mode string
	// Fields are the sort keys in order of priority; empty in random mode.
	Fields []SortField
}

// ParseSort reads the parameters of orcon.sort@1. In SIMPLE mode (the
// default) at least one field is required; in RANDOM mode there is none.
func ParseSort(params jsonvalue.Value) (SortParams, error) {
	m, err := readMembers(params, paramSortMode, paramSortFields)
	if err != nil {
		return SortParams{}, err
	}
	mode, err := m.enum(paramSortMode, SortSimple, SortSimple, SortRandom)
	if err != nil {
		return SortParams{}, err
	}
	out := SortParams{Mode: mode}
	raw, hasFields := m[paramSortFields]
	if mode == SortRandom {
		if hasFields && (raw.Kind() != jsonvalue.KindArray || raw.Len() != 0) {
			return SortParams{}, fmt.Errorf("%s must be empty in the %s mode", paramSortFields, SortRandom)
		}
		return out, nil
	}
	if !hasFields || raw.Kind() != jsonvalue.KindArray || raw.Len() == 0 {
		return SortParams{}, fmt.Errorf("%s must list at least one field in the %s mode", paramSortFields, SortSimple)
	}
	if raw.Len() > maxListedFields {
		return SortParams{}, fmt.Errorf("%s has more than %d fields", paramSortFields, maxListedFields)
	}
	for i, f := range raw.Items() {
		sf, err := parseSortField(f, i)
		if err != nil {
			return SortParams{}, err
		}
		for _, prev := range out.Fields {
			if prev.Field == sf.Field {
				return SortParams{}, fmt.Errorf("%s names the field %q more than once", paramSortFields, sf.Field)
			}
		}
		out.Fields = append(out.Fields, sf)
	}
	return out, nil
}

func parseSortField(v jsonvalue.Value, i int) (SortField, error) {
	if v.Kind() != jsonvalue.KindObject {
		return SortField{}, fmt.Errorf("%s[%d] must be an object", paramSortFields, i)
	}
	var sf SortField
	var hasField bool
	for _, m := range v.Members() {
		switch m.Key {
		case paramSortField:
			name, err := fieldName(m.Value, fmt.Sprintf("%s[%d].%s", paramSortFields, i, paramSortField))
			if err != nil {
				return SortField{}, err
			}
			sf.Field, hasField = name, true
		case paramSortOrder:
			s, ok := m.Value.Str()
			if !ok || (s != OrderAscending && s != OrderDescending) {
				return SortField{}, fmt.Errorf("%s[%d].%s must be %s or %s", paramSortFields, i, paramSortOrder, OrderAscending, OrderDescending)
			}
			sf.Descending = s == OrderDescending
		default:
			return SortField{}, fmt.Errorf("%s[%d] has unknown key %q", paramSortFields, i, m.Key)
		}
	}
	if !hasField {
		return SortField{}, fmt.Errorf("%s[%d] has no %s", paramSortFields, i, paramSortField)
	}
	return sf, nil
}

// ApplySort returns the items in order. The sort is stable: items that tie on
// every field keep their input order. The seed fixes the order of the RANDOM
// mode. The input slice is not changed.
func ApplySort(p SortParams, items []execution.Item, seed uint64) []execution.Item {
	out := slices.Clone(items)
	if p.Mode == SortRandom {
		return shuffled(out, seed)
	}
	slices.SortStableFunc(out, func(a, b execution.Item) int {
		for _, f := range p.Fields {
			c := compareValues(lookup(a.JSON, f.Field), lookup(b.JSON, f.Field))
			if f.Descending {
				c = -c
			}
			if c != 0 {
				return c
			}
		}
		return 0
	})
	return out
}

// shuffled orders items by a SHA-256 of the seed and the position of each, so
// the same seed gives the same permutation on every platform and the
// permutation is uniform for a hash with independent outputs.
func shuffled(items []execution.Item, seed uint64) []execution.Item {
	type keyed struct {
		key  [sha256.Size]byte
		item execution.Item
	}
	ks := make([]keyed, len(items))
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[:8], seed)
	for i := range items {
		binary.BigEndian.PutUint64(buf[8:], uint64(i))
		ks[i] = keyed{key: sha256.Sum256(buf[:]), item: items[i]}
	}
	slices.SortFunc(ks, func(a, b keyed) int { return bytes.Compare(a.key[:], b.key[:]) })
	for i := range ks {
		items[i] = ks[i].item
	}
	return items
}

// valueRank orders the kinds of value when two values of different kinds are
// compared: a missing field or null first, then booleans, numbers, strings,
// arrays and objects.
func valueRank(v jsonvalue.Value) int {
	switch v.Kind() {
	case jsonvalue.KindInvalid, jsonvalue.KindNull:
		return 0
	case jsonvalue.KindBool:
		return 1
	case jsonvalue.KindNumber:
		return 2
	case jsonvalue.KindString:
		return 3
	case jsonvalue.KindArray:
		return 4
	case jsonvalue.KindObject:
		return 5
	default:
		return 6
	}
}

// compareValues is the total order of the sort: by kind rank, then within a
// kind. Numbers compare by value, strings by UTF-16 code units (the order of
// RFC 8785, which is the same on every platform), booleans false before true,
// and arrays and objects by their canonical text.
func compareValues(a, b jsonvalue.Value) int {
	ra, rb := valueRank(a), valueRank(b)
	if ra != rb {
		return ra - rb
	}
	switch a.Kind() {
	case jsonvalue.KindBool:
		x, _ := a.Bool()
		y, _ := b.Bool()
		switch {
		case x == y:
			return 0
		case !x:
			return -1
		default:
			return 1
		}
	case jsonvalue.KindNumber:
		x, okx := a.Float64()
		y, oky := b.Float64()
		switch {
		case !okx || !oky:
			return 0
		case x < y:
			return -1
		case x > y:
			return 1
		default:
			return 0
		}
	case jsonvalue.KindString:
		x, _ := a.Str()
		y, _ := b.Str()
		return jsonvalue.CompareUTF16(x, y)
	case jsonvalue.KindArray, jsonvalue.KindObject:
		x, errx := jsonvalue.Canonical(a)
		y, erry := jsonvalue.Canonical(b)
		if errx != nil || erry != nil {
			return 0
		}
		return jsonvalue.CompareUTF16(string(x), string(y))
	case jsonvalue.KindInvalid, jsonvalue.KindNull:
		return 0
	}
	return 0
}

func runSort(_ context.Context, _ *Runner, call execution.NodeCall) (execution.NodeOutput, error) {
	p, err := ParseSort(call.Node.Parameters)
	if err != nil {
		return execution.NodeOutput{}, invalid(call.Node.Type, err)
	}
	return output(ApplySort(p, call.Input.Items(), call.Env.Seed)), nil
}
