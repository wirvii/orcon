// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package jsonvalue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

// MaxDepth is the deepest nesting of arrays and objects Parse accepts. It
// bounds the stack used on untrusted input.
const MaxDepth = 256

// Errors returned by [Parse].
var (
	// ErrSyntax marks input that is not a single well-formed JSON text.
	ErrSyntax = errors.New("jsonvalue: invalid JSON")
	// ErrDuplicateKey marks an object that repeats a key.
	ErrDuplicateKey = errors.New("jsonvalue: duplicate object key")
	// ErrTooDeep marks input nested deeper than [MaxDepth].
	ErrTooDeep = errors.New("jsonvalue: nesting too deep")
)

// Parse reads exactly one JSON text. It is stricter than encoding/json: the
// input must be valid UTF-8, an object may not repeat a key, nesting is
// bounded by [MaxDepth], nothing but whitespace may follow the value, and
// numbers keep their decimal text.
func Parse(data []byte) (Value, error) {
	if !utf8.Valid(data) {
		return Value{}, fmt.Errorf("%w: input is not valid UTF-8", ErrSyntax)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := parseValue(dec, 0)
	if err != nil {
		return Value{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Value{}, fmt.Errorf("%w: data after the top-level value", ErrSyntax)
	}
	return v, nil
}

func parseValue(dec *json.Decoder, depth int) (Value, error) {
	tok, err := dec.Token()
	if err != nil {
		return Value{}, fmt.Errorf("%w: %w", ErrSyntax, err)
	}
	switch t := tok.(type) {
	case json.Delim:
		if depth >= MaxDepth {
			return Value{}, ErrTooDeep
		}
		if t == '[' {
			return parseArray(dec, depth+1)
		}
		return parseObject(dec, depth+1)
	case string:
		return String(t), nil
	case json.Number:
		return Value{kind: KindNumber, text: t.String()}, nil
	case bool:
		return Bool(t), nil
	case nil:
		return Null(), nil
	default:
		return Value{}, fmt.Errorf("%w: unexpected token %v", ErrSyntax, tok)
	}
}

func parseArray(dec *json.Decoder, depth int) (Value, error) {
	items := []Value{}
	for dec.More() {
		item, err := parseValue(dec, depth)
		if err != nil {
			return Value{}, err
		}
		items = append(items, item)
	}
	if _, err := dec.Token(); err != nil { // the closing bracket
		return Value{}, fmt.Errorf("%w: %w", ErrSyntax, err)
	}
	return Value{kind: KindArray, items: items}, nil
}

func parseObject(dec *json.Decoder, depth int) (Value, error) {
	members := []Member{}
	seen := map[string]struct{}{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return Value{}, fmt.Errorf("%w: %w", ErrSyntax, err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return Value{}, fmt.Errorf("%w: object key is not a string", ErrSyntax)
		}
		if _, dup := seen[key]; dup {
			return Value{}, fmt.Errorf("%w: %q", ErrDuplicateKey, key)
		}
		seen[key] = struct{}{}
		val, err := parseValue(dec, depth)
		if err != nil {
			return Value{}, err
		}
		members = append(members, Member{Key: key, Value: val})
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return Value{}, fmt.Errorf("%w: %w", ErrSyntax, err)
	}
	return Value{kind: KindObject, members: members}, nil
}
