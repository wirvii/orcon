// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/cli"
)

// reloadServer is a running server of a test, with the configuration file it reads.
type reloadServer struct {
	file    string
	dataDir string
	stderr  *syncBuffer
	done    chan int
	cancel  context.CancelFunc
	// registered counts the servers registered once this one is up.
	registered int
}

func registrations(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(cli.RunDir(dir), "server.*"))
	return matches
}

// startReloadServer starts orcon serve on a loopback port with a configuration file and a fast poll.
func startReloadServer(t *testing.T, body string) *reloadServer {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ORCON_DATA_DIR", dir)
	t.Setenv("ORCON_CONFIG", file)
	return startServerIn(t, dir, file)
}

// startServerIn starts one more server over the data directory and file of the test.
// Extra environment entries apply to this server only.
func startServerIn(t *testing.T, dir, file string, extraEnv ...string) *reloadServer {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	s := &reloadServer{registered: len(registrations(dir)), file: file, dataDir: dir, stderr: &syncBuffer{}, done: make(chan int, 1), cancel: cancel}
	var stdout syncBuffer
	deps := serveDeps{newSandbox: productionSandbox, environ: append(os.Environ(), extraEnv...), reloadPoll: 10 * time.Millisecond}
	go func() {
		s.done <- runServe(ctx, []string{"--http-address=127.0.0.1:0"}, &stdout, s.stderr, deps)
	}()
	waitForListenAddr(t, s.stderr, s.done)
	waitFor(t, "the server to register", func() bool { return len(registrations(dir)) > s.registered })
	s.registered = len(registrations(dir))
	t.Cleanup(func() { s.stop(t) })
	return s
}

func (s *reloadServer) stop(t *testing.T) {
	t.Helper()
	s.cancel()
	select {
	case code := <-s.done:
		if code != exitOK {
			t.Errorf("server exit code = %d; stderr %s", code, s.stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Error("the server did not stop")
	}
}

func (s *reloadServer) write(t *testing.T, body string) {
	t.Helper()
	if err := os.WriteFile(s.file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond) // polling a real file and a real server: there is no injected clock in the composition root
	}
	t.Fatalf("timed out waiting for %s", what)
}

// reload runs orcon config reload -o json against the data directory of s.
func reload(t *testing.T) (code int, res cli.ReloadResult, stderr string) {
	t.Helper()
	code, rep, stderr := reloadAll(t)
	if len(rep.Servers) > 0 {
		res = rep.Servers[0]
	}
	return code, res, stderr
}

// reloadAll runs orcon config reload -o json and returns the whole report.
func reloadAll(t *testing.T) (code int, rep cli.ReloadReport, stderr string) {
	t.Helper()
	var stdout, errOut bytes.Buffer
	code = run(t.Context(), []string{"config", "reload", "-o", "json"}, &stdout, &errOut)
	if stdout.Len() > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
			t.Fatalf("reload output is not JSON: %v\n%s", err, stdout.String())
		}
	}
	return code, rep, errOut.String()
}

func TestReloadRequestAppliedAndReported(t *testing.T) {
	s := startReloadServer(t, "log:\n  level: info\n")
	s.write(t, "log:\n  level: debug\nupdateCheck:\n  enabled: true\n")

	code, res, stderr := reload(t)
	if code != exitOK {
		t.Fatalf("exit = %d, stderr %s", code, stderr)
	}
	slices.Sort(res.AppliedKeys)
	if want := []string{"log.level", "updateCheck.enabled"}; !slices.Equal(res.AppliedKeys, want) {
		t.Errorf("appliedKeys = %v, want %v", res.AppliedKeys, want)
	}
	if len(res.RestartRequiredKeys) != 0 || len(res.Errors) != 0 {
		t.Errorf("result = %+v, want nothing pending and no errors", res)
	}
	if res.RequestID == "" {
		t.Error("the result carries no request id")
	}
	if !strings.Contains(s.stderr.String(), "configuration reloaded") {
		t.Errorf("the log does not say the configuration was reloaded:\n%s", s.stderr.String())
	}
	entries, err := os.ReadDir(cli.RunDir(s.dataDir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "reload-request") {
			t.Errorf("the request %s was not consumed", e.Name())
		}
	}
	// The new level is in effect: debug records now reach the log.
	if code, _, _ := reload(t); code != exitOK {
		t.Fatalf("a second reload must work, exit %d", code)
	}
}

