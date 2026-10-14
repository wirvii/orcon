// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command orcon is the Orcon binary: the server and its commands. It is the
// composition root: it builds every dependency, hands the arguments to the
// subcommand they name (commands.go) and owns the process lifetime. All other
// packages receive their dependencies as arguments.
package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// Exit codes returned by run (ADR 0058 section 4, and EX_TEMPFAIL for a restart).
const (
	exitOK         = 0
	exitError      = 1
	exitUsage      = 2
	exitValidation = 5
	// The exit code of a graceful shutdown asking for a restart (EX_TEMPFAIL):
	// the expression sandbox is saturated or stuck, and the restart policy of
	// the deployment must start the process again (ADR-0018 section 3).
	exitRestart = 75
)

// version and commit are injected by the release build with
// -X main.version=<version> -X main.commit=<commit> (.goreleaser.yaml).
var (
	version = "dev"
	commit  = ""
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run is the testable body of the process. It returns the exit code and never
// calls os.Exit. The first argument names the subcommand (commands.go); help goes
// to stdout, diagnostics and logs to stderr.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return dispatch(ctx, args, stdout, stderr)
}
