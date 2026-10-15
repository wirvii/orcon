// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package page holds the pieces of cursor pagination (ADR-0053 sections 1 and 2):
// the opaque, authenticated page token, the limit, and the sort expression
// checked against the operation's allowlist.
//
// A page token is keyset based: it carries the position after the last item the
// client saw, so concurrent inserts never duplicate or skip an item that existed
// when the walk began. It is bound to the organization, the filters and the sort
// it was issued for; using it with anything else is 400 invalid-page-token.
package page

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Limits of the limit parameter (PAGE-1).
const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// MinKeyLength is the shortest key a [Codec] accepts, in bytes.
const MinKeyLength = 32

// Errors of the package.
var (
	// ErrInvalidToken means the token is malformed, forged, or was issued for
	// another organization, filter set or sort.
	ErrInvalidToken = errors.New("invalid page token")
	// ErrInvalidLimit means limit is not an integer from 1 to 200.
	ErrInvalidLimit = errors.New("invalid limit")
	// ErrInvalidSort means the sort expression names a field the operation does not allow.
	ErrInvalidSort = errors.New("invalid sort")
	// ErrKeyTooShort means the token key is shorter than [MinKeyLength].
	ErrKeyTooShort = errors.New("the page token key is too short")
)

// Cursor is the position after the last item of a page.
type Cursor struct {
	// Position holds the sort key values of the last item and, last, its id; the
	// next page starts strictly after it in the total order.
	Position []string `json:"p"`
}

// Codec issues and checks page tokens. The key is derived from the instance key
// (ADR-0042); a token does not survive a change of key.
type Codec struct {
	key []byte
}

// NewCodec returns a codec for key, which must be at least [MinKeyLength] bytes.
func NewCodec(key []byte) (*Codec, error) {
	if len(key) < MinKeyLength {
		return nil, ErrKeyTooShort
	}
	return &Codec{key: append([]byte(nil), key...)}, nil
}

// payload is the signed content of a token.
type payload struct {
	Org     string   `json:"o"`
	Binding string   `json:"b"`
	Cursor  []string `json:"p"`
}

// Binding fingerprints the parameters a token is bound to: the filters and the
// sort expression, in a canonical order. Pass name=value pairs of every filter
// that was sent and the sort.
func Binding(pairs ...string) string {
	h := sha256.New()
	for _, p := range pairs {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// Encode returns the opaque token for the position after the last item of a
// page, bound to org and binding.
func (c *Codec) Encode(org, binding string, cur Cursor) (string, error) {
	body, err := json.Marshal(payload{Org: org, Binding: binding, Cursor: cur.Position})
	if err != nil {
		return "", fmt.Errorf("encode the page token: %w", err)
	}
	mac := hmac.New(sha256.New, c.key)
	mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// Decode checks a token and returns its cursor. It fails with
// [ErrInvalidToken] when the token is malformed, was not signed by this key, or
// was issued for another organization or binding.
func (c *Codec) Decode(token, org, binding string) (Cursor, error) {
	encodedBody, encodedMAC, ok := strings.Cut(token, ".")
	if !ok {
		return Cursor{}, ErrInvalidToken
	}
	body, err := base64.RawURLEncoding.DecodeString(encodedBody)
	if err != nil {
		return Cursor{}, ErrInvalidToken
	}
	got, err := base64.RawURLEncoding.DecodeString(encodedMAC)
	if err != nil {
		return Cursor{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, c.key)
	mac.Write(body)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return Cursor{}, ErrInvalidToken
	}
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Cursor{}, ErrInvalidToken
	}
	if p.Org != org || p.Binding != binding || len(p.Cursor) == 0 {
		return Cursor{}, ErrInvalidToken
	}
	return Cursor{Position: p.Cursor}, nil
}

// Limit reads the limit query parameter: empty means [DefaultLimit]; anything
// else must be an integer from 1 to [MaxLimit].
func Limit(raw string) (int, error) {
	if raw == "" {
		return DefaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > MaxLimit {
		return 0, ErrInvalidLimit
	}
	return n, nil
}

// Term is one field of a sort expression.
type Term struct {
	// Field is the name of the field.
	Field string
	// Descending is true for a term written with a leading minus.
	Descending bool
}

// ParseSort reads a sort expression such as "name,-updatedAt". Every term must
// be an allowed field (the field name, without the minus); an empty expression
// is the default expression. The order is always made total by the caller with
// the id as the last key.
func ParseSort(raw string, allowed []string, def string) ([]Term, error) {
	if raw == "" {
		raw = def
	}
	allow := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allow[strings.TrimPrefix(a, "-")] = true
	}
	var terms []Term
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		desc := strings.HasPrefix(part, "-")
		field := strings.TrimPrefix(part, "-")
		if field == "" || !allow[field] || seen[field] {
			return nil, fmt.Errorf("%w: %q", ErrInvalidSort, part)
		}
		seen[field] = true
		terms = append(terms, Term{Field: field, Descending: desc})
	}
	return terms, nil
}

// String returns the canonical form of terms, the value to bind a token to.
func String(terms []Term) string {
	parts := make([]string, len(terms))
	for i, t := range terms {
		parts[i] = t.Field
		if t.Descending {
			parts[i] = "-" + t.Field
		}
	}
	return strings.Join(parts, ",")
}
