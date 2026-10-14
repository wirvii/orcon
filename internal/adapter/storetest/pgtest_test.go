// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/storetest"
)

// The PostgreSQL server of the suites lives in the storetest package, so that
// the engine tests of other packages share it.

func enablePostgres()              { storetest.EnablePostgres() }
func postgresEnabled() bool        { return storetest.PostgresEnabled() }
func runWithPostgres(m *testing.M) { storetest.RunWithPostgres(m) }

func newPostgresDatabase(tb testing.TB) string {
	tb.Helper()
	return storetest.NewPostgresDatabase(tb)
}
