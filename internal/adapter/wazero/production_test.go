// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
)

// holdingGuest answers the first host call of a request only when released,
// which keeps the call inside the host: a call that cannot be stopped by its
// deadline, as a native built-in without a check would be.
type holdingGuest struct {
	release chan struct{}
	began   atomic.Int64
	last    atomic.Int64 // unix nanoseconds of the latest host call that began
}

func newHoldingGuest() *holdingGuest { return &holdingGuest{release: make(chan struct{})} }

func (g *holdingGuest) HostCall(ctx context.Context, op int32, _ []byte) ([]byte, error) {
	g.began.Add(1)
	g.last.Store(time.Now().UnixNano())
	if op == expr.OpInputAll {
		select {
		case <-g.release:
		case <-ctx.Done():
			<-g.release
		}
		return []byte("[]"), nil
	}
	return nil, nil
}
func (g *holdingGuest) Now() time.Time { return time.Unix(1791913530, 0).UTC() }
func (g *holdingGuest) Read(p []byte)  { clear(p) }

// metricsLog records the accounting calls.
type metricsLog struct {
	abandoned, saturated atomic.Int64
	running              atomic.Int64
	mu                   sync.Mutex
	overruns             []time.Duration
	waits                []time.Duration
}

func (m *metricsLog) Abandoned()             { m.abandoned.Add(1) }
func (m *metricsLog) AbandonedRunning(d int) { m.running.Add(int64(d)) }
func (m *metricsLog) Saturated()             { m.saturated.Add(1) }
func (m *metricsLog) SlotWait(d time.Duration) {
	m.mu.Lock()
	m.waits = append(m.waits, d)
	m.mu.Unlock()
}
func (m *metricsLog) AbandonOverrun(d time.Duration) {
	m.mu.Lock()
	m.overruns = append(m.overruns, d)
	m.mu.Unlock()
}

const holdRequest = `{"e":"$input.all().length","m":"v","i":0,"r":0,"n":1791913530250,"z":"UTC"}` + "\x01\x01\x01\x01"

func openProduction(t *testing.T, opts ...wazero.Option) (*wazero.Sandbox, func() expr.Runtime) {
	t.Helper()
	sb := wazero.New(opts...)
	t.Cleanup(func() { _ = sb.Close(context.WithoutCancel(t.Context())) })
	cfg := expr.New(expr.Options{}).SandboxConfig()
	return sb, func() expr.Runtime {
		rt, err := sb.Open(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = rt.Close(context.WithoutCancel(t.Context())) })
		return rt
	}
}

// abandonHeld starts a call that stays inside the host, lets its deadline
// pass and returns what the caller saw, with the guest still holding the call.
func abandonHeld(t *testing.T, rt expr.Runtime, g *holdingGuest) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	_, err := rt.Call(ctx, []byte(holdRequest), g)
	return err
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestProductionDefaultIsTheAbandonMode checks that a sandbox built without
// options stops evaluations the production way.
func TestProductionDefaultIsTheAbandonMode(t *testing.T) {
	_, open := openProduction(t)
	rt := open()
	g := newHoldingGuest()
	defer close(g.release)
	start := time.Now()
	err := abandonHeld(t, rt, g)
	if !errors.Is(err, expr.ErrTimeout) {
		t.Fatalf("got %v, want ErrTimeout", err)
	}
	// The guest still holds the call, yet the caller has its answer.
	if took := time.Since(start); defaultTestTimeout < time.Second && took > 30*time.Millisecond+callerSlack {
		t.Errorf("the caller waited %v for a call that could not be stopped", took)
	}
}

