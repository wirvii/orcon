// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// embeddedPolicy holds the declaration files of the policy directory.
//
//go:embed policy/*.txt
var embeddedPolicy embed.FS

// Names of the policy files the schema checks read (spec 4.4.7). Every list
// lives in those files; none is duplicated in code.
const (
	fileInstanceTables  = "instance_tables.txt"
	fileAppendOnly      = "append_only_tables.txt"
	fileSoftDelete      = "soft_delete_tables.txt"
	fileExternalIDs     = "external_ids.txt"
	fileSingular        = "singular_exceptions.txt"
	fileDenylist        = "abbreviation_denylist.txt"
	fileAllowedAcronyms = "allowed_acronyms.txt"
	fileReservedWords   = "reserved_words.txt"
	fileEnumWords       = "enumeration_words.txt"
	fileExceptions      = "exceptions.txt"
)

// Dialect names.
const (
	// DialectSQLite names the SQLite dialect.
	DialectSQLite = "sqlite"
	// DialectPostgres names the PostgreSQL dialect.
	DialectPostgres = "postgres"
)

// Finding is one rule violation found in a database.
type Finding struct {
	// Rule is the rulebook rule, for example "NM-3" or "TI-4".
	Rule string
	// Object names what the finding is about, for example "table flow".
	Object string
	// Message says what is wrong and how to fix it.
	Message string
}

func (f Finding) String() string {
	return f.Rule + " " + f.Object + ": " + f.Message
}

// Policy holds the declarations of internal/adapter/storetest/dbrules/policy
// that the schema checks need.
type Policy struct {
	instanceTables map[string]bool
	appendOnly     map[string]bool
	softDelete     map[string]bool
	externalIDs    map[string]bool
	singular       map[string]bool
	allowed        map[string]bool
	denied         map[string]string // token to full word(s)
	reserved       map[string]bool
	enumWords      []string
	exceptions     []policyException
}

type policyException struct {
	rule    string
	dialect string
	object  string
}

// LoadPolicy reads the embedded policy files. A missing or malformed file is
// an error: a check never runs without the list it needs.
func LoadPolicy() (*Policy, error) {
	sub, err := fs.Sub(embeddedPolicy, "policy")
	if err != nil {
		return nil, fmt.Errorf("dbrules: embedded policy: %w", err)
	}
	return LoadPolicyFS(sub, time.Now())
}

var (
	commentedLineRE = regexp.MustCompile(`^(\S+)\s+#\s+(\S.*)$`)
	acronymTokenRE  = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	externalIDRE    = regexp.MustCompile(`^(\*|[a-z][a-z0-9_]*)\.[a-z][a-z0-9_]*$`)
	snakeCaseRE     = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	exceptionRE     = regexp.MustCompile(`^([A-Z]{2}-\d+) {2,}(sqlite|postgres):(\S+) {2,}(\S.*?) {2,}(\d{4}-\d{2}-\d{2}) {2,}approved-by:(\S+)$`)
)

// LoadPolicyFS reads the policy files from fsys. The time now decides which
// exceptions are past their date.
func LoadPolicyFS(fsys fs.FS, now time.Time) (*Policy, error) {
	p := &Policy{}
	var errs []error
	load := func(name string, pattern *regexp.Regexp) map[string]string {
		m, err := readCommented(fsys, name, pattern)
		if err != nil {
			errs = append(errs, err)
		}
		return m
	}
	p.instanceTables = keysOf(load(fileInstanceTables, nil))
	p.appendOnly = keysOf(load(fileAppendOnly, nil))
	p.softDelete = keysOf(load(fileSoftDelete, snakeCaseRE))
	p.externalIDs = keysOf(load(fileExternalIDs, externalIDRE))
	p.singular = keysOf(load(fileSingular, snakeCaseRE))
	p.allowed = keysOf(load(fileAllowedAcronyms, acronymTokenRE))
	p.denied = load(fileDenylist, acronymTokenRE)
	p.enumWords = sortedKeys(load(fileEnumWords, snakeCaseRE))

	reserved, err := readLines(fsys, fileReservedWords)
	if err != nil {
		errs = append(errs, err)
	}
	p.reserved = map[string]bool{}
	for _, l := range reserved {
		if !snakeCaseRE.MatchString(l.text) {
			errs = append(errs, fmt.Errorf("dbrules: %s line %d: %q is not one lowercase token", fileReservedWords, l.line, l.text))
			continue
		}
		p.reserved[l.text] = true
	}
	for tok := range p.allowed {
		if _, both := p.denied[tok]; both {
			errs = append(errs, fmt.Errorf("dbrules: token %q is in both %s and %s", tok, fileAllowedAcronyms, fileDenylist))
		}
	}
	exc, err := readExceptions(fsys, now)
	if err != nil {
		errs = append(errs, err)
	}
	p.exceptions = exc
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return p, nil
}

