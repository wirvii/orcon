// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"slices"
	"testing"
)

// annexExemptions are the core types that rows of SPEC-003 Annex B leave
// without a counterpart: Data Table, and n8n's internal nodes without public
// documentation (Amendment 10). Adding a type here is a visible review: it
// needs an amendment of the annex first.
var annexExemptions = []string{
	"n8n-nodes-base.dataTable",
	"n8n-nodes-base.e2eTest", "n8n-nodes-base.e2eTestPollingTrigger",
	"n8n-nodes-base.simulate", "n8n-nodes-base.simulateTrigger",
}

// TestCoreExemptionsMatchAnnex keeps the unsupported rows of the core table,
// the exemption function of the inventory check and the annex together.
func TestCoreExemptionsMatchAnnex(t *testing.T) {
	var unsupported []string
	for _, row := range coreNodes {
		if row.Mapped() {
			continue
		}
		for _, short := range row.N8nTypes {
			unsupported = append(unsupported, basePrefix+short)
		}
	}
	slices.Sort(unsupported)
	want := slices.Clone(annexExemptions)
	slices.Sort(want)
	if !slices.Equal(unsupported, want) {
		t.Errorf("unsupported rows = %v, want exactly the annex rows %v", unsupported, want)
	}
	for _, typ := range annexExemptions {
		if !documentedGap(typ) {
			t.Errorf("documentedGap(%s) = false", typ)
		}
	}
	// Track Time Saved is mapped (P-TS1 = A) and so is not an exemption.
	if row, ok := LookupCore(basePrefix + "timeSaved"); !ok || !row.Mapped() || documentedGap(basePrefix+"timeSaved") {
		t.Error("timeSaved must be a mapped row, not an exemption")
	}
	// A type without a row never passes as an exemption.
	if documentedGap("n8n-nodes-base.somethingNew") {
		t.Error("an unknown type must not be exempt")
	}
}
