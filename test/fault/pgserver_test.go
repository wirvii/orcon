// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package fault_test

// The PostgreSQL half of the battery needs a server; see the same helper in
// internal/adapter/postgres/pgtest_test.go for the environment variables. The
// container is started here, in a test file, and handed to storetest, which creates
// a database per case. internal/adapter/storetest has the same starter in its own
// test files.

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/wirvii/orcon/internal/adapter/storetest"
)

const defaultImage = "postgres:17"

// runWithPostgres starts the server when the PostgreSQL tests are on, runs the tests and
// stops the server. Use it as the body of TestMain; it returns the exit code of m.Run.
func runWithPostgres(m *testing.M) int {
	if os.Getenv("ORCON_TEST_POSTGRES") == "1" {
		storetest.EnablePostgres()
	}
	if !storetest.PostgresEnabled() {
		return m.Run()
	}
	stop := func() {}
	if dsn := os.Getenv("ORCON_TEST_POSTGRES_DSN"); dsn != "" {
		storetest.UsePostgresServer(dsn, nil)
		announce(dsn)
	} else {
		dsn, terminate, err := startContainer(os.Getenv("ORCON_TEST_POSTGRES_IMAGE"))
		storetest.UsePostgresServer(dsn, err)
		if terminate != nil {
			stop = terminate
		}
	}
	code := m.Run()
	stop()
	return code
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

func announce(string) {
	fmt.Fprintln(os.Stderr, "pgserver: using the existing PostgreSQL server named by ORCON_TEST_POSTGRES_DSN; this is a local convenience and not container evidence")
}