// TestAbandonedHostCallsRefused checks that, once a call is abandoned, its
// requests to the host are refused: it reads no more execution data and draws
// no more random numbers (ADR-0018 section 3, amendment 10, points 3 and 9).
func TestAbandonedHostCallsRefused(t *testing.T) {
	sb, open := openProduction(t)
	_ = sb
	rt := open()
	g := newHoldingGuest()
	// After the host call is released the code keeps drawing random numbers
	// until something stops it.
	req := `{"e":"(() => { $input.all(); for (;;) Math.random(); })()","m":"v","i":0,"r":0,"n":1791913530250,"z":"UTC"}` + "\x01\x01\x01\x01"
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	_, err := rt.Call(ctx, []byte(req), g)
	returned := time.Now()
	if !errors.Is(err, expr.ErrTimeout) {
		t.Fatalf("got %v, want ErrTimeout", err)
	}
	close(g.release)
	reporter, ok := rt.(overshootReporter)
	if !ok {
		t.Fatal("the runtime does not report the end of an abandoned call")
	}
	if _, abandoned := reporter.AbandonedOvershoot(t.Context()); !abandoned {
		t.Fatal("the call was not abandoned")
	}
	if last := time.Unix(0, g.last.Load()); last.After(returned) {
		t.Errorf("the abandoned call reached the host %v after its caller left", last.Sub(returned))
	}
}

// TestAbandonSaturation checks the limit of abandoned calls still running: at
// the limit a new call fails at once with ErrSandboxSaturated, which is not an
// error of the expression, and the failure is counted.
func TestAbandonSaturation(t *testing.T) {
	m := &metricsLog{}
	sb, open := openProduction(t, wazero.WithMaxAbandoned(2), wazero.WithMetrics(m))
	guests := []*holdingGuest{newHoldingGuest(), newHoldingGuest()}
	for _, g := range guests {
		if err := abandonHeld(t, open(), g); !errors.Is(err, expr.ErrTimeout) {
			t.Fatalf("got %v, want ErrTimeout", err)
		}
	}
	if r := sb.HealthReport(); r.AbandonedRunning != 2 || r.MaxAbandoned != 2 || r.Abandoned != 2 {
		t.Fatalf("report = %+v, want two abandoned calls running of a limit of two", r)
	}
	select {
	case <-sb.Degraded():
		t.Fatal("reaching the limit is not yet a refusal; the process stays up")
	default:
	}

	rt := open()
	start := time.Now()
	_, err := rt.Call(t.Context(), []byte(holdRequest), newHoldingGuest())
	if !errors.Is(err, expr.ErrSandboxSaturated) {
		t.Fatalf("got %v, want ErrSandboxSaturated", err)
	}
	var ee *expr.EvalError
	if errors.As(err, &ee) || errors.Is(err, expr.ErrTimeout) {
		t.Errorf("saturation must not look like an error of the expression: %v", err)
	}
	if took := time.Since(start); took > 50*time.Millisecond {
		t.Errorf("the refusal took %v, want it at once", took)
	}
	select {
	case <-sb.Degraded():
		t.Error("a refusal that drains by itself must not restart the process")
	default:
	}
	if r := sb.HealthReport(); r.Saturated != 1 || m.saturated.Load() != 1 || m.abandoned.Load() != 2 || m.running.Load() != 2 {
		t.Errorf("report = %+v, metrics = abandoned %d saturated %d running %d", r, m.abandoned.Load(), m.saturated.Load(), m.running.Load())
	}

	// The abandoned calls end, the room comes back.
	for _, g := range guests {
		close(g.release)
	}
	eventually(t, "the abandoned calls to end", func() bool { return sb.HealthReport().AbandonedRunning == 0 })
	if m.running.Load() != 0 {
		t.Errorf("the running gauge is %d after every abandoned call ended", m.running.Load())
	}
	m.mu.Lock()
	overruns := len(m.overruns)
	m.mu.Unlock()
	if overruns != 2 {
		t.Errorf("%d overruns recorded, want 2", overruns)
	}
	rt2 := open()
	g := newHoldingGuest()
	close(g.release)
	if _, err := rt2.Call(t.Context(), []byte(holdRequest), g); err != nil {
		t.Errorf("a call after the room came back: %v", err)
	}
	select {
	case <-sb.Degraded():
		t.Error("saturation that recovered must not have degraded the process")
	default:
	}
}

