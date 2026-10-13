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

// readNames reads a policy list: one name per line followed by an optional
// "# reason"; blank lines and comment lines are ignored.
func readNames(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("policy list: %w", err)
	}
	defer f.Close()
	out := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if fields := strings.Fields(line); len(fields) > 0 {
			out[strings.ToLower(fields[0])] = true
		}
	}
	return out, sc.Err()
}

// allowEntry is one reviewed statement of the engine plane.
type allowEntry struct {
	dialect string
	query   string
	reason  string
	line    int
}

func (e allowEntry) key() string { return e.dialect + ":" + e.query }

// allowlist is engine_allowlist.yaml.
type allowlist struct {
	path    string
	entries []allowEntry
}

func (l *allowlist) find(dialect, name string) *allowEntry {
	for i := range l.entries {
		if l.entries[i].dialect == dialect && l.entries[i].query == name {
			return &l.entries[i]
		}
	}
	return nil
}

// known reports whether the dialect is one of the adapter directories checked.
func (l *allowlist) known(dialect string, adapterDirs []string) bool {
	for _, d := range adapterDirs {
		if filepath.Base(d) == dialect {
			return true
		}
	}
	return false
}

var yamlLine = regexp.MustCompile(`^(dialect|query|reason):\s*(.*?)\s*$`)

// readEngineAllowlist parses the restricted list format of engine_allowlist.yaml:
//
//   - dialect: postgres
//     query: ClaimDueTimer
//     reason: cross-organization scheduling scan (ADR-0047 section 2)
//
// A missing file is an empty allowlist. Comment lines start with "#".
func readEngineAllowlist(path string) (*allowlist, []Finding, error) {
	list := &allowlist{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return list, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var findings []Finding
	bad := func(line int, msg string) {
		findings = append(findings, Finding{File: path, Line: line, Rule: "TI-3", Msg: msg})
	}
	var cur *allowEntry
	flush := func() {
		if cur == nil {
			return
		}
		switch {
		case cur.dialect != "sqlite" && cur.dialect != "postgres":
			bad(cur.line, fmt.Sprintf("dialect %q must be sqlite or postgres", cur.dialect))
		case cur.query == "":
			bad(cur.line, "entry without a query")
		case cur.reason == "":
			bad(cur.line, fmt.Sprintf("entry %s %s needs a reason", cur.dialect, cur.query))
		default:
			list.entries = append(list.entries, *cur)
		}
		cur = nil
	}
	for n, raw := range strings.Split(string(data), "\n") {
		line := n + 1
		text := strings.TrimRight(raw, " \t\r")
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "[]" || trimmed == "---" {
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "- "); ok {
			flush()
			cur = &allowEntry{line: line}
			trimmed = rest
		}
		m := yamlLine.FindStringSubmatch(trimmed)
		if m == nil || cur == nil {
			bad(line, fmt.Sprintf("cannot read %q: entries are \"- dialect:\", \"query:\" and \"reason:\"", trimmed))
			continue
		}
		switch m[1] {
		case "dialect":
			cur.dialect = m[2]
		case "query":
			cur.query = m[2]
		case "reason":
			cur.reason = m[2]
		}
	}
	flush()
	return list, findings, nil
}

// query is one named statement of a queries/*.sql file.
type query struct {
	file     string
	name     string
	line     int // line of the "-- name:" line
	scope    string
	body     string
	bodyLine int // file line of the first body line
}

var (
	nameLine  = regexp.MustCompile(`^--\s*name:\s*(\w+)`)
	scopeLine = regexp.MustCompile(`^--\s*orcon:scope(?:\s+(\S+))?`)
)

// readQueries reads every .sql file of dir.
func readQueries(dir string) ([]query, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []query
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out = append(out, splitQueries(path, string(data))...)
	}
	return out, nil
}

// splitQueries cuts a file at its "-- name:" lines. The "-- orcon:scope" line
// sits among the comment lines right after the name line.
func splitQueries(path, text string) []query {
	var out []query
	var cur *query
	var body []string
	finish := func() {
		if cur != nil {
			cur.body = strings.Join(body, "\n")
			out = append(out, *cur)
		}
		cur, body = nil, nil
	}
	inHeader := false
	for n, raw := range strings.Split(text, "\n") {
		line := n + 1
		if m := nameLine.FindStringSubmatch(strings.TrimSpace(raw)); m != nil {
			finish()
			cur = &query{file: path, name: m[1], line: line, bodyLine: line + 1}
			inHeader = true
			continue
		}
		if cur == nil {
			continue
		}
		trimmed := strings.TrimSpace(raw)
		if inHeader && strings.HasPrefix(trimmed, "--") {
			if m := scopeLine.FindStringSubmatch(trimmed); m != nil {
				cur.scope = m[1]
			}
			cur.bodyLine = line + 1
			continue
		}
		if trimmed != "" {
			inHeader = false
		}
		if len(body) == 0 && trimmed == "" {
			cur.bodyLine = line + 1
			continue
		}
		body = append(body, raw)
	}
	finish()
	return out
}

// scopedTablesOf returns the tables the migrations of dir leave in place, minus
// the instance tables. Migrations apply in file name order.
func scopedTablesOf(dir string, instance map[string]bool) (scopedTables, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return scopedTables{}, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	tables := map[string]bool{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		toks, err := lex(string(data), 1)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, name), err)
		}
		applyTableStatements(toks, tables)
	}
	out := scopedTables{}
	for t := range tables {
		if !instance[t] {
			out[t] = true
		}
	}
	return out, nil
}

// applyTableStatements records CREATE TABLE and DROP TABLE statements.
func applyTableStatements(toks []token, tables map[string]bool) {
	for i := 0; i < len(toks); i++ {
		verb := toks[i]
		if !verb.isWord("create") && !verb.isWord("drop") {
			continue
		}
		j := i + 1
		for j < len(toks) && (toks[j].isWord("temp") || toks[j].isWord("temporary") || toks[j].isWord("virtual")) {
			j++
		}
		if j >= len(toks) || !toks[j].isWord("table") {
			continue
		}
		j++
		for j < len(toks) && (toks[j].isWord("if") || toks[j].isWord("not") || toks[j].isWord("exists")) {
			j++
		}
		name := ""
		for j < len(toks) && toks[j].kind == kindWord {
			name = toks[j].text
			if j+2 < len(toks) && toks[j+1].isPunct(".") {
				j += 2
				continue
			}
			break
		}
		if name == "" {
			continue
		}
		if verb.isWord("create") {
			tables[name] = true
		} else {
			delete(tables, name)
		}
	}
}

// outsideAllowed are the repository directories where .sql files may live.
var outsideAllowed = []string{
	"internal/adapter/sqlite/",
	"internal/adapter/postgres/",
	"internal/adapter/storetest/",
	"tools/",   // test data of the tools
	".agents/", // skill templates
	".claude/", // generated copy of the skills
}

// skipDirs are never walked.
var skipDirs = map[string]bool{".git": true, "node_modules": true, ".bin": true, "bin": true, "dist": true, ".turbo": true, ".pnpm-store": true}

// checkOutsideSQL reports the .sql files that live outside the data-access layer.
func checkOutsideSQL(root string) ([]Finding, error) {
	var out []Finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".sql") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for _, prefix := range outsideAllowed {
			if strings.HasPrefix(rel, prefix) {
				return nil
			}
		}
		out = append(out, Finding{File: path, Line: 1, Rule: "SQL-1",
			Msg: "SQL file outside internal/adapter/sqlite and internal/adapter/postgres; SQL lives only in the data-access layer (orcon-db TI-2)"})
		return nil
	})
	return out, err
}
