// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"slices"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// iterationNames are the expression names whose value changes from one
// iteration of a loop to the next without a body node writing it: the data of
// the previous node and the iteration counter itself.
var iterationNames = []string{"$json", "$input", "$binary", "$prevNode", "$items", "$runIndex"}

// loopExits implements V13, the loop-exit heuristic (ADR-0010 sections 6
// and 11), a warning. A loop ends when the routing of its body sends a token
// out of the body instead of back to the loop control. If the expressions that
// make that choice read nothing the body writes, nothing in an iteration can
// change the outcome, so the loop only ends by reaching maxIterations.
//
// The rule is a heuristic and errs toward silence: an expression it cannot
// classify (a syntax error, an unknown name, a node chosen at run time)
// counts as a reference, so the warning appears only when every name the exit
// expressions use is known to come from outside the body.
func (v *validator) loopExits(g *graph) {
	for li := range v.doc.Nodes {
		loop := &v.doc.Nodes[li]
		if typeName(loop.Type) != typeLoop {
			continue
		}
		sources := v.backEdgeSources(li)
		if len(sources) == 0 {
			continue // a loop without back edge is reported by V2
		}
		body := v.loopBody(g, li, sources)
		exits := v.exitEdges(g, li, body)
		path := ptr("nodes", li)
		if len(exits) == 0 {
			v.r.warnf(CodeV13, path,
				"loop %q has no exit; it can only end with LoopLimitExceeded (maxIterations %d)", loop.ID, loop.Loop.MaxIterations)
			continue
		}
		if v.exitReadsBody(exits, body, v.carriedWrites(body)) {
			continue
		}
		v.r.warnf(CodeV13, path,
			"the condition that ends loop %q does not read anything its body writes (no output of a node inside the loop, no iteration counter), so it cannot change between iterations; the loop may only end by reaching maxIterations (%d)",
			loop.ID, loop.Loop.MaxIterations)
	}
}

// backEdgeSources lists the nodes that have a back edge into the loop node.
func (v *validator) backEdgeSources(loop int) []int {
	id := v.doc.Nodes[loop].ID
	var out []int
	for _, e := range v.doc.Edges {
		if e.Kind != EdgeBack || e.To != id {
			continue
		}
		for i := range v.doc.Nodes {
			if v.doc.Nodes[i].ID == e.From.Node && !slices.Contains(out, i) {
				out = append(out, i)
			}
		}
	}
	return out
}

// loopBody returns the nodes between the loop control and its back edges: those
// the control reaches that reach a source of a back edge.
func (v *validator) loopBody(g *graph, loop int, sources []int) map[int]bool {
	body := map[int]bool{}
	for x := range v.doc.Nodes {
		if x == loop || !g.reaches(loop, x) {
			continue
		}
		if slices.Contains(sources, x) || slices.ContainsFunc(sources, func(s int) bool { return g.reaches(x, s) }) {
			body[x] = true
		}
	}
	return body
}

// exitEdge is a forward edge from a node of the body to a node outside it.
type exitEdge struct {
	from int
	edge Edge
}

func (v *validator) exitEdges(g *graph, loop int, body map[int]bool) []exitEdge {
	var out []exitEdge
	for from := range body {
		for _, ei := range g.outEdges[from] {
			e := v.doc.Edges[ei]
			if to := g.index[e.To]; to != loop && !body[to] {
				out = append(out, exitEdge{from: from, edge: e})
			}
		}
	}
	return out
}

// exitReadsBody reports whether any expression deciding the exit reads
// something the body writes, or cannot be classified.
func (v *validator) exitReadsBody(exits []exitEdge, body map[int]bool, carried map[string]bool) bool {
	names := map[string]int{}
	for i := range v.doc.Nodes {
		names[v.doc.Nodes[i].Name] = i
	}
	seen := map[int]bool{}
	for _, x := range exits {
		var analyses []expr.Analysis
		if x.edge.Condition != "" {
			analyses = append(analyses, analyzeCondition(x.edge.Condition))
		}
		if !seen[x.from] {
			seen[x.from] = true
			analyses = append(analyses, analyzeTemplates(v.doc.Nodes[x.from].Parameters)...)
		}
		for i := range analyses {
			if readsBody(&analyses[i], body, names, carried) {
				return true
			}
		}
	}
	return false
}

// analyzeCondition analyzes an edge condition, which is written with or
// without the leading "=" and braces.
func analyzeCondition(cond string) expr.Analysis {
	if expr.IsTemplate(cond) {
		return expr.AnalyzeTemplate(cond)
	}
	return expr.Analyze(cond)
}

// analyzeTemplates analyzes every template string inside a parameters value.
func analyzeTemplates(v jsonvalue.Value) []expr.Analysis {
	switch v.Kind() {
	case jsonvalue.KindString:
		s, _ := v.Str()
		if expr.IsTemplate(s) {
			return []expr.Analysis{expr.AnalyzeTemplate(s)}
		}
	case jsonvalue.KindArray:
		var out []expr.Analysis
		for _, item := range v.Items() {
			out = append(out, analyzeTemplates(item)...)
		}
		return out
	case jsonvalue.KindObject:
		var out []expr.Analysis
		for _, m := range v.Members() {
			out = append(out, analyzeTemplates(m.Value)...)
		}
		return out
	case jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool, jsonvalue.KindNumber:
	}
	return nil
}

// readsBody reports whether the analyzed expression reads the data a loop
// iteration changes, or cannot be told apart.
func readsBody(a *expr.Analysis, body map[int]bool, names map[string]int, carried map[string]bool) bool {
	if len(a.Findings) > 0 {
		return true // could not be fully classified: stay silent
	}
	for _, id := range iterationNames {
		if a.Uses(id) {
			return true
		}
	}
	for _, r := range a.NodeRefs {
		i, ok := names[r.Name]
		if !ok || body[i] {
			return true
		}
	}
	for _, name := range variableReadsOf(a) {
		if carried[name] {
			return true
		}
	}
	return false
}

// carriedWrites returns the iteration variables with carry that a node of the
// loop body writes (ADR-0008 section 4.3), found through [writersOf]: they are
// the state an iteration hands to the next one.
func (v *validator) carriedWrites(body map[int]bool) map[string]bool {
	carry := map[string]bool{}
	for _, vr := range v.doc.Variables {
		if vr.Carry != nil && *vr.Carry {
			carry[vr.Name] = true
		}
	}
	out := map[string]bool{}
	for i := range body {
		for _, name := range v.writers(&v.doc.Nodes[i]) {
			if carry[name] {
				out[name] = true
			}
		}
	}
	return out
}

// variableReadsOf returns the execution variables an expression reads. No
// accepted document defines how an expression reads one (ADR-0018 section 1
// lists no name for it; $vars is the project environment), so it returns
// nothing until that decision is taken (SPEC-001 amendment 10, B.3). V13 calls
// it, so completing it is the only change the rule needs.
func variableReadsOf(*expr.Analysis) []string { return nil }
