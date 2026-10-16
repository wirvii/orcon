// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"io"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/platform/config"
)

// loadTestConfig loads the configuration from the given environment the way
// the composition root does, with the data directory kept out of the host.
func loadTestConfig(t *testing.T, environ ...string) *config.Config {
	t.Helper()
	schema := config.Default()
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	fv := schema.BindFlags(flags)
	if !slices.ContainsFunc(environ, func(kv string) bool { return strings.HasPrefix(kv, "ORCON_DATA_DIR=") }) {
		environ = append(environ, "ORCON_DATA_DIR="+t.TempDir())
	}
	res := schema.Load(config.Input{Flags: fv, Environ: environ, FS: config.OSFileSystem{}, GOOS: runtime.GOOS})
	if err := res.Err(); err != nil {
		t.Fatalf("load the configuration: %v", err)
	}
	return res.Config
}

func TestPostgresOptionsBoundThePoolByMaxConnections(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		environ []string
		want    int32
	}{
		{name: "the schema default is 20", want: 20},
		{name: "the operator setting reaches the pool", environ: []string{"ORCON_STORE_POSTGRES_MAX_CONNECTIONS=7"}, want: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			environ := append([]string{
				"ORCON_STORE_DRIVER=postgres",
				"ORCON_STORE_POSTGRES_DSN=postgres://app@127.0.0.1:5432/orcon",
			}, tt.environ...)
			cfg := loadTestConfig(t, environ...)

			opts := postgresOptions(cfg, "1.2.3")
			if opts.MaxConns != tt.want {
				t.Fatalf("MaxConns = %d, want %d", opts.MaxConns, tt.want)
			}
			if opts.DSN != "postgres://app@127.0.0.1:5432/orcon" || opts.BinaryVersion != "1.2.3" {
				t.Fatalf("the DSN and the binary version must pass through unchanged: %+v", opts)
			}
		})
	}
}
