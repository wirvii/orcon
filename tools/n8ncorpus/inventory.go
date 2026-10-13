// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// defaultRegistry is where the package metadata of the pinned release is read.
const defaultRegistry = "https://registry.npmjs.org"

// Limits on what is read from the registry: a package tarball and the one
// metadata file inside it.
const (
	maxTarball       = 256 << 20
	maxNodeTypesFile = 64 << 20
	nodeTypesPath    = "package/dist/types/nodes.json"
	coreCategory     = "Core Nodes"
	basePackage      = "n8n-nodes-base"
)

// InventoryNode is one node type of the pinned release.
type InventoryNode struct {
	Type string `json:"type"`
	// Group is CORE when the public category of the node is the core nodes one.
	Group string `json:"group"`
}

// Inventory lists the node types of the pinned n8n release. Only type names
// and the group are kept: nothing of n8n's code or text is copied.
type Inventory struct {
	N8nVersion string `json:"n8nVersion"`
	// NodesBaseVersion is the version of the built-in nodes package the
	// pinned release depends on.
	NodesBaseVersion string          `json:"nodesBaseVersion"`
	Source           string          `json:"source"`
	Nodes            []InventoryNode `json:"nodes"`
}

type packageMeta struct {
	Dependencies map[string]string `json:"dependencies"`
	Dist         struct {
		Tarball   string `json:"tarball"`
		Integrity string `json:"integrity"`
	} `json:"dist"`
}

// fetchInventory reads the node types of the built-in nodes package that the
// pinned n8n release depends on, from the public package registry. It reads
// the one metadata file the package publishes for editors (the list of node
// descriptions) and never runs or copies package code.
func (f *fetcher) fetchInventory(ctx context.Context, registry, n8nVersion string) (*Inventory, error) {
	var top packageMeta
	if err := f.getJSON(ctx, registry+"/n8n/"+n8nVersion, &top); err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}
	baseVersion := top.Dependencies[basePackage]
	if baseVersion == "" {
		return nil, fmt.Errorf("inventory: n8n %s does not depend on %s", n8nVersion, basePackage)
	}
	var base packageMeta
	if err := f.getJSON(ctx, registry+"/"+basePackage+"/"+baseVersion, &base); err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}
	tarball, err := f.fetch(ctx, base.Dist.Tarball, maxTarball)
	if err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}
	if err := checkIntegrity(tarball, base.Dist.Integrity); err != nil {
		return nil, fmt.Errorf("inventory: %s %s: %w", basePackage, baseVersion, err)
	}
	raw, err := extractFile(tarball, nodeTypesPath, maxNodeTypesFile)
	if err != nil {
		return nil, fmt.Errorf("inventory: %s %s: %w", basePackage, baseVersion, err)
	}
	nodes, err := parseNodeTypes(raw)
	if err != nil {
		return nil, fmt.Errorf("inventory: %w", err)
	}
	return &Inventory{
		N8nVersion: n8nVersion, NodesBaseVersion: baseVersion,
		Source: registry + "/" + basePackage + "/" + baseVersion,
		Nodes:  nodes,
	}, nil
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

// checkIntegrity verifies a sha512 subresource integrity string; a registry
// that offers none is accepted, one that offers a different algorithm fails.
func checkIntegrity(data []byte, integrity string) error {
	if integrity == "" {
		return nil
	}
	want, ok := strings.CutPrefix(integrity, "sha512-")
	if !ok {
		return fmt.Errorf("unsupported integrity %q", integrity)
	}
	sum := sha512.Sum512(data)
	if base64.StdEncoding.EncodeToString(sum[:]) != want {
		return errors.New("the tarball does not match its integrity digest")
	}
	return nil
}

// extractFile returns one file of a gzipped tarball without writing anything
// to disk.
func extractFile(tarball []byte, name string, max int64) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return nil, fmt.Errorf("read tarball: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s is not in the tarball", name)
		}
		if err != nil {
			return nil, fmt.Errorf("read tarball: %w", err)
		}
		if hdr.Name != name || hdr.Typeflag != tar.TypeReg {
			continue
		}
		if hdr.Size > max {
			return nil, fmt.Errorf("%s is larger than %d bytes", name, max)
		}
		return io.ReadAll(io.LimitReader(tr, max))
	}
}

// parseNodeTypes reads the list of node descriptions and keeps the name and
// whether the node is in the core category. The category list lives in the
// object of public documentation metadata of the description: the function
// looks for a "categories" array in the object members, so it needs no more of
// the description's shape. A node may appear once per version; it is core if
// any entry says so.
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
		g := "OTHER"
		if inCoreCategory(n) {
			g = "CORE"
		}
		if group[name] != "CORE" {
			group[name] = g
		}
	}
	if len(group) == 0 {
		return nil, errors.New("the node type list is empty")
	}
	out := make([]InventoryNode, 0, len(group))
	for name, g := range group {
		out = append(out, InventoryNode{Type: basePackage + "." + name, Group: g})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out, nil
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
