// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/execution/store"
)

// runnableChannel is the LISTEN/NOTIFY channel that says tokens may be runnable
// (SPEC-001 C4b). The notice carries no payload: a listener only wakes its
// idle workers, which claim from the tables as they always do.
const runnableChannel = "orcon_runnable"

// listenerApplication names the connection that listens, in pg_stat_activity, so
// that an operator can tell it from the pooled connections of the process.
const listenerApplication = "orcon-notice"

// Compile-time check: the store offers the wake-up across processes.
var _ store.Notifier = (*Store)(nil)

// NotifyRunnable sends the runnable notice on its own connection. The notice
// leaves with the statement, after the commit of the step that made the tokens
// runnable, so that a listener never wakes for a commit it cannot see.
func (s *Store) NotifyRunnable(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, "SELECT pg_notify($1, '')", runnableChannel); err != nil {
		return fmt.Errorf("notify %s: %w", runnableChannel, err)
	}
	return nil
}

// ListenRunnable holds one pooled connection on LISTEN and calls wake for each
// notice until ctx ends. A connection that fails is taken out of the pool and
// the error is returned, so that the caller can start the listening again on a
// new connection. The connection is closed, never returned to the pool, because
// it carries the LISTEN.
func (s *Store) ListenRunnable(ctx context.Context, wake func()) error {
	pooled, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire listening connection: %w", err)
	}
	conn := pooled.Hijack()
	defer conn.Close(context.WithoutCancel(ctx)) //nolint:errcheck // the connection is ending; nothing to report
	if _, err := conn.Exec(ctx, "SET application_name = '"+listenerApplication+"'"); err != nil {
		return fmt.Errorf("name the listening connection: %w", err)
	}
	if _, err := conn.Exec(ctx, "LISTEN "+runnableChannel); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("listen %s: %w", runnableChannel, err)
	}
	for {
		if _, err := conn.WaitForNotification(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("wait for %s: %w", runnableChannel, err)
		}
		wake()
	}
}
