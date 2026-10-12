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
	"strings"
	"syscall"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/adapter/webui"
	"github.com/wirvii/orcon/internal/platform/config"
)

// Exit codes returned by run.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is the testable body of the process. It returns the exit code and never
// calls os.Exit. The process runs until ctx is cancelled, then shuts down
// within the configured shutdown timeout.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("orcon", flag.ContinueOnError)
	fs.SetOutput(stderr)
	role := fs.String("role", "standalone", "standalone|api|scheduler|worker")
	addr := fs.String("addr", ":8080", "HTTP listen address")
	shutdownTimeout := fs.Duration("shutdown-timeout", defaultShutdownTimeout, "maximum time to drain in-flight requests on shutdown")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}

	selected := config.Role(strings.ToLower(*role))
	if !selected.Valid() {
		fmt.Fprintln(stderr, "invalid role:", *role)
		return exitUsage
	}

	logger := slog.New(slog.NewJSONHandler(stderr, nil))

	switch selected {
	case config.RoleStandalone, config.RoleAPI:
		if err := serve(ctx, logger, selected, *addr, *shutdownTimeout); err != nil {
			logger.ErrorContext(ctx, "server stopped with error", "role", selected, "error", err)
			return exitError
		}
		return exitOK
	case config.RoleScheduler, config.RoleWorker:
		logger.InfoContext(ctx, "role has no processing yet", "role", selected)
		return exitOK
	default:
		// Unreachable: Valid rejected every other value above.
		return exitUsage
	}
}

// newHandler assembles the HTTP routes served by the standalone and api roles.
func newHandler() (http.Handler, error) {
	assets, err := webui.Assets()
	if err != nil {
		return nil, fmt.Errorf("build handler: %w", err)
	}
	mux := http.NewServeMux()
	mux.Handle(httpapi.HealthPath, httpapi.HealthHandler())
	mux.Handle("/", http.FileServer(http.FS(assets)))
	return mux, nil
}

// serve listens on addr and serves HTTP until ctx is cancelled, then drains
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
	// Shutdown must outlive the cancelled ctx, so detach cancellation and bound it explicitly.
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
