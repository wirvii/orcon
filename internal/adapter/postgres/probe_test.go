// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/postgres"
)

func TestProbeAuthenticatesAndRunsSelectOne(t *testing.T) {
	dsn := newPostgresDatabase(t)
	if err := postgres.Probe(t.Context(), dsn); err != nil {
		t.Fatalf("Probe with good credentials = %v", err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(u.User.Username(), "WRONG-PASSWORD-xyz")
	err = postgres.Probe(t.Context(), u.String())
	if err == nil {
		t.Fatal("a wrong password must fail: a reachable port is not enough")
	}
	if strings.Contains(err.Error(), "WRONG-PASSWORD-xyz") {
		t.Fatalf("the error repeats the password: %v", err)
	}
}

func TestProbeRejectsUnusableConnectionStrings(t *testing.T) {
	t.Parallel()

	if err := postgres.Probe(t.Context(), ""); err == nil {
		t.Error("an empty connection string must fail")
	}
	err := postgres.Probe(t.Context(), "postgres://u:SECRETPW@%zz/db")
	if err == nil || strings.Contains(err.Error(), "SECRETPW") {
		t.Errorf("Probe = %v, want a parse error without the password", err)
	}
	err = postgres.Probe(t.Context(), "postgres://u:SECRETPW@127.0.0.1:1/db?connect_timeout=1")
	if err == nil || strings.Contains(err.Error(), "SECRETPW") {
		t.Errorf("Probe = %v, want a connect error without the password", err)
	}
}
