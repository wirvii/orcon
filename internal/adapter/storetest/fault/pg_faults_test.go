// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package fault_test

import (
	"fmt"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution/store"
)

// requirePostgres skips the test unless the -store flag selects PostgreSQL: the rows
// below cut the network to a database or end its connections, which only a server
// can do.
func requirePostgres(t *testing.T) {
	t.Helper()
	kinds, err := storetest.KindsFor(storeFlag())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(kinds, storetest.PostgreSQL) {
		t.Skip("this row needs PostgreSQL: run with -store=postgres or -store=both")
	}
}

// pool returns the pool of the parent's store, which only PostgreSQL has.
func (w *world) pool() *pgxpool.Pool {
	w.t.Helper()
	withPool, ok := w.store.(interface{ Pool() *pgxpool.Pool })
	if !ok {
		w.t.Fatal("the store has no pool: the rows that use it need PostgreSQL")
	}
	return withPool.Pool()
}

// superuser connects to the database of the world as the server's superuser: the
// pooled connections run as the application role, which may not end other sessions.
func (w *world) superuser() *pgx.Conn {
	w.t.Helper()
	pool := w.pool()
	conn, err := pgx.ConnectConfig(w.t.Context(), pool.Config().ConnConfig.Copy())
	if err != nil {
		w.t.Fatalf("connect as the superuser: %v", err)
	}
	w.t.Cleanup(func() { _ = conn.Close(w.t.Context()) })
	return conn
}

// hostOf returns the host:port the parent reaches the database on.
func (w *world) hostOf() string {
	w.t.Helper()
	pool := w.pool()
	cfg := pool.Config().ConnConfig
	return fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
}

// waitActivations waits until the children have activated at least n nodes.
func (w *world) waitActivations(n int, within time.Duration) {
	w.t.Helper()
	deadline := time.Now().Add(within)
	for len(w.activations()) < n {
		if time.Now().After(deadline) {
			w.t.Fatalf("fewer than %d nodes were activated within %v", n, within)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// missingExecutions returns how many of the executions cannot be read back.
func (w *world) missingExecutions(ids []store.ExecutionID) int {
	w.t.Helper()
	missing := 0
	for _, id := range ids {
		if _, err := w.store.GetExecution(w.t.Context(), storetest.ScopeOf(w.env.Org), id); err != nil {
			missing++
		}
	}
	return missing
}

// TestNetworkCutToDatabase (FI-07, PostgreSQL): a worker whose connection to the
// database is cut in the middle of its work cannot renew its leases, so they expire
// and the work is recovered by another worker, which finishes it once. The cut worker
// commits nothing while it is cut, and when the network returns it does not commit
// what it computed without its lease.
func TestNetworkCutToDatabase(t *testing.T) {
	requirePostgres(t)
	w := newWorld(t, storetest.PostgreSQL)
	var ids []store.ExecutionID
	p := w.publish(sequence("t1", "t2", "t3"))
	for range 4 {
		ids = append(ids, w.start(p))
	}
	proxy := newCutProxy(t, w.hostOf())
	c := w.startChild(envDatabase+"="+proxy.DSN(w.path), envLease+"=1000", envNodeMS+"=300")
	c.waitLine("READY")
	w.waitActivations(1, 20*time.Second)
	proxy.Cut()
	time.Sleep(3 * time.Second) // three leases of one second expire at the database
	proxy.Restore()
	// The work resumes elsewhere: a second worker, on a connection of its own, finishes it.
	w.finish(envLease+"=1000", envNodeMS+"=300")
	if r := c.wait(90 * time.Second); r.Killed || r.Code != 0 {
		t.Fatalf("the cut worker ended with %+v, want an orderly exit once the network is back", r)
	}
	proxy.Close()
	w.open()
	for _, id := range ids {
		w.verifyExecution(id, 5)
	}
	if n := countType(w, ids[0], "lease.expired") + countType(w, ids[1], "lease.expired") + countType(w, ids[2], "lease.expired") + countType(w, ids[3], "lease.expired"); n < 1 {
		t.Errorf("no lease.expired event: the leases of the cut worker were never recovered")
	}
	w.integrity()
}

// TestPostgresConnectionsEnded (FI-08, PostgreSQL): the database loses every
// connection of the workers at once, as a restart of the server does to its clients.
// No accepted execution is lost, and the work resumes once the workers connect again.
// This does not model a failover to a synchronous replica: the harness has one server.
func TestPostgresConnectionsEnded(t *testing.T) {
	requirePostgres(t)
	w := newWorld(t, storetest.PostgreSQL)
	p := w.publish(sequence("t1", "t2", "t3"))
	var ids []store.ExecutionID
	for range 4 {
		ids = append(ids, w.start(p))
	}
	c := w.startChild(envLease+"=1000", envNodeMS+"=300")
	c.waitLine("READY")
	w.waitActivations(1, 20*time.Second)
	admin := w.superuser()
	if _, err := admin.Exec(w.t.Context(), "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND pid <> pg_backend_pid()"); err != nil {
		t.Fatalf("end the connections: %v", err)
	}
	w.finish(envLease+"=1000", envNodeMS+"=300")
	if r := c.wait(90 * time.Second); r.Killed || r.Code != 0 {
		t.Fatalf("the worker ended with %+v, want it to reconnect and finish", r)
	}
	w.open()
	if n := w.missingExecutions(ids); n != 0 {
		t.Errorf("%d of the %d accepted executions cannot be read after the connections ended", n, len(ids))
	}
	for _, id := range ids {
		w.verifyExecution(id, 5)
	}
}

// TestRaceAcrossProcesses (FI-09, and the multi-process races of C4b on PostgreSQL):
// three worker processes share the work. One of them is stopped in the middle of
// its tokens for longer than their leases, so the others take those tokens over; when
// it continues, its commits are fenced. Every execution then runs each node once,
// with no duplicate transition, on either store.
func TestRaceAcrossProcesses(t *testing.T) {
	requirePostgres(t) // SQLite takes its file for one process at a time (SPEC-001 C2a)
	{
		const executions = 8
		w := newWorld(t, storetest.PostgreSQL)
		p := w.publish(sequence("t1", "t2", "t3"))
		var ids []store.ExecutionID
		for range executions {
			ids = append(ids, w.start(p))
		}
		w.release()
		var workers []*child
		for range 3 {
			c := w.startChild(envLease+"=1000", envNodeMS+"=50")
			c.waitLine("READY")
			workers = append(workers, c)
		}
		w.waitActivations(3, 30*time.Second)
		workers[0].signal(syscall.SIGSTOP)
		time.Sleep(3 * time.Second)
		workers[0].signal(syscall.SIGCONT)
		for i, c := range workers {
			if r := c.wait(3 * time.Minute); r.Killed || r.Code != 0 {
				t.Fatalf("worker %d ended with %+v", i, r)
			}
		}
		w.open()
		for _, id := range ids {
			w.verifyExecution(id, 5)
		}
		w.integrity()
	}
}
