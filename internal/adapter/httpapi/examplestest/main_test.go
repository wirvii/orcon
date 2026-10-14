// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/httpapi/examplestest"
)

// helperEnv switches the test binary into a tiny server, so Launch can be tested
// against a real process without the product binary.
const helperEnv = "EXAMPLESTEST_HELPER_SERVER"

func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) != "" {
		// The helper process serves until interrupted. A failure panics, which ends the
		// process with a non-zero code that Stop reports; os.Exit is not allowed here.
		if code := runHelperServer(); code != 0 {
			panic(fmt.Sprintf("helper server failed with code %d", code))
		}
		return
	}
	goleak.VerifyTestMain(m)
}

// runHelperServer serves /healthz on --addr until it gets an interrupt.
func runHelperServer() int {
	fs := flag.NewFlagSet("helper", flag.ContinueOnError)
	_ = fs.String("role", "", "")
	addr := fs.String("addr", "", "")
	args := os.Args[1:]
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", *addr)
	if err != nil {
		return 1
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return 1
	}
	return 0
}

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}

func TestLaunchStartsAndStopsABinary(t *testing.T) {
	t.Setenv(helperEnv, "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	srv, err := examplestest.Launch(t.Context(), examplestest.LaunchConfig{
		Binary:       exe,
		Args:         []string{"--", "--role", "standalone"},
		StartTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if !strings.HasPrefix(srv.BaseURL, "http://127.0.0.1:") {
		t.Errorf("BaseURL = %q, want a loopback address", srv.BaseURL)
	}
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.BaseURL+"/healthz", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := (&http.Client{Transport: transport}).Do(req)
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz status = %d, want 200", resp.StatusCode)
	}
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestLaunchReportsABinaryThatExits(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	// Without the helper variable the test binary treats the arguments as test flags and exits.
	_, err = examplestest.Launch(t.Context(), examplestest.LaunchConfig{Binary: exe, Args: []string{"-test.run=^$"}, StartTimeout: 10 * time.Second})
	if err == nil {
		t.Fatal("Launch succeeded for a binary that exits at once")
	}
}

func TestLaunchRequiresABinary(t *testing.T) {
	if _, err := examplestest.Launch(t.Context(), examplestest.LaunchConfig{}); err == nil {
		t.Fatal("Launch without a binary succeeded")
	}
}
