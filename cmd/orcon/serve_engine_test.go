// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/egress"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/test/mockprovider"
)

// TestServeStandaloneRunsWebhookExecution starts the server the way the binary
// does (runServe and the production sandbox) on a SQLite file that holds a
// published webhook flow. A public request starts the execution, the workers run
// it, and after a graceful shutdown the database shows it SUCCEEDED. The network
// policy of the outgoing call admits the loopback mock provider; everything else
// is the production wiring.
func TestServeStandaloneRunsWebhookExecution(t *testing.T) {
	ctx := t.Context()
	t.Setenv("ORCON_DATA_DIR", t.TempDir())
	dbPath := filepath.Join(t.TempDir(), "orcon.db")
	provider := mockprovider.New()
	providerSrv := httptest.NewServer(provider)
	t.Cleanup(providerSrv.Close)

	org, _ := publishOnDisk(t, dbPath, providerSrv.URL)

	addr := freeLoopbackAddr(t)
	deps := serveDeps{
		newSandbox: productionSandbox,
		environ:    os.Environ(),
		reloadPoll: 20 * time.Millisecond,
		openEngine: loopbackEngine,
	}
	var stdout, stderr syncBuffer
	runCtx, cancelServer := context.WithCancel(ctx)
	exited := make(chan int, 1)
	args := []string{"--http-address=" + addr, "--store-sqlite-path=" + dbPath, "--http-shutdown-timeout=10s", "--log-level=debug"}
	go func() { exited <- runServe(runCtx, args, &stdout, &stderr, deps) }()
	// stopServer shuts the server down once and returns its exit code.
	var code int
	var stopped bool
	stopServer := func() int {
		if !stopped {
			cancelServer()
			code = <-exited
			stopped = true
		}
		return code
	}
	t.Cleanup(func() { stopServer() })

	waitHealthy(t, addr, &stderr)
	post, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+addr+"/hooks/"+projectSlug+"/orders", strings.NewReader(`{"amount": 42}`))
	if err != nil {
		t.Fatal(err)
	}
	post.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(post)
	if err != nil {
		t.Fatalf("post the event: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	execRef := resp.Header.Get("Orcon-Execution-Id")
	waitProviderRequests(t, provider, 1)

	if code := stopServer(); code != exitOK {
		t.Fatalf("runServe exited with %d; stderr:\n%s", code, stderr.String())
	}

	st, err := sqlite.Open(ctx, sqlite.Options{Path: dbPath, BinaryVersion: "0.0.0"})
	if err != nil {
		t.Fatalf("reopen the database: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	u, err := id.Parse(execRef)
	if err != nil {
		t.Fatalf("parse the execution id: %v", err)
	}
	ex, err := st.GetExecution(ctx, org, store.ExecutionID(u))
	if err != nil {
		t.Fatalf("read the execution: %v", err)
	}
	if ex.Status != string(execution.ExecSucceeded) {
		t.Fatalf("execution status %q, want SUCCEEDED; stderr:\n%s", ex.Status, stderr.String())
	}
}

// projectSlug is the slug of the project the on-disk setup creates.
const projectSlug = "engine"

// publishOnDisk creates the organization, the project and one published, active
// webhook flow in the database file, then closes the file so the server can open it.
func publishOnDisk(t *testing.T, dbPath, providerURL string) (identity.OrgID, identity.ProjectID) {
	t.Helper()
	st, err := sqlite.Open(t.Context(), sqlite.Options{Path: dbPath, BinaryVersion: "0.0.0"})
	if err != nil {
		t.Fatalf("open the database: %v", err)
	}
	env := storetest.NewEngineEnvOn(t, storetest.SQLite, st, storetest.EngineOptions{})
	env.Setup()
	eng, err := newEngine(st, engineOptions{
		role: config.RoleStandalone, logger: slog.New(slog.DiscardHandler), version: "test",
		senderPolicy: loopbackPolicy, credentials: testCredentials,
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	publishWebhookFlow(t, env, eng, providerURL)
	if err := st.Close(); err != nil {
		t.Fatalf("close the database: %v", err)
	}
	return env.Org, env.Project
}

// loopbackEngine is the engine opener of the test: the production opener with a
// network policy that admits the loopback provider and the test credentials.
func loopbackEngine(ctx context.Context, cfg *config.Config, sandbox expr.Sandbox, logger *slog.Logger) (*engine, func() error, error) {
	st, closeStore, err := openStore(ctx, cfg, "test")
	if err != nil {
		return nil, nil, err
	}
	eng, err := newEngine(st, engineOptions{
		role: cfg.Role(), logger: logger, version: "test", sandbox: sandbox,
		senderPolicy: loopbackPolicy, credentials: testCredentials,
	})
	if err != nil {
		_ = closeStore()
		return nil, nil, err
	}
	return eng, closeStore, nil
}

// loopbackPolicy grants the host of a destination over plain http, private
// addresses included, for the mock provider on 127.0.0.1.
func loopbackPolicy(u *url.URL) egress.Policy {
	return egress.Policy{Domains: []string{u.Hostname()}, Schemes: []string{"http"}, AllowPrivateNetworks: true}
}

func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func waitHealthy(t *testing.T, addr string, logs *syncBuffer) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/healthz", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the server at %s did not become healthy; stderr:\n%s", addr, logs.String())
}

func waitProviderRequests(t *testing.T, p *mockprovider.Provider, want int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if len(p.Requests()) >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the provider received %d requests, want %d", len(p.Requests()), want)
}
