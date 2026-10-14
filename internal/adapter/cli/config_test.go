// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/cli"
	"github.com/wirvii/orcon/internal/platform/config"
)

const (
	cfgPath    = "/etc/orcon/config.yaml"
	sentinel   = "S3NTINEL-do-not-print"
	sentinelB6 = "UzNOVElORUwtZG8tbm90LXByaW50" // base64 of S3NTINEL-do-not-print
)

type memFS map[string]string

func (m memFS) ReadFile(name string, _ int64) ([]byte, error) {
	d, ok := m[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return []byte(d), nil
}

func (m memFS) Stat(name string) (fs.FileInfo, error) {
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

// decode reads `dotted.path: value` lines; it stands in for the YAML codec.
func decode(data []byte, _ func(string) bool) ([]config.FileEntry, error) {
	var out []config.FileEntry
	sc := bufio.NewScanner(bytes.NewReader(data))
	for line := 1; sc.Scan(); line++ {
		path, value, ok := strings.Cut(strings.TrimSpace(sc.Text()), ":")
		if !ok {
			continue
		}
		out = append(out, config.FileEntry{Path: path, Line: line, Scalar: strings.TrimSpace(value)})
	}
	return out, nil
}

type outcome struct {
	out, err bytes.Buffer
	code     int
	handled  bool
}

func exec(t *testing.T, env cli.Env, args ...string) *outcome {
	t.Helper()
	r := &outcome{}
	env.Stdout, env.Stderr = &r.out, &r.err
	if env.Environ == nil {
		env.Environ = []string{"HOME=/home/orcon"}
	}
	if env.FS == nil {
		env.FS = memFS{}
	}
	env.Decode = decode
	env.GOOS = "linux"
	switch {
	case len(args) > 0 && args[0] == "config":
		r.code, r.handled = cli.RunConfig(t.Context(), args[1:], env), true
	case len(args) > 0 && args[0] == "docs":
		r.code, r.handled = cli.RunDocs(args[1:], env), true
	}
	return r
}

func TestConfigCheckRedactsSecretsAndShowsSource(t *testing.T) {
	t.Parallel()

	files := memFS{
		cfgPath:            "store.driver: postgres\nhttp.address: 127.0.0.1:9\nstore.postgres.dsnFile: /run/secrets/dsn\nkeys.master.key: " + sentinelB6,
		"/run/secrets/dsn": "postgres://u:" + sentinel + "@db/orcon\n",
	}
	env := cli.Env{
		FS:      files,
		Environ: []string{"HOME=/home/orcon", "ORCON_STORE_POSTGRES_MAX_CONNECTIONS=33", "ORCON_ROLE=api", "ORCON_BLOBS_S3_SECRET_ACCESS_KEY=" + sentinel},
	}
	for _, format := range []string{"table", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			e := env
			e.MarshalYAML = json.Marshal // a YAML codec stand-in: JSON is YAML
			r := exec(t, e, "config", "check", "--config", cfgPath, "-o", format, "--log-level", "warn")
			if r.code != cli.ExitOK {
				t.Fatalf("exit = %d, stderr = %q, stdout = %q", r.code, r.err.String(), r.out.String())
			}
			out := r.out.String()
			if strings.Contains(out, sentinel) || strings.Contains(out, sentinelB6) {
				t.Fatalf("output contains a secret value:\n%s", out)
			}
			if strings.Count(out, config.Redacted) < 3 {
				t.Fatalf("output must show %s for the three secrets set:\n%s", config.Redacted, out)
			}
			for _, want := range []string{
				"env:ORCON_STORE_POSTGRES_MAX_CONNECTIONS", "env:ORCON_ROLE", "file:" + cfgPath + ":1", "file:" + cfgPath + ":2",
				"file:/run/secrets/dsn", "flag", "default",
			} {
				if !strings.Contains(out, want) {
					t.Errorf("output lacks the source %q:\n%s", want, out)
				}
			}
		})
	}
}

