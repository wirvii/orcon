// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"errors"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Node is one node of an n8n workflow, reduced to what mapping needs.
type Node struct {
	// ID is the id n8n gave the node; it may be empty in old exports.
	ID string
	// Name is the display name, unique in the workflow.
	Name string
	// Type is the full n8n node type, such as "n8n-nodes-base.if".
	Type string
	// TypeVersion is the text of the number n8n wrote, such as "2.1". It is
	// "1" when the workflow omits it, as n8n does.
	TypeVersion string
	// Parameters is the parameters object, or the zero value when absent.
	Parameters jsonvalue.Value
	// Disabled reports a node switched off in the editor.
	Disabled bool
	// Credentials are references by type and name. Nothing else of the
	// credentials member is ever read, so no secret can reach a mapping.
	Credentials []CredentialRef
}

// CredentialRef points at a credential of an n8n workflow by type and name.
type CredentialRef struct {
	Type string
	Name string
}

// Template is one n8n workflow of the public template library.
type Template struct {
	// ID is the library id; 0 when the document has none.
	ID int
	// Views is the total view count of the library entry; 0 when unknown.
	Views int
	Nodes []Node
}

// ErrNoNodes marks a document that holds no nodes array.
var ErrNoNodes = errors.New("n8nimport: the document has no nodes array")

// ParseTemplate reads a template as the library serves it
// ({"workflow":{"id":…,"totalViews":…,"workflow":{"nodes":[…]}}}), the same
// without the outer wrapper, or a plain n8n export ({"nodes":[…]}).
func ParseTemplate(data []byte) (Template, error) {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return Template{}, fmt.Errorf("n8nimport: parse template: %w", err)
	}
	var t Template
	root := v
	if w, ok := v.Get("workflow"); ok && w.Kind() == jsonvalue.KindObject {
		t.ID = intMember(w, "id")
		t.Views = intMember(w, "totalViews")
		root = w
		if inner, ok := w.Get("workflow"); ok && inner.Kind() == jsonvalue.KindObject {
			root = inner
		}
	} else {
		t.ID = intMember(v, "id")
	}
	nodes, ok := root.Get("nodes")
	if !ok || nodes.Kind() != jsonvalue.KindArray {
		return Template{}, ErrNoNodes
	}
	for i, item := range nodes.Items() {
		n, err := nodeFrom(item)
		if err != nil {
			return Template{}, fmt.Errorf("n8nimport: node %d: %w", i, err)
		}
		t.Nodes = append(t.Nodes, n)
	}
	return t, nil
}

func nodeFrom(v jsonvalue.Value) (Node, error) {
	if v.Kind() != jsonvalue.KindObject {
		return Node{}, errors.New("not an object")
	}
	n := Node{
		ID:          strMember(v, "id"),
		Name:        strMember(v, "name"),
		Type:        strMember(v, "type"),
		TypeVersion: "1",
	}
	if n.Type == "" {
		return Node{}, errors.New("no type")
	}
	if tv, ok := v.Get("typeVersion"); ok {
		if text, isNum := tv.NumberText(); isNum {
			n.TypeVersion = text
		}
	}
	if p, ok := v.Get("parameters"); ok && p.Kind() == jsonvalue.KindObject {
		n.Parameters = p
	}
	if d, ok := v.Get("disabled"); ok {
		n.Disabled, _ = d.Bool()
	}
	if c, ok := v.Get("credentials"); ok {
		for _, m := range c.Members() {
			ref := CredentialRef{Type: m.Key}
			if m.Value.Kind() == jsonvalue.KindObject {
				ref.Name = strMember(m.Value, "name")
			}
			n.Credentials = append(n.Credentials, ref)
		}
	}
	return n, nil
}

func strMember(v jsonvalue.Value, key string) string {
	m, ok := v.Get(key)
	if !ok {
		return ""
	}
	s, _ := m.Str()
	return s
}

func intMember(v jsonvalue.Value, key string) int {
	m, ok := v.Get(key)
	if !ok {
		return 0
	}
	n, ok := m.Int64()
	if !ok || n < 0 || n > 1<<31-1 {
		return 0
	}
	return int(n)
}

// IsAI reports whether the node belongs to the AI design (SPEC-006), whose
// coverage does not depend on connectors.
func (n *Node) IsAI() bool { return IsAIType(n.Type) }

// IsAIType reports whether an n8n node type belongs to the AI design: the
// LangChain package, and the legacy OpenAI application node, which SPEC-006
// maps to the provider connectors (docs/reference/n8n-ai-coverage.md).
func IsAIType(t string) bool { return strings.HasPrefix(t, aiPrefix) || t == basePrefix+"openAi" }

const (
	aiPrefix   = "@n8n/n8n-nodes-langchain."
	basePrefix = "n8n-nodes-base."
)

// IsBuiltin reports whether an n8n node type comes from the n8n base package,
// as opposed to the LangChain package or a community package.
func IsBuiltin(t string) bool { return strings.HasPrefix(t, basePrefix) }

// Unit is the unit of coverage: what a node asks of the importer (ADR-0037
// section 2). Resource and Operation are empty when the node has no such
// parameter or leaves it at its default.
type Unit struct {
	Type        string
	TypeVersion string
	Resource    string
	Operation   string
}

// String formats the unit as type@typeVersion[resource:operation].
func (u Unit) String() string {
	s := u.Type + "@" + u.TypeVersion
	if u.Resource != "" || u.Operation != "" {
		s += "[" + u.Resource + ":" + u.Operation + "]"
	}
	return s
}

// Selector is what a node says about the operation it runs.
type Selector struct {
	Resource, Operation string
	// Expression is set when the resource or the operation is an expression
	// (a string starting with "="); the importer cannot read which operation
	// the node runs (SPEC-003 amendment 8, S-7).
	Expression bool
}

// SelectorOf reads the n8n resource and operation parameters of a node.
func SelectorOf(n *Node) Selector {
	var s Selector
	read := func(key string) string {
		p, ok := n.Parameters.Get(key)
		if !ok {
			return ""
		}
		str, isStr := p.Str()
		if !isStr {
			return ""
		}
		if strings.HasPrefix(str, "=") {
			s.Expression = true
			return ""
		}
		return str
	}
	s.Resource = read("resource")
	s.Operation = read("operation")
	return s
}

// UnitOf returns the unit of coverage of a node.
func UnitOf(n *Node) (Unit, Selector) {
	sel := SelectorOf(n)
	return Unit{Type: n.Type, TypeVersion: n.TypeVersion, Resource: sel.Resource, Operation: sel.Operation}, sel
}
