// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import "slices"

// Disposition says how an n8n core node reaches Orcon.
type Disposition string

// Dispositions of the core table. The values appear in the coverage report,
// so they follow UPPER_SNAKE (ADR-0080 section 3).
const (
	// DispEngine is covered by the flow engine (SPEC-001).
	DispEngine Disposition = "ENGINE"
	// DispNode is a native Orcon node of the data and utility set (CN13).
	DispNode Disposition = "NODE"
	// DispConnector is an official connector (declarative or native).
	DispConnector Disposition = "CONNECTOR"
	// DispHost is a host-access node, disabled by default (R-HOST-01).
	DispHost Disposition = "HOST"
	// DispTrigger is a trigger family of SPEC-002.
	DispTrigger Disposition = "TRIGGER"
	// DispEditor is editor data, not an executable node.
	DispEditor Disposition = "EDITOR"
	// DispUnsupported has no counterpart yet; the reason says why.
	DispUnsupported Disposition = "UNSUPPORTED"
)

// NamedTemplate is the Orcon column of a row whose types each become the
// Orcon node of the same short name: Sort is orcon.sort@1, Split Out is
// orcon.splitOut@1 (SPEC-003 Annex B).
const NamedTemplate = "orcon.<name>@1"

// CoreNode is one row of the core table: n8n node types that share one
// counterpart in Orcon.
type CoreNode struct {
	// N8nTypes are the full n8n types the row covers, legacy ones included.
	N8nTypes []string
	// Orcon is the Orcon node type or construct that replaces them.
	Orcon string
	// Cut is the cut or spec that delivers the counterpart.
	Cut         string
	Disposition Disposition
	Note        string
}

