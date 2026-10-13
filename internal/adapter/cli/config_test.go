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

type run struct {
	out, err bytes.Buffer
	code     int
	handled  bool
}

func exec(t *testing.T, env cli.Env, args ...string) *run {
	t.Helper()
	r := &run{}
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

func TestReloadNoServerTimesOut(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(cli.RunDir(dir), 0o700); err != nil {
		t.Fatal(err)
	}
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
	if err != nil || len(entries) != 0 {
		t.Errorf("the command left files behind: %v %v", entries, err)
	}
	// Without a run directory there is no server at all and the command does not wait.
	empty := cli.Env{Environ: []string{"HOME=/h", "ORCON_DATA_DIR=" + t.TempDir()}, ReloadWait: 10 * time.Second}
	started = time.Now()
	if r := exec(t, empty, "config", "reload"); r.code != cli.ExitError || time.Since(started) > 2*time.Second {
		t.Errorf("no run directory: exit %d after %v", r.code, time.Since(started))
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
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		cli.WatchReloadRequests(ctx, dir, 10*time.Millisecond, func(string) cli.ReloadResult {
			return cli.ReloadResult{AppliedKeys: []string{"log.level"}, RestartRequiredKeys: []string{"role"}}
		}, slog.New(slog.DiscardHandler))
	}()
	res, err := cli.RequestReload(t.Context(), dir, 5*time.Second, 10*time.Millisecond)
	cancel()
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if res.RequestID == "" || len(res.AppliedKeys) != 1 || len(res.RestartRequiredKeys) != 1 {
		t.Fatalf("result = %+v", res)
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
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(request); os.IsNotExist(err) {
			break
		}
		time.Sleep(10 * time.Millisecond) // polling a real file for the watcher to consume it
	}
	cancel()
	<-done
	if _, err := os.Stat(request); !os.IsNotExist(err) {
		t.Fatalf("the malformed request was not consumed: %v", err)
	}
}

func TestConfigReloadFormats(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := cli.EnsureRunDir(dir); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		cli.WatchReloadRequests(ctx, dir, 10*time.Millisecond, func(string) cli.ReloadResult {
			return cli.ReloadResult{AppliedKeys: []string{"log.level"}, RestartRequiredKeys: []string{"http.address"}}
		}, slog.New(slog.DiscardHandler))
	}()
	defer func() { cancel(); <-done }()
	env := cli.Env{Environ: []string{"HOME=/h", "ORCON_DATA_DIR=" + dir}, MarshalYAML: json.Marshal, ReloadWait: 5 * time.Second, ReloadPoll: 10 * time.Millisecond}
	for format, want := range map[string]string{
		"table": "Applied: log.level",
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
