// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command orcon is the Orcon server. It is the composition root: it parses
// flags, builds every dependency for the selected role and owns the process
// lifetime. All other packages receive their dependencies as arguments.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/adapter/webui"
	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/config"
)

// Exit codes returned by run.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
	// The exit code of a graceful shutdown asking for a restart (EX_TEMPFAIL):
	// the expression sandbox is saturated or stuck, and the restart policy of
	// the deployment must start the process again (ADR-0018 section 3).
	exitRestart = 75
)

// HTTP server limits. They bound slow or abusive clients (implementation
// contract: bounded resources, explicit timeouts).
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
)

const defaultShutdownTimeout = 30 * time.Second

// version and commit are injected by the release build with
// -X main.version=<version> -X main.commit=<commit> (.goreleaser.yaml).
var (
	version = "dev"
	commit  = ""
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is the testable body of the process. It returns the exit code and never
// calls os.Exit. The process runs until ctx is canceled, then shuts down
// within the configured shutdown timeout. Help goes to stdout, diagnostics
// and logs to stderr.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runWith(ctx, args, stdout, stderr, productionSandbox)
}

// exprSandbox is what the composition root keeps of the expression sandbox:
// the signal that the process must restart, and its release.
type exprSandbox interface {
	expr.Health
	Close(ctx context.Context) error
}

// sandboxFactory builds the expression sandbox of a process.
type sandboxFactory func(maxAbandoned, slots int, logger *slog.Logger) exprSandbox

// productionSandbox builds the sandbox with the stop mode of production, which
// is the default of the adapter: the composition root has no way to pick
// another (TestProductionStopMode).
func productionSandbox(maxAbandoned, slots int, logger *slog.Logger) exprSandbox { //nolint:ireturn // a sandboxFactory returns the port the root keeps
	return wazero.New(wazero.WithMaxAbandoned(maxAbandoned), wazero.WithSlots(slots), wazero.WithLogger(logger))
}

func runWith(ctx context.Context, args []string, stdout, stderr io.Writer, newSandbox sandboxFactory) int {
	flags := flag.NewFlagSet("orcon", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	role := flags.String("role", "standalone", "standalone|api|scheduler|worker")
	addr := flags.String("addr", ":8080", "HTTP listen address")
	shutdownTimeout := flags.Duration("shutdown-timeout", defaultShutdownTimeout, "maximum time to drain in-flight requests on shutdown")
	showVersion := flags.Bool("version", false, "print the version and exit")
	slots := flags.Int("expr-sandbox-slots", 0, "expression sandbox calls that may execute guest code at once (1 to GOMAXPROCS-1; 0 means GOMAXPROCS-1)")
	maxAbandoned := flags.Int("expr-max-abandoned", config.SandboxMaxAbandonedDefault,
		"maximum expression evaluations abandoned at their deadline that may still be running (1 to 64)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stdout, flags)
			return exitOK
		}
		_, _ = fmt.Fprintln(stderr, "orcon:", err)
		printUsage(stderr, flags)
		return exitUsage
	}

	if *showVersion {
		_, _ = fmt.Fprintf(stdout, "orcon %s (%s)\n", version, commit)
		return exitOK
	}

	selected := config.Role(strings.ToLower(*role))
	if !selected.Valid() {
		// A failed write to stderr leaves nothing better to do.
		_, _ = fmt.Fprintln(stderr, "invalid role:", *role)
		return exitUsage
	}

	if !config.ValidSandboxMaxAbandoned(*maxAbandoned) {
		_, _ = fmt.Fprintf(stderr, "invalid --expr-max-abandoned: %d (want %d to %d)\n",
			*maxAbandoned, config.SandboxMaxAbandonedMin, config.SandboxMaxAbandonedMax)
		return exitUsage
	}

	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	// The sandbox needs a scheduler slot for timers, callers and the API besides
	// the slots that run guest code, so with one slot it cannot work: the
	// operating system shares the core by time and does preempt the thread of
	// the guest.
	procs := runtime.GOMAXPROCS(0)
	if procs < 2 {
		runtime.GOMAXPROCS(2)
		logger.InfoContext(ctx, "GOMAXPROCS raised to 2 so the expression sandbox keeps a scheduler slot free", "was", procs)
		procs = 2
	}
	slotCount := *slots
	if slotCount == 0 {
		slotCount = procs - 1
	}
	if !config.ValidSandboxSlots(slotCount, procs) {
		_, _ = fmt.Fprintf(stderr, "invalid --expr-sandbox-slots: %d (want 1 to %d)\n", slotCount, procs-1)
		return exitUsage
	}

	switch selected {
	case config.RoleStandalone, config.RoleAPI:
		return serveWithSandbox(ctx, logger, selected, *addr, *shutdownTimeout, *maxAbandoned, slotCount, newSandbox)
	case config.RoleScheduler, config.RoleWorker:
		logger.InfoContext(ctx, "role has no processing yet", "role", selected)
		return exitOK
	default:
		// Unreachable: Valid rejected every other value above.
		return exitUsage
	}
}

