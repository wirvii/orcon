// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"context"
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
	// A call still running this long after it was abandoned is a hang: the test
	// stops waiting for it and fails on the bound.
	hangLimit = 20 * time.Second
)

// stretchReporter is implemented by the runtimes of this adapter.
type stretchReporter interface {
	Stretch() time.Duration
	Tail() time.Duration
	First() time.Duration
}

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
	// stretch is the longest time the guest went without calling the host.
	stretch, tail, first time.Duration
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
	{"join", `new Array(2e6).fill('a').join(',').length`},
	{"concat", `(() => { let a = new Array(1e6).fill(1); for (let i = 0; i < 6; i++) a = a.concat(a); return a.length; })()`},
	{"flat", `new Array(2e6).fill([1, 2]).flat().length`},
	{"toUpperCase", `'é'.repeat(5e6).toUpperCase().length`},
	{"encodeURIComponent", `encodeURIComponent('é'.repeat(3e6)).length`},
	{"loop", `(() => { for (;;) {} })()`},

	// The cases measured in BL-030 (SPEC-001 amendment 10, section C).
	{"unshift in a loop", `(() => { const a = new Array(3e6).fill(1); for (let i = 0; i < 3e3; i++) a.unshift(1); return 1; })()`},
	{"string comparison in a loop", `(() => { const a = 'a'.repeat(1e7), b = 'a'.repeat(1e7); let n = 0; for (let i = 0; i < 500; i++) if (a < b) n++; return n; })()`},
	{"copyWithin in a loop", `(() => { const a = new Array(3e6).fill(1); for (let i = 0; i < 3e3; i++) a.copyWithin(0, 1); return 1; })()`},
	{"array spread in a loop", `(() => { const a = new Array(2e6).fill(1); let n = 0; for (let i = 0; i < 30; i++) n += [...a].length; return n; })()`},
	{"Object.keys in a loop", `(() => { const o = {}; for (let i = 0; i < 1e5; i++) o['k' + i] = i; let n = 0; for (let i = 0; i < 500; i++) n += Object.keys(o).length; return n; })()`},
	{"trim", `' '.repeat(2e7).trim().length`},
	{"escape", `escape(' '.repeat(5e6)).length`},
	{"Array.from", `Array.from({ length: 5e6 }).length`},
	{"BigInt multiplication", `(() => { const x = (1n << 520000n) - 1n; $input.all(); return x * x; })()`},
	{"BigInt multiplication in a loop", `(() => { const x = (1n << 520000n) - 1n; $input.all(); for (;;) x * x; })()`},
	{"BigInt division in a loop", `(() => { const x = (1n << 1000000n) - 1n, y = (1n << 400000n) + 1n; $input.all(); for (;;) x / y; })()`},
	{"includes inside filter", `(() => { const ids = Array.from({ length: 1e5 }, (_, i) => i), seen = Array.from({ length: 1e5 }, (_, i) => i + 1e5); return ids.filter((id) => !seen.includes(id)).length; })()`},
}

// TestSandboxIsolationAbandon checks, for each built-in the interrupt-poll
// patch covers, that a call made to run long ends for the caller at the
// deadline (A.4 (a)) and that the abandoned call itself stops soon after
// (A.4 (b)), then closes its runtime.
func TestSandboxIsolationAbandon(t *testing.T) {
	ev := evaluatorIn(t, wazero.ModeCooperativeAbandon, expr.Options{Timeout: abandonDeadline, MemoryLimit: 64 << 20})
	sb := sharedSandbox(wazero.ModeCooperativeAbandon)
	for _, tc := range abandonCases {
		t.Run(tc.name, func(t *testing.T) {
			runAbandonCase(t, sb, ev.SandboxConfig(), tc.src, false, true)
		})
	}
}