func TestConfigCheckNeverPrintsCredentialsOfAnAddress(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"table", "json", "yaml"} {
		env := cli.Env{MarshalYAML: json.Marshal, Environ: []string{"HOME=/h", "ORCON_BACKUP_TARGET=s3://AKIA:" + sentinel + "@bucket/p", "ORCON_OBSERVABILITY_OTLP_ENDPOINT=https://u:" + sentinel + "@c:4318"}}
		r := exec(t, env, "config", "check", "-o", format)
		if r.code != cli.ExitValidation {
			t.Errorf("-o %s: exit = %d, want %d", format, r.code, cli.ExitValidation)
		}
		if strings.Contains(r.out.String()+r.err.String(), sentinel) {
			t.Errorf("-o %s prints the credentials of an address:\n%s", format, r.out.String())
		}
		if !strings.Contains(r.out.String(), "credentials") {
			t.Errorf("-o %s does not explain the error:\n%s", format, r.out.String())
		}
	}
}

func TestConfigCheckJSONShape(t *testing.T) {
	t.Parallel()

	r := exec(t, cli.Env{}, "config", "check", "-o", "json")
	if r.code != cli.ExitOK {
		t.Fatalf("exit = %d: %s", r.code, r.out.String())
	}
	var got struct {
		Valid   bool `json:"valid"`
		Entries []struct {
			Key    string `json:"key"`
			Value  any    `json:"value"`
			Source string `json:"source"`
			Reload string `json:"reload"`
		} `json:"entries"`
		Issues []any `json:"issues"`
	}
	if err := json.Unmarshal(r.out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if !got.Valid || len(got.Entries) < 60 || len(got.Issues) != 0 {
		t.Fatalf("valid=%v entries=%d issues=%d", got.Valid, len(got.Entries), len(got.Issues))
	}
	for _, e := range got.Entries {
		if e.Key == "store.postgres.maxConnections" {
			if n, ok := e.Value.(float64); !ok || n != 20 {
				t.Errorf("maxConnections = %v (%T), want the number 20", e.Value, e.Value)
			}
			if e.Source != "default" || e.Reload != "restart" {
				t.Errorf("source %q reload %q", e.Source, e.Reload)
			}
		}
	}
}

func TestConfigCheckDefaultFormat(t *testing.T) {
	t.Parallel()

	if r := exec(t, cli.Env{IsTerminal: true}, "config", "check"); !strings.HasPrefix(r.out.String(), "KEY") {
		t.Errorf("on a terminal the default is a table, got %q", r.out.String()[:20])
	}
	if r := exec(t, cli.Env{}, "config", "check"); !strings.HasPrefix(r.out.String(), "{") {
		t.Errorf("off a terminal the default is JSON, got %q", r.out.String()[:20])
	}
}

func TestConfigCheckExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  cli.Env
		args []string
		want int
		err  string
	}{
		{"valid", cli.Env{}, []string{"config", "check", "-o", "table"}, cli.ExitOK, ""},
		{"warnings only", cli.Env{Environ: []string{"HOME=/h", "ORCON_NOISE=1"}}, []string{"config", "check", "-o", "table"}, cli.ExitOK, ""},
		{"validation error", cli.Env{Environ: []string{"HOME=/h", "ORCON_ROLE=worker"}}, []string{"config", "check", "-o", "table"}, cli.ExitValidation, ""},
		{"missing file", cli.Env{}, []string{"config", "check", "--config", "/missing"}, cli.ExitValidation, ""},
		{"secret by flag", cli.Env{}, []string{"config", "check", "--store-postgres-dsn", sentinel}, cli.ExitUsage, "--store-postgres-dsn-file"},
		{"unknown flag", cli.Env{}, []string{"config", "check", "--nope"}, cli.ExitUsage, "not defined"},
		{"bad format", cli.Env{}, []string{"config", "check", "-o", "xml"}, cli.ExitUsage, "table, json or yaml"},
		{"no command", cli.Env{}, []string{"config"}, cli.ExitUsage, "usage"},
		{"unknown command", cli.Env{}, []string{"config", "bogus"}, cli.ExitUsage, "unknown config command"},
		{"help", cli.Env{}, []string{"config", "check", "--help"}, cli.ExitOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := exec(t, tt.env, tt.args...)
			if r.code != tt.want {
				t.Fatalf("exit = %d, want %d; stdout %q stderr %q", r.code, tt.want, r.out.String(), r.err.String())
			}
			if !strings.Contains(r.err.String(), tt.err) {
				t.Errorf("stderr = %q, want it to contain %q", r.err.String(), tt.err)
			}
			if strings.Contains(r.err.String()+r.out.String(), sentinel) {
				t.Error("a secret given on the command line was echoed")
			}
		})
	}
}

