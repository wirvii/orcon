// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// The check reads the statement with the PostgreSQL parser (ADR-0046 section 3)
// and fails closed: a construct it does not understand in a statement of scope
// org is a finding, never a pass. What it guarantees is stated in
// docs of the command: every organization-scoped table a statement of scope org
// reads or writes is constrained, in the part of the statement that every
// result row must satisfy, by org_id = <parameter>.

// problem is one violation inside a statement.
type problem struct {
	line int
	msg  string
}

// scopedTables is the set of organization-scoped table names.
type scopedTables map[string]bool

// checker holds the state of one statement.
type checker struct {
	text     string // the normalized statement; offsets of nodes index into it
	baseLine int
	scoped   scopedTables
	instance map[string]bool // tables known to have no org_id
	strict   bool            // scope org: an unknown table is a finding
	ctes     []map[string]bool
	problems []problem
}

func (c *checker) lineAt(offset int32) int {
	if offset < 0 || int(offset) > len(c.text) {
		return c.baseLine
	}
	return c.baseLine + strings.Count(c.text[:offset], "\n")
}

func (c *checker) fail(offset int32, format string, args ...any) {
	c.problems = append(c.problems, problem{c.lineAt(offset), fmt.Sprintf(format, args...)})
}

// checkStatement parses text and checks it. scope is org or instance.
func checkStatement(text string, baseLine int, dialect, scope string, scoped scopedTables, instance map[string]bool) []problem {
	normalized := dropInsertOr(normalizeForParser(text, dialect))
	c := &checker{text: normalized, baseLine: baseLine, scoped: scoped, instance: instance, strict: scope == "org"}
	if dialect == "sqlite" && replacesRows(normalized) {
		c.fail(-1, "REPLACE deletes any row that conflicts on a unique key, which can be a row of another organization; use INSERT ... ON CONFLICT with org_id")
		return c.problems
	}
	res, err := pg_query.Parse(normalized)
	if err != nil {
		c.fail(-1, "cannot read the statement (%v); rewrite it in the plain SQL both stores accept", firstLine(err.Error()))
		return c.problems
	}
	if len(res.Stmts) != 1 {
		c.fail(-1, "a query holds exactly one statement, found %d", len(res.Stmts))
		return c.problems
	}
	top := res.Stmts[0].Stmt
	switch inner(top).Interface().(type) {
	case *pg_query.SelectStmt, *pg_query.InsertStmt, *pg_query.UpdateStmt, *pg_query.DeleteStmt:
	default:
		c.fail(-1, "statement %s is not a SELECT, INSERT, UPDATE or DELETE", inner(top).Descriptor().Name())
		return c.problems
	}
	c.checkFunctions(top)
	if scope == "instance" {
		c.checkInstance(top)
	} else {
		c.checkQuery(top, nil)
	}
	sort.SliceStable(c.problems, func(i, j int) bool {
		if c.problems[i].line != c.problems[j].line {
			return c.problems[i].line < c.problems[j].line
		}
		return c.problems[i].msg < c.problems[j].msg
	})
	return c.problems
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// inner returns the message held by a Node wrapper (an empty message for nil).
func inner(n *pg_query.Node) protoreflect.Message {
	if n == nil {
		return (&pg_query.Node{}).ProtoReflect()
	}
	m := n.ProtoReflect()
	od := m.Descriptor().Oneofs().ByName("node")
	fd := m.WhichOneof(od)
	if fd == nil {
		return m
	}
	return m.Get(fd).Message()
}

// walk visits every message below m, parents before children; fn returns false
// to skip the children of a message.
func walk(m protoreflect.Message, fn func(protoreflect.Message) bool) {
	if !m.IsValid() || !fn(m) {
		return
	}
	m.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsList() && fd.Message() != nil:
			l := v.List()
			for i := range l.Len() {
				walk(l.Get(i).Message(), fn)
			}
		case fd.Message() != nil && !fd.IsMap():
			walk(v.Message(), fn)
		}
		return true
	})
}

