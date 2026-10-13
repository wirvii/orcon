// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package egress

import (
	"context"
	"log/slog"
	"time"
)

// Outcome classifies how a request ended, as seen by the egress client.
type Outcome string

// Request outcomes. They are a closed set, safe as metric labels.
const (
	// OutcomeCompleted: the response headers arrived.
	OutcomeCompleted Outcome = "COMPLETED"
	// OutcomeDenied: the policy refused the request.
	OutcomeDenied Outcome = "DENIED"
	// OutcomeFailed: the request was allowed but the exchange failed.
	OutcomeFailed Outcome = "FAILED"
	// OutcomeTooLarge: a request or response exceeded the size limit.
	OutcomeTooLarge Outcome = "TOO_LARGE"
)

// Event describes one request hop for metrics and audit. A redirect produces
// one event per hop. It never holds a path, query, header, body or credential.
type Event struct {
	Outcome  Outcome
	Reason   Reason // set when Outcome is OutcomeDenied
	Method   string
	Scheme   string
	Host     string
	Status   int // zero unless Outcome is OutcomeCompleted
	Duration time.Duration
}

// Observer receives one Event per request hop. The composition root adapts it
// to counters and histograms. Implementations must be safe for concurrent
// use and must not block.
type Observer interface {
	Observe(Event)
}

// ObserverFunc adapts a function to Observer.
type ObserverFunc func(Event)

// Observe implements Observer.
func (f ObserverFunc) Observe(e Event) { f(e) }

func (c *Client) emit(ctx context.Context, e Event) {
	if c.observer != nil {
		c.observer.Observe(e)
	}
	if c.logger == nil {
		return
	}
	level := slog.LevelDebug
	if e.Outcome == OutcomeDenied || e.Outcome == OutcomeTooLarge {
		level = slog.LevelWarn
	}
	// Only fixed fields: scheme, host and reason never carry a secret, unlike
	// the path or query of a URL.
	c.logger.LogAttrs(ctx, level, "egress request",
		slog.String("outcome", string(e.Outcome)),
		slog.String("reason", string(e.Reason)),
		slog.String("method", e.Method),
		slog.String("scheme", e.Scheme),
		slog.String("host", e.Host),
		slog.Int("status", e.Status),
		slog.Duration("duration", e.Duration),
	)
}
