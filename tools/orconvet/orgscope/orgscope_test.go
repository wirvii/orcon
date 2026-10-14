// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package orgscope_test

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/wirvii/orcon/tools/orconvet/orgscope"
)

func TestOrgScope(t *testing.T) {
	// The allowlist is named explicitly: the fixtures are not a repository.
	// A copy of it planted deeper (the rogue package) must not authorize anything.
	td := analysistest.TestData()
	list := filepath.Join(td, "src", "fixture", "internal", "adapter", "storetest", "dbrules", "policy", "dynamic_sql_allowlist.txt")
	if err := orgscope.Analyzer.Flags.Set("allowlist", list); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = orgscope.Analyzer.Flags.Set("allowlist", "") })

	// Each fixture package marks the expected diagnostics with `// want`
	// comments; analysistest fails on a missing or an extra diagnostic.
	// The packages without markers (storetest, connectors) prove that the
	// declared exceptions raise nothing.
	analysistest.Run(t, td, orgscope.Analyzer,
		"fixture/internal/area",
		"fixture/internal/adapter/sqlite/helper",
		"fixture/internal/adapter/sqlite/store",
		"fixture/internal/adapter/sqlite/reviewed",
		"fixture/internal/adapter/sqlite/rogue",
		"fixture/internal/adapter/postgres/store",
		"fixture/internal/adapter/storetest",
		"fixture/internal/adapter/connectors/postgres",
	)
}
