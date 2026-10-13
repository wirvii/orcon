// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
)

const (
	abandonDeadline = 50 * time.Millisecond
	// The wait past the deadline that a caller may still have (callerSlack).
	callerSlack = 5 * time.Millisecond
	// The bound on how long an abandoned call keeps running after the
	// caller stopped waiting (overshootBound): the time to the next poll of the interrupt handler
	// in a patched built-in. It is at most 40 ms on the development machine, and
	// the bound leaves room for a loaded runner.
	overshootBound = 100 * time.Millisecond
)

// overshootReporter is implemented by the runtimes of ModeCooperativeAbandon.
type overshootReporter interface {
	AbandonedOvershoot(ctx context.Context) (time.Duration, bool)
}

// abandonOutcome is what one abandoned call looked like.
type abandonOutcome struct {
	err       error
	took      time.Duration
	overshoot time.Duration
	abandoned bool
}

// driveToDeadline runs src on a fresh runtime of the sandbox, past the
// deadline, and reports how the caller and the abandoned call ended.
func driveToDeadline(tb testing.TB, sb *wazero.Sandbox, cfg expr.SandboxConfig, src string) abandonOutcome {
	tb.Helper()
	rt, err := sb.Open(tb.Context(), cfg)
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = rt.Close(context.WithoutCancel(tb.Context())) }()
	call := func(ctx context.Context, code string) error {
		req := `{"e":` + quote(code) + `,"m":"v","i":0,"r":0,"n":1791913530250,"z":"UTC"}` + "\x01\x01\x01\x01"
		_, err := rt.Call(ctx, []byte(req), &countingGuest{ops: map[int32]int{}})
		return err
	}
	if err := call(tb.Context(), "1"); err != nil {
		tb.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(tb.Context(), abandonDeadline)
	defer cancel()
	start := time.Now()
	out := abandonOutcome{err: call(ctx, src)}
	out.took = time.Since(start)
	rep, ok := rt.(overshootReporter)
	if !ok {
		tb.Fatal("the runtime does not report the overshoot of an abandoned call")
	}
	out.overshoot, out.abandoned = rep.AbandonedOvershoot(tb.Context())
	// A runtime that lost a call is never reused.
	if out.abandoned && call(tb.Context(), "1") == nil {
		tb.Error("an abandoned runtime answered a later call")
	}
	return out
}

// abandonCases are the built-ins the interrupt-poll patch covers, each with an
// input that would run for seconds unpatched, plus a bytecode loop.
var abandonCases = []struct{ name, src string }{
	{"indexOf", `'a'.repeat(4e6).indexOf('a'.repeat(2000) + 'b')`},
	{"lastIndexOf", `'a'.repeat(4e6).lastIndexOf('b' + 'a'.repeat(2000))`},
	{"includes", `'a'.repeat(4e6).includes('a'.repeat(2000) + 'b')`},
	{"replaceAll", `'a'.repeat(4e6).replaceAll('a', 'bb').length`},
	{"replace with a pattern", `'a'.repeat(4e6).replace(/a/g, 'bb').length`},
	{"split", `'a'.repeat(4e6).split('').length`},
	{"repeat", `'ab'.repeat(8e6).length`},
	{"padStart", `''.padStart(2e7, 'ab').length`},
	{"JSON.parse", `JSON.parse('[' + '1,'.repeat(2e6) + '1]').length`},
	{"JSON.stringify", `JSON.stringify(new Array(2e6).fill(1)).length`},
	{"JSON.stringify nested", `(() => { let o = []; for (let i = 0; i < 1e5; i++) o = [o]; return JSON.stringify(o).length; })()`},
	{"join", `new Array(3e6).fill('a').join(',').length`},
	{"concat", `(() => { let a = new Array(1e6).fill(1); for (let i = 0; i < 6; i++) a = a.concat(a); return a.length; })()`},
	{"flat", `new Array(2e6).fill([1, 2]).flat().length`},
	{"toUpperCase", `'é'.repeat(5e6).toUpperCase().length`},
	{"encodeURIComponent", `encodeURIComponent('é'.repeat(3e6)).length`},
	{"loop", `(() => { for (;;) {} })()`},
}

// TestSandboxIsolationAbandon checks, for each built-in the interrupt-poll
// patch covers, that a call made to run long ends for the caller at the
// deadline (A.4 (a)) and that the abandoned call itself stops soon after
// (A.4 (b)), then closes its runtime.
func TestSandboxIsolationAbandon(t *testing.T) {
	ev := evaluatorIn(t, wazero.ModeCooperativeAbandon, expr.Options{Timeout: abandonDeadline, MemoryLimit: 64 << 20})
	sb := sharedSandbox(wazero.ModeCooperativeAbandon)
	strict := defaultTestTimeout < time.Second // not under the race detector
	for _, tc := range abandonCases {
		t.Run(tc.name, func(t *testing.T) {
			out := driveToDeadline(t, sb, ev.SandboxConfig(), tc.src)
			if !errors.Is(out.err, expr.ErrTimeout) {
				t.Fatalf("got %v after %v, want ErrTimeout", out.err, out.took)
			}
			if limit := abandonDeadline + callerSlack; out.took > limit {
				if strict {
					t.Errorf("the caller waited %v, more than the deadline plus %v", out.took, callerSlack)
				} else {
					t.Logf("the caller waited %v (not enforced under the race detector)", out.took)
				}
			}
			if out.abandoned && out.overshoot > overshootBound && strict {
				t.Errorf("the abandoned call ran %v after the caller stopped waiting, more than %v", out.overshoot, overshootBound)
			}
			t.Logf("caller %v after start, abandoned=%v, call ran %v more", out.took.Round(time.Millisecond), out.abandoned, out.overshoot.Round(time.Millisecond))
		})
	}
}
