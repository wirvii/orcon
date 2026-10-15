// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wirvii/orcon/internal/adapter/cli"
	"github.com/wirvii/orcon/internal/adapter/codec/configyaml"
	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/adapter/webui"
	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/config"
)

// HTTP server limits. They bound slow or abusive clients (implementation
// contract: bounded resources, explicit timeouts).
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
)

// serveCommand is orcon serve. Its settings (role, http.address,
// http.shutdownTimeout, expr.*) come from the configuration schema with the
// precedence of R-CONF-01: it has no flags of its own.
func serveCommand() command {
	return command{
		name:    "serve",
		summary: "run the server (--role standalone|api|scheduler|worker)",
		run: func(ctx context.Context, args []string, stdout, stderr io.Writer) int {
			return runServe(ctx, args, stdout, stderr, serveDeps{
				newSandbox: productionSandbox,
				environ:    os.Environ(),
				reloadPoll: cli.ReloadPollInterval,
				openEngine: productionEngine,
			})
		},
	}
}

// serveDeps is what runServe takes from the outside, so tests can replace it.
type serveDeps struct {
	newSandbox sandboxFactory
	environ    []string
	// reloadPoll is how often the server looks for a reload request.
	reloadPoll time.Duration
	// openEngine opens the database and wires the execution engine of a role
	// that serves executions or public addresses. Nil means the process runs no
	// engine (tests that need no database).
	openEngine engineOpener
}

// engineOpener builds the execution engine of the process over the store that the
// security wiring already opened. The store is closed by that wiring, not here.
type engineOpener func(st engineStore, cfg *config.Config, sandbox expr.Sandbox, logger *slog.Logger) (*engine, error)

// productionEngine is the engine opener of the server: the engine over the open store,
// with the process version.
func productionEngine(st engineStore, cfg *config.Config, sandbox expr.Sandbox, logger *slog.Logger) (*engine, error) {
	return newEngine(st, engineOptions{role: cfg.Role(), logger: logger, version: version, sandbox: sandbox})
}

// servesEngine reports whether a role opens the engine. Only standalone does for
// now: the api role runs on PostgreSQL in several processes, and its public
// webhook addresses arrive with that topology's wiring (SPEC-002 T2 follow-up).
func servesEngine(role config.Role) bool {
	return role == config.RoleStandalone
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

// runServe is the start-up sequence of the server, in one place (AL-8.1):
// flags, configuration, logger, checks of the sandbox settings, then the role.
func runServe(ctx context.Context, args []string, stdout, stderr io.Writer, deps serveDeps) int {
	schema := config.Default()
	flags := flag.NewFlagSet("orcon serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	fv := schema.BindFlags(flags)
	if err := schema.ParseFlags(flags, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printServeUsage(stdout, flags)
			return exitOK
		}
		_, _ = fmt.Fprintln(stderr, "orcon:", err)
		return exitUsage
	}
	load := func() *config.Result {
		return schema.Load(config.Input{
			Flags: fv, Environ: deps.environ, FS: config.OSFileSystem{}, Decode: configyaml.Decode, GOOS: runtime.GOOS,
		})
	}
	res := load()
	if err := res.Err(); err != nil {
		_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
		return exitValidation
	}
	cfg := res.Config

	level := new(slog.LevelVar)
	level.Set(logLevel(cfg.Value("log.level").Text()))
	logger := newLogger(stderr, cfg.Value("log.format").Text(), level)
	for _, issue := range res.Issues {
		logger.WarnContext(ctx, "configuration warning", "key", issue.Key, "message", issue.Message)
	}

	selected := cfg.Role()
	if selected == config.RoleScheduler || selected == config.RoleWorker {
		logger.InfoContext(ctx, "role has no processing yet", "role", selected)
		return exitOK
	}

	maxAbandoned := cfg.Value("expr.maxAbandoned").Int()
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
	slots := cfg.Value("expr.sandboxSlots").Int()
	if slots == 0 {
		slots = procs - 1
	}
	if !config.ValidSandboxSlots(slots, procs) {
		_, _ = fmt.Fprintf(stderr, "invalid --expr-sandbox-slots: %d (want 1 to %d)\n", slots, procs-1)
		return exitUsage
	}

	reloader := config.NewReloader(cfg, load)
	reloader.Subscribe(func(_, current *config.Config, _ []string) {
		level.Set(logLevel(current.Value("log.level").Text()))
	})

	// The watchers stop when workCtx ends; the deferred calls run stopWork first, then wait.
	var wg sync.WaitGroup
	defer wg.Wait()
	workCtx, stopWork := context.WithCancel(ctx)
	defer stopWork()
	startReloadWatchers(workCtx, &wg, cfg.Value("dataDir").Text(), deps.reloadPoll, reloader, logger)

	// The roles that serve the API open the store, check the keys and wire the identity
	// services first: a failure refuses to start, before anything is listened on.
	var sec *security
	if selected == config.RoleStandalone || selected == config.RoleAPI {
		opened, err := openSecurity(ctx, cfg, logger, version)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "orcon: %v\n", err)
			return exitValidation
		}
		sec = opened
		defer func() {
			if err := sec.close(); err != nil {
				logger.ErrorContext(ctx, "close the store", "error", err)
			}
		}()
	}

	return serveWithSandbox(ctx, logger, serveSettings{
		role:            selected,
		addr:            cfg.Value("http.address").Text(),
		shutdownTimeout: cfg.Value("http.shutdownTimeout").Duration(),
		maxAbandoned:    maxAbandoned,
		slots:           slots,
		cfg:             cfg,
		openEngine:      deps.openEngine,
		sec:             sec,
	}, deps.newSandbox)
}

