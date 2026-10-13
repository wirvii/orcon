// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package nosecretlog_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/wirvii/orcon/tools/orconvet/nosecretlog"
)

func TestNoSecretLog(t *testing.T) {
	// Each fixture package marks the expected diagnostics with `// want`
	// comments; analysistest fails on a missing or an extra diagnostic.
	analysistest.Run(t, analysistest.TestData(), nosecretlog.Analyzer, "fixture/secrets", "fixture/clean")
}
