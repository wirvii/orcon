// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"io"

	"github.com/wirvii/orcon/internal/adapter/cli"
)

// n8nCommand is orcon n8n <command>; the second level lives in internal/adapter/cli.
func n8nCommand() command {
	return command{
		name:    "n8n",
		summary: "measure how much of an n8n template corpus Orcon covers",
		run: func(ctx context.Context, args []string, stdout, stderr io.Writer) int {
			return cli.N8n(ctx, args, stdout, stderr)
		},
	}
}
