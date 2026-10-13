// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"slices"
	"strings"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Reason says why a node keeps a template from importing completely.
type Reason string

// Reasons. The values appear in the coverage report (UPPER_SNAKE).
const (
	// ReasonUnmappedType: an n8n built-in type that neither the core table
	// nor the catalog knows. A connector can fix it.
	ReasonUnmappedType Reason = "UNMAPPED_TYPE"
	// ReasonUnmappedUnit: the catalog knows the type but not this
	// typeVersion, resource or operation. A connector can fix it.
	ReasonUnmappedUnit Reason = "UNMAPPED_OPERATION"
	// ReasonSelectorExpression: the resource or operation is an expression,
	// so the importer cannot tell which operation the node runs.
	ReasonSelectorExpression Reason = "SELECTOR_EXPRESSION"
	// ReasonUnsupportedCore: a core node the design has no counterpart for.
	ReasonUnsupportedCore Reason = "UNSUPPORTED_CORE"
	// ReasonCommunity: a node of a community package; only the converter
	// (ADR-0035) can fix it.
	ReasonCommunity Reason = "COMMUNITY_NODE"
	// ReasonExpression: an expression the analyzer of ADR-0018 does not accept.
	ReasonExpression Reason = "EXPRESSION"
	// ReasonAINode and ReasonAIExpression depend on the AI design, not on
	// connectors; they are counted apart (ADR-0037 section 3).
	ReasonAINode       Reason = "AI_NODE"
	ReasonAIExpression Reason = "AI_EXPRESSION"
)

// fixable reports whether adding a connector to the catalog removes the
// reason.
func (r Reason) fixable() bool { return r == ReasonUnmappedType || r == ReasonUnmappedUnit }

// aiReason reports whether the reason belongs to the AI design.
func (r Reason) aiReason() bool { return r == ReasonAINode || r == ReasonAIExpression }

// Blocker is one thing that keeps a template from importing completely.
// Detail never holds template content: only a node type, a unit or the names
// of analyzer finding kinds.
type Blocker struct {
	Reason Reason
	Unit   Unit
	Detail string
}

// Result is the classification of one template.
type Result struct {
	ID    int
	Views int
	// Nodes is the number of nodes of the template.
	Nodes int
	// HasAI is set when the template uses an AI node or an AI-only expression.
	HasAI bool
	// Blockers are the reasons other than the AI design, without repeats.
	Blockers []Blocker
	// Units are the units of coverage the template uses, sorted, without
	// repeats; core and AI nodes are not listed.
	Units []Unit
}

// ImportableIgnoringAI reports whether nothing but the AI design stands in
// the way.
func (r *Result) ImportableIgnoringAI() bool { return len(r.Blockers) == 0 }

// Importable reports whether the template imports completely today: no
// blocker and no AI node.
func (r *Result) Importable() bool { return len(r.Blockers) == 0 && !r.HasAI }

// Fixed reports whether some blocker cannot be removed by adding a connector.
func (r *Result) Fixed() bool {
	return slices.ContainsFunc(r.Blockers, func(b Blocker) bool { return !b.Reason.fixable() })
}

// Classify decides, node by node, whether a template imports completely
// against the catalog.
func Classify(t *Template, cat *Catalog) Result {
	res := Result{ID: t.ID, Views: t.Views, Nodes: len(t.Nodes)}
	seenBlocker := map[Blocker]bool{}
	seenUnit := map[Unit]bool{}
	add := func(b Blocker) {
		if b.Reason.aiReason() {
			res.HasAI = true
			return
		}
		if !seenBlocker[b] {
			seenBlocker[b] = true
			res.Blockers = append(res.Blockers, b)
		}
	}
	for i := range t.Nodes {
		n := &t.Nodes[i]
		typ, tool := normalizeType(n.Type)
		if tool {
			res.HasAI = true // a tool variant is a node offered to an agent
		}
		switch {
		case IsAIType(typ):
			add(Blocker{Reason: ReasonAINode, Unit: Unit{Type: typ}})
		case !IsBuiltin(typ):
			add(Blocker{Reason: ReasonCommunity, Unit: Unit{Type: typ}})
		default:
			classifyBuiltin(n, typ, cat, add, seenUnit, &res)
		}
		for _, b := range expressionBlockers(n, typ) {
			add(b)
		}
	}
	slices.SortFunc(res.Blockers, func(a, b Blocker) int {
		if c := strings.Compare(string(a.Reason), string(b.Reason)); c != 0 {
			return c
		}
		if c := strings.Compare(a.Unit.String(), b.Unit.String()); c != 0 {
			return c
		}
		return strings.Compare(a.Detail, b.Detail)
	})
	slices.SortFunc(res.Units, func(a, b Unit) int { return strings.Compare(a.String(), b.String()) })
	return res
}

func classifyBuiltin(n *Node, typ string, cat *Catalog, add func(Blocker), seen map[Unit]bool, res *Result) {
	if row, ok := LookupCore(typ); ok {
		if !row.Mapped() {
			add(Blocker{Reason: ReasonUnsupportedCore, Unit: Unit{Type: typ}})
		}
		return
	}
	unit, sel := UnitOf(n)
	unit.Type = typ
	if sel.Expression {
		add(Blocker{Reason: ReasonSelectorExpression, Unit: Unit{Type: typ, TypeVersion: unit.TypeVersion}})
		return
	}
	if !seen[unit] {
		seen[unit] = true
		res.Units = append(res.Units, unit)
	}
	if _, _, ok := cat.Lookup(unit); ok {
		return
	}
	if cat.Has(typ) {
		add(Blocker{Reason: ReasonUnmappedUnit, Unit: unit})
		return
	}
	add(Blocker{Reason: ReasonUnmappedType, Unit: unit})
}

// normalizeType folds the "Tool" variant of a built-in node (the same node
// offered to an AI agent as a tool) into its base type.
func normalizeType(t string) (base string, tool bool) {
	if !IsBuiltin(t) {
		return t, false
	}
	if stem, ok := strings.CutSuffix(t, "Tool"); ok && stem != basePrefix {
		return stem, true
	}
	return t, false
}

// expressionBlockers analyzes every expression of the node parameters with
// the analyzer of ADR-0018.
func expressionBlockers(n *Node, typ string) []Blocker {
	var out []Blocker
	walkStrings(n.Parameters, func(s string) {
		if !expr.IsTemplate(s) {
			return
		}
		a := expr.AnalyzeTemplate(s)
		if a.Supported() {
			return
		}
		onlyAI := true
		kinds := map[string]bool{}
		for _, f := range a.Findings {
			kinds[string(f.Kind)] = true
			if f.Name != "$fromAI" {
				onlyAI = false
			}
		}
		if onlyAI {
			out = append(out, Blocker{Reason: ReasonAIExpression, Unit: Unit{Type: typ}})
			return
		}
		names := make([]string, 0, len(kinds))
		for k := range kinds {
			names = append(names, k)
		}
		slices.Sort(names)
		out = append(out, Blocker{Reason: ReasonExpression, Unit: Unit{Type: typ}, Detail: strings.Join(names, ",")})
	})
	return out
}

// walkStrings calls visit with every string value inside v.
func walkStrings(v jsonvalue.Value, visit func(string)) {
	switch v.Kind() {
	case jsonvalue.KindString:
		s, _ := v.Str()
		visit(s)
	case jsonvalue.KindArray:
		for _, item := range v.Items() {
			walkStrings(item, visit)
		}
	case jsonvalue.KindObject:
		for _, m := range v.Members() {
			walkStrings(m.Value, visit)
		}
	case jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool, jsonvalue.KindNumber:
	}
}
