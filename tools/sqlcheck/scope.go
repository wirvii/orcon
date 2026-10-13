// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"sort"
)

// A statement is split into blocks: one per SELECT, INSERT, UPDATE or DELETE,
// including the ones nested in parentheses (subqueries, common table
// expressions). Every token belongs to the innermost block that contains it, so
// a predicate of a subquery never vouches for a table of the outer query.

type blockKind int

const (
	blockSelect blockKind = iota
	blockInsert
	blockUpdate
	blockDelete
)

// tableRef is a table named in a FROM, JOIN, UPDATE, DELETE or USING position.
type tableRef struct {
	table string
	alias string // equals table when the statement gives no alias
	line  int
}

// block is one statement-level construct.
type block struct {
	kind   blockKind
	group  int   // index of the "(" that encloses the block, -1 at statement level
	owned  []int // indexes of the tokens that belong to this block
	refs   []tableRef
	target *tableRef // the INSERT target
}

// analysis is the structure of one statement.
type analysis struct {
	toks   []token
	parent []int // enclosing "(" of every token, -1 at statement level
	blocks []*block
	owner  []*block // innermost block of every token, nil outside any block
}

// statementKeywords start a block.
var statementKeywords = map[string]blockKind{
	"select": blockSelect, "insert": blockInsert, "replace": blockInsert,
	"update": blockUpdate, "delete": blockDelete,
}

// analyze builds the block structure of toks.
func analyze(toks []token) (*analysis, error) {
	a := &analysis{toks: toks, parent: make([]int, len(toks)), owner: make([]*block, len(toks))}
	var open []int
	for i, t := range toks {
		if len(open) > 0 {
			a.parent[i] = open[len(open)-1]
		} else {
			a.parent[i] = -1
		}
		switch {
		case t.isPunct("("):
			open = append(open, i)
		case t.isPunct(")"):
			if len(open) == 0 {
				return nil, fmt.Errorf("line %d: unbalanced closing parenthesis", t.line)
			}
			open = open[:len(open)-1]
			a.parent[i] = -1
			if len(open) > 0 {
				a.parent[i] = open[len(open)-1]
			}
		}
	}
	if len(open) != 0 {
		return nil, fmt.Errorf("line %d: unbalanced parentheses", toks[open[0]].line)
	}

	var stack []*block
	for i, t := range toks {
		// Leave the blocks whose parenthesis closed.
		for len(stack) > 0 && !a.within(a.parent[i], stack[len(stack)-1].group) {
			stack = stack[:len(stack)-1]
		}
		if t.kind == kindWord {
			stack = a.openBlock(stack, i)
		}
		if len(stack) > 0 {
			top := stack[len(stack)-1]
			top.owned = append(top.owned, i)
			a.owner[i] = top
		}
	}
	for _, b := range a.blocks {
		a.collectRefs(b)
	}
	return a, nil
}

// within reports whether the group opened at inner is inside, or is, the group
// opened at outer (-1 is the statement level, which contains everything).
func (a *analysis) within(inner, outer int) bool {
	for {
		if inner == outer {
			return true
		}
		if inner < 0 {
			return false
		}
		inner = a.parent[inner]
	}
}

// openBlock handles the keyword at index i and returns the updated stack.
func (a *analysis) openBlock(stack []*block, i int) []*block {
	word := a.toks[i].text
	group := a.parent[i]
	switch word {
	case "union", "intersect", "except":
		// A set operator ends the SELECT before it at the same level.
		for len(stack) > 0 && stack[len(stack)-1].group == group && stack[len(stack)-1].kind == blockSelect {
			stack = stack[:len(stack)-1]
		}
		return stack
	case "replace":
		// replace( is the string function; REPLACE INTO is an insert.
		if i+1 >= len(a.toks) || !a.toks[i+1].isWord("into") {
			return stack
		}
	}
	kind, ok := statementKeywords[word]
	if !ok {
		return stack
	}
	if kind == blockUpdate && i > 0 && (a.toks[i-1].isWord("for") || a.toks[i-1].isWord("key") || a.toks[i-1].isWord("do")) {
		return stack // FOR UPDATE is a locking clause and ON CONFLICT DO UPDATE belongs to the insert
	}
	if kind == blockSelect {
		// A sibling SELECT (after a set operator was already handled) replaces the previous one.
		for len(stack) > 0 && stack[len(stack)-1].group == group && stack[len(stack)-1].kind == blockSelect {
			stack = stack[:len(stack)-1]
		}
	}
	b := &block{kind: kind, group: group}
	a.blocks = append(a.blocks, b)
	return append(stack, b)
}

