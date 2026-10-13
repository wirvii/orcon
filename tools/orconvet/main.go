// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Command orconvet is Orcon's go vet tool: a multichecker holding the
// project-specific analyzers. Run it with
//
//	go vet -vettool=$(pwd)/.bin/orconvet ./...
//
// Current analyzers: nosecretlog. The orgscope analyzer (SPEC-004) joins later.
package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/wirvii/orcon/tools/orconvet/nosecretlog"
)

func main() {
	multichecker.Main(nosecretlog.Analyzer)
}