// allowedFunctions are the only functions a query of scope org or instance may
// call, unqualified: aggregates, window functions, time and text helpers that
// read no table and change no setting. Anything else (set_config,
// current_setting, a user-defined function that could read or delete rows, an
// orcon_engine or orcon_context function) is a finding; the engine plane is the
// place for such calls, through the reviewed allowlist.
var allowedFunctions = map[string]bool{
	"count": true, "sum": true, "avg": true, "min": true, "max": true, "bool_and": true, "bool_or": true,
	"array_agg": true, "string_agg": true, "row_number": true, "rank": true, "dense_rank": true,
	"lag": true, "lead": true, "first_value": true, "last_value": true,
	"coalesce": true, "greatest": true, "least": true, "abs": true, "ceil": true, "floor": true, "round": true,
	"lower": true, "upper": true, "length": true, "substr": true, "substring": true, "trim": true, "concat": true,
	"now": true, "clock_timestamp": true, "date_trunc": true, "generate_series": true, "unnest": true,
	"strftime": true, "datetime": true, "ifnull": true, "iif": true, "instr": true,
}

// syntaxFunctions are the pg_catalog functions the parser produces for SQL
// syntax (TRIM, EXTRACT, POSITION, OVERLAY, AT TIME ZONE).
var syntaxFunctions = map[string]bool{
	"extract": true, "btrim": true, "ltrim": true, "rtrim": true, "date_part": true, "timezone": true, "overlay": true, "position": true,
}

// checkFunctions reports every function call of the statement that is not on
// the allowed list, wherever it stands.
func (c *checker) checkFunctions(top *pg_query.Node) {
	walk(top.ProtoReflect(), func(m protoreflect.Message) bool {
		fc, ok := m.Interface().(*pg_query.FuncCall)
		if !ok {
			return true
		}
		parts := make([]string, 0, len(fc.Funcname))
		for _, n := range fc.Funcname {
			parts = append(parts, n.GetString_().GetSval())
		}
		name := strings.Join(parts, ".")
		ok = len(parts) == 1 && allowedFunctions[parts[0]]
		if len(parts) == 2 && parts[0] == "pg_catalog" {
			// The parser spells TRIM, SUBSTRING, EXTRACT, POSITION and OVERLAY this way.
			ok = allowedFunctions[parts[1]] || syntaxFunctions[parts[1]]
		}
		if !ok {
			c.fail(fc.Location, "function %s is not on the list of functions a query of scope org or instance may call (it could read or change data the check cannot see)", name)
		}
		return true
	})
}

// checkInstance verifies a statement of scope instance: it references no
// organization-scoped table anywhere.
func (c *checker) checkInstance(top *pg_query.Node) {
	cteNames := map[string]bool{}
	walk(top.ProtoReflect(), func(m protoreflect.Message) bool {
		if cte, ok := m.Interface().(*pg_query.CommonTableExpr); ok {
			cteNames[cte.Ctename] = true
		}
		return true
	})
	walk(top.ProtoReflect(), func(m protoreflect.Message) bool {
		if rv, ok := m.Interface().(*pg_query.RangeVar); ok && rv.Schemaname == "" && !cteNames[rv.Relname] && c.scoped[rv.Relname] {
			c.fail(rv.Location, "statement of scope instance references organization-scoped table %s", rv.Relname)
		}
		return true
	})
}

// outerPins are the reference names of enclosing blocks whose org_id is pinned.
type outerPins map[string]bool

