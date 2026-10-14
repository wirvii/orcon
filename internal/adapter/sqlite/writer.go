// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/wirvii/orcon/internal/flow"
)

// ErrClosed is returned by operations on a closed store.
var ErrClosed = flow.ErrClosed

// maxGroup bounds the jobs one transaction commits together.
const maxGroup = 64

// writeFunc is the body of one write: it runs inside the shared transaction of
// its group, under a savepoint of its own.
type writeFunc func(ctx context.Context, tx *sql.Tx) error

type writeJob struct {
	ctx  context.Context //nolint:containedctx // the job carries the caller's context to the writer goroutine
	fn   writeFunc
	done chan error
}

// writer is the single writer of the database (ADR-0015 section 5): one
// goroutine owns the only write connection and applies queued jobs with group
// commit. Jobs that arrive while a transaction is being committed share the
// next transaction, each under its own savepoint, so one failing job rolls back
// alone and one fsync serves the whole group.
type writer struct {
	db   *sql.DB
	jobs chan *writeJob
	quit chan struct{}
	done chan struct{}

	closeOnce sync.Once
}

func newWriter(ctx context.Context, db *sql.DB) *writer {
	w := &writer{
		db:   db,
		jobs: make(chan *writeJob, 4*maxGroup),
		quit: make(chan struct{}),
		done: make(chan struct{}),
	}
	// The goroutine's own statements outlive the call that opened the store.
	go w.run(context.WithoutCancel(ctx))
	return w
}

// submit queues fn and waits for its group to commit.
func (w *writer) submit(ctx context.Context, fn writeFunc) error {
	job := &writeJob{ctx: ctx, fn: fn, done: make(chan error, 1)}
	select {
	case w.jobs <- job:
	case <-w.quit:
		return ErrClosed
	case <-ctx.Done():
		return fmt.Errorf("queue write: %w", ctx.Err())
	}
	select {
	case err := <-job.done:
		return err
	case <-w.done:
		// The goroutine ended; it answers every queued job before it ends.
		select {
		case err := <-job.done:
			return err
		default:
			return ErrClosed
		}
	}
}

// close stops the goroutine after it answered the queued jobs.
func (w *writer) close() {
	w.closeOnce.Do(func() { close(w.quit) })
	<-w.done
}

func (w *writer) run(ctx context.Context) {
	defer close(w.done)
	for {
		select {
		case first := <-w.jobs:
			w.commit(ctx, w.collect(first))
		case <-w.quit:
			for {
				select {
				case job := <-w.jobs:
					job.done <- ErrClosed
				default:
					return
				}
			}
		}
	}
}

// collect gathers the jobs already queued behind first, up to maxGroup.
func (w *writer) collect(first *writeJob) []*writeJob {
	batch := []*writeJob{first}
	for len(batch) < maxGroup {
		select {
		case job := <-w.jobs:
			batch = append(batch, job)
		default:
			return batch
		}
	}
	return batch
}

// commit applies a group in one transaction.
func (w *writer) commit(ctx context.Context, batch []*writeJob) {
	results := make([]error, len(batch))
	fail := func(err error) {
		for i, job := range batch {
			if results[i] == nil {
				results[i] = err
			}
			job.done <- results[i]
		}
	}
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		fail(fmt.Errorf("begin write transaction: %w", err))
		return
	}
	for i, job := range batch {
		if err := job.ctx.Err(); err != nil {
			results[i] = fmt.Errorf("write canceled before it ran: %w", err)
			continue
		}
		results[i] = w.runJob(ctx, tx, job)
	}
	if err := tx.Commit(); err != nil {
		// Nothing of the group is durable. The jobs that succeeded report the commit error.
		cerr := fmt.Errorf("commit write transaction: %w", err)
		for i := range results {
			if results[i] == nil {
				results[i] = cerr
			}
		}
	}
	for i, job := range batch {
		job.done <- results[i]
	}
}

// The statements of a job's savepoint are constants (orcon-db TI-8). The jobs of a group run one after the
// other, each releasing its savepoint before the next begins, so one name serves them all.
const (
	savepointStmt  = "SAVEPOINT job"
	rollbackToStmt = "ROLLBACK TO job"
	releaseStmt    = "RELEASE job"
)

func (w *writer) runJob(ctx context.Context, tx *sql.Tx, job *writeJob) (err error) {
	// Savepoint statements run under the goroutine's context, not the caller's: a half-rolled-back group would be worse.
	bg := ctx
	if _, err := tx.ExecContext(bg, savepointStmt); err != nil {
		return fmt.Errorf("savepoint: %w", err)
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("write panicked: %v", r)
		}
		if err != nil {
			if _, rerr := tx.ExecContext(bg, rollbackToStmt); rerr != nil {
				err = errors.Join(err, fmt.Errorf("rollback to savepoint: %w", rerr))
			}
		}
		if _, rerr := tx.ExecContext(bg, releaseStmt); rerr != nil {
			err = errors.Join(err, fmt.Errorf("release savepoint: %w", rerr))
		}
	}()
	return job.fn(job.ctx, tx) //nolint:contextcheck // the job runs under the context of the caller that queued it
}
