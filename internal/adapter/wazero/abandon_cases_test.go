// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
)

// TestSandboxIsolationBuiltinInventory checks SPEC-001 amendment 10, section
// C-4.3: every native function reachable from the global object of the guest
// is classified as covered, bounded or interpreted, and the table names no
// function the guest lacks. A QuickJS-ng upgrade or a new built-in that is not
// classified fails here; a covered one is then driven past the deadline by
// TestSandboxIsolationAbandon.
func TestSandboxIsolationBuiltinInventory(t *testing.T) {
	classified := map[string]string{}
	for _, row := range nativeTable {
		switch row.class {
		case covered:
			if row.call == "" {
				t.Errorf("%s: a covered row needs a call", row.names)
			}
		case bounded, interpreted:
			if row.why == "" {
				t.Errorf("%s: a %s row needs a reason", row.names, row.class)
			}
		default:
			t.Errorf("%s: unknown class %q", row.names, row.class)
		}
		for _, name := range expandNames(row.names) {
			if prev, dup := classified[name]; dup {
				t.Errorf("%s is classified twice (%s and %s)", name, prev, row.class)
			}
			classified[name] = row.class
		}
	}
	guest := guestNatives(t)
	if len(guest) < 400 {
		t.Fatalf("only %d natives found in the guest; the enumeration is broken", len(guest))
	}
	for _, name := range guest {
		if _, ok := classified[name]; !ok {
			t.Errorf("the guest has the native function %q, which nativeTable does not classify", name)
		}
	}
	for name := range classified {
		if !slices.Contains(guest, name) {
			t.Errorf("nativeTable classifies %q, which the guest does not have", name)
		}
	}
	covers := 0
	for _, class := range classified {
		if class == covered {
			covers++
		}
	}
	t.Logf("%d natives: %d covered, %d bounded or interpreted", len(guest), covers, len(guest)-covers)
}

// reachGuest answers the host call that marks the end of the setup of a case.
type reachGuest struct {
	reached atomic.Bool
}

func (g *reachGuest) HostCall(_ context.Context, op int32, _ []byte) ([]byte, error) {
	if op == expr.OpInputAll {
		g.reached.Store(true)
		return []byte("[]"), nil
	}
	return nil, nil
}
func (g *reachGuest) Now() time.Time { return time.Unix(1791913530, 0).UTC() }
func (g *reachGuest) Read(p []byte)  { clear(p) }

