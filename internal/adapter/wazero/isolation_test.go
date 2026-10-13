// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func evalOne(t *testing.T, ev *expr.Evaluator, s *expr.Session, src string) (expr.Result, error) {
	t.Helper()
	p, err := ev.Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	return s.Eval(t.Context(), p, newEnv(t, `{"secret":"s3cr3t"}`))
}

// TestSandboxIsolation checks the isolation of ADR-0018 section 3: a memory
// bomb, an infinite loop, runaway recursion and attempts to reach the host
// fail only their own evaluation, inside the limits, and nothing survives
// between executions.
func TestSandboxIsolation(t *testing.T) {
	for _, mode := range []wazero.Mode{wazero.ModeHardLimit, wazero.ModeCooperativeLimit} {
		t.Run(mode.String(), func(t *testing.T) { sandboxIsolation(t, mode) })
	}
}

func sandboxIsolation(t *testing.T, mode wazero.Mode) {
	t.Helper()
	// A deadline case runs under a short limit. A case that must hit a memory
	// limit runs under a long one, so a slow machine does not stop it by time
	// before it fills memory.
	ev := evaluatorIn(t, mode, expr.Options{Timeout: 150 * time.Millisecond, MemoryLimit: 32 << 20})
	evMemory := evaluatorIn(t, mode, expr.Options{Timeout: time.Minute, MemoryLimit: 32 << 20})
	prime(t, ev)

	stops := []struct {
		name string
		src  string
		want error
		ev   *expr.Evaluator
	}{
		{"memory bomb by buffers", `={{ (() => { const a = []; for (;;) a.push(new Uint8Array(1 << 22)); })() }}`, expr.ErrMemory, evMemory},
		{"memory bomb by array buffers", `={{ (() => { const a = []; for (;;) a.push(new ArrayBuffer(1 << 24)); })() }}`, expr.ErrMemory, evMemory},
		{"memory bomb by typed array", `={{ new Uint8Array(1 << 30).length }}`, expr.ErrMemory, evMemory},
		{"infinite loop", `={{ (() => { for (;;) {} })() }}`, expr.ErrTimeout, ev},
		{"infinite loop in a callback", `={{ [1, 2, 3].map(() => { while (true) {} }) }}`, expr.ErrTimeout, ev},
		{"regular expression backtracking", `={{ /^(a+)+$/.test('a'.repeat(40) + 'b') }}`, expr.ErrTimeout, ev},
		{"deep recursion", `={{ (function f(n) { return f(n + 1) + 1; })(0) }}`, expr.ErrStack, evMemory},
		{"deep recursion in a callback", `={{ (() => { const f = (n) => [n].map((x) => f(x + 1)); return f(0); })() }}`, expr.ErrStack, evMemory},
		{"doubling a string", `={{ (() => { let s = 'x'; for (;;) s += s; })() }}`, nil, evMemory},
		{"deep recursion through JSON", `={{ (() => { let o = []; for (let i = 0; i < 1e5; i++) o = [o]; return JSON.stringify(o).length; })() }}`, nil, ev},
	}
	for _, tc := range stops {
		t.Run(tc.name, func(t *testing.T) {
			ev := tc.ev
			s := ev.Open()
			defer func() { _ = s.Close(t.Context()) }()
			start := time.Now()
			_, err := evalOne(t, ev, s, tc.src)
			took := time.Since(start)
			if tc.want == nil {
				// Whatever the outcome, it must be an ordinary failure or value, never a hang or a crash.
				if took > time.Minute {
					t.Errorf("took %v", took)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			// A deadline must stop the code soon after it passes; the time a memory
			// limit takes depends on how fast the machine fills memory (the race
			// detector slows that down a lot), so only deadlines are held to a bound.
			if errors.Is(tc.want, expr.ErrTimeout) && took > 20*time.Second {
				t.Errorf("the deadline was enforced after %v, too late", took)
			}
			// The session is usable afterwards: the failed runtime was discarded.
			res, err := evalOne(t, ev, s, `={{ 1 + 1 }}`)
			if err != nil {
				t.Fatalf("after the failure: %v", err)
			}
			if got := outcome(res, err); got != "2" {
				t.Errorf("after the failure = %s, want 2", got)
			}
		})
	}

	t.Run("no access to the host", func(t *testing.T) {
		s := ev.Open()
		defer func() { _ = s.Close(t.Context()) }()
		// Every name an escape would need is undefined.
		res, err := evalOne(t, ev, s, `={{ ['require', 'process', 'fetch', 'XMLHttpRequest', 'WebAssembly', 'std', 'os', 'Deno', 'Bun',
			'setTimeout', 'setInterval', 'import', 'module', 'exports', '__host', '__orcon', 'console', 'print', 'scriptArgs', 'luxon']
			.filter((n) => typeof globalThis[n] !== 'undefined') }}`)
		if err != nil {
			t.Fatal(err)
		}
		if got := outcome(res, nil); got != "[]" {
			t.Errorf("host names reachable from the sandbox: %s", got)
		}
		// There is no module loader: a dynamic import yields nothing to read.
		if res, err := evalOne(t, ev, s, `={{ import('fs') }}`); err == nil && outcome(res, nil) != "{}" {
			t.Errorf("dynamic import gave %s", outcome(res, nil))
		}
		res, err = evalOne(t, ev, s, `={{ (() => {}).constructor('return typeof require')() }}`)
		if err != nil || outcome(res, err) != `"undefined"` {
			t.Errorf("function constructor: got %s / %v, want undefined", outcome(res, err), err)
		}
		_, err = evalOne(t, ev, s, `={{ $env.HOME }}`)
		var ee *expr.EvalError
		if !errors.As(err, &ee) || ee.Message != "access to env vars denied" {
			t.Errorf("$env: error = %v, want access denied", err)
		}
	})

	t.Run("no state survives between evaluations or executions", func(t *testing.T) {
		a, b := ev.Open(), ev.Open()
		defer func() { _ = a.Close(t.Context()); _ = b.Close(t.Context()) }()
		// Attempts to leave state behind in session a. Each either throws or is ignored.
		for _, src := range []string{
			`={{ (globalThis.leak = 1, typeof leak) }}`,
			`={{ (() => { leaked = 2; return typeof leaked; })() }}`,
			`={{ (Object.prototype.polluted = true, ({}).polluted) }}`,
			`={{ (Array.prototype.polluted = true, [].polluted) }}`,
			`={{ (String.prototype.shout = function () { return this + '!'; }, typeof 'x'.shout) }}`,
			`={{ (Math.random = () => 0.5, Math.random()) }}`,
			`={{ (Date.now = () => 1, Date.now()) }}`,
			`={{ (JSON.stringify = () => 'hacked', JSON.stringify({})) }}`,
			`={{ (DateTime.now = () => null, 1) }}`,
			`={{ (DateTime.prototype.hacked = 1, typeof DateTime.now().hacked) }}`,
		} {
			_, _ = evalOne(t, ev, a, src) // the outcome does not matter; the leak check below does
		}
		for _, check := range []struct{ src, want string }{
			{`={{ typeof leak }}`, `"undefined"`},
			{`={{ typeof leaked }}`, `"undefined"`},
			{`={{ ({}).polluted }}`, `undefined`},
			{`={{ [].polluted }}`, `undefined`},
			{`={{ typeof 'x'.shout }}`, `"undefined"`},
			{`={{ JSON.stringify({}) }}`, `"{}"`},
			{`={{ DateTime.now().isValid }}`, `true`},
			{`={{ typeof DateTime.now().hacked }}`, `"undefined"`},
			{`={{ Math.random() === 0.5 }}`, `false`},
		} {
			for name, sess := range map[string]*expr.Session{"same session": a, "other execution": b} {
				res, err := evalOne(t, ev, sess, check.src)
				if err != nil || outcome(res, err) != check.want {
					t.Errorf("%s: %s = %s / %v, want %s", name, check.src, outcome(res, err), err, check.want)
				}
			}
		}
		// Data of one execution is not visible to another.
		res, err := b.Eval(t.Context(), mustCompile(t, ev, `={{ typeof globalThis.secret + typeof $json.secret }}`), newEnv(t, `{}`))
		if err != nil || outcome(res, err) != `"undefinedundefined"` {
			t.Errorf("cross-execution data = %s / %v", outcome(res, err), err)
		}
	})

	t.Run("one runtime per execution", func(t *testing.T) {
		before := ev.Stats().Opened
		a, b := ev.Open(), ev.Open()
		defer func() { _ = a.Close(t.Context()); _ = b.Close(t.Context()) }()
		for _, s := range []*expr.Session{a, b} {
			if _, err := evalOne(t, ev, s, `={{ [1].map(x => x) }}`); err != nil {
				t.Fatal(err)
			}
		}
		if got := ev.Stats().Opened - before; got != 2 {
			t.Errorf("two executions opened %d runtimes, want 2", got)
		}
	})
}

// prime compiles the module and builds the snapshot of the evaluator's sandbox
// outside the measured calls: the first open of a process is slow, and slower
// still under the race detector, which has nothing to do with the limits.
func prime(t *testing.T, ev *expr.Evaluator) {
	t.Helper()
	s := ev.Open()
	defer func() { _ = s.Close(t.Context()) }()
	if _, err := evalOne(t, ev, s, `={{ 1 }}`); err != nil {
		t.Fatal(err)
	}
}

func mustCompile(t *testing.T, ev *expr.Evaluator, src string) *expr.Program {
	t.Helper()
	p, err := ev.Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestSandboxContainsABadNeighbor is the shape of fault injection FI-14 at
// the scale of a unit test: among many concurrent executions one runs an
// infinite loop and one a memory bomb; only those two fail.
func TestSandboxContainsABadNeighbor(t *testing.T) {
	for _, mode := range []wazero.Mode{wazero.ModeHardLimit, wazero.ModeCooperativeLimit} {
		t.Run(mode.String(), func(t *testing.T) { badNeighbor(t, mode) })
	}
}

func badNeighbor(t *testing.T, mode wazero.Mode) {
	t.Helper()
	// Only the runaway loop is held to a short deadline; the other executions
	// and the memory bomb get a long one, so a slow machine fails none of them
	// by time.
	evLoop := evaluatorIn(t, mode, expr.Options{Timeout: 300 * time.Millisecond, MemoryLimit: 32 << 20})
	ev := evaluatorIn(t, mode, expr.Options{Timeout: time.Minute, MemoryLimit: 32 << 20})
	prime(t, ev)
	const executions = 24
	results := make([]error, executions)
	var wg sync.WaitGroup
	for i := range executions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ev := ev
			var src string
			switch i {
			case 3:
				ev = evLoop
				src = `={{ (() => { for (;;) {} })() }}`
			case 11:
				src = `={{ (() => { const a = []; for (;;) a.push(new Uint8Array(1 << 22)); })() }}`
			default:
				src = fmt.Sprintf(`={{ [1, 2, 3].map(x => x * %d).join('-') }}`, i)
			}
			s := ev.Open()
			defer func() { _ = s.Close(t.Context()) }()
			p, err := ev.Compile(src)
			if err != nil {
				results[i] = err
				return
			}
			env := newEnv(t, `{"n":2}`)
			_, results[i] = s.Eval(t.Context(), p, env)
		}()
	}
	wg.Wait()
	for i, err := range results {
		switch i {
		case 3:
			if !errors.Is(err, expr.ErrTimeout) {
				t.Errorf("execution %d (loop): %v, want a timeout", i, err)
			}
		case 11:
			if !errors.Is(err, expr.ErrMemory) {
				t.Errorf("execution %d (bomb): %v, want a memory failure", i, err)
			}
		default:
			if err != nil {
				t.Errorf("execution %d failed because of a neighbor: %v", i, err)
			}
		}
	}
	_ = jsonvalue.Null
}
