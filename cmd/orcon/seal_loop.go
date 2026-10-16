// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"
	"time"
)

// sealPace is how often the audit sealer runs and how it behaves when a pass fails.
type sealPace struct {
	// interval is the pause between passes while they succeed (SPEC-004 R-AUD-03).
	interval time.Duration
	// maxBackoff caps the pause between passes while they fail; the pause doubles from
	// interval on every failed pass.
	maxBackoff time.Duration
	// logEvery is the shortest time between two logged failures: a failure that lasts is
	// reported once per this interval, with how many passes failed in between.
	logEvery time.Duration
}

// defaultSealPace seals within 50 ms, backs off to 30 s on failure and logs a lasting failure once a minute.
var defaultSealPace = sealPace{interval: sealInterval, maxBackoff: 30 * time.Second, logEvery: time.Minute}

// sealLoop calls seal until ctx ends. A pass that fails does not repeat at the normal pace: the
// pause doubles up to maxBackoff, the failure is logged on its first pass and then once per
// logEvery, and the recovery is logged once.
func sealLoop(ctx context.Context, logger *slog.Logger, pace sealPace, seal func(context.Context) (int, error)) {
	delay := pace.interval
	var failedPasses int
	var lastLogged time.Time
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		_, err := seal(ctx)
		switch {
		case err == nil:
			if failedPasses > 0 {
				logger.InfoContext(ctx, "audit sealing recovered", "failed_passes", failedPasses)
			}
			failedPasses = 0
			delay = pace.interval
		case ctx.Err() != nil:
			return
		default:
			failedPasses++
			now := time.Now()
			if failedPasses == 1 || now.Sub(lastLogged) >= pace.logEvery {
				logger.ErrorContext(ctx, "audit sealing failed", "error", err, "failed_passes", failedPasses, "retry_in", min(delay*2, pace.maxBackoff).String())
				lastLogged = now
			}
			delay = min(delay*2, pace.maxBackoff)
		}
		timer.Reset(delay)
	}
}
