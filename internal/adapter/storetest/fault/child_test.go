// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package fault_test

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/platform/id"
)

// The fault tests run the engine in real processes: the test binary starts itself
// as a worker (a child process) with the environment below, and kills it, stops it
// or lets it run. The child opens the database file, runs the engine until the work
// is done, and says what it is doing on its standard output, one line at a time.
const (
	// The presence of this variable makes the test binary a child.
	envChild = "ORCON_FAULT_CHILD"
	// The SQLite file the child works.
	envDatabase = "ORCON_FAULT_DB"
	// The file the child appends a line to for every node it activates.
	envActivations = "ORCON_FAULT_ACTIVATIONS"
	// The next three say where and how the child dies or stops:
	// at the count-th hit of the point, it kills itself (kill), or tells the parent and
	// waits to be stopped (hold).
	envPoint  = "ORCON_FAULT_POINT"
	envCount  = "ORCON_FAULT_COUNT"
	envAction = "ORCON_FAULT_ACTION"
	// The activation of this node kills the process: a poison item.
	envCrashNode = "ORCON_FAULT_CRASH_NODE"
	// The size, in bytes, the files of the child may reach (a full disk).
	envFileLimit = "ORCON_FAULT_FILE_LIMIT"
	// The lease TTL of the child, in milliseconds.
	envLease = "ORCON_FAULT_LEASE_MS"
	// How long every node takes, in milliseconds, and how long the poison node waits before it kills the process.
	envNodeMS       = "ORCON_FAULT_NODE_MS"
	envCrashDelayMS = "ORCON_FAULT_CRASH_DELAY_MS"
	// The shutdown grace of the child, in milliseconds.
	envGraceMS = "ORCON_FAULT_GRACE_MS"
)

// logger is the activation log of the child: a line is appended in one write, which
// survives the death of the process.
type activationRunner struct {
	file       *os.File
	crashNode  string
	nodeTime   time.Duration
	crashAfter time.Duration
	inflight   atomic.Int64
	inner      executiontest.TraceRunner
}

func (r *activationRunner) Execute(ctx context.Context, call execution.NodeCall) (execution.NodeOutput, error) {
	if r.file != nil {
		if _, err := fmt.Fprintf(r.file, "%s %s\n", call.NodeExecution, call.Node.ID); err != nil {
			return execution.NodeOutput{}, err
		}
	}
	r.inflight.Add(1)
	defer r.inflight.Add(-1)
	if r.crashNode != "" && string(call.Node.ID) == r.crashNode {
		time.Sleep(r.crashAfter) // the other tokens of the process are in the middle of their nodes
		if r.inflight.Load() > 1 {
			sayf("CRASH shared")
		} else {
			sayf("CRASH alone")
		}
		killSelf()
	}
	if r.nodeTime > 0 {
		select {
		case <-ctx.Done():
			return execution.NodeOutput{}, fmt.Errorf("node %s canceled: %w", call.Node.ID, ctx.Err())
		case <-time.After(r.nodeTime):
		}
	}
	return r.inner.Execute(ctx, call)
}

func killSelf() {
	_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
	select {} // the signal ends the process before this returns
}

// childFaults kills or holds the process at the count-th hit of a point.
type childFaults struct {
	point  durable.Point
	count  int64
	action string
	hits   atomic.Int64
}

func (f *childFaults) Hit(_ context.Context, p durable.Point, token store.TokenID) error {
	if f.action == "" || p != f.point {
		return nil
	}
	if f.hits.Add(1) != f.count {
		return nil
	}
	switch f.action {
	case "kill":
		killSelf()
	case "hold":
		// The parent stops and continues the process; the wait ends when the process is continued.
		resumed := make(chan os.Signal, 1)
		signal.Notify(resumed, syscall.SIGCONT)
		defer signal.Stop(resumed)
		sayf("HOLD %s %s", p, token)
		select {
		case <-resumed:
		case <-time.After(5 * time.Minute):
		}
	}
	return nil
}

