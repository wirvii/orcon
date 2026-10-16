// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// maxFieldNameBytes bounds one field name or field path a parameter names.
const maxFieldNameBytes = 255

// maxListedFields bounds a list of field names in a parameter.
const maxListedFields = 100

// members is the parameters object of a node, read against a closed list of
// names. A name the node does not define is an error, so a misspelled
// parameter never silently does nothing (the rule orcon.set@1 set).
type members map[string]jsonvalue.Value

// readMembers reads a parameters object. An absent or null object has no
// members. A key that is not in allowed is an error.
func readMembers(params jsonvalue.Value, allowed ...string) (members, error) {
	out := members{}
	if params.IsZero() || params.Kind() == jsonvalue.KindNull {
		return out, nil
	}
	if params.Kind() != jsonvalue.KindObject {
		return nil, errors.New("parameters must be an object")
	}
	for _, m := range params.Members() {
		if !slices.Contains(allowed, m.Key) {
			return nil, fmt.Errorf("unknown parameter %q", m.Key)
		}
		out[m.Key] = m.Value
	}
	return out, nil
}

// enum reads a parameter whose value is one of the listed UPPER_SNAKE names.
// An absent parameter takes the default.
func (m members) enum(key, def string, allowed ...string) (string, error) {
	v, ok := m[key]
	if !ok {
		return def, nil
	}
	s, isString := v.Str()
	if !isString || !slices.Contains(allowed, s) {
		return "", fmt.Errorf("%s must be one of %s", key, strings.Join(allowed, ", "))
	}
	return s, nil
}

// integer reads a whole number in [lo, hi]; an absent one takes the default.
func (m members) integer(key string, def, lo, hi int64) (int64, error) {
	v, ok := m[key]
	if !ok {
		return def, nil
	}
	n, isInt := v.Int64()
	if !isInt || n < lo || n > hi {
		return 0, fmt.Errorf("%s must be a whole number from %d to %d", key, lo, hi)
	}
	return n, nil
}

// fieldName reads a field name or field path from a value.
func fieldName(v jsonvalue.Value, what string) (string, error) {
	s, ok := v.Str()
	if !ok || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("%s must be a non-empty string", what)
	}
	if len(s) > maxFieldNameBytes {
		return "", fmt.Errorf("%s is longer than %d bytes", what, maxFieldNameBytes)
	}
	if strings.HasPrefix(s, expressionPrefix) {
		return "", fmt.Errorf("%s must be a literal field name, not an expression", what)
	}
	return s, nil
}

// lookup reads a field of an item. A name that is a key of the item is that
// key, dots included; otherwise a name with dots is a path through nested
// objects, the way a person writes "customer.address.city". An array on the
// path is not entered. A field that is not there is the absent value, which
// reports true from IsZero.
func lookup(item jsonvalue.Value, name string) jsonvalue.Value {
	if v, ok := item.Get(name); ok {
		return v
	}
	if !strings.Contains(name, ".") {
		return jsonvalue.Value{}
	}
	cur := item
	for _, part := range strings.Split(name, ".") {
		next, ok := cur.Get(part)
		if !ok {
			return jsonvalue.Value{}
		}
		cur = next
	}
	return cur
}

// templateOf is the expression template of a condition written as plain text
// or as a template: plain text is the code of one segment (ADR-0018 section 1).
func templateOf(s string) string {
	if strings.HasPrefix(s, expressionPrefix) {
		return s
	}
	return "={{ " + s + " }}"
}
