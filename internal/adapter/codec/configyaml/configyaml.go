// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package configyaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/wirvii/orcon/internal/platform/config"
)

// maxDepth bounds how deep sections nest. Real files nest as deep as the
// longest key path; the bound only stops a hostile document.
const maxDepth = 16

// Decode parses the configuration file. A mapping becomes dotted paths, one
// entry per value. The function isLeaf says which paths hold a value: a mapping or a sequence
// found at such a path is the value (a map or a list), not a section.
//
// The returned error names the line of the problem. On error no entries are returned.
func Decode(data []byte, isLeaf func(path string) bool) ([]config.FileEntry, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil // an empty file sets nothing
		}
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	var extra yaml.Node
	switch err := dec.Decode(&extra); {
	case err == nil:
		return nil, errors.New("the file holds more than one YAML document")
	case !errors.Is(err, io.EOF):
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	root := &doc
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil, nil
		}
		root = root.Content[0]
	}
	if root.Kind == yaml.ScalarNode && root.Tag == "!!null" {
		return nil, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: the file must be a mapping of settings", root.Line)
	}
	w := &walker{isLeaf: isLeaf, seen: map[string]int{}}
	if err := w.section("", root, 0); err != nil {
		return nil, err
	}
	return w.entries, nil
}

type walker struct {
	isLeaf  func(string) bool
	entries []config.FileEntry
	seen    map[string]int // path -> line, to reject duplicates
}

func (w *walker) section(prefix string, m *yaml.Node, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("line %d: sections nest too deep", m.Line)
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		key, val := m.Content[i], m.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag == "!!merge" || key.Value == "<<" {
			return fmt.Errorf("line %d: setting names must be plain words; merge keys and complex keys are not supported", key.Line)
		}
		if key.Value == "" || strings.Contains(key.Value, ".") {
			return fmt.Errorf("line %d: %q is not a valid setting name; nest sections instead of using dots", key.Line, key.Value)
		}
		path := key.Value
		if prefix != "" {
			path = prefix + "." + key.Value
		}
		if err := w.value(path, key, val, depth); err != nil {
			return err
		}
	}
	return nil
}

func (w *walker) value(path string, key, val *yaml.Node, depth int) error {
	if val.Kind == yaml.AliasNode || val.Anchor != "" {
		return fmt.Errorf("line %d: anchors and aliases are not supported", key.Line)
	}
	if val.Kind == yaml.MappingNode && !w.isLeaf(path) {
		return w.section(path, val, depth+1)
	}
	if first, dup := w.seen[path]; dup {
		return fmt.Errorf("line %d: %q is set twice (first at line %d)", key.Line, path, first)
	}
	w.seen[path] = key.Line
	entry := config.FileEntry{Path: path, Line: key.Line}
	switch val.Kind {
	case yaml.ScalarNode:
		if val.Tag == "!!null" {
			entry.Kind = config.EntryNull
		} else {
			entry.Scalar = val.Value
		}
	case yaml.SequenceNode:
		items, err := scalars(val)
		if err != nil {
			return err
		}
		entry.Kind, entry.List = config.EntryList, items
	case yaml.MappingNode:
		pairs, err := members(val)
		if err != nil {
			return err
		}
		entry.Kind, entry.Map = config.EntryMap, pairs
	case yaml.DocumentNode, yaml.AliasNode:
		return fmt.Errorf("line %d: unsupported value for %q", key.Line, path)
	}
	w.entries = append(w.entries, entry)
	return nil
}

func scalars(seq *yaml.Node) ([]string, error) {
	items := make([]string, 0, len(seq.Content))
	for _, n := range seq.Content {
		if n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
			return nil, fmt.Errorf("line %d: list items must be plain values", n.Line)
		}
		items = append(items, n.Value)
	}
	return items, nil
}

func members(m *yaml.Node) (map[string]string, error) {
	out := make(map[string]string, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		if k.Kind != yaml.ScalarNode || v.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("line %d: map members must be plain name: value pairs", k.Line)
		}
		if _, dup := out[k.Value]; dup {
			return nil, fmt.Errorf("line %d: %q is set twice", k.Line, k.Value)
		}
		out[k.Value] = v.Value
	}
	return out, nil
}

// Marshal writes v as YAML with two-space indentation, for orcon config check -o yaml.
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encode YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode YAML: %w", err)
	}
	return buf.Bytes(), nil
}
