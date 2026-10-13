// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// LintSQLite checks the schema of a SQLite database against the rulebook by
// reading its catalog (sqlite_schema, pragma_table_list, pragma_table_xinfo,
// pragma_foreign_key_list, pragma_index_list and pragma_index_xinfo). The
// migrations must be applied first. Constraint names are read from the
// CONSTRAINT <name> text of sqlite_schema.sql, which the pragmas do not expose.
// A catalog that cannot be read is a finding, not a pass.
func LintSQLite(ctx context.Context, db *sql.DB, p *Policy) []Finding {
	m, err := readSQLiteModel(ctx, db, p)
	if err != nil {
		return []Finding{{Rule: "CK-4", Object: "catalog", Message: "cannot read the SQLite catalog: " + err.Error()}}
	}
	return lintModel(m, p)
}

type sqliteObject struct {
	kind, name, table, sqlText string
}

func readSQLiteModel(ctx context.Context, db *sql.DB, p *Policy) (*model, error) {
	objects, err := sqliteObjects(ctx, db)
	if err != nil {
		return nil, err
	}
	kinds, err := sqliteTableKinds(ctx, db)
	if err != nil {
		return nil, err
	}
	m := &model{dialect: DialectSQLite}
	byName := map[string]*table{}
	for _, o := range objects {
		switch o.kind {
		case "table":
			switch kinds[o.name].kind {
			case "shadow":
				continue
			case "virtual":
				t := &table{name: o.name, virtual: true}
				m.tables = append(m.tables, t)
				byName[o.name] = t
				continue
			}
			t, err := readSQLiteTable(ctx, db, o, kinds[o.name].strict)
			if err != nil {
				return nil, fmt.Errorf("table %s: %w", o.name, err)
			}
			m.tables = append(m.tables, t)
			byName[o.name] = t
		case "view":
			m.extra = append(m.extra, Finding{Rule: "TI-10", Object: "view " + o.name, Message: "views over data are forbidden"})
		case "trigger":
			if !p.excepted("TI-10", DialectSQLite, o.name) {
				m.extra = append(m.extra, Finding{Rule: "TI-10", Object: "trigger " + o.name, Message: "triggers need an owner-approved TI-10 exception in " + fileExceptions})
			}
		}
	}
	for _, o := range objects {
		if o.kind != "index" || o.sqlText == "" || byName[o.table] == nil || byName[o.table].virtual {
			continue
		}
		if err := addSQLiteIndex(ctx, db, byName[o.table], o); err != nil {
			return nil, fmt.Errorf("index %s: %w", o.name, err)
		}
	}
	for _, t := range m.tables {
		resolveForeignKeyTargets(t, byName)
		if t.virtual {
			continue
		}
		if !t.strict {
			m.extra = append(m.extra, Finding{Rule: "TY-3", Object: "table " + t.name, Message: "every SQLite table is STRICT"})
		}
		if t.withoutRowID && !p.excepted("TY-3", DialectSQLite, t.name) {
			m.extra = append(m.extra, Finding{Rule: "TY-3", Object: "table " + t.name, Message: "WITHOUT ROWID needs benchmark evidence and an owner-approved exception"})
		}
	}
	return m, nil
}

