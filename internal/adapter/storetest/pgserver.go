// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

// The PostgreSQL half of the suites needs a server. Starting one (through
// testcontainers) is the job of the test binary that asks for PostgreSQL, in a
// _test.go file, so no production-reachable package imports the container client
// and its dependencies; these helpers hold the server once it is up and create a
// database per test. Two test binaries use them: the contract suite of this package
// and the isolation battery of test/isolation. See internal/adapter/postgres/pgtest_test.go
// for the environment variables.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
)

var server struct {
	enabled  bool
	adminDSN string
	err      error
}

// EnablePostgres turns the PostgreSQL tests on regardless of the environment, for a
// test run that asked for PostgreSQL explicitly (-store=postgres). Call it
// before the server is started.
func EnablePostgres() { server.enabled = true }

// PostgresEnabled reports whether this run has a PostgreSQL server.
func PostgresEnabled() bool { return server.enabled }

// UsePostgresServer records the server the PostgreSQL tests run against: the DSN
// of a superuser on it, or the error that kept it from starting.
func UsePostgresServer(adminDSN string, err error) {
	server.enabled = true
	server.adminDSN, server.err = adminDSN, err
}

// RequirePostgres skips the test when PostgreSQL tests are off and fails it when they
// are on but the server could not start.
func RequirePostgres(tb testing.TB) {
	tb.Helper()
	if !server.enabled {
		tb.Skip("PostgreSQL tests are off: set ORCON_TEST_POSTGRES=1 (make test-postgres)")
	}
	if server.err != nil {
		tb.Fatalf("PostgreSQL is enabled but unavailable: %v", server.err)
	}
}

// NewPostgresDatabase creates a database with a unique name, drops it when the test
// ends and returns the connection string of a superuser on it. The cluster
// roles of the engine are created by the first migration, not here.
func NewPostgresDatabase(tb testing.TB) string {
	tb.Helper()
	RequirePostgres(tb)
	ctx := tb.Context()
	admin, err := pgx.Connect(ctx, server.adminDSN)
	if err != nil {
		tb.Fatalf("connect to the PostgreSQL server: %v", err)
	}
	defer admin.Close(context.WithoutCancel(ctx)) //nolint:errcheck // closing the admin connection cannot fail a test
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		tb.Fatal(err)
	}
	name := "orcon_" + hex.EncodeToString(suffix[:])
	// The name of a database cannot be a parameter; it is generated here from random bytes.
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE template0"); err != nil {
		tb.Fatalf("create database: %v", err)
	}
	tb.Cleanup(func() {
		cleanup := context.WithoutCancel(ctx)
		c, err := pgx.Connect(cleanup, server.adminDSN)
		if err != nil {
			tb.Errorf("drop %s: %v", name, err)
			return
		}
		defer c.Close(cleanup) //nolint:errcheck // closing the admin connection cannot fail a test
		if _, err := c.Exec(cleanup, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			tb.Errorf("drop %s: %v", name, err)
		}
	})
	u, err := url.Parse(server.adminDSN)
	if err != nil {
		tb.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}
