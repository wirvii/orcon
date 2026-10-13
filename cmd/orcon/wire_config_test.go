// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/platform/config"
)

func TestConfigCheckThroughTheBinaryEntryPoint(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "dsn")
	if err := os.WriteFile(secret, []byte("postgres://u:TOPSECRET@db.example.com/orcon\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "config.yaml")
	body := "role: api\nstore:\n  driver: postgres\n  postgres:\n    dsnFile: " + secret + "\nhttp:\n  address: 0.0.0.0:9000\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCON_STORE_POSTGRES_MAX_CONNECTIONS", "12")
	t.Setenv("ORCON_DATA_DIR", dir)

	var stdout, stderr bytes.Buffer
	code := run(t.Context(), []string{"config", "check", "--config", file, "-o", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stdout %s stderr %s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "TOPSECRET") {
		t.Fatal("the output contains the secret")
	}
	var rep struct {
		Entries []struct{ Key, Source string } `json:"entries"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, e := range rep.Entries {
		sources[e.Key] = e.Source
	}
	for key, want := range map[string]string{
		"store.postgres.maxConnections": "env:ORCON_STORE_POSTGRES_MAX_CONNECTIONS",
		"http.address":                  "file:" + file + ":7",
		"store.postgres.dsn":            "file:" + secret + " (file:" + file + ":5)",
		"log.level":                     "default",
	} {
		if sources[key] != want {
			t.Errorf("source of %s = %q, want %q", key, sources[key], want)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(t.Context(), []string{"config", "check", "--config", file, "--role", "worker", "-o", "table"}, &stdout, &stderr); code != 0 {
		t.Fatalf("a worker with PostgreSQL must be valid, exit = %d: %s", code, stdout.String())
	}
	stdout.Reset()
	if code := run(t.Context(), []string{"config", "check", "--config", file, "--store-driver", "sqlite", "-o", "yaml"}, &stdout, &stderr); code != 5 {
		t.Fatalf("sqlite with role api must exit 5, got %d", code)
	}
	if !strings.Contains(stdout.String(), "severity: error") {
		t.Errorf("YAML output lacks the error:\n%s", stdout.String())
	}
}

func TestServerFlagsAreNotMistakenForCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code, handled := runSubcommand(t.Context(), []string{"--version"}, &stdout, &stderr); handled || code != 0 {
		t.Fatalf("handled = %v code = %d, want the server flags left alone", handled, code)
	}
}

func TestDSNHostPort(t *testing.T) {
	t.Parallel()

	tests := []struct{ dsn, host, port string }{
		{"postgres://u:p@db.example.com:6543/x?sslmode=require", "db.example.com", "6543"},
		{"postgres://u:p@db.example.com/x", "db.example.com", "5432"},
		{"postgresql://u@[::1]:5433/x", "::1", "5433"},
		{"postgres://u@a:1,b:2/x", "a", "1"},
		{"host=db port=6000 user=u password=p dbname=x", "db", "6000"},
		{"host=db user=u", "db", "5432"},
		{"user=u", "", "5432"},
		{"postgres://%zz", "", "5432"},
	}
	for _, tt := range tests {
		host, port := dsnHostPort(tt.dsn)
		if host != tt.host || port != tt.port {
			t.Errorf("dsnHostPort(%q) = %q, %q; want %q, %q", tt.dsn, host, port, tt.host, tt.port)
		}
	}
}

func TestDialDSN(t *testing.T) {
	t.Parallel()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	if err := dialDSN(t.Context(), "postgres://u:pw@127.0.0.1:"+port+"/x"); err != nil {
		t.Fatalf("dial a listening port: %v", err)
	}
	_ = ln.Close()
	err = dialDSN(t.Context(), "postgres://u:SECRETPW@127.0.0.1:"+port+"/x")
	if err == nil || strings.Contains(err.Error(), "SECRETPW") {
		t.Fatalf("dial a closed port = %v, want an error without the password", err)
	}
	if err := dialDSN(t.Context(), "user=u"); err == nil {
		t.Fatal("a connection string without a host must fail")
	}
}

func TestCheckBackupTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkBackupTarget("file://" + filepath.ToSlash(dir)); err != nil {
		t.Errorf("an existing directory: %v", err)
	}
	if err := checkBackupTarget("file://" + filepath.ToSlash(file)); err == nil {
		t.Error("a file is not a backup directory")
	}
	if err := checkBackupTarget("file://" + filepath.ToSlash(filepath.Join(dir, "missing"))); err == nil {
		t.Error("a missing directory must fail")
	}
	if err := checkBackupTarget("s3://bucket/p"); err == nil {
		t.Error("s3 has no probe yet and must say so")
	}
}

func TestPIDFileAndReloadSignal(t *testing.T) {
	dir := t.TempDir()
	cfg := configWithDataDir(t, dir)

	if err := signalReload(t.Context(), cfg); err == nil || !strings.Contains(err.Error(), "no running local instance") {
		t.Fatalf("without a pid file: %v", err)
	}

	hup := make(chan os.Signal, 1)
	sigs, stop := notifyReload()
	defer stop()
	go func() { hup <- <-sigs }()

	release, err := claimPIDFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, pidFileName)) //nolint:gosec // G304: a path inside t.TempDir
	if err != nil || strings.TrimSpace(string(data)) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("pid file = %q, %v", data, err)
	}
	if err := signalReload(t.Context(), cfg); err != nil {
		t.Fatalf("signalReload: %v", err)
	}
	select {
	case <-hup:
	case <-time.After(5 * time.Second):
		t.Fatal("SIGHUP never arrived")
	}
	release()
	if _, err := os.Stat(filepath.Join(dir, pidFileName)); !os.IsNotExist(err) {
		t.Fatalf("release left the pid file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, pidFileName), []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := signalReload(t.Context(), cfg); err == nil || !strings.Contains(err.Error(), "process id") {
		t.Fatalf("garbage pid file: %v", err)
	}
}

func TestReloadLoop(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	loads := 0
	schema := config.Default()
	loadFn := func() *config.Result {
		mu.Lock()
		defer mu.Unlock()
		loads++
		return schema.Load(config.Input{Environ: []string{"ORCON_DATA_DIR=/d"}, GOOS: "linux"})
	}
	r := config.NewReloader(loadFn().Config, loadFn)
	sigs := make(chan os.Signal)
	ctx, cancel := context.WithCancel(t.Context())
	var logs bytes.Buffer
	done := make(chan struct{})
	go func() {
		defer close(done)
		reloadLoop(ctx, sigs, r, slog.New(slog.NewTextHandler(&logs, nil)))
	}()
	sigs <- syscall.SIGHUP
	sigs <- syscall.SIGHUP // the loop reads the second only after the first reload is done
	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	if loads != 3 {
		t.Fatalf("loads = %d, want 1 initial + 2 reloads", loads)
	}
	if !strings.Contains(logs.String(), "configuration reloaded") {
		t.Fatalf("logs = %q", logs.String())
	}
}

func configWithDataDir(t *testing.T, dir string) *config.Config {
	t.Helper()
	res := config.Default().Load(config.Input{Environ: []string{"ORCON_DATA_DIR=" + dir}, GOOS: "linux"})
	if err := res.Err(); err != nil {
		t.Fatal(err)
	}
	return res.Config
}
