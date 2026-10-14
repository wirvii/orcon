// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// TestConfigCheckAgreesWithServe is the reason serve reads the same schema: what
// check reports as effective is what the server uses.
func TestConfigCheckAgreesWithServe(t *testing.T) {
	t.Setenv("ORCON_DATA_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := run(t.Context(), []string{"config", "check", "-o", "table"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d: %s", code, stdout.String())
	}
	var line string
	for l := range strings.SplitSeq(stdout.String(), "\n") {
		if strings.HasPrefix(l, "http.address ") {
			line = l
		}
	}
	if !strings.Contains(line, "127.0.0.1:8080") || !strings.Contains(line, "default") {
		t.Fatalf("check line = %q, want the loopback default", line)
	}
}

func TestProbeDatabase(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ORCON_DATA_DIR", dir)

	var stdout, stderr bytes.Buffer
	code := run(t.Context(), []string{"config", "check", "--probe", "-o", "table"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "will be created on first start") || !strings.Contains(stdout.String(), "database (store.driver): OK") {
		t.Fatalf("a missing SQLite file is not a failure; exit %d:\n%s", code, stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "orcon.db")); !os.IsNotExist(err) {
		t.Fatalf("the probe created the database: %v", err)
	}

	stdout.Reset()
	t.Setenv("ORCON_STORE_DRIVER", "postgres")
	t.Setenv("ORCON_STORE_POSTGRES_DSN", "postgres://u:SECRETPW@127.0.0.1:1/db?connect_timeout=1")
	code = run(t.Context(), []string{"config", "check", "--probe", "-o", "table", "--role", "api"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("a failed probe exits 1, got %d:\n%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAILED") || strings.Contains(stdout.String(), "SECRETPW") {
		t.Fatalf("output:\n%s", stdout.String())
	}
}