func TestConfigCheckYAMLNeedsACodec(t *testing.T) {
	t.Parallel()

	if r := exec(t, cli.Env{}, "config", "check", "-o", "yaml"); r.code != cli.ExitError {
		t.Fatalf("exit = %d, want %d without a YAML encoder", r.code, cli.ExitError)
	}
}

func TestConfigCheckNoNetworkWithoutProbe(t *testing.T) {
	t.Parallel()

	probe := cli.Probe{Name: "database", Run: func(context.Context, *config.Config) (string, error) {
		t.Error("a probe ran without --probe")
		return "", nil
	}}
	r := exec(t, cli.Env{Probes: []cli.Probe{probe}}, "config", "check", "-o", "json")
	if r.code != cli.ExitOK {
		t.Fatalf("exit = %d", r.code)
	}
	if strings.Contains(r.out.String(), `"probes"`) {
		t.Error("the report mentions probes that did not run")
	}
}

func TestConfigCheckProbe(t *testing.T) {
	t.Parallel()

	var deadline time.Duration
	probes := []cli.Probe{
		{Name: "ok", Key: "store.driver", Run: func(ctx context.Context, _ *config.Config) (string, error) {
			d, _ := ctx.Deadline()
			deadline = time.Until(d)
			return "fine", nil
		}},
		{Name: "off", Applies: func(*config.Config) bool { return false }, Run: func(context.Context, *config.Config) (string, error) { return "", errors.New("never") }},
		{Name: "later", Run: func(context.Context, *config.Config) (string, error) { return "", cli.ErrProbeUnavailable }},
	}
	r := exec(t, cli.Env{Probes: probes}, "config", "check", "--probe", "-o", "table")
	if r.code != cli.ExitOK {
		t.Fatalf("exit = %d: %s", r.code, r.out.String())
	}
	if deadline <= 9*time.Second || deadline > 10*time.Second {
		t.Errorf("probe deadline = %v, want about 10s", deadline)
	}
	for _, want := range []string{"PROBE  ok (store.driver): OK - fine", "PROBE  off: SKIPPED", "PROBE  later: SKIPPED"} {
		if !strings.Contains(r.out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, r.out.String())
		}
	}
}

func TestConfigCheckProbeFailureScrubsSecrets(t *testing.T) {
	t.Parallel()

	env := cli.Env{
		Environ: []string{"HOME=/h", "ORCON_STORE_DRIVER=postgres", "ORCON_ROLE=api", "ORCON_STORE_POSTGRES_DSN=postgres://u:" + sentinel + "@db/x"},
		Probes: []cli.Probe{{Name: "database", Run: func(_ context.Context, c *config.Config) (string, error) {
			return "", errors.New("connect " + c.Secret("store.postgres.dsn").Reveal() + ": refused")
		}}},
	}
	r := exec(t, env, "config", "check", "--probe", "-o", "table")
	if r.code != cli.ExitError {
		t.Fatalf("exit = %d, want %d for a failed probe", r.code, cli.ExitError)
	}
	if strings.Contains(r.out.String(), sentinel) || !strings.Contains(r.out.String(), "PROBE  database: FAILED") {
		t.Fatalf("output:\n%s", r.out.String())
	}
}

