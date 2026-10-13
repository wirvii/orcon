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
	r.code, r.handled = cli.Run(t.Context(), args, env)
	return r
}

func TestRunIgnoresServerArguments(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{nil, {"--role", "api"}, {"serve"}} {
		if r := exec(t, cli.Env{}, args...); r.handled {
			t.Errorf("Run(%q) handled = true, want false so the server flags apply", args)
		}
	}
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

	probe := cli.Probe{Name: "database", Run: func(context.Context, *config.Config) error {
		t.Error("a probe ran without --probe")
		return nil
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
		{Name: "ok", Run: func(ctx context.Context, _ *config.Config) error {
			d, _ := ctx.Deadline()
			deadline = time.Until(d)
			return nil
		}},
		{Name: "off", Applies: func(*config.Config) bool { return false }, Run: func(context.Context, *config.Config) error { return errors.New("never") }},
		{Name: "later", Run: func(context.Context, *config.Config) error { return cli.ErrProbeUnavailable }},
	}
	r := exec(t, cli.Env{Probes: probes}, "config", "check", "--probe", "-o", "table")
	if r.code != cli.ExitOK {
		t.Fatalf("exit = %d: %s", r.code, r.out.String())
	}
	if deadline <= 9*time.Second || deadline > 10*time.Second {
		t.Errorf("probe deadline = %v, want about 10s", deadline)
	}
	for _, want := range []string{"PROBE  ok: ok", "PROBE  off: skipped", "PROBE  later: skipped"} {
		if !strings.Contains(r.out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, r.out.String())
		}
	}
}

func TestConfigCheckProbeFailureScrubsSecrets(t *testing.T) {
	t.Parallel()

	env := cli.Env{
		Environ: []string{"HOME=/h", "ORCON_STORE_DRIVER=postgres", "ORCON_ROLE=api", "ORCON_STORE_POSTGRES_DSN=postgres://u:" + sentinel + "@db/x"},
		Probes: []cli.Probe{{Name: "database", Run: func(_ context.Context, c *config.Config) error {
			return errors.New("connect " + c.Secret("store.postgres.dsn").Reveal() + ": refused")
		}}},
	}
	r := exec(t, env, "config", "check", "--probe", "-o", "table")
	if r.code != cli.ExitValidation {
		t.Fatalf("exit = %d, want %d for a failed probe", r.code, cli.ExitValidation)
	}
	if strings.Contains(r.out.String(), sentinel) || !strings.Contains(r.out.String(), "PROBE  database: failed") {
		t.Fatalf("output:\n%s", r.out.String())
	}
}

func TestConfigCheckProbeSkippedWhenConfigHasErrors(t *testing.T) {
	t.Parallel()

	probe := cli.Probe{Name: "database", Run: func(context.Context, *config.Config) error {
		t.Error("probed an invalid configuration")
		return nil
	}}
	r := exec(t, cli.Env{Environ: []string{"HOME=/h", "ORCON_ROLE=worker"}, Probes: []cli.Probe{probe}}, "config", "check", "--probe", "-o", "table")
	if r.code != cli.ExitValidation || !strings.Contains(r.out.String(), "skipped") {
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

func TestConfigReload(t *testing.T) {
	t.Parallel()

	var got string
	env := cli.Env{Reload: func(_ context.Context, c *config.Config) error {
		got = c.Value("dataDir").Text()
		return nil
	}}
	r := exec(t, env, "config", "reload", "--data-dir", "/d")
	if r.code != cli.ExitOK || got != "/d" {
		t.Fatalf("exit = %d dataDir = %q: %s", r.code, got, r.err.String())
	}

	failing := cli.Env{Reload: func(context.Context, *config.Config) error { return errors.New("no running local instance") }}
	if r := exec(t, failing, "config", "reload"); r.code != cli.ExitPrecondition || !strings.Contains(r.err.String(), "no running local instance") {
		t.Errorf("exit = %d stderr = %q", r.code, r.err.String())
	}
	if r := exec(t, cli.Env{}, "config", "reload"); r.code != cli.ExitPrecondition {
		t.Errorf("without a way to reach an instance: exit = %d", r.code)
	}
	invalid := cli.Env{Environ: []string{"HOME=/h", "ORCON_ROLE=worker"}, Reload: func(context.Context, *config.Config) error {
		t.Error("asked the instance to reload an invalid configuration")
		return nil
	}}
	if r := exec(t, invalid, "config", "reload"); r.code != cli.ExitValidation {
		t.Errorf("invalid configuration: exit = %d", r.code)
	}
}
