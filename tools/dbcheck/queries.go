// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// forEachQuery runs fn for every query of every dialect.
func forEachQuery(w *world, fn func(q *query)) {
	for _, d := range w.dialects {
		for _, q := range d.queries {
			fn(q)
		}
	}
}

// statementKind is the verb of the statement: the first select, insert,
// update, delete or replace at parenthesis depth zero (a WITH list is skipped
// because its words are inside parentheses or are not those verbs).
func (q *query) statementKind() string {
	for _, t := range q.toks {
		if t.depth != 0 || t.kind != tokWord {
			continue
		}
		switch t.lower {
		case "select", "insert", "update", "delete", "replace", "values":
			return t.lower
		}
	}
	return ""
}

func (q *query) excepted(w *world, rule string) bool {
	a, ok := q.anno("exception")
	return ok && a.arg == rule && w.exc.has(rule, q.dialect, q.name)
}

func (q *query) finding(line int, rule, msg, fix string) Finding {
	if line == 0 {
		line = q.line
	}
	return Finding{File: q.file, Line: line, Rule: rule, Msg: fmt.Sprintf("query %s: %s", q.name, msg), Fix: fix}
}

// checkSelectStar is "query-select-star" (QY-2): "*" in a SELECT list or in
// RETURNING. count(*) is not a select item: its star is inside parentheses.
func checkSelectStar(w *world) []Finding {
	var out []Finding
	forEachQuery(w, func(q *query) {
		for i, t := range q.toks {
			if t.kind != tokWord || (t.lower != "select" && t.lower != "returning") {
				continue
			}
			for j := i + 1; j < len(q.toks); j++ {
				u := q.toks[j]
				if u.depth < t.depth {
					break
				}
				if u.depth != t.depth {
					continue
				}
				if t.lower == "select" && u.kind == tokWord && endsSelectList(u.lower) {
					break
				}
				if u.isOp("*") && isSelectItemStart(q.toks[j-1]) {
					out = append(out, q.finding(u.line, "QY-2", fmt.Sprintf("\"*\" in %s", strings.ToUpper(t.lower)), "list the columns explicitly"))
				}
			}
		}
	})
	return out
}

func endsSelectList(w string) bool {
	switch w {
	case "from", "where", "group", "order", "limit", "union", "intersect", "except", "having", "window", "returning":
		return true
	}
	return false
}

func isSelectItemStart(prev token) bool {
	switch {
	case prev.isPunct(",") || prev.isPunct("."):
		return true
	case prev.kind == tokWord:
		switch prev.lower {
		case "select", "distinct", "all", "returning":
			return true
		}
	}
	return false
}

var (
	jsonOpsPostgres = map[string]bool{"->": true, "->>": true, "#>": true, "#>>": true, "#-": true, "@?": true, "?": true, "?|": true, "?&": true}
	jsonOpsSQLite   = map[string]bool{"->": true, "->>": true}
	jsonFuncPrefix  = regexp.MustCompile(`^(json|jsonb)_`)
	jsonFuncExact   = map[string]bool{"to_json": true, "to_jsonb": true, "row_to_json": true, "array_to_json": true}
)

// checkJSONPath is "query-json-path" (JS-5). The lexer does not know types,
// so "@>" and "<@" are not flagged (CK-2).
func checkJSONPath(w *world) []Finding {
	var out []Finding
	forEachQuery(w, func(q *query) {
		if q.excepted(w, "JS-5") {
			return
		}
		ops := jsonOpsPostgres
		if q.dialect == dialectSQLite {
			ops = jsonOpsSQLite
		}
		for i, t := range q.toks {
			switch {
			case t.kind == tokOp && ops[t.text]:
				out = append(out, q.finding(t.line, "JS-5", fmt.Sprintf("JSON-path operator %q", t.text), "move the value into a real column (JS-2) or ask the owner for a JS-5 exception"))
			case t.kind == tokWord && i+1 < len(q.toks) && q.toks[i+1].isPunct("(") && isJSONFunction(q.dialect, t.lower):
				out = append(out, q.finding(t.line, "JS-5", fmt.Sprintf("JSON function %s()", t.lower), "move the value into a real column (JS-2) or ask the owner for a JS-5 exception"))
			}
		}
	})
	return out
}

