// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
)

const (
	loadDeadline = 100 * time.Millisecond
	// The slot wait is shorter than the deadline, so a call that waits for a
	// slot cannot get one: the slots are held by calls that run until their
	// deadline at least.
	loadSlotWait = 50 * time.Millisecond
	// The slack is the margin of the bounds of SPEC-001 amendment 10, section D.
	loadSlack = 5 * time.Millisecond
)

// schedulerBound is the owner's bound (2026-10-13) on how late the 1 ms
// heartbeat may run and on the longest garbage-collection pause while hostile
// calls run. A single miss fails: there is no retry. It holds until BL-033
// moves the sandbox to a separate process, a gate before the first release.
const schedulerBound = 15 * time.Millisecond

// startHeartbeat runs a 1 ms timer and returns a function that stops it and
// reports the latest it ran.
func startHeartbeat() (stop func() time.Duration) {
	done := make(chan struct{})
	res := make(chan time.Duration, 1)
	go func() {
		var worst time.Duration
		last := time.Now()
		for {
			select {
			case <-done:
				res <- worst
				return
			case <-time.After(time.Millisecond):
			}
			now := time.Now()
			if d := now.Sub(last) - time.Millisecond; d > worst {
				worst = d
			}
			last = now
		}
	}()
	return func() time.Duration { close(done); return <-res }
}

// gcPauses reads the longest stop-the-world pause the runtime has recorded.
func gcPauses() *metrics.Float64Histogram {
	s := []metrics.Sample{{Name: "/sched/pauses/total/gc:seconds"}}
	metrics.Read(s)
	if s[0].Value.Kind() != metrics.KindFloat64Histogram {
		return nil
	}
	return s[0].Value.Float64Histogram()
}

// maxPauseSince returns the upper edge of the highest bucket that gained
// pauses between two readings.
func maxPauseSince(before, after *metrics.Float64Histogram) time.Duration {
	if before == nil || after == nil {
		return 0
	}
	var worst float64
	for i, c := range after.Counts {
		if c > before.Counts[i] {
			hi := after.Buckets[i+1]
			if math.IsInf(hi, 1) {
				hi = after.Buckets[i]
			}
			worst = max(worst, hi)
		}
	}
	return time.Duration(worst * float64(time.Second))
}

// loadSources are the hostile calls of the load test: the cases of
// abandonCases that run for seconds unstopped, those of BL-030 first.
func loadSources() []string {
	var out []string
	for _, c := range abandonCases[17:] {
		out = append(out, c.src)
	}
	for _, c := range abandonCases[:17] {
		out = append(out, c.src)
	}
	return out
}

type loadResult struct {
	err  error
	took time.Duration
}

// loadStats is what one run of the load measured.
type loadStats struct {
	Calls, Holders, Refused int
	// HeldMax is the longest wait of a call that held a slot, RefusedMax the
	// longest wait of a call refused for want of one.
	HeldMax, RefusedMax time.Duration
	// Heartbeat is the latest the 1 ms timer ran; GCPause the longest pause.
	Heartbeat, GCPause time.Duration
}