func TestConfigCheckProbeSkippedWhenConfigHasErrors(t *testing.T) {
	t.Parallel()

	probe := cli.Probe{Name: "database", Run: func(context.Context, *config.Config) (string, error) {
		t.Error("probed an invalid configuration")
		return "", nil
	}}
	r := exec(t, cli.Env{Environ: []string{"HOME=/h", "ORCON_ROLE=worker"}, Probes: []cli.Probe{probe}}, "config", "check", "--probe", "-o", "table")
	if r.code != cli.ExitValidation || !strings.Contains(r.out.String(), "SKIPPED") {
		t.Fatalf("exit = %d, out:\n%s", r.code, r.out.String())
	}
}

func TestConfigSchema(t *testing.T) {
	t.Parallel()

	r := exec(t, cli.Env{}, "config", "schema")
	if r.code != cli.ExitOK {
		t.Fatalf("exit = %d: %s", r.code, r.err.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(r.out.Bytes(), &doc); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if doc["additionalProperties"] != false || doc["$schema"] == nil {
		t.Errorf("the schema must close unknown keys: %v", doc["additionalProperties"])
	}
	props, _ := doc["properties"].(map[string]any)
	store, _ := props["store"].(map[string]any)
	sp, _ := store["properties"].(map[string]any)
	pg, _ := sp["postgres"].(map[string]any)
	pp, _ := pg["properties"].(map[string]any)
	for _, name := range []string{"dsn", "dsnFile", "maxConnections"} {
		if pp[name] == nil {
			t.Errorf("store.postgres.%s is missing from the schema", name)
		}
	}
	if r := exec(t, cli.Env{}, "config", "schema", "extra"); r.code != cli.ExitUsage {
		t.Errorf("extra argument: exit = %d", r.code)
	}
}

// watch runs a reload watcher over dir until the test ends. The handler gets the keys to answer with.
func watch(t *testing.T, dir string, answer cli.ReloadResult) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		cli.WatchReloadRequests(ctx, dir, 10*time.Millisecond, func(string) cli.ReloadResult { return answer }, slog.New(slog.DiscardHandler))
	}()
	t.Cleanup(func() { cancel(); <-done })
}

// registerSilentServer leaves the registration of a server that never answers.
func registerSilentServer(t *testing.T, dir string) {
	t.Helper()
	if err := cli.EnsureRunDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cli.RunDir(dir), "server.silent"), []byte("silent\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReloadNoServerTimesOut(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	registerSilentServer(t, dir)
	env := cli.Env{
		Environ:    []string{"HOME=/h", "ORCON_DATA_DIR=" + dir},
		ReloadWait: 300 * time.Millisecond,
		ReloadPoll: 20 * time.Millisecond,
	}
	started := time.Now()
	r := exec(t, env, "config", "reload")
	if r.code != cli.ExitError {
		t.Fatalf("exit = %d, want %d; stdout %q stderr %q", r.code, cli.ExitError, r.out.String(), r.err.String())
	}
	if took := time.Since(started); took < 250*time.Millisecond || took > 5*time.Second {
		t.Errorf("took %v, want about the wait of 300 ms", took)
	}
	want := "no running Orcon server picked up the reload request in " + dir + "; is it running with this data directory?"
	if !strings.Contains(r.err.String(), want) {
		t.Errorf("stderr = %q, want %q", r.err.String(), want)
	}
	entries, err := os.ReadDir(cli.RunDir(dir))
	if err != nil || len(entries) != 1 || entries[0].Name() != "server.silent" {
		t.Errorf("the command left files behind: %v %v", entries, err)
	}
	// Without a live server there is nothing to wait for and the command does not wait.
	empty := cli.Env{Environ: []string{"HOME=/h", "ORCON_DATA_DIR=" + t.TempDir()}, ReloadWait: 10 * time.Second}
	started = time.Now()
	if r := exec(t, empty, "config", "reload"); r.code != cli.ExitError || time.Since(started) > 2*time.Second {
		t.Errorf("no server: exit %d after %v", r.code, time.Since(started))
	}
}

func TestReloadChannelRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cli.EnsureRunDir(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(cli.RunDir(dir))
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("run directory = %v, %v; want mode 0700", info, err)
	}
	watch(t, dir, cli.ReloadResult{AppliedKeys: []string{"log.level"}, RestartRequiredKeys: []string{"role"}})
	waitRegistered(t, dir, 1)
	rep, err := cli.RequestReload(t.Context(), dir, 5*time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if rep.RequestID == "" || len(rep.Servers) != 1 || len(rep.Unanswered) != 0 {
		t.Fatalf("report = %+v", rep)
	}
	got := rep.Servers[0]
	if got.ServerID == "" || got.RequestID != rep.RequestID || len(got.AppliedKeys) != 1 || len(got.RestartRequiredKeys) != 1 {
		t.Fatalf("result = %+v", got)
	}
	entries, err := os.ReadDir(cli.RunDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "reload-") {
			t.Errorf("%s was left in the run directory", e.Name())
		}
	}
}

func waitRegistered(t *testing.T, dir string, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		matches, _ := filepath.Glob(filepath.Join(cli.RunDir(dir), "server.*"))
		if len(matches) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond) // polling real files for the watchers to register
	}
	t.Fatalf("%d servers did not register", n)
}

func TestReloadTwoServersAnswerOnceEach(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cli.EnsureRunDir(dir); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	for range 2 {
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			defer close(done)
			cli.WatchReloadRequests(ctx, dir, 10*time.Millisecond, func(string) cli.ReloadResult {
				calls.Add(1)
				return cli.ReloadResult{AppliedKeys: []string{"log.level"}}
			}, slog.New(slog.DiscardHandler))
		}()
		t.Cleanup(func() { cancel(); <-done })
	}
	waitRegistered(t, dir, 2)
	rep, err := cli.RequestReload(t.Context(), dir, 5*time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Servers) != 2 || rep.Servers[0].ServerID == rep.Servers[1].ServerID {
		t.Fatalf("servers = %+v, want two different servers", rep.Servers)
	}
	time.Sleep(100 * time.Millisecond) // give a server that wrongly handles an id twice time to do it
	if got := calls.Load(); got != 2 {
		t.Fatalf("handlers ran %d times, want once per server", got)
	}
}

func TestReloadReportsAServerThatDoesNotAnswer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	registerSilentServer(t, dir)
	watch(t, dir, cli.ReloadResult{AppliedKeys: []string{"log.level"}})
	waitRegistered(t, dir, 2)
	env := cli.Env{Environ: []string{"HOME=/h", "ORCON_DATA_DIR=" + dir}, ReloadWait: 300 * time.Millisecond, ReloadPoll: 10 * time.Millisecond}
	r := exec(t, env, "config", "reload", "-o", "table")
	if r.code != cli.ExitError {
		t.Fatalf("exit = %d, want %d when a running server did not answer", r.code, cli.ExitError)
	}
	if !strings.Contains(r.out.String(), "reloaded. Applied: log.level") || !strings.Contains(r.out.String(), "No answer in time from: silent") {
		t.Errorf("output:\n%s", r.out.String())
	}
}

func TestReloadChannelIgnoresMalformedRequests(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cli.EnsureRunDir(dir); err != nil {
		t.Fatal(err)
	}
	request := filepath.Join(cli.RunDir(dir), "reload-request")
	if err := os.WriteFile(request, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		cli.WatchReloadRequests(ctx, dir, 10*time.Millisecond, func(string) cli.ReloadResult {
			t.Error("a malformed request reached the handler")
			return cli.ReloadResult{}
		}, slog.New(slog.DiscardHandler))
	}()
	waitRegistered(t, dir, 1)
	time.Sleep(100 * time.Millisecond) // several polls pass over the malformed file
	cancel()
	<-done
}

