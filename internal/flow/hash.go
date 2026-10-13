// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Hash is the content hash of a flow document: SHA-256 over the JSON
// Canonicalization Scheme text (RFC 8785) of the semantic document, which is
// the document without its "editor" member (ADR-0007 section 6).
type Hash [sha256.Size]byte

// String returns the lowercase hexadecimal form of the hash.
func (h Hash) String() string { return hex.EncodeToString(h[:]) }

// ParseHash reads the hexadecimal form produced by [Hash.String].
func ParseHash(s string) (Hash, error) {
	var h Hash
	raw, err := hex.DecodeString(s)
	if err != nil || len(raw) != len(h) {
		return h, fmt.Errorf("parse flow content hash %q: want %d hexadecimal digits", s, 2*len(h))
	}
	copy(h[:], raw)
	return h, nil
}

// ContentHash returns the content hash of a document. Two documents that
// differ only in their editor data have the same hash.
func ContentHash(d *Document) (Hash, error) {
	return ContentHashOfValue(d.Value())
}

// ContentHashOfValue returns the content hash of a document in JSON form. It
// is what [ContentHash] computes, for callers that hold the JSON value.
func ContentHashOfValue(v jsonvalue.Value) (Hash, error) {
	canonical, err := jsonvalue.Canonical(v.Without("editor"))
	if err != nil {
		return Hash{}, fmt.Errorf("canonicalize flow document: %w", err)
	}
	return sha256.Sum256(canonical), nil
}
