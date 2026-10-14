// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package cli holds the subcommands of the orcon binary that are not the
// server itself. Each command is a function of the shape
// func(ctx, args, stdout, stderr) int that returns the process exit code and
// never calls os.Exit; cmd/orcon dispatches to them.
//
// It is an adapter: it parses flags and files, and passes plain values to the
// areas, which hold the rules.
package cli