// checkQuery checks a SELECT, INSERT, UPDATE or DELETE node.
func (c *checker) checkQuery(n *pg_query.Node, outer outerPins) {
	switch s := inner(n).Interface().(type) {
	case *pg_query.SelectStmt:
		c.checkSelect(s, outer)
	case *pg_query.InsertStmt:
		c.checkInsert(s, outer)
	case *pg_query.UpdateStmt:
		c.checkModify(modifyParts{with: s.WithClause, target: s.Relation, from: s.FromClause, where: s.WhereClause,
			rest: []*pg_query.Node{nodeOf(s.TargetList), nodeOf(s.ReturningList)}}, outer)
	case *pg_query.DeleteStmt:
		c.checkModify(modifyParts{with: s.WithClause, target: s.Relation, from: s.UsingClause, where: s.WhereClause,
			rest: []*pg_query.Node{nodeOf(s.ReturningList)}}, outer)
	default:
		c.fail(-1, "statement %s is not understood; fail closed", inner(n).Descriptor().Name())
	}
}

// nodeOf wraps a list of nodes in a List node so a walk can cover it.
func nodeOf(list []*pg_query.Node) *pg_query.Node {
	return &pg_query.Node{Node: &pg_query.Node_List{List: &pg_query.List{Items: list}}}
}

// pushCTEs checks the common table expressions of a WITH clause and makes their
// names visible until popCTEs.
func (c *checker) pushCTEs(w *pg_query.WithClause) {
	names := map[string]bool{}
	if w != nil {
		for _, n := range w.Ctes {
			if cte := n.GetCommonTableExpr(); cte != nil {
				names[cte.Ctename] = true
			}
		}
	}
	c.ctes = append(c.ctes, names)
	if w == nil {
		return
	}
	for _, n := range w.Ctes {
		if cte := n.GetCommonTableExpr(); cte != nil {
			c.checkQuery(cte.Ctequery, nil)
		}
	}
}

func (c *checker) popCTEs() { c.ctes = c.ctes[:len(c.ctes)-1] }

func (c *checker) isCTE(name string) bool {
	for _, names := range c.ctes {
		if names[name] {
			return true
		}
	}
	return false
}

// ref is one item of a FROM list.
type ref struct {
	name   string // the reference name: the alias, else the table name
	table  string
	scoped bool
	known  bool // a table known to have no org_id
	line   int32
}

// join records the shape of a join for the ON clause.
type join struct {
	kind        pg_query.JoinType
	left, right []string
	quals       *pg_query.Node
	natural     bool
	using       bool
}

// scopeInfo is the FROM list of one block.
type scopeInfo struct {
	refs    []ref
	joins   []join
	derived bool // a subquery or function stands in the list
	names   map[string]bool
}

func (s *scopeInfo) add(r ref) { s.refs = append(s.refs, r); s.names[r.name] = true }

// collectFrom reads a FROM list and returns the reference names under each item.
func (c *checker) collectFrom(items []*pg_query.Node, info *scopeInfo) []string {
	var all []string
	for _, it := range items {
		all = append(all, c.collectItem(it, info)...)
	}
	return all
}

func (c *checker) collectItem(n *pg_query.Node, info *scopeInfo) []string {
	switch v := inner(n).Interface().(type) {
	case *pg_query.RangeVar:
		return c.collectRangeVar(v, info)
	case *pg_query.JoinExpr:
		l := c.collectItem(v.Larg, info)
		r := c.collectItem(v.Rarg, info)
		info.joins = append(info.joins, join{kind: v.Jointype, left: l, right: r, quals: v.Quals,
			natural: v.IsNatural, using: len(v.UsingClause) > 0})
		return append(slices.Clone(l), r...)
	case *pg_query.RangeSubselect:
		info.derived = true
		c.checkQuery(v.Subquery, nil)
		name := aliasName(v.Alias)
		info.add(ref{name: name, table: "", line: -1})
		return []string{name}
	case *pg_query.RangeFunction:
		info.derived = true
		c.walkSublinks(n, nil)
		name := aliasName(v.Alias)
		info.add(ref{name: name, line: -1})
		return []string{name}
	default:
		c.fail(-1, "FROM item %s is not understood; fail closed", inner(n).Descriptor().Name())
		return nil
	}
}

