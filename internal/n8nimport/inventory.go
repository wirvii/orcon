// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"errors"
	"fmt"
	"slices"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Inventory is the node-type list of one n8n release, as an extraction step
// outside this repository writes it from the public node reference:
//
//	{"n8nVersion":"2.43.4","nodes":[{"type":"n8n-nodes-base.if","group":"CORE"}]}
//
// Group is CORE for the nodes the n8n documentation lists as core nodes.
// Only type names are kept; no n8n source is involved.
type Inventory struct {
	N8nVersion string
	Nodes      []InventoryNode
}

// InventoryNode is one node type of the inventory.
type InventoryNode struct {
	Type  string
	Group string
}

// GroupCore is the inventory group of the n8n core nodes.
const GroupCore = "CORE"

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
	inv := &Inventory{N8nVersion: strMember(v, "n8nVersion")}
	for _, n := range nodes.Items() {
		inv.Nodes = append(inv.Nodes, InventoryNode{Type: strMember(n, "type"), Group: strMember(n, "group")})
	}
	return inv, nil
}

// CheckInventory returns the core types of the inventory that have no
// counterpart in the core table, sorted. A new core type blocks the release
// gate until it has one (SPEC-003 Annex B). A row that exists but marks the
// type unsupported counts as a gap too, except for the types the design
// leaves out on purpose and documents.
func CheckInventory(inv *Inventory) []string {
	var gaps []string
	for _, n := range inv.Nodes {
		if n.Group != GroupCore {
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

// documentedGap reports the core types the design leaves unsupported on
// purpose, each with an owner backlog item.
func documentedGap(nodeType string) bool { return nodeType == basePrefix+"dataTable" }
