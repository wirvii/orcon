// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// The fault matrix of ADR-0019 section 3 for the durable engine over SQLite, with
// real processes: the test binary starts itself as a worker, and the test kills it
// with SIGKILL, stops it with SIGSTOP, or runs it with a file size limit. Rows that
// need no process of their own (FI-09, FI-10, FI-11, FI-16) are in
// internal/execution/durable; the PostgreSQL rows arrive with cut C4b.
package fault_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
)

// storeFlag selects the store: SQLite is the one with processes in this cut.
func storeFlag() string { return executiontest.StoreFlag("sqlite") }

func TestMain(m *testing.M) {
	if os.Getenv(envChild) != "" {
		exitProcess(runChild())
	}
	m.Run()
}

func exitProcess(code int) {
	os.Exit(code) //nolint:forbidigo // AL-8: the child process of a fault test ends with its own exit code
}

func requireSQLite(t *testing.T) {
	t.Helper()
	switch storeFlag() {
	case "sqlite", "both", "auto":
	case "postgres":
		t.Skip("the process rows of the fault matrix run on PostgreSQL with cut C4b; the in-process rows run there with `make test-fault STORE=postgres`")
	default:
		t.Fatalf("unknown store %q", storeFlag())
	}
}

// world is a database file and what the parent knows about it.
type world struct {
	t     *testing.T
	path  string
	log   string
	store *sqlite.Store
	env   *storetest.EngineEnv
	eng   *durable.Engine
}

func newWorld(t *testing.T) *world {
	t.Helper()
	dir := t.TempDir()
	w := &world{t: t, path: filepath.Join(dir, "orcon.db"), log: filepath.Join(dir, "activations.log")}
	w.open()
	w.env.Setup()
	return w
}

// open takes the database file for the parent.
func (w *world) open() {
	w.t.Helper()
	w.store = storetest.OpenSQLiteFile(w.t, w.path, storetest.EngineOptions{})
	if w.env == nil {
		w.env = storetest.NewEngineEnvOn(w.t, storetest.SQLite, w.store, storetest.EngineOptions{})
	} else {
		w.env.Store = w.store
	}
	w.eng = w.env.Engine(durable.Config{Concurrency: 1}, execution.Deps{Runner: &executiontest.TraceRunner{}})
}

// release gives the file up so that a child can take it.
func (w *world) release() {
	w.t.Helper()
	if err := w.store.Close(); err != nil {
		w.t.Fatalf("close the store: %v", err)
	}
}

func (w *world) publish(b *executiontest.Builder) *storetest.Published {
	d := b.Doc()
	return w.env.Publish(&d)
}

func (w *world) start(p *storetest.Published) store.ExecutionID {
	return w.env.Start(w.eng, p, executiontest.Batch(`{"n":1}`))
}

// activations returns the nodes the children activated, one per line of the log.
func (w *world) activations() []string {
	data, err := os.ReadFile(w.log)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		w.t.Fatal(err)
	}
	return strings.Fields(strings.ReplaceAll(string(data), "\n", " "))
}

// child is a worker process.
type child struct {
	t     *testing.T
	cmd   *exec.Cmd
	lines chan string
	exit  chan error
}

// startChild runs a worker on the database with the extra environment.
func (w *world) startChild(extra ...string) *child {
	w.t.Helper()
	cmd := exec.CommandContext(context.WithoutCancel(w.t.Context()), os.Args[0], "-test.run=^$") //nolint:gosec // G204: the test starts its own binary
	cmd.Env = append(os.Environ(), envChild+"=worker", envDatabase+"="+w.path, envActivations+"="+w.log)
	cmd.Env = append(cmd.Env, extra...)
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		w.t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		w.t.Fatalf("start the worker process: %v", err)
	}
	c := &child{t: w.t, cmd: cmd, lines: make(chan string, 256), exit: make(chan error, 1)}
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			c.lines <- sc.Text()
		}
		close(c.lines)
	}()
	go func() { c.exit <- cmd.Wait() }()
	w.t.Cleanup(func() { _ = cmd.Process.Kill() })
	return c
}

