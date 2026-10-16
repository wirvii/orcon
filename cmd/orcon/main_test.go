// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/platform/config"
)

func TestMain(m *testing.M) {
	// The PostgreSQL test server runs for the whole package when it is on; the
	// leak check runs after the tests and the server stops in the cleanup, after it.
	stop := startPostgres()
	goleak.VerifyTestMain(m, goleak.Cleanup(func(int) { stop() }))
}

// syncBuffer is a bytes.Buffer safe for concurrent use: run writes logs from
// the server goroutine while the test reads them.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p) //nolint:wrapcheck // test double: errors pass through unchanged
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestRunVersion(t *testing.T) {
	// Not parallel: the test replaces the package variables the release build injects.
	oldVersion, oldCommit := version, commit
	t.Cleanup(func() { version, commit = oldVersion, oldCommit })
	version, commit = "1.2.3", "0123456789abcdef"

	for _, args := range [][]string{{"--version"}, {"version"}} {
		var stdout, stderr syncBuffer
		if code := run(t.Context(), args, &stdout, &stderr); code != exitOK {
			t.Fatalf("run(%q) exit code = %d, want %d (stderr %q)", args, code, exitOK, stderr.String())
		}
		if got, want := stdout.String(), "orcon 1.2.3 (0123456789abcdef)\n"; got != want {
			t.Fatalf("run(%q) stdout = %q, want %q", args, got, want)
		}
	}
}

func TestCommandsSortedAndUnique(t *testing.T) {
	t.Parallel()

	list := commands()
	if len(list) == 0 {
		t.Fatal("no commands")
	}
	for i, c := range list {
		if c.name == "" || c.summary == "" || c.run == nil {
			t.Errorf("command %d (%q) is incomplete", i, c.name)
		}
		if i > 0 && list[i-1].name >= c.name {
			t.Errorf("commands are not strictly sorted: %q comes after %q", c.name, list[i-1].name)
		}
	}
}

func TestUnknownSubcommandIsUsageError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		wantCode int
		out, err string
	}{
		{"no command", nil, exitUsage, "", "usage: orcon <command>"},
		{"unknown command", []string{"bogus"}, exitUsage, "", `unknown command "bogus"`},
		{"legacy root flags", []string{"--role=standalone"}, exitUsage, "", "unknown command"},
		{"help", []string{"help"}, exitOK, "serve", ""},
		{"-h", []string{"-h"}, exitOK, "config", ""},
		{"--help", []string{"--help"}, exitOK, "usage: orcon", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr syncBuffer
			if code := run(t.Context(), tt.args, &stdout, &stderr); code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.out) || !strings.Contains(stderr.String(), tt.err) {
				t.Fatalf("stdout %q stderr %q; want %q and %q", stdout.String(), stderr.String(), tt.out, tt.err)
			}
		})
	}
}

// serveEnv gives a server of a test its own data directory and returns the arguments that
// select PostgreSQL, so the api, scheduler and worker roles are valid. Nothing connects to it.
func serveEnv(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ORCON_DATA_DIR", dir)
	provisionMasterKey(t, dir)
	t.Setenv("ORCON_STORE_POSTGRES_DSN", "postgres://u:p@127.0.0.1:1/db")
	return []string{"serve", "--store-driver=postgres"}
}