// fromStoppers end a FROM list.
var fromStoppers = map[string]bool{
	"where": true, "group": true, "having": true, "order": true, "limit": true, "offset": true,
	"union": true, "intersect": true, "except": true, "returning": true, "window": true,
	"for": true, "set": true, "on": true, "fetch": true, "values": true, "select": true,
}

// aliasStoppers cannot be an alias.
var aliasStoppers = map[string]bool{
	"where": true, "join": true, "left": true, "right": true, "inner": true, "outer": true, "full": true,
	"cross": true, "natural": true, "on": true, "using": true, "group": true, "order": true,
	"having": true, "limit": true, "offset": true, "union": true, "intersect": true, "except": true,
	"set": true, "returning": true, "window": true, "fetch": true, "for": true, "lateral": true,
	"tablesample": true, "values": true, "select": true, "do": true, "from": true,
}

// collectRefs fills b.refs (and b.target for an insert) from the tokens it owns.
func (a *analysis) collectRefs(b *block) {
	inFrom := false
	expect := false // the next word is a table name
	for k := 0; k < len(b.owned); k++ {
		i := b.owned[k]
		t := a.toks[i]
		if t.kind == kindWord {
			switch {
			case t.text == "from" || t.text == "using":
				inFrom, expect = true, true
				continue
			case t.text == "join":
				inFrom, expect = true, true
				continue
			case t.text == "update" && b.kind == blockUpdate && k == 0:
				inFrom, expect = false, true
				continue
			case t.text == "into" && b.kind == blockInsert:
				ref, next := a.readRef(b, k+1)
				b.target = ref
				k = next - 1
				continue
			case fromStoppers[t.text]:
				inFrom, expect = false, false
				continue
			}
		}
		if inFrom && t.isPunct(",") && !expect && a.parent[i] == b.group {
			expect = true
			continue
		}
		if expect && (t.kind == kindWord || t.isPunct("(")) {
			expect = false
			if t.isPunct("(") || t.text == "lateral" || t.text == "only" {
				expect = t.text == "only" || t.text == "lateral"
				continue
			}
			ref, next := a.readRef(b, k)
			if ref != nil {
				b.refs = append(b.refs, *ref)
			}
			k = next - 1
		}
	}
}

// readRef reads [schema.]table [[AS] alias] starting at b.owned[k] and returns
// the reference (nil when the position is a function call or something else)
// with the position after it.
func (a *analysis) readRef(b *block, k int) (*tableRef, int) {
	if k >= len(b.owned) || a.toks[b.owned[k]].kind != kindWord {
		return nil, k
	}
	name := a.toks[b.owned[k]]
	k++
	for k+1 < len(b.owned) && a.toks[b.owned[k]].isPunct(".") && a.toks[b.owned[k+1]].kind == kindWord {
		name = a.toks[b.owned[k+1]]
		k += 2
	}
	if k < len(b.owned) && a.toks[b.owned[k]].isPunct("(") && b.kind != blockInsert {
		return nil, k // a function such as generate_series(...)
	}
	ref := &tableRef{table: name.text, alias: name.text, line: name.line}
	if k < len(b.owned) && a.toks[b.owned[k]].isWord("as") {
		k++
	}
	if k < len(b.owned) {
		if next := a.toks[b.owned[k]]; next.kind == kindWord && !aliasStoppers[next.text] && !(b.kind == blockInsert) {
			ref.alias = next.text
			k++
		}
	}
	return ref, k
}

// scopedTables is the set of organization-scoped table names.
type scopedTables map[string]bool

