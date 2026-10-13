// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/n8nimport"
)

// annexB lists the n8n core node types of SPEC-003 Annex B and the trigger
// inventory of SPEC-002 Annex B.1, as the names the public documentation
// gives them. Every one needs a row in the core table.
var annexB = []string{
	"if", "switch", "merge", "splitInBatches", "wait", "executeWorkflow", "stopAndError", "noOp",
	"code", "function", "functionItem", "httpRequest", "set",
	"aggregate", "splitOut", "summarize", "sort", "limit", "filter", "renameKeys", "removeDuplicates",
	"compareDatasets", "itemLists",
	"dateTime", "crypto", "jwt", "totp", "xml", "html", "markdown",
	"convertToFile", "extractFromFile", "compression", "editImage", "moveBinaryData", "spreadsheetFile",
	"executionData", "debugHelper",
	"graphql", "rssFeedRead", "emailSend", "ftp", "ssh", "ldap", "git",
	"executeCommand", "readBinaryFile", "readBinaryFiles", "writeBinaryFile", "readWriteFile",
	"respondToWebhook",
	"webhook", "scheduleTrigger", "interval", "cron", "manualTrigger", "executeWorkflowTrigger", "errorTrigger",
	"n8nTrigger", "activationTrigger", "workflowTrigger", "evaluationTrigger", "formTrigger", "form",
	"emailReadImap", "sseTrigger", "rssFeedReadTrigger", "localFileTrigger",
	"dataTable",
}

func TestCoreNodeListComplete(t *testing.T) {
	for _, short := range annexB {
		typ := "n8n-nodes-base." + short
		row, ok := n8nimport.LookupCore(typ)
		if !ok {
			t.Errorf("Annex B type %s has no row in the core table", typ)
			continue
		}
		if row.Cut == "" || row.Disposition == "" {
			t.Errorf("%s: row without cut or disposition: %+v", typ, row)
		}
	}

	seen := map[string]bool{}
	for _, row := range n8nimport.CoreNodes() {
		if len(row.N8nTypes) == 0 {
			t.Errorf("row %+v covers no type", row)
		}
		for _, short := range row.N8nTypes {
			if strings.HasPrefix(short, "n8n-nodes-base.") {
				t.Errorf("%s: write the short name, the table adds the package", short)
			}
			if seen[short] {
				t.Errorf("%s appears in two rows", short)
			}
			seen[short] = true
		}
		if row.Mapped() && row.Orcon == "" {
			t.Errorf("%v maps to nothing", row.N8nTypes)
		}
		if !row.Mapped() && row.Note == "" {
			t.Errorf("%v is unsupported without a reason", row.N8nTypes)
		}
	}
	// The table adds only the sticky note to Annex B (see the open questions).
	for short := range seen {
		if !slices.Contains(annexB, short) && short != "stickyNote" {
			t.Errorf("%s is in the table but not in Annex B", short)
		}
	}
	// The only unsupported core type is the one the design defers.
	for _, row := range n8nimport.CoreNodes() {
		if !row.Mapped() && !slices.Equal(row.N8nTypes, []string{"dataTable"}) {
			t.Errorf("unexpected unsupported row %v", row.N8nTypes)
		}
	}
}

func TestCoreNodeTableIsACopy(t *testing.T) {
	rows := n8nimport.CoreNodes()
	rows[0].N8nTypes[0] = "tampered"
	if _, ok := n8nimport.LookupCore("n8n-nodes-base.tampered"); ok {
		t.Fatal("changing the returned table changed the table")
	}
	row, _ := n8nimport.LookupCore("n8n-nodes-base.if")
	row.N8nTypes[0] = "tampered"
	if _, ok := n8nimport.LookupCore("n8n-nodes-base.if"); !ok {
		t.Fatal("changing a returned row changed the table")
	}
}

func TestCoreNodeInventory(t *testing.T) {
	inv, err := n8nimport.ParseInventory([]byte(`{"n8nVersion":"9.9.9","nodes":[
	  {"type":"n8n-nodes-base.if","group":"CORE"},
	  {"type":"n8n-nodes-base.dataTable","group":"CORE"},
	  {"type":"n8n-nodes-base.slack","group":"INTEGRATION"},
	  {"type":"n8n-nodes-base.brandNewCore","group":"CORE"},
	  {"type":"n8n-nodes-base.brandNewCore","group":"CORE"},
	  {"type":"n8n-nodes-base.anotherNewCore","group":"CORE"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	got := n8nimport.CheckInventory(inv)
	want := []string{"n8n-nodes-base.anotherNewCore", "n8n-nodes-base.brandNewCore"}
	if !slices.Equal(got, want) {
		t.Fatalf("gaps = %v, want %v: a new core type blocks the gate, a documented gap and integrations do not", got, want)
	}
	if _, err := n8nimport.ParseInventory([]byte(`{"nodes":3}`)); err == nil {
		t.Error("an inventory without a nodes array must fail")
	}
	if _, err := n8nimport.ParseInventory([]byte(`not json`)); err == nil {
		t.Error("an unreadable inventory must fail")
	}
}

// TestCoreNodeListAgainstPinnedRelease checks the core table against the
// inventory of the pinned n8n release when ORCON_N8N_INVENTORY names its file.
// Producing the inventory is a step outside this repository (it reads the
// public node reference); without it the test is skipped.
func TestCoreNodeListAgainstPinnedRelease(t *testing.T) {
	path := os.Getenv("ORCON_N8N_INVENTORY")
	if path == "" {
		t.Skip("ORCON_N8N_INVENTORY is not set")
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: the path comes from the test environment
	if err != nil {
		t.Fatal(err)
	}
	inv, err := n8nimport.ParseInventory(data)
	if err != nil {
		t.Fatal(err)
	}
	if gaps := n8nimport.CheckInventory(inv); len(gaps) > 0 {
		t.Fatalf("core types of n8n %s without a counterpart: %v", inv.N8nVersion, gaps)
	}
}
