// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/storetest/dbrules"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// TestPostgresSessionSettings checks every connection of the pool, not one: the
// settings come from the connection parameters and the AfterConnect hook, and a
// pool opens each connection separately.
func TestPostgresSessionSettings(t *testing.T) {
	const conns = 4
	s := open(t, newPostgresDatabase(t), func(o *postgres.Options) { o.MaxConns = conns })
	ctx := t.Context()
	held := make([]*pgxpool.Conn, 0, conns)
	txs := make([]pgx.Tx, 0, conns)
	for range conns {
		c, err := s.Pool().Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, c)
		tx, err := c.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		txs = append(txs, tx)
	}
	pids := map[uint32]bool{}
	for i, tx := range txs {
		pids[tx.Conn().PgConn().PID()] = true
		for _, f := range dbrules.CheckPostgresSession(ctx, tx) {
			t.Errorf("connection %d: %s", i, f)
		}
	}
	if len(pids) != conns {
		t.Fatalf("the checks ran on %d distinct connections, want %d", len(pids), conns)
	}
	for _, tx := range txs {
		_ = tx.Rollback(context.WithoutCancel(ctx))
	}
	for _, c := range held {
		c.Release()
	}
}

// TestLeakedOrganizationContextIsRefused plants a session-level organization on
// the only pooled connection: the next transaction must not run (TI-6).
func TestLeakedOrganizationContextIsRefused(t *testing.T) {
	s := open(t, newPostgresDatabase(t), func(o *postgres.Options) { o.MaxConns = 1 })
	ctx := t.Context()
	if _, err := s.Pool().Exec(ctx, "SELECT set_config('orcon.org_id', '00000000-0000-7000-8000-000000000001', false)"); err != nil {
		t.Fatal(err)
	}
	scope, err := issue.New(id.NewSequence().New())
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.GetOrganization(ctx, scope)
	if !errors.Is(err, postgres.ErrSession) {
		t.Fatalf("error %v, want ErrSession", err)
	}
}

// loginDSN returns the DSN of a new login role that is not a superuser, created
// on the server of dsn, and drops the role when the test ends. It is a member
// of orcon_application when member is true.
func loginDSN(t *testing.T, dsn string, member bool) string {
	t.Helper()
	admin := connect(t, dsn)
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	role := "orcon_login_" + hex.EncodeToString(suffix[:])
	const password = "login-password"
	ctx := t.Context()
	//orcon:dynamic-sql test helper: names a throwaway login role
	if _, err := admin.Exec(ctx, "CREATE ROLE "+role+" LOGIN NOSUPERUSER NOCREATEROLE NOCREATEDB NOBYPASSRLS PASSWORD '"+password+"'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.WithoutCancel(ctx)
		c, err := pgx.Connect(cleanup, dsn)
		if err != nil {
			t.Errorf("drop role %s: %v", role, err)
			return
		}
		defer c.Close(cleanup) //nolint:errcheck // closing the admin connection cannot fail a test
		//orcon:dynamic-sql test helper: names a throwaway login role
		if _, err := c.Exec(cleanup, "DROP ROLE IF EXISTS "+role); err != nil {
			t.Errorf("drop role %s: %v", role, err)
		}
	})
	if member {
		//orcon:dynamic-sql test helper: names a throwaway login role
		if _, err := admin.Exec(ctx, "GRANT orcon_application TO "+role); err != nil {
			t.Fatal(err)
		}
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(role, password)
	return u.String()
}

// TestNonSuperuserLogin runs the store the way production does: migrations by
// the schema owner, the runtime pool by a login role that is only a member of
// orcon_application. Row-level security then applies for real.
func TestNonSuperuserLogin(t *testing.T) {
	dsn := newPostgresDatabase(t)
	migrator := open(t, dsn)
	if err := migrator.Close(); err != nil {
		t.Fatal(err)
	}
	runtime := loginDSN(t, dsn, true)
	s, err := postgres.Open(t.Context(), postgres.Options{DSN: runtime, MigrationDSN: dsn, BinaryVersion: binaryVersion})
	if err != nil {
		t.Fatalf("open as a login role: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := t.Context()

	ids := id.NewSequence()
	now := time.Now()
	org := identity.OrgID(ids.New())
	if err := s.CreateOrganization(ctx, identity.OrgRecord{ID: org, Name: "A", Slug: "a", Status: identity.OrgActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("create organization: %v", err)
	}
	project := identity.ProjectID(ids.New())
	scope, err := issue.New(id.UUID(org))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateProject(ctx, scope, identity.ProjectRecord{ID: project, OrgID: org, Handle: "p", Name: "P", Slug: "p", IsProtected: true, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := s.GetProject(ctx, scope, project); err != nil {
		t.Fatalf("get project: %v", err)
	}

	// Without an organization context the runtime role sees no project row.
	tx, err := s.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck // read-only transaction
	for _, f := range dbrules.CheckPostgresSession(ctx, tx) {
		t.Errorf("%s", f)
	}
	var seen int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM project").Scan(&seen); err != nil || seen != 0 {
		t.Fatalf("project rows visible without an organization context: %d, %v", seen, err)
	}
	var privileged bool
	if err := tx.QueryRow(ctx, "SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = session_user").Scan(&privileged); err != nil || privileged {
		t.Fatalf("the login role must not be a superuser or bypass row-level security: %v, %v", privileged, err)
	}
}

// TestLoginWithoutTheApplicationRoleCannotOpen: a login role that may not
// SET ROLE orcon_application never gets a usable pool.
func TestLoginWithoutTheApplicationRoleCannotOpen(t *testing.T) {
	dsn := newPostgresDatabase(t)
	migrator := open(t, dsn)
	if err := migrator.Close(); err != nil {
		t.Fatal(err)
	}
	runtime := loginDSN(t, dsn, false)
	if s, err := postgres.Open(t.Context(), postgres.Options{DSN: runtime, MigrationDSN: dsn, BinaryVersion: binaryVersion}); err == nil {
		_ = s.Close()
		t.Fatal("a login role outside orcon_application opened the store")
	}
}