func isJSONFunction(dialect, name string) bool {
	if jsonFuncPrefix.MatchString(name) {
		return true
	}
	if dialect == dialectSQLite {
		return name == "json" || name == "jsonb"
	}
	return jsonFuncExact[name]
}

// topLevel returns the tokens of the statement at parenthesis depth zero.
func (q *query) topLevel() []token {
	var out []token
	for _, t := range q.toks {
		if t.depth == 0 {
			out = append(out, t)
		}
	}
	return out
}

func hasTopWord(q *query, word string) bool {
	for _, t := range q.topLevel() {
		if t.isWord(word) {
			return true
		}
	}
	return false
}

// checkLimit is "query-limit" (QY-6): a :many or :batchmany SELECT needs a
// top-level LIMIT; UPDATE and DELETE need a top-level WHERE. (An INSERT ...
// RETURNING :many has no LIMIT to give and is bounded by its VALUES.)
func checkLimit(w *world) []Finding {
	var out []Finding
	forEachQuery(w, func(q *query) {
		kind := q.statementKind()
		if (q.cmd == ":many" || q.cmd == ":batchmany") && kind == "select" && !hasTopWord(q, "limit") {
			out = append(out, q.finding(0, "QY-6", "a :many query has no top-level LIMIT", "add LIMIT with a bounded parameter"))
		}
		if (kind == "update" || kind == "delete") && !hasTopWord(q, "where") {
			out = append(out, q.finding(0, "QY-6", strings.ToUpper(kind)+" without WHERE", "bound the statement with WHERE (for bulk work: WHERE id IN (SELECT id ... LIMIT $n))"))
		}
	})
	return out
}

// checkOffset is "query-offset" (QY-5).
func checkOffset(w *world) []Finding {
	var out []Finding
	forEachQuery(w, func(q *query) {
		if q.excepted(w, "QY-5") {
			return
		}
		for _, t := range q.toks {
			if t.isWord("offset") {
				out = append(out, q.finding(t.line, "QY-5", "OFFSET pagination", "use keyset pagination: ORDER BY <columns>, id with WHERE (<columns>, id) < ($1, $2)"))
			}
		}
	})
	return out
}

// checkKeyset is "query-keyset" (QY-5): the last ORDER BY term of a :many
// query is a unique key (id, <alias>.id or sequence_number).
func checkKeyset(w *world) []Finding {
	var out []Finding
	forEachQuery(w, func(q *query) {
		if q.cmd != ":many" && q.cmd != ":batchmany" {
			return
		}
		top := q.topLevel()
		by := -1
		for i := 0; i+1 < len(top); i++ {
			if top[i].isWord("order") && top[i+1].isWord("by") {
				by = i + 2
			}
		}
		if by < 0 {
			return
		}
		var terms [][]token
		var cur []token
		for _, t := range top[by:] {
			if t.kind == tokWord && (t.lower == "limit" || t.lower == "offset" || t.lower == "for") {
				break
			}
			if t.isPunct(",") {
				terms = append(terms, cur)
				cur = nil
				continue
			}
			cur = append(cur, t)
		}
		terms = append(terms, cur)
		last := terms[len(terms)-1]
		// drop ASC / DESC / NULLS FIRST|LAST
		for len(last) > 0 {
			t := last[len(last)-1]
			if t.kind == tokWord && (t.lower == "asc" || t.lower == "desc" || t.lower == "first" || t.lower == "last" || t.lower == "nulls") {
				last = last[:len(last)-1]
				continue
			}
			break
		}
		if !isUniqueKeyTerm(last) {
			line := q.line
			if len(last) > 0 {
				line = last[0].line
			}
			out = append(out, q.finding(line, "QY-5", "the last ORDER BY term is not a unique key (id, <alias>.id or sequence_number)", "end the ordering with id"))
		}
	})
	return out
}

func isUniqueKeyTerm(term []token) bool {
	if len(term) == 1 {
		n, ok := term[0].name()
		return ok && (n == "id" || n == "sequence_number")
	}
	if len(term) == 3 && term[1].isPunct(".") {
		n, ok := term[2].name()
		return ok && (n == "id" || n == "sequence_number")
	}
	return false
}

