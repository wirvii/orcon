// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
)

// countingGuest counts the host operations a call makes.
type countingGuest struct {
	mu  sync.Mutex
	ops map[int32]int
}

func (g *countingGuest) HostCall(context.Context, int32, []byte) ([]byte, error) { return nil, nil }
func (g *countingGuest) Now() time.Time                                          { return time.Unix(1791913530, 0).UTC() }
func (g *countingGuest) Read(p []byte)                                           { clear(p) }

type opCounter struct {
	countingGuest
}

func (g *opCounter) HostCall(ctx context.Context, op int32, payload []byte) ([]byte, error) {
	g.mu.Lock()
	g.ops[op]++
	g.mu.Unlock()
	return (&countingGuest{}).HostCall(ctx, op, payload)
}

// TestDateCasesMakeFewHostCalls records the profile behind SPEC-001 amendment
// 10, A.3: in the date cases of the report the calls to the host for zone
// conversions and hashes are not what the time goes to. A date in UTC makes
// none; a date in a named zone makes a few per distinct instant, and the
// snapshot already holds the zones the warm-up touched.
func TestDateCasesMakeFewHostCalls(t *testing.T) {
	ev := evaluatorIn(t, wazero.ModeCooperativeLimit, expr.Options{})
	rt, err := sharedSandbox(wazero.ModeCooperativeLimit).Open(t.Context(), ev.SandboxConfig())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Close(context.WithoutCancel(t.Context())) })
	call := func(code, zone string) map[int32]int {
		g := &opCounter{countingGuest: countingGuest{ops: map[int32]int{}}}
		req := `{"e":` + quote(code) + `,"m":"v","i":0,"r":0,"n":1791913530250,"z":"` + zone + `"}` + "\x01\x01\x01\x01"
		if _, err := rt.Call(t.Context(), []byte(req), g); err != nil {
			t.Fatal(err)
		}
		return g.ops
	}
	for _, c := range []string{
		"$now.toFormat('yyyy-MM-dd')", "$now.plus({ days: 7 }).toISO()", "DateTime.fromISO('2026-03-14T10:30:00Z').toFormat('dd/MM/yyyy')",
	} {
		call(c, "UTC") // the first call of a runtime
		ops := call(c, "UTC")
		if ops[expr.OpZoneFields] != 0 || ops[expr.OpHash] != 0 {
			t.Errorf("%s in UTC made host calls %v, want none", c, ops)
		}
	}
	zoned := call("$now.plus({ days: 7 }).toFormat('yyyy-MM-dd HH:mm')", "America/Bogota")
	again := call("$now.plus({ days: 7 }).toFormat('yyyy-MM-dd HH:mm')", "America/Bogota")
	t.Logf("a date operation in America/Bogota makes %d zone conversions the first time and %d afterwards", zoned[expr.OpZoneFields], again[expr.OpZoneFields])
	if again[expr.OpZoneFields] > 4 {
		t.Errorf("%d zone conversions for one date operation; cache them", again[expr.OpZoneFields])
	}
}

func quote(s string) string {
	b := []byte{'"'}
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b = append(b, '\\')
		case r < 0x20:
			b = append(b, fmt.Sprintf("\\u%04x", r)...)
			continue
		}
		b = append(b, string(r)...)
	}
	return string(append(b, '"'))
}
