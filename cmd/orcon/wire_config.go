// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/wirvii/orcon/internal/adapter/cli"
	"github.com/wirvii/orcon/internal/adapter/codec/configyaml"
	"github.com/wirvii/orcon/internal/platform/config"
)

// pidFileName is the file the running local instance writes in its data
// directory, so orcon config reload can find it.
const pidFileName = "orcon.pid"

// runSubcommand runs a command of the cli adapter (orcon config ..., orcon docs
// generate) when args name one. Later cuts add their commands in the cli package;
// this is the one place that wires them.
func runSubcommand(ctx context.Context, args []string, stdout, stderr io.Writer) (code int, handled bool) {
	return cli.Run(ctx, args, cliEnv(stdout, stderr))
}

func cliEnv(stdout, stderr io.Writer) cli.Env {
	return cli.Env{
		Stdout:      stdout,
		Stderr:      stderr,
		Environ:     os.Environ(),
		FS:          config.OSFileSystem{},
		Decode:      configyaml.Decode,
		MarshalYAML: configyaml.Marshal,
		GOOS:        runtime.GOOS,
		IsTerminal:  isTerminal(stdout),
		Probes:      configProbes(),
		Reload:      signalReload,
	}
}

// isTerminal reports whether w is a character device, which is what a terminal is.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// configProbes are the connection checks of orcon config check --probe. They
// read and never write. The database check only proves the network path; an
// authenticated check belongs to the PostgreSQL adapter, and the key management
// service and S3 checks arrive with the cuts that own those clients.
func configProbes() []cli.Probe {
	return []cli.Probe{
		{
			Name:    "database (network reachability)",
			Applies: func(c *config.Config) bool { return c.Value("store.driver").Text() == "postgres" },
			Run: func(ctx context.Context, c *config.Config) error {
				return dialDSN(ctx, c.Secret("store.postgres.dsn").Reveal())
			},
		},
		{
			Name: "key management service",
			Applies: func(c *config.Config) bool {
				p := c.Value("keys.master.provider").Text()
				return p != "file" && p != "env"
			},
			Run: func(context.Context, *config.Config) error { return cli.ErrProbeUnavailable },
		},
		{
			Name:    "backup target",
			Applies: func(c *config.Config) bool { return c.Value("backup.target").Text() != "" },
			Run: func(_ context.Context, c *config.Config) error {
				return checkBackupTarget(c.Value("backup.target").Text())
			},
		},
	}
}

// dialDSN opens a TCP connection to the host of a PostgreSQL connection string and closes it.
// Errors never contain the string: it holds a password.
func dialDSN(ctx context.Context, dsn string) error {
	host, port := dsnHostPort(dsn)
	if host == "" {
		return errors.New("the connection string has no host")
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return fmt.Errorf("cannot reach the database host: %w", err)
	}
	return conn.Close()
}

// dsnHostPort extracts the first host and port from a URL or key=value connection string.
func dsnHostPort(dsn string) (host, port string) {
	port = "5432"
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", port
		}
		hostPart := u.Host
		if i := strings.IndexByte(hostPart, ','); i >= 0 {
			hostPart = hostPart[:i]
		}
		if h, p, err := net.SplitHostPort(hostPart); err == nil {
			return h, p
		}
		return strings.Trim(hostPart, "[]"), port
	}
	for field := range strings.FieldsSeq(dsn) {
		name, value, ok := strings.Cut(field, "=")
		switch {
		case ok && name == "host":
			host, _, _ = strings.Cut(value, ",")
		case ok && name == "port":
			port, _, _ = strings.Cut(value, ",")
		}
	}
	return host, port
}

// checkBackupTarget checks that a file target exists and is a directory. An S3
// target needs the S3 client of the backup cut.
func checkBackupTarget(target string) error {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "file" {
		return cli.ErrProbeUnavailable
	}
	info, err := os.Stat(filepath.FromSlash(u.Path))
	switch {
	case err != nil:
		return fmt.Errorf("the backup directory is not reachable: %w", err)
	case !info.IsDir():
		return errors.New("the backup target is not a directory")
	default:
		return nil
	}
}

// claimPIDFile writes the pid of this process to <dataDir>/orcon.pid and returns
// the function that removes it. The server calls it at start so orcon config
// reload can find the instance.
func claimPIDFile(dataDir string) (release func(), err error) {
	path := filepath.Join(dataDir, pidFileName)
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write pid file: %w", err)
	}
	return func() { _ = os.Remove(path) }, nil // best effort: a stale file is detected when read
}

// signalReload asks the local instance to reread its configuration by sending
// SIGHUP to the pid in its pid file (R-CONF-09). Windows has no SIGHUP, so it
// needs another channel, which is not decided yet.
func signalReload(_ context.Context, cfg *config.Config) error {
	if runtime.GOOS == "windows" {
		return errors.New("reloading a running instance is not available on Windows yet; restart the service to apply changes")
	}
	path := filepath.Join(cfg.Value("dataDir").Text(), pidFileName)
	data, err := os.ReadFile(path) //nolint:gosec // G304: the path is the pid file in the operator's own data directory
	if err != nil {
		return fmt.Errorf("no running local instance: cannot read %s (is the server running with this dataDir?)", path)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return fmt.Errorf("%s does not hold a process id", path)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	if err := proc.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("process %d did not accept the reload request (stale %s?): %w", pid, pidFileName, err)
	}
	return nil
}

// reloadLoop rereads the configuration each time a signal arrives on sigs, until
// ctx is canceled. A failed reload keeps the running configuration and is logged
// once. It returns when ctx is canceled; the caller runs it in a goroutine it waits for.
func reloadLoop(ctx context.Context, sigs <-chan os.Signal, r *config.Reloader, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-sigs:
			report, err := r.Reload()
			switch {
			case err != nil:
				logger.ErrorContext(ctx, "configuration reload failed; the running configuration is kept", "error", err)
			default:
				logger.InfoContext(ctx, "configuration reloaded", "applied", report.Applied, "needs_restart", report.Pending)
			}
		}
	}
}

// notifyReload delivers SIGHUP to a channel for reloadLoop and returns the function that stops the delivery.
func notifyReload() (sigs <-chan os.Signal, stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	return ch, func() { signal.Stop(ch) }
}
