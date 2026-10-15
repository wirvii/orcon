// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package postgres is the PostgreSQL storage adapter (ADR-0015 section 6):
// pgx/v5 with READ COMMITTED transactions, the session settings of orcon-db
// CN-7 and CN-8, row-level security with the organization context set per
// transaction (ADR-0046), automatic migrations under an advisory lock and the
// queries of the area ports compiled by sqlc. It implements the ports of the
// flow area and supports PostgreSQL 14 and later.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// ApplicationRole is the runtime role every pooled connection switches to
// (orcon-db CN-8, TI-5).
const ApplicationRole = "orcon_application"

// ErrClosed is returned by operations on a closed store.
var ErrClosed = flow.ErrClosed

// Options configure [Open].
type Options struct {
	// DSN connects the runtime pool. The role it logs in as must be allowed to
	// SET ROLE orcon_application (a superuser, or a member of that role).
	DSN string
	// MigrationDSN connects the migration session, which runs as the schema
	// owner. It defaults to DSN.
	MigrationDSN string
	// BinaryVersion is the semantic version of this binary (see the sqlite
	// adapter for how it is used).
	BinaryVersion string
	// ApplicationName is the application_name of the sessions, "orcon-" plus
	// the process role; "orcon" when empty.
	ApplicationName string
	// Clock supplies the instants the adapter writes; the system clock when nil.
	Clock clock.Clock
	// MaxConns bounds the pool; the pgx default when zero.
	MaxConns int32
	// StatementTimeout bounds one statement of the request plane; 30 s when zero.
	StatementTimeout time.Duration
	// Migrations overrides the embedded migrations (tests); Dir names the
	// directory inside it ("." when empty).
	Migrations    fs.FS
	MigrationsDir string
}

// Store is an open PostgreSQL database. It is safe for concurrent use.
type Store struct {
	opts  Options
	clock clock.Clock
	pool  *pgxpool.Pool

	// rule routes the events the engine functions append to the outbox.
	rule atomic.Pointer[store.OutboxRule]

	// mu orders Close after the operations in flight: every operation holds a
	// read lock for its whole duration, Close takes the write lock.
	mu     sync.RWMutex
	closed bool
}

// Open applies the pending migrations and opens the runtime pool. It refuses
// to start when the schema requires a newer binary or when an applied
// migration was edited.
func Open(ctx context.Context, opts Options) (*Store, error) {
	if opts.DSN == "" {
		return nil, errors.New("open postgres store: DSN is required")
	}
	if opts.Clock == nil {
		opts.Clock = clock.System{}
	}
	migrationDSN := opts.MigrationDSN
	if migrationDSN == "" {
		migrationDSN = opts.DSN
	}
	fsys, dir := opts.Migrations, opts.MigrationsDir
	if fsys == nil {
		fsys, dir = Migrations()
	}
	conn, err := pgx.Connect(ctx, migrationDSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres store: connect for migrations: %w", err)
	}
	err = Migrate(ctx, conn, MigrateOptions{FS: fsys, Dir: dir, BinaryVersion: opts.BinaryVersion, Clock: opts.Clock})
	if cerr := conn.Close(context.WithoutCancel(ctx)); cerr != nil && err == nil {
		err = fmt.Errorf("close migration connection: %w", cerr)
	}
	if err != nil {
		return nil, fmt.Errorf("open postgres store: %w", err)
	}
	pool, err := NewPool(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("open postgres store: %w", err)
	}
	return &Store{opts: opts, clock: opts.Clock, pool: pool}, nil
}

// NewPool builds the runtime pool with the session settings of orcon-db CN-7
// and CN-8: TimeZone UTC, application_name, statement, lock and idle-in-
// transaction timeouts, and SET ROLE orcon_application on every connection.
func NewPool(ctx context.Context, opts Options) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse DSN: %w", err)
	}
	statement := opts.StatementTimeout
	if statement <= 0 {
		statement = 30 * time.Second
	}
	app := opts.ApplicationName
	if app == "" {
		app = "orcon"
	}
	params := cfg.ConnConfig.RuntimeParams
	params["TimeZone"] = "UTC"
	params["application_name"] = app
	params["statement_timeout"] = strconv.FormatInt(statement.Milliseconds(), 10)
	params["idle_in_transaction_session_timeout"] = "10000"
	params["lock_timeout"] = "5000"
	params["default_transaction_isolation"] = "read committed"
	if opts.MaxConns > 0 {
		cfg.MaxConns = opts.MaxConns
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, "SET ROLE "+ApplicationRole); err != nil {
			return fmt.Errorf("set role %s: %w", ApplicationRole, err)
		}
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect pool: %w", err)
	}
	return pool, nil
}

