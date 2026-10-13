// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package jsonvalue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
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
	// ErrLoneSurrogate marks a string with a \\uD800-\\uDFFF escape that is not
	// half of a valid surrogate pair (RFC 8785 and RFC 7493 forbid it).
	ErrLoneSurrogate = errors.New("jsonvalue: lone surrogate escape")
)

// Parse reads exactly one JSON text. It is stricter than encoding/json: the
// input must be valid UTF-8, an object may not repeat a key, nesting is
// bounded by [MaxDepth], nothing but whitespace may follow the value, and
// numbers keep their decimal text.
func Parse(data []byte) (Value, error) {
	if !utf8.Valid(data) {
		return Value{}, fmt.Errorf("%w: input is not valid UTF-8", ErrSyntax)
	}
	if err := checkSurrogates(data); err != nil {
		return Value{}, err
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

// checkSurrogates rejects unpaired \\u surrogate escapes inside strings.
// The standard decoder would turn each into U+FFFD, so "a\\ud800b" and "a<U+FFFD>b"
// would parse to the same string and share a canonical text and a hash.
func checkSurrogates(data []byte) error {
	inString := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		switch {
		case !inString:
			inString = c == '"'
		case c == '"':
			inString = false
		case c == '\\' && i+1 < len(data):
			if data[i+1] != 'u' {
				i++ // skip the escaped character
				continue
			}
			unit, ok := hex4(data, i+2)
			if !ok {
				return nil // malformed escape: the decoder reports it
			}
			i += 5
			switch {
			case unit >= 0xDC00 && unit <= 0xDFFF:
				return fmt.Errorf("%w: low surrogate without a high surrogate", ErrLoneSurrogate)
			case unit >= 0xD800 && unit <= 0xDBFF:
				next, paired := uint16(0), false
				if i+6 < len(data) && data[i+1] == '\\' && data[i+2] == 'u' {
					next, paired = hex4(data, i+3)
				}
				if !paired || next < 0xDC00 || next > 0xDFFF {
					return fmt.Errorf("%w: high surrogate without a low surrogate", ErrLoneSurrogate)
				}
				i += 6
			}
		}
	}
	return nil
}

func hex4(data []byte, at int) (uint16, bool) {
	if at+4 > len(data) {
		return 0, false
	}
	n, err := strconv.ParseUint(string(data[at:at+4]), 16, 16)
	return uint16(n), err == nil
}