// IsInstanceTable reports whether the table is listed in instance_tables.txt:
// it belongs to the instance and carries no org_id (orcon-db TI-1).
func (p *Policy) IsInstanceTable(table string) bool { return p.instanceTables[table] }

type policyLine struct {
	text string
	line int
}

func readLines(fsys fs.FS, name string) ([]policyLine, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("dbrules: policy file %s: %w", name, err)
	}
	defer f.Close() //nolint:errcheck // read-only file
	var out []policyLine
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		out = append(out, policyLine{text: l, line: n})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("dbrules: policy file %s: %w", name, err)
	}
	return out, nil
}

// readCommented reads "<value>  # reason" lines and returns value to reason.
func readCommented(fsys fs.FS, name string, pattern *regexp.Regexp) (map[string]string, error) {
	lines, err := readLines(fsys, name)
	if err != nil {
		return map[string]string{}, err
	}
	out := make(map[string]string, len(lines))
	var errs []error
	for _, l := range lines {
		m := commentedLineRE.FindStringSubmatch(l.text)
		if m == nil {
			errs = append(errs, fmt.Errorf("dbrules: %s line %d: %q is not \"<value>  # <reason>\"", name, l.line, l.text))
			continue
		}
		if pattern != nil && !pattern.MatchString(m[1]) {
			errs = append(errs, fmt.Errorf("dbrules: %s line %d: %q does not match %s", name, l.line, m[1], pattern))
			continue
		}
		out[m[1]] = m[2]
	}
	return out, errors.Join(errs...)
}