// waitLine waits for a line of standard output that starts with prefix.
func (c *child) waitLine(prefix string) string {
	const within = 20 * time.Second
	c.t.Helper()
	deadline := time.After(within)
	for {
		select {
		case line, ok := <-c.lines:
			if !ok {
				c.t.Fatalf("the worker process ended before it printed %q", prefix)
			}
			if strings.HasPrefix(line, prefix) {
				return line
			}
			if strings.HasPrefix(line, "ERROR") {
				c.t.Fatalf("worker process: %s", line)
			}
		case <-deadline:
			c.t.Fatalf("no %q from the worker process within %v", prefix, within)
		}
	}
}

// result waits for the process to end and says how.
type result struct {
	Killed bool
	Code   int
}

func (c *child) wait(within time.Duration) result {
	c.t.Helper()
	select {
	case err := <-c.exit:
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				return result{Killed: ws.Signal() == syscall.SIGKILL, Code: -int(ws.Signal())}
			}
			return result{Code: ee.ExitCode()}
		}
		if err != nil {
			c.t.Fatalf("wait for the worker process: %v", err)
		}
		return result{}
	case <-time.After(within):
		_ = c.cmd.Process.Kill()
		c.t.Fatalf("the worker process did not end within %v", within)
		return result{}
	}
}

func (c *child) signal(sig syscall.Signal) {
	c.t.Helper()
	if err := c.cmd.Process.Signal(sig); err != nil {
		c.t.Fatalf("signal %v: %v", sig, err)
	}
}

// finish runs a worker with no fault until the work is done.
func (w *world) finish(extra ...string) {
	w.t.Helper()
	c := w.startChild(extra...)
	c.waitLine("READY")
	if r := c.wait(60 * time.Second); r.Killed || r.Code != 0 {
		w.t.Fatalf("the worker process that finishes the work ended with %+v", r)
	}
}

// verifyExecution checks the rows of an execution that ran to its end: each node of
// the flow ran once, whatever the number of times a process computed it.
func (w *world) verifyExecution(execID store.ExecutionID, wantNodes int) {
	const wantStatus = execution.ExecSucceeded
	w.t.Helper()
	ex, err := w.store.GetExecution(w.t.Context(), storetest.ScopeOf(w.env.Org), execID)
	if err != nil {
		w.t.Fatal(err)
	}
	if ex.Status != string(wantStatus) {
		w.t.Fatalf("execution %s is %s, want %s", execID, ex.Status, wantStatus)
	}
	started, succeeded := 0, 0
	seen := map[string]int{}
	for _, e := range w.env.Events(execID) { //nolint:gocritic // reading the event itself is clearer than indexing
		switch e.Type {
		case "node.started":
			started++
			seen[string(e.Reference)]++
		case "node.succeeded":
			succeeded++
		}
	}
	if started != wantNodes || succeeded != wantNodes {
		w.t.Errorf("execution %s: %d node.started and %d node.succeeded events, want %d of each", execID, started, succeeded, wantNodes)
	}
	for ref, n := range seen {
		if n != 1 {
			w.t.Errorf("execution %s: node execution %s started %d times", execID, ref, n)
		}
	}
	if live := w.env.LiveTokens(execID); len(live) != 0 && wantStatus == execution.ExecSucceeded {
		w.t.Errorf("execution %s ended with %d live tokens", execID, len(live))
	}
}

func (w *world) integrity() {
	w.t.Helper()
	var result string
	if err := w.store.Writer().QueryRowContext(w.t.Context(), "PRAGMA integrity_check").Scan(&result); err != nil || result != "ok" {
		w.t.Fatalf("PRAGMA integrity_check = %q, %v", result, err)
	}
	rows, err := w.store.Writer().QueryContext(w.t.Context(), "PRAGMA foreign_key_check")
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck // read-only check
	if rows.Next() {
		w.t.Fatal("PRAGMA foreign_key_check found a violation")
	}
}

func forkJoin(n int) *executiontest.Builder {
	b := executiontest.NewFlow("fork join").Start("s").Parallel("p").Parallel("j").Task("after").End("e").Join("j", flow.JoinAll)
	b.Chain("s", "p")
	for i := range n {
		id := fmt.Sprintf("b%d", i)
		b.Task(id).Edge("p", id).Edge(id, "j")
	}
	return b.Chain("j", "after", "e")
}

