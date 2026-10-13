// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package id_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/platform/id"
)

func TestV7ShapeAndOrder(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	g := id.NewV7Generator(func() time.Time { return now })
	var prev id.UUID
	for i := range 10000 {
		u := g.New()
		if u.Version() != 7 || u[8]&0xc0 != 0x80 {
			t.Fatalf("%s: version %d, variant byte %#x", u, u.Version(), u[8])
		}
		if i > 0 && bytes.Compare(prev[:], u[:]) >= 0 {
			t.Fatalf("%s is not after %s although the clock did not move", u, prev)
		}
		prev = u
	}
	// The embedded time is the generator's clock, in milliseconds.
	ms := int64(prev[0])<<40 | int64(prev[1])<<32 | int64(prev[2])<<24 | int64(prev[3])<<16 | int64(prev[4])<<8 | int64(prev[5])
	if ms < now.UnixMilli() {
		t.Fatalf("embedded time %d is before the clock %d", ms, now.UnixMilli())
	}
	// A clock that moves back never moves the identities back.
	now = now.Add(-time.Hour)
	if u := g.New(); bytes.Compare(prev[:], u[:]) >= 0 {
		t.Fatalf("%s is not after %s after the clock went back", u, prev)
	}
}

func TestStringParseRoundTrip(t *testing.T) {
	u := id.NewV7Generator(time.Now).New()
	text := u.String()
	if len(text) != 36 || text != string(bytes.ToLower([]byte(text))) {
		t.Fatalf("text form %q", text)
	}
	back, err := id.Parse(text)
	if err != nil || back != u {
		t.Fatalf("parse %q = %v, %v", text, back, err)
	}
	if upper, err := id.Parse(string(bytes.ToUpper([]byte(text)))); err != nil || upper != u {
		t.Fatalf("upper case form: %v, %v", upper, err)
	}
	fromBytes, err := id.FromBytes(u.Bytes())
	if err != nil || fromBytes != u {
		t.Fatalf("bytes round trip: %v, %v", fromBytes, err)
	}
}

func TestInvalid(t *testing.T) {
	for _, s := range []string{"", "x", "00000000-0000-0000-0000-00000000000", "00000000_0000-0000-0000-000000000000", "0000000g-0000-0000-0000-000000000000"} {
		if _, err := id.Parse(s); !errors.Is(err, id.ErrInvalid) {
			t.Errorf("Parse(%q) error %v", s, err)
		}
	}
	if _, err := id.FromBytes(make([]byte, 15)); !errors.Is(err, id.ErrInvalid) {
		t.Error("FromBytes accepted 15 bytes")
	}
	if !id.Nil.IsZero() || id.NewSequence().New().IsZero() {
		t.Error("IsZero")
	}
}

func TestSequenceIsDeterministicAndOrdered(t *testing.T) {
	a, b := id.NewSequence(), id.NewSequence()
	var prev id.UUID
	for range 100 {
		x, y := a.New(), b.New()
		if x != y {
			t.Fatalf("two sequences diverged: %s and %s", x, y)
		}
		if bytes.Compare(prev[:], x[:]) >= 0 {
			t.Fatalf("%s is not after %s", x, prev)
		}
		prev = x
	}
	if prev.Version() != 7 {
		t.Fatalf("version %d", prev.Version())
	}
}