// Close closes the pool.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.pool.Close()
	return nil
}

// Pool returns the runtime pool, for the checks that inspect sessions
// (internal/adapter/storetest/dbrules). Application code uses the port methods.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// inOrg runs fn in a READ COMMITTED transaction whose organization context is
// org (orcon-db TI-6: a transaction-local setting, never a session one).
func (s *Store) inOrg(ctx context.Context, op string, org fmt.Stringer, readOnly bool, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	return s.inOrgIso(ctx, op, org, pgx.ReadCommitted, readOnly, fn)
}

// inOrgIso is inOrg at an isolation level of the caller's choosing: a read
// that must see one snapshot of several statements asks for REPEATABLE READ.
func (s *Store) inOrgIso(ctx context.Context, op string, org fmt.Stringer, iso pgx.TxIsoLevel, readOnly bool, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("%s: %w", op, ErrClosed)
	}
	mode := pgx.ReadWrite
	if readOnly {
		mode = pgx.ReadOnly
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: iso, AccessMode: mode})
	if err != nil {
		return mapError(op, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	if err := assertFreshSession(ctx, tx); err != nil {
		return mapError(op, err)
	}
	if _, err := tx.Exec(ctx, "SELECT set_config('orcon.org_id', $1, true)", org.String()); err != nil {
		return mapError(op, err)
	}
	if err := fn(ctx, sqlcgen.New(tx)); err != nil {
		return mapError(op, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return mapError(op, err)
	}
	committed = true
	return nil
}

// inScope is [Store.inOrg] for a caller-supplied scope. The zero scope scopes to
// nothing and is refused before a transaction starts (orcon-db TI-2).
func (s *Store) inScope(ctx context.Context, op string, org orgscope.Org, readOnly bool, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	if org.IsZero() {
		return fmt.Errorf("%s: %w", op, identity.ErrNoScope)
	}
	return s.inOrg(ctx, op, org, readOnly, fn)
}

// inScopeRecord is [Store.inScope] for a write of a record that names its
// organization: the record must belong to the organization of the scope.
func (s *Store) inScopeRecord(ctx context.Context, op string, org orgscope.Org, recordOrg identity.OrgID, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	if org.IsZero() {
		return fmt.Errorf("%s: %w", op, identity.ErrNoScope)
	}
	if !recordOrg.Matches(org) {
		return fmt.Errorf("%s: %w", op, identity.ErrScopeMismatch)
	}
	return s.inOrg(ctx, op, org, false, fn)
}

// instance runs fn for statements on instance tables, which row-level security
// does not scope.
func (s *Store) instance(ctx context.Context, op string, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("%s: %w", op, ErrClosed)
	}
	return mapError(op, fn(ctx, sqlcgen.New(s.pool)))
}

// instanceTx runs fn in one transaction for statements on instance tables,
// which row-level security does not scope but which must commit together.
func (s *Store) instanceTx(ctx context.Context, op string, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return fmt.Errorf("%s: %w", op, ErrClosed)
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: pgx.ReadWrite})
	if err != nil {
		return mapError(op, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	if err := fn(ctx, sqlcgen.New(tx)); err != nil {
		return mapError(op, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return mapError(op, err)
	}
	committed = true
	return nil
}

// ErrSession means a pooled connection is not in the state every transaction
// must start from; the transaction is not run.
var ErrSession = errors.New("postgres session is not in the expected state")

// assertFreshSession checks, at the start of each transaction, that the
// connection runs as orcon_application and that no organization context is left
// over from an earlier transaction (orcon-db CN-8, TI-6): a leaked context would
// let a transaction read another organization's rows.
func assertFreshSession(ctx context.Context, tx pgx.Tx) error {
	var user, orgContext string
	if err := tx.QueryRow(ctx, "SELECT current_user::text, coalesce(current_setting('orcon.org_id', true), '')").Scan(&user, &orgContext); err != nil {
		return err
	}
	if user != ApplicationRole {
		return fmt.Errorf("%w: the role is %q, want %q", ErrSession, user, ApplicationRole)
	}
	if orgContext != "" {
		return fmt.Errorf("%w: the organization context is %q at the start of a transaction, want empty", ErrSession, orgContext)
	}
	return nil
}

// RouteEvents implements [store.EventRouter].
func (s *Store) RouteEvents(rule store.OutboxRule) {
	if rule == nil {
		s.rule.Store(nil)
		return
	}
	s.rule.Store(&rule)
}

// outboxRule returns the rule in force, nil when none is.
func (s *Store) outboxRule() store.OutboxRule {
	if r := s.rule.Load(); r != nil {
		return *r
	}
	return nil
}
