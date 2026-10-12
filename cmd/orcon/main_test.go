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
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
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

func TestRun(t *testing.T) {
	t.Run("invalid role exits 2", func(t *testing.T) {
		var stdout, stderr syncBuffer
		code := run(t.Context(), []string{"--role=invalid"}, &stdout, &stderr)
		if code != exitUsage {
			t.Fatalf("exit code = %d, want %d", code, exitUsage)
		}
		if !strings.Contains(stderr.String(), "invalid role: invalid") {
			t.Fatalf("stderr = %q, want the invalid role message", stderr.String())
		}
	})

	t.Run("unknown flag exits 2", func(t *testing.T) {
		var stdout, stderr syncBuffer
		if code := run(t.Context(), []string{"--nope"}, &stdout, &stderr); code != exitUsage {
			t.Fatalf("exit code = %d, want %d", code, exitUsage)
		}
	})

	t.Run("help exits 0", func(t *testing.T) {
		var stdout, stderr syncBuffer
		if code := run(t.Context(), []string{"-h"}, &stdout, &stderr); code != exitOK {
			t.Fatalf("exit code = %d, want %d", code, exitOK)
		}
	})

	for _, role := range []string{"scheduler", "worker"} {
		t.Run(role+" logs a notice and exits 0 without listening", func(t *testing.T) {
			var stdout, stderr syncBuffer
			code := run(t.Context(), []string{"--role=" + role}, &stdout, &stderr)
			if code != exitOK {
				t.Fatalf("exit code = %d, want %d", code, exitOK)
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
		var stdout, stderr syncBuffer
		code := run(t.Context(), []string{"--addr=256.256.256.256:0"}, &stdout, &stderr)
		if code != exitError {
			t.Fatalf("exit code = %d, want %d", code, exitError)
		}
	})

	for _, role := range []string{"standalone", "api"} {
		t.Run(role+" serves and shuts down on cancel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var stdout, stderr syncBuffer
			done := make(chan int, 1)
			go func() {
				done <- run(ctx, []string{"--role=" + role, "--addr=127.0.0.1:0", "--shutdown-timeout=5s"}, &stdout, &stderr)
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
