// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/platform/config"
)

const cfgPath = "/etc/orcon/config.yaml"

func TestDefaultsAreValid(t *testing.T) {
	t.Parallel()

	res := load(t, input{})
	if err := res.Err(); err != nil {
		t.Fatalf("Load with no input = %v, want no errors", err)
	}
	if got, want := res.Config.Value("http.address").Text(), "127.0.0.1:8080"; got != want {
		t.Fatalf("http.address = %q, want %q", got, want)
	}
	if got := res.Config.Value("ops.address").Text(); got != "" {
		t.Fatalf("ops.address = %q, want empty (operations listener off)", got)
	}
	for path, want := range map[string]bool{
		"observability.metrics.prometheus.enabled": false,
		"observability.metrics.otlp.enabled":       false,
		"observability.traces.otlp.enabled":        false,
		"observability.logs.otlp.enabled":          false,
		"updateCheck.enabled":                      false,
	} {
		if got := res.Config.Value(path).Bool(); got != want {
			t.Errorf("%s = %v, want %v (secure default, R-CONF-08)", path, got, want)
		}
	}
	if got := res.Config.Value("store.sqlite.synchronous").Text(); got != "FULL" {
		t.Errorf("store.sqlite.synchronous = %q, want FULL", got)
	}
	if got := res.Config.Value("backup.target").Text(); got != "" {
		t.Errorf("backup.target = %q, want empty (no scheduled target)", got)
	}
}

func TestComputedDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		goos    string
		environ []string
		key     string
		want    string
	}{
		{"linux home", "linux", env("HOME", "/home/a"), "dataDir", "/home/a/.local/share/orcon"},
		{"linux xdg", "linux", env("HOME", "/home/a", "XDG_DATA_HOME", "/data"), "dataDir", "/data/orcon"},
		{"darwin", "darwin", env("HOME", "/Users/a"), "dataDir", "/Users/a/Library/Application Support/Orcon"},
		{"windows", "windows", env("LOCALAPPDATA", `C:\Users\a\AppData\Local`), "dataDir", `C:\Users\a\AppData\Local\Orcon`},
		{"windows sqlite path", "windows", env("LOCALAPPDATA", `C:\L`), "store.sqlite.path", `C:\L\Orcon\orcon.db`},
		{"sqlite path under data dir", "linux", env("ORCON_DATA_DIR", "/var/lib/orcon"), "store.sqlite.path", "/var/lib/orcon/orcon.db"},
		{"blobs path", "linux", env("ORCON_DATA_DIR", "/var/lib/orcon/"), "blobs.filesystem.path", "/var/lib/orcon/blobs"},
		{"master key path", "linux", env("ORCON_DATA_DIR", "/d"), "keys.master.file.path", "/d/keys/master.key"},
		{"spool", "linux", env("ORCON_DATA_DIR", "/d"), "backup.spoolDirectory", "/d/tmp"},
		{"blobs backend sqlite", "linux", env("ORCON_DATA_DIR", "/d"), "blobs.backend", "filesystem"},
		{"blobs backend postgres", "linux", env("ORCON_DATA_DIR", "/d", "ORCON_STORE_DRIVER", "postgres"), "blobs.backend", "postgres"},
		{"backup before upgrade sqlite", "linux", env("ORCON_DATA_DIR", "/d"), "backup.beforeUpgrade", "true"},
		{"backup before upgrade postgres", "linux", env("ORCON_DATA_DIR", "/d", "ORCON_STORE_DRIVER", "postgres"), "backup.beforeUpgrade", "false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res := load(t, input{goos: tt.goos, environ: tt.environ})
			if got := res.Config.Value(tt.key).Text(); got != tt.want {
				t.Fatalf("%s = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestDataDirNeedsAHomeOrAnExplicitValue(t *testing.T) {
	t.Parallel()

	res := load(t, input{environ: env()})
	if !hasIssue(res, config.SeverityError, "dataDir", "HOME is not set") {
		t.Fatalf("issues = %v, want a dataDir error that names HOME", res.Issues)
	}
}

func TestPrecedencePerKey(t *testing.T) {
	t.Parallel()

	files := fakeFS{cfgPath: {data: strings.Join([]string{
		"http.shutdownTimeout: 10s",
		"http.address: 0.0.0.0:1111",
		"log.level: warn",
		"log.format: json",
	}, "\n")}}
	res := load(t, input{
		args:    []string{"--config", cfgPath, "--http-shutdown-timeout", "40s"},
		environ: baseEnv("ORCON_HTTP_SHUTDOWN_TIMEOUT", "20s", "ORCON_HTTP_ADDRESS", "0.0.0.0:2222", "ORCON_LOG_FORMAT", "text"),
		files:   files,
	})
	if err := res.Err(); err != nil {
		t.Fatalf("Load = %v", err)
	}
	tests := []struct {
		key, want, source string
	}{
		{"http.shutdownTimeout", "40s", "flag"},
		{"http.address", "0.0.0.0:2222", "env:ORCON_HTTP_ADDRESS"},
		{"log.format", "text", "env:ORCON_LOG_FORMAT"},
		{"log.level", "warn", "file:" + cfgPath + ":3"},
		{"role", "standalone", "default"},
	}
	for _, tt := range tests {
		if got := res.Config.Value(tt.key).Text(); got != tt.want {
			t.Errorf("%s = %q, want %q", tt.key, got, tt.want)
		}
		if got := res.Config.Source(tt.key).String(); got != tt.source {
			t.Errorf("source of %s = %q, want %q", tt.key, got, tt.source)
		}
	}
}

func TestPrecedenceIsPerKeyNotPerBlock(t *testing.T) {
	t.Parallel()

	files := fakeFS{cfgPath: {data: "store.postgres.maxConnections: 7\nstore.postgres.declaredCapacity: 10GiB"}}
	res := load(t, input{
		environ: baseEnv("ORCON_CONFIG", cfgPath, "ORCON_STORE_POSTGRES_MAX_CONNECTIONS", "9"),
		files:   files,
	})
	if got := res.Config.Value("store.postgres.maxConnections").Int(); got != 9 {
		t.Errorf("maxConnections = %d, want 9 (environment beats the file)", got)
	}
	if got := res.Config.Value("store.postgres.declaredCapacity").Bytes(); got != 10<<30 {
		t.Errorf("declaredCapacity = %d, want 10GiB from the file next to an env-set sibling", got)
	}
	if errs := res.Err(); errs != nil {
		t.Errorf("a file value overridden by the environment is not an error, got %v", errs)
	}
}

func TestEmptyEnvironmentVariableIsNotSet(t *testing.T) {
	t.Parallel()

	res := load(t, input{environ: baseEnv("ORCON_HTTP_ADDRESS", "")})
	if got := res.Config.Source("http.address"); got.Kind != config.SourceDefault {
		t.Fatalf("source = %v, want default for an empty variable", got)
	}
}

func TestSecretFilesAreReadWithoutTheFinalNewline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, content, want string
	}{
		{"one newline", "s3cret\n", "s3cret"},
		{"crlf", "s3cret\r\n", "s3cret"},
		{"none", "s3cret", "s3cret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res := load(t, input{
				args:  []string{"--store-postgres-dsn-file", "/run/secrets/dsn"},
				files: fakeFS{"/run/secrets/dsn": {data: tt.content, mode: 0o400}},
			})
			if got := res.Config.Secret("store.postgres.dsn").Reveal(); got != tt.want {
				t.Fatalf("dsn = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSecretFileWithAnEmbeddedNewlineIsRejected(t *testing.T) {
	t.Parallel()

	res := load(t, input{
		args:  []string{"--store-postgres-dsn-file", "/run/secrets/dsn"},
		files: fakeFS{"/run/secrets/dsn": {data: "line1\nline2\n", mode: 0o400}},
	})
	if !hasIssue(res, config.SeverityError, "store.postgres.dsn", "control characters") {
		t.Fatalf("issues = %v", res.Issues)
	}
	for _, i := range res.Issues {
		if strings.Contains(i.Message, "line1") {
			t.Fatalf("the message repeats the secret: %v", i)
		}
	}
}

func TestSecretFileReadableByOthersWarns(t *testing.T) {
	t.Parallel()

	res := load(t, input{
		args:  []string{"--store-postgres-dsn-file", "/run/secrets/dsn"},
		files: fakeFS{"/run/secrets/dsn": {data: "x", mode: 0o644}},
	})
	if !hasIssue(res, config.SeverityWarning, "store.postgres.dsn", "readable by other users") {
		t.Fatalf("issues = %v, want a warning about the file mode", res.Issues)
	}
	res = load(t, input{
		args:  []string{"--store-postgres-dsn-file", "/run/secrets/dsn"},
		files: fakeFS{"/run/secrets/dsn": {data: "x", mode: 0o644}},
		goos:  "windows",
	})
	if hasIssue(res, config.SeverityWarning, "store.postgres.dsn", "readable by other users") {
		t.Fatalf("a Windows file mode says nothing; got %v", res.Issues)
	}
}

func TestMissingSecretFileIsAnError(t *testing.T) {
	t.Parallel()

	res := load(t, input{environ: baseEnv("ORCON_STORE_POSTGRES_DSN_FILE", "/nope")})
	if !hasIssue(res, config.SeverityError, "store.postgres.dsn", "cannot read the secret file /nope") {
		t.Fatalf("issues = %v", res.Issues)
	}
}

func TestFileVariantForEverySecret(t *testing.T) {
	t.Parallel()

	secrets := 0
	for _, k := range config.Default().Keys() {
		if !k.Secret {
			continue
		}
		secrets++
		content := "aGVsbG8=\n"
		want := "aGVsbG8="
		if k.Kind == config.KindMap {
			content, want = "Authorization=Bearer abc\n", "Authorization=Bearer abc"
		}
		files := fakeFS{"/run/secrets/v": {data: content, mode: 0o400}, cfgPath: {data: k.FileKey() + ": /run/secrets/v", mode: 0o600}}
		layers := map[string]input{
			"flag": {args: []string{"--" + k.FlagFileName(), "/run/secrets/v"}, files: files},
			"env":  {environ: baseEnv(k.EnvFileName(), "/run/secrets/v"), files: files},
			"file": {args: []string{"--config", cfgPath}, files: files},
		}
		for name, in := range layers {
			t.Run(k.Path+"/"+name, func(t *testing.T) {
				t.Parallel()

				res := load(t, in)
				if hasIssue(res, config.SeverityError, k.Path, "") {
					t.Fatalf("issues = %v", issuesFor(res, k.Path))
				}
				got := res.Config.Value(k.Path)
				if got.IsZero() {
					t.Fatalf("%s is empty, want the content of the file", k.Path)
				}
				if got.String() != config.Redacted {
					t.Fatalf("String() = %q, want %q", got.String(), config.Redacted)
				}
				if plain := got.Secret().Reveal(); k.Kind != config.KindMap && plain != want {
					t.Fatalf("value = %q, want %q", plain, want)
				}
				if src := res.Config.Source(k.Path); src.Kind != config.SourceSecretFile || src.Path != "/run/secrets/v" {
					t.Fatalf("source = %v, want the secret file", src)
				}
			})
		}
	}
	if secrets < 8 {
		t.Fatalf("found %d secret keys, want the whole set of the schema", secrets)
	}
}

func TestValueAndFileIsError(t *testing.T) {
	t.Parallel()

	files := fakeFS{
		"/run/secrets/dsn": {data: "from-file", mode: 0o400},
		cfgPath:            {data: "store.postgres.dsn: from-yaml\nstore.postgres.dsnFile: /run/secrets/dsn", mode: 0o600},
	}
	tests := []struct {
		name string
		in   input
	}{
		{"environment", input{environ: baseEnv("ORCON_STORE_POSTGRES_DSN", "v", "ORCON_STORE_POSTGRES_DSN_FILE", "/run/secrets/dsn"), files: files}},
		{"file", input{args: []string{"--config", cfgPath}, files: files}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res := load(t, tt.in)
			if !hasIssue(res, config.SeverityError, "store.postgres.dsn", "set twice at the same level") {
				t.Fatalf("issues = %v, want the value-and-file error", res.Issues)
			}
		})
	}
}

func TestFileVariantAtAHigherLayerBeatsAValueAtALowerOne(t *testing.T) {
	t.Parallel()

	res := load(t, input{
		args:    []string{"--config", cfgPath},
		environ: baseEnv("ORCON_STORE_POSTGRES_DSN_FILE", "/run/secrets/dsn"),
		files: fakeFS{
			"/run/secrets/dsn": {data: "from-file", mode: 0o400},
			cfgPath:            {data: "store.postgres.dsn: from-yaml", mode: 0o600},
		},
	})
	if got := res.Config.Secret("store.postgres.dsn").Reveal(); got != "from-file" {
		t.Fatalf("dsn = %q, want the environment file variant to win", got)
	}
}

func TestSecretValueFlagsRejected(t *testing.T) {
	t.Parallel()

	schema := config.Default()
	for _, k := range schema.Keys() {
		if !k.Secret {
			continue
		}
		t.Run(k.Path, func(t *testing.T) {
			t.Parallel()

			fset := flag.NewFlagSet("test", flag.ContinueOnError)
			fset.SetOutput(io.Discard)
			schema.BindFlags(fset)
			if fset.Lookup(k.FlagName()) != nil {
				t.Fatalf("flag --%s exists; a secret has only --%s", k.FlagName(), k.FlagFileName())
			}
			if fset.Lookup(k.FlagFileName()) == nil {
				t.Fatalf("flag --%s is missing", k.FlagFileName())
			}
			for _, args := range [][]string{
				{"--" + k.FlagName() + "=hunter2-the-secret"},
				{"-" + k.FlagName(), "hunter2-the-secret"},
			} {
				err := schema.ParseFlags(fset, args)
				if !errors.Is(err, config.ErrValueAsFlag) {
					t.Fatalf("ParseFlags(%q) = %v, want ErrValueAsFlag", args, err)
				}
				if strings.Contains(err.Error(), "hunter2") {
					t.Fatalf("the error repeats the secret: %v", err)
				}
				if !strings.Contains(err.Error(), "--"+k.FlagFileName()) {
					t.Fatalf("the error does not name the file flag: %v", err)
				}
			}
		})
	}
}

func TestBoolFlagsNeedNoValue(t *testing.T) {
	t.Parallel()

	res := load(t, input{args: []string{"--store-migrations-auto=false", "--update-check-enabled"}})
	if res.Config.Value("store.migrations.auto").Bool() {
		t.Error("store.migrations.auto = true, want false")
	}
	if !res.Config.Value("updateCheck.enabled").Bool() {
		t.Error("updateCheck.enabled = false, want true from the bare flag")
	}
}

func TestUnknownFileKeyIsError(t *testing.T) {
	t.Parallel()

	files := fakeFS{cfgPath: {data: strings.Join([]string{
		"http.adress: 1.2.3.4:80",
		"http: oops",
		"store.postgres.maxConnectionsFile: /x",
	}, "\n")}}
	res := load(t, input{args: []string{"--config", cfgPath}, files: files})
	for _, key := range []string{"http.adress", "http", "store.postgres.maxConnectionsFile"} {
		if len(issuesFor(res, key)) == 0 {
			t.Errorf("no issue for %s; issues = %v", key, res.Issues)
		}
	}
	if !hasIssue(res, config.SeverityError, "http.adress", cfgPath+":1") {
		t.Errorf("the error must name file and line; issues = %v", issuesFor(res, "http.adress"))
	}
	if !hasIssue(res, config.SeverityError, "http", "section") {
		t.Errorf("a section used as a value gets its own message; issues = %v", issuesFor(res, "http"))
	}
	if res.Err() == nil {
		t.Fatal("Err = nil, want a validation error")
	}
}

func TestUnknownEnvIsWarning(t *testing.T) {
	t.Parallel()

	res := load(t, input{environ: baseEnv(
		"ORCON_NOT_A_KEY", "1",
		"ORCON_URL", "https://client", // client variables share the prefix and are fine
		"ORCON_TOKEN", "t",
		"ORCON_KEYS_MASTER_KEY", "x", // the derived name of an aliased key is not accepted
	)})
	if !hasIssue(res, config.SeverityWarning, "ORCON_NOT_A_KEY", "unknown environment variable") {
		t.Errorf("issues = %v, want a warning for ORCON_NOT_A_KEY", res.Issues)
	}
	if !hasIssue(res, config.SeverityWarning, "ORCON_KEYS_MASTER_KEY", "unknown") {
		t.Errorf("issues = %v, want a warning for the derived name of an aliased key", res.Issues)
	}
	for _, i := range res.Issues {
		if i.Key == "ORCON_URL" || i.Key == "ORCON_TOKEN" {
			t.Errorf("client variable %s must not be reported: %v", i.Key, i)
		}
		if i.Severity == config.SeverityError {
			t.Errorf("unknown environment variables are warnings, got %v", i)
		}
	}
	if res.Err() != nil {
		t.Errorf("Err = %v, want nil", res.Err())
	}
}

func TestEnvAliasesFromSpec004(t *testing.T) {
	t.Parallel()

	res := load(t, input{environ: baseEnv(
		"ORCON_PUBLIC_URL", "https://orcon.example.com",
		"ORCON_MASTER_KEY", "aGVsbG8=",
		"ORCON_MASTER_KEY_BACKUP_ACK", "ab12cd34",
	)})
	if got := res.Config.Value("publicUrl").Text(); got != "https://orcon.example.com" {
		t.Errorf("publicUrl = %q", got)
	}
	if got := res.Config.Secret("keys.master.key").Reveal(); got != "aGVsbG8=" {
		t.Errorf("keys.master.key = %q", got)
	}
	if got := res.Config.Value("keys.master.backupAcknowledgement").Text(); got != "ab12cd34" {
		t.Errorf("keys.master.backupAcknowledgement = %q", got)
	}
	for path, want := range map[string]string{
		"publicUrl":                         "ORCON_PUBLIC_URL",
		"keys.master.key":                   "ORCON_MASTER_KEY",
		"keys.master.backupAcknowledgement": "ORCON_MASTER_KEY_BACKUP_ACK",
	} {
		k, ok := config.Default().Key(path)
		if !ok || k.EnvName() != want {
			t.Errorf("env name of %s = %q, want %q", path, k.EnvName(), want)
		}
	}
	if k, _ := config.Default().Key("keys.master.key"); k.EnvFileName() != "ORCON_MASTER_KEY_FILE" {
		t.Errorf("file variant of the aliased key = %q, want ORCON_MASTER_KEY_FILE", k.EnvFileName())
	}
}

func TestInvalidValuesNameTheKeyAndNeverTheSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		in    input
		key   string
		match string
	}{
		{"enum", input{args: []string{"--role", "boss"}}, "role", "must be one of"},
		{"int range", input{args: []string{"--store-postgres-max-connections", "0"}}, "store.postgres.maxConnections", "between 1 and 1000"},
		{"int", input{environ: baseEnv("ORCON_STORE_POSTGRES_MAX_CONNECTIONS", "many")}, "store.postgres.maxConnections", "must be an integer"},
		{"float range", input{args: []string{"--runtime-memory-limit-ratio", "2"}}, "runtime.memoryLimitRatio", "between 0.5 and 0.95"},
		{"duration", input{args: []string{"--http-shutdown-timeout", "-5s"}}, "http.shutdownTimeout", "positive duration"},
		{"address", input{args: []string{"--http-address", "nowhere"}}, "http.address", "host:port"},
		{"bool", input{args: []string{"--update-check-enabled=maybe"}}, "updateCheck.enabled", "true or false"},
		{"bytes", input{args: []string{"--store-postgres-declared-capacity", "lots"}}, "store.postgres.declaredCapacity", "size in bytes"},
		{"url", input{args: []string{"--public-url", "not a url"}}, "publicUrl", "absolute URL"},
		{"cidr", input{args: []string{"--http-trusted-proxies", "10.0.0.0/8,bogus"}}, "http.trustedProxies", "CIDR"},
		{"cron", input{args: []string{"--backup-schedule", "daily"}}, "backup.schedule", "five fields"},
		{"target", input{args: []string{"--backup-target", "ftp://x"}}, "backup.target", "file:///"},
		{"age", input{args: []string{"--keys-recovery-public-key", "nope"}}, "keys.recoveryPublicKey", "age1"},
		{"empty with default", input{args: []string{"--log-level="}}, "log.level", "must not be empty"},
		{"secret base64", input{environ: baseEnv("ORCON_MASTER_KEY", "not base64 !!")}, "keys.master.key", "must be base64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res := load(t, tt.in)
			if !hasIssue(res, config.SeverityError, tt.key, tt.match) {
				t.Fatalf("issues for %s = %v, want one containing %q", tt.key, issuesFor(res, tt.key), tt.match)
			}
			for _, i := range res.Issues {
				if strings.Contains(i.Message, "not base64 !!") {
					t.Fatalf("the message repeats a secret value: %v", i)
				}
			}
		})
	}
}

