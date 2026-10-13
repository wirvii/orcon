// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
)

// forbiddenTypes are the first words of column types TY-2 forbids, with the
// reason shown in the finding. They are SQL type names, not Orcon names.
var forbiddenTypes = map[string]string{
	"varchar": "varchar(n)", "nvarchar": "varchar(n)", "char": "char(n)", "nchar": "char(n)", "bpchar": "char(n)",
	"character": "character types", "date": "date", "time": "time", "timetz": "timetz", "interval": "interval",
	"money": "money", "real": "real", "float": "floating point", "float4": "real", "float8": "double precision",
	"double": "double precision", "serial": "serial", "bigserial": "serial", "smallserial": "serial",
	"serial2": "serial", "serial4": "serial", "serial8": "serial", "citext": "citext", "numeric": "numeric",
	"decimal": "numeric", "any": "ANY", "datetime": "datetime",
}

// engineFunctionSchemas are the only schemas that may hold functions (NM-13).
var engineFunctionSchemas = map[string]bool{"orcon_engine": true, "orcon_context": true}

// checkForbiddenDDL is "ddl-forbidden" (TY-2, TY-3, ID-1, TY-5, TI-10, NM-11).
func checkForbiddenDDL(w *world) []Finding {
	var out []Finding
	for _, d := range w.dialects {
		for _, m := range d.migrations {
			if m.toks == nil {
				continue
			}
			for _, s := range m.stmts {
				a := &ddlAnalysis{w: w, m: m, s: s}
				out = append(out, a.run()...)
			}
		}
	}
	return out
}

type ddlAnalysis struct {
	w   *world
	m   *migration
	s   stmt
	out []Finding
}

func (a *ddlAnalysis) add(line int, rule, msg, fix string) {
	a.out = append(a.out, Finding{File: a.m.path, Line: line, Rule: rule, Msg: msg, Fix: fix})
}

func (a *ddlAnalysis) run() []Finding {
	toks := a.s.toks
	a.scanGlobal()
	if len(toks) == 0 {
		return a.out
	}
	switch {
	case toks[0].isWord("create"):
		a.create()
	case toks[0].isWord("alter"):
		a.alter()
	}
	return a.out
}

// scanGlobal rejects tokens that are wrong anywhere in a migration statement.
func (a *ddlAnalysis) scanGlobal() {
	toks := a.s.toks
	for i, t := range toks {
		if t.kind != tokWord {
			continue
		}
		next := func() bool { return i+1 < len(toks) && toks[i+1].isPunct("(") }
		switch t.lower {
		case "gen_random_uuid", "uuid_generate_v4", "uuidv7":
			a.add(t.line, "ID-1", t.lower+"() creates an identity in the database", "generate ids in Go with internal/platform/id and pass them as parameters")
		case "now":
			if next() {
				a.add(t.line, "TY-5", "now() sets a time in the database", "pass the instant from the injected clock")
			}
		case "current_timestamp":
			a.add(t.line, "TY-5", "CURRENT_TIMESTAMP sets a time in the database", "pass the instant from the injected clock")
		case "autoincrement":
			a.add(t.line, "ID-1", "AUTOINCREMENT creates an identity in the database", "generate ids in Go with internal/platform/id")
		}
	}
}

// createInfo describes a CREATE statement: its object kind and the index of
// the first token after the kind keyword.
func createInfo(toks []token) (kind string, virtual bool, next int) {
	i := 1
	for i < len(toks) && toks[i].kind == tokWord {
		switch toks[i].lower {
		case "or", "replace", "temp", "temporary", "unique", "materialized", "recursive", "unlogged", "global", "local":
			i++
			continue
		case "virtual":
			virtual = true
			i++
			continue
		}
		break
	}
	if i >= len(toks) || toks[i].kind != tokWord {
		return "", virtual, i
	}
	return toks[i].lower, virtual, i + 1
}

// skipOptions moves past "IF NOT EXISTS" and "CONCURRENTLY".
func skipOptions(toks []token, i int) int {
	for i < len(toks) {
		switch {
		case toks[i].isWord("if") && i+2 < len(toks) && toks[i+1].isWord("not") && toks[i+2].isWord("exists"):
			i += 3
		case toks[i].isWord("concurrently"):
			i++
		default:
			return i
		}
	}
	return i
}

