// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestSealLoopBacksOffAndLogsOncePerInterval: a pass that keeps failing is retried at a growing pause,
// not every interval, and the failure is logged once, not once per pass. When the pass recovers, the
// loop returns to its normal pace and logs the recovery once.
func TestSealLoopBacksOffAndLogsOncePerInterval(t *testing.T) {
	var logs syncBuffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	pace := sealPace{interval: time.Millisecond, maxBackoff: 20 * time.Millisecond, logEvery: time.Hour}

	var mu sync.Mutex
	calls, healthy := 0, false
	seal := func(context.Context) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if healthy {
			return 1, nil
		}
		return 0, errors.New("seal audit of org:x: mac key: key not found")
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); sealLoop(ctx, logger, pace, seal) }()

	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	failing := calls
	healthy = true
	mu.Unlock()
	// Without a backoff a 1 ms pace would make about 300 passes; with it, the pause reaches 20 ms.
	if failing > 40 {
		t.Errorf("%d failed passes in 300 ms, want the pause to grow", failing)
	}
	if got := strings.Count(logs.String(), "audit sealing failed"); got != 1 {
		t.Errorf("the lasting failure was logged %d times, want once:\n%s", got, logs.String())
	}
	deadline := time.After(2 * time.Second)
	for !strings.Contains(logs.String(), "audit sealing recovered") {
		select {
		case <-deadline:
			t.Fatalf("the recovery was not logged:\n%s", logs.String())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-done
	if got := strings.Count(logs.String(), "audit sealing recovered"); got != 1 {
		t.Errorf("recovery logged %d times, want once", got)
	}
}