func sqliteObjects(ctx context.Context, db *sql.DB) ([]sqliteObject, error) {
	rows, err := db.QueryContext(ctx, `SELECT type, name, tbl_name, coalesce(sql, '') FROM sqlite_schema WHERE name NOT LIKE 'sqlite\_%' ESCAPE '\' ORDER BY type, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only query
	var out []sqliteObject
	for rows.Next() {
		var o sqliteObject
		if err := rows.Scan(&o.kind, &o.name, &o.table, &o.sqlText); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

type sqliteTableKind struct {
	kind   string
	strict bool
}

func sqliteTableKinds(ctx context.Context, db *sql.DB) (map[string]sqliteTableKind, error) {
	rows, err := db.QueryContext(ctx, `SELECT name, type, strict FROM pragma_table_list WHERE schema = 'main'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only query
	out := map[string]sqliteTableKind{}
	for rows.Next() {
		var name, kind string
		var strict int
		if err := rows.Scan(&name, &kind, &strict); err != nil {
			return nil, err
		}
		out[name] = sqliteTableKind{kind: kind, strict: strict == 1}
	}
	return out, rows.Err()
}

func readSQLiteTable(ctx context.Context, db *sql.DB, o sqliteObject, strict bool) (*table, error) {
	t := &table{name: o.name, strict: strict}
	cols, err := db.QueryContext(ctx, `SELECT name, type, "notnull", pk FROM pragma_table_xinfo(?) WHERE hidden IN (0, 2, 3) ORDER BY cid`, o.name)
	if err != nil {
		return nil, err
	}
	defer cols.Close() //nolint:errcheck // read-only query
	type pkCol struct {
		pos  int
		name string
	}
	var pks []pkCol
	for cols.Next() {
		var c column
		var notNull, pk int
		if err := cols.Scan(&c.name, &c.typ, &notNull, &pk); err != nil {
			return nil, err
		}
		c.typ = strings.ToLower(c.typ)
		c.notNull = notNull == 1
		t.columns = append(t.columns, &c)
		if pk > 0 {
			pks = append(pks, pkCol{pk, c.name})
		}
	}
	if err := cols.Err(); err != nil {
		return nil, err
	}
	sort.Slice(pks, func(i, j int) bool { return pks[i].pos < pks[j].pos })
	for _, k := range pks {
		t.pkCols = append(t.pkCols, k.name)
	}
	if err := addSQLiteForeignKeys(ctx, db, t); err != nil {
		return nil, err
	}
	parsed, err := parseCreateTable(o.sqlText)
	if err != nil {
		return nil, err
	}
	applyParsedConstraints(t, parsed)
	return t, nil
}

func addSQLiteForeignKeys(ctx context.Context, db *sql.DB, t *table) error {
	rows, err := db.QueryContext(ctx, `SELECT id, "table", "from", coalesce("to", ''), upper(on_delete) FROM pragma_foreign_key_list(?) ORDER BY id, seq`, t.name)
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck // read-only query
	byID := map[int]*foreignKey{}
	var order []int
	for rows.Next() {
		var id int
		var ref, from, to, onDelete string
		if err := rows.Scan(&id, &ref, &from, &to, &onDelete); err != nil {
			return err
		}
		fk := byID[id]
		if fk == nil {
			fk = &foreignKey{refTable: strings.ToLower(ref), onDelete: strings.ToLower(onDelete)}
			byID[id] = fk
			order = append(order, id)
		}
		fk.cols = append(fk.cols, from)
		if to != "" {
			fk.refCols = append(fk.refCols, to)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range order {
		t.fks = append(t.fks, byID[id])
	}
	return nil
}

// resolveForeignKeyTargets fills the referenced columns of a foreign key that
// names only the table (it then refers to the primary key).
func resolveForeignKeyTargets(t *table, byName map[string]*table) {
	for _, fk := range t.fks {
		if len(fk.refCols) == 0 {
			if ref := byName[fk.refTable]; ref != nil {
				fk.refCols = append([]string(nil), ref.pkCols...)
			}
		}
	}
}

func applyParsedConstraints(t *table, parsed *parsedTable) {
	t.withoutRowID = parsed.withoutRowID
	t.unnamed = append(t.unnamed, parsed.unnamed...)
	for _, c := range parsed.constraints {
		switch c.kind {
		case "primary":
			t.pkName = c.name
			t.indexes = append(t.indexes, &index{name: c.name, cols: c.cols, unique: true, kind: "primary"})
		case "unique":
			t.indexes = append(t.indexes, &index{name: c.name, cols: c.cols, unique: true, kind: "unique"})
		case "check":
			t.checks = append(t.checks, &check{name: c.name, toks: c.expr})
		case "foreign":
			for _, fk := range t.fks {
				if fk.name == "" && equalStrings(fk.cols, c.cols) && strings.EqualFold(fk.refTable, c.refTable) {
					fk.name = c.name
				}
			}
		}
	}
	// A primary key declared without an index entry (an unnamed or column-level one) still counts for support.
	hasPK := false
	for _, ix := range t.indexes {
		if ix.kind == "primary" {
			hasPK = true
		}
	}
	if !hasPK && len(t.pkCols) > 0 {
		t.indexes = append(t.indexes, &index{cols: t.pkCols, unique: true, kind: "primary"})
	}
}

func addSQLiteIndex(ctx context.Context, db *sql.DB, t *table, o sqliteObject) error {
	var unique int
	if err := db.QueryRowContext(ctx, `SELECT "unique" FROM pragma_index_list(?) WHERE name = ?`, t.name, o.name).Scan(&unique); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT coalesce(name, '') FROM pragma_index_xinfo(?) WHERE key = 1 ORDER BY seqno`, o.name)
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck // read-only query
	ix := &index{name: o.name, unique: unique == 1, kind: "index"}
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return err
		}
		if col == "" {
			ix.hasExpr = true
			continue
		}
		ix.cols = append(ix.cols, col)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	ix.predicate = indexPredicate(o.sqlText)
	t.indexes = append(t.indexes, ix)
	return nil
}

// indexPredicate returns the normalized WHERE text of CREATE INDEX text.
func indexPredicate(sqlText string) string {
	toks, err := tokenize(sqlText)
	if err != nil {
		return ""
	}
	for i, t := range toks {
		if t.depth == 0 && t.isWord("where") {
			return normalizePredicate(toks[i+1:])
		}
	}
	return ""
}

// normalizePredicate lowercases the tokens and drops spaces and parentheses so
// the predicates of both engines compare equal.
func normalizePredicate(toks []sqlToken) string {
	var b strings.Builder
	for _, t := range toks {
		if t.isPunct("(") || t.isPunct(")") {
			continue
		}
		b.WriteString(strings.ToLower(t.text))
	}
	return b.String()
}

// parsedConstraint is a constraint read from CREATE TABLE text.
type parsedConstraint struct {
	name     string
	kind     string // primary, unique, check, foreign
	cols     []string
	expr     []sqlToken
	refTable string
	refCols  []string
}

type parsedTable struct {
	constraints  []parsedConstraint
	unnamed      []string
	withoutRowID bool
}

// parseCreateTable reads the constraints of CREATE TABLE text, table-level and
// column-level, with their CONSTRAINT names.
func parseCreateTable(sqlText string) (*parsedTable, error) {
	toks, err := tokenize(sqlText)
	if err != nil {
		return nil, err
	}
	out := &parsedTable{}
	open := -1
	for i, t := range toks {
		if t.isPunct("(") && t.depth == 0 {
			open = i
			break
		}
	}
	if open < 0 {
		return out, nil
	}
	inside, closeIdx, ok := parenGroup(toks, open)
	if !ok {
		return nil, errors.New("unbalanced CREATE TABLE text")
	}
	opts := words(toks[closeIdx+1:])
	for i := 0; i+1 < len(opts); i++ {
		if opts[i] == "without" && opts[i+1] == "rowid" {
			out.withoutRowID = true
		}
	}
	for _, el := range splitTopLevel(inside, toks[open].depth+1) {
		if len(el) == 0 {
			continue
		}
		switch el[0].lower {
		case "constraint", "primary", "unique", "check", "foreign":
			out.addElementConstraint(el)
		default:
			out.addColumnConstraints(el)
		}
	}
	return out, nil
}

func (p *parsedTable) addElementConstraint(el []sqlToken) {
	name, i := "", 0
	if el[0].isWord("constraint") && len(el) > 1 {
		name, _ = el[1].ident()
		i = 2
	}
	if i >= len(el) {
		return
	}
	c := parsedConstraint{name: name}
	switch el[i].lower {
	case "primary":
		c.kind = "primary"
		c.cols, _ = identList(el, i+2)
	case "unique":
		c.kind = "unique"
		c.cols, _ = identList(el, i+1)
	case "check":
		c.kind = "check"
		c.expr, _, _ = parenGroup(el, i+1)
	case "foreign":
		c.kind = "foreign"
		var end int
		c.cols, end = identList(el, i+2)
		c.refTable, c.refCols = readReference(el, end+1)
	default:
		return
	}
	p.add(c, "table-level "+strings.ToUpper(el[i].lower)+" "+strings.Join(c.cols, ", "))
}

func (p *parsedTable) add(c parsedConstraint, description string) {
	p.constraints = append(p.constraints, c)
	if c.name == "" {
		p.unnamed = append(p.unnamed, description)
	}
}

// readReference reads "REFERENCES table (cols)" starting at or after toks[i].
func readReference(toks []sqlToken, i int) (string, []string) {
	for ; i < len(toks); i++ {
		if !toks[i].isWord("references") || i+1 >= len(toks) {
			continue
		}
		table, _ := toks[i+1].ident()
		var cols []string
		if i+2 < len(toks) && toks[i+2].isPunct("(") {
			cols, _ = identList(toks, i+2)
		}
		return table, cols
	}
	return "", nil
}

// addColumnConstraints reads the inline constraints of one column definition.
func (p *parsedTable) addColumnConstraints(el []sqlToken) {
	col, ok := el[0].ident()
	if !ok {
		return
	}
	base := el[0].depth
	name := ""
	for i := 1; i < len(el); i++ {
		t := el[i]
		if t.depth != base || t.kind != 'w' {
			continue
		}
		switch t.lower {
		case "constraint":
			if i+1 < len(el) {
				name, _ = el[i+1].ident()
				i++
			}
		case "primary":
			p.add(parsedConstraint{name: name, kind: "primary", cols: []string{col}}, "inline PRIMARY KEY on "+col)
			name = ""
		case "unique":
			p.add(parsedConstraint{name: name, kind: "unique", cols: []string{col}}, "inline UNIQUE on "+col)
			name = ""
		case "check":
			expr, _, _ := parenGroup(el, i+1)
			p.add(parsedConstraint{name: name, kind: "check", expr: expr}, "inline CHECK on "+col)
			name = ""
		case "references":
			refTable, refCols := readReference(el, i)
			p.add(parsedConstraint{name: name, kind: "foreign", cols: []string{col}, refTable: refTable, refCols: refCols}, "inline REFERENCES on "+col)
			name = ""
		}
	}
}