// runChild is the body of a child process. It returns the exit code.
func runChild() int {
	if v := os.Getenv(envFileLimit); v != "" {
		limit, _ := strconv.ParseUint(v, 10, 64)
		signal.Ignore(syscall.SIGXFSZ)
		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: limit, Max: limit}); err != nil {
			sayf("ERROR setrlimit: %v", err)
			return 2
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	st, err := sqlite.Open(ctx, sqlite.Options{Path: os.Getenv(envDatabase), BinaryVersion: "1.0.0", Synchronous: sqlite.SynchronousFull})
	if err != nil {
		sayf("ERROR open: %v", err)
		return 2
	}
	defer st.Close() //nolint:errcheck // the process is ending; the parent verifies the file
	runner := &activationRunner{crashNode: os.Getenv(envCrashNode)}
	if path := os.Getenv(envActivations); path != "" {
		if runner.file, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err != nil { //nolint:gosec // the path is the one the parent passed through the environment
			sayf("ERROR activations: %v", err)
			return 2
		}
	}
	lease := 15 * time.Second
	if v := os.Getenv(envLease); v != "" {
		ms, _ := strconv.Atoi(v)
		lease = time.Duration(ms) * time.Millisecond
	}
	grace := 5 * time.Second
	if v := os.Getenv(envGraceMS); v != "" {
		ms, _ := strconv.Atoi(v)
		grace = time.Duration(ms) * time.Millisecond
	}
	if v := os.Getenv(envNodeMS); v != "" {
		ms, _ := strconv.Atoi(v)
		runner.nodeTime = time.Duration(ms) * time.Millisecond
	}
	if v := os.Getenv(envCrashDelayMS); v != "" {
		ms, _ := strconv.Atoi(v)
		runner.crashAfter = time.Duration(ms) * time.Millisecond
	}
	// The concurrency is the default of the configuration: the poison item is tested with the neighbors it has there.
	cfg := durable.Config{
		Worker: execution.WorkerID(id.NewV7Generator(time.Now).New()), Role: execution.WorkerStandalone,
		LeaseTTL: lease, RenewEvery: lease / 3, PollEvery: 10 * time.Millisecond, ReapEvery: 50 * time.Millisecond, ShutdownGrace: grace,
	}
	if point := os.Getenv(envPoint); point != "" {
		count, _ := strconv.ParseInt(os.Getenv(envCount), 10, 64)
		cfg.Faults = &childFaults{point: durable.Point(point), count: count, action: os.Getenv(envAction)}
	}
	eng, err := durable.New(&cfg, st, st, storetest.FlowReader{}, execution.Deps{Runner: runner}, durable.PlanLimits{})
	if err != nil {
		sayf("ERROR engine: %v", err)
		return 2
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- eng.Run(runCtx) }()
	sayf("READY concurrency=%d", cfg.Concurrency)
	go watchIdle(runCtx, st, cancel)
	if err := <-done; err != nil {
		sayf("ERROR run: %v", err)
		return 1
	}
	sayf("DONE")
	return 0
}

// watchIdle ends the run when no execution is running or queued any more.
func watchIdle(ctx context.Context, st *sqlite.Store, cancel context.CancelFunc) {
	quiet := 0
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		busy, err := anyActive(ctx, st)
		if err != nil {
			continue
		}
		if busy {
			quiet = 0
			continue
		}
		quiet++
		if quiet >= 4 {
			cancel()
			return
		}
	}
}

// anyActive reports whether any organization has an execution that still has work: queued or running.
func anyActive(ctx context.Context, st *sqlite.Store) (bool, error) {
	var n int
	err := st.Reader().QueryRowContext(ctx, `SELECT count(*) FROM execution WHERE status IN ('QUEUED', 'RUNNING')`).Scan(&n)
	return n > 0, err
}

// say writes a line on the standard output of the child, which the parent reads.
func sayf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", args...)
}