// problem is one violation inside a statement.
type problem struct {
	line int
	msg  string
}

// checkOrgStatement verifies a statement of scope org: every organization-scoped
// table it reads or writes is restricted by org_id = <parameter>.
func checkOrgStatement(a *analysis, scoped scopedTables) []problem {
	var out []problem
	for _, b := range a.blocks {
		if b.kind == blockInsert {
			out = append(out, a.checkInsert(b, scoped)...)
		}
		out = append(out, a.checkRestricted(b, scoped)...)
	}
	return out
}

// checkInstanceStatement verifies a statement of scope instance: it touches no
// organization-scoped table.
func checkInstanceStatement(a *analysis, scoped scopedTables) []problem {
	var out []problem
	for _, b := range a.blocks {
		for _, r := range b.allRefs() {
			if scoped[r.table] {
				out = append(out, problem{r.line, fmt.Sprintf("statement of scope instance references organization-scoped table %s", r.table)})
			}
		}
	}
	return out
}

func (b *block) allRefs() []tableRef {
	refs := append([]tableRef(nil), b.refs...)
	if b.target != nil {
		refs = append(refs, *b.target)
	}
	return refs
}

// equality is one comparison of org_id found in a WHERE or ON position.
type equality struct {
	left, right string // qualifier of each side; "" for none
	param       bool   // the right side is a parameter
}

// checkRestricted checks the SELECT, UPDATE and DELETE side of a block: each
// scoped table reference is restricted by an org_id comparison that every row of
// the result must satisfy.
func (a *analysis) checkRestricted(b *block, scoped scopedTables) []problem {
	var refs []tableRef
	for _, r := range b.refs {
		if scoped[r.table] {
			refs = append(refs, r)
		}
	}
	if len(refs) == 0 {
		return nil
	}
	restricted := a.restrictedAliases(b, scoped)
	var out []problem
	for _, r := range refs {
		if !restricted[r.alias] && !restricted[r.table] {
			out = append(out, problem{r.line, fmt.Sprintf("organization-scoped table %s is not restricted by org_id = <parameter> (WHERE or JOIN ON)", r.table)})
		}
	}
	return out
}

// restrictedAliases returns the aliases (or table names) of b whose org_id is
// pinned to a parameter, directly or through equal org_id columns. A correlated
// subquery may lean on the enclosing block: o.org_id = outer.org_id restricts o
// when outer is restricted there.
func (a *analysis) restrictedAliases(b *block, scoped scopedTables) map[string]bool {
	own := map[string]bool{}
	var refs []tableRef
	for _, r := range b.refs {
		own[r.alias], own[r.table] = true, true
		if scoped[r.table] {
			refs = append(refs, r)
		}
	}
	var outer map[string]bool
	if b.group >= 0 {
		if enclosing := a.owner[b.group]; enclosing != nil && enclosing != b {
			outer = a.restrictedAliases(enclosing, scoped)
		}
	}
	restricted := map[string]bool{}
	var links [][2]string
	single := ""
	if len(refs) == 1 {
		single = refs[0].alias
	}
	for _, e := range a.equalities(b) {
		left, right := e.left, e.right
		if left == "" {
			left = single
		}
		if e.param {
			if left != "" {
				restricted[left] = true
			}
			continue
		}
		if right == "" {
			right = single
		}
		if left != "" && right != "" {
			links = append(links, [2]string{left, right})
		}
	}
	pinned := func(q string) bool { return restricted[q] || (!own[q] && outer[q]) }
	for changed := true; changed; {
		changed = false
		for _, l := range links {
			switch {
			case pinned(l[0]) && !restricted[l[1]] && own[l[1]]:
				restricted[l[1]], changed = true, true
			case pinned(l[1]) && !restricted[l[0]] && own[l[0]]:
				restricted[l[0]], changed = true, true
			}
		}
	}
	return restricted
}

