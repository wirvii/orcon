// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package id generates and formats the identity of a database row: a UUIDv7
// (RFC 9562) created by the application and passed to the database as a
// parameter (orcon-db ID-1). It is a platform package: it imports no area or
// adapter.
package id

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// UUID is a 128-bit identifier in RFC 9562 network byte order, so byte order
// equals time order for UUIDv7 values (orcon-db ID-2). The zero value is the
// nil UUID and never identifies a row.
type UUID [16]byte

// Nil is the zero UUID.
var Nil UUID

// IsZero reports whether u is the nil UUID.
func (u UUID) IsZero() bool { return u == Nil }

// Bytes returns a copy of the 16 bytes.
func (u UUID) Bytes() []byte {
	b := make([]byte, len(u))
	copy(b, u[:])
	return b
}

// String returns the lowercase hyphenated 36-character form (ID-3).
func (u UUID) String() string {
	var buf [36]byte
	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], u[10:])
	return string(buf[:])
}

// Version returns the UUID version nibble.
func (u UUID) Version() int { return int(u[6] >> 4) }

// ErrInvalid is wrapped by the errors of [Parse] and [FromBytes].
var ErrInvalid = errors.New("invalid UUID")

// Parse reads the hyphenated form produced by [UUID.String]. Upper case digits
// are accepted.
func Parse(s string) (UUID, error) {
	var u UUID
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return u, fmt.Errorf("parse %q: %w", s, ErrInvalid)
	}
	compact := s[0:8] + s[9:13] + s[14:18] + s[19:23] + s[24:]
	if _, err := hex.Decode(u[:], []byte(compact)); err != nil {
		return Nil, fmt.Errorf("parse %q: %w", s, ErrInvalid)
	}
	return u, nil
}

// FromBytes converts the 16 bytes a database returns.
func FromBytes(b []byte) (UUID, error) {
	var u UUID
	if len(b) != len(u) {
		return Nil, fmt.Errorf("%d bytes, want 16: %w", len(b), ErrInvalid)
	}
	copy(u[:], b)
	return u, nil
}

// Generator creates row identities. Production code uses [NewV7Generator];
// tests inject [NewSequence] so no test depends on the clock embedded in an
// identity (ID-4).
type Generator interface {
	New() UUID
}

// V7Generator creates UUIDv7 values: 48 bits of Unix milliseconds, a 12-bit
// counter and 62 random bits. The counter makes values from one generator
// strictly increasing even within one millisecond (RFC 9562 section 6.2,
// method 1).
type V7Generator struct {
	now func() time.Time

	mu       sync.Mutex
	lastMs   int64
	sequence uint16 // 12 bits in rand_a
}

// NewV7Generator returns a generator that reads the instant from now.
func NewV7Generator(now func() time.Time) *V7Generator {
	return &V7Generator{now: now}
}

// New returns a new UUIDv7. It panics only if the system random source fails,
// which the standard library treats as fatal as well.
func (g *V7Generator) New() UUID {
	g.mu.Lock()
	defer g.mu.Unlock()
	ms := g.now().UnixMilli()
	if ms <= g.lastMs {
		ms = g.lastMs
		g.sequence++
		if g.sequence > 0x0fff {
			// The counter overflowed inside one millisecond: move on to the next.
			ms++
			g.sequence = 0
		}
	} else {
		var seed [2]byte
		if _, err := rand.Read(seed[:]); err != nil {
			panic(fmt.Sprintf("id: system random source failed: %v", err))
		}
		// Start below the top of the range so the counter has room to grow.
		g.sequence = binary.BigEndian.Uint16(seed[:]) & 0x07ff
	}
	g.lastMs = ms
	var u UUID
	// ms is a positive Unix millisecond count, so the conversion is exact.
	binary.BigEndian.PutUint64(u[0:8], uint64(ms)<<16)
	binary.BigEndian.PutUint16(u[6:8], 0x7000|g.sequence)
	if _, err := rand.Read(u[8:]); err != nil {
		panic(fmt.Sprintf("id: system random source failed: %v", err))
	}
	u[8] = 0x80 | (u[8] & 0x3f)
	return u
}

// Sequence is a deterministic generator for tests: it returns UUIDv7-shaped
// values whose last eight bytes count up from one.
type Sequence struct {
	mu sync.Mutex
	n  uint64
}

// NewSequence returns a generator starting at 1.
func NewSequence() *Sequence { return &Sequence{} }

// New returns the next value.
func (s *Sequence) New() UUID {
	s.mu.Lock()
	s.n++
	n := s.n
	s.mu.Unlock()
	var u UUID
	u[6] = 0x70
	binary.BigEndian.PutUint64(u[8:], 0x8000000000000000|n)
	return u
}