func aliasName(a *pg_query.Alias) string {
	if a == nil {
		return ""
	}
	return a.Aliasname
}

func (c *checker) collectRangeVar(v *pg_query.RangeVar, info *scopeInfo) []string {
	name := v.Relname
	if v.Alias != nil {
		name = v.Alias.Aliasname
	}
	if info.names[name] {
		c.fail(v.Location, "reference name %s appears twice in one FROM list", name)
	}
	isCTE := v.Schemaname == "" && c.isCTE(v.Relname)
	r := ref{name: name, table: v.Relname, line: v.Location}
	switch {
	case isCTE:
		info.derived = true
	case c.scoped[v.Relname]:
		r.scoped = true
	case c.instance[v.Relname]:
		r.known = true
	default:
		info.derived = true // a table the migrations do not show: do not resolve bare org_id against it
		c.unknownTable(v)
	}
	info.add(r)
	return []string{name}
}

// unknownTable reports a table that is neither created by the migrations as an
// organization-scoped table nor listed as an instance table: the check cannot
// tell whether it holds organization data.
func (c *checker) unknownTable(v *pg_query.RangeVar) {
	if c.strict {
		c.fail(v.Location, "table %s is neither organization-scoped (created by the migrations) nor listed in instance_tables.txt", v.Relname)
	}
}

// pin is one fact "this reference name has org_id = a parameter".
type pin struct {
	names map[string]bool
	links [][2]string
}

func emptyPin() pin { return pin{names: map[string]bool{}} }

func (p pin) merge(o pin) pin {
	for k := range o.names {
		p.names[k] = true
	}
	p.links = append(p.links, o.links...)
	return p
}

// pinsOf extracts what a condition guarantees for every row that satisfies it.
// An OR guarantees only what all its branches do; NOT, CASE, IS NULL, functions
// and everything else guarantee nothing.
func pinsOf(n *pg_query.Node, single string) pin {
	switch v := inner(n).Interface().(type) {
	case *pg_query.BoolExpr:
		switch v.Boolop {
		case pg_query.BoolExprType_AND_EXPR:
			p := emptyPin()
			for _, a := range v.Args {
				p = p.merge(pinsOf(a, single))
			}
			return p
		case pg_query.BoolExprType_OR_EXPR:
			var acc *pin
			for _, a := range v.Args {
				x := pinsOf(a, single)
				if acc == nil {
					acc = &pin{names: x.names}
					continue
				}
				for k := range acc.names {
					if !x.names[k] {
						delete(acc.names, k)
					}
				}
			}
			if acc == nil {
				return emptyPin()
			}
			return pin{names: acc.names}
		}
	case *pg_query.A_Expr:
		if v.Kind != pg_query.A_Expr_Kind_AEXPR_OP || len(v.Name) != 1 || v.Name[0].GetString_().GetSval() != "=" {
			return emptyPin()
		}
		lq, lok := orgIDColumn(v.Lexpr, single)
		rq, rok := orgIDColumn(v.Rexpr, single)
		p := emptyPin()
		switch {
		case lok && isParam(v.Rexpr):
			p.names[lq] = true
		case rok && isParam(v.Lexpr):
			p.names[rq] = true
		case lok && rok:
			p.links = append(p.links, [2]string{lq, rq})
		}
		return p
	}
	return emptyPin()
}

// orgIDColumn recognizes [qualifier.]org_id and returns the qualifier (single
// when none is written; "" when it cannot be resolved).
func orgIDColumn(n *pg_query.Node, single string) (string, bool) {
	cr := n.GetColumnRef()
	if cr == nil || len(cr.Fields) == 0 || len(cr.Fields) > 2 {
		return "", false
	}
	last := cr.Fields[len(cr.Fields)-1].GetString_()
	if last == nil || last.Sval != "org_id" {
		return "", false
	}
	if len(cr.Fields) == 1 {
		return single, single != ""
	}
	q := cr.Fields[0].GetString_()
	return q.GetSval(), q != nil
}