// equalities finds comparisons of the form [q.]org_id = param, param = [q.]org_id
// and [q.]org_id = [q2.]org_id among the tokens b owns, in a WHERE or ON clause
// and not under an OR or a NOT.
func (a *analysis) equalities(b *block) []equality {
	var out []equality
	clause := ""
	for k, i := range b.owned {
		t := a.toks[i]
		if t.kind == kindWord && a.parent[i] == b.group {
			switch t.text {
			case "select", "from", "join", "where", "group", "having", "order", "set", "values", "returning", "limit", "using", "window":
				clause = t.text
			case "on":
				clause = "on"
				if k+1 < len(b.owned) && a.toks[b.owned[k+1]].isWord("conflict") {
					clause = "conflict"
				}
			}
		}
		if !t.isWord("org_id") || (clause != "where" && clause != "on") {
			continue
		}
		e, ok := a.comparison(b, k)
		if !ok || !a.conjunctive(b, i) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// comparison reads the comparison around the org_id token at b.owned[k].
func (a *analysis) comparison(b *block, k int) (equality, bool) {
	at := func(n int) (token, bool) {
		if n < 0 || n >= len(b.owned) {
			return token{}, false
		}
		return a.toks[b.owned[n]], true
	}
	qualifierBefore := func(n int) (string, int) {
		if dot, ok := at(n - 1); ok && dot.isPunct(".") {
			if q, ok := at(n - 2); ok && q.kind == kindWord {
				return q.text, n - 2
			}
		}
		return "", n
	}
	left, start := qualifierBefore(k)
	// org_id = <right>
	if op, ok := at(k + 1); ok && op.isOp("=") {
		if r, ok := at(k + 2); ok {
			if r.kind == kindParam {
				return equality{left: left, param: true}, true
			}
			if r.kind == kindWord {
				if dot, ok := at(k + 3); ok && dot.isPunct(".") {
					if col, ok := at(k + 4); ok && col.isWord("org_id") {
						return equality{left: left, right: r.text}, true
					}
				} else if r.text == "org_id" {
					return equality{left: left}, true
				}
			}
		}
	}
	// <param> = org_id
	if op, ok := at(start - 1); ok && op.isOp("=") {
		if l, ok := at(start - 2); ok && l.kind == kindParam {
			return equality{left: left, param: true}, true
		}
	}
	return equality{}, false
}

// conjunctive reports whether the token at index i is combined with the rest of
// the clause by AND only: no OR at its own parenthesis level or at any level
// above it inside the block, and no NOT in front of a group on the way up.
func (a *analysis) conjunctive(b *block, i int) bool {
	level := a.parent[i]
	for {
		for _, j := range b.owned {
			if a.parent[j] == level && a.toks[j].isWord("or") {
				return false
			}
		}
		if level == b.group || level < 0 {
			return true
		}
		if level > 0 && a.toks[level-1].isWord("not") {
			return false
		}
		level = a.parent[level]
	}
}

// checkInsert verifies that an INSERT into a scoped table sets org_id from a
// parameter.
func (a *analysis) checkInsert(b *block, scoped scopedTables) []problem {
	if b.target == nil || !scoped[b.target.table] {
		return nil
	}
	table := b.target.table
	line := b.target.line
	// Position after the target: the column list.
	pos := -1
	for k, i := range b.owned {
		if a.toks[i].isWord("into") {
			pos = k
			break
		}
	}
	cols, after := a.columnList(b, pos)
	if cols == nil {
		return []problem{{line, fmt.Sprintf("INSERT into organization-scoped table %s needs an explicit column list that includes org_id", table)}}
	}
	idx := -1
	for n, c := range cols {
		if c == "org_id" {
			idx = n
		}
	}
	if idx < 0 {
		return []problem{{line, fmt.Sprintf("INSERT into organization-scoped table %s does not set org_id", table)}}
	}
	if after >= len(b.owned) {
		return a.checkInsertSelect(b, idx, table)
	}
	switch word := a.toks[b.owned[after]]; {
	case word.isWord("values"):
		return a.checkValues(b, after+1, idx, table)
	case word.isWord("default"):
		return []problem{{line, fmt.Sprintf("INSERT DEFAULT VALUES into organization-scoped table %s does not set org_id", table)}}
	}
	return nil
}

// checkInsertSelect handles INSERT ... SELECT: the SELECT is a block of its own
// whose scoped tables are checked there; here the item that feeds org_id must be
// a parameter or an org_id column.
func (a *analysis) checkInsertSelect(b *block, idx int, table string) []problem {
	next := b.owned[len(b.owned)-1] + 1
	for _, sel := range a.blocks {
		if sel.kind != blockSelect || len(sel.owned) == 0 || sel.owned[0] != next {
			continue
		}
		item, start := 0, 1
		for m := 1; m <= len(sel.owned); m++ {
			end := m == len(sel.owned)
			if !end {
				j := sel.owned[m]
				end = (a.toks[j].isPunct(",") && a.parent[j] == sel.group) || (a.toks[j].isWord("from") && a.parent[j] == sel.group)
			}
			if !end {
				continue
			}
			if item == idx {
				items := sel.owned[start:m]
				if len(items) == 0 || !(isParamItem(a.toks, items) || a.toks[items[len(items)-1]].isWord("org_id")) {
					return []problem{{a.toks[next].line, fmt.Sprintf("INSERT into organization-scoped table %s: the org_id value must be a parameter or an org_id column", table)}}
				}
				return nil
			}
			if m < len(sel.owned) && a.toks[sel.owned[m]].isWord("from") {
				break
			}
			item++
			start = m + 1
		}
	}
	return nil
}

// columnList reads "(a, b, c)" after the target table; pos is the position of
// INTO in b.owned.
func (a *analysis) columnList(b *block, pos int) ([]string, int) {
	k := pos + 1
	// skip [schema.]table [alias]
	for k < len(b.owned) && (a.toks[b.owned[k]].kind == kindWord || a.toks[b.owned[k]].isPunct(".")) && !a.toks[b.owned[k]].isWord("values") && !a.toks[b.owned[k]].isWord("select") && !a.toks[b.owned[k]].isWord("default") {
		k++
	}
	if k >= len(b.owned) || !a.toks[b.owned[k]].isPunct("(") {
		return nil, k
	}
	var cols []string
	for k++; k < len(b.owned) && !a.toks[b.owned[k]].isPunct(")"); k++ {
		if t := a.toks[b.owned[k]]; t.kind == kindWord {
			cols = append(cols, t.text)
		}
	}
	return cols, k + 1
}

// checkValues verifies that the value at column index idx of every tuple is a
// parameter.
func (a *analysis) checkValues(b *block, from, idx int, table string) []problem {
	var out []problem
	for k := from; k < len(b.owned); k++ {
		i := b.owned[k]
		if a.toks[i].kind == kindWord && a.parent[i] == b.group {
			break // ON CONFLICT, RETURNING: no more tuples
		}
		if !a.toks[i].isPunct("(") || a.parent[i] != b.group {
			continue
		}
		// split the tuple at its own commas
		item, start := 0, k+1
		closeAt := k
		for ; closeAt < len(b.owned); closeAt++ {
			j := b.owned[closeAt]
			if a.toks[j].isPunct(")") && a.parent[j] == b.group {
				break
			}
		}
		for m := k + 1; m <= closeAt && m < len(b.owned); m++ {
			j := b.owned[m]
			if m == closeAt || (a.toks[j].isPunct(",") && a.parent[j] == i) {
				if item == idx {
					if !isParamItem(a.toks, b.owned[start:m]) {
						out = append(out, problem{a.toks[i].line, fmt.Sprintf("INSERT into organization-scoped table %s: the org_id value must be a parameter", table)})
					}
				}
				item++
				start = m + 1
			}
		}
		k = closeAt
	}
	return out
}

// isParamItem reports whether the tokens are one parameter, optionally cast.
func isParamItem(toks []token, idx []int) bool {
	if len(idx) == 0 || toks[idx[0]].kind != kindParam {
		return false
	}
	return len(idx) == 1 || toks[idx[1]].isPunct("::")
}

// sortProblems orders problems by line then message.
func sortProblems(ps []problem) {
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].line != ps[j].line {
			return ps[i].line < ps[j].line
		}
		return ps[i].msg < ps[j].msg
	})
}