func TestConfigReloadFormats(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cli.EnsureRunDir(dir); err != nil {
		t.Fatal(err)
	}
	watch(t, dir, cli.ReloadResult{AppliedKeys: []string{"log.level"}, RestartRequiredKeys: []string{"http.address"}})
	waitRegistered(t, dir, 1)
	env := cli.Env{Environ: []string{"HOME=/h", "ORCON_DATA_DIR=" + dir}, MarshalYAML: json.Marshal, ReloadWait: 5 * time.Second, ReloadPoll: 10 * time.Millisecond}
	for format, want := range map[string]string{
		"table": "reloaded. Applied: log.level",
		"json":  `"appliedKeys"`,
		"yaml":  `"restartRequiredKeys"`,
	} {
		r := exec(t, env, "config", "reload", "-o", format)
		if r.code != cli.ExitOK || !strings.Contains(r.out.String(), want) {
			t.Errorf("-o %s: exit %d out %q, want it to contain %q", format, r.code, r.out.String(), want)
		}
	}
	if r := exec(t, env, "config", "reload", "-o", "table"); !strings.Contains(r.out.String(), "need a restart") || !strings.Contains(r.out.String(), "http.address") {
		t.Errorf("the restart keys are not reported: %q", r.out.String())
	}
	if r := exec(t, env, "config", "reload", "-o", "xml"); r.code != cli.ExitUsage {
		t.Errorf("bad format: exit %d", r.code)
	}
}

func TestConfigReloadExitsFiveWhenAnyServerRefuses(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cli.EnsureRunDir(dir); err != nil {
		t.Fatal(err)
	}
	watch(t, dir, cli.ReloadResult{AppliedKeys: []string{"log.level"}})
	watch(t, dir, cli.ReloadResult{Errors: []string{"log.level: invalid"}})
	waitRegistered(t, dir, 2)
	env := cli.Env{Environ: []string{"HOME=/h", "ORCON_DATA_DIR=" + dir}, ReloadWait: 5 * time.Second, ReloadPoll: 10 * time.Millisecond}
	r := exec(t, env, "config", "reload", "-o", "table")
	if r.code != cli.ExitValidation {
		t.Fatalf("exit = %d, want %d; out %s", r.code, cli.ExitValidation, r.out.String())
	}
	if !strings.Contains(r.out.String(), "refused") || !strings.Contains(r.out.String(), "reloaded. Applied") {
		t.Errorf("both results must be shown:\n%s", r.out.String())
	}
}

func TestConcurrentReloadCommandsBothSucceed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cli.EnsureRunDir(dir); err != nil {
		t.Fatal(err)
	}
	watch(t, dir, cli.ReloadResult{AppliedKeys: []string{"log.level"}})
	waitRegistered(t, dir, 1)
	const commands = 4
	errs := make(chan error, commands)
	reports := make(chan *cli.ReloadReport, commands)
	for range commands {
		go func() {
			rep, err := cli.RequestReload(t.Context(), dir, 10*time.Second, 10*time.Millisecond)
			errs <- err
			reports <- rep
		}()
	}
	ids := map[string]bool{}
	for range commands {
		if err := <-errs; err != nil {
			t.Errorf("a concurrent command failed: %v", err)
		}
		if rep := <-reports; rep != nil {
			if len(rep.Servers) != 1 || len(rep.Unanswered) != 0 {
				t.Errorf("report = %+v", rep)
			}
			ids[rep.RequestID] = true
		}
	}
	if len(ids) != commands {
		t.Errorf("%d different requests answered, want %d", len(ids), commands)
	}
	if _, err := os.Stat(filepath.Join(cli.RunDir(dir), "reload.lock")); !os.IsNotExist(err) {
		t.Errorf("the lock was not released: %v", err)
	}
}