// serveWithSandbox runs the HTTP server with the expression sandbox beside it.
// When the sandbox reports that it is saturated or holds a stuck call, the
// process shuts down through the ordinary graceful shutdown and exits with
// [exitRestart], so the restart policy of the deployment starts it again.
func serveWithSandbox(ctx context.Context, logger *slog.Logger, role config.Role, addr string, shutdownTimeout time.Duration, maxAbandoned, slots int, newSandbox sandboxFactory) int {
	sb := newSandbox(maxAbandoned, slots, logger)
	effective := min(maxAbandoned, slots)
	defer func() { _ = sb.Close(context.WithoutCancel(ctx)) }()
	// The abandoned evaluations of a process can hold this much memory at most.
	logger.InfoContext(ctx, "expression sandbox limits",
		"slots", slots, "max_abandoned", effective, "memory_bound_bytes", uint64(slots)*uint64(expr.DefaultMemoryLimit)) //nolint:gosec // slots is validated to 1..GOMAXPROCS-1 by run

	serveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var restart atomic.Bool
	watched := make(chan struct{})
	go func() {
		defer close(watched)
		select {
		case <-sb.Degraded():
			report := sb.HealthReport()
			restart.Store(true)
			logger.ErrorContext(ctx, "expression sandbox is saturated or stuck: shutting down so the process restarts",
				"reason", expr.RestartReason,
				"abandoned_running", report.AbandonedRunning, "stuck", report.Stuck, "max_abandoned", report.MaxAbandoned,
				"abandoned_total", report.Abandoned, "saturated_total", report.Saturated)
			cancel()
		case <-serveCtx.Done():
		}
	}()

	err := serve(serveCtx, logger, role, addr, shutdownTimeout)
	cancel()
	<-watched
	switch {
	case err != nil:
		logger.ErrorContext(ctx, "server stopped with error", "role", role, "error", err)
		return exitError
	case restart.Load():
		return exitRestart
	default:
		return exitOK
	}
}

func printUsage(w io.Writer, flags *flag.FlagSet) {
	_, _ = fmt.Fprintln(w, "usage: orcon [flags]")
	flags.SetOutput(w)
	flags.PrintDefaults()
}

// newHandler assembles the HTTP routes served by the standalone and api roles.
func newHandler() (http.Handler, error) {
	assets, err := webui.Assets()
	if err != nil {
		return nil, fmt.Errorf("build handler: %w", err)
	}
	mux := http.NewServeMux()
	mux.Handle(httpapi.HealthPath, httpapi.HealthHandler())
	mux.Handle("/", webui.Handler(assets))
	return mux, nil
}

// serve listens on addr and serves HTTP until ctx is canceled, then drains
// in-flight requests for at most shutdownTimeout.
func serve(ctx context.Context, logger *slog.Logger, role config.Role, addr string, shutdownTimeout time.Duration) error {
	handler, err := newHandler()
	if err != nil {
		return err
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	// The goroutine below is owned by this function: serve always receives
	// from serveErr before returning.
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	logger.InfoContext(ctx, "listening", "role", role, "listen_addr", ln.Addr().String())

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
	}

	logger.InfoContext(ctx, "shutting down", "role", role)
	// Shutdown must outlive the canceled ctx, so detach cancellation and bound it explicitly.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	shutdownErr := srv.Shutdown(shutdownCtx)
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("serve http: %w", err))
	}
	if shutdownErr != nil {
		return fmt.Errorf("shut down http server: %w", shutdownErr)
	}
	return nil
}