func TestRun(t *testing.T) {
	t.Run("invalid role exits 5", func(t *testing.T) {
		serveEnv(t)
		var stdout, stderr syncBuffer
		code := run(t.Context(), []string{"serve", "--role=invalid"}, &stdout, &stderr)
		if code != exitValidation {
			t.Fatalf("exit code = %d, want %d", code, exitValidation)
		}
		if !strings.Contains(stderr.String(), "role") || !strings.Contains(stderr.String(), "must be one of") {
			t.Fatalf("stderr = %q, want the invalid role message", stderr.String())
		}
	})

	t.Run("unknown flag exits 2", func(t *testing.T) {
		var stdout, stderr syncBuffer
		if code := run(t.Context(), []string{"serve", "--nope"}, &stdout, &stderr); code != exitUsage {
			t.Fatalf("exit code = %d, want %d", code, exitUsage)
		}
	})

	t.Run("the legacy flags are gone", func(t *testing.T) {
		for _, flag := range []string{"--addr=127.0.0.1:0", "--shutdown-timeout=5s"} {
			var stdout, stderr syncBuffer
			if code := run(t.Context(), []string{"serve", flag}, &stdout, &stderr); code != exitUsage {
				t.Fatalf("serve %s exit code = %d, want %d (no aliases)", flag, code, exitUsage)
			}
		}
	})

	t.Run("help exits 0", func(t *testing.T) {
		var stdout, stderr syncBuffer
		if code := run(t.Context(), []string{"serve", "-h"}, &stdout, &stderr); code != exitOK {
			t.Fatalf("exit code = %d, want %d", code, exitOK)
		}
		for _, want := range []string{"-http-address", "-http-shutdown-timeout", "-expr-sandbox-slots", "-role"} {
			if !strings.Contains(stdout.String(), want) {
				t.Errorf("usage lacks %s", want)
			}
		}
	})

	for _, role := range []string{"scheduler", "worker"} {
		t.Run(role+" logs a notice and exits 0 without listening", func(t *testing.T) {
			args := append(serveEnv(t), "--role="+role)
			var stdout, stderr syncBuffer
			code := run(t.Context(), args, &stdout, &stderr)
			if code != exitOK {
				t.Fatalf("exit code = %d, want %d; stderr %s", code, exitOK, stderr.String())
			}
			if !strings.Contains(stderr.String(), "role has no processing yet") {
				t.Fatalf("stderr = %q, want the scaffold notice", stderr.String())
			}
			if strings.Contains(stderr.String(), "listen_addr") {
				t.Fatalf("stderr = %q, role must not listen", stderr.String())
			}
		})
	}

	t.Run("listen failure exits 1", func(t *testing.T) {
		// The store opens before the listener, so this case uses the SQLite file of the data directory;
		// SQLite serves the standalone role only (one process owns the file).
		_ = serveEnv(t)
		args := []string{"serve", "--store-driver=sqlite", "--role=standalone", "--http-address=256.256.256.256:0"}
		var stdout, stderr syncBuffer
		if code := run(t.Context(), args, &stdout, &stderr); code != exitError {
			t.Fatalf("exit code = %d, want %d", code, exitError)
		}
	})

	for _, role := range []string{"standalone", "api"} {
		t.Run(role+" serves and shuts down on cancel", func(t *testing.T) {
			_ = serveEnv(t)
			args := []string{"serve"}
			t.Setenv("ORCON_STORE_POSTGRES_DSN", "")
			if role == "api" {
				// The api role runs over PostgreSQL only; the process-level suite uses the
				// server named by ORCON_TEST_POSTGRES_DSN when one is configured.
				dsn := os.Getenv("ORCON_TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("the api role needs PostgreSQL: set ORCON_TEST_POSTGRES_DSN")
				}
				t.Setenv("ORCON_STORE_POSTGRES_DSN", dsn)
				args = []string{"serve", "--store-driver=postgres"}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var stdout, stderr syncBuffer
			done := make(chan int, 1)
			go func() {
				done <- run(ctx, append(args, "--role="+role, "--http-address=127.0.0.1:0", "--http-shutdown-timeout=5s"), &stdout, &stderr)
			}()

			base := "http://" + waitForListenAddr(t, &stderr, done)
			assertGet(t, base+"/healthz", http.StatusOK, "ok")
			assertGet(t, base+"/", http.StatusOK, "")

			cancel()
			select {
			case code := <-done:
				if code != exitOK {
					t.Fatalf("exit code = %d, want %d; stderr: %s", code, exitOK, stderr.String())
				}
			case <-time.After(10 * time.Second):
				t.Fatal("run did not return after cancellation")
			}
		})
	}
}

// TestServeReadsConfig proves that orcon serve takes role, http.address and the
// shutdown timeout from the configuration with the precedence of R-CONF-01:
// flag, then environment, then file, then default.
func TestServeReadsConfig(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	body := "http:\n  address: 256.256.256.256:1\n  shutdownTimeout: 7s\nlog:\n  level: debug\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	t.Setenv("ORCON_DATA_DIR", dataDir)
	provisionMasterKey(t, dataDir)

	cases := []struct {
		name string
		env  string
		args []string
	}{
		{"flag beats file", "", []string{"serve", "--config", file, "--http-address=127.0.0.1:0"}},
		{"environment beats file", "127.0.0.1:0", []string{"serve", "--config", file}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ORCON_HTTP_ADDRESS", tc.env)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var stdout, stderr syncBuffer
			done := make(chan int, 1)
			go func() { done <- run(ctx, tc.args, &stdout, &stderr) }()
			addr := waitForListenAddr(t, &stderr, done)
			if !strings.HasPrefix(addr, "127.0.0.1:") {
				t.Fatalf("listening on %q, want the address of the higher layer", addr)
			}
			if !strings.Contains(stderr.String(), `"level":"DEBUG"`) && !strings.Contains(stderr.String(), `"level":"INFO"`) {
				t.Errorf("no log record: %s", stderr.String())
			}
			cancel()
			if code := <-done; code != exitOK {
				t.Fatalf("exit code = %d; stderr %s", code, stderr.String())
			}
		})
	}

	t.Run("file alone is used", func(t *testing.T) {
		t.Setenv("ORCON_HTTP_ADDRESS", "")
		var stdout, stderr syncBuffer
		if code := run(t.Context(), []string{"serve", "--config", file}, &stdout, &stderr); code != exitError {
			t.Fatalf("exit code = %d, want %d: the file's unusable address must be the one tried; stderr %s", code, exitError, stderr.String())
		}
		if !strings.Contains(stderr.String(), "256.256.256.256:1") {
			t.Fatalf("stderr = %q, want the address from the file", stderr.String())
		}
	})

	t.Run("the default listens on loopback", func(t *testing.T) {
		cfg := config.Default().Load(config.Input{Environ: []string{"ORCON_DATA_DIR=/d"}, GOOS: "linux"})
		if got := cfg.Config.Value("http.address").Text(); got != "127.0.0.1:8080" {
			t.Fatalf("default http.address = %q", got)
		}
	})
}

