// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/identity"
)

// raw runs statements the ports do not offer, straight against the database of a
// store, to prove what the schema enforces on its own. Statements use "?" for
// parameters; identities are passed through [raw.id]. On PostgreSQL the
// statement runs as the runtime role (orcon_application), so row-level security
// applies and an organization-scoped statement names its organization.
type raw interface {
	// exec runs a statement and returns its error. org is the organization
	// context on PostgreSQL; the zero value runs without one.
	exec(ctx context.Context, org identity.OrgID, query string, args ...any) error
	// count runs a query that returns one integer.
	count(ctx context.Context, org identity.OrgID, query string, args ...any) (int64, error)
	// id converts an identity to the form a parameter takes.
	id(u [16]byte) any
	// instant converts a time to the form a parameter takes.
	instant(t time.Time) any
	dialect() string
}

func rawOf(tb testing.TB, s storetest.Store) raw { //nolint:ireturn // the callers choose a dialect, not a type
	tb.Helper()
	switch v := s.(type) {
	case *sqlite.Store:
		return rawSQLite{v}
	case *postgres.Store:
		return rawPostgres{v.Pool()}
	default:
		tb.Fatalf("no raw access for %T", s)
		return nil
	}
}

type rawSQLite struct{ s *sqlite.Store }

func (rawSQLite) dialect() string   { return "sqlite" }
func (rawSQLite) id(u [16]byte) any { return u[:] }

func (rawSQLite) instant(t time.Time) any { return t.UTC().UnixMicro() }

func (r rawSQLite) exec(ctx context.Context, _ identity.OrgID, query string, args ...any) error {
	_, err := r.s.Writer().ExecContext(ctx, query, args...)
	return err
}

func (r rawSQLite) count(ctx context.Context, _ identity.OrgID, query string, args ...any) (int64, error) {
	var n int64
	err := r.s.Reader().QueryRowContext(ctx, query, args...).Scan(&n)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	return n, nil
}

type rawPostgres struct{ pool *pgxpool.Pool }

func (rawPostgres) dialect() string         { return "postgres" }
func (rawPostgres) instant(t time.Time) any { return t.UTC() }

func (rawPostgres) id(u [16]byte) any {
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// rebind turns "?" into $1, $2, ... .
func rebind(query string) string {
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (r rawPostgres) inTx(ctx context.Context, org identity.OrgID, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // rolled back after a failure, committed otherwise
	if !org.IsZero() {
		if _, err := tx.Exec(ctx, "SELECT set_config('orcon.org_id', $1, true)", org.String()); err != nil {
			return err
		}
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r rawPostgres) exec(ctx context.Context, org identity.OrgID, query string, args ...any) error {
	return r.inTx(ctx, org, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, rebind(query), args...)
		return err
	})
}

func (r rawPostgres) count(ctx context.Context, org identity.OrgID, query string, args ...any) (int64, error) {
	var n int64
	err := r.inTx(ctx, org, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, rebind(query), args...).Scan(&n)
	})
	return n, err
}
