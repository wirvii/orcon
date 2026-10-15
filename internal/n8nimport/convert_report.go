// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"fmt"
	"strings"
)

// OutcomeCounts summarizes the outcomes of a conversion.
type OutcomeCounts struct {
	Nodes       int
	Converted   int
	Adapted     int
	NeedsReview int
	Unsupported int
}

// OutcomeCounts tallies the node outcomes.
func (c *Conversion) OutcomeCounts() OutcomeCounts {
	var k OutcomeCounts
	for _, n := range c.Nodes {
		k.Nodes++
		switch n.Outcome {
		case OutcomeConverted:
			k.Converted++
		case OutcomeAdapted:
			k.Adapted++
		case OutcomeNeedsReview:
			k.NeedsReview++
		case OutcomeUnsupported:
			k.Unsupported++
		}
	}
	return k
}

// Markdown renders the import report for a person: a summary, one row per
// node with what became of it, and the findings that belong to no node. The
// flow can be published only when no node is unsupported and every node the
// flow uses is installed.
func (c *Conversion) Markdown() string {
	var b strings.Builder
	k := c.OutcomeCounts()
	fmt.Fprintf(&b, "# n8n import report: %s\n\n", c.reportName())
	fmt.Fprintf(&b, "- Nodes: %d (converted %d, adapted %d, needs review %d, unsupported %d).\n",
		k.Nodes, k.Converted, k.Adapted, k.NeedsReview, k.Unsupported)
	if k.Unsupported > 0 {
		b.WriteString("- The flow cannot be published while a node is unsupported: each one is kept as an `orcon.unsupported@1` node and must be rebuilt.\n")
	}
	b.WriteString("- Credentials are imported by type and name only; create them again in Orcon.\n\n")

	b.WriteString("## Nodes\n\n")
	b.WriteString("| Node | n8n type | Result | Orcon type | Notes |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, n := range c.Nodes {
		orcon := n.OrconType
		if orcon == "" {
			orcon = "-"
		}
		fmt.Fprintf(&b, "| %s | `%s` v%s | %s | `%s` | %s |\n",
			cell(n.Name), n.N8nType, n.TypeVersion, n.Outcome, orcon, cell(notes(n.Findings)))
	}
	if len(c.Findings) > 0 {
		b.WriteString("\n## Findings outside a node\n\n")
		for _, f := range c.Findings {
			fmt.Fprintf(&b, "- %s: %s\n", f.Code, f.Message)
		}
	}
	if len(c.Problems) > 0 {
		b.WriteString("\n## Validation problems of the produced flow\n\n")
		for _, p := range c.Problems {
			fmt.Fprintf(&b, "- %s\n", p)
		}
	}
	return b.String()
}

// notes joins the messages of a node's findings.
func notes(fs []Finding) string {
	parts := make([]string, 0, len(fs))
	for _, f := range fs {
		parts = append(parts, string(f.Code)+": "+f.Message)
	}
	return strings.Join(parts, "; ")
}

// cell escapes a value for a Markdown table cell.
func cell(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ")
}

// reportName is the workflow name for the report title.
func (c *Conversion) reportName() string {
	if c.Name != "" {
		return c.Name
	}
	return "Imported n8n workflow"
}
