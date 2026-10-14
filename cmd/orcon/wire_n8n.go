// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"

	"github.com/wirvii/orcon/internal/adapter/cli"
)

// runN8n runs "orcon n8n ..." when the arguments start with the word n8n and
// reports whether it did. It is the only place that wires the n8n commands.
//
// The server-side cut O2 (SPEC-015) introduces a single dispatcher of
// subcommands in internal/adapter/cli; when it reaches main, this function
// folds into it as one more case and this file goes away.
func runN8n(ctx context.Context, args []string, stdout, stderr io.Writer) (code int, handled bool) {
	if len(args) == 0 || args[0] != "n8n" {
		return 0, false
	}
	return cli.N8n(ctx, args[1:], stdout, stderr), true
}
