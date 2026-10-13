// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	dialectSQLite   = "sqlite"
	dialectPostgres = "postgres"
)

// Finding is one rule violation, printed as
// "<file>:<line>: <rule> <message> (fix: <fix>)".
type Finding struct {
	File string
	Line int
	Rule string
	Msg  string
	Fix  string
}

func (f Finding) String() string {
	s := fmt.Sprintf("%s:%d: %s %s", f.File, f.Line, f.Rule, f.Msg)
	if f.Fix != "" {
		s += " (fix: " + f.Fix + ")"
	}
	return s
}

// Names of the declaration files of internal/adapter/storetest/dbrules/policy
// (spec 4.4.7). Every list lives in those files; none is duplicated in code.
const (
	fileInstanceTables   = "instance_tables.txt"
	fileAppendOnly       = "append_only_tables.txt"
	fileSoftDelete       = "soft_delete_tables.txt"
	fileExternalIDs      = "external_ids.txt"
	fileSingular         = "singular_exceptions.txt"
	fileDenylist         = "abbreviation_denylist.txt"
	fileAllowedAcronyms  = "allowed_acronyms.txt"
	fileReservedWords    = "reserved_words.txt"
	fileHotQueryPatterns = "hot_query_patterns.txt"
	fileDialectOnly      = "dialect_only_queries.txt"
	fileQueryVerbs       = "query_verbs.txt"
	fileEnumWords        = "enumeration_words.txt"
	fileExceptions       = "exceptions.txt"
)

// listEntry is one data line of a policy file.
type listEntry struct {
	value   string // first field
	comment string // text after "# "
	line    int
}

// policy is the parsed content of the policy directory.
type policy struct {
	dir string

	instanceTables map[string]listEntry
	appendOnly     map[string]listEntry
	softDelete     map[string]listEntry
	externalIDs    map[string]listEntry
	singular       map[string]listEntry
	allowed        map[string]listEntry
	denied         map[string]listEntry
	reserved       map[string]listEntry
	dialectOnly    map[string]listEntry
	queryVerbs     map[string]listEntry
	enumWords      map[string]listEntry
	hotPatterns    []hotPattern

	// ordered keeps the entries of the two naming lists in file order, so the
	// order and duplicate checks of checkNamingLists see what the file says.
	allowedOrdered []listEntry
	deniedOrdered  []listEntry
}

type hotPattern struct {
	re   *regexp.Regexp
	line int
}

var (
	tokenRE       = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	commentedRE   = regexp.MustCompile(`^(\S+)\s+#\s+(\S.*)$`)
	externalIDRE  = regexp.MustCompile(`^(\*|[a-z][a-z0-9_]*)\.[a-z][a-z0-9_]*$`)
	dialectOnlyRE = regexp.MustCompile(`^(sqlite|postgres):([A-Z][A-Za-z0-9]*)$`)
	verbRE        = regexp.MustCompile(`^[A-Z][a-z]+$`)
	snakeRE       = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
)

// loadPolicy reads every declaration file of dir. A missing or malformed file
// is a finding, never a silent default: a check that cannot load a list it
// needs must fail.
func loadPolicy(dir string) (*policy, []Finding) {
	p := &policy{dir: dir}
	var fs []Finding
	add := func(f []Finding) { fs = append(fs, f...) }

	var f []Finding
	p.instanceTables, f = loadCommented(dir, fileInstanceTables, nil)
	add(f)
	p.appendOnly, f = loadCommented(dir, fileAppendOnly, nil)
	add(f)
	p.softDelete, f = loadCommented(dir, fileSoftDelete, snakeRE)
	add(f)
	p.externalIDs, f = loadCommented(dir, fileExternalIDs, externalIDRE)
	add(f)
	p.singular, f = loadCommented(dir, fileSingular, snakeRE)
	add(f)
	p.dialectOnly, f = loadCommented(dir, fileDialectOnly, dialectOnlyRE)
	add(f)
	p.queryVerbs, f = loadCommented(dir, fileQueryVerbs, verbRE)
	add(f)
	p.enumWords, f = loadCommented(dir, fileEnumWords, snakeRE)
	add(f)

	var ordered []listEntry
	p.allowed, ordered, f = loadNamingList(dir, fileAllowedAcronyms)
	p.allowedOrdered = ordered
	add(f)
	p.denied, ordered, f = loadNamingList(dir, fileDenylist)
	p.deniedOrdered = ordered
	add(f)

	p.reserved, f = loadReserved(dir)
	add(f)
	p.hotPatterns, f = loadHotPatterns(dir)
	add(f)
	return p, fs
}

