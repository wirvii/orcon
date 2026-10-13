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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

// ApplicationRole is the runtime role every pooled connection switches to
// (orcon-db CN-8, TI-5).
const ApplicationRole = "orcon_application"

// ErrClosed is returned by operations on a closed store.
var ErrClosed = errors.New("postgres store is closed")

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
	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
	return nil
}

// Pool returns the runtime pool, for the checks that inspect sessions
// (internal/adapter/storetest/dbrules). Application code uses the port methods.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// inOrg runs fn in a READ COMMITTED transaction whose organization context is
// org (orcon-db TI-6: a transaction-local setting, never a session one).
func (s *Store) inOrg(ctx context.Context, op string, org id.UUID, readOnly bool, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	if s.pool == nil {
		return fmt.Errorf("%s: %w", op, ErrClosed)
	}
	mode := pgx.ReadWrite
	if readOnly {
		mode = pgx.ReadOnly
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted, AccessMode: mode})
	if err != nil {
		return mapError(op, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
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

// instance runs fn for statements on instance tables, which row-level security
// does not scope.
func (s *Store) instance(ctx context.Context, op string, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	if s.pool == nil {
		return fmt.Errorf("%s: %w", op, ErrClosed)
	}
	return mapError(op, fn(ctx, sqlcgen.New(s.pool)))
}

var (
	_ flow.InstanceStore = (*Store)(nil)
	_ flow.ProjectStore  = (*Store)(nil)
	_ flow.Store         = (*Store)(nil)
	_ flow.VersionStore  = (*Store)(nil)
	_ flow.SnapshotStore = (*Store)(nil)
)
