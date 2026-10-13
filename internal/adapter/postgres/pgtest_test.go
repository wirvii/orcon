// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres_test

// The PostgreSQL tests need a server. With ORCON_TEST_POSTGRES=1 the test binary
// starts one container of ORCON_TEST_POSTGRES_IMAGE (default postgres:17; make
// test-postgres and CI pin 14 and 17 by digest) through testcontainers, creates
// one database per test and terminates only that container at the end.
// ORCON_TEST_POSTGRES_DSN points the tests at an existing superuser connection
// instead, a developer convenience for machines without a container runtime.
// Without either the PostgreSQL tests are skipped and say so.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const defaultImage = "postgres:17"

var server struct {
	enabled  bool
	adminDSN string
	err      error
}

// runWithPostgres starts the server when the PostgreSQL tests are on, runs the tests and
// stops the server. Use it as the body of TestMain.
func runWithPostgres(m *testing.M) {
	if os.Getenv("ORCON_TEST_POSTGRES") == "1" {
		server.enabled = true
	}
	if !server.enabled {
		m.Run()
		return
	}
	stop := func() {}
	if dsn := os.Getenv("ORCON_TEST_POSTGRES_DSN"); dsn != "" {
		server.adminDSN = dsn
		announce(dsn)
	} else {
		dsn, terminate, err := startContainer(os.Getenv("ORCON_TEST_POSTGRES_IMAGE"))
		server.adminDSN, server.err = dsn, err
		if terminate != nil {
			stop = terminate
		}
	}
	m.Run()
	stop()
}

func startContainer(image string) (dsn string, terminate func(), err error) {
	if image == "" {
		image = defaultImage
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c, err := testcontainers.Run(ctx, image,
		testcontainers.WithExposedPorts("5432/tcp"),
		testcontainers.WithEnv(map[string]string{"POSTGRES_PASSWORD": "postgres"}),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(3*time.Minute)),
	)
	if err != nil {
		if c != nil {
			_ = testcontainers.TerminateContainer(c)
		}
		return "", nil, fmt.Errorf("start %s: %w", image, err)
	}
	terminate = func() {
		if err := testcontainers.TerminateContainer(c); err != nil {
			fmt.Fprintf(os.Stderr, "pgserver: terminate %s: %v\n", image, err)
		}
	}
	host, err := c.Host(ctx)
	if err != nil {
		return "", terminate, err
	}
	port, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return "", terminate, err
	}
	return "postgres://postgres:postgres@" + net.JoinHostPort(host, port.Port()) + "/postgres?sslmode=disable", terminate, nil
}

// requirePostgres skips the test when PostgreSQL tests are off and fails it when they
// are on but the server could not start.
func requirePostgres(tb testing.TB) {
	tb.Helper()
	if !server.enabled {
		tb.Skip("PostgreSQL tests are off: set ORCON_TEST_POSTGRES=1 (make test-postgres)")
	}
	if server.err != nil {
		tb.Fatalf("PostgreSQL is enabled but unavailable: %v", server.err)
	}
}

// newPostgresDatabase creates a database with a unique name, drops it when the test
// ends and returns the connection string of a superuser on it. The cluster
// roles of the engine are created by the first migration, not here.
func newPostgresDatabase(tb testing.TB) string {
	tb.Helper()
	requirePostgres(tb)
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
	//orcon:dynamic-sql test helper: names a throwaway database or runs a statement of a rule check
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
		//orcon:dynamic-sql test helper: names a throwaway database or runs a statement of a rule check
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

func announce(dsn string) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pgserver: ORCON_TEST_POSTGRES_DSN is set but cannot be parsed")
		return
	}
	version := "unknown version"
	if conn, err := pgx.ConnectConfig(context.Background(), cfg); err == nil {
		_ = conn.QueryRow(context.Background(), "SHOW server_version").Scan(&version)
		_ = conn.Close(context.Background())
	}
	fmt.Fprintf(os.Stderr, "pgserver: using the existing PostgreSQL server %s:%d (%s) from ORCON_TEST_POSTGRES_DSN; this is a local convenience and not container evidence\n", cfg.Host, cfg.Port, version)
}
