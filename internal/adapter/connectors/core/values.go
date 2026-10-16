// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"slices"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// valueKey is a string that is the same for two values exactly when they are
// the same JSON value: objects are equal whatever the order of their members,
// numbers are equal when they have the same decimal value (1, 1.0 and 1e0 are
// one number) and no number is rounded, so two long integers that a double
// cannot tell apart stay different. A string is written with its length, so
// no two different values can produce the same key.
func valueKey(v jsonvalue.Value) string {
	var b strings.Builder
	writeKey(&b, v)
	return b.String()
}

func writeKey(b *strings.Builder, v jsonvalue.Value) {
	switch v.Kind() {
	case jsonvalue.KindInvalid:
		b.WriteByte('u')
	case jsonvalue.KindNull:
		b.WriteByte('n')
	case jsonvalue.KindBool:
		if t, _ := v.Bool(); t {
			b.WriteByte('t')
		} else {
			b.WriteByte('f')
		}
	case jsonvalue.KindNumber:
		text, _ := v.NumberText()
		b.WriteString("d" + decimalKey(text) + ";")
	case jsonvalue.KindString:
		s, _ := v.Str()
		b.WriteString("s" + strconv.Itoa(len(s)) + ":" + s)
	case jsonvalue.KindArray:
		b.WriteByte('[')
		for _, it := range v.Items() {
			writeKey(b, it)
		}
		b.WriteByte(']')
	case jsonvalue.KindObject:
		ms := v.Members()
		slices.SortFunc(ms, func(x, y jsonvalue.Member) int { return jsonvalue.CompareUTF16(x.Key, y.Key) })
		b.WriteByte('{')
		for _, m := range ms {
			b.WriteString(strconv.Itoa(len(m.Key)) + ":" + m.Key)
			writeKey(b, m.Value)
		}
		b.WriteByte('}')
	}
}

// decimalKey writes the decimal text of a JSON number in one normal form:
// sign, the significant digits without trailing zeros, and a power of ten.
// Zero has no sign. A text that is not a plain decimal is returned as it is.
func decimalKey(text string) string {
	neg := strings.HasPrefix(text, "-")
	t := strings.TrimPrefix(text, "-")
	mantissa, exp := t, 0
	if i := strings.IndexAny(t, "eE"); i >= 0 {
		e, err := strconv.Atoi(t[i+1:])
		if err != nil {
			return text
		}
		mantissa, exp = t[:i], e
	}
	whole, frac := mantissa, ""
	if i := strings.IndexByte(mantissa, '.'); i >= 0 {
		whole, frac = mantissa[:i], mantissa[i+1:]
	}
	digits := strings.TrimLeft(whole+frac, "0")
	exp -= len(frac)
	if digits == "" {
		return "0"
	}
	trimmed := strings.TrimRight(digits, "0")
	exp += len(digits) - len(trimmed)
	out := trimmed + "e" + strconv.Itoa(exp)
	if neg {
		out = "-" + out
	}
	return out
}

// withoutField returns the item without the named field: the key of that exact
// name, or the last member of a path through nested objects. Any other item,
// or one that lacks the field, is returned as it is.
func withoutField(item jsonvalue.Value, name string) jsonvalue.Value {
	if _, ok := item.Get(name); ok {
		return item.Without(name)
	}
	head, rest, nested := strings.Cut(name, ".")
	if !nested {
		return item
	}
	child, ok := item.Get(head)
	if !ok || child.Kind() != jsonvalue.KindObject {
		return item
	}
	return item.With(head, withoutField(child, rest))
}

// onlyFields returns an object with the named fields of the item that are
// there, each under its full name, in the order the names are listed.
func onlyFields(item jsonvalue.Value, names []string) jsonvalue.Value {
	out, _ := jsonvalue.Object()
	for _, name := range names {
		if v := lookup(item, name); !v.IsZero() {
			out = out.With(name, v)
		}
	}
	return out
}

// mergedLineage is the lineage of an item that comes from several input
// items: every reference once, in the order the items come.
func mergedLineage(items []execution.Item) []execution.LineageRef {
	var out []execution.LineageRef
	seen := map[execution.LineageRef]bool{}
	for _, it := range items {
		for _, ref := range it.Lineage {
			if !seen[ref] {
				seen[ref] = true
				out = append(out, ref)
			}
		}
	}
	return out
}