func sequence(names ...string) *executiontest.Builder {
	b := executiontest.NewFlow("sequence").Start("s")
	ids := []string{"s"}
	for _, n := range names {
		b.Task(n)
		ids = append(ids, n)
	}
	b.End("e")
	return b.Chain(append(ids, "e")...)
}

// TestKillDuringForkAndJoin (FI-04): a worker killed with SIGKILL at each commit of a
// fork and join, in the middle of the transaction or right after it, leaves an
// execution that finishes with every branch exactly once and exactly one continuation.
func TestKillDuringForkAndJoin(t *testing.T) {
	requireSQLite(t)
	const branches = 6
	const commits = branches + 4 // the fork, the branches, the join continuation and the end
	for _, point := range []durable.Point{durable.PointInCommit, durable.PointCommitted} {
		for n := 1; n <= commits; n++ {
			t.Run(fmt.Sprintf("%s/%d", point, n), func(t *testing.T) {
				w := newWorld(t)
				execID := w.start(w.publish(forkJoin(branches)))
				w.release()
				c := w.startChild(envPoint+"="+string(point), envCount+"="+strconv.Itoa(n), envAction+"=kill", envLease+"=400")
				c.waitLine("READY")
				r := c.wait(60 * time.Second)
				switch {
				case r.Killed:
					w.finish(envLease + "=400")
				case r.Code != 0:
					t.Fatalf("worker process ended with %+v", r)
				case n <= branches+2:
					t.Fatalf("the worker was not killed at %s %d of a flow that commits %d times", point, n, commits)
				}
				w.open()
				w.verifyExecution(execID, 5+branches)
				if got := countType(w, execID, "join.fired"); got != 1 {
					t.Errorf("%d join.fired events, want exactly one continuation", got)
				}
				if got := countType(w, execID, "token.forked"); got != 1 {
					t.Errorf("%d token.forked events, want one fork", got)
				}
				w.integrity()
			})
		}
	}
}

func countType(w *world, execID store.ExecutionID, typ string) int {
	n := 0
	for _, e := range w.env.Events(execID) { //nolint:gocritic // reading the event itself is clearer than indexing
		if e.Type == typ {
			n++
		}
	}
	return n
}

// TestStoppedWorkerIsFenced (FI-05): a worker stopped with SIGSTOP in the middle of
// a token, for longer than its lease, and continued with SIGCONT, does not commit
// what it computed; its token is recovered and the work is committed once.
func TestStoppedWorkerIsFenced(t *testing.T) {
	requireSQLite(t)
	seconds := 3
	if v := os.Getenv("ORCON_FAULT_STOP_SECONDS"); v != "" {
		seconds, _ = strconv.Atoi(v)
	}
	w := newWorld(t)
	execID := w.start(w.publish(sequence("t1", "t2", "t3")))
	w.release()
	c := w.startChild(envPoint+"="+string(durable.PointWorked), envCount+"=1", envAction+"=hold", envLease+"=1000")
	hold := c.waitLine("HOLD")
	t.Logf("worker process held at %q; stopping it for %d s", hold, seconds)
	c.signal(syscall.SIGSTOP)
	time.Sleep(time.Duration(seconds) * time.Second)
	c.signal(syscall.SIGCONT)
	if r := c.wait(5*time.Minute + time.Duration(seconds)*time.Second); r.Killed || r.Code != 0 {
		t.Fatalf("worker process ended with %+v", r)
	}
	w.open()
	w.verifyExecution(execID, 5)
	if got := countType(w, execID, "lease.expired"); got < 1 {
		t.Errorf("%d lease.expired events: the lease of the stopped worker was never recovered", got)
	}
	// What the stopped worker computed was computed again by the worker that holds the token now.
	if n := len(w.activations()); n < 6 {
		t.Logf("%d activations logged; the stopped worker had computed its steps before it was stopped", n)
	}
	w.integrity()
}