// waitForListenAddr polls the structured log until the server reports the
// address it bound, failing if run exits first or the wait times out.
func waitForListenAddr(t *testing.T, stderr *syncBuffer, done <-chan int) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case code := <-done:
			t.Fatalf("run exited early with %d; stderr: %s", code, stderr.String())
		default:
		}
		sc := bufio.NewScanner(strings.NewReader(stderr.String()))
		for sc.Scan() {
			var rec struct {
				Msg        string `json:"msg"`
				ListenAddr string `json:"listen_addr"`
			}
			if json.Unmarshal(sc.Bytes(), &rec) == nil && rec.Msg == "listening" && rec.ListenAddr != "" {
				return rec.ListenAddr
			}
		}
		time.Sleep(10 * time.Millisecond) // polling a real server socket: there is no injected clock in the composition root
	}
	t.Fatalf("server never reported its address; stderr: %s", stderr.String())
	return ""
}

func assertGet(t *testing.T, url string, wantStatus int, wantBody string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = res.Body.Close() }() // nothing to recover from a failed close in a test
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	if res.StatusCode != wantStatus {
		t.Fatalf("GET %s status = %d, want %d", url, res.StatusCode, wantStatus)
	}
	if wantBody != "" && string(body) != wantBody {
		t.Fatalf("GET %s body = %q, want %q", url, body, wantBody)
	}
	client.CloseIdleConnections()
}
