// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport_test

import (
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

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
	// Added by Amendment 9 and by the pinned inventory.
	"start", "n8n", "htmlExtract", "readPDF", "iCal", "timeSaved", "simulate", "simulateTrigger",
	"e2eTest", "e2eTestPollingTrigger",
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
	// The only unsupported core types are the ones the design defers or that
	// n8n keeps for its own tests.
	for _, row := range n8nimport.CoreNodes() {
		if row.Mapped() {
			continue
		}
		if !slices.Contains([]string{"dataTable", "e2eTest", "timeSaved"}, row.N8nTypes[0]) {
			t.Errorf("unexpected unsupported row %v", row.N8nTypes)
		}
	}
	for short, want := range map[string]n8nimport.Disposition{
		"stickyNote": n8nimport.DispEditor, "start": n8nimport.DispTrigger, "n8n": n8nimport.DispConnector,
	} {
		if row, _ := n8nimport.LookupCore("n8n-nodes-base." + short); row.Disposition != want {
			t.Errorf("%s: disposition %s, want %s", short, row.Disposition, want)
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
// inventory of the pinned n8n release, which tools/n8ncorpus writes next to the
// corpus. The test is skipped only when ORCON_N8N_CORPUS is not set; with a
// corpus and no inventory it fails.
func TestCoreNodeListAgainstPinnedRelease(t *testing.T) {
	dir := os.Getenv("ORCON_N8N_CORPUS")
	if dir == "" {
		t.Skip("ORCON_N8N_CORPUS is not set")
	}
	inv, err := n8nimport.ReadInventory(os.DirFS(dir), n8nimport.InventoryFile)
	if err != nil {
		t.Fatalf("the corpus has no inventory of the pinned release: %v", err)
	}
	if len(inv.Nodes) == 0 || inv.N8nVersion == "" {
		t.Fatalf("empty inventory: %+v", inv)
	}
	if gaps := n8nimport.CheckInventory(inv); len(gaps) > 0 {
		t.Fatalf("core types of n8n %s without a counterpart: %v", inv.N8nVersion, gaps)
	}
	for typ, question := range n8nimport.PendingOwner(inv) {
		t.Logf("%s waits for the owner decision %s: the release gate cannot close while it is listed", typ, question)
	}
}

// TestAIPlanCoversInventory checks the AI plan against the inventory of the
// pinned release: every AI type of the inventory is classified (planned,
// explicitly unsupported or explicitly unnamed), and the plan names no type the
// inventory lacks. It runs only with ORCON_N8N_CORPUS and needs an inventory
// that has the AI group.
func TestAIPlanCoversInventory(t *testing.T) {
	dir := os.Getenv("ORCON_N8N_CORPUS")
	if dir == "" {
		t.Skip("ORCON_N8N_CORPUS is not set")
	}
	inv, err := n8nimport.ReadInventory(os.DirFS(dir), n8nimport.InventoryFile)
	if err != nil {
		t.Fatalf("the corpus has no inventory of the pinned release: %v", err)
	}
	inInventory := map[string]bool{}
	ai := 0
	for _, n := range inv.Nodes {
		inInventory[n.Type] = true
		if n.Group != n8nimport.GroupAI {
			continue
		}
		ai++
		if !n8nimport.AIClassified(n.Type) {
			t.Errorf("the AI type %s is not in the AI plan or in the list of unnamed types: classify it", n.Type)
		}
	}
	if ai == 0 {
		t.Fatal("the inventory has no AI group: write it again with the current tool")
	}
	for _, typ := range n8nimport.AIPlanTypes() {
		if !inInventory[typ] {
			t.Errorf("the AI plan names %s, which the inventory of n8n %s lacks", typ, inv.N8nVersion)
		}
	}
}

func TestAITransformCountsOnlyWhenMapped(t *testing.T) {
	tp, err := n8nimport.ParseTemplate([]byte(`{"nodes":[{"name":"T","type":"n8n-nodes-base.aiTransform","typeVersion":1,"parameters":{}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if res := n8nimport.Classify(&tp, nil); res.Importable() || !res.HasAI {
		t.Errorf("an AI node is unmapped unless the catalog maps it: %+v", res)
	}
	if res := n8nimport.Classify(&tp, n8nimport.PlannedBase()); !res.Importable() || !res.HasAI {
		t.Errorf("the planned catalog maps AI Transform: %+v", res)
	}
	if _, ok := n8nimport.LookupCore("n8n-nodes-base.aiTransform"); ok {
		t.Error("AI Transform is an AI node and has no row in the core table")
	}
	code := n8nimport.AIPlanTypes()
	if !slices.Contains(code, "@n8n/n8n-nodes-langchain.code") {
		t.Fatal("LangChain Code must be classified")
	}
	for _, e := range n8nimport.PlannedAI() {
		if e.Type == "@n8n/n8n-nodes-langchain.code" || e.Type == "@n8n/n8n-nodes-langchain.manualChatTrigger" {
			t.Errorf("%s must stay unmapped", e.Type)
		}
	}
}

func TestReadInventory(t *testing.T) {
	good := fstest.MapFS{n8nimport.InventoryFile: {Data: []byte(`{"n8nVersion":"1","nodes":[{"type":"n8n-nodes-base.if","group":"CORE"}]}`)}}
	if inv, err := n8nimport.ReadInventory(good, n8nimport.InventoryFile); err != nil || len(inv.Nodes) != 1 {
		t.Fatalf("inventory = %+v, %v", inv, err)
	}
	if _, err := n8nimport.ReadInventory(fstest.MapFS{}, n8nimport.InventoryFile); err == nil {
		t.Fatal("a missing inventory must be an error")
	}
}