// TestPoisonItem (FI-12, SPEC-001 Enmienda 12 section 12-1): a node that kills the process
// every time it is activated is suspended as poison after three abandonments, with the
// default concurrency of the worker and healthy executions sharing its process, and
// the healthy ones finish without any of their tokens being suspended. A node longer
// than the shutdown grace that crosses three orderly shutdowns is never counted.
func TestPoisonItem(t *testing.T) {
	requireSQLite(t)
	t.Run("a poison item next to healthy executions", poisonNextToHealthy)
	t.Run("three orderly shutdowns across a long node", orderlyShutdowns)
}

func poisonNextToHealthy(t *testing.T) {
	const threshold = 3
	w := newWorld(t)
	poison := w.start(w.publish(sequence("poison")))
	var others []store.ExecutionID
	healthy := w.publish(sequence("h1", "h2", "h3", "h4", "h5", "h6"))
	for range 3 {
		others = append(others, w.start(healthy))
	}
	maxHealthy := int64(0)
	var crashes []string
	for deaths := 0; ; {
		w.release()
		c := w.startChild(envCrashNode+"=poison", envNodeMS+"=800", envCrashDelayMS+"=1000")
		ready := c.waitLine("READY")
		if want := fmt.Sprintf("READY concurrency=%d", durable.DefaultConcurrency); ready != want {
			t.Fatalf("worker says %q, want the default concurrency: %q", ready, want)
		}
		r := c.wait(120 * time.Second)
		crash := drainLines(c, "CRASH")
		w.open()
		for _, id := range others {
			live := w.env.LiveTokens(id)
			for i := range live {
				if live[i].Status == "SUSPENDED" {
					t.Fatalf("a healthy token was suspended: %+v", live[i])
				}
				maxHealthy = max(maxHealthy, live[i].AbandonedCount)
			}
		}
		if !r.Killed {
			if r.Code != 0 {
				t.Fatalf("worker process ended with %+v", r)
			}
			break
		}
		crashes = append(crashes, crash)
		if deaths++; deaths > 6 {
			t.Fatal("the poison item kept killing the process after six restarts")
		}
	}
	if len(crashes) != threshold {
		t.Errorf("the process died %d times (%v), want %d: the third abandonment suspends the token", len(crashes), crashes, threshold)
	}
	if len(crashes) > 0 && crashes[len(crashes)-1] != "CRASH alone" {
		t.Errorf("the crash that reached the threshold was %q: it must happen with the token alone in its process", crashes[len(crashes)-1])
	}
	if shared := slices.Index(crashes, "CRASH shared"); shared < 0 {
		t.Errorf("no crash happened with healthy tokens in the process (%v): the test did not exercise the neighbors", crashes)
	}
	if maxHealthy >= threshold {
		t.Errorf("a healthy token was abandoned %d times, want fewer than %d", maxHealthy, threshold)
	}
	tokens := w.env.LiveTokens(poison)
	if len(tokens) != 1 || tokens[0].Status != "SUSPENDED" || tokens[0].SuspendReason != "POISON" || tokens[0].AbandonedCount != threshold {
		t.Fatalf("tokens of the poison execution = %+v, want one SUSPENDED POISON token abandoned %d times", tokens, threshold)
	}
	if got := countType(w, poison, "token.poisoned"); got != 1 {
		t.Errorf("%d token.poisoned events, want 1", got)
	}
	requests, err := w.store.ListAttentionRequests(t.Context(), storetest.ScopeOf(w.env.Org), "OPEN", store.Cursor{}, 10)
	if err != nil || len(requests) != 1 || requests[0].Cause != "POISON_TOKEN" {
		t.Errorf("attention requests = %+v, %v; want one POISON_TOKEN", requests, err)
	}
	if ex, err := w.store.GetExecution(t.Context(), storetest.ScopeOf(w.env.Org), poison); err != nil || ex.Status != string(execution.ExecSuspended) {
		t.Errorf("poison execution = %+v, %v; want SUSPENDED", ex, err)
	}
	for _, id := range others {
		w.verifyExecution(id, 8)
	}
}

// drainLines returns the last line of the process that starts with prefix.
func drainLines(c *child, prefix string) string {
	last := ""
	for line := range c.lines {
		if strings.HasPrefix(line, prefix) {
			last = line
		}
	}
	return last
}