func TestReloadInvalidKeepsPrevious(t *testing.T) {
	s := startReloadServer(t, "log:\n  level: info\n")
	s.write(t, "log:\n  level: shout\nbogus: 1\n")

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run(t.Context(), []string{"config", "reload", "-o", "table"}, &out, &errOut)
	if code != exitValidation {
		t.Fatalf("exit = %d, want %d; stdout %s stderr %s", code, exitValidation, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "refused") || !strings.Contains(out.String(), "log.level") {
		t.Errorf("stdout = %q, want the refusal and the key", out.String())
	}
	if !strings.Contains(s.stderr.String(), "reload refused") {
		t.Errorf("the log does not say the reload was refused:\n%s", s.stderr.String())
	}

	// The previous configuration is whole: a valid edit that changes only the level applies on top of it.
	s.write(t, "log:\n  level: error\n")
	code, res, stderr := reload(t)
	if code != exitOK || !slices.Equal(res.AppliedKeys, []string{"log.level"}) {
		t.Fatalf("exit = %d result %+v stderr %s; want log.level applied (from info, not from the refused value)", code, res, stderr)
	}
}

func TestReloadRestartKeysReported(t *testing.T) {
	s := startReloadServer(t, "log:\n  level: info\nhttp:\n  shutdownTimeout: 5s\n")
	s.write(t, "log:\n  level: debug\nhttp:\n  shutdownTimeout: 9s\n")

	code, res, stderr := reload(t)
	if code != exitOK {
		t.Fatalf("exit = %d, stderr %s", code, stderr)
	}
	if !slices.Equal(res.AppliedKeys, []string{"log.level"}) {
		t.Errorf("appliedKeys = %v, want only the live key", res.AppliedKeys)
	}
	if !slices.Equal(res.RestartRequiredKeys, []string{"http.shutdownTimeout"}) {
		t.Errorf("restartRequiredKeys = %v, want http.shutdownTimeout", res.RestartRequiredKeys)
	}
	if !strings.Contains(s.stderr.String(), "need a restart") {
		t.Errorf("the log does not warn about the restart keys:\n%s", s.stderr.String())
	}
}

func TestReloadSecretFileIsReread(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "token")
	if err := os.WriteFile(secret, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := startReloadServer(t, "backup:\n  s3:\n    secretAccessKeyFile: "+secret+"\n")
	if err := os.WriteFile(secret, []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, res, stderr := reload(t)
	if code != exitOK || !slices.Equal(res.AppliedKeys, []string{"backup.s3.secretAccessKey"}) {
		t.Fatalf("exit %d result %+v stderr %s", code, res, stderr)
	}
	if strings.Contains(s.stderr.String(), "two") && strings.Contains(s.stderr.String(), "one") {
		t.Error("a secret reached the log")
	}
}

func TestReloadWithoutAServerFailsFast(t *testing.T) {
	t.Setenv("ORCON_DATA_DIR", t.TempDir())
	var out, errOut bytes.Buffer
	code := run(t.Context(), []string{"config", "reload"}, &out, &errOut)
	if code != exitError || !strings.Contains(errOut.String(), "no running Orcon server picked up the reload request") {
		t.Fatalf("exit = %d stderr %q", code, errOut.String())
	}
}

func TestLoggerLevelFollowsReload(t *testing.T) {
	t.Parallel()

	level := new(slog.LevelVar)
	var buf bytes.Buffer
	logger := newLogger(&buf, "json", level)
	level.Set(logLevel("warn"))
	logger.Info("hidden")
	level.Set(logLevel("debug"))
	logger.Debug("shown")
	if strings.Contains(buf.String(), "hidden") || !strings.Contains(buf.String(), "shown") {
		t.Fatalf("log = %q", buf.String())
	}
	if logLevel("error") != slog.LevelError || logLevel("") != slog.LevelInfo {
		t.Error("logLevel maps the names wrongly")
	}
	var text bytes.Buffer
	newLogger(&text, "text", level).Info("x")
	if strings.HasPrefix(text.String(), "{") {
		t.Errorf("text format wrote JSON: %q", text.String())
	}
}

// TestReloadTwoServers starts two servers over one data directory: one reload request reaches both,
// each answers once, and a server whose own secret file broke refuses without stopping the other.
func TestReloadTwoServers(t *testing.T) {
	first := startReloadServer(t, "log:\n  level: info\n")
	secret := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(secret, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second := startServerIn(t, first.dataDir, first.file, "ORCON_BACKUP_S3_SECRET_ACCESS_KEY_FILE="+secret)

	first.write(t, "log:\n  level: debug\n")
	code, rep, stderr := reloadAll(t)
	if code != exitOK || len(rep.Servers) != 2 || len(rep.Unanswered) != 0 {
		t.Fatalf("exit %d report %+v stderr %s; want both servers applied", code, rep, stderr)
	}
	for _, s := range rep.Servers {
		if !slices.Equal(s.AppliedKeys, []string{"log.level"}) || len(s.Errors) > 0 {
			t.Errorf("server %s result = %+v", s.ServerID, s)
		}
	}
	if rep.Servers[0].ServerID == rep.Servers[1].ServerID {
		t.Error("both results come from the same server")
	}
	for _, s := range []*reloadServer{first, second} {
		if got := strings.Count(s.stderr.String(), "configuration reloaded"); got != 1 {
			t.Errorf("a server reloaded %d times, want once", got)
		}
	}

	if err := os.Remove(secret); err != nil {
		t.Fatal(err)
	}
	first.write(t, "log:\n  level: warn\n")
	code, rep, stderr = reloadAll(t)
	if code != exitValidation || len(rep.Servers) != 2 {
		t.Fatalf("exit %d report %+v stderr %s; want 5 with both results", code, rep, stderr)
	}
	refused, applied := 0, 0
	for _, s := range rep.Servers {
		switch {
		case len(s.Errors) > 0:
			refused++
		case slices.Equal(s.AppliedKeys, []string{"log.level"}):
			applied++
		}
	}
	if refused != 1 || applied != 1 {
		t.Fatalf("refused %d applied %d, want one of each: %+v", refused, applied, rep.Servers)
	}
}
