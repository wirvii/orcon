// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"log/slog"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
)

// The listening connection is started again with a growing pause, from the
// first to the last of these, so that a database that is down is not hammered.
const (
	listenRetryFirst = 100 * time.Millisecond
	listenRetryLast  = 5 * time.Second
)

// notifier returns the notification port of the store, or nil when the store has
// none or the configuration turns the wake-up across processes off.
//
//nolint:ireturn // the optional port of the store; nil when the store has none or the notice is off
func (e *Engine) notifier() store.Notifier {
	if e.cfg.DisableNotify {
		return nil
	}
	n, _ := e.store.(store.Notifier)
	return n
}

// notifyRunnable tells the other processes that tokens may be runnable. It is
// best effort: a lost notice only means that a worker elsewhere claims at its
// next poll, so a failure is logged and nothing else.
func (e *Engine) notifyRunnable(ctx context.Context) {
	n := e.notifier()
	if n == nil {
		return
	}
	if err := n.NotifyRunnable(ctx); err != nil && ctx.Err() == nil {
		e.cfg.Logger.WarnContext(ctx, "runnable notice not sent; idle workers claim at their next poll", slog.Any("error", err))
	}
}

// listenLoop keeps this process listening for the notices of the others and
// wakes one idle worker per notice. When the listening connection fails it says
// so and tries again with a pause that grows up to listenRetryLast; until it is
// back, the idle workers claim by polling, which is the guarantee (SPEC-001 E-REC-3).
func (e *Engine) listenLoop(ctx context.Context, n store.Notifier) {
	retry := listenRetryFirst
	for ctx.Err() == nil {
		started := time.Now()
		err := n.ListenRunnable(ctx, func() { e.signal(1) })
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > time.Minute {
			retry = listenRetryFirst // it was up for a while: the failure is new
		}
		e.cfg.Logger.WarnContext(ctx, "notice channel lost; idle workers poll until it is back", slog.Any("error", err), slog.Duration("retry", retry))
		t := time.NewTimer(retry)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
		retry = min(retry*2, listenRetryLast)
	}
}