// isParam recognizes a parameter, optionally cast.
func isParam(n *pg_query.Node) bool {
	if n.GetParamRef() != nil {
		return true
	}
	if tc := n.GetTypeCast(); tc != nil {
		return tc.Arg.GetParamRef() != nil
	}
	return false
}

// blockFacts computes which reference names of a block are pinned.
func (c *checker) blockFacts(info *scopeInfo, where *pg_query.Node, outer outerPins) map[string]bool {
	scopedRefs, others := 0, 0
	single := ""
	for _, r := range info.refs {
		switch {
		case r.scoped:
			scopedRefs++
			single = r.name
		case !r.known:
			others++
		}
	}
	if scopedRefs != 1 || others > 0 {
		single = ""
	}
	type fact struct {
		p        pin
		eligible map[string]bool // nil: every name
	}
	var facts []fact
	if where != nil {
		facts = append(facts, fact{p: pinsOf(where, single)})
	}
	for _, j := range info.joins {
		if j.quals == nil || j.natural || j.using {
			continue
		}
		var elig []string
		switch j.kind {
		case pg_query.JoinType_JOIN_INNER:
			elig = append(slices.Clone(j.left), j.right...)
		case pg_query.JoinType_JOIN_LEFT:
			elig = j.right
		case pg_query.JoinType_JOIN_RIGHT:
			elig = j.left
		default:
			continue // FULL and anything else: an ON predicate restricts no side
		}
		set := map[string]bool{}
		for _, e := range elig {
			set[e] = true
		}
		facts = append(facts, fact{p: pinsOf(j.quals, single), eligible: set})
	}
	pinned := map[string]bool{}
	for _, f := range facts {
		for name := range f.p.names {
			if info.names[name] && (f.eligible == nil || f.eligible[name]) {
				pinned[name] = true
			}
		}
	}
	has := func(q string) bool { return pinned[q] || (!info.names[q] && outer[q]) }
	for changed := true; changed; {
		changed = false
		for _, f := range facts {
			for _, l := range f.p.links {
				for _, pair := range [][2]string{{l[0], l[1]}, {l[1], l[0]}} {
					from, to := pair[0], pair[1]
					if has(from) && !pinned[to] && info.names[to] && (f.eligible == nil || f.eligible[to]) {
						pinned[to], changed = true, true
					}
				}
			}
		}
	}
	return pinned
}

// require reports every scoped reference of the block that is not pinned.
func (c *checker) require(info *scopeInfo, pinned map[string]bool) {
	for _, r := range info.refs {
		if r.scoped && !pinned[r.name] {
			c.fail(r.line, "organization-scoped table %s is not restricted by org_id = <parameter> in WHERE or in an ON clause that restricts it", r.table)
		}
	}
}

// merge the outer pins and the pins of this block for nested queries.
func nested(outer outerPins, info *scopeInfo, pinned map[string]bool) outerPins {
	out := outerPins{}
	for k, v := range outer {
		if !info.names[k] {
			out[k] = v
		}
	}
	for k := range pinned {
		out[k] = true
	}
	return out
}

