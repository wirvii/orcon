// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// Defaults of LaunchConfig.
const (
	defaultStartTimeout = 30 * time.Second
	stopGrace           = 10 * time.Second
	healthPollInterval  = 50 * time.Millisecond
)

// LaunchConfig describes how to start the real binary.
type LaunchConfig struct {
	// Binary is the path of the orcon binary (ORCON_BIN, default bin/orcon).
	Binary string
	// Args are the arguments before the listen address ("--addr <addr>" is
	// appended). The default replaces them with
	// "serve --role standalone --http-address <port> --data-dir <tmp>".
	Args []string
	// Env is added to the environment of the process (KEY=value), for example the
	// database settings of the engine under test.
	Env []string
	// Dir is the working directory; empty uses the current one.
	Dir string
	// Stderr receives the process log; nil discards it.
	Stderr io.Writer
	// StartTimeout bounds the wait for /healthz; default 30 s.
	StartTimeout time.Duration
}

// Server is a running binary started by Launch.
type Server struct {
	// BaseURL is the address the binary listens on.
	BaseURL string
	cancel  context.CancelFunc
	done    chan error
}

// Launch starts the binary on a free loopback port and waits for /healthz.
// The caller must call Stop.
func Launch(ctx context.Context, cfg LaunchConfig) (*Server, error) {
	if cfg.Binary == "" {
		return nil, errors.New("launch: Binary is required")
	}
	if cfg.StartTimeout <= 0 {
		cfg.StartTimeout = defaultStartTimeout
	}
	addr, err := freeLoopbackAddr(ctx)
	if err != nil {
		return nil, err
	}
	args := append(append([]string{}, cfg.Args...), "--addr", addr)
	if cfg.Args == nil { // the default is a real serve invocation, which takes the address as --http-address
		dataDir, err := os.MkdirTemp("", "orcon-examples-")
		if err != nil {
			return nil, fmt.Errorf("launch: data dir: %w", err)
		}
		args = []string{"serve", "--role", "standalone", "--http-address", addr, "--data-dir", dataDir}
	}
	procCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	cmd := exec.CommandContext(procCtx, cfg.Binary, args...) //nolint:gosec // the binary path is chosen by the test harness, not by input
	cmd.Dir = cfg.Dir
	cmd.Env = append(os.Environ(), cfg.Env...)
	cmd.Stderr = cfg.Stderr
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = stopGrace
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("launch %s: %w", cfg.Binary, err)
	}
	s := &Server{BaseURL: "http://" + addr, cancel: cancel, done: make(chan error, 1)}
	// The goroutine is owned by Stop, which receives from done.
	go func() { s.done <- cmd.Wait() }()
	if err := s.waitHealthy(ctx, cfg.StartTimeout); err != nil {
		_ = s.Stop()
		return nil, err
	}
	return s, nil
}

// Stop asks the process to shut down, waits for it and reports a failure to
// exit cleanly. It is safe to call once.
func (s *Server) Stop() error {
	s.cancel()
	err := <-s.done
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		// Dying from a signal (exit code -1) is the stop; a process that exits on its own with a code is not.
		if exit.ExitCode() == -1 {
			return nil
		}
		return fmt.Errorf("stop server: exit code %d: %w", exit.ExitCode(), err)
	}
	if errors.Is(err, context.Canceled) {
		// The interrupt was delivered and the process exited with code 0.
		return nil
	}
	if err != nil {
		return fmt.Errorf("stop server: %w", err)
	}
	return nil
}

func (s *Server) waitHealthy(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := &http.Client{Timeout: time.Second}
	for {
		select {
		case err := <-s.done:
			s.done <- err
			return fmt.Errorf("the binary exited before it was healthy: %w", err)
		default:
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL+"/healthz", nil)
		if err != nil {
			return fmt.Errorf("health request: %w", err)
		}
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("the binary was not healthy within %s", timeout)
		case <-time.After(healthPollInterval):
		}
	}
}

// freeLoopbackAddr asks the kernel for a free loopback port. The port can be
// taken again before the binary binds it; the health wait reports that case.
func freeLoopbackAddr(ctx context.Context) (string, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("find a free port: %w", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		return "", fmt.Errorf("release the probe port: %w", err)
	}
	return addr, nil
}