// runLoad drives 2*S+2 hostile calls at once on a machine with procs
// processors and S slots, with the garbage collector running all the time and
// a heartbeat that measures how late the scheduler runs a 1 ms timer. It
// returns the problems found.
func runLoad(t *testing.T, procs, slots int) (loadStats, []string) {
	t.Helper()
	prev := runtime.GOMAXPROCS(procs)
	defer runtime.GOMAXPROCS(prev)
	effective := min(2, slots)
	calls := 2*slots + 2

	m := &metricsLog{}
	sb := wazero.New(wazero.WithSlots(slots), wazero.WithMaxAbandoned(2), wazero.WithSlotWait(loadSlotWait), wazero.WithMetrics(m))
	defer func() { _ = sb.Close(context.WithoutCancel(t.Context())) }()
	cfg := expr.New(expr.Options{Timeout: loadDeadline, MemoryLimit: 64 << 20}).SandboxConfig()
	srcs := loadSources()

	rts := make([]expr.Runtime, calls)
	defer func() {
		for _, rt := range rts {
			if rt != nil {
				_ = rt.Close(context.WithoutCancel(t.Context()))
			}
		}
	}()
	for i := range rts {
		rt, err := sb.Open(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		rts[i] = rt
	}

	// The load: a collector that never rests, with something to collect, and a
	// heartbeat.
	oldGC := debug.SetGCPercent(10)
	defer debug.SetGCPercent(oldGC)
	stop := make(chan struct{})
	var bg sync.WaitGroup
	bg.Add(1)
	go func() {
		defer bg.Done()
		var keep [][]byte
		for {
			select {
			case <-stop:
				return
			default:
			}
			keep = append(keep, make([]byte, 1<<16))
			if len(keep) > 64 {
				keep = nil
			}
			runtime.GC()
		}
	}()
	stopBeat := startHeartbeat()
	time.Sleep(20 * time.Millisecond)
	pausesBefore := gcPauses()

	results := make([]loadResult, calls)
	var start, done sync.WaitGroup
	start.Add(1)
	for i := range calls {
		done.Add(1)
		go func() {
			defer done.Done()
			req := `{"e":` + quote(srcs[i%len(srcs)]) + `,"m":"v","i":0,"r":0,"n":1791913530250,"z":"UTC"}` + "\x01\x01\x01\x01"
			ctx := expr.WithTimeout(t.Context(), loadDeadline)
			start.Wait()
			begin := time.Now()
			_, err := rts[i].Call(ctx, []byte(req), &reachGuest{})
			results[i] = loadResult{err: err, took: time.Since(begin)}
		}()
	}
	start.Done()
	done.Wait()
	// Let the abandoned calls end while the load goes on.
	deadline := time.Now().Add(5 * time.Second)
	for sb.HealthReport().AbandonedRunning > 0 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	pausesAfter := gcPauses()
	close(stop)
	bg.Wait()
	worstBeat := stopBeat()

	var problems []string
	stats := loadStats{Calls: calls, Heartbeat: worstBeat, GCPause: maxPauseSince(pausesBefore, pausesAfter)}
	timeouts, saturated := 0, 0
	for i, r := range results {
		switch {
		case errors.Is(r.err, expr.ErrTimeout):
			timeouts++
			stats.HeldMax = max(stats.HeldMax, r.took)
			if r.took > loadDeadline+loadSlack {
				problems = append(problems, fmt.Sprintf("call %d held a slot and waited %v, more than the deadline plus %v", i, r.took, loadSlack))
			}
		case errors.Is(r.err, expr.ErrSandboxSaturated):
			saturated++
			stats.RefusedMax = max(stats.RefusedMax, r.took)
			if r.took > loadSlotWait+loadSlack {
				problems = append(problems, fmt.Sprintf("call %d waited %v for a slot, more than %v plus %v", i, r.took, loadSlotWait, loadSlack))
			}
		default:
			t.Fatalf("call %d: got %v after %v, want ErrTimeout or ErrSandboxSaturated", i, r.err, r.took)
		}
	}
	// The slots are the room: the calls that got one ran to their deadline, the
	// rest were refused without waiting for a call.
	stats.Holders, stats.Refused = timeouts, saturated
	if timeouts != slots || saturated != calls-slots {
		problems = append(problems, fmt.Sprintf("%d timeouts and %d refusals, want %d and %d", timeouts, saturated, slots, calls-slots))
	}
	if defaultTestTimeout < time.Second {
		if worstBeat > schedulerBound {
			t.Errorf("procs=%d slots=%d: the heartbeat ran %v late, more than %v", procs, slots, worstBeat, schedulerBound)
		}
		if p := stats.GCPause; p > schedulerBound {
			t.Errorf("procs=%d slots=%d: a garbage-collection pause of %v, more than %v", procs, slots, p, schedulerBound)
		}
	}

	// Saturation is a refusal that drains by itself: no restart of the
	// process, and the room comes back.
	select {
	case <-sb.Degraded():
		problems = append(problems, "the sandbox asked for a restart for a load that drained by itself")
	default:
	}
	r := sb.HealthReport()
	if r.AbandonedRunning != 0 {
		problems = append(problems, fmt.Sprintf("%d abandoned calls still running after the load", r.AbandonedRunning))
	}
	if r.MaxAbandoned != effective || r.Slots != slots {
		problems = append(problems, fmt.Sprintf("report %+v, want %d slots and a limit of %d", r, slots, effective))
	}
	if r.Saturated < uint64(calls-slots) { //nolint:gosec // both are small positive counts
		problems = append(problems, fmt.Sprintf("%d refusals counted, want at least %d", r.Saturated, calls-slots))
	}
	rt, err := sb.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Close(context.WithoutCancel(t.Context())) }()
	g := newHoldingGuest()
	close(g.release)
	if _, err := rt.Call(t.Context(), []byte(holdRequest), g); err != nil {
		problems = append(problems, fmt.Sprintf("a call after the load: %v", err))
	}
	t.Logf("procs=%d slots=%d calls=%d: %d timeouts, %d refusals, heartbeat %v late, gc pause <= %v",
		procs, slots, calls, timeouts, saturated, worstBeat, maxPauseSince(pausesBefore, pausesAfter))
	return stats, problems
}