// checkSelect checks a SELECT, a set operation or a VALUES list.
func (c *checker) checkSelect(s *pg_query.SelectStmt, outer outerPins) {
	c.pushCTEs(s.WithClause)
	defer c.popCTEs()
	if s.Op != pg_query.SetOperation_SETOP_NONE && s.Op != pg_query.SetOperation_SET_OPERATION_UNDEFINED {
		c.checkQuery(&pg_query.Node{Node: &pg_query.Node_SelectStmt{SelectStmt: s.Larg}}, outer)
		c.checkQuery(&pg_query.Node{Node: &pg_query.Node_SelectStmt{SelectStmt: s.Rarg}}, outer)
		c.walkSublinks(nodeOf(s.SortClause), outer)
		return
	}
	if s.IntoClause != nil {
		c.fail(-1, "SELECT INTO creates a table")
	}
	info := &scopeInfo{names: map[string]bool{}}
	c.collectFrom(s.FromClause, info)
	pinned := c.blockFacts(info, s.WhereClause, outer)
	c.require(info, pinned)
	next := nested(outer, info, pinned)
	for _, part := range []*pg_query.Node{
		nodeOf(s.TargetList), s.WhereClause, nodeOf(s.GroupClause), s.HavingClause, nodeOf(s.WindowClause),
		nodeOf(s.SortClause), s.LimitOffset, s.LimitCount, nodeOf(s.LockingClause), nodeOf(s.DistinctClause),
	} {
		c.walkSublinks(part, next)
	}
	for _, v := range s.ValuesLists {
		c.walkSublinks(v, next)
	}
	for _, j := range info.joins {
		c.walkSublinks(j.quals, next)
	}
}

// modifyParts are the pieces of an UPDATE or a DELETE.
type modifyParts struct {
	with   *pg_query.WithClause
	target *pg_query.RangeVar
	from   []*pg_query.Node
	where  *pg_query.Node
	rest   []*pg_query.Node
}

func (c *checker) checkModify(p modifyParts, outer outerPins) {
	c.pushCTEs(p.with)
	defer c.popCTEs()
	info := &scopeInfo{names: map[string]bool{}}
	c.collectRangeVar(p.target, info)
	c.collectFrom(p.from, info)
	pinned := c.blockFacts(info, p.where, outer)
	c.require(info, pinned)
	next := nested(outer, info, pinned)
	c.walkSublinks(p.where, next)
	for _, r := range p.rest {
		c.walkSublinks(r, next)
	}
	for _, j := range info.joins {
		c.walkSublinks(j.quals, next)
	}
}

// walkSublinks finds the queries nested in an expression (subqueries, EXISTS,
// IN) and checks each with the pins of the enclosing block.
func (c *checker) walkSublinks(n *pg_query.Node, outer outerPins) {
	if n == nil {
		return
	}
	walk(n.ProtoReflect(), func(m protoreflect.Message) bool {
		switch v := m.Interface().(type) {
		case *pg_query.SubLink:
			c.walkSublinks(v.Testexpr, outer)
			c.checkQuery(v.Subselect, outer)
			return false
		case *pg_query.SelectStmt, *pg_query.InsertStmt, *pg_query.UpdateStmt, *pg_query.DeleteStmt, *pg_query.MergeStmt:
			// A statement outside a sub-link position: check it rather than skip it.
			if node := wrapStatement(v); node != nil {
				c.checkQuery(node, outer)
			}
			return false
		}
		return true
	})
}

func wrapStatement(v any) *pg_query.Node {
	switch s := v.(type) {
	case *pg_query.SelectStmt:
		return &pg_query.Node{Node: &pg_query.Node_SelectStmt{SelectStmt: s}}
	case *pg_query.InsertStmt:
		return &pg_query.Node{Node: &pg_query.Node_InsertStmt{InsertStmt: s}}
	case *pg_query.UpdateStmt:
		return &pg_query.Node{Node: &pg_query.Node_UpdateStmt{UpdateStmt: s}}
	case *pg_query.DeleteStmt:
		return &pg_query.Node{Node: &pg_query.Node_DeleteStmt{DeleteStmt: s}}
	case *pg_query.MergeStmt:
		return &pg_query.Node{Node: &pg_query.Node_MergeStmt{MergeStmt: s}}
	}
	return nil
}