func TestAddressesRefuseCredentialsAndNeverPrintThem(t *testing.T) {
	t.Parallel()

	const leaked = "S3CRET-PASSWORD"
	tests := []struct{ key, flag, value string }{
		{"backup.target", "--backup-target", "s3://AKIA:" + leaked + "@bucket/prefix"},
		{"backup.keys.target", "--backup-keys-target", "file://user:" + leaked + "@/keys"},
		{"observability.otlp.endpoint", "--observability-otlp-endpoint", "https://user:" + leaked + "@collector:4318"},
		{"observability.otlp.endpoint", "--observability-otlp-endpoint", "user:" + leaked + "@collector:4318"},
		{"updateCheck.url", "--update-check-url", "https://user:" + leaked + "@api.github.com/x"},
		{"publicUrl", "--public-url", "https://user:" + leaked + "@orcon.example.com"},
		{"blobs.s3.endpoint", "--blobs-s3-endpoint", "https://user:" + leaked + "@s3.example.com"},
		{"keys.master.vaultTransit.address", "--keys-master-vault-transit-address", "https://user:" + leaked + "@vault:8200"},
	}
	for _, tt := range tests {
		res := load(t, input{args: []string{tt.flag + "=" + tt.value}})
		if !hasIssue(res, config.SeverityError, tt.key, "credentials") {
			t.Errorf("%s=%q: issues = %v, want a credentials error", tt.key, tt.value, issuesFor(res, tt.key))
		}
		if got := res.Config.Value(tt.key).Text(); strings.Contains(got, leaked) {
			t.Errorf("%s keeps the rejected value %q", tt.key, got)
		}
		for _, i := range res.Issues {
			if strings.Contains(i.Message, leaked) {
				t.Errorf("an issue repeats the credentials: %v", i)
			}
		}
	}
	// Addresses without credentials stay valid, host:port included.
	for _, v := range []string{"collector:4317", "https://collector:4318", "localhost"} {
		res := load(t, input{args: []string{"--observability-otlp-endpoint=" + v}})
		if hasIssue(res, config.SeverityError, "observability.otlp.endpoint", "") {
			t.Errorf("endpoint %q refused: %v", v, res.Issues)
		}
	}
}