var (
	deleteAnnoRE    = regexp.MustCompile(`^(audit:[a-z][a-z0-9_.]*|retention:[a-z][a-z0-9_]*|bookkeeping:\S.*)$`)
	exceptionRuleRE = regexp.MustCompile(`^[A-Z]{2}-\d+$`)
)

// checkAnnotations is "query-annotations" (QY-9, DL-2, TI-2).
func checkAnnotations(w *world) []Finding {
	var out []Finding
	forEachQuery(w, func(q *query) {
		seen := map[string]int{}
		for _, a := range q.annos {
			if first, dup := seen[a.key]; dup && a.key != "exception" {
				out = append(out, q.finding(a.line, "QY-9", fmt.Sprintf("annotation orcon:%s repeated (first on line %d)", a.key, first), "keep one"))
				continue
			}
			seen[a.key] = a.line
			switch a.key {
			case "scope":
				if a.arg != "org" && a.arg != "instance" && a.arg != "engine" {
					out = append(out, q.finding(a.line, "TI-2", fmt.Sprintf("orcon:scope %q must be org, instance or engine", a.arg), "write -- orcon:scope org|instance|engine"))
				}
			case "hot":
				if a.arg != "" {
					out = append(out, q.finding(a.line, "QY-9", "orcon:hot takes no argument", "write -- orcon:hot"))
				}
			case "delete":
				if !deleteAnnoRE.MatchString(a.arg) {
					out = append(out, q.finding(a.line, "DL-2", fmt.Sprintf("orcon:delete %q must be audit:<action>, retention:<class> or bookkeeping:<reason>", a.arg), "write -- orcon:delete audit:<action>"))
				}
			case "include-deleted":
				if a.arg == "" {
					out = append(out, q.finding(a.line, "DL-4", "orcon:include-deleted needs a reason", "write -- orcon:include-deleted <reason>"))
				}
			case "exception":
				if !exceptionRuleRE.MatchString(a.arg) {
					out = append(out, q.finding(a.line, "QY-9", fmt.Sprintf("orcon:exception %q must name a rule id such as JS-5", a.arg), "write -- orcon:exception <rule-id>"))
				} else if !w.exc.has(a.arg, q.dialect, q.name) {
					out = append(out, q.finding(a.line, "QY-9", fmt.Sprintf("orcon:exception %s has no owner-approved line for %s:%s in policy/exceptions.txt", a.arg, q.dialect, q.name), "ask the owner to approve the exception in a pull request"))
				}
			default:
				out = append(out, q.finding(a.line, "QY-9", fmt.Sprintf("unknown annotation orcon:%s", a.key), "use scope, hot, delete, include-deleted or exception"))
			}
		}
		if _, ok := seen["scope"]; !ok {
			out = append(out, q.finding(0, "TI-2", "missing -- orcon:scope", "add -- orcon:scope org|instance|engine after the name line"))
		}
		if q.statementKind() == "delete" {
			if _, ok := seen["delete"]; !ok {
				out = append(out, q.finding(0, "DL-2", "DELETE without -- orcon:delete", "add -- orcon:delete audit:<action>, retention:<class> or bookkeeping:<reason>"))
			}
		}
	})
	return out
}

// namesSoftDeleteTable reports whether the query reads or writes a table of
// soft_delete_tables.txt. An INSERT INTO target is not a read of live rows and
// does not count.
func namesSoftDeleteTable(q *query, tables map[string]listEntry) []string {
	var hit []string
	for i, t := range q.toks {
		if t.kind != tokWord {
			continue
		}
		if _, ok := tables[t.lower]; !ok || i == 0 {
			continue
		}
		prev := q.toks[i-1]
		if prev.kind != tokWord {
			continue
		}
		switch prev.lower {
		case "from", "join", "update":
		case "into":
			if i >= 2 && q.toks[i-2].isWord("insert") {
				continue
			}
		default:
			continue
		}
		hit = append(hit, t.lower)
	}
	return hit
}

func hasDeletedAtIsNull(q *query) bool {
	for i := 0; i+2 < len(q.toks); i++ {
		if q.toks[i].isWord("deleted_at") && q.toks[i+1].isWord("is") && q.toks[i+2].isWord("null") {
			return true
		}
	}
	return false
}

