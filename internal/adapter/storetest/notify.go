// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// listenerApplication is the application_name of the connection that listens for the
// runnable notices (internal/adapter/postgres/notify.go).
const listenerApplication = "orcon-notice"

// EndNoticeListeners waits until the store's connection that listens for the
// runnable notices is up, then ends it, as a network fault or a restart of the
// server would. The engines of the store then claim by polling until they listen
// again (SPEC-001 E-REC-3). The connection ends as the server's superuser, because
// the pooled connections run as the application role, which may not end other
// sessions. A store without notices (SQLite) has nothing to end.
func EndNoticeListeners(tb testing.TB, s Store) {
	tb.Helper()
	withPool, ok := s.(interface{ Pool() *pgxpool.Pool })
	if !ok {
		tb.Skip("the store has no notice connection: SQLite claims by polling and the in-process wake-up")
	}
	conn, err := pgx.ConnectConfig(tb.Context(), withPool.Pool().Config().ConnConfig.Copy())
	if err != nil {
		tb.Fatalf("connect as the server superuser: %v", err)
	}
	tb.Cleanup(func() { _ = conn.Close(tb.Context()) })
	deadline := time.Now().Add(5 * time.Second)
	for {
		var n int
		if err := conn.QueryRow(tb.Context(), "SELECT count(*) FROM pg_stat_activity WHERE application_name = $1", listenerApplication).Scan(&n); err != nil {
			tb.Fatalf("look for the listening connection: %v", err)
		}
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			tb.Fatal("no listening connection within 5 s")
		}
		<-time.After(2 * time.Millisecond)
	}
	if _, err := conn.Exec(tb.Context(), "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name = $1 AND pid <> pg_backend_pid()", listenerApplication); err != nil {
		tb.Fatalf("terminate the listening connection: %v", err)
	}
}