// readDataLines returns the non-blank, non-comment lines of a policy file.
func readDataLines(dir, name string) ([]listEntry, []Finding) {
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path) //nolint:gosec // path is built from the policy directory given on the command line
	if err != nil {
		msg := "policy file cannot be read: " + err.Error()
		if errors.Is(err, fs.ErrNotExist) {
			msg = "policy file is missing"
		}
		return nil, []Finding{{File: path, Line: 1, Rule: "CK-17", Msg: msg, Fix: "restore the file from internal/adapter/storetest/dbrules/policy (a check never passes without its list)"}}
	}
	var out []listEntry
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, listEntry{value: line, line: n})
	}
	return out, nil
}

// loadCommented reads "<value>  # reason" lines. valuePattern, when set,
// constrains the value.
func loadCommented(dir, name string, valuePattern *regexp.Regexp) (map[string]listEntry, []Finding) {
	lines, fs := readDataLines(dir, name)
	path := filepath.Join(dir, name)
	out := make(map[string]listEntry, len(lines))
	for _, l := range lines {
		m := commentedRE.FindStringSubmatch(l.value)
		if m == nil {
			fs = append(fs, Finding{File: path, Line: l.line, Rule: "CK-17", Msg: fmt.Sprintf("malformed line %q: expected \"<value>  # <reason>\"", l.value), Fix: "add the reason after \"# \""})
			continue
		}
		if valuePattern != nil && !valuePattern.MatchString(m[1]) {
			fs = append(fs, Finding{File: path, Line: l.line, Rule: "CK-17", Msg: fmt.Sprintf("value %q does not match %s", m[1], valuePattern), Fix: "correct the value"})
			continue
		}
		if prev, dup := out[m[1]]; dup {
			fs = append(fs, Finding{File: path, Line: l.line, Rule: "CK-17", Msg: fmt.Sprintf("duplicate entry %q (first on line %d)", m[1], prev.line), Fix: "remove the duplicate"})
			continue
		}
		out[m[1]] = listEntry{value: m[1], comment: m[2], line: l.line}
	}
	return out, fs
}

// loadNamingList reads allowed_acronyms.txt or abbreviation_denylist.txt:
// "<token>  # <expansion>" with the token matching ^[a-z][a-z0-9]*$. Order
// and duplicates are judged by checkNamingLists, so every parsable line is
// kept in file order.
func loadNamingList(dir, name string) (map[string]listEntry, []listEntry, []Finding) {
	lines, fs := readDataLines(dir, name)
	path := filepath.Join(dir, name)
	set := make(map[string]listEntry, len(lines))
	var ordered []listEntry
	for _, l := range lines {
		m := commentedRE.FindStringSubmatch(l.value)
		if m == nil {
			fs = append(fs, Finding{File: path, Line: l.line, Rule: "NM-3", Msg: fmt.Sprintf("line %q has no \"# <expansion>\"", l.value), Fix: "write \"<token>  # <expansion>\""})
			continue
		}
		if !tokenRE.MatchString(m[1]) {
			fs = append(fs, Finding{File: path, Line: l.line, Rule: "NM-3", Msg: fmt.Sprintf("token %q does not match ^[a-z][a-z0-9]*$", m[1]), Fix: "use one lowercase token"})
			continue
		}
		e := listEntry{value: m[1], comment: m[2], line: l.line}
		ordered = append(ordered, e)
		if _, dup := set[m[1]]; !dup {
			set[m[1]] = e
		}
	}
	return set, ordered, fs
}

func loadReserved(dir string) (map[string]listEntry, []Finding) {
	lines, fs := readDataLines(dir, fileReservedWords)
	path := filepath.Join(dir, fileReservedWords)
	out := make(map[string]listEntry, len(lines))
	for _, l := range lines {
		if !regexp.MustCompile(`^[a-z_][a-z0-9_]*$`).MatchString(l.value) {
			fs = append(fs, Finding{File: path, Line: l.line, Rule: "CK-17", Msg: fmt.Sprintf("reserved word %q must be one lowercase token per line", l.value), Fix: "write one token per line"})
			continue
		}
		out[l.value] = listEntry{value: l.value, line: l.line}
	}
	return out, fs
}

func loadHotPatterns(dir string) ([]hotPattern, []Finding) {
	lines, fs := readDataLines(dir, fileHotQueryPatterns)
	path := filepath.Join(dir, fileHotQueryPatterns)
	var out []hotPattern
	for _, l := range lines {
		re, err := regexp.Compile(l.value)
		if err != nil {
			fs = append(fs, Finding{File: path, Line: l.line, Rule: "QY-4", Msg: fmt.Sprintf("pattern %q is not a valid regular expression: %v", l.value, err), Fix: "correct the pattern"})
			continue
		}
		out = append(out, hotPattern{re: re, line: l.line})
	}
	return out, fs
}

