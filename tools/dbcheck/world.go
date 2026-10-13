// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// config says where the files to check are.
type config struct {
	// Dialect adapter directories: each holds migrations/ and queries/.
	SQLiteDir   string
	PostgresDir string
	PolicyDir   string
	// Base is the revision migrations.sum is compared with ("" = resolve).
	Base string
	// Git runs git; tests replace it. A nil Git means git is unavailable.
	Git gitRunner
	Now func() time.Time
}

// migration is one parsed migration file.
type migration struct {
	dialect string
	path    string // as shown in findings
	name    string
	number  int // 0 when the name has no valid number
	data    []byte
	toks    []token // with comments
	stmts   []stmt
	header  header
}

// header is the "-- orcon:" block at the top of a migration (MG-2).
type header struct {
	kind      string
	floor     string
	contracts string
	noTx      bool
	kindLine  int
	floorLine int
	// problems found while reading the header (unknown or duplicated keys).
	problems []Finding
}

type annotation struct {
	key  string
	arg  string
	line int
}

// query is one sqlc query of a dialect's query files.
type query struct {
	dialect string
	file    string
	name    string
	cmd     string
	line    int
	annos   []annotation
	toks    []token // code tokens of the statement
}

func (q *query) anno(key string) (annotation, bool) {
	for _, a := range q.annos {
		if a.key == key {
			return a, true
		}
	}
	return annotation{}, false
}

// dialectState holds everything read for one dialect.
type dialectState struct {
	name       string
	dir        string
	migrations []*migration
	queries    []*query
}

func (d *dialectState) migrationsDir() string { return filepath.Join(d.dir, "migrations") }
func (d *dialectState) queriesDir() string    { return filepath.Join(d.dir, "queries") }
func (d *dialectState) sumPath() string       { return filepath.Join(d.migrationsDir(), "migrations.sum") }

// world is the parsed input of every check.
type world struct {
	cfg               config
	dialects          []*dialectState
	policy            *policy
	policyFindings    []Finding
	exc               *exceptions
	exceptionFindings []Finding
	loadFindings      []Finding // files that could not be read or lexed
	notes             []string
}

func (w *world) note(format string, args ...any) {
	w.notes = append(w.notes, fmt.Sprintf(format, args...))
}

var migrationNameRE = regexp.MustCompile(`^(\d{4})_([a-z][a-z0-9]*(?:_[a-z0-9]+)*)\.sql$`)

// loadWorld reads and parses every input. Problems are findings, so a broken
// input is reported by "dbcheck all" rather than aborting it.
func loadWorld(cfg config) *world {
	w := &world{cfg: cfg}
	now := time.Now
	if cfg.Now != nil {
		now = cfg.Now
	}
	w.policy, w.policyFindings = loadPolicy(cfg.PolicyDir)
	w.exc, w.exceptionFindings = loadExceptions(cfg.PolicyDir, now())
	for _, d := range []struct{ name, dir string }{{dialectSQLite, cfg.SQLiteDir}, {dialectPostgres, cfg.PostgresDir}} {
		ds := &dialectState{name: d.name, dir: d.dir}
		w.loadMigrations(ds)
		w.loadQueries(ds)
		w.dialects = append(w.dialects, ds)
	}
	return w
}

func (w *world) dialect(name string) *dialectState {
	for _, d := range w.dialects {
		if d.name == name {
			return d
		}
	}
	return nil
}

func (w *world) loadMigrations(ds *dialectState) {
	dir := ds.migrationsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		w.loadFindings = append(w.loadFindings, Finding{File: dir, Line: 1, Rule: "MG-1", Msg: "migrations directory cannot be read: " + err.Error(), Fix: "create " + dir + " with its migrations.sum"})
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path) //nolint:gosec // path comes from the directory listing of a developer tool
		if err != nil {
			w.loadFindings = append(w.loadFindings, Finding{File: path, Line: 1, Rule: "MG-1", Msg: "migration cannot be read: " + err.Error()})
			continue
		}
		m := &migration{dialect: ds.name, path: path, name: e.Name(), data: data}
		if sub := migrationNameRE.FindStringSubmatch(e.Name()); sub != nil {
			m.number, _ = strconv.Atoi(sub[1])
		}
		toks, err := lex(string(data), ds.name)
		if err != nil {
			w.loadFindings = append(w.loadFindings, Finding{File: path, Line: 1, Rule: "MG-2", Msg: "SQL cannot be read: " + err.Error(), Fix: "correct the SQL"})
			ds.migrations = append(ds.migrations, m)
			continue
		}
		m.toks = toks
		m.stmts = splitStatements(codeTokens(toks))
		m.header = parseHeader(toks)
		ds.migrations = append(ds.migrations, m)
	}
	sort.Slice(ds.migrations, func(i, j int) bool { return ds.migrations[i].name < ds.migrations[j].name })
}