func TestListAndMapValues(t *testing.T) {
	t.Parallel()

	files := fakeFS{cfgPath: {data: "http.trustedProxies: [10.0.0.0/8, 192.168.0.0/16]\nobservability.otlp.headers: {Authorization=Bearer t0k3n, X-Org=acme}", mode: 0o600}}
	res := load(t, input{args: []string{"--config", cfgPath}, files: files})
	if err := res.Err(); err != nil {
		t.Fatalf("Load = %v", err)
	}
	if diff := cmp.Diff([]string{"10.0.0.0/8", "192.168.0.0/16"}, res.Config.Value("http.trustedProxies").List()); diff != "" {
		t.Errorf("trustedProxies mismatch (-want +got):\n%s", diff)
	}
	headers := res.Config.Value("observability.otlp.headers")
	if headers.String() != config.Redacted {
		t.Errorf("headers print as %q, want %q", headers.String(), config.Redacted)
	}
	if diff := cmp.Diff(map[string]string{"Authorization": "Bearer t0k3n", "X-Org": "acme"}, headers.Map()); diff != "" {
		t.Errorf("headers mismatch (-want +got):\n%s", diff)
	}
	if hasIssue(res, config.SeverityWarning, "observability.otlp.headers", "other users can read") {
		t.Errorf("a 0600 file must not warn: %v", res.Issues)
	}
}

