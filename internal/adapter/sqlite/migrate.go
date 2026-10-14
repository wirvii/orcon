// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"

	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/migration"
)

// embeddedMigrations holds the migration files of this dialect, with the same
// numbering and names as the PostgreSQL directory (orcon-db MG-1).
//
//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// Migrations returns the embedded migration files and their directory.
func Migrations() (fs.FS, string) { return embeddedMigrations, "migrations" }

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

func (s *Store) migrate(ctx context.Context) error {
	fsys, dir := s.opts.Migrations, s.opts.MigrationsDir
	if fsys == nil {
		fsys, dir = Migrations()
	}
	return Migrate(ctx, s.writeDB, MigrateOptions{FS: fsys, Dir: dir, BinaryVersion: s.opts.BinaryVersion, Clock: s.clock})
}

// Migrate applies the pending migrations to db, which must be the single
// write connection of the process. The check against the schema comes first:
// a binary below the effective compatibility floor, or an applied migration
// whose checksum changed, fails before anything is applied (ADR-0015 section
// 4). Each migration runs in one exclusive transaction together with its
// schema_migration row, so the file lock plus BEGIN IMMEDIATE is the migration
// lock.
func Migrate(ctx context.Context, db *sql.DB, opts MigrateOptions) error {
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
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	defer conn.Close() //nolint:errcheck // returning the connection to the pool cannot fail the migration
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

// readApplied returns the rows of schema_migration; a database without the
// table yet has none.
func readApplied(ctx context.Context, conn *sql.Conn) ([]migration.Applied, error) {
	var exists int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'schema_migration'`).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, nil
	}
	rows, err := conn.QueryContext(ctx, `SELECT version, checksum, binary_version, compatibility_floor FROM schema_migration ORDER BY version LIMIT 100000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // the loop reports the error through rows.Err
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

func apply(ctx context.Context, conn *sql.Conn, m migration.Migration, opts MigrateOptions) (err error) {
	rebuild := m.Kind == migration.KindRebuild
	if rebuild {
		// A table rebuild needs foreign keys off for the duration of the transaction
		// (orcon-db MG-8); the pragma is a no-op inside a transaction, so it comes first.
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
			return err
		}
		defer func() {
			if _, ferr := conn.ExecContext(context.WithoutCancel(ctx), "PRAGMA foreign_keys = ON"); ferr != nil {
				err = errors.Join(err, ferr)
			}
		}()
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if _, rerr := conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK"); rerr != nil {
				err = errors.Join(err, rerr)
			}
		}
	}()
	//orcon:dynamic-sql the migration runner executes the embedded migration files
	if _, err := conn.ExecContext(ctx, m.SQL); err != nil {
		return err
	}
	if rebuild {
		if err := checkForeignKeys(ctx, conn); err != nil {
			return err
		}
	}
	now := opts.Clock.Now().UTC().UnixMicro()
	if _, err := conn.ExecContext(ctx,
		`INSERT INTO schema_migration (version, checksum, applied_at, binary_version, compatibility_floor, created_at) VALUES (?1, ?2, ?3, ?4, ?5, ?3)`,
		m.Version, m.Checksum, now, opts.BinaryVersion, m.RecordedFloor(opts.BinaryVersion)); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

func checkForeignKeys(ctx context.Context, conn *sql.Conn) error {
	rows, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck // the loop reports the error through rows.Err
	if rows.Next() {
		return errors.New("foreign_key_check found rows that violate a foreign key after the rebuild")
	}
	return rows.Err()
}