// TestAbandonStuck checks that an abandoned call still running after the stuck
// limit marks the sandbox degraded, and that the mark is lifted from the
// count when the call finally ends.
func TestAbandonStuck(t *testing.T) {
	sb, open := openProduction(t, wazero.WithStuckAfter(80*time.Millisecond))
	g := newHoldingGuest()
	if err := abandonHeld(t, open(), g); !errors.Is(err, expr.ErrTimeout) {
		t.Fatalf("got %v, want ErrTimeout", err)
	}
	select {
	case <-sb.Degraded():
		t.Fatal("degraded before the stuck limit")
	default:
	}
	select {
	case <-sb.Degraded():
	case <-time.After(10 * time.Second):
		t.Fatal("a call abandoned past the stuck limit did not degrade the sandbox")
	}
	if r := sb.HealthReport(); r.Stuck != 1 || r.AbandonedRunning != 1 {
		t.Errorf("report = %+v, want one stuck call", r)
	}
	close(g.release)
	eventually(t, "the stuck call to end", func() bool { return sb.HealthReport().AbandonedRunning == 0 })
	if r := sb.HealthReport(); r.Stuck != 0 {
		t.Errorf("report = %+v, want no stuck call left", r)
	}
}

// TestAbandonRestartSignal checks the signal the composition root turns into
// a restart: it is closed once, carries the counts, and a healthy sandbox
// never closes it.
func TestAbandonRestartSignal(t *testing.T) {
	sb, open := openProduction(t, wazero.WithMaxAbandoned(1), wazero.WithStuckAfter(100*time.Millisecond))
	select {
	case <-sb.Degraded():
		t.Fatal("a sandbox that abandoned nothing is degraded")
	default:
	}
	g := newHoldingGuest()
	defer close(g.release)
	if err := abandonHeld(t, open(), g); !errors.Is(err, expr.ErrTimeout) {
		t.Fatal(err)
	}
	for range 2 { // the second refusal must not close the channel twice
		if _, err := open().Call(t.Context(), []byte(holdRequest), newHoldingGuest()); !errors.Is(err, expr.ErrSandboxSaturated) {
			t.Fatalf("got %v, want ErrSandboxSaturated", err)
		}
	}
	select {
	case <-sb.Degraded():
	case <-time.After(10 * time.Second):
		t.Fatal("the restart signal was not raised although the sandbox stayed at its limit")
	}
	if r := sb.HealthReport(); r.Saturated != 2 || r.MaxAbandoned != 1 {
		t.Errorf("report = %+v", r)
	}
}