// objectName reads an optionally schema-qualified name at i, returning the
// schema ("" if none), the name and the index after it.
func objectName(toks []token, i int) (schema, name string, next int) {
	if i >= len(toks) {
		return "", "", i
	}
	n, ok := toks[i].name()
	if !ok {
		return "", "", i
	}
	if i+2 < len(toks) && toks[i+1].isPunct(".") {
		if n2, ok2 := toks[i+2].name(); ok2 {
			return n, n2, i + 3
		}
	}
	return "", n, i + 1
}

func (a *ddlAnalysis) create() {
	toks := a.s.toks
	kind, virtual, i := createInfo(toks)
	line := toks[0].line
	switch kind {
	case "view":
		a.add(line, "TI-10", "CREATE VIEW over data (views bypass row-level security in PostgreSQL 14)", "query the tables and select the columns in the query")
	case "domain":
		a.add(line, "TY-2", "CREATE DOMAIN is forbidden", "use the mapped base type with a CHECK on the column")
	case "type":
		if hasWordSeq(a.s.words(), "as", "enum") {
			a.add(line, "TY-2", "CREATE TYPE ... AS ENUM is forbidden", "use text with a named CHECK constraint (IN-3)")
		} else {
			a.add(line, "TY-2", "CREATE TYPE (composite and range types) is forbidden", "use columns or a child table")
		}
	case "trigger":
		i = skipOptions(toks, i)
		_, name, _ := objectName(toks, i)
		if !a.w.exc.has("TI-10", a.m.dialect, name) {
			a.add(line, "TI-10", fmt.Sprintf("CREATE TRIGGER %s is not in policy/exceptions.txt", name), "triggers need an owner-approved TI-10 exception")
		}
	case "function", "procedure":
		i = skipOptions(toks, i)
		schema, _, _ := objectName(toks, i)
		if !engineFunctionSchemas[schema] {
			a.add(line, "TI-10", "functions live only in schemas orcon_engine and orcon_context", "create the function as orcon_engine.<verb>_<object>")
		}
	case "index":
		i = skipOptions(toks, i)
		if i < len(toks) && toks[i].isWord("on") {
			a.add(line, "NM-11", "index without an explicit name", "write CREATE INDEX ix_<table>_<columns> ON ...")
		}
	case "table":
		if !virtual {
			a.createTable(skipOptions(toks, i))
		}
	}
}

// tableBody finds the parenthesised column list after the table name.
func tableBody(toks []token, i int) (open, closeIdx int, ok bool) {
	_, _, i = objectName(toks, i)
	if i >= len(toks) || !toks[i].isPunct("(") {
		return 0, 0, false
	}
	open = i
	for j := open + 1; j < len(toks); j++ {
		if toks[j].isPunct(")") && toks[j].depth == toks[open].depth {
			return open, j, true
		}
	}
	return 0, 0, false
}