func orderlyShutdowns(t *testing.T) {
	w := newWorld(t)
	long := w.start(w.publish(sequence("long")))
	for shutdown := 1; shutdown <= 3; shutdown++ {
		w.release()
		// The node outlasts the grace period of the process, so the shutdown cancels it.
		c := w.startChild(envNodeMS+"=60000", envGraceMS+"=300")
		c.waitLine("READY")
		time.Sleep(700 * time.Millisecond) // the token is claimed and the node is running
		c.signal(syscall.SIGTERM)
		if r := c.wait(30 * time.Second); r.Killed || r.Code != 0 {
			t.Fatalf("shutdown %d: the worker ended with %+v, want an orderly exit", shutdown, r)
		}
		w.open()
		tokens := w.env.LiveTokens(long)
		if len(tokens) != 1 || tokens[0].Status != "RUNNABLE" || tokens[0].AbandonedCount != 0 {
			t.Fatalf("shutdown %d: tokens = %+v, want one RUNNABLE token with no abandonment", shutdown, tokens)
		}
	}
	w.release()
	w.finish()
	w.open()
	w.verifyExecution(long, 3)
	if got := countType(w, long, "lease.expired"); got != 0 {
		t.Errorf("%d lease.expired events for leases the worker gave back", got)
	}
}

// TestKillDuringWrites (FI-15): processes killed with SIGKILL at random moments while
// they commit leave a database with no corruption in which every accepted execution
// finishes.
func TestKillDuringWrites(t *testing.T) {
	requireSQLite(t)
	const executions = 24
	w := newWorld(t)
	p := w.publish(sequence("a", "b", "c"))
	var ids []store.ExecutionID
	for range executions {
		ids = append(ids, w.start(p))
	}
	rng := rand.New(rand.NewPCG(3, 5)) //nolint:gosec // G404: picks when to kill, not a secret
	for round := range 6 {
		w.release()
		c := w.startChild(envLease + "=300")
		c.waitLine("READY")
		time.Sleep(time.Duration(5+rng.IntN(60)) * time.Millisecond)
		c.signal(syscall.SIGKILL)
		if r := c.wait(30 * time.Second); !r.Killed && r.Code != 0 {
			t.Fatalf("round %d: worker process ended with %+v", round, r)
		}
		w.open()
		w.integrity()
	}
	w.release()
	w.finish(envLease + "=300")
	w.open()
	for _, id := range ids {
		w.verifyExecution(id, 5)
	}
	w.integrity()
}

// TestDiskFull (FI-15): a worker that cannot write any more (the size of the files it
// may write is limited) fails its commits without corrupting the database, and the
// executions finish once it can write again.
func TestDiskFull(t *testing.T) {
	requireSQLite(t)
	const executions = 40
	w := newWorld(t)
	p := w.publish(sequence("a", "b", "c", "d"))
	var ids []store.ExecutionID
	for range executions {
		ids = append(ids, w.start(p))
	}
	w.release()
	size := fileSize(t, w.path) + fileSize(t, w.path+"-wal")
	limit := size + 96<<10
	c := w.startChild(envFileLimit+"="+strconv.FormatInt(limit, 10), envLease+"=300")
	c.waitLine("READY")
	time.Sleep(3 * time.Second) // it works until the limit stops its writes
	c.signal(syscall.SIGKILL)
	c.wait(30 * time.Second)
	w.open()
	w.integrity()
	done := 0
	for _, id := range ids {
		if ex, err := w.store.GetExecution(t.Context(), storetest.ScopeOf(w.env.Org), id); err == nil && ex.Status == string(execution.ExecSucceeded) {
			done++
		}
	}
	t.Logf("%d of %d executions finished before the files reached their limit of %d bytes", done, executions, limit)
	if done == executions {
		t.Fatal("the file size limit never stopped the worker: the test did not exercise a full disk")
	}
	w.release()
	w.finish(envLease + "=300")
	w.open()
	for _, id := range ids {
		w.verifyExecution(id, 6)
	}
	w.integrity()
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}
