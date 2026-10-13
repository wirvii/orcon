// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// checkMigrationNames is "migration-names" (NM-3, NM-15, MG-1).
func checkMigrationNames(w *world) []Finding {
	var out []Finding
	for _, d := range w.dialects {
		var numbered []*migration
		for _, m := range d.migrations {
			sub := migrationNameRE.FindStringSubmatch(m.name)
			if sub == nil {
				out = append(out, Finding{File: m.path, Line: 1, Rule: "NM-15",
					Msg: fmt.Sprintf("migration name %q does not match ^\\d{4}_[a-z][a-z0-9]*(_[a-z0-9]+)*\\.sql$", m.name),
					Fix: "rename the file to NNNN_<snake_description>.sql"})
				continue
			}
			numbered = append(numbered, m)
			for _, hit := range w.policy.abbreviationFindings(splitTokens(sub[2]), "") {
				out = append(out, Finding{File: m.path, Line: 1, Rule: "NM-3",
					Msg: fmt.Sprintf("abbreviation %q in migration name %q", hit.token, m.name),
					Fix: "spell it out: " + hit.expansion})
			}
		}
		sort.SliceStable(numbered, func(i, j int) bool { return numbered[i].number < numbered[j].number })
		expected := 1
		last := 0
		for _, m := range numbered {
			switch {
			case m.number == last:
				out = append(out, Finding{File: m.path, Line: 1, Rule: "MG-1", Msg: fmt.Sprintf("duplicate migration number %04d", m.number), Fix: "give each migration its own number"})
			case m.number != expected:
				out = append(out, Finding{File: m.path, Line: 1, Rule: "MG-1", Msg: fmt.Sprintf("migration numbers must be contiguous from 0001: expected %04d, found %04d", expected, m.number), Fix: "renumber the new migration"})
				expected = m.number + 1
			default:
				expected++
			}
			last = m.number
		}
	}
	return out
}

// checkMigrationPairs is "migration-pairs" (MG-1, MG-2, MG-11).
func checkMigrationPairs(w *world) []Finding {
	var out []Finding
	byNumber := map[string]map[int]*migration{}
	for _, d := range w.dialects {
		byNumber[d.name] = map[int]*migration{}
		for _, m := range d.migrations {
			if m.number > 0 {
				byNumber[d.name][m.number] = m
			}
		}
	}
	lite, pg := w.dialect(dialectSQLite), w.dialect(dialectPostgres)
	nums := map[int]bool{}
	for _, d := range w.dialects {
		for n := range byNumber[d.name] {
			nums[n] = true
		}
	}
	ordered := make([]int, 0, len(nums))
	for n := range nums {
		ordered = append(ordered, n)
	}
	sort.Ints(ordered)
	for _, n := range ordered {
		a, b := byNumber[dialectSQLite][n], byNumber[dialectPostgres][n]
		switch {
		case a == nil:
			out = append(out, Finding{File: b.path, Line: 1, Rule: "MG-1", Msg: fmt.Sprintf("migration %04d has no pair in the %s directory", n, dialectSQLite), Fix: "add " + filepath.Join(lite.migrationsDir(), b.name)})
		case b == nil:
			out = append(out, Finding{File: a.path, Line: 1, Rule: "MG-1", Msg: fmt.Sprintf("migration %04d has no pair in the %s directory", n, dialectPostgres), Fix: "add " + filepath.Join(pg.migrationsDir(), a.name)})
		default:
			if a.name != b.name {
				out = append(out, Finding{File: b.path, Line: 1, Rule: "MG-1", Msg: fmt.Sprintf("pair %04d has different names: %s and %s", n, a.name, b.name), Fix: "use one name in both directories"})
			}
			out = append(out, headerDifferences(a, b)...)
		}
	}
	for _, d := range w.dialects {
		for _, m := range d.migrations {
			if m.number == 0 {
				continue
			}
			for _, kind := range []string{"seed", "verify"} {
				name := fmt.Sprintf("%04d_%s.sql", m.number, kind)
				path := filepath.Join(d.migrationsDir(), "testdata", name)
				if _, err := os.Stat(path); err != nil {
					out = append(out, Finding{File: m.path, Line: 1, Rule: "MG-11", Msg: fmt.Sprintf("missing %s", path), Fix: "add the " + kind + " file of the upgrade test"})
				}
			}
		}
	}
	return out
}

