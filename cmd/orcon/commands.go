// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
)

// command is a top-level subcommand of the binary. Second-level commands
// such as config check and config schema are resolved inside the entry of their area,
// never here.
type command struct {
	name    string
	summary string
	// hidden commands run but are not listed in the usage.
	hidden bool
	run    func(ctx context.Context, args []string, stdout, stderr io.Writer) int
}

// commands lists the subcommands, one line each, sorted by name. Adding a
// subcommand is one wire_<area>.go file with its entry and one line here, in
// alphabetical position (a literal list, because init-time registration is not
// allowed: architecture-layers.md AL-8.3). TestCommandsSortedAndUnique keeps it honest.
func commands() []command {
	return []command{
		configCommand(),
		docsCommand(),
		n8nCommand(),
		serveCommand(),
		versionCommand(),
	}
}

// dispatch runs the subcommand named by the first argument. No argument or an
// unknown name is a usage error (exit 2); help, -h and --help print the usage and exit 0.
func dispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exitUsage
	}
	name := args[0]
	switch name {
	case "help", "-h", "--help":
		printUsage(stdout)
		return exitOK
	case "--version":
		name = "version"
	}
	for _, c := range commands() {
		if c.name == name {
			return c.run(ctx, args[1:], stdout, stderr)
		}
	}
	_, _ = fmt.Fprintf(stderr, "orcon: unknown command %q\n\n", args[0])
	printUsage(stderr)
	return exitUsage
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage: orcon <command> [flags]")
	_, _ = fmt.Fprintln(w, "\ncommands:")
	for _, c := range commands() {
		if !c.hidden {
			_, _ = fmt.Fprintf(w, "  %-8s %s\n", c.name, c.summary)
		}
	}
	_, _ = fmt.Fprintln(w, "\nRun orcon <command> --help for the flags of a command.")
}
