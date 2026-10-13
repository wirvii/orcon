// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"
	"os"
	"runtime"

	"github.com/wirvii/orcon/internal/adapter/cli"
	"github.com/wirvii/orcon/internal/adapter/codec/configyaml"
	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/platform/config"
)

// configCommand is orcon config check|schema|reload; the second level lives in internal/adapter/cli.
func configCommand() command {
	return command{
		name:    "config",
		summary: "check, print the schema of, or reload the configuration",
		run: func(ctx context.Context, args []string, stdout, stderr io.Writer) int {
			return cli.RunConfig(ctx, args, cliEnv(stdout, stderr))
		},
	}
}

// docsCommand is the hidden orcon docs generate, which writes the references that come from the code.
func docsCommand() command {
	return command{
		name:    "docs",
		summary: "generate the reference documentation",
		hidden:  true,
		run: func(_ context.Context, args []string, stdout, stderr io.Writer) int {
			return cli.RunDocs(args, cliEnv(stdout, stderr))
		},
	}
}

func cliEnv(stdout, stderr io.Writer) cli.Env {
	return cli.Env{
		Stdout:      stdout,
		Stderr:      stderr,
		Environ:     os.Environ(),
		FS:          config.OSFileSystem{},
		Decode:      configyaml.Decode,
		MarshalYAML: configyaml.Marshal,
		GOOS:        runtime.GOOS,
		IsTerminal:  isTerminal(stdout),
		Probes:      configProbes(),
	}
}

// isTerminal reports whether w is a character device, which is what a terminal is.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// configProbes are the connection checks of orcon config check --probe. They read
// and never write. Whoever registers the key of an outgoing connection registers
// its probe (key management service, backup target, S3, SMTP); O2 owns the database.
func configProbes() []cli.Probe {
	return []cli.Probe{
		{
			Name: "database",
			Key:  "store.driver",
			Run:  probeDatabase,
		},
	}
}

// probeDatabase opens the configured database the way the server would, without
// changing it: an authenticated SELECT 1 in a read-only transaction on PostgreSQL,
// a read-only open on SQLite.
func probeDatabase(ctx context.Context, c *config.Config) (string, error) {
	if c.Value("store.driver").Text() == "postgres" {
		return "", postgres.Probe(ctx, c.Secret("store.postgres.dsn").Reveal())
	}
	path := c.Value("store.sqlite.path").Text()
	exists, err := sqlite.Probe(ctx, path)
	if err == nil && !exists {
		return path + " does not exist yet; it will be created on first start", nil
	}
	return path, err
}
