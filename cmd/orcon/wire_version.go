// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
)

// versionCommand prints the version of the binary.
func versionCommand() command {
	return command{
		name:    "version",
		summary: "print the version",
		run: func(_ context.Context, _ []string, stdout, _ io.Writer) int {
			_, _ = fmt.Fprintf(stdout, "orcon %s (%s)\n", version, commit)
			return exitOK
		},
	}
}