func TestListFromEnvironmentAndFlag(t *testing.T) {
	t.Parallel()

	res := load(t, input{environ: baseEnv("ORCON_HTTP_TRUSTED_PROXIES", "10.0.0.0/8, 172.16.0.0/12")})
	if diff := cmp.Diff([]string{"10.0.0.0/8", "172.16.0.0/12"}, res.Config.Value("http.trustedProxies").List()); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
	res = load(t, input{args: []string{"--observability-otlp-headers-file", "/h"}, files: fakeFS{"/h": {data: "a=1,b=2\n", mode: 0o400}}})
	if diff := cmp.Diff(map[string]string{"a": "1", "b": "2"}, res.Config.Value("observability.otlp.headers").Map()); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestWrongShapeInTheFile(t *testing.T) {
	t.Parallel()

	files := fakeFS{cfgPath: {data: "http.address: [a, b]\nhttp.trustedProxies: {a=b}\nlog.level:"}}
	res := load(t, input{args: []string{"--config", cfgPath}, files: files})
	if !hasIssue(res, config.SeverityError, "http.address", "single value") {
		t.Errorf("issues = %v", issuesFor(res, "http.address"))
	}
	if !hasIssue(res, config.SeverityError, "http.trustedProxies", "must be a list") {
		t.Errorf("issues = %v", issuesFor(res, "http.trustedProxies"))
	}
	if got := res.Config.Source("log.level"); got.Kind != config.SourceDefault {
		t.Errorf("a null value counts as not set, source = %v", got)
	}
}

func TestMissingAndHugeConfigFiles(t *testing.T) {
	t.Parallel()

	res := load(t, input{args: []string{"--config", "/missing.yaml"}})
	if !hasIssue(res, config.SeverityError, "config", "cannot read the configuration file /missing.yaml: file does not exist") {
		t.Errorf("issues = %v", res.Issues)
	}
	res = load(t, input{args: []string{"--config", cfgPath}, files: fakeFS{cfgPath: {data: strings.Repeat("#", 1<<20+5)}}})
	if !hasIssue(res, config.SeverityError, "config", "larger than") {
		t.Errorf("issues = %v", res.Issues)
	}
}

func TestSecretsInAWorldReadableFileWarn(t *testing.T) {
	t.Parallel()

	files := fakeFS{cfgPath: {data: "store.postgres.dsn: opaque-connection-value", mode: 0o644}}
	res := load(t, input{args: []string{"--config", cfgPath}, files: files})
	if !hasIssue(res, config.SeverityWarning, "store.postgres.dsn", "other users can read") {
		t.Fatalf("issues = %v", res.Issues)
	}
	files[cfgPath] = fakeFile{data: files[cfgPath].data, mode: 0o600}
	res = load(t, input{args: []string{"--config", cfgPath}, files: files})
	if hasIssue(res, config.SeverityWarning, "store.postgres.dsn", "other users can read") {
		t.Fatalf("a 0600 file must not warn: %v", res.Issues)
	}
}

func TestCrossValidation(t *testing.T) {
	t.Parallel()

	const dsn = "ORCON_STORE_POSTGRES_DSN"
	tests := []struct {
		name  string
		in    input
		sev   config.Severity
		key   string
		match string
		want  bool
	}{
		{"http publicUrl on a public host", input{environ: baseEnv("ORCON_PUBLIC_URL", "http://orcon.example.com")}, config.SeverityError, "publicUrl", "require HTTPS", true},
		{"http publicUrl on localhost", input{environ: baseEnv("ORCON_PUBLIC_URL", "http://localhost:8080")}, config.SeverityError, "publicUrl", "", false},
		{"http publicUrl on 127.0.0.2", input{environ: baseEnv("ORCON_PUBLIC_URL", "http://127.0.0.2:8080")}, config.SeverityError, "publicUrl", "", false},
		{"http publicUrl on ::1", input{environ: baseEnv("ORCON_PUBLIC_URL", "http://[::1]:8080")}, config.SeverityError, "publicUrl", "", false},
		{"https publicUrl", input{environ: baseEnv("ORCON_PUBLIC_URL", "https://orcon.example.com")}, config.SeverityError, "publicUrl", "", false},
		{
			"http publicUrl with TLS in the binary",
			input{environ: baseEnv("ORCON_PUBLIC_URL", "http://orcon.example.com", "ORCON_HTTP_TLS_CERTIFICATE_FILE", "/c", "ORCON_HTTP_TLS_KEY_FILE", "/k")},
			config.SeverityError, "publicUrl", "", false,
		},
		{"sqlite with a worker", input{environ: baseEnv("ORCON_ROLE", "worker")}, config.SeverityError, "store.driver", "only one process", true},
		{"sqlite with api", input{environ: baseEnv("ORCON_ROLE", "api")}, config.SeverityError, "store.driver", "only one process", true},
		{"postgres with a worker", input{environ: baseEnv("ORCON_ROLE", "worker", "ORCON_STORE_DRIVER", "postgres", dsn, "postgres://h/db")}, config.SeverityError, "store.driver", "", false},
		{"postgres without dsn", input{environ: baseEnv("ORCON_STORE_DRIVER", "postgres", "ORCON_ROLE", "api")}, config.SeverityError, "store.postgres.dsn", "required", true},
		{
			"filesystem blobs on postgres",
			input{environ: baseEnv("ORCON_STORE_DRIVER", "postgres", dsn, "postgres://h/db", "ORCON_BLOBS_BACKEND", "filesystem", "ORCON_ROLE", "api")},
			config.SeverityWarning, "blobs.backend", "shared by every process", true,
		},
		{"postgres blobs on sqlite", input{environ: baseEnv("ORCON_BLOBS_BACKEND", "postgres")}, config.SeverityError, "blobs.backend", "store.driver must be postgres", true},
		{"s3 blobs without bucket", input{environ: baseEnv("ORCON_BLOBS_BACKEND", "s3")}, config.SeverityError, "blobs.s3.bucket", "required", true},
		{
			"keys target equal to data target",
			input{environ: baseEnv("ORCON_BACKUP_TARGET", "file:///backups/orcon", "ORCON_BACKUP_KEYS_TARGET", "file:///backups/orcon/")},
			config.SeverityError, "backup.keys.target", "equals backup.target", true,
		},
		{
			"keys target inside data target",
			input{environ: baseEnv("ORCON_BACKUP_TARGET", "file:///backups", "ORCON_BACKUP_KEYS_TARGET", "file:///backups/keys")},
			config.SeverityError, "backup.keys.target", "inside backup.target", true,
		},
		{
			"data target inside keys target",
			input{environ: baseEnv("ORCON_BACKUP_TARGET", "s3://b/p/data", "ORCON_BACKUP_KEYS_TARGET", "s3://b/p")},
			config.SeverityError, "backup.keys.target", "contains backup.target", true,
		},
		{
			"sibling directories",
			input{environ: baseEnv("ORCON_BACKUP_TARGET", "file:///backups/data", "ORCON_BACKUP_KEYS_TARGET", "file:///backups/database-keys")},
			config.SeverityError, "backup.keys.target", "", false,
		},
		{
			"different buckets",
			input{environ: baseEnv("ORCON_BACKUP_TARGET", "s3://data/orcon", "ORCON_BACKUP_KEYS_TARGET", "s3://keys/orcon")},
			config.SeverityError, "backup.keys.target", "", false,
		},
		{"schedule without target", input{environ: baseEnv("ORCON_BACKUP_SCHEDULE", "0 3 * * *")}, config.SeverityError, "backup.schedule", "needs backup.target", true},
		{"env master key warns", input{environ: baseEnv("ORCON_MASTER_KEY", "aGVsbG8=", "ORCON_KEYS_MASTER_PROVIDER", "env")}, config.SeverityWarning, "keys.master.provider", "process environment", true},
		{"env provider needs the key", input{environ: baseEnv("ORCON_KEYS_MASTER_PROVIDER", "env")}, config.SeverityError, "keys.master.key", "required", true},
		{"kms provider needs the key arn", input{environ: baseEnv("ORCON_KEYS_MASTER_PROVIDER", "awskms")}, config.SeverityError, "keys.master.awskms.keyArn", "required", true},
		{"vault provider needs the mount", input{environ: baseEnv("ORCON_KEYS_MASTER_PROVIDER", "vault-transit")}, config.SeverityError, "keys.master.vaultTransit.mount", "required", true},
		{"gcp provider needs the key name", input{environ: baseEnv("ORCON_KEYS_MASTER_PROVIDER", "gcpkms")}, config.SeverityError, "keys.master.gcpkms.keyName", "required", true},
		{"azure provider needs the key id", input{environ: baseEnv("ORCON_KEYS_MASTER_PROVIDER", "azurekv")}, config.SeverityError, "keys.master.azurekv.keyId", "required", true},
		{"vault provider needs token", input{environ: baseEnv("ORCON_KEYS_MASTER_PROVIDER", "vault-transit")}, config.SeverityError, "keys.master.vaultTransit.token", "required", true},
		{"tls certificate without key", input{environ: baseEnv("ORCON_HTTP_TLS_CERTIFICATE_FILE", "/c")}, config.SeverityError, "http.tls.keyFile", "together", true},
		{"otlp without endpoint", input{environ: baseEnv("ORCON_OBSERVABILITY_TRACES_OTLP_ENABLED", "true")}, config.SeverityError, "observability.otlp.endpoint", "required", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res := load(t, tt.in)
			got := hasIssue(res, tt.sev, tt.key, tt.match)
			if got != tt.want {
				t.Fatalf("issue on %s containing %q: got %v, want %v; issues = %v", tt.key, tt.match, got, tt.want, res.Issues)
			}
		})
	}
}

