// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package sqlite is the SQLite storage adapter (ADR-0015 section 5): one
// process per database file guarded by an exclusive lock file, WAL, a single
// writer goroutine applying transactions with group commit, a pool of
// read-only connections, automatic migrations under the lock and the queries of
// the area ports compiled by sqlc. It implements the ports of the flow area.
//
// Every connection sets foreign_keys, busy_timeout and trusted_schema through
// the driver's per-connection pragma parameters (orcon-db CN-2).
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/clock"
)

// ErrLocked means another process holds the database file.
var ErrLocked = errors.New("database file is in use by another process")

// Synchronous is the durability of the write-ahead log.
type Synchronous int

// Durability modes (PRAGMA synchronous).
const (
	// SynchronousFull makes every commit durable on power loss. It is the default.
	SynchronousFull Synchronous = 2
	// SynchronousNormal is the documented weaker opt-in: a power loss may lose
	// the newest commits, never corrupt the file.
	SynchronousNormal Synchronous = 1
)

const (
	defaultBusyTimeout = 5 * time.Second
	lockSuffix         = ".lock"
	closeTimeout       = 5 * time.Second
)

// Options configure [Open].
type Options struct {
	// Path is the database file. Its directory must exist. The lock file is
	// Path plus ".lock".
	Path string
	// BinaryVersion is the semantic version of this binary, recorded as the
	// compatibility floor of migrations that declare "self" and compared with
	// the floor of the schema.
	BinaryVersion string
	// Clock supplies the instants the adapter writes; the system clock when nil.
	Clock clock.Clock
	// Synchronous is the durability mode; [SynchronousFull] when zero.
	Synchronous Synchronous
	// PoisonThreshold is the number of abandonments after which a token is
	// suspended as poison, counting the leases released when the process starts;
	// [DefaultPoisonThreshold] when zero.
	PoisonThreshold int64
	// Readers is the size of the read connection pool; GOMAXPROCS when zero.
	Readers int
	// BusyTimeout bounds the wait for a locked database; 5 s when zero.
	BusyTimeout time.Duration
	// Migrations overrides the embedded migrations (tests); Dir names the
	// directory inside it ("." when empty).
	Migrations    fs.FS
	MigrationsDir string
}

// Store is an open SQLite database. It is safe for concurrent use.
type Store struct {
	opts    Options
	clock   clock.Clock
	lock    *processLock
	writeDB *sql.DB
	readDB  *sql.DB
	writer  *writer
	reads   *sqlcgen.Queries

	// mu orders Close after the operations in flight: every operation holds a
	// read lock for its whole duration, Close takes the write lock.
	mu     sync.RWMutex
	closed bool
}

// Open takes the process lock, opens the writer and the readers, applies the
// pending migrations and returns the store. It refuses to start when another
// process holds the file ([ErrLocked]), when the schema requires a newer binary
// or when an applied migration was edited.
func Open(ctx context.Context, opts Options) (*Store, error) {
	if opts.Path == "" {
		return nil, errors.New("open sqlite store: Path is required")
	}
	if opts.Clock == nil {
		opts.Clock = clock.System{}
	}
	if opts.Synchronous == 0 {
		opts.Synchronous = SynchronousFull
	}
	if opts.PoisonThreshold <= 0 {
		opts.PoisonThreshold = DefaultPoisonThreshold
	}
	if opts.Readers <= 0 {
		opts.Readers = runtime.GOMAXPROCS(0)
	}
	if opts.BusyTimeout <= 0 {
		opts.BusyTimeout = defaultBusyTimeout
	}
	lock, err := acquireLock(opts.Path + lockSuffix)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store %s: %w", opts.Path, err)
	}
	s := &Store{opts: opts, clock: opts.Clock, lock: lock}
	if err := s.open(ctx); err != nil {
		_ = s.closeResources(ctx)
		return nil, fmt.Errorf("open sqlite store %s: %w", opts.Path, err)
	}
	return s, nil
}

