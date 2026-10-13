// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// nodeRef is a reference from an expression to the output of another node:
// $('Name') or $node["Name"] (ADR-0018 section 1).
type nodeRef struct {
	Name string
	// Guarded is true when the reference is followed by .isExecuted, the
	// explicit test that the node ran (ADR-0008 section 4.2).
	Guarded bool
}

// scanNodeRefs finds the node references in the text of one expression. It is
// a lexical scan, not a parse: it recognizes the two reference forms and
// string literals in single or double quotes, which is what the data-access
// rule needs. The full grammar belongs to the expression engine (ADR-0018).
func scanNodeRefs(expr string) []nodeRef {
	var refs []nodeRef
	for i := range len(expr) {
		if expr[i] != '$' {
			continue
		}
		rest := expr[i+1:]
		var closer byte
		switch {
		case strings.HasPrefix(rest, "("):
			rest, closer = rest[1:], ')'
		case strings.HasPrefix(rest, "node"):
			after := strings.TrimLeft(rest[len("node"):], " \t\r\n")
			if !strings.HasPrefix(after, "[") {
				continue
			}
			rest, closer = after[1:], ']'
		default:
			continue
		}
		name, tail, ok := readQuoted(strings.TrimLeft(rest, " \t\r\n"))
		if !ok {
			continue
		}
		tail = strings.TrimLeft(tail, " \t\r\n")
		if tail == "" || tail[0] != closer {
			continue
		}
		tail = strings.TrimLeft(tail[1:], " \t\r\n")
		guarded := false
		if after, found := strings.CutPrefix(tail, "."); found {
			after = strings.TrimLeft(after, " \t\r\n")
			if member, found := strings.CutPrefix(after, "isExecuted"); found {
				guarded = member == "" || !isIdentChar(member[0])
			}
		}
		refs = append(refs, nodeRef{Name: name, Guarded: guarded})
	}
	return refs
}

func isIdentChar(c byte) bool {
	return c == '_' || c == '$' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// readQuoted reads a single- or double-quoted JavaScript string literal at
// the start of s and returns its value and the text after the closing quote.
func readQuoted(s string) (value, rest string, ok bool) {
	if s == "" || (s[0] != '\'' && s[0] != '"') {
		return "", "", false
	}
	quote := s[0]
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' && i+1 < len(s):
			i++
			b.WriteByte(s[i])
		case c == quote:
			return b.String(), s[i+1:], true
		default:
			b.WriteByte(c)
		}
	}
	return "", "", false
}

// templateSegments returns the code inside the {{ ... }} segments of a
// template string (a parameter value that starts with "=").
func templateSegments(s string) []string {
	body, ok := strings.CutPrefix(s, "=")
	if !ok {
		return nil
	}
	var out []string
	for {
		_, after, found := strings.Cut(body, "{{")
		if !found {
			return out
		}
		code, rest, closed := strings.Cut(after, "}}")
		if !closed {
			return append(out, after)
		}
		out = append(out, code)
		body = rest
	}
}

// refsInTemplate returns the node references of a parameter value: only the
// {{ }} segments of a string that starts with "=" hold code.
func refsInTemplate(s string) []nodeRef {
	var refs []nodeRef
	for _, seg := range templateSegments(s) {
		refs = append(refs, scanNodeRefs(seg)...)
	}
	return refs
}

// refsInValue walks a parameters object and collects the references of every
// template string in it.
func refsInValue(v jsonvalue.Value) []nodeRef {
	switch v.Kind() {
	case jsonvalue.KindString:
		s, _ := v.Str()
		return refsInTemplate(s)
	case jsonvalue.KindArray:
		var refs []nodeRef
		for _, item := range v.Items() {
			refs = append(refs, refsInValue(item)...)
		}
		return refs
	case jsonvalue.KindObject:
		var refs []nodeRef
		for _, m := range v.Members() {
			refs = append(refs, refsInValue(m.Value)...)
		}
		return refs
	case jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool, jsonvalue.KindNumber:
		return nil
	default:
		return nil
	}
}

// refsInExpression returns the references of a field that is an expression as
// a whole (an edge condition, a completion condition), with or without the
// leading "=" and braces.
func refsInExpression(s string) []nodeRef {
	if strings.HasPrefix(s, "=") {
		return refsInTemplate(s)
	}
	return scanNodeRefs(s)
}

// nodeReads lists the node references a node makes through its parameters
// and its expression fields, plus the conditions of its outgoing edges, which
// are evaluated on its output. The compensation precondition is left out: it
// runs after the node finished and may read the node's own output.
func nodeReads(n *Node, outgoing []Edge) []nodeRef {
	refs := refsInValue(n.Parameters)
	if m := n.MultiInstance; m != nil {
		refs = append(refs, refsInExpression(m.InputCollection)...)
		refs = append(refs, refsInExpression(m.CompletionCondition)...)
	}
	if c := n.Call; c != nil && strings.HasPrefix(c.Target, "=") {
		refs = append(refs, refsInTemplate(c.Target)...)
	}
	for _, e := range outgoing {
		refs = append(refs, refsInExpression(e.Condition)...)
	}
	return refs
}
