// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package jsonvalue

import (
	"fmt"
	"math"
	"slices"
	"strconv"
)

// Kind is the JSON type of a [Value].
type Kind uint8

// The kinds of JSON value. The zero Kind marks an absent value.
const (
	// KindInvalid is the kind of the zero Value: no value at all.
	KindInvalid Kind = iota
	// KindNull is JSON null.
	KindNull
	// KindBool is JSON true or false.
	KindBool
	// KindNumber is a JSON number, kept as its decimal text.
	KindNumber
	// KindString is a JSON string.
	KindString
	// KindArray is a JSON array.
	KindArray
	// KindObject is a JSON object with unique keys.
	KindObject
)

// String returns the lowercase name of the kind.
func (k Kind) String() string {
	switch k {
	case KindInvalid:
		return "absent"
	case KindNull:
		return "null"
	case KindBool:
		return "boolean"
	case KindNumber:
		return "number"
	case KindString:
		return "string"
	case KindArray:
		return "array"
	case KindObject:
		return "object"
	default:
		return "kind(" + strconv.Itoa(int(k)) + ")"
	}
}

// Member is one key and value of a JSON object.
type Member struct {
	Key   string
	Value Value
}

// Value is an immutable JSON value. The zero Value is absent (KindInvalid).
// Constructors copy their arguments and accessors return copies, so a Value
// can be shared freely between goroutines.
type Value struct {
	kind    Kind
	flag    bool
	text    string // string content, or the decimal text of a number
	items   []Value
	members []Member // insertion order, keys unique
}

// Null returns the JSON null value.
func Null() Value { return Value{kind: KindNull} }

// Bool returns a JSON boolean.
func Bool(b bool) Value { return Value{kind: KindBool, flag: b} }

// String returns a JSON string.
func String(s string) Value { return Value{kind: KindString, text: s} }

// Int returns a JSON number holding n.
func Int(n int64) Value { return Value{kind: KindNumber, text: strconv.FormatInt(n, 10)} }

// Float returns a JSON number holding the shortest decimal text of f.
// It fails when f is NaN or infinite, which JSON cannot represent.
func Float(f float64) (Value, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return Value{}, fmt.Errorf("jsonvalue: %v is not a JSON number", f)
	}
	return Value{kind: KindNumber, text: strconv.FormatFloat(f, 'g', -1, 64)}, nil
}

// Number returns a JSON number from its decimal text, which must follow the
// JSON number grammar (RFC 8259 section 6).
func Number(text string) (Value, error) {
	if !validNumber(text) {
		return Value{}, fmt.Errorf("jsonvalue: %q is not a JSON number", text)
	}
	return Value{kind: KindNumber, text: text}, nil
}

// Array returns a JSON array of the given items.
func Array(items ...Value) Value {
	return Value{kind: KindArray, items: slices.Clone(items)}
}

// Object returns a JSON object of the given members. It fails on a duplicate
// key, because the keys of a JSON object are unique in Orcon.
func Object(members ...Member) (Value, error) {
	seen := make(map[string]struct{}, len(members))
	for _, m := range members {
		if _, dup := seen[m.Key]; dup {
			return Value{}, fmt.Errorf("jsonvalue: duplicate object key %q", m.Key)
		}
		seen[m.Key] = struct{}{}
	}
	return Value{kind: KindObject, members: slices.Clone(members)}, nil
}

// Kind returns the JSON type of v.
func (v Value) Kind() Kind { return v.kind }

// IsZero reports whether v is the absent value.
func (v Value) IsZero() bool { return v.kind == KindInvalid }

// Bool returns the boolean held by v and whether v is a boolean.
func (v Value) Bool() (value, ok bool) { return v.flag, v.kind == KindBool }

// Str returns the string held by v and whether v is a string.
func (v Value) Str() (string, bool) {
	if v.kind != KindString {
		return "", false
	}
	return v.text, true
}

