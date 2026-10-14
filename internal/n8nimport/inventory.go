// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Inventory is the node-type list of one n8n release. The tool n8ncorpus writes
// it next to the corpus. The preferred source is a locally running instance of
// the pinned release, which serves the list of node types to its editor
// (source RUNNING_INSTANCE); the fallback is the dated public core-node index
// of docs.n8n.io (source DOCS_INDEX). No n8n package is downloaded or opened,
// only the node type and its group are kept, and no n8n code or text is copied:
//
//	{"n8nVersion":"2.43.4","source":"RUNNING_INSTANCE","nodes":[{"type":"n8n-nodes-base.if","group":"CORE"}]}
//
// Group is CORE for the nodes whose public category is the core nodes one, AI
// for the AI types (the LangChain package and the base nodes SPEC-006 Annex B
// names as AI nodes), and OTHER for the rest.
type Inventory struct {
	N8nVersion string
	// Source is RUNNING_INSTANCE or DOCS_INDEX.
	Source string
	Nodes  []InventoryNode
}

// InventoryNode is one node type of the inventory.
type InventoryNode struct {
	Type  string
	Group string
}

// Inventory groups.
const (
	// GroupCore is the group of the n8n core nodes.
	GroupCore = "CORE"
	// GroupAI is the group of the AI types.
	GroupAI = "AI"
)

// InventoryFile is the name of the inventory inside a corpus directory.
const InventoryFile = "inventory.json"

// ReadInventory reads the inventory that travels with a corpus. A missing
// file is an error: the core list cannot be verified without it.
func ReadInventory(fsys fs.FS, name string) (*Inventory, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("n8nimport: read inventory: %w", err)
	}
	return ParseInventory(data)
}

// ParseInventory reads an inventory file.
func ParseInventory(data []byte) (*Inventory, error) {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("n8nimport: parse inventory: %w", err)
	}
	nodes, ok := v.Get("nodes")
	if !ok || nodes.Kind() != jsonvalue.KindArray {
		return nil, errors.New("n8nimport: the inventory has no nodes array")
	}
	inv := &Inventory{N8nVersion: strMember(v, "n8nVersion"), Source: strMember(v, "source")}
	for _, n := range nodes.Items() {
		inv.Nodes = append(inv.Nodes, InventoryNode{Type: strMember(n, "type"), Group: strMember(n, "group")})
	}
	return inv, nil
}

// CheckInventory returns the core types of the inventory that have no
// counterpart, sorted. A new core type blocks the release gate until a row of
// SPEC-003 Annex B names it. A core type whose row marks it unsupported is a
// gap too, except the ones an Annex B row names as out of scope (see
// documentedGap). A core type that SPEC-006 Annex B classifies as an AI node is
// covered by the AI plan.
func CheckInventory(inv *Inventory) []string {
	var gaps []string
	for _, n := range inv.Nodes {
		if n.Group != GroupCore {
			continue
		}
		if AIClassified(n.Type) {
			continue
		}
		row, ok := LookupCore(n.Type)
		if !ok || (!row.Mapped() && !documentedGap(n.Type)) {
			gaps = append(gaps, n.Type)
		}
	}
	slices.Sort(gaps)
	return slices.Compact(gaps)
}

// documentedGap is the exact copy of the rows of SPEC-003 Annex B that leave a
// core type without a counterpart: Data Table, and n8n's internal nodes without
// public documentation (Amendment 10). There is no other exemption list, and a
// new type enters here only by an amendment of the annex;
// TestCoreExemptionsMatchAnnex keeps this function and the table together.
func documentedGap(nodeType string) bool {
	switch nodeType {
	case basePrefix + "dataTable",
		basePrefix + "e2eTest", basePrefix + "e2eTestPollingTrigger",
		basePrefix + "simulate", basePrefix + "simulateTrigger":
		return true
	default:
		return false
	}
}