// checkInsert checks an INSERT: org_id is set from a parameter, the source
// query is checked, and an upsert cannot touch another organization's row.
func (c *checker) checkInsert(s *pg_query.InsertStmt, outer outerPins) {
	c.pushCTEs(s.WithClause)
	defer c.popCTEs()
	table := s.Relation.Relname
	isScoped := c.scoped[table]
	if !isScoped && !c.instance[table] && !c.isCTE(table) {
		c.unknownTable(s.Relation)
	}
	if isScoped {
		c.checkInsertScoped(s)
	}
	if s.SelectStmt != nil {
		c.checkQuery(s.SelectStmt, outer)
	}
	if oc := s.OnConflictClause; oc != nil && oc.Action == pg_query.OnConflictAction_ONCONFLICT_UPDATE {
		c.checkUpsert(s, oc, isScoped, outer)
	}
	c.walkSublinks(nodeOf(s.ReturningList), outer)
}

func (c *checker) checkInsertScoped(s *pg_query.InsertStmt) {
	table := s.Relation.Relname
	idx := -1
	for i, col := range s.Cols {
		if rt := col.GetResTarget(); rt != nil && rt.Name == "org_id" {
			idx = i
		}
	}
	if len(s.Cols) == 0 || idx < 0 {
		c.fail(s.Relation.Location, "INSERT into organization-scoped table %s must list its columns and set org_id", table)
		return
	}
	sel := s.SelectStmt.GetSelectStmt()
	if sel == nil {
		c.fail(s.Relation.Location, "INSERT into organization-scoped table %s: DEFAULT VALUES does not set org_id", table)
		return
	}
	c.requireOrgItem(sel, idx, table, s.Relation.Location)
}

// requireOrgItem checks that the item at idx of every row source is a parameter
// (VALUES) or a parameter or an org_id column (SELECT).
func (c *checker) requireOrgItem(sel *pg_query.SelectStmt, idx int, table string, loc int32) {
	switch {
	case sel.Op != pg_query.SetOperation_SETOP_NONE && sel.Op != pg_query.SetOperation_SET_OPERATION_UNDEFINED:
		c.requireOrgItem(sel.Larg, idx, table, loc)
		c.requireOrgItem(sel.Rarg, idx, table, loc)
	case len(sel.ValuesLists) > 0:
		for _, row := range sel.ValuesLists {
			items := row.GetList().GetItems()
			if idx >= len(items) || !isParam(items[idx]) {
				c.fail(loc, "INSERT into organization-scoped table %s: the org_id value must be a parameter", table)
			}
		}
	default:
		if idx >= len(sel.TargetList) {
			c.fail(loc, "INSERT into organization-scoped table %s: the SELECT has no org_id item", table)
			return
		}
		item := sel.TargetList[idx].GetResTarget().GetVal()
		if _, isCol := orgIDColumn(item, "x"); !isParam(item) && !isCol {
			c.fail(loc, "INSERT into organization-scoped table %s: the org_id value must be a parameter or an org_id column", table)
		}
	}
}

// checkUpsert verifies ON CONFLICT ... DO UPDATE on a scoped table: the conflict
// target names org_id (so the existing row is the same organization's), or the
// DO UPDATE has its own org_id = <parameter> condition.
func (c *checker) checkUpsert(s *pg_query.InsertStmt, oc *pg_query.OnConflictClause, isScoped bool, outer outerPins) {
	c.walkSublinks(nodeOf(oc.TargetList), outer)
	c.walkSublinks(oc.WhereClause, outer)
	if !isScoped {
		return
	}
	targetsOrg := false
	if oc.Infer != nil && oc.Infer.Conname == "" {
		for _, e := range oc.Infer.IndexElems {
			if ie := e.GetIndexElem(); ie != nil && ie.Name == "org_id" {
				targetsOrg = true
			}
		}
	}
	if targetsOrg {
		return
	}
	name := s.Relation.Relname
	if s.Relation.Alias != nil {
		name = s.Relation.Alias.Aliasname
	}
	if oc.WhereClause != nil && pinsOf(oc.WhereClause, name).names[name] {
		return
	}
	c.fail(s.Relation.Location, "ON CONFLICT DO UPDATE on organization-scoped table %s must name org_id in the conflict target or add WHERE %s.org_id = <parameter>", s.Relation.Relname, name)
}
