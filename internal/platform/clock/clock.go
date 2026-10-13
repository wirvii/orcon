// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package clock supplies the instants the application writes to the database.
// The application, not the database, decides every stored instant
// (orcon-db TY-5), so tests can drive time. It is a platform package: it
// imports no area or adapter.
package clock

import (
	"sync"
	"time"
)

// Clock reports the current instant.
type Clock interface {
	Now() time.Time
}

// System is the wall clock.
type System struct{}

// Now returns the current time in UTC.
func (System) Now() time.Time { return time.Now().UTC() }

// Fake is a clock tests move by hand.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake returns a fake clock set to start.
func NewFake(start time.Time) *Fake { return &Fake{now: start.UTC()} }

// Now returns the current fake instant.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the clock forward by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
}

// Set moves the clock to t.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	f.now = t.UTC()
	f.mu.Unlock()
}

// Micros truncates t to microseconds in UTC, the precision both stores keep
// (orcon-db TY-4).
func Micros(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }
