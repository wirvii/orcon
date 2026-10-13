// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero

import (
	"context"
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
}

type noMetrics struct{}

func (noMetrics) Abandoned()                   {}
func (noMetrics) AbandonedRunning(int)         {}
func (noMetrics) AbandonOverrun(time.Duration) {}
func (noMetrics) Saturated()                   {}

// tracker accounts for the abandoned calls of one sandbox, which is one
// process in production. It enforces the limit of abandoned calls still
// running, finds the stuck ones, and tells the composition root when the
// process must restart.
type tracker struct {
	max        int
	stuckAfter time.Duration
	log        *slog.Logger
	metrics    Metrics

	mu        sync.Mutex
	running   int
	stuck     int
	abandoned uint64
	saturated uint64

	degraded     chan struct{}
	degradedOnce sync.Once
}

var _ expr.Health = (*Sandbox)(nil)

func newTracker(maxAbandoned int, stuckAfter time.Duration, log *slog.Logger, m Metrics) *tracker {
	if !config.ValidSandboxMaxAbandoned(maxAbandoned) {
		maxAbandoned = min(max(maxAbandoned, config.SandboxMaxAbandonedMin), config.SandboxMaxAbandonedMax)
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
	return &tracker{max: maxAbandoned, stuckAfter: stuckAfter, log: log, metrics: m, degraded: make(chan struct{})}
}

// admit decides whether a new sandbox call may start. With the limit of
// abandoned calls reached it refuses, counts the refusal and marks the
// process degraded: it can no longer serve expressions and must restart.
func (t *tracker) admit(ctx context.Context) error {
	t.mu.Lock()
	full := t.running >= t.max
	if full {
		t.saturated++
	}
	t.mu.Unlock()
	if !full {
		return nil
	}
	t.metrics.Saturated()
	t.markDegraded()
	t.log.ErrorContext(ctx, "expression sandbox saturated: new calls are refused until the process restarts",
		"abandoned_running", t.max, "max_abandoned", t.max)
	return expr.ErrSandboxSaturated
}

func (t *tracker) markDegraded() { t.degradedOnce.Do(func() { close(t.degraded) }) }

// abandonment is the record of one abandoned call.
type abandonment struct {
	t       *tracker
	at      time.Time
	timer   *time.Timer
	isStuck bool
	mu      sync.Mutex
	done    bool
}

// abandon registers a call abandoned at its deadline. The log line names the
// organization, execution and node and the deadline; never the expression or
// its data.
func (t *tracker) abandon(ctx context.Context, deadline time.Duration) *abandonment {
	id := expr.CallIdentityFrom(ctx)
	t.mu.Lock()
	t.running++
	t.abandoned++
	running := t.running
	t.mu.Unlock()
	t.metrics.Abandoned()
	t.metrics.AbandonedRunning(1)
	t.log.WarnContext(context.WithoutCancel(ctx), "expression evaluation abandoned at its deadline",
		"organization_id", id.OrganizationID, "execution_id", id.ExecutionID, "node_id", id.NodeID,
		"deadline", deadline, "abandoned_running", running)
	a := &abandonment{t: t, at: time.Now()}
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
	a.t.mu.Unlock()
	a.t.metrics.AbandonedRunning(-1)
	a.t.metrics.AbandonOverrun(time.Since(a.at))
}

// Degraded implements [expr.Health]: the channel is closed when the sandbox
// refused a call because of the limit of abandoned calls, or holds a stuck
// one. The process must then shut down gracefully and restart.
func (s *Sandbox) Degraded() <-chan struct{} { return s.track.degraded }

// HealthReport implements [expr.Health].
func (s *Sandbox) HealthReport() expr.HealthReport {
	t := s.track
	t.mu.Lock()
	defer t.mu.Unlock()
	return expr.HealthReport{
		AbandonedRunning: t.running, Stuck: t.stuck, MaxAbandoned: t.max,
		Abandoned: t.abandoned, Saturated: t.saturated,
	}
}