// parseHeader reads the "-- orcon:" comments that come before the first code
// token of a migration.
func parseHeader(toks []token) header {
	var h header
	seen := map[string]int{}
	for _, t := range toks {
		if t.kind != tokComment {
			break
		}
		rest, ok := strings.CutPrefix(t.text, "orcon:")
		if !ok {
			continue
		}
		key, arg, _ := strings.Cut(rest, " ")
		arg = strings.TrimSpace(arg)
		if first, dup := seen[key]; dup {
			h.problems = append(h.problems, Finding{Line: t.line, Rule: "MG-2", Msg: fmt.Sprintf("header key %q repeated (first on line %d)", key, first), Fix: "keep one line per key"})
			continue
		}
		seen[key] = t.line
		switch key {
		case "kind":
			h.kind, h.kindLine = arg, t.line
		case "compatibility-floor":
			h.floor, h.floorLine = arg, t.line
		case "contracts":
			h.contracts = arg
		case "no-transaction":
			h.noTx = true
		default:
			h.problems = append(h.problems, Finding{Line: t.line, Rule: "MG-2", Msg: fmt.Sprintf("unknown header key %q", key), Fix: "use kind, compatibility-floor, contracts or no-transaction"})
		}
	}
	return h
}

var queryNameRE = regexp.MustCompile(`^name:\s+(\S+)\s+(:\S+)\s*$`)

var validCmds = map[string]bool{
	":one": true, ":many": true, ":exec": true, ":execrows": true, ":execresult": true,
	":execlastid": true, ":batchone": true, ":batchmany": true, ":batchexec": true, ":copyfrom": true,
}

func (w *world) loadQueries(ds *dialectState) {
	dir := ds.queriesDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// The queries directory appears with the first adapter slice; a
			// tree without queries has nothing to check.
			w.note("%s: no queries directory yet; query checks have nothing to check for %s", dir, ds.name)
			return
		}
		w.loadFindings = append(w.loadFindings, Finding{File: dir, Line: 1, Rule: "QY-1", Msg: "queries directory cannot be read: " + err.Error()})
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path) //nolint:gosec // path comes from the directory listing of a developer tool
		if err != nil {
			w.loadFindings = append(w.loadFindings, Finding{File: path, Line: 1, Rule: "QY-1", Msg: "query file cannot be read: " + err.Error()})
			continue
		}
		toks, err := lex(string(data), ds.name)
		if err != nil {
			w.loadFindings = append(w.loadFindings, Finding{File: path, Line: 1, Rule: "QY-1", Msg: "SQL cannot be read: " + err.Error(), Fix: "correct the SQL"})
			continue
		}
		qs, fs := parseQueries(ds.name, path, toks)
		ds.queries = append(ds.queries, qs...)
		w.loadFindings = append(w.loadFindings, fs...)
	}
	sort.SliceStable(ds.queries, func(i, j int) bool {
		if ds.queries[i].file != ds.queries[j].file {
			return ds.queries[i].file < ds.queries[j].file
		}
		return ds.queries[i].line < ds.queries[j].line
	})
}

// parseQueries cuts a sqlc query file into queries. A query starts at its
// "-- name: <Name> :<cmd>" comment and ends at the ";" that closes its
// statement. Its "-- orcon:" annotations are the comments between the name
// line and the end of the statement.
func parseQueries(dialect, path string, toks []token) ([]*query, []Finding) {
	var out []*query
	var fs []Finding
	var cur *query
	closed, reported := false, false
	for _, t := range toks {
		if t.kind == tokComment {
			if m := queryNameRE.FindStringSubmatch(t.text); m != nil {
				cur = &query{dialect: dialect, file: path, name: m[1], cmd: m[2], line: t.line}
				closed, reported = false, false
				if !validCmds[m[2]] {
					fs = append(fs, Finding{File: path, Line: t.line, Rule: "QY-9", Msg: fmt.Sprintf("query %s has unknown command %q", m[1], m[2]), Fix: "use :one, :many, :exec, :execrows, :execresult, :batchone, :batchmany, :batchexec or :copyfrom"})
				}
				out = append(out, cur)
				continue
			}
			if strings.HasPrefix(t.text, "name:") {
				fs = append(fs, Finding{File: path, Line: t.line, Rule: "QY-9", Msg: fmt.Sprintf("malformed query name line %q", t.text), Fix: "write \"-- name: <Name> :<command>\""})
				continue
			}
			if rest, ok := strings.CutPrefix(t.text, "orcon:"); ok && cur != nil && !closed {
				key, arg, _ := strings.Cut(rest, " ")
				cur.annos = append(cur.annos, annotation{key: key, arg: strings.TrimSpace(arg), line: t.line})
			}
			continue
		}
		if cur == nil {
			fs = append(fs, Finding{File: path, Line: t.line, Rule: "QY-9", Msg: "SQL before the first \"-- name:\" line", Fix: "start every query with \"-- name: <Name> :<command>\""})
			return out, fs
		}
		if closed {
			if !reported {
				fs = append(fs, Finding{File: path, Line: t.line, Rule: "QY-9", Msg: fmt.Sprintf("SQL after the end of query %s without a new \"-- name:\" line", cur.name), Fix: "one statement per query"})
				reported = true
			}
			continue
		}
		if t.isPunct(";") && t.depth == 0 {
			closed = true
			continue
		}
		cur.toks = append(cur.toks, t)
	}
	return out, fs
}
