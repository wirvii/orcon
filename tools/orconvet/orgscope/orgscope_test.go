// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package orgscope_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/wirvii/orcon/tools/orconvet/orgscope"
)

func TestOrgScope(t *testing.T) {
	// Each fixture package marks the expected diagnostics with `// want`
	// comments; analysistest fails on a missing or an extra diagnostic.
	// The packages without markers (storetest, connectors) prove that the
	// declared exceptions raise nothing.
	analysistest.Run(t, analysistest.TestData(), orgscope.Analyzer,
		"fixture/internal/area",
		"fixture/internal/adapter/sqlite/helper",
		"fixture/internal/adapter/sqlite/store",
		"fixture/internal/adapter/postgres/store",
		"fixture/internal/adapter/storetest",
		"fixture/internal/adapter/connectors/postgres",
	)
}