// coreNodes is SPEC-003 Annex B plus the rows that the trigger inventory of
// SPEC-002 Annex B.1 and the editor data of ADR-0007 add. It is checked
// against the n8n version the corpus pins by TestCoreNodeListComplete and by
// CheckInventory; a new core type blocks the release gate until it has a row.
var coreNodes = []CoreNode{
	{[]string{"if"}, "orcon.if@1", "SPEC-001 C3", DispEngine, "per-item routing (ADR-0010 section 4)"},
	{[]string{"switch"}, "orcon.switch@1", "SPEC-001 C3", DispEngine, "rules and expression modes"},
	{[]string{"merge"}, "join block", "SPEC-001 C3", DispEngine, "the SQL query mode is not supported in V1 and is reported"},
	{[]string{"splitInBatches"}, "orcon.loop@1", "SPEC-001 C3", DispEngine, "the importer converts the cycle"},
	{[]string{"wait"}, "durable wait", "SPEC-001 C8", DispEngine, "ADR-0010 section 8"},
	{[]string{"executeWorkflow"}, "callActivity", "SPEC-001 C9", DispEngine, "ADR-0010 section 7"},
	{[]string{"stopAndError"}, "orcon.stopAndError@1", "SPEC-003 CN13", DispNode, "final error with a code"},
	{[]string{"noOp"}, "orcon.noop@1", "SPEC-003 CN13", DispNode, ""},
	{[]string{"code", "function", "functionItem"}, "orcon.code@1", "SPEC-003 CN8", DispNode, "Function and Function Item become JavaScript with items"},
	{[]string{"httpRequest"}, "orcon.http@1", "SPEC-003 CN3", DispNode, "GET and HEAD are READ; every other method is UNSAFE unless a key is declared"},
	{[]string{"set"}, "orcon.set@1", "SPEC-003 CN13", DispNode, "Edit Fields"},
	{[]string{"aggregate", "splitOut", "summarize", "sort", "limit", "filter", "renameKeys", "removeDuplicates", "compareDatasets", "itemLists"},
		NamedTemplate, "SPEC-003 CN13", DispNode, "Remove Duplicates across executions uses flow data"},
	{[]string{"dateTime", "crypto", "jwt", "totp", "xml", "html", "markdown"}, NamedTemplate, "SPEC-003 CN13", DispNode, "pure native nodes"},
	{[]string{"convertToFile", "extractFromFile", "compression", "editImage", "moveBinaryData", "spreadsheetFile"},
		"orcon.file.*@1", "SPEC-003 CN13", DispNode, "streaming native nodes (ADR-0016)"},
	{[]string{"executionData", "debugHelper"}, "orcon.executionData@1, orcon.debug@1", "SPEC-003 CN13", DispNode, ""},
	{[]string{"graphql", "rssFeedRead", "emailSend", "ftp", "ldap"}, "official connector", "SPEC-003 CN4/CN13", DispConnector, "declarative or native"},
	{[]string{"ssh", "git"}, "orcon.host.*@1", "SPEC-003 CN13", DispHost, "local SSH and Git are host-access nodes"},
	{[]string{"executeCommand", "readBinaryFile", "readBinaryFiles", "writeBinaryFile", "readWriteFile"},
		"orcon.host.*@1", "SPEC-003 CN13", DispHost, "disabled by default; allowlists of the admin"},
	{[]string{"htmlExtract"}, "orcon.html@1", "SPEC-003 CN13", DispNode, "legacy name of the HTML node, whose extraction operation replaces it (Amendment 10)"},
	{[]string{"readPDF"}, "orcon.file.*@1", "SPEC-003 CN13", DispNode, "legacy; n8n replaced it with Extract from File, PDF format (Amendment 10)"},
	{[]string{"iCal"}, "orcon.file.*@1", "SPEC-003 CN13", DispNode, "legacy; today's documentation covers it with Convert to File, ICS (Amendment 10)"},
	{[]string{"n8n"}, "official declarative connector over the public n8n REST API", "SPEC-003 CN13", DispConnector,
		"acts on the n8n instance the flow managed, never on the Orcon API; no n8n name or logo (Amendment 9)"},
	{[]string{"start"}, "orcon.manual@1", "SPEC-002", DispTrigger, "legacy entry point of old n8n flows, a manual trigger (Amendment 9)"},
	{[]string{"respondToWebhook"}, "orcon.respond@1", "SPEC-002 T3", DispTrigger, ""},
	{[]string{"webhook"}, "orcon.webhook@1", "SPEC-002", DispTrigger, "family WEBHOOK"},
	{[]string{"scheduleTrigger", "interval", "cron"}, "orcon.schedule@1", "SPEC-002", DispTrigger, "family SCHEDULE; Interval and Cron become rules"},
	{[]string{"manualTrigger"}, "orcon.manual@1", "SPEC-002", DispTrigger, "family INTERNAL"},
	{[]string{"executeWorkflowTrigger"}, "orcon.calledByFlow@1", "SPEC-002", DispTrigger, "family INTERNAL"},
	{[]string{"errorTrigger"}, "orcon.errorTrigger@1", "SPEC-002", DispTrigger, "family INTERNAL"},
	{[]string{"n8nTrigger", "activationTrigger", "workflowTrigger"}, "orcon.lifecycle@1", "SPEC-002", DispTrigger, "family INTERNAL"},
	{[]string{"evaluationTrigger"}, "orcon.evaluation@1", "SPEC-002", DispTrigger, "hook; dataset and metrics belong to BL-006"},
	{[]string{"formTrigger", "form"}, "orcon.form@1", "SPEC-002 T5", DispTrigger, "form pages are durable waits"},
	{[]string{"emailReadImap"}, "orcon.imap@1", "SPEC-002", DispTrigger, "family LISTEN"},
	{[]string{"sseTrigger"}, "orcon.sse@1", "SPEC-002", DispTrigger, "family LISTEN"},
	{[]string{"rssFeedReadTrigger"}, "orcon.rss@1", "SPEC-002", DispTrigger, "family POLL"},
	{[]string{"localFileTrigger"}, "orcon.localFile@1", "SPEC-002", DispTrigger, "family LISTEN; disabled by default"},
	{[]string{"stickyNote"}, "editor.stickyNotes[]", "SPEC-014 (editor); importer CN2", DispEditor, "editor data (ADR-0007 section 2), not a node: left out of node and type counts (Amendment 9)"},
	{[]string{"dataTable"}, "", "own backlog item", DispUnsupported, "tabular storage has no design yet (SPEC-003 Q14); the importer reports it"},
	{[]string{"e2eTest", "e2eTestPollingTrigger", "simulate", "simulateTrigger"}, "", "out of scope", DispUnsupported,
		"n8n's internal nodes without public documentation; reimplementing them would mean reading n8n code (Q9, Amendment 10)"},
	{[]string{"timeSaved"}, "orcon.noop@1", "SPEC-003 CN13", DispNode,
		"Track Time Saved passes the items through; Orcon does not record the minutes saved and the import report says so (owner decision P-TS1, Amendment 10)"},
}

// CoreNodes returns the core table. The caller owns the result.
func CoreNodes() []CoreNode {
	out := make([]CoreNode, len(coreNodes))
	for i, c := range coreNodes {
		c.N8nTypes = slices.Clone(c.N8nTypes)
		out[i] = c
	}
	return out
}

var coreIndex = buildCoreIndex()

func buildCoreIndex() map[string]int {
	idx := make(map[string]int)
	for i, c := range coreNodes {
		for _, t := range c.N8nTypes {
			idx[basePrefix+t] = i
		}
	}
	return idx
}

// LookupCore returns the core row of a full n8n node type.
func LookupCore(nodeType string) (CoreNode, bool) {
	i, ok := coreIndex[nodeType]
	if !ok {
		return CoreNode{}, false
	}
	c := coreNodes[i]
	c.N8nTypes = slices.Clone(c.N8nTypes)
	return c, true
}

// Mapped reports whether the row has a counterpart in Orcon.
func (c CoreNode) Mapped() bool { return c.Disposition != DispUnsupported }

// IsStickyNote reports whether the type is the n8n sticky note, which is
// editor data and not a node.
func IsStickyNote(nodeType string) bool { return nodeType == basePrefix+"stickyNote" }

// IsHTTPRequest reports whether the type is the n8n HTTP Request node.
func IsHTTPRequest(nodeType string) bool { return nodeType == basePrefix+"httpRequest" }