// checkNamingLists validates the two NM-3 lists against each other and
// against their own format rules (NM-3 step 5): a token in both lists, a
// duplicate, or a list that is not in alphabetical order. Line-level format
// errors (a missing expansion, a token outside ^[a-z][a-z0-9]*$) are reported
// by the loader.
func checkNamingLists(p *policy) []Finding {
	var fs []Finding
	for _, e := range p.allowedOrdered {
		if d, both := p.denied[e.value]; both {
			fs = append(fs, Finding{File: filepath.Join(p.dir, fileAllowedAcronyms), Line: e.line, Rule: "NM-3",
				Msg: fmt.Sprintf("token %q is in both %s and %s (line %d)", e.value, fileAllowedAcronyms, fileDenylist, d.line),
				Fix: "keep the token in one list only"})
		}
	}
	fs = append(fs, orderedListFindings(filepath.Join(p.dir, fileAllowedAcronyms), p.allowedOrdered)...)
	fs = append(fs, orderedListFindings(filepath.Join(p.dir, fileDenylist), p.deniedOrdered)...)
	sort.SliceStable(fs, func(i, j int) bool {
		return fs[i].File < fs[j].File || fs[i].File == fs[j].File && fs[i].Line < fs[j].Line
	})
	return fs
}

func orderedListFindings(path string, entries []listEntry) []Finding {
	var fs []Finding
	seen := map[string]int{}
	for i, e := range entries {
		if first, dup := seen[e.value]; dup {
			fs = append(fs, Finding{File: path, Line: e.line, Rule: "NM-3", Msg: fmt.Sprintf("duplicate token %q (first on line %d)", e.value, first), Fix: "remove the duplicate"})
		} else {
			seen[e.value] = e.line
		}
		if i > 0 && e.value < entries[i-1].value {
			fs = append(fs, Finding{File: path, Line: e.line, Rule: "NM-3", Msg: fmt.Sprintf("token %q is out of alphabetical order (after %q)", e.value, entries[i-1].value), Fix: "sort the list alphabetically"})
		}
	}
	return fs
}

// splitTokens splits a name into the tokens NM-3 judges. A PascalCase name
// (a sqlc query name) splits before an uppercase letter that follows a
// lowercase letter or digit, and before the last uppercase letter of an
// uppercase run that is followed by a lowercase letter; a snake_case name
// splits on "_". Every token is lowercased.
func splitTokens(name string) []string {
	if strings.Contains(name, "_") || name == strings.ToLower(name) {
		var out []string
		for _, t := range strings.Split(name, "_") {
			if t != "" {
				out = append(out, strings.ToLower(t))
			}
		}
		return out
	}
	runes := []rune(name)
	var out []string
	start := 0
	isUpper := func(r rune) bool { return r >= 'A' && r <= 'Z' }
	isLowerOrDigit := func(r rune) bool { return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' }
	isLower := func(r rune) bool { return r >= 'a' && r <= 'z' }
	for i := 1; i < len(runes); i++ {
		r := runes[i]
		if !isUpper(r) {
			continue
		}
		prev := runes[i-1]
		boundary := isLowerOrDigit(prev) || (isUpper(prev) && i+1 < len(runes) && isLower(runes[i+1]))
		if boundary {
			out = append(out, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}
	out = append(out, strings.ToLower(string(runes[start:])))
	return out
}

func isDigits(s string) bool {
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

// abbreviationHit is a token the denied list rejects, with the full word(s)
// the list gives for it.
type abbreviationHit struct {
	token     string
	expansion string
}

// abbreviationFindings applies the NM-3 decision to the tokens of tokens
// (already split). prefix is the constraint or index prefix (pk, fk, uq, ck,
// ix) accepted as the first token of an object name of that kind, or "".
// It returns one hit per denied token.
func (p *policy) abbreviationFindings(tokens []string, prefix string) []abbreviationHit {
	var out []abbreviationHit
	for i, t := range tokens {
		if _, ok := p.allowed[t]; ok {
			continue
		}
		if isDigits(t) {
			continue
		}
		if i == 0 && prefix != "" && t == prefix {
			continue
		}
		if e, denied := p.denied[t]; denied {
			out = append(out, abbreviationHit{token: t, expansion: strings.TrimSpace(e.comment)})
		}
	}
	return out
}

// checkPolicy is the "policy" check: every policy file loads and is well
// formed, the naming lists agree, and the exceptions file is valid and not
// expired.
func checkPolicy(w *world) []Finding {
	fs := append([]Finding(nil), w.policyFindings...)
	if w.policy != nil {
		fs = append(fs, checkNamingLists(w.policy)...)
	}
	fs = append(fs, w.exceptionFindings...)
	return fs
}
