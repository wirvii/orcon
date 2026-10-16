// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"errors"
	"fmt"
	"strings"
)

// MaxToolNameLength is the longest tool name every provider accepts.
const MaxToolNameLength = 64

// ToolNames maps the tool names an orcon flow uses to the names sent to the
// provider, and back. Providers accept only ^[a-zA-Z0-9_-]{1,64}$ (ADR-0061
// §3), so a name such as "crm.lookup" travels as "crm_lookup". The mapping is
// recorded so a call from the model resolves to the declared tool.
type ToolNames struct {
	toWire   map[string]string
	fromWire map[string]string
}

// NewToolNames builds the mapping for tools. It fails with
// [ErrToolNameCollision] when two declared names sanitize to the same wire
// name, and with an error for an empty declared name.
func NewToolNames(tools []Tool) (ToolNames, error) {
	n := ToolNames{
		toWire:   make(map[string]string, len(tools)),
		fromWire: make(map[string]string, len(tools)),
	}
	for _, t := range tools {
		if t.Name == "" {
			return ToolNames{}, errors.New("tool name is empty")
		}
		wire := SanitizeToolName(t.Name)
		if prev, taken := n.fromWire[wire]; taken && prev != t.Name {
			return ToolNames{}, fmt.Errorf("tools %q and %q map to %q: %w", prev, t.Name, wire, ErrToolNameCollision)
		}
		n.toWire[t.Name] = wire
		n.fromWire[wire] = t.Name
	}
	return n, nil
}

// Wire returns the provider name of the declared tool name, and false when
// the tool was not declared.
func (n ToolNames) Wire(name string) (string, bool) {
	w, ok := n.toWire[name]
	return w, ok
}

// Canonical returns the declared tool name of a wire name the model sent, and
// false when the model named a tool that was not declared.
func (n ToolNames) Canonical(wire string) (string, bool) {
	c, ok := n.fromWire[wire]
	return c, ok
}

// SanitizeToolName replaces every character outside [a-zA-Z0-9_-] with an
// underscore and truncates the result to [MaxToolNameLength] characters. It
// is deterministic, so the same declared name always yields the same wire name.
func SanitizeToolName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if isToolNameRune(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	// Every kept rune is ASCII, so truncating by bytes cannot split a character.
	s := b.String()
	if len(s) > MaxToolNameLength {
		s = s[:MaxToolNameLength]
	}
	return s
}

func isToolNameRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '_', r == '-':
		return true
	default:
		return false
	}
}