func headerDifferences(a, b *migration) []Finding {
	var out []Finding
	diff := func(what, x, y string) {
		if x != y {
			out = append(out, Finding{File: b.path, Line: 1, Rule: "MG-2", Msg: fmt.Sprintf("pair %04d has different %s headers: %q (%s) and %q (%s)", a.number, what, x, a.dialect, y, b.dialect), Fix: "align the header of the pair"})
		}
	}
	diff("kind", a.header.kind, b.header.kind)
	diff("compatibility-floor", a.header.floor, b.header.floor)
	diff("contracts", a.header.contracts, b.header.contracts)
	return out
}

var (
	floorRE     = regexp.MustCompile(`^(self|\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?)$`)
	contractsRE = regexp.MustCompile(`^\d{4}$`)
)

var migrationKinds = map[string]bool{"additive": true, "expand": true, "contract": true, "rebuild": true, "data": true}

// checkMigrationHeaders is "migration-headers" (MG-2).
func checkMigrationHeaders(w *world) []Finding {
	var out []Finding
	for _, d := range w.dialects {
		for _, m := range d.migrations {
			if m.toks == nil {
				continue // unreadable SQL is reported at load time
			}
			h := m.header
			for _, p := range h.problems {
				p.File = m.path
				out = append(out, p)
			}
			hf := func(line int, msg, fix string) {
				if line == 0 {
					line = 1
				}
				out = append(out, Finding{File: m.path, Line: line, Rule: "MG-2", Msg: msg, Fix: fix})
			}
			switch {
			case h.kind == "":
				hf(1, "missing \"-- orcon:kind\" header", "add \"-- orcon:kind additive|expand|contract|rebuild|data\"")
			case !migrationKinds[h.kind]:
				hf(h.kindLine, fmt.Sprintf("unknown kind %q", h.kind), "use additive, expand, contract, rebuild or data")
			}
			switch {
			case h.floor == "":
				hf(1, "missing \"-- orcon:compatibility-floor\" header", "add \"-- orcon:compatibility-floor <semver>|self\"")
			case !floorRE.MatchString(h.floor):
				hf(h.floorLine, fmt.Sprintf("compatibility floor %q is neither a semantic version nor \"self\"", h.floor), "write MAJOR.MINOR.PATCH or self")
			}
			switch {
			case h.kind == "contract" && h.contracts == "":
				hf(h.kindLine, "kind contract needs \"-- orcon:contracts NNNN\"", "name the expand migration this one contracts")
			case h.kind != "contract" && h.contracts != "":
				hf(h.kindLine, fmt.Sprintf("\"-- orcon:contracts\" is only valid with kind contract, not %q", h.kind), "remove the header")
			case h.contracts != "" && !contractsRE.MatchString(h.contracts):
				hf(h.kindLine, fmt.Sprintf("contracts value %q is not a four-digit migration number", h.contracts), "write NNNN")
			}
			out = append(out, noTransactionFindings(m)...)
		}
	}
	return out
}

func noTransactionFindings(m *migration) []Finding {
	var out []Finding
	concurrently := false
	for _, s := range m.stmts {
		if hasWordSeq(s.words(), "create", "index") || hasWordSeq(s.words(), "create", "unique", "index") {
			for _, w := range s.words() {
				if w == "concurrently" {
					concurrently = true
				}
			}
		}
	}
	switch {
	case m.header.noTx && m.dialect == dialectSQLite:
		out = append(out, Finding{File: m.path, Line: 1, Rule: "MG-2", Msg: "\"-- orcon:no-transaction\" is valid only in PostgreSQL migrations", Fix: "remove the header"})
	case m.header.noTx:
		single := len(m.stmts) == 1 && concurrently
		if !single {
			out = append(out, Finding{File: m.path, Line: 1, Rule: "MG-9", Msg: "a no-transaction migration holds exactly one CREATE INDEX CONCURRENTLY statement", Fix: "move other statements to their own migration"})
		}
	case concurrently && m.dialect == dialectPostgres:
		out = append(out, Finding{File: m.path, Line: 1, Rule: "MG-9", Msg: "CREATE INDEX CONCURRENTLY cannot run in a transaction", Fix: "add \"-- orcon:no-transaction\" and keep it the only statement"})
	}
	return out
}

// hasWordSeq reports whether words contains the sequence seq contiguously.
func hasWordSeq(words []string, seq ...string) bool {
	return indexWordSeq(words, 0, seq...) >= 0
}

