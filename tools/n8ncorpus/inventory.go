// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Limits and fixed names of the running-instance reader.
const (
	maxNodeTypesFile = 64 << 20
	coreCategory     = "Core Nodes"
	basePackage      = "n8n-nodes-base"
	aiPackage        = "@n8n/n8n-nodes-langchain"

	// sourceRunningInstance is the inventory source this tool produces.
	sourceRunningInstance = "RUNNING_INSTANCE"

	settingsPath  = "/rest/settings"
	nodeTypesPath = "/types/nodes.json"
)

// baseAITypes are the base-package nodes that SPEC-006 Annex B names as AI
// nodes and that the coverage check treats apart from the core list.
var baseAITypes = map[string]bool{basePackage + ".aiTransform": true}

// InventoryNode is one node type of the pinned release.
type InventoryNode struct {
	Type string `json:"type"`
	// Group is CORE when the public category of the node is the core nodes
	// one, AI for the AI types, and OTHER for the rest.
	Group string `json:"group"`
}

// Inventory lists the node types of the pinned n8n release. Only type names
// and the group are kept: nothing of n8n's code or text is copied.
type Inventory struct {
	// N8nVersion is the version the instance reported, equal to the pinned one.
	N8nVersion string `json:"n8nVersion"`
	// Source is RUNNING_INSTANCE: the list comes from an instance of the pinned
	// release that someone started locally. No n8n package is downloaded or
	// opened.
	Source string          `json:"source"`
	Nodes  []InventoryNode `json:"nodes"`
}

// fetchInventory reads the node types that a locally running instance of the
// pinned n8n release serves to its editor, after checking that the instance
// declares the pinned version.
func (f *fetcher) fetchInventory(ctx context.Context, instanceURL, pinned string) (*Inventory, error) {
	instanceURL = strings.TrimRight(instanceURL, "/")
	var settings struct {
		Data struct {
			VersionCli string `json:"versionCli"`
		} `json:"data"`
	}
	if err := f.getJSON(ctx, instanceURL+settingsPath, &settings); err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}
	if settings.Data.VersionCli != pinned {
		return nil, fmt.Errorf("inventory: the instance runs n8n %q, the pinned release is %q", settings.Data.VersionCli, pinned)
	}
	raw, err := f.fetch(ctx, instanceURL+nodeTypesPath, maxNodeTypesFile)
	if err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}
	nodes, err := parseNodeTypes(raw)
	if err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}
	return &Inventory{N8nVersion: pinned, Source: sourceRunningInstance, Nodes: nodes}, nil
}

func (f *fetcher) getJSON(ctx context.Context, url string, v any) error {
	body, err := f.fetch(ctx, url, 32<<20)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	return nil
}

// parseNodeTypes reads the list of node descriptions and keeps the type and
// the group. The category list lives in the object of public documentation
// metadata of the description: the function looks for a "categories" array in
// the object members, so it needs no more of the description's shape. A node
// may appear once per version; it is core if any entry says so.
func parseNodeTypes(raw []byte) ([]InventoryNode, error) {
	var list []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("parse the node types: %w", err)
	}
	group := map[string]string{}
	for _, n := range list {
		var name string
		if err := json.Unmarshal(n["name"], &name); err != nil || name == "" {
			continue
		}
		if !strings.Contains(name, ".") {
			name = basePackage + "." + name
		}
		g := groupOf(name, inCoreCategory(n))
		if group[name] == "" || g == "CORE" {
			group[name] = g
		}
	}
	if len(group) == 0 {
		return nil, errors.New("the node type list is empty")
	}
	out := make([]InventoryNode, 0, len(group))
	for name, g := range group {
		out = append(out, InventoryNode{Type: name, Group: g})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out, nil
}

func groupOf(nodeType string, core bool) string {
	switch {
	case strings.HasPrefix(nodeType, aiPackage+".") || baseAITypes[nodeType]:
		return "AI"
	case core:
		return "CORE"
	default:
		return "OTHER"
	}
}

// inCoreCategory reports whether any object member of the description carries
// a categories array that includes the core nodes category.
func inCoreCategory(node map[string]json.RawMessage) bool {
	for _, member := range node {
		var meta struct {
			Categories []string `json:"categories"`
		}
		if json.Unmarshal(member, &meta) != nil {
			continue
		}
		for _, c := range meta.Categories {
			if strings.TrimSpace(c) == coreCategory {
				return true
			}
		}
	}
	return false
}

// writeInventory writes dir/inventory.json and returns its digest.
func writeInventory(dir string, inv *Inventory) (string, error) {
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, "inventory.json"), data, 0o600); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// updateManifest records the inventory in dir/manifest.json.
func updateManifest(dir string, m Manifest) error {
	mb, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), append(mb, '\n'), 0o600)
}