func (s *Store) open(ctx context.Context) error {
	writeDB, err := sql.Open("sqlite", DSN(s.opts.Path, RoleWriter, s.opts))
	if err != nil {
		return fmt.Errorf("open writer: %w", err)
	}
	s.writeDB = writeDB
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)
	writeDB.SetConnMaxLifetime(0)
	if err := writeDB.PingContext(ctx); err != nil {
		return fmt.Errorf("connect writer: %w", err)
	}
	if err := s.migrate(ctx); err != nil {
		return err
	}
	readDB, err := sql.Open("sqlite", DSN(s.opts.Path, RoleReader, s.opts))
	if err != nil {
		return fmt.Errorf("open readers: %w", err)
	}
	s.readDB = readDB
	readDB.SetMaxOpenConns(s.opts.Readers)
	readDB.SetMaxIdleConns(s.opts.Readers)
	if err := readDB.PingContext(ctx); err != nil {
		return fmt.Errorf("connect readers: %w", err)
	}
	s.reads = sqlcgen.New(readDB)
	s.writer = newWriter(ctx, writeDB)
	// A standalone process is the only holder of the file: whatever a crashed predecessor left
	// leased belongs to nobody (ADR-0013 section 9).
	var tokenTable int
	if err := s.writeDB.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'token'`).Scan(&tokenTable); err != nil {
		return fmt.Errorf("look for the token table: %w", err)
	}
	if tokenTable == 0 {
		return nil // migrations without the execution tables (tests of the runner)
	}
	if _, err := s.releaseAllLeases(ctx); err != nil {
		return fmt.Errorf("release the leases of a previous process: %w", err)
	}
	return nil
}

// Role says what a connection is for.
type Role string

// Connection roles.
const (
	RoleWriter Role = "writer"
	RoleReader Role = "reader"
)

// DSN returns the driver connection string of a role. Every pragma goes
// through the driver's per-connection parameters, so a reconnect inherits it
// (orcon-db CN-2). The writer begins every transaction with BEGIN IMMEDIATE.
func DSN(path string, role Role, opts Options) string {
	busy := opts.BusyTimeout
	if busy <= 0 {
		busy = defaultBusyTimeout
	}
	sync := opts.Synchronous
	if sync == 0 {
		sync = SynchronousFull
	}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout("+strconv.FormatInt(busy.Milliseconds(), 10)+")")
	q.Add("_pragma", "trusted_schema(0)")
	switch role {
	case RoleWriter:
		q.Add("_pragma", "journal_mode(wal)")
		q.Add("_pragma", "synchronous("+strconv.Itoa(int(sync))+")")
		q.Add("_pragma", "journal_size_limit(67108864)")
		q.Set("_txlock", "immediate")
	case RoleReader:
		q.Add("_pragma", "query_only(1)")
	}
	return "file:" + escapePath(path) + "?" + q.Encode()
}

// escapePath percent-encodes the characters that would end the path of a
// "file:" URI.
func escapePath(p string) string {
	r := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23")
	return r.Replace(p)
}

// Close stops the writer, closes the connections and releases the process
// lock. It is safe to call twice.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.closeResources(context.Background())
}

func (s *Store) closeResources(ctx context.Context) error {
	var errs []error
	if s.writer != nil {
		s.writer.close()
		s.writer = nil
	}
	if s.readDB != nil {
		errs = append(errs, s.readDB.Close())
		s.readDB = nil
	}
	if s.writeDB != nil {
		if s.reads != nil {
			// Let the planner refresh its statistics for the next start (CN-5).
			octx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
			_, err := s.writeDB.ExecContext(octx, "PRAGMA optimize")
			cancel()
			if err != nil {
				errs = append(errs, fmt.Errorf("pragma optimize: %w", err))
			}
		}
		errs = append(errs, s.writeDB.Close())
		s.writeDB = nil
	}
	if s.lock != nil {
		errs = append(errs, s.lock.release())
		s.lock = nil
	}
	return errors.Join(errs...)
}

// Writer returns the single write connection pool, for the checks that inspect
// connections (internal/adapter/storetest/dbrules). Application code uses the
// port methods. The pool is valid until Close.
func (s *Store) Writer() *sql.DB { return s.writeDB }

// Reader returns the read-only connection pool, for the same checks.
func (s *Store) Reader() *sql.DB { return s.readDB }

// write runs fn in the writer under the group commit.
func (s *Store) write(ctx context.Context, op string, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("%s: %w", op, ErrClosed)
	}
	err := s.writer.submit(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return fn(ctx, sqlcgen.New(tx))
	})
	return annotate(op, err)
}

// read runs fn on a read connection.
func (s *Store) read(ctx context.Context, op string, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("%s: %w", op, ErrClosed)
	}
	return annotate(op, fn(ctx, s.reads))
}

// annotate maps a driver error once; errors that already wrap an area error
// only gain the operation name.
func annotate(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, flow.ErrNotFound) || errors.Is(err, flow.ErrConflict) || errors.Is(err, flow.ErrReference) ||
		errors.Is(err, flow.ErrInvalidRecord) || errors.Is(err, flow.ErrDraftConflict) || errors.Is(err, ErrClosed) {
		return fmt.Errorf("%s: %w", op, err)
	}
	return mapError(op, err)
}