// TestAbandonUnderLoad checks SPEC-001 amendment 10, section D: with more
// hostile calls than slots, the garbage collector running and two or four
// processors, the calls that hold a slot have their timeout at the deadline,
// the others are refused at the slot-wait limit, the process keeps serving
// timers (a heartbeat) and its pauses stay short, and nothing waits for a call
// it cannot stop. The heartbeat and the pauses are held to schedulerBound
// with no retry; the other timing bounds hold on a machine that runs other
// work, so a configuration that misses one is run again, up to three times.
func TestAbandonUnderLoad(t *testing.T) {
	for _, procs := range []int{2, 4} {
		seenDefault := false
		for _, slots := range []int{max(procs-1, 1), 1} {
			if procs == 2 && slots == 1 && seenDefault {
				continue // the same configuration as the default
			}
			seenDefault = true
			t.Run(fmt.Sprintf("procs=%d/slots=%d", procs, slots), func(t *testing.T) {
				var last []string
				for attempt := 1; attempt <= 3; attempt++ {
					_, last = runLoad(t, procs, slots)
					if len(last) == 0 {
						return
					}
					t.Logf("attempt %d: %v", attempt, last)
					if defaultTestTimeout >= time.Second {
						t.Logf("not enforced under the race detector")
						return
					}
				}
				for _, p := range last {
					t.Error(p)
				}
			})
		}
	}
}

// pastDeadline has a deadline that already passed and no timer: Err stays nil
// and Done never closes, so only a guest that reads the clock can notice.
type pastDeadline struct{ context.Context } //nolint:containedctx // a test double that wraps a context to change its deadline

func (pastDeadline) Deadline() (time.Time, bool) { return time.Now().Add(-time.Second), true }

// TestAbandonDeadlineWithoutTimer checks that the deadline is read from the
// clock by the guest's own interrupt checks and not only from a timer: a
// context whose deadline has passed but whose timer never fires still stops a
// loop, with ErrTimeout, within a few milliseconds.
func TestAbandonDeadlineWithoutTimer(t *testing.T) {
	sb := wazero.New()
	defer func() { _ = sb.Close(context.WithoutCancel(t.Context())) }()
	cfg := expr.New(expr.Options{}).SandboxConfig()
	rt, err := sb.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Close(context.WithoutCancel(t.Context())) }()
	req := `{"e":"(() => { for (;;) {} })()","m":"v","i":0,"r":0,"n":1791913530250,"z":"UTC"}` + "\x01\x01\x01\x01"
	ctx := pastDeadline{t.Context()}
	if ctx.Err() != nil {
		t.Fatal("the test context must not be done")
	}
	start := time.Now()
	_, err = rt.Call(ctx, []byte(req), &reachGuest{})
	if !errors.Is(err, expr.ErrTimeout) {
		t.Fatalf("got %v, want ErrTimeout", err)
	}
	if took := time.Since(start); took > 100*time.Millisecond && defaultTestTimeout < time.Second {
		t.Errorf("the loop stopped after %v, want a few milliseconds", took)
	}
}
