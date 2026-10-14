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
	// Track Time Saved waits for the owner (P-TS1); it is not an exemption.
	pending := basePrefix + "timeSaved"
	var rest []string
	for _, typ := range unsupported {
		if typ != pending {
			rest = append(rest, typ)
		}
	}
	slices.Sort(rest)
	want := slices.Clone(annexExemptions)
	slices.Sort(want)
	if !slices.Equal(rest, want) {
		t.Errorf("unsupported rows (timeSaved aside) = %v, want exactly the annex rows %v", rest, want)
	}
	for _, typ := range append(slices.Clone(annexExemptions), pending) {
		if got := documentedGap(typ); got != (typ != pending) {
			t.Errorf("documentedGap(%s) = %v", typ, got)
		}
	}
	if len(pendingOwner) != 1 || pendingOwner[pending] != "P-TS1" {
		t.Errorf("pending owner decisions = %v, want only timeSaved (P-TS1)", pendingOwner)
	}
	if !slices.Contains(unsupported, pending) {
		t.Error("timeSaved must have a row without a destination")
	}
	// A type without a row never passes as an exemption.
	if documentedGap("n8n-nodes-base.somethingNew") {
		t.Error("an unknown type must not be exempt")
	}
}
