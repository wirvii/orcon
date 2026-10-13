// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package clock_test

import (
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/platform/clock"
)

func TestFake(t *testing.T) {
	start := time.Date(2026, 10, 13, 12, 0, 0, 0, time.FixedZone("UTC-5", -5*3600))
	f := clock.NewFake(start)
	if !f.Now().Equal(start) || f.Now().Location() != time.UTC {
		t.Fatalf("Now = %v", f.Now())
	}
	f.Advance(90 * time.Second)
	if want := start.Add(90 * time.Second); !f.Now().Equal(want) {
		t.Fatalf("after Advance: %v, want %v", f.Now(), want)
	}
	f.Set(start)
	if !f.Now().Equal(start) {
		t.Fatalf("after Set: %v", f.Now())
	}
}

func TestSystemAndMicros(t *testing.T) {
	before := time.Now()
	got := clock.System{}.Now()
	if got.Before(before.Add(-time.Second)) || got.Location() != time.UTC {
		t.Fatalf("System.Now = %v", got)
	}
	m := clock.Micros(time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.FixedZone("x", 3600)))
	if m.Nanosecond() != 123456000 || m.Location() != time.UTC || m.Hour() != 2 {
		t.Fatalf("Micros = %v", m)
	}
}