// elements splits toks[open+1:closeIdx] at commas of the outermost level.
func elements(toks []token, open, closeIdx int) [][]token {
	var out [][]token
	var cur []token
	for _, t := range toks[open+1 : closeIdx] {
		if t.isPunct(",") && t.depth == toks[open].depth+1 {
			out = append(out, cur)
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

func (a *ddlAnalysis) createTable(i int) {
	toks := a.s.toks
	_, table, _ := objectName(toks, i)
	open, closeIdx, ok := tableBody(toks, i)
	if !ok {
		if a.m.dialect == dialectSQLite {
			a.add(toks[0].line, "TY-3", fmt.Sprintf("table %s is created without a column list and cannot be STRICT", table), "declare the columns and end the statement with STRICT")
		}
		return
	}
	if a.m.dialect == dialectSQLite {
		a.sqliteOptions(table, toks[closeIdx+1:], toks[0].line)
	}
	for _, el := range elements(toks, open, closeIdx) {
		if len(el) == 0 {
			continue
		}
		switch el[0].lower {
		case "constraint":
			continue
		case "primary", "unique", "foreign", "check", "exclude":
			a.add(el[0].line, "NM-11", fmt.Sprintf("unnamed %s constraint in table %s", strings.ToUpper(el[0].lower), table), "write CONSTRAINT <name> first")
			continue
		case "like":
			continue
		}
		a.column(table, el)
	}
}

// sqliteOptions checks the table options after the closing parenthesis.
func (a *ddlAnalysis) sqliteOptions(table string, opts []token, line int) {
	strict, withoutRowID := false, false
	words := make([]string, 0, len(opts))
	for _, t := range opts {
		if t.kind == tokWord {
			words = append(words, t.lower)
		}
	}
	for i, w := range words {
		if w == "strict" {
			strict = true
		}
		if w == "without" && i+1 < len(words) && words[i+1] == "rowid" {
			withoutRowID = true
		}
	}
	if !strict {
		a.add(line, "TY-3", fmt.Sprintf("SQLite table %s is not STRICT", table), "end the CREATE TABLE statement with STRICT")
	}
	if withoutRowID && !a.w.exc.has("TY-3", dialectSQLite, table) {
		a.add(line, "TY-3", fmt.Sprintf("WITHOUT ROWID on table %s needs benchmark evidence and an owner-approved exception", table), "remove WITHOUT ROWID or add a TY-3 exception")
	}
}

// constraintKeywords end the type of a column definition.
var constraintKeywords = map[string]bool{
	"constraint": true, "not": true, "null": true, "default": true, "primary": true, "unique": true,
	"check": true, "references": true, "collate": true, "generated": true, "as": true, "autoincrement": true,
}

// column checks one column definition: name, type, defaults and constraints.
func (a *ddlAnalysis) column(table string, el []token) {
	name, ok := el[0].name()
	if !ok {
		return
	}
	rest := el[1:]
	// type tokens run to the first constraint keyword
	var typeToks []token
	idx := 0
	for idx < len(rest) {
		if rest[idx].kind == tokWord && constraintKeywords[rest[idx].lower] {
			break
		}
		typeToks = append(typeToks, rest[idx])
		idx++
	}
	a.columnType(table, name, el[0].line, typeToks)

	named := false
	hasDefault := false
	for i := idx; i < len(rest); i++ {
		t := rest[i]
		if t.depth > el[0].depth && !t.isPunct("(") && !t.isPunct(")") {
			// tokens inside parentheses belong to an expression
			continue
		}
		if t.kind != tokWord {
			continue
		}
		switch t.lower {
		case "constraint":
			named = true
			i++ // the constraint name
		case "default":
			hasDefault = true
		case "primary", "unique", "check", "references":
			if !named {
				a.add(t.line, "NM-11", fmt.Sprintf("unnamed %s constraint on column %s.%s", strings.ToUpper(t.lower), table, name), "write CONSTRAINT <name> before it")
			}
			named = false
		}
	}
	if hasDefault && (name == "id" || strings.HasSuffix(name, "_at")) {
		rule := "TY-5"
		if name == "id" {
			rule = "ID-1"
		}
		a.add(el[0].line, rule, fmt.Sprintf("column %s.%s has a DEFAULT; identities and instants are supplied by the application", table, name), "remove the DEFAULT and pass the value")
	}
}

// columnType rejects the forbidden types of TY-2 and unexcepted jsonb (JS-4).
func (a *ddlAnalysis) columnType(table, column string, line int, typ []token) {
	if len(typ) == 0 {
		return
	}
	object := table + "." + column
	var words []string
	for _, t := range typ {
		if t.kind == tokWord {
			words = append(words, t.lower)
		}
		if t.isPunct("[") || t.isPunct("]") {
			if a.m.dialect == dialectPostgres {
				a.add(line, "TY-2", fmt.Sprintf("column %s is an array", object), "use a child table or a JSON document")
				return
			}
		}
	}
	if len(words) == 0 {
		return
	}
	first := words[0]
	switch {
	case first == "array" || containsWord(words, "array"):
		a.add(line, "TY-2", fmt.Sprintf("column %s is an array", object), "use a child table or a JSON document")
	case first == "timestamp":
		if !hasWordSeq(words, "with", "time", "zone") {
			a.add(line, "TY-2", fmt.Sprintf("column %s uses timestamp without time zone", object), "use timestamptz (PostgreSQL) or INTEGER unix microseconds (SQLite)")
		}
	case first == "jsonb":
		if !a.w.exc.has("JS-4", a.m.dialect, object) {
			a.add(line, "JS-4", fmt.Sprintf("column %s is jsonb without an approved exception", object), "use bytea (PostgreSQL) or BLOB (SQLite) for documents")
		}
	default:
		if reason, bad := forbiddenTypes[first]; bad {
			if (first == "real" || first == "numeric" || first == "decimal") && a.w.exc.has("TY-2", a.m.dialect, object) {
				return
			}
			a.add(line, "TY-2", fmt.Sprintf("column %s uses forbidden type %s", object, reason), "use the type TY-1 maps this to")
		}
	}
}

func (a *ddlAnalysis) alter() {
	toks := a.s.toks
	if len(toks) < 3 || !toks[1].isWord("table") {
		return
	}
	i := skipOptions(toks, 2)
	if i < len(toks) && toks[i].isWord("only") {
		i++
	}
	_, table, i := objectName(toks, i)
	for ; i < len(toks); i++ {
		t := toks[i]
		if !t.isWord("add") || t.depth != 0 {
			continue
		}
		j := i + 1
		if j < len(toks) && toks[j].isWord("column") {
			j++
		}
		j = skipOptions(toks, j)
		if j >= len(toks) {
			return
		}
		switch toks[j].lower {
		case "primary", "unique", "foreign", "check", "exclude":
			if toks[j].kind == tokWord {
				a.add(toks[j].line, "NM-11", fmt.Sprintf("unnamed %s constraint added to table %s", strings.ToUpper(toks[j].lower), table), "write ADD CONSTRAINT <name> ...")
			}
			continue
		case "constraint":
			continue
		}
		end := j
		for end < len(toks) && !(toks[end].isPunct(",") && toks[end].depth == 0) {
			end++
		}
		a.column(table, toks[j:end])
		i = end - 1
	}
}

// checkEnumValues is "ddl-enum-values" (NM-16, IN-3, CK-24): every string
// literal of an enumeration CHECK in a migration is UPPER_SNAKE.
func checkEnumValues(w *world) []Finding {
	if len(w.policy.enumWords) == 0 {
		return []Finding{{File: w.policy.dir + "/" + fileEnumWords, Line: 1, Rule: "CK-24", Msg: "enumeration words are missing or empty, so enumeration values cannot be checked", Fix: "restore policy/enumeration_words.txt"}}
	}
	words := make([]string, 0, len(w.policy.enumWords))
	for k := range w.policy.enumWords {
		words = append(words, k)
	}
	var out []Finding
	for _, d := range w.dialects {
		for _, m := range d.migrations {
			if m.toks == nil {
				continue
			}
			for _, s := range m.stmts {
				out = append(out, enumStatementFindings(w, m, s, words)...)
			}
		}
	}
	return out
}

// enumStatementFindings judges every CHECK (...) of one statement.
func enumStatementFindings(w *world, m *migration, s stmt, words []string) []Finding {
	table := statementTable(s)
	var out []Finding
	for k, t := range s.toks {
		if !t.isWord("check") || k+1 >= len(s.toks) || !s.toks[k+1].isPunct("(") {
			continue
		}
		inside, _, ok := tokenGroup(s.toks, k+1)
		if !ok {
			continue
		}
		name := ""
		if k >= 2 && s.toks[k-2].isWord("constraint") {
			name, _ = s.toks[k-1].name()
		}
		if name != "" && w.exc.has("NM-16", m.dialect, table+"."+name) {
			continue
		}
		et := toEnumToks(inside)
		for _, lit := range badEnumLiterals(et, words) {
			line := t.line
			object := name
			if object == "" {
				object = "(unnamed)"
			}
			out = append(out, Finding{File: m.path, Line: line, Rule: "NM-16",
				Msg: fmt.Sprintf("enumeration value %q in CHECK %s is not UPPER_SNAKE", lit.value, object),
				Fix: "'" + upperSnakeFix(lit.value) + "'"})
		}
	}
	return out
}

// statementTable returns the table a CREATE TABLE or ALTER TABLE statement is about.
func statementTable(s stmt) string {
	toks := s.toks
	if len(toks) == 0 {
		return ""
	}
	switch {
	case toks[0].isWord("create"):
		kind, _, i := createInfo(toks)
		if kind != "table" {
			return ""
		}
		_, name, _ := objectName(toks, skipOptions(toks, i))
		return name
	case toks[0].isWord("alter") && len(toks) > 2 && toks[1].isWord("table"):
		i := skipOptions(toks, 2)
		if i < len(toks) && toks[i].isWord("only") {
			i++
		}
		_, name, _ := objectName(toks, i)
		return name
	}
	return ""
}

// tokenGroup returns the tokens inside the parentheses that open at toks[i].
func tokenGroup(toks []token, i int) ([]token, int, bool) {
	for j := i + 1; j < len(toks); j++ {
		if toks[j].isPunct(")") && toks[j].depth == toks[i].depth {
			return toks[i+1 : j], j, true
		}
	}
	return nil, 0, false
}
