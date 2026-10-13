// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

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

// The PostgreSQL tests need a server. With ORCON_TEST_POSTGRES=1 the test
// binary starts one container of ORCON_TEST_POSTGRES_IMAGE (default
// postgres:17; CI and make test-postgres pin 14 and 17 by digest) through
// testcontainers and creates one database per test. ORCON_TEST_POSTGRES_DSN
// points the tests at an existing superuser connection instead (a developer
// convenience for machines without a container runtime). Without
// ORCON_TEST_POSTGRES the PostgreSQL tests are skipped and say so.

const defaultPostgresImage = "postgres:17"

var pgServer struct {
	enabled  bool
	adminDSN string
	err      error
}

func TestMain(m *testing.M) {
	if os.Getenv("ORCON_TEST_POSTGRES") != "1" {
		m.Run()
		return
	}
	pgServer.enabled = true
	stop := func() {}
	if dsn := os.Getenv("ORCON_TEST_POSTGRES_DSN"); dsn != "" {
		pgServer.adminDSN = dsn
		announceExistingServer(dsn)
	} else {
		dsn, terminate, err := startPostgresContainer(os.Getenv("ORCON_TEST_POSTGRES_IMAGE"))
		pgServer.adminDSN, pgServer.err = dsn, err
		if terminate != nil {
			stop = terminate
		}
	}
	m.Run()
	stop()
}

func startPostgresContainer(image string) (dsn string, terminate func(), err error) {
	if image == "" {
		image = defaultPostgresImage
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
			fmt.Fprintf(os.Stderr, "dbrules: terminate %s: %v\n", image, err)
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

// requirePostgres skips the test when PostgreSQL tests are not enabled and fails
// it when they are enabled but the server could not start.
func requirePostgres(t *testing.T) {
	t.Helper()
	if !pgServer.enabled {
		t.Skip("PostgreSQL tests are off: set ORCON_TEST_POSTGRES=1 (make test-postgres)")
	}
	if pgServer.err != nil {
		t.Fatalf("PostgreSQL is enabled but unavailable: %v", pgServer.err)
	}
}

// newPostgresDatabase creates a database with a unique name, drops it when the
// test ends, and returns its connection string.
func newPostgresDatabase(t *testing.T) string {
	t.Helper()
	requirePostgres(t)
	ctx := t.Context()
	admin, err := pgx.Connect(ctx, pgServer.adminDSN)
	if err != nil {
		t.Fatalf("connect to the PostgreSQL server: %v", err)
	}
	defer admin.Close(context.WithoutCancel(ctx)) //nolint:errcheck // closing the admin connection cannot fail a test
	ensureRoles(t, admin)
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	name := "dbrules_" + hex.EncodeToString(suffix[:])
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE template0"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.WithoutCancel(ctx)
		c, err := pgx.Connect(cleanup, pgServer.adminDSN)
		if err != nil {
			t.Errorf("drop %s: %v", name, err)
			return
		}
		defer c.Close(cleanup) //nolint:errcheck // closing the admin connection cannot fail a test
		if _, err := c.Exec(cleanup, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop %s: %v", name, err)
		}
	})
	u, err := url.Parse(pgServer.adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

// ensureRoles creates the cluster-level roles of the rulebook (NM-13, TI-5)
// when they are missing.
func ensureRoles(t *testing.T, admin *pgx.Conn) {
	t.Helper()
	const ddl = `
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'orcon_owner') THEN
    CREATE ROLE orcon_owner NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'orcon_application') THEN
    CREATE ROLE orcon_application NOLOGIN NOSUPERUSER NOBYPASSRLS;
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'orcon_engine') THEN
    CREATE ROLE orcon_engine NOLOGIN NOBYPASSRLS;
  END IF;
END $$`
	if _, err := admin.Exec(t.Context(), ddl, pgx.QueryExecModeSimpleProtocol); err != nil {
		t.Fatalf("create roles: %v", err)
	}
}

// connectPostgres opens a connection to dsn that closes with the test.
func connectPostgres(t *testing.T, dsn string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.WithoutCancel(t.Context())); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	return conn
}

// execPostgres runs SQL text with several statements.
func execPostgres(t *testing.T, conn *pgx.Conn, statements string) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), statements, pgx.QueryExecModeSimpleProtocol); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

// announceExistingServer prints one line saying that an existing server is in
// use, with its address and version and never its password. Results from such a
// run are not evidence for I8 or T5, which run against containers.
func announceExistingServer(dsn string) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dbrules: ORCON_TEST_POSTGRES_DSN is set but cannot be parsed")
		return
	}
	version := "unknown version"
	if conn, err := pgx.ConnectConfig(context.Background(), cfg); err == nil {
		_ = conn.QueryRow(context.Background(), "SHOW server_version").Scan(&version)
		_ = conn.Close(context.Background())
	}
	fmt.Fprintf(os.Stderr, "dbrules: using the existing PostgreSQL server %s:%d (%s) from ORCON_TEST_POSTGRES_DSN; this is a local convenience and not container evidence\n", cfg.Host, cfg.Port, version)
}
