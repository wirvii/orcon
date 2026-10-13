// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/config"
)

// DefaultStuckAfter is how long an abandoned call may keep running before the
// process counts it as stuck (ADR-0018 section 3, amendment 10).
const DefaultStuckAfter = 10 * time.Second

// Names of the metrics of the abandon accounting. They are provisional until
// the observability ADR fixes the naming (as ADR-0073 does for the others).
const (
	MetricAbandonedTotal        = "orcon_expr_sandbox_abandoned_total"
	MetricAbandonedRunning      = "orcon_expr_sandbox_abandoned_running"
	MetricAbandonOverrunSeconds = "orcon_expr_sandbox_abandon_overrun_seconds"
	MetricSlotWaitSeconds       = "orcon_expr_sandbox_slot_wait_seconds"
	MetricSaturatedTotal        = "orcon_expr_sandbox_saturated_total"
)

// Metrics receives the accounting of abandoned calls. The telemetry package
// implements it by recording the four metrics named above.
type Metrics interface {
	// Abandoned counts a call abandoned at its deadline
	// (orcon_expr_sandbox_abandoned_total).
	Abandoned()
	// AbandonedRunning moves the gauge of abandoned calls still running by
	// delta, +1 or -1 (orcon_expr_sandbox_abandoned_running).
	AbandonedRunning(delta int)
	// AbandonOverrun records how long an abandoned call ran after it was
	// abandoned (orcon_expr_sandbox_abandon_overrun_seconds).
	AbandonOverrun(d time.Duration)
	// Saturated counts a call refused because the limit of abandoned calls
	// still running was reached (orcon_expr_sandbox_saturated_total).
	Saturated()
	// SlotWait records how long a call waited for an execution slot, zero when
	// one was free (orcon_expr_sandbox_slot_wait_seconds).
	SlotWait(d time.Duration)
}

type noMetrics struct{}

func (noMetrics) Abandoned()                   {}
func (noMetrics) AbandonedRunning(int)         {}
func (noMetrics) AbandonOverrun(time.Duration) {}
func (noMetrics) Saturated()                   {}
func (noMetrics) SlotWait(time.Duration)       {}

// tracker accounts for the abandoned calls of one sandbox, which is one
// process in production. It enforces the limit of abandoned calls still
// running, finds the stuck ones, and tells the composition root when the
// process must restart.
type tracker struct {
	max        int
	stuckAfter time.Duration
	// slots is a semaphore: a call holds one from before it enters the guest
	// until the guest returns, abandoned calls included, so at most cap(slots)
	// calls execute guest code and one scheduler slot stays free for timers,
	// callers and the API (amendment 10, section D).
	slots    chan struct{}
	slotWait time.Duration
	log      *slog.Logger
	metrics  Metrics

	mu        sync.Mutex
	running   int
	stuck     int
	abandoned uint64
	saturated uint64

	// satTimer fires when the sandbox has been at its limit for the stuck
	// period; guarded by mu.
	satTimer *time.Timer

	degraded     chan struct{}
	degradedOnce sync.Once
}

var _ expr.Health = (*Sandbox)(nil)

// DefaultSlotWait is how long a call waits for an execution slot before it
// fails with [expr.ErrSandboxSaturated].
const DefaultSlotWait = time.Second