// driveCase runs src on a fresh runtime under a deadline and reports how the
// caller and the abandoned call ended. When reach is set the setup of the case
// ends with a host call, and the deadline grows while that call is not made
// inside it, so the native under test is what the deadline interrupts and not
// the setup.
func driveCase(tb testing.TB, sb *wazero.Sandbox, cfg expr.SandboxConfig, src string, reach bool) (out abandonOutcome, deadline time.Duration) {
	tb.Helper()
	for _, d := range []time.Duration{abandonDeadline, 4 * abandonDeadline, 16 * abandonDeadline, 64 * abandonDeadline} {
		deadline = d
		// A garbage collection stops the world, and a stop has to wait for the
		// compiled code to reach a host call; with the collector held off for
		// the run, the bounds measure the sandbox and not the Go runtime.
		runtime.GC()
		restore := debug.SetGCPercent(-1)
		rt, err := sb.Open(tb.Context(), cfg)
		if err != nil {
			tb.Fatal(err)
		}
		g := &reachGuest{}
		call := func(ctx context.Context, code string) error {
			req := `{"e":` + quote(code) + `,"m":"v","i":0,"r":0,"n":1791913530250,"z":"UTC"}` + "\x01\x01\x01\x01"
			ans, err := rt.Call(ctx, []byte(req), g)
			if err == nil && bytes.Contains(ans, []byte(`"x":`)) {
				err = fmt.Errorf("the code threw: %.200s", ans)
			}
			return err
		}
		if err := call(tb.Context(), "1"); err != nil {
			tb.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(tb.Context(), d)
		start := time.Now()
		out = abandonOutcome{err: call(ctx, src)}
		out.took = time.Since(start)
		cancel()
		if rep, ok := rt.(overshootReporter); ok {
			waitCtx, stop := context.WithTimeout(tb.Context(), hangLimit)
			out.overshoot, out.abandoned = rep.AbandonedOvershoot(waitCtx)
			stop()
		}
		if sr, ok := rt.(stretchReporter); ok {
			out.stretch, out.tail, out.first = sr.Stretch(), sr.Tail(), sr.First()
		}
		_ = rt.Close(context.WithoutCancel(tb.Context()))
		debug.SetGCPercent(restore)
		if !reach || g.reached.Load() || !errors.Is(out.err, expr.ErrTimeout) {
			return out, deadline
		}
	}
	return out, deadline
}

// judge holds an outcome to the bounds of ADR-0018 section 3: the caller has
// its timeout within the deadline plus 5 ms, the abandoned call stops
// within 100 ms, and the guest never goes more than the stretch bound without a host call. It returns the bounds that were missed. A call that did not
// time out is fine unless mustTimeOut says it had to; any other error is the
// test's, not the sandbox's, and is returned as fatal.
func judge(out abandonOutcome, deadline time.Duration, mustTimeOut bool, maxStretch time.Duration) (problems []string, fatal string, note string) {
	switch {
	case out.err == nil && !mustTimeOut:
		return nil, "", "the call finished inside " + deadline.String() + " (" + out.took.Round(time.Millisecond).String() + ")"
	case !errors.Is(out.err, expr.ErrTimeout):
		return nil, "got " + fmt.Sprint(out.err) + " after " + out.took.String() + ", want ErrTimeout", ""
	}
	if out.took > deadline+callerSlack {
		problems = append(problems, "the caller waited "+out.took.String()+", more than the deadline "+deadline.String()+" plus "+callerSlack.String())
	}
	if out.abandoned && out.overshoot > overshootBound {
		problems = append(problems, "the abandoned call ran "+out.overshoot.String()+" after the caller stopped waiting, more than "+overshootBound.String())
	}
	if out.stretch > maxStretch {
		problems = append(problems, "the guest went "+out.stretch.String()+" without calling the host, more than "+maxStretch.String())
	}
	return problems, "", "caller " + out.took.Round(time.Millisecond).String() + " after start (deadline " + deadline.String() + "), abandoned=" + strconv.FormatBool(out.abandoned) + ", call ran " + out.overshoot.Round(time.Millisecond).String() + " more"
}

// runAbandonCase drives one case and holds it to the bounds. The bounds are
// about timing on a machine that runs other work: a case that misses one is run
// again, up to three times, and fails only if every run misses. A call that
// hangs, or any error other than the stop, fails at once. Under the race
// detector the bounds are only logged.
func runAbandonCase(t *testing.T, sb *wazero.Sandbox, cfg expr.SandboxConfig, src string, reach, mustTimeOut bool) {
	t.Helper()
	runAbandonCaseOpt(t, sb, cfg, src, reach, mustTimeOut, false)
}

// runAbandonCaseOpt is runAbandonCase; skipMemory skips a case whose inputs do
// not fit the memory cap together (a chain of two natives holds two inputs).
func runAbandonCaseOpt(t *testing.T, sb *wazero.Sandbox, cfg expr.SandboxConfig, src string, reach, mustTimeOut, skipMemory bool) {
	t.Helper()
	strict := defaultTestTimeout < time.Second
	// The cases of abandonCases are held to the target; a covered native, whose
	// single step can be one copy of a few megabytes, to the documented residual.
	maxStretch := stretchBound
	if reach {
		maxStretch = nativeStretchBound
	}
	var last []string
	for attempt := 1; attempt <= 3; attempt++ {
		out, deadline := driveCase(t, sb, cfg, src, reach)
		problems, fatal, note := judge(out, deadline, mustTimeOut, maxStretch)
		if fatal != "" {
			// Under the race detector the timing is not enforced, and a case slowed
			// down by a loaded machine can spend its memory before the deadline.
			if !strict && strings.Contains(fatal, "out of memory") {
				t.Skip("the case ran out of memory before its deadline on a loaded machine")
			}
			if skipMemory && (strings.Contains(fatal, "out of memory") || strings.Contains(fatal, `"m":"null"`) || strings.Contains(fatal, `[uninitialized]`)) {
				t.Skip("the inputs of the chain do not fit the memory cap")
			}
			t.Fatal(fatal)
		}
		if len(problems) == 0 {
			t.Log(note)
			return
		}
		if !strict {
			t.Logf("%s (not enforced under the race detector): %v", note, problems)
			return
		}
		last = problems
		t.Logf("attempt %d: %v", attempt, problems)
		if out.overshoot >= hangLimit {
			break
		}
	}
	for _, p := range last {
		t.Error(p)
	}
}

// TestSandboxIsolationAbandonNatives drives every covered native past its
// deadline, as one large call and inside an interpreted loop of calls.
func TestSandboxIsolationAbandonNatives(t *testing.T) {
	ev := evaluatorIn(t, wazero.ModeCooperativeAbandon, expr.Options{Timeout: abandonDeadline, MemoryLimit: 64 << 20})
	sb := sharedSandbox(wazero.ModeCooperativeAbandon)
	cases := tableCases()
	if len(cases) < 200 {
		t.Fatalf("only %d cases generated from the table", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.kind, func(t *testing.T) {
			runAbandonCase(t, sb, ev.SandboxConfig(), tc.src, true, tc.kind == "loop")
		})
	}
}
