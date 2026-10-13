// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/expr"
)

// fakeSandbox is an expression sandbox that the test degrades on demand.
type fakeSandbox struct {
	degraded chan struct{}
	once     sync.Once
	report   expr.HealthReport
	closed   chan struct{}
}

func newFakeSandbox() *fakeSandbox {
	return &fakeSandbox{degraded: make(chan struct{}), closed: make(chan struct{})}
}

func (f *fakeSandbox) Degraded() <-chan struct{}       { return f.degraded }
func (f *fakeSandbox) HealthReport() expr.HealthReport { return f.report }
func (f *fakeSandbox) Close(context.Context) error     { close(f.closed); return nil }
func (f *fakeSandbox) degrade()                        { f.once.Do(func() { close(f.degraded) }) }

// TestAbandonRestartSignal checks the wiring of the restart signal
// (SPEC-001 amendment 10, B.1 point 6): when the sandbox reports that it is
// saturated or stuck, the process shuts down gracefully and exits with code
// 75, logging the reason and the counts at error level; the server stops
// answering and the sandbox is released. A healthy sandbox changes nothing.
func TestAbandonRestartSignal(t *testing.T) {
	fake := newFakeSandbox()
	fake.report = expr.HealthReport{AbandonedRunning: 8, Stuck: 1, MaxAbandoned: 8, Abandoned: 11, Saturated: 3}
	var gotMax int
	factory := func(maxAbandoned int, _ *slog.Logger) exprSandbox {
		gotMax = maxAbandoned
		return fake
	}
	var stdout, stderr syncBuffer
	done := make(chan int, 1)
	go func() {
		done <- runWith(t.Context(), []string{"--addr=127.0.0.1:0", "--shutdown-timeout=5s", "--expr-max-abandoned=5"}, &stdout, &stderr, factory)
	}()
	base := "http://" + waitForListenAddr(t, &stderr, done)
	assertGet(t, base+"/healthz", http.StatusOK, "ok")

	select {
	case code := <-done:
		t.Fatalf("a healthy sandbox ended the process with %d", code)
	case <-time.After(50 * time.Millisecond):
	}

	fake.degrade()
	select {
	case code := <-done:
		if code != exitRestart || exitRestart != 75 {
			t.Fatalf("exit code = %d, want 75; stderr: %s", code, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the process did not shut down after the sandbox degraded")
	}
	if gotMax != 5 {
		t.Errorf("the sandbox was built with max abandoned %d, want 5", gotMax)
	}
	log := stderr.String()
	for _, want := range []string{`"level":"ERROR"`, `"reason":"sandbox_abandoned"`, `"abandoned_running":8`, `"stuck":1`, `"saturated_total":3`, `"shutting down"`, `"abandoned_memory_bound_bytes":335544320`} {
		if !strings.Contains(log, want) {
			t.Errorf("stderr lacks %s:\n%s", want, log)
		}
	}
	select {
	case <-fake.closed:
	default:
		t.Error("the sandbox was not released")
	}
}

func TestRunRejectsAnInvalidAbandonLimit(t *testing.T) {
	for _, v := range []string{"0", "65", "-1"} {
		var stdout, stderr syncBuffer
		code := runWith(t.Context(), []string{"--expr-max-abandoned=" + v}, &stdout, &stderr, func(int, *slog.Logger) exprSandbox {
			t.Error("the sandbox was built with an invalid limit")
			return newFakeSandbox()
		})
		if code != exitUsage || !strings.Contains(stderr.String(), "invalid --expr-max-abandoned") {
			t.Errorf("--expr-max-abandoned=%s: exit %d, stderr %q", v, code, stderr.String())
		}
	}
}