func newTracker(maxAbandoned, slots int, slotWait, stuckAfter time.Duration, log *slog.Logger, m Metrics) *tracker {
	if !config.ValidSandboxMaxAbandoned(maxAbandoned) {
		maxAbandoned = min(max(maxAbandoned, config.SandboxMaxAbandonedMin), config.SandboxMaxAbandonedMax)
	}
	slots = max(slots, 1)
	// Every live call holds a slot, so no more than that many can be abandoned.
	maxAbandoned = min(maxAbandoned, slots)
	if slotWait <= 0 {
		slotWait = DefaultSlotWait
	}
	if stuckAfter <= 0 {
		stuckAfter = DefaultStuckAfter
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	if m == nil {
		m = noMetrics{}
	}
	return &tracker{
		max: maxAbandoned, stuckAfter: stuckAfter, log: log, metrics: m, degraded: make(chan struct{}),
		slots: make(chan struct{}, slots), slotWait: slotWait,
	}
}

// acquireSlot takes an execution slot. The wait does not count in the
// deadline of the evaluation and is bounded: when it runs out, or the outer
// context ends first, the call fails with [expr.ErrSandboxSaturated], an error
// of the infrastructure that does not mark the process degraded by itself.
func (t *tracker) acquireSlot(ctx context.Context) error {
	select {
	case t.slots <- struct{}{}:
		t.metrics.SlotWait(0)
		return nil
	default:
	}
	began := time.Now()
	timer := time.NewTimer(t.slotWait)
	defer timer.Stop()
	select {
	case t.slots <- struct{}{}:
		t.metrics.SlotWait(time.Since(began))
		return nil
	case <-timer.C:
	case <-ctx.Done():
	}
	t.mu.Lock()
	t.saturated++
	t.mu.Unlock()
	t.metrics.Saturated()
	return fmt.Errorf("%w: no execution slot was free within %v", expr.ErrSandboxSaturated, t.slotWait)
}

// releaseSlot returns the slot of a call whose guest has returned.
func (t *tracker) releaseSlot() { <-t.slots }

// admit decides whether a new sandbox call may start. With the limit of
// abandoned calls reached it refuses and counts the refusal. The process is
// marked degraded only if the condition persists (see [tracker.watchSaturation]):
// a burst that drains by itself, such as the cancellation of a batch of
// executions, does not restart it.
func (t *tracker) admit(ctx context.Context) error {
	t.mu.Lock()
	full := t.running >= t.max
	if full {
		t.saturated++
		t.watchSaturationLocked()
	}
	t.mu.Unlock()
	if !full {
		return nil
	}
	t.metrics.Saturated()
	t.log.ErrorContext(ctx, "expression sandbox saturated: new calls are refused until abandoned calls finish",
		"abandoned_running", t.max, "max_abandoned", t.max)
	return expr.ErrSandboxSaturated
}

// watchSaturationLocked starts the timer that marks the process degraded if
// the sandbox is still at its limit after the stuck period. It is idempotent
// and is stopped when room comes back.
func (t *tracker) watchSaturationLocked() {
	if t.satTimer != nil {
		return
	}
	t.satTimer = time.AfterFunc(t.stuckAfter, func() {
		t.mu.Lock()
		t.satTimer = nil
		full := t.running >= t.max
		t.mu.Unlock()
		if full {
			t.markDegraded()
		}
	})
}

func (t *tracker) markDegraded() { t.degradedOnce.Do(func() { close(t.degraded) }) }

// abandonment is the record of one abandoned call.
type abandonment struct {
	t       *tracker
	at      time.Time
	timer   *time.Timer
	isStuck bool
	// counted is set for a call abandoned at its deadline.
	counted bool
	mu      sync.Mutex
	done    bool
}

// abandon registers a call the caller gave up on. It enforces the limit
// strictly: when abandoned calls still running already equal the limit it
// registers nothing and reports false, and the caller must keep waiting for
// its call instead of leaving it behind, so the memory the abandoned calls can
// hold is bounded by the limit whatever the concurrency. A call abandoned
// because of its deadline is counted, logged (organization, execution and node
// ids and the deadline, never the expression or its data) and measured;
// one abandoned because its context was canceled holds a slot but is not an
// abandon at the deadline.
func (t *tracker) abandon(ctx context.Context, deadline time.Duration, atDeadline bool) *abandonment {
	id := expr.CallIdentityFrom(ctx)
	t.mu.Lock()
	t.running++
	if t.running >= t.max {
		t.watchSaturationLocked() // the process is saturated: new calls are refused from now on
	}
	if atDeadline {
		t.abandoned++
	}
	running := t.running
	t.mu.Unlock()
	t.metrics.AbandonedRunning(1)
	if atDeadline {
		t.metrics.Abandoned()
		t.log.WarnContext(context.WithoutCancel(ctx), "expression evaluation abandoned at its deadline",
			"organization_id", id.OrganizationID, "execution_id", id.ExecutionID, "node_id", id.NodeID,
			"deadline", deadline, "abandoned_running", running)
	}
	a := &abandonment{t: t, at: time.Now(), counted: atDeadline}
	a.timer = time.AfterFunc(t.stuckAfter, a.markStuck)
	return a
}

func (a *abandonment) markStuck() {
	a.mu.Lock()
	if a.done {
		a.mu.Unlock()
		return
	}
	a.isStuck = true
	a.mu.Unlock()
	a.t.mu.Lock()
	a.t.stuck++
	a.t.mu.Unlock()
	a.t.markDegraded()
}

// ended is called when the abandoned call finished and its runtime is closed.
func (a *abandonment) ended() {
	a.timer.Stop()
	a.mu.Lock()
	a.done = true
	wasStuck := a.isStuck
	a.mu.Unlock()
	a.t.mu.Lock()
	a.t.running--
	if wasStuck {
		a.t.stuck--
	}
	if a.t.running < a.t.max && a.t.satTimer != nil {
		a.t.satTimer.Stop()
		a.t.satTimer = nil
	}
	a.t.mu.Unlock()
	a.t.metrics.AbandonedRunning(-1)
	if a.counted {
		a.t.metrics.AbandonOverrun(time.Since(a.at))
	}
}

// Degraded implements [expr.Health]: the channel is closed when the sandbox
// has stayed at its limit of abandoned calls for the stuck period, or holds a
// stuck abandoned call. The process must then shut down gracefully and restart.
func (s *Sandbox) Degraded() <-chan struct{} { return s.track.degraded }

// HealthReport implements [expr.Health].
func (s *Sandbox) HealthReport() expr.HealthReport {
	t := s.track
	t.mu.Lock()
	defer t.mu.Unlock()
	return expr.HealthReport{
		AbandonedRunning: t.running, Stuck: t.stuck, MaxAbandoned: t.max, Slots: cap(t.slots),
		Abandoned: t.abandoned, Saturated: t.saturated,
	}
}