// TestAbandonIsLoggedWithIdsOnly checks the accounting line of an abandoned
// call: organization, execution and node ids and the deadline, never the
// expression text or its data.
func TestAbandonIsLoggedWithIdsOnly(t *testing.T) {
	var buf lockedBuffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	_, open := openProduction(t, wazero.WithLogger(logger))
	g := newHoldingGuest()
	defer close(g.release)
	rt := open()
	ctx := expr.WithCallIdentity(t.Context(), expr.CallIdentity{OrganizationID: "org-1", ExecutionID: "exec-1", NodeID: "node-1"})
	ctx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	if _, err := rt.Call(ctx, []byte(holdRequest), g); !errors.Is(err, expr.ErrTimeout) {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`"level":"WARN"`, `"organization_id":"org-1"`, `"execution_id":"exec-1"`, `"node_id":"node-1"`, `"deadline"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q lacks %s", out, want)
		}
	}
	if strings.Contains(out, "input.all") {
		t.Errorf("the log carries the expression text: %s", out)
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// TestAbandonCapHoldsUnderConcurrency checks that the calls that execute guest
// code, and so the abandoned ones, never exceed the slots, however many calls
// reach their deadline at once, and that no caller waits for its call: those
// that cannot get a slot fail with ErrSandboxSaturated after the slot wait,
// those that have one get ErrTimeout at their deadline (SPEC-001 amendment 10,
// section D).
func TestAbandonCapHoldsUnderConcurrency(t *testing.T) {
	const slots, calls = 2, 12
	m := &metricsLog{}
	sb, open := openProduction(t, wazero.WithMaxAbandoned(slots), wazero.WithSlots(slots), wazero.WithSlotWait(200*time.Millisecond), wazero.WithMetrics(m))
	var peak atomic.Int64
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		for {
			if n := int64(sb.HealthReport().AbandonedRunning); n > peak.Load() {
				peak.Store(n)
			}
			select {
			case <-stop:
				return
			default:
				time.Sleep(time.Millisecond)
			}
		}
	}()
	guests := make([]*holdingGuest, calls)
	rts := make([]expr.Runtime, calls)
	for i := range calls {
		guests[i], rts[i] = newHoldingGuest(), open()
	}
	var wg sync.WaitGroup
	errs := make([]error, calls)
	took := make([]time.Duration, calls)
	for i := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			errs[i] = abandonHeld(t, rts[i], guests[i])
			took[i] = time.Since(start)
		}()
	}
	wg.Wait() // every caller has its answer while every guest is still held
	timeouts, saturated := 0, 0
	for i, err := range errs {
		switch {
		case errors.Is(err, expr.ErrTimeout):
			timeouts++
		case errors.Is(err, expr.ErrSandboxSaturated):
			saturated++
		default:
			t.Errorf("call %d: got %v", i, err)
		}
		if limit := 30*time.Millisecond + 200*time.Millisecond + 100*time.Millisecond; took[i] > limit && defaultTestTimeout < time.Second {
			t.Errorf("call %d waited %v for its answer", i, took[i])
		}
	}
	if timeouts != slots || saturated != calls-slots {
		t.Errorf("%d calls timed out and %d were refused, want %d and %d", timeouts, saturated, slots, calls-slots)
	}
	if n := sb.HealthReport().AbandonedRunning; n > slots {
		t.Errorf("%d abandoned calls running with %d slots", n, slots)
	}
	for _, g := range guests {
		close(g.release)
	}
	eventually(t, "the abandoned calls to end", func() bool { return sb.HealthReport().AbandonedRunning == 0 })
	close(stop)
	<-sampled
	if peak.Load() > slots {
		t.Errorf("the abandoned calls peaked at %d with %d slots", peak.Load(), slots)
	}
}

// TestCancellationIsNotATimeout checks BL-031: a canceled context surfaces as
// the cancellation, not as the time limit, is not logged or counted as an
// abandon at the deadline, and a storm of cancellations neither saturates the
// sandbox for good nor restarts the process.
func TestCancellationIsNotATimeout(t *testing.T) {
	var buf lockedBuffer
	m := &metricsLog{}
	const storm = 24
	sb, open := openProduction(t, wazero.WithMetrics(m), wazero.WithSlots(storm), wazero.WithMaxAbandoned(storm), wazero.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil))))
	guests := make([]*holdingGuest, storm)
	var wg sync.WaitGroup
	errs := make([]error, storm)
	rts := make([]expr.Runtime, storm)
	for i := range storm {
		guests[i], rts[i] = newHoldingGuest(), open()
	}
	ctx, cancel := context.WithCancel(t.Context())
	for i := range storm {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = rts[i].Call(ctx, []byte(holdRequest), guests[i])
		}()
	}
	eventually(t, "every call to reach the host", func() bool {
		for _, g := range guests {
			if g.began.Load() == 0 {
				return false
			}
		}
		return true
	})
	cancel()
	time.Sleep(100 * time.Millisecond)
	if n := sb.HealthReport().AbandonedRunning; n > storm {
		t.Errorf("%d runs held after cancellation, over the %d slots", n, storm)
	}
	for _, g := range guests {
		close(g.release)
	}
	wg.Wait()
	for i, err := range errs {
		if !errors.Is(err, context.Canceled) || errors.Is(err, expr.ErrTimeout) {
			t.Errorf("call %d: got %v, want the cancellation and not ErrTimeout", i, err)
		}
	}
	eventually(t, "the canceled runs to end", func() bool { return sb.HealthReport().AbandonedRunning == 0 })
	if r := sb.HealthReport(); r.Abandoned != 0 || m.abandoned.Load() != 0 {
		t.Errorf("cancellations counted as abandons at the deadline: report %+v, metric %d", r, m.abandoned.Load())
	}
	if strings.Contains(buf.String(), "abandoned at its deadline") {
		t.Errorf("a cancellation was logged as an abandon at the deadline: %s", buf.String())
	}
	select {
	case <-sb.Degraded():
		t.Error("a storm of cancellations restarted the process")
	default:
	}
	g := newHoldingGuest()
	close(g.release)
	if _, err := open().Call(t.Context(), []byte(holdRequest), g); err != nil {
		t.Errorf("a call after the storm: %v", err)
	}
}