func printServeUsage(w io.Writer, flags *flag.FlagSet) {
	_, _ = fmt.Fprintln(w, "usage: orcon serve [flags]")
	_, _ = fmt.Fprintln(w, "\nEvery setting is a flag; see the configuration reference (orcon config check).")
	_, _ = fmt.Fprintln(w, "\nflags:")
	flags.SetOutput(w)
	flags.PrintDefaults()
	flags.SetOutput(io.Discard)
}

func logLevel(name string) slog.Level {
	switch name {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// newLogger builds the process logger: text on a terminal when the format is auto, JSON otherwise.
func newLogger(w io.Writer, format string, level *slog.LevelVar) *slog.Logger {
	opts := &slog.HandlerOptions{Level: level}
	if format == "text" || (format == "auto" && isTerminal(w)) {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}

// startReloadWatchers starts the two ways a running server rereads its
// configuration (R-CONF-09): the request file of orcon config reload and SIGHUP.
// Both goroutines end with ctx and are waited for through wg. A data directory
// that cannot hold the run directory turns the request file off with a warning.
func startReloadWatchers(ctx context.Context, wg *sync.WaitGroup, dataDir string, poll time.Duration, r *config.Reloader, logger *slog.Logger) {
	handle := func(string) cli.ReloadResult { return reloadOnce(ctx, r, logger) }
	if err := cli.EnsureRunDir(dataDir); err != nil {
		logger.WarnContext(ctx, "orcon config reload cannot reach this server", "error", err)
	} else {
		wg.Go(func() { cli.WatchReloadRequests(ctx, dataDir, poll, handle, logger) })
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP)
	wg.Go(func() {
		defer signal.Stop(sigs)
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigs:
				reloadOnce(ctx, r, logger)
			}
		}
	})
}

// reloadOnce rereads the configuration and says what happened, in the log and in the result for the command.
func reloadOnce(ctx context.Context, r *config.Reloader, logger *slog.Logger) cli.ReloadResult {
	report, err := r.Reload()
	res := cli.ReloadResult{AppliedKeys: report.Applied, RestartRequiredKeys: report.Pending, Errors: []string{}}
	if err != nil {
		for _, issue := range report.Issues {
			if issue.Severity == config.SeverityError {
				res.Errors = append(res.Errors, issue.Key+": "+issue.Message)
			}
		}
		logger.ErrorContext(ctx, "configuration reload refused; the running configuration is kept", "errors", res.Errors)
		return res
	}
	if len(report.Pending) > 0 {
		logger.WarnContext(ctx, "configuration changed in keys that need a restart; they keep their running value", "keys", report.Pending)
	}
	logger.InfoContext(ctx, "configuration reloaded", "applied", report.Applied, "needs_restart", report.Pending)
	return res
}

// serveSettings are the resolved settings serveWithSandbox needs.
type serveSettings struct {
	role            config.Role
	addr            string
	shutdownTimeout time.Duration
	maxAbandoned    int
	slots           int
	cfg             *config.Config
	openEngine      engineOpener
	// sec is the security wiring of the API roles; nil for the roles without an API.
	// The engine runs over its store, so the process holds one database handle.
	sec *security
}

// serveWithSandbox runs the HTTP server with the expression sandbox beside it.
// When the sandbox reports that it is saturated or holds a stuck call, the
// process shuts down through the ordinary graceful shutdown and exits with
// [exitRestart], so the restart policy of the deployment starts it again.
func serveWithSandbox(ctx context.Context, logger *slog.Logger, s serveSettings, newSandbox sandboxFactory) int {
	sb := newSandbox(s.maxAbandoned, s.slots, logger)
	effective := min(s.maxAbandoned, s.slots)
	defer func() { _ = sb.Close(context.WithoutCancel(ctx)) }()

	// The engine runs over the store the security wiring opened; that wiring closes it.
	var eng *engine
	if s.openEngine != nil && servesEngine(s.role) && s.sec != nil {
		var exprSB expr.Sandbox
		if es, ok := sb.(expr.Sandbox); ok {
			exprSB = es
		}
		var err error
		eng, err = s.openEngine(s.sec.store, s.cfg, exprSB, logger)
		if err != nil {
			logger.ErrorContext(ctx, "the execution engine could not start", "error", err)
			return exitError
		}
	}
	// The abandoned evaluations of a process can hold this much memory at most.
	logger.InfoContext(ctx, "expression sandbox limits",
		"slots", s.slots, "max_abandoned", effective, "memory_bound_bytes", uint64(s.slots)*uint64(expr.DefaultMemoryLimit)) //nolint:gosec // slots is validated to 1..GOMAXPROCS-1 by runServe

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

	var ingress httpapi.WebhookIngress
	var engineErr chan error
	if eng != nil {
		ingress = eng.ingress()
		engineErr = make(chan error, 1)
		go func() {
			err := eng.run(serveCtx)
			if err != nil {
				cancel()
			}
			engineErr <- err
		}()
	}

	err := serve(serveCtx, logger, s.role, s.addr, s.shutdownTimeout, s.sec, ingress)
	cancel()
	<-watched
	if engineErr != nil {
		if err2 := <-engineErr; err2 != nil {
			logger.ErrorContext(ctx, "the execution engine stopped with error", "error", err2)
			err = errors.Join(err, fmt.Errorf("execution engine: %w", err2))
		}
	}
	switch {
	case err != nil:
		logger.ErrorContext(ctx, "server stopped with error", "role", s.role, "error", err)
		return exitError
	case restart.Load():
		return exitRestart
	default:
		return exitOK
	}
}

// newHandler assembles the HTTP routes served by the standalone and api roles, with the
// refusing identity and authorization implementations.
func newHandler(role config.Role, logger *slog.Logger) (http.Handler, error) {
	return newHandlerWith(role, logger, nil, nil)
}

// newHandlerWith assembles the HTTP routes over the security wiring of the process. The public
// trigger addresses are served by webhooks when it is not nil.
func newHandlerWith(role config.Role, logger *slog.Logger, sec *security, webhooks httpapi.WebhookIngress) (http.Handler, error) {
	assets, err := webui.Assets()
	if err != nil {
		return nil, fmt.Errorf("build handler: %w", err)
	}
	mux := http.NewServeMux()
	mux.Handle(httpapi.HealthPath, httpapi.HealthHandler())
	if err := mountAPIWith(mux, role, logger, sec, webhooks); err != nil {
		return nil, fmt.Errorf("build handler: %w", err)
	}
	mux.Handle("/", webui.Handler(assets))
	return mux, nil
}

// serve listens on addr and serves HTTP until ctx is canceled, then drains
// in-flight requests for at most shutdownTimeout. With a security wiring, the audit
// sealer runs beside the server and stops with it.
func serve(ctx context.Context, logger *slog.Logger, role config.Role, addr string, shutdownTimeout time.Duration, sec *security, webhooks httpapi.WebhookIngress) error {
	handler, err := newHandlerWith(role, logger, sec, webhooks)
	if err != nil {
		return err
	}
	if sec != nil {
		// The sealer stops when serve returns for any reason, including a failed listener:
		// stopSealer runs before the wait, which is registered first and so runs last.
		sealCtx, stopSealer := context.WithCancel(ctx)
		sealed := make(chan struct{})
		go func() {
			defer close(sealed)
			sec.runSealer(sealCtx, logger)
		}()
		defer func() { <-sealed }()
		defer stopSealer()
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