// checkSoftDelete is "query-soft-delete" (DL-4).
func checkSoftDelete(w *world) []Finding {
	var out []Finding
	forEachQuery(w, func(q *query) {
		tables := namesSoftDeleteTable(q, w.policy.softDelete)
		if len(tables) == 0 || hasDeletedAtIsNull(q) {
			return
		}
		if a, ok := q.anno("include-deleted"); ok && a.arg != "" {
			return
		}
		out = append(out, q.finding(0, "DL-4", fmt.Sprintf("names soft-delete table %s without deleted_at IS NULL", tables[0]), "filter deleted_at IS NULL or add -- orcon:include-deleted <reason>"))
	})
	return out
}

// checkHotTag is "query-hot-tag" (QY-4).
func checkHotTag(w *world) []Finding {
	var out []Finding
	forEachQuery(w, func(q *query) {
		if _, tagged := q.anno("hot"); tagged {
			return
		}
		for _, p := range w.policy.hotPatterns {
			if p.re.MatchString(q.name) {
				out = append(out, q.finding(0, "QY-4", fmt.Sprintf("the name matches hot pattern %q but the query is not tagged", p.re), "add -- orcon:hot after the name line"))
				return
			}
		}
	})
	return out
}

// checkQueryParity is "query-parity" (QY-1): both dialects expose the same
// query names, except those in dialect_only_queries.txt.
func checkQueryParity(w *world) []Finding {
	var out []Finding
	names := map[string]map[string]*query{}
	for _, d := range w.dialects {
		names[d.name] = map[string]*query{}
		for _, q := range d.queries {
			if first, dup := names[d.name][q.name]; dup {
				out = append(out, q.finding(0, "QY-1", fmt.Sprintf("name defined twice in %s (first at %s:%d)", d.name, first.file, first.line), "give each query its own name"))
				continue
			}
			names[d.name][q.name] = q
		}
	}
	for _, d := range w.dialects {
		other := dialectSQLite
		if d.name == dialectSQLite {
			other = dialectPostgres
		}
		for _, q := range d.queries {
			if _, both := names[other][q.name]; both {
				continue
			}
			if _, only := w.policy.dialectOnly[d.name+":"+q.name]; only {
				continue
			}
			out = append(out, q.finding(0, "QY-1", fmt.Sprintf("exists in %s but not in %s", d.name, other), "add the query to the other dialect or list it in dialect_only_queries.txt"))
		}
	}
	keys := make([]string, 0, len(w.policy.dialectOnly))
	for k := range w.policy.dialectOnly {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		dialect, name, _ := strings.Cut(k, ":")
		other := dialectSQLite
		if dialect == dialectSQLite {
			other = dialectPostgres
		}
		e := w.policy.dialectOnly[k]
		path := w.policy.dir + "/" + fileDialectOnly
		_, here := names[dialect][name]
		_, there := names[other][name]
		switch {
		case !here:
			out = append(out, Finding{File: path, Line: e.line, Rule: "QY-1", Msg: fmt.Sprintf("%s lists %s, which does not exist", fileDialectOnly, k), Fix: "remove the stale line"})
		case there:
			out = append(out, Finding{File: path, Line: e.line, Rule: "QY-1", Msg: fmt.Sprintf("%s lists %s, which exists in both dialects", fileDialectOnly, k), Fix: "remove the line"})
		}
	}
	return out
}

// checkQueryNames is "query-names" (NM-3, NM-14).
func checkQueryNames(w *world) []Finding {
	var out []Finding
	forEachQuery(w, func(q *query) {
		if !startsWithVerb(q.name, w.policy.queryVerbs) {
			out = append(out, q.finding(0, "NM-14", "the name does not start with a verb of query_verbs.txt", "start with one of the listed verbs, then the object"))
		}
		for _, hit := range w.policy.abbreviationFindings(splitTokens(q.name), "") {
			out = append(out, q.finding(0, "NM-3", fmt.Sprintf("abbreviation %q in the name", hit.token), "spell it out: "+hit.expansion))
		}
	})
	return out
}

func startsWithVerb(name string, verbs map[string]listEntry) bool {
	for v := range verbs {
		if !strings.HasPrefix(name, v) {
			continue
		}
		if len(name) == len(v) {
			return true
		}
		if c := name[len(v)]; c >= 'A' && c <= 'Z' {
			return true
		}
	}
	return false
}