func indexWordSeq(words []string, from int, seq ...string) int {
	for i := from; i+len(seq) <= len(words); i++ {
		match := true
		for j, s := range seq {
			if words[i+j] != s {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// sumLine is one "<file>  <sha256>" line of migrations.sum.
type sumLine struct {
	file string
	hash string
	line int
}

var sumLineRE = regexp.MustCompile(`^(\S+) {2}([0-9a-f]{64})$`)

// readSum parses migrations.sum content; "#" lines and blanks are comments.
func readSum(path string, data []byte) (map[string]sumLine, []Finding) {
	out := map[string]sumLine{}
	var fs []Finding
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	n := 0
	for sc.Scan() {
		n++
		l := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
			continue
		}
		m := sumLineRE.FindStringSubmatch(l)
		if m == nil {
			fs = append(fs, Finding{File: path, Line: n, Rule: "MG-5", Msg: fmt.Sprintf("malformed line %q: expected \"<file>  <sha256>\"", l), Fix: "restore the line; use make db-sum for new files"})
			continue
		}
		if prev, dup := out[m[1]]; dup {
			fs = append(fs, Finding{File: path, Line: n, Rule: "MG-5", Msg: fmt.Sprintf("file %s is listed twice (first on line %d)", m[1], prev.line), Fix: "remove the duplicate line"})
			continue
		}
		out[m[1]] = sumLine{file: m[1], hash: m[2], line: n}
	}
	return out, fs
}

func fileHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// checkMigrationImmutable is "migration-immutable" (MG-5).
func checkMigrationImmutable(w *world) []Finding {
	var out []Finding
	rev, baseErr := resolveBase(w.cfg.Git, firstDir(w), w.cfg.Base)
	switch {
	case baseErr == nil:
		w.note("migration-immutable: comparing migrations.sum with base revision %s", rev)
	case errors.Is(baseErr, errNoBase):
		w.note("migration-immutable: no base revision available (%v); compared only against migrations.sum", baseErr)
		rev = ""
	default:
		out = append(out, Finding{File: w.dialects[0].sumPath(), Line: 1, Rule: "MG-5", Msg: baseErr.Error(), Fix: "set DBCHECK_BASE to a revision that exists (CI uses the pull request base) and fetch the full history"})
		rev = ""
	}
	for _, d := range w.dialects {
		out = append(out, immutableForDialect(w, d, rev)...)
	}
	return out
}

func firstDir(w *world) string {
	if len(w.dialects) > 0 {
		return w.dialects[0].dir
	}
	return "."
}

func immutableForDialect(w *world, d *dialectState, rev string) []Finding {
	var out []Finding
	path := d.sumPath()
	data, err := os.ReadFile(path) //nolint:gosec // path is built from the adapter directory given on the command line
	if err != nil {
		msg := "migrations.sum cannot be read: " + err.Error()
		if errors.Is(err, fs.ErrNotExist) {
			msg = "migrations.sum is missing"
		}
		return []Finding{{File: path, Line: 1, Rule: "MG-5", Msg: msg, Fix: "create it with make db-sum"}}
	}
	sum, fs := readSum(path, data)
	out = append(out, fs...)
	present := map[string]bool{}
	for _, m := range d.migrations {
		present[m.name] = true
		line, listed := sum[m.name]
		switch {
		case !listed:
			out = append(out, Finding{File: m.path, Line: 1, Rule: "MG-5", Msg: "migration is not listed in migrations.sum", Fix: "run make db-sum to append its line"})
		case line.hash != fileHash(m.data):
			out = append(out, Finding{File: m.path, Line: 1, Rule: "MG-5", Msg: fmt.Sprintf("migration changed after it was recorded (migrations.sum line %d): checksum differs", line.line), Fix: "revert the edit and write a new migration"})
		}
	}
	names := make([]string, 0, len(sum))
	for n := range sum {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if !present[n] {
			out = append(out, Finding{File: path, Line: sum[n].line, Rule: "MG-5", Msg: fmt.Sprintf("%s is listed but the file is missing (removed or renamed)", n), Fix: "restore the migration; a published migration is never removed"})
		}
	}
	if rev == "" {
		return out
	}
	baseData, exists, err := showFile(w.cfg.Git, d.migrationsDir(), rev, "migrations.sum")
	if err != nil {
		return append(out, Finding{File: path, Line: 1, Rule: "MG-5", Msg: fmt.Sprintf("cannot read migrations.sum at base %s: %v", rev, err)})
	}
	if !exists {
		w.note("migration-immutable: %s does not exist at base %s; nothing to compare", path, rev)
		return out
	}
	baseSum, _ := readSum(path, baseData)
	baseNames := make([]string, 0, len(baseSum))
	for n := range baseSum {
		baseNames = append(baseNames, n)
	}
	sort.Strings(baseNames)
	for _, n := range baseNames {
		cur, ok := sum[n]
		switch {
		case !ok:
			out = append(out, Finding{File: path, Line: 1, Rule: "MG-5", Msg: fmt.Sprintf("line for %s exists in base %s but was removed", n, rev), Fix: "restore the line; published migrations are immutable"})
		case cur.hash != baseSum[n].hash:
			out = append(out, Finding{File: path, Line: cur.line, Rule: "MG-5", Msg: fmt.Sprintf("line for %s differs from base %s", n, rev), Fix: "restore the line and add a new migration instead"})
		}
	}
	return out
}

// runSum appends the missing lines of migrations.sum for every .sql migration
// and never rewrites an existing line.
func runSum(w *world) ([]string, error) {
	var added []string
	for _, d := range w.dialects {
		path := d.sumPath()
		data, err := os.ReadFile(path) //nolint:gosec // path is built from the adapter directory given on the command line
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return added, fmt.Errorf("read %s: %w", path, err)
		}
		sum, problems := readSum(path, data)
		if len(problems) > 0 {
			return added, fmt.Errorf("%s: cannot append to a malformed file: %s", path, problems[0])
		}
		var buf strings.Builder
		buf.Write(data)
		if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
			buf.WriteString("\n")
		}
		for _, m := range d.migrations {
			if _, ok := sum[m.name]; ok {
				continue
			}
			fmt.Fprintf(&buf, "%s  %s\n", m.name, fileHash(m.data))
			added = append(added, path+": "+m.name)
		}
		if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil { //nolint:gosec // checked-in text file
			return added, fmt.Errorf("write %s: %w", path, err)
		}
	}
	return added, nil
}

// checkExpandContract is "migration-expand-contract" (MG-6, MG-7).
func checkExpandContract(w *world) []Finding {
	var out []Finding
	for _, d := range w.dialects {
		for _, m := range d.migrations {
			if m.toks == nil {
				continue
			}
			out = append(out, breakingStatements(m)...)
			if m.header.kind == "contract" {
				out = append(out, contractReleaseFindings(w, d, m)...)
			}
		}
	}
	return out
}

// breakingStatements reports the statements MG-7 rejects outside kind
// contract. A rebuild migration in SQLite follows the twelve-step procedure
// (MG-8), which creates a new table, drops the old one and renames the new
// one, so DROP TABLE and RENAME are accepted there and nowhere else.
func breakingStatements(m *migration) []Finding {
	if m.header.kind == "contract" {
		return nil
	}
	rebuildSQLite := m.header.kind == "rebuild" && m.dialect == dialectSQLite
	var out []Finding
	report := func(line int, what string) {
		out = append(out, Finding{File: m.path, Line: line, Rule: "MG-7", Msg: fmt.Sprintf("%s is only allowed in a contract migration (kind is %q)", what, orNone(m.header.kind)), Fix: "split the change into an expand migration and a later contract migration"})
	}
	for _, s := range m.stmts {
		words := s.words()
		if len(words) == 0 {
			continue
		}
		if !rebuildSQLite {
			if hasWordSeq(words, "drop", "table") {
				report(s.line, "DROP TABLE")
			}
			if len(words) > 1 && words[0] == "alter" && words[1] == "table" && containsWord(words, "rename") {
				report(s.line, "RENAME")
			}
		}
		if words[0] == "alter" {
			if hasWordSeq(words, "drop", "column") {
				report(s.line, "DROP COLUMN")
			}
			for i := indexWordSeq(words, 0, "alter", "column"); i >= 0; i = indexWordSeq(words, i+1, "alter", "column") {
				// ALTER COLUMN <name> [SET DATA] TYPE <type>
				rest := words[min(i+3, len(words)):]
				if len(rest) > 0 && rest[0] == "type" || len(rest) > 2 && rest[0] == "set" && rest[1] == "data" && rest[2] == "type" {
					report(s.line, "ALTER COLUMN ... TYPE")
				}
			}
			if hasWordSeq(words, "set", "not", "null") {
				report(s.line, "SET NOT NULL")
			}
			out = append(out, addColumnNotNull(m, s)...)
		}
	}
	return out
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

func containsWord(words []string, w string) bool {
	for _, x := range words {
		if x == w {
			return true
		}
	}
	return false
}

// addColumnNotNull finds "ADD [COLUMN] ... NOT NULL" without DEFAULT inside
// one ALTER TABLE action.
func addColumnNotNull(m *migration, s stmt) []Finding {
	var out []Finding
	toks := s.toks
	for i, t := range toks {
		if !t.isWord("add") || t.depth != 0 {
			continue
		}
		// The next word decides: ADD CONSTRAINT / PRIMARY / UNIQUE / ... is not a column.
		j := i + 1
		if j < len(toks) && toks[j].isWord("column") {
			j++
		} else if j < len(toks) && toks[j].kind == tokWord {
			switch toks[j].lower {
			case "constraint", "primary", "unique", "foreign", "check", "exclude":
				continue
			}
		}
		notNull, hasDefault := false, false
		for k := j; k < len(toks); k++ {
			u := toks[k]
			if u.depth == 0 && (u.isPunct(",") || u.isWord("add")) && k > j {
				break
			}
			if u.depth != 0 {
				continue
			}
			if u.isWord("default") {
				hasDefault = true
			}
			if u.isWord("not") && k+1 < len(toks) && toks[k+1].isWord("null") {
				notNull = true
			}
		}
		if notNull && !hasDefault {
			out = append(out, Finding{File: m.path, Line: t.line, Rule: "MG-7", Msg: fmt.Sprintf("ADD COLUMN ... NOT NULL without DEFAULT is only allowed in a contract migration (kind is %q)", orNone(m.header.kind)), Fix: "add the column as nullable or with a DEFAULT, backfill, and tighten in a later contract migration"})
		}
	}
	return out
}

// contractReleaseFindings checks that the migration a contract migration
// removes was shipped (as an expand migration) in an earlier release tag. With
// no release tags the condition cannot apply; that is declared as a note.
func contractReleaseFindings(w *world, d *dialectState, m *migration) []Finding {
	if m.header.contracts == "" || !contractsRE.MatchString(m.header.contracts) {
		return nil // reported by migration-headers
	}
	var out []Finding
	var target *migration
	for _, o := range d.migrations {
		if fmt.Sprintf("%04d", o.number) == m.header.contracts {
			target = o
		}
	}
	if target == nil {
		out = append(out, Finding{File: m.path, Line: m.header.kindLine, Rule: "MG-6", Msg: fmt.Sprintf("contracts %s names a migration that does not exist", m.header.contracts), Fix: "name the expand migration this one contracts"})
	} else if target.header.kind != "expand" {
		out = append(out, Finding{File: m.path, Line: m.header.kindLine, Rule: "MG-6", Msg: fmt.Sprintf("contracts %s names %s, whose kind is %q, not expand", m.header.contracts, target.name, orNone(target.header.kind)), Fix: "a contract migration removes what an expand migration replaced"})
	}
	tags, err := releaseTags(w.cfg.Git, d.dir)
	switch {
	case err != nil:
		w.note("migration-expand-contract: release tags unavailable (%v); the condition \"expand shipped in an earlier release\" is not applicable here", err)
		return out
	case len(tags) == 0:
		w.note("migration-expand-contract: no release tags (v*) exist; the condition \"expand shipped in an earlier release\" is not applicable")
		return out
	}
	names, err := listDir(w.cfg.Git, d.dir, tags[0], "migrations")
	if err != nil {
		return append(out, Finding{File: m.path, Line: 1, Rule: "MG-6", Msg: fmt.Sprintf("cannot list migrations at release %s: %v", tags[0], err)})
	}
	shipped := false
	for _, n := range names {
		if strings.HasPrefix(n, m.header.contracts+"_") {
			shipped = true
		}
	}
	if !shipped {
		out = append(out, Finding{File: m.path, Line: m.header.kindLine, Rule: "MG-6", Msg: fmt.Sprintf("migration %s was not shipped in release %s: a contract needs its expand in an earlier release", m.header.contracts, tags[0]), Fix: "ship the expand migration in a release first"})
	}
	return out
}