func readExceptions(fsys fs.FS, now time.Time) ([]policyException, error) {
	lines, err := readLines(fsys, fileExceptions)
	if err != nil {
		return nil, err
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var out []policyException
	var errs []error
	for _, l := range lines {
		m := exceptionRE.FindStringSubmatch(l.text)
		if m == nil {
			errs = append(errs, fmt.Errorf("dbrules: %s line %d: malformed exception (needs five fields and approved-by:)", fileExceptions, l.line))
			continue
		}
		until, perr := time.Parse("2006-01-02", m[5])
		if perr != nil || until.Before(today) {
			errs = append(errs, fmt.Errorf("dbrules: %s line %d: exception for %s %s:%s has an invalid or past date %s", fileExceptions, l.line, m[1], m[2], m[3], m[5]))
			continue
		}
		out = append(out, policyException{rule: m[1], dialect: m[2], object: m[3]})
	}
	return out, errors.Join(errs...)
}

func keysOf(m map[string]string) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (p *Policy) excepted(rule, dialect, object string) bool {
	for _, e := range p.exceptions {
		if e.rule == rule && e.dialect == dialect && e.object == object {
			return true
		}
	}
	return false
}

func (p *Policy) isExternalID(table, column string) bool {
	return p.externalIDs[table+"."+column] || p.externalIDs["*."+column]
}

// splitWords splits an identifier into the tokens NM-3 judges: snake_case on
// "_", lowercased.
func splitWords(name string) []string {
	var out []string
	for _, t := range strings.Split(name, "_") {
		if t != "" {
			out = append(out, strings.ToLower(t))
		}
	}
	return out
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// deniedTokens applies the NM-3 decision: a token is accepted if it is an
// allowed acronym, only digits, or the required prefix in first position;
// otherwise a token on the denied list is a finding. It returns the denied
// tokens with their full words.
func (p *Policy) deniedTokens(name, prefix string) [][2]string {
	var out [][2]string
	for i, t := range splitWords(name) {
		switch {
		case p.allowed[t], allDigits(t), i == 0 && prefix != "" && t == prefix:
		default:
			if full, bad := p.denied[t]; bad {
				out = append(out, [2]string{t, full})
			}
		}
	}
	return out
}

// column, table, index and the other types below are the engine-neutral model
// the rules run on. LintSQLite and LintPostgres fill it from the catalog.

type column struct {
	name    string
	typ     string // lowercase declared type (SQLite) or format_type (PostgreSQL)
	notNull bool
}

type foreignKey struct {
	name     string
	cols     []string
	refTable string
	refCols  []string
	onDelete string // lowercase: "no action", "restrict", "cascade", "set null", "set default"
}

type index struct {
	name      string
	cols      []string
	unique    bool
	predicate string // normalized WHERE text of a partial index
	hasExpr   bool
	kind      string // "primary", "unique" (constraint) or "index"
}

type check struct {
	name string
	toks []sqlToken
}

type table struct {
	name         string
	virtual      bool
	strict       bool
	withoutRowID bool
	columns      []*column
	pkName       string
	pkCols       []string
	fks          []*foreignKey
	indexes      []*index
	checks       []*check
	// unnamed lists constraints the catalog shows without an explicit name (SQLite reads them from the CREATE text).
	unnamed []string
}

type model struct {
	dialect string
	tables  []*table
	// extra holds findings of checks that belong to one engine (RLS, roles, views, triggers).
	extra []Finding
	// otherNames lists identifiers of other objects: kind, name.
	otherNames [][2]string
}

func (t *table) column(name string) *column {
	for _, c := range t.columns {
		if c.name == name {
			return c
		}
	}
	return nil
}

func (t *table) hasCheckOn(col string) bool {
	for _, c := range t.checks {
		for _, tok := range c.toks {
			if n, ok := tok.ident(); ok && n == col {
				return true
			}
		}
	}
	return false
}

func equalStrings(a, b []string) bool { return slices.Equal(a, b) }

func hasPrefixStrings(whole, prefix []string) bool {
	return len(prefix) <= len(whole) && equalStrings(whole[:len(prefix)], prefix)
}

type collector struct {
	findings []Finding
}

func (c *collector) addf(rule, object, format string, args ...any) {
	c.findings = append(c.findings, Finding{Rule: rule, Object: object, Message: fmt.Sprintf(format, args...)})
}

// lintModel runs the engine-neutral rules of spec 4.4.2 on the model.
func lintModel(m *model, p *Policy) []Finding {
	c := &collector{}
	byName := make(map[string]*table, len(m.tables))
	for _, t := range m.tables {
		byName[t.name] = t
	}
	for _, t := range m.tables {
		lintTableNames(c, t, p)
		if t.virtual {
			continue
		}
		lintColumns(c, m, t, p)
		lintTenancy(c, t, byName, p)
		lintIntegrity(c, t, p)
		lintEnumValues(c, m.dialect, t, p)
		lintActors(c, t, p)
		lintSoftDelete(c, t, p)
		lintDocuments(c, m, t, p)
		lintRedundantIndexes(c, t)
	}
	lintExternalIDWildcards(c, m, p)
	for _, o := range m.otherNames {
		checkIdentifier(c, o[0]+" "+o[1], o[1], "", p)
	}
	out := append(append([]Finding(nil), c.findings...), m.extra...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		if out[i].Object != out[j].Object {
			return out[i].Object < out[j].Object
		}
		return out[i].Message < out[j].Message
	})
	return out
}

// checkIdentifier applies NM-1 (shape and length), NM-12 (reserved words) and
// NM-3 (abbreviations) to one identifier. The prefix is the constraint or
// index prefix accepted in first position.
func checkIdentifier(c *collector, object, name, prefix string, p *Policy) {
	if !snakeCaseRE.MatchString(name) || len(name) > 63 {
		c.addf("NM-1", object, "name %q is not snake_case of at most 63 bytes", name)
	}
	if p.reserved[name] {
		c.addf("NM-12", object, "name %q is a PostgreSQL reserved word or SQLite keyword", name)
	}
	for _, d := range p.deniedTokens(name, prefix) {
		c.addf("NM-3", object, "abbreviation %q in %q (fix: %s)", d[0], name, d[1])
	}
}

func lintTableNames(c *collector, t *table, p *Policy) {
	obj := "table " + t.name
	checkIdentifier(c, obj, t.name, "", p)
	if strings.HasSuffix(t.name, "s") && !p.singular[t.name] {
		c.addf("NM-2", obj, "table names are singular; %q ends in s and is not in %s", t.name, fileSingular)
	}
	if t.virtual {
		return
	}
	for _, col := range t.columns {
		checkIdentifier(c, "column "+t.name+"."+col.name, col.name, "", p)
	}
	namePrefix := func(prefix string) string { return prefix + "_" + t.name }
	if t.pkName != "" && t.pkName != namePrefix("pk") {
		c.addf("NM-11", "constraint "+t.pkName, "the primary key of %s is named pk_%s", t.name, t.name)
	}
	if t.pkName != "" {
		checkIdentifier(c, "constraint "+t.pkName, t.pkName, "pk", p)
	}
	for _, fk := range t.fks {
		lintForeignKeyName(c, t, fk, p)
	}
	for _, ck := range t.checks {
		if ck.name == "" {
			continue
		}
		checkIdentifier(c, "constraint "+ck.name, ck.name, "ck", p)
		if !strings.HasPrefix(ck.name, "ck_"+t.name+"_") {
			c.addf("NM-11", "constraint "+ck.name, "check constraints of %s are named ck_%s_<rule>", t.name, t.name)
		}
	}
	for _, ix := range t.indexes {
		if ix.kind == "primary" {
			continue
		}
		lintIndexName(c, t, ix, p)
	}
	for _, u := range t.unnamed {
		c.addf("NM-11", "table "+t.name, "%s has no explicit CONSTRAINT name", u)
	}
}

func lintForeignKeyName(c *collector, t *table, fk *foreignKey, p *Policy) {
	obj := "constraint " + fk.name
	if fk.name == "" {
		c.addf("NM-11", "table "+t.name, "a foreign key to %s has no explicit CONSTRAINT name", fk.refTable)
		return
	}
	checkIdentifier(c, obj, fk.name, "fk", p)
	if !strings.HasPrefix(fk.name, "fk_"+t.name+"_") || !strings.HasSuffix(fk.name, "_"+fk.refTable) {
		c.addf("NM-11", obj, "foreign keys of %s to %s are named fk_%s_[<qualifier>_]%s", t.name, fk.refTable, t.name, fk.refTable)
	}
}

func lintIndexName(c *collector, t *table, ix *index, p *Policy) {
	obj := "index " + ix.name
	prefix := "ix"
	if ix.unique {
		prefix = "uq"
	}
	if ix.name == "" {
		c.addf("NM-11", "table "+t.name, "a unique constraint has no explicit CONSTRAINT name")
		return
	}
	checkIdentifier(c, obj, ix.name, prefix, p)
	want := prefix + "_" + t.name
	if !strings.HasPrefix(ix.name, want+"_") {
		c.addf("NM-11", obj, "%s of %s are named %s_<columns>", map[bool]string{true: "unique constraints and indexes", false: "indexes"}[ix.unique], t.name, want)
		return
	}
	if ix.hasExpr {
		return
	}
	full := want + "_" + strings.Join(ix.cols, "_")
	if ix.name != full && len(full) <= 63 {
		c.addf("NM-11", obj, "the name lists the columns in order: expected %s", full)
	}
}

var (
	authorColumnRE = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*_by_principal_id$`)
	isHasRE        = regexp.MustCompile(`^(is|has)_`)
)

func lintColumns(c *collector, m *model, t *table, p *Policy) {
	obj := "table " + t.name
	if t.name != "schema_migration" && !equalStrings(t.pkCols, []string{"id"}) {
		c.addf("NM-4", obj, "the primary key is the single column id (found %v)", t.pkCols)
	}
	if t.column("created_at") == nil {
		c.addf("NM-6", obj, "missing column created_at")
	}
	appendOnly := p.appendOnly[t.name]
	switch {
	case appendOnly && t.column("updated_at") != nil:
		c.addf("IN-11", obj, "append-only table must not have updated_at")
	case !appendOnly && t.column("updated_at") == nil:
		c.addf("NM-6", obj, "missing column updated_at (only tables of %s may omit it)", fileAppendOnly)
	}
	for _, col := range t.columns {
		lintColumnType(c, m, t, col, p)
	}
}

func lintColumnType(c *collector, m *model, t *table, col *column, p *Policy) {
	obj := "column " + t.name + "." + col.name
	instant := isInstantType(m.dialect, col.typ)
	if strings.HasSuffix(col.name, "_at") && !instant {
		c.addf("NM-6", obj, "columns ending in _at hold instants: use %s (found %s)", instantTypeName(m.dialect), col.typ)
	}
	if m.dialect == DialectPostgres && instant && !strings.HasSuffix(col.name, "_at") {
		c.addf("NM-6", obj, "timestamptz columns end in _at")
	}
	if isBoolean(m, t, col) {
		if !isHasRE.MatchString(col.name) {
			c.addf("NM-7", obj, "boolean columns start with is_ or has_")
		}
		if m.dialect == DialectSQLite && !hasBoolCheck(t, col.name) {
			c.addf("IN-10", obj, "SQLite booleans carry CHECK (%s IN (0, 1))", col.name)
		}
	} else if isHasRE.MatchString(col.name) && m.dialect == DialectSQLite && !hasBoolCheck(t, col.name) {
		c.addf("IN-10", obj, "columns starting with is_ or has_ are booleans and carry CHECK (%s IN (0, 1))", col.name)
	}
	if reason := forbiddenType(m.dialect, col.typ); reason != "" && !p.excepted("TY-2", m.dialect, t.name+"."+col.name) {
		c.addf("TY-2", obj, "%s is forbidden", reason)
	}
	if m.dialect == DialectSQLite && isIdentityColumn(col.name) && !p.isExternalID(t.name, col.name) {
		if col.typ != "blob" {
			c.addf("ID-2", obj, "identifier columns are BLOB in SQLite (found %s)", col.typ)
		} else if !hasLengthCheck(t, col.name) {
			c.addf("IN-10", obj, "SQLite identifier columns carry CHECK (length(%s) = 16)", col.name)
		}
	}
}

func isIdentityColumn(name string) bool {
	return name == "id" || strings.HasSuffix(name, "_id")
}

func isInstantType(dialect, typ string) bool {
	if dialect == DialectPostgres {
		return strings.HasPrefix(typ, "timestamp with time zone")
	}
	return typ == "integer"
}

func instantTypeName(dialect string) string {
	if dialect == DialectPostgres {
		return "timestamptz"
	}
	return "INTEGER unix microseconds"
}

// isBoolean tells a boolean column: PostgreSQL boolean, or a SQLite INTEGER
// with a (0, 1) CHECK.
func isBoolean(m *model, t *table, col *column) bool {
	if m.dialect == DialectPostgres {
		return col.typ == "boolean"
	}
	return col.typ == "integer" && hasBoolCheck(t, col.name)
}

func hasBoolCheck(t *table, col string) bool {
	for _, ck := range t.checks {
		if boolListFollows(ck.toks, col) {
			return true
		}
	}
	return false
}

// boolListFollows reports whether the tokens hold "col IN ( 0 , 1 )".
func boolListFollows(toks []sqlToken, col string) bool {
	for i := 0; i+6 < len(toks); i++ {
		if n, ok := toks[i].ident(); !ok || n != col || !toks[i+1].isWord("in") || !toks[i+2].isPunct("(") {
			continue
		}
		if toks[i+3].text == "0" && toks[i+4].isPunct(",") && toks[i+5].text == "1" && toks[i+6].isPunct(")") {
			return true
		}
	}
	return false
}

// hasLengthCheck reports whether a CHECK holds "length(col) = 16".
func hasLengthCheck(t *table, col string) bool {
	for _, ck := range t.checks {
		for i := 0; i+5 < len(ck.toks); i++ {
			if !ck.toks[i].isWord("length") || !ck.toks[i+1].isPunct("(") {
				continue
			}
			if n, ok := ck.toks[i+2].ident(); ok && n == col && ck.toks[i+3].isPunct(")") && ck.toks[i+4].isPunct("=") && ck.toks[i+5].text == "16" {
				return true
			}
		}
	}
	return false
}

func lintTenancy(c *collector, t *table, byName map[string]*table, p *Policy) {
	obj := "table " + t.name
	org := t.column("org_id")
	switch {
	case org == nil && !p.instanceTables[t.name]:
		c.addf("TI-1", obj, "organization-scoped tables carry org_id NOT NULL; this table has none and is not in %s", fileInstanceTables)
	case org != nil && !org.notNull:
		c.addf("TI-1", obj, "org_id is NOT NULL")
	}
	for _, fk := range t.fks {
		ref := byName[fk.refTable]
		if ref == nil || org == nil || ref.column("org_id") == nil {
			continue
		}
		if len(fk.cols) != 2 || fk.cols[0] != "org_id" || !equalStrings(fk.refCols, []string{"org_id", "id"}) {
			c.addf("TI-7", "constraint "+fk.name, "foreign keys between organization tables are composite (org_id, <parent>_id) to %s (org_id, id)", fk.refTable)
			continue
		}
		if !hasUnique(ref, []string{"org_id", "id"}) {
			c.addf("ID-8", "table "+ref.name, "referenced by %s: it needs a unique constraint over (org_id, id)", fk.name)
		}
	}
}

func lastOf(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1]
}

func hasUnique(t *table, cols []string) bool {
	for _, ix := range t.indexes {
		if (ix.unique || ix.kind == "primary") && ix.predicate == "" && !ix.hasExpr && equalStrings(ix.cols, cols) {
			return true
		}
	}
	return false
}

func lintIntegrity(c *collector, t *table, p *Policy) {
	inFK := map[string]bool{}
	for _, fk := range t.fks {
		for _, col := range fk.cols {
			inFK[col] = true
		}
	}
	for _, col := range t.columns {
		if col.name == "id" || !strings.HasSuffix(col.name, "_id") {
			continue
		}
		if !inFK[col.name] && !p.isExternalID(t.name, col.name) {
			c.addf("IN-1", "column "+t.name+"."+col.name, "every *_id column is a foreign key; this one is not and is not in %s", fileExternalIDs)
		}
	}
	for _, fk := range t.fks {
		lintForeignKey(c, t, fk)
	}
	for _, col := range t.columns {
		if isEnumerationColumn(col.name, p.enumWords) && !t.hasCheckOn(col.name) {
			c.addf("IN-3", "column "+t.name+"."+col.name, "enumerated columns have a CHECK constraint listing the values")
		}
	}
}

func lintForeignKey(c *collector, t *table, fk *foreignKey) {
	obj := "constraint " + fk.name
	if fk.name == "" {
		obj = "foreign key of " + t.name + " to " + fk.refTable
	}
	switch fk.onDelete {
	case "cascade", "restrict", "set null":
	default:
		c.addf("IN-1", obj, "every foreign key declares ON DELETE CASCADE, RESTRICT or SET NULL (found %s)", orDefault(fk.onDelete, "no action"))
	}
	if len(fk.cols) > 1 && fk.onDelete == "set null" {
		c.addf("IN-6", obj, "a composite foreign key never uses ON DELETE SET NULL")
	}
	if !supportedByIndex(t, fk.cols) {
		c.addf("IN-7", obj, "no index starts with %s", strings.Join(fk.cols, ", "))
	}
	if col := lastOf(fk.cols); col != "" && !strings.HasSuffix(col, fk.refTable+"_id") {
		c.addf("NM-5", obj, "the foreign key column %s is named [<qualifier>_]%s_id", col, fk.refTable)
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func supportedByIndex(t *table, cols []string) bool {
	for _, ix := range t.indexes {
		if !ix.hasExpr && hasPrefixStrings(ix.cols, cols) {
			return true
		}
	}
	return false
}

func isEnumerationColumn(name string, enumWords []string) bool {
	for _, w := range enumWords {
		if name == w || strings.HasSuffix(name, "_"+w) {
			return true
		}
	}
	return false
}

func lintActors(c *collector, t *table, p *Policy) {
	for _, col := range t.columns {
		if !strings.HasSuffix(col.name, "_by") && !strings.Contains(col.name, "_by_") {
			continue
		}
		obj := "column " + t.name + "." + col.name
		if p.isExternalID(t.name, col.name) {
			continue
		}
		if !authorColumnRE.MatchString(col.name) {
			c.addf("IN-12", obj, "actor columns are <verb>_by_principal_id")
			continue
		}
		if col.notNull {
			c.addf("IN-12", obj, "actor columns are nullable (NULL when the principal was deleted)")
		}
		var fk *foreignKey
		for _, f := range t.fks {
			if equalStrings(f.cols, []string{col.name}) {
				fk = f
			}
		}
		switch {
		case fk == nil || fk.refTable != "principal" || !equalStrings(fk.refCols, []string{"id"}):
			c.addf("IN-12", obj, "actor columns have a single-column foreign key to principal(id)")
		case fk.onDelete != "set null":
			c.addf("IN-12", obj, "the foreign key to principal is ON DELETE SET NULL (found %s)", orDefault(fk.onDelete, "no action"))
		}
	}
}

func lintSoftDelete(c *collector, t *table, p *Policy) {
	listed := p.softDelete[t.name]
	has := t.column("deleted_at") != nil
	obj := "table " + t.name
	switch {
	case has && !listed:
		c.addf("DL-4", obj, "deleted_at exists only on tables listed in %s", fileSoftDelete)
	case listed && !has:
		c.addf("DL-4", obj, "listed in %s but has no deleted_at", fileSoftDelete)
	}
	if !listed || !has {
		return
	}
	if t.column("deleted_by_principal_id") == nil {
		c.addf("DL-4", obj, "soft-delete tables have deleted_by_principal_id")
	}
	partial := 0
	for _, ix := range t.indexes {
		if ix.unique && strings.Contains(ix.predicate, "deleted_atisnull") {
			partial++
		}
	}
	if partial != 1 {
		c.addf("DL-4", obj, "soft-delete tables have exactly one partial unique index WHERE deleted_at IS NULL (found %d)", partial)
	}
}

func lintDocuments(c *collector, m *model, t *table, p *Policy) {
	for _, col := range t.columns {
		obj := "column " + t.name + "." + col.name
		if col.typ == "jsonb" && !p.excepted("JS-4", m.dialect, t.name+"."+col.name) {
			c.addf("JS-4", obj, "jsonb needs an owner-approved exception; documents are bytea")
		}
		if col.name != "document" && !strings.HasSuffix(col.name, "_document") {
			continue
		}
		fmtName := col.name + "_format"
		sibling := t.column(fmtName)
		switch {
		case sibling == nil:
			c.addf("JS-3", obj, "a JSON document column has a sibling %s with a CHECK", fmtName)
		case !t.hasCheckOn(fmtName):
			c.addf("JS-3", "column "+t.name+"."+fmtName, "the document format column has a CHECK over the known tags")
		}
	}
}

func lintRedundantIndexes(c *collector, t *table) {
	for _, a := range t.indexes {
		if a.unique || a.kind != "index" || a.hasExpr {
			continue
		}
		for _, b := range t.indexes {
			if a == b || b.hasExpr || a.predicate != b.predicate {
				continue
			}
			if hasPrefixStrings(b.cols, a.cols) && (len(b.cols) > len(a.cols) || b.unique || b.kind != "index" || a.name > b.name) {
				c.addf("QY-7", "index "+a.name, "its columns are a prefix of %s", orDefault(b.name, "another index"))
				break
			}
		}
	}
}

// forbiddenType names why a column type is forbidden (TY-2), or "".
func forbiddenType(dialect, typ string) string {
	typ = strings.ToLower(typ)
	if dialect == DialectSQLite {
		switch typ {
		case "real":
			return "REAL"
		case "any":
			return "ANY"
		}
		return ""
	}
	switch {
	case strings.HasSuffix(typ, "[]"):
		return "an array type"
	case strings.HasPrefix(typ, "character"):
		return "varchar(n) and char(n)"
	case strings.HasPrefix(typ, "timestamp without time zone"):
		return "timestamp without time zone"
	case strings.HasPrefix(typ, "time "), typ == "time", strings.HasPrefix(typ, "time("):
		return "time and timetz"
	case typ == "date":
		return "date"
	case strings.HasPrefix(typ, "interval"):
		return "interval"
	case typ == "money":
		return "money"
	case typ == "real", typ == "double precision":
		return typ
	case strings.HasPrefix(typ, "numeric"):
		return "numeric"
	case typ == "citext":
		return "citext"
	}
	return ""
}

// lintExternalIDWildcards rejects an external_ids.txt entry "*.<x>_id" when a
// table <x> exists: a wildcard is for identifiers that do not identify a row of
// Orcon (nodes and edges inside a stored document, identifiers of providers).
func lintExternalIDWildcards(c *collector, m *model, p *Policy) {
	tables := map[string]bool{}
	for _, t := range m.tables {
		tables[t.name] = true
	}
	keys := make([]string, 0, len(p.externalIDs))
	for k := range p.externalIDs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		column, ok := strings.CutPrefix(k, "*.")
		if !ok {
			continue
		}
		if table, isID := strings.CutSuffix(column, "_id"); isID && tables[table] {
			c.addf("IN-1", "policy "+fileExternalIDs, "entry %s applies to every table, but table %s exists and its identifier is a foreign key", k, table)
		}
	}
}
