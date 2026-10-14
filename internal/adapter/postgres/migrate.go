// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/migration"
)

// embeddedMigrations holds the migration files of this dialect, with the same
// numbering and names as the SQLite directory (orcon-db MG-1).
//
//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Migrations returns the embedded migration files and their directory.
func Migrations() (fs.FS, string) { return embeddedMigrations, "migrations" }

// migrationLockKey is the advisory lock that serializes the migrations of all
// processes sharing the database (ADR-0015 section 4). It is the first eight
// bytes of the SHA-256 of "orcon.migrations".
const migrationLockKey int64 = 0x6f72636f6e6d6967

// MigrateOptions configure [Migrate].
type MigrateOptions struct {
	// FS and Dir locate the migration files; the embedded ones when FS is nil.
	FS  fs.FS
	Dir string
	// BinaryVersion is the version of the binary applying them.
	BinaryVersion string
	// Clock supplies applied_at; the system clock when nil.
	Clock clock.Clock
	// UpTo applies migrations only up to this number; all when zero.
	UpTo int
}

// Migrate applies the pending migrations on conn, which must log in as the
// schema owner. It takes the advisory migration lock for the duration, checks
// the schema first (a binary below the effective compatibility floor, or a
// changed checksum, fails before anything is applied) and applies each
// migration in one transaction together with its schema_migration row. The
// session sets lock_timeout to 5 s so a migration fails instead of queueing
// behind traffic (orcon-db MG-9).
func Migrate(ctx context.Context, conn *pgx.Conn, opts MigrateOptions) error {
	if opts.FS == nil {
		opts.FS, opts.Dir = Migrations()
	}
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.Clock == nil {
		opts.Clock = clock.System{}
	}
	embedded, err := migration.Load(opts.FS, opts.Dir)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("migrate: take the migration lock: %w", err)
	}
	defer func() {
		// The lock is session-level: it also ends with the session, so a failed unlock only delays.
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()
	if _, err := conn.Exec(ctx, "SET lock_timeout = '5s'"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	applied, err := readApplied(ctx, conn)
	if err != nil {
		return fmt.Errorf("migrate: read schema_migration: %w", err)
	}
	plan, err := migration.Check(opts.BinaryVersion, applied, embedded)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	for _, m := range plan.Pending {
		if opts.UpTo > 0 && m.Version > opts.UpTo {
			break
		}
		if err := apply(ctx, conn, m, opts); err != nil {
			return fmt.Errorf("migrate: apply %s: %w", m.Name, err)
		}
	}
	return nil
}

func readApplied(ctx context.Context, conn *pgx.Conn) ([]migration.Applied, error) {
	var regclass *string
	if err := conn.QueryRow(ctx, `SELECT to_regclass('public.schema_migration')::text`).Scan(&regclass); err != nil {
		return nil, err
	}
	if regclass == nil {
		return nil, nil
	}
	rows, err := conn.Query(ctx, `SELECT version, checksum, binary_version, compatibility_floor FROM schema_migration ORDER BY version LIMIT 100000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []migration.Applied
	for rows.Next() {
		var a migration.Applied
		if err := rows.Scan(&a.Version, &a.Checksum, &a.BinaryVersion, &a.Floor); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

const insertApplied = `INSERT INTO schema_migration (version, checksum, applied_at, binary_version, compatibility_floor, created_at) VALUES ($1, $2, $3, $4, $5, $3)`

func apply(ctx context.Context, conn *pgx.Conn, m migration.Migration, opts MigrateOptions) (err error) {
	now := opts.Clock.Now().UTC().Truncate(time.Microsecond)
	record := func(exec func(ctx context.Context, sql string, args ...any) error) error {
		return exec(ctx, insertApplied, m.Version, m.Checksum, now, opts.BinaryVersion, m.RecordedFloor(opts.BinaryVersion))
	}
	if m.NoTransaction {
		// One CREATE INDEX CONCURRENTLY cannot run inside a transaction (MG-9); the
		// row is written after it, and a crash in between is repaired by the
		// idempotent IF NOT EXISTS on the next start.
		//orcon:dynamic-sql the migration runner executes the embedded migration files and the bookkeeping statement it is handed
		if _, err := conn.Exec(ctx, m.SQL, pgx.QueryExecModeSimpleProtocol); err != nil {
			return err
		}
		return record(func(ctx context.Context, sql string, args ...any) error {
			//orcon:dynamic-sql the migration runner executes the embedded migration files and the bookkeeping statement it is handed
			_, err := conn.Exec(ctx, sql, args...)
			return err
		})
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	// err is the named result: the deferred rollback joins its error to it.
	committed := false
	defer func() {
		if !committed {
			if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
				err = errors.Join(err, rerr)
			}
		}
	}()
	// A migration file holds several statements: the simple protocol runs them in one round trip.
	//orcon:dynamic-sql the migration runner executes the embedded migration files and the bookkeeping statement it is handed
	if _, err := tx.Exec(ctx, m.SQL, pgx.QueryExecModeSimpleProtocol); err != nil {
		return err
	}
	if err := record(func(ctx context.Context, sql string, args ...any) error {
		//orcon:dynamic-sql the migration runner executes the embedded migration files and the bookkeeping statement it is handed
		_, err := tx.Exec(ctx, sql, args...)
		return err
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}