func TestStaleReloadLockIsTakenOver(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cli.EnsureRunDir(dir); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(cli.RunDir(dir), "reload.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	watch(t, dir, cli.ReloadResult{})
	waitRegistered(t, dir, 1)
	if _, err := cli.RequestReload(t.Context(), dir, 5*time.Second, 10*time.Millisecond); err != nil {
		t.Fatalf("a dead command's lock must not block: %v", err)
	}
}

func TestRunDirectoryHygiene(t *testing.T) {
	t.Parallel()

	t.Run("a symbolic link is refused and its target left alone", func(t *testing.T) {
		t.Parallel()
		dir, target := t.TempDir(), t.TempDir()
		if err := os.Chmod(target, 0o755); err != nil { //nolint:gosec // G302: the test needs a mode to see that nothing changes it
			t.Fatal(err)
		}
		if err := os.Symlink(target, cli.RunDir(dir)); err != nil {
			t.Skipf("no symbolic links here: %v", err)
		}
		err := cli.EnsureRunDir(dir)
		if err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("EnsureRunDir = %v, want a symbolic link error", err)
		}
		if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0o755 {
			t.Fatalf("the target of the link was changed: %v %v", info, err)
		}
		if _, err := cli.RequestReload(t.Context(), dir, time.Second, 10*time.Millisecond); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatalf("RequestReload = %v, want a symbolic link error", err)
		}
	})

	t.Run("stale files are expired when a server starts", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := cli.EnsureRunDir(dir); err != nil {
			t.Fatal(err)
		}
		run := cli.RunDir(dir)
		old := time.Now().Add(-time.Hour)
		for _, name := range []string{"server.dead", "reload-result.dead", "reload-request", "reload.lock", ".tmp-123"} {
			path := filepath.Join(run, name)
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(run, "server.alive"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		watch(t, dir, cli.ReloadResult{})
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(filepath.Join(run, "server.dead")); os.IsNotExist(err) {
				break
			}
			time.Sleep(5 * time.Millisecond) // polling for the watcher to start and expire the old files
		}
		for _, name := range []string{"server.dead", "reload-result.dead", "reload-request", "reload.lock", ".tmp-123"} {
			if _, err := os.Stat(filepath.Join(run, name)); !os.IsNotExist(err) {
				t.Errorf("%s was not expired: %v", name, err)
			}
		}
		if _, err := os.Stat(filepath.Join(run, "server.alive")); err != nil {
			t.Errorf("a fresh registration of another server was removed: %v", err)
		}
	})

	t.Run("a registration older than the expiry is not a live server", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := cli.EnsureRunDir(dir); err != nil {
			t.Fatal(err)
		}
		dead := filepath.Join(cli.RunDir(dir), "server.dead")
		if err := os.WriteFile(dead, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-time.Hour)
		if err := os.Chtimes(dead, old, old); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		_, err := cli.RequestReload(t.Context(), dir, 5*time.Second, 10*time.Millisecond)
		if !errors.Is(err, cli.ErrNoServer) || time.Since(started) > time.Second {
			t.Fatalf("RequestReload = %v after %v; want ErrNoServer at once", err, time.Since(started))
		}
	})

	t.Run("the heartbeat touches the registration instead of rewriting it", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := cli.EnsureRunDir(dir); err != nil {
			t.Fatal(err)
		}
		watch(t, dir, cli.ReloadResult{})
		waitRegistered(t, dir, 1)
		matches, _ := filepath.Glob(filepath.Join(cli.RunDir(dir), "server.*"))
		first, err := os.Stat(matches[0])
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			next, err := os.Stat(matches[0])
			if err == nil && next.ModTime().After(first.ModTime()) {
				if !os.SameFile(first, next) {
					t.Fatal("the registration was replaced, not touched")
				}
				return
			}
			time.Sleep(5 * time.Millisecond) // polling the modification time the heartbeat moves
		}
		t.Fatal("the heartbeat never moved the modification time")
	})
}