func TestValidationErrorListsEveryError(t *testing.T) {
	t.Parallel()

	res := load(t, input{environ: baseEnv("ORCON_ROLE", "worker", "ORCON_PUBLIC_URL", "http://example.com", "ORCON_NOISE", "1")})
	err := res.Err()
	var ve *config.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Err = %v, want *ValidationError", err)
	}
	if len(ve.Issues) != 2 {
		t.Fatalf("errors = %v, want exactly the two errors and not the warning", ve.Issues)
	}
	msg := err.Error()
	for _, want := range []string{"publicUrl", "store.driver"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to name %s", msg, want)
		}
	}
	if res.Issues[len(res.Issues)-1].Severity != config.SeverityWarning {
		t.Errorf("warnings must come after errors: %v", res.Issues)
	}
}

func TestDurationAccessors(t *testing.T) {
	t.Parallel()

	res := load(t, input{args: []string{"--http-shutdown-timeout", "45s"}})
	if got := res.Config.Value("http.shutdownTimeout").Duration(); got != 45*time.Second {
		t.Fatalf("Duration = %v, want 45s", got)
	}
	if got := res.Config.Value("runtime.memoryLimitRatio").Float(); got != 0.9 {
		t.Fatalf("Float = %v, want 0.9", got)
	}
	if got := res.Config.Role(); got != config.RoleStandalone {
		t.Fatalf("Role = %q", got)
	}
}