// NumberText returns the decimal text of a number and whether v is a number.
func (v Value) NumberText() (string, bool) {
	if v.kind != KindNumber {
		return "", false
	}
	return v.text, true
}

// Int64 returns the number as an integer. It reports false when v is not a
// number or its text is not an integer in the int64 range.
func (v Value) Int64() (int64, bool) {
	if v.kind != KindNumber {
		return 0, false
	}
	n, err := strconv.ParseInt(v.text, 10, 64)
	return n, err == nil
}

// Float64 returns the number as a float64. It reports false when v is not a
// number or the value does not fit a finite float64.
func (v Value) Float64() (float64, bool) {
	if v.kind != KindNumber {
		return 0, false
	}
	f, err := strconv.ParseFloat(v.text, 64)
	if err != nil || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// Len returns the number of items of an array or members of an object, and 0
// for any other kind.
func (v Value) Len() int {
	switch v.kind {
	case KindArray:
		return len(v.items)
	case KindObject:
		return len(v.members)
	case KindInvalid, KindNull, KindBool, KindNumber, KindString:
		return 0
	default:
		return 0
	}
}

// Items returns a copy of the items of an array, or nil for any other kind.
func (v Value) Items() []Value {
	if v.kind != KindArray {
		return nil
	}
	return slices.Clone(v.items)
}

// Members returns a copy of the members of an object in insertion order, or
// nil for any other kind.
func (v Value) Members() []Member {
	if v.kind != KindObject {
		return nil
	}
	return slices.Clone(v.members)
}

// Get returns the member named key of an object.
func (v Value) Get(key string) (Value, bool) {
	if v.kind != KindObject {
		return Value{}, false
	}
	for _, m := range v.members {
		if m.Key == key {
			return m.Value, true
		}
	}
	return Value{}, false
}

// Without returns a copy of the object v with the member named key removed.
// Any other kind, or an object without the key, is returned unchanged.
func (v Value) Without(key string) Value {
	if v.kind != KindObject {
		return v
	}
	kept := make([]Member, 0, len(v.members))
	for _, m := range v.members {
		if m.Key != key {
			kept = append(kept, m)
		}
	}
	return Value{kind: KindObject, members: kept}
}

// With returns a copy of the object v where the member named key holds val,
// replacing an existing member or appending a new one. Any other kind is
// returned unchanged.
func (v Value) With(key string, val Value) Value {
	if v.kind != KindObject {
		return v
	}
	members := slices.Clone(v.members)
	for i := range members {
		if members[i].Key == key {
			members[i].Value = val
			return Value{kind: KindObject, members: members}
		}
	}
	return Value{kind: KindObject, members: append(members, Member{Key: key, Value: val})}
}

// Equal reports whether a and b are the same value. Numbers compare by text,
// objects by members irrespective of order.
func Equal(a, b Value) bool {
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case KindInvalid, KindNull:
		return true
	case KindBool:
		return a.flag == b.flag
	case KindNumber, KindString:
		return a.text == b.text
	case KindArray:
		return slices.EqualFunc(a.items, b.items, Equal)
	case KindObject:
		if len(a.members) != len(b.members) {
			return false
		}
		for _, m := range a.members {
			other, ok := b.Get(m.Key)
			if !ok || !Equal(m.Value, other) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// validNumber reports whether s follows the JSON number grammar.
func validNumber(s string) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	switch {
	case i < len(s) && s[i] == '0':
		i++
	case i < len(s) && s[i] >= '1' && s[i] <= '9':
		i = skipDigits(s, i)
	default:
		return false
	}
	if i < len(s) && s[i] == '.' {
		j := skipDigits(s, i+1)
		if j == i+1 {
			return false
		}
		i = j
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		j := skipDigits(s, i)
		if j == i {
			return false
		}
		i = j
	}
	return i == len(s)
}

func skipDigits(s string, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}
