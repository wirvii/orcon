// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/credential/envelope"
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
	var gotMax, gotSlots int
	factory := func(maxAbandoned, slots int, _ *slog.Logger) exprSandbox {
		gotMax = maxAbandoned
		gotSlots = slots
		return fake
	}
	var stdout, stderr syncBuffer
	done := make(chan int, 1)
	go func() {
		done <- runServe(t.Context(), []string{"--http-address=127.0.0.1:0", "--http-shutdown-timeout=5s", "--expr-max-abandoned=5"}, &stdout, &stderr, testDeps(t, factory))
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
	if gotSlots < 1 || gotSlots > max(runtime.GOMAXPROCS(0)-1, 1) {
		t.Errorf("the sandbox was built with %d slots", gotSlots)
	}
	if gotMax != 5 {
		t.Errorf("the sandbox was built with max abandoned %d, want 5", gotMax)
	}
	log := stderr.String()
	for _, want := range []string{`"level":"ERROR"`, `"reason":"sandbox_abandoned"`, `"abandoned_running":8`, `"stuck":1`, `"saturated_total":3`, `"shutting down"`, `"memory_bound_bytes":`} {
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

func TestRunRejectsAnInvalidSlotCount(t *testing.T) {
	for _, v := range []string{"-1", "1000"} {
		var stdout, stderr syncBuffer
		code := runServe(t.Context(), []string{"--expr-sandbox-slots=" + v}, &stdout, &stderr, testDeps(t, func(int, int, *slog.Logger) exprSandbox {
			t.Error("the sandbox was built with an invalid slot count")
			return newFakeSandbox()
		}))
		// A negative count fails the range of the key (a configuration error); a count above the
		// processors fails the runtime check.
		want, text := exitUsage, "invalid --expr-sandbox-slots"
		if v == "-1" {
			want, text = exitValidation, "expr.sandboxSlots"
		}
		if code != want || !strings.Contains(stderr.String(), text) {
			t.Errorf("--expr-sandbox-slots=%s: exit %d, stderr %q", v, code, stderr.String())
		}
	}
}

func TestRunRejectsAnInvalidAbandonLimit(t *testing.T) {
	for _, v := range []string{"0", "65", "-1"} {
		var stdout, stderr syncBuffer
		code := runServe(t.Context(), []string{"--expr-max-abandoned=" + v}, &stdout, &stderr, testDeps(t, func(int, int, *slog.Logger) exprSandbox {
			t.Error("the sandbox was built with an invalid limit")
			return newFakeSandbox()
		}))
		if code != exitValidation || !strings.Contains(stderr.String(), "expr.maxAbandoned") {
			t.Errorf("--expr-max-abandoned=%s: exit %d, stderr %q", v, code, stderr.String())
		}
	}
}

// testDeps returns the dependencies of a server of a test: its own data directory, the
// environment of the test and a fast poll for reload requests.
func testDeps(t *testing.T, factory sandboxFactory) serveDeps {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ORCON_DATA_DIR", dir)
	provisionMasterKey(t, dir)
	return serveDeps{newSandbox: factory, environ: os.Environ(), reloadPoll: 20 * time.Millisecond}
}

// provisionMasterKey writes the master key file the tests run with into dataDir and sets
// the backup acknowledgement of its fingerprint, as an operator does before the first start
// (the serving roles refuse to start without a verified master key).
func provisionMasterKey(t *testing.T, dataDir string) {
	t.Helper()
	raw := bytes.Repeat([]byte{0x42}, envelope.KeySize)
	material, err := envelope.NewKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "keys"), 0o700); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	if err := os.WriteFile(filepath.Join(dataDir, "keys", "master.key"), []byte(encoded), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCON_MASTER_KEY_BACKUP_ACK", envelope.Fingerprint(material))
}
