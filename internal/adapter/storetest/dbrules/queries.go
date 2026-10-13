// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Query is one sqlc query read from a dialect's query files.
type Query struct {
	// Name is the sqlc query name.
	Name string
	// Command is the sqlc command, for example ":many".
	Command string
	// File and Line say where the query is.
	File string
	Line int
	// SQL is the statement with sqlc named parameters turned into the dialect's
	// positional markers (? for SQLite, $n for PostgreSQL), without the final ";".
	SQL string
	// Hot is true when the query carries -- orcon:hot.
	Hot bool
	// Ordered is true when the statement has a top-level ORDER BY.
	Ordered bool
}

// List reports whether the query returns several rows (:many or :batchmany).
func (q Query) List() bool { return q.Command == ":many" || q.Command == ":batchmany" }

var nameLineRE = regexp.MustCompile(`^--\s*name:\s+(\S+)\s+(:\S+)\s*$`)

// LoadQueries reads every *.sql file of dir in fsys. The dialect is DialectSQLite
// or DialectPostgres and decides the parameter markers. Only the structure the
// plan checks need is read; the style rules of the queries are tools/dbcheck's.
func LoadQueries(fsys fs.FS, dir, dialect string) ([]Query, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("dbrules: queries directory %s: %w", dir, err)
	}
	var out []Query
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		file := path.Join(dir, e.Name())
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, fmt.Errorf("dbrules: %w", err)
		}
		qs, err := parseQueryFile(file, string(data), dialect)
		if err != nil {
			return nil, err
		}
		out = append(out, qs...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}

func parseQueryFile(file, src, dialect string) ([]Query, error) {
	var out []Query
	var cur *Query
	var body []string
	flush := func() error {
		if cur == nil {
			return nil
		}
		sqlText := strings.TrimSpace(strings.Join(body, "\n"))
		sqlText = strings.TrimSpace(strings.TrimSuffix(sqlText, ";"))
		if sqlText == "" {
			return fmt.Errorf("dbrules: %s:%d: query %s has no statement", file, cur.Line, cur.Name)
		}
		converted, err := positionalParameters(sqlText, dialect)
		if err != nil {
			return fmt.Errorf("dbrules: %s:%d: query %s: %w", file, cur.Line, cur.Name, err)
		}
		cur.SQL = converted
		toks, err := tokenize(converted)
		if err != nil {
			return fmt.Errorf("dbrules: %s:%d: query %s: %w", file, cur.Line, cur.Name, err)
		}
		cur.Ordered = hasTopLevelOrderBy(toks)
		out = append(out, *cur)
		cur, body = nil, nil
		return nil
	}
	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := nameLineRE.FindStringSubmatch(trimmed); m != nil {
			if err := flush(); err != nil {
				return nil, err
			}
			cur = &Query{Name: m[1], Command: m[2], File: file, Line: i + 1}
			continue
		}
		if cur == nil {
			continue
		}
		if rest, ok := strings.CutPrefix(trimmed, "--"); ok {
			if strings.TrimSpace(rest) == "orcon:hot" {
				cur.Hot = true
			}
			continue
		}
		body = append(body, stripTrailingComment(line))
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}

// stripTrailingComment removes a "-- ..." comment that follows SQL on a line,
// leaving quoted text alone.
func stripTrailingComment(line string) string {
	var quote byte
	for i := range len(line) {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '-' && i+1 < len(line) && line[i+1] == '-':
			return strings.TrimRight(line[:i], " \t")
		}
	}
	return line
}

func hasTopLevelOrderBy(toks []sqlToken) bool {
	for i := 0; i+1 < len(toks); i++ {
		if toks[i].depth == 0 && toks[i].isWord("order") && toks[i+1].isWord("by") {
			return true
		}
	}
	return false
}

var sqlcArgRE = regexp.MustCompile(`sqlc\.(?:n?arg|slice)\(\s*'?"?(\w+)'?"?\s*\)`)

// positionalParameters turns sqlc named parameters (@name and sqlc.arg(name))
// into positional markers. SQLite gets "?" for each; PostgreSQL gets "$n",
// numbered after the highest explicit $n, one number per distinct name.
func positionalParameters(sqlText, dialect string) (string, error) {
	sqlText = sqlcArgRE.ReplaceAllString(sqlText, "@$1")
	toks, err := tokenize(sqlText)
	if err != nil {
		return "", err
	}
	next := 1
	for _, t := range toks {
		if t.kind == 'n' && strings.HasPrefix(t.text, "$") && len(t.text) > 1 {
			var n int
			if _, err := fmt.Sscanf(t.text, "$%d", &n); err == nil && n >= next {
				next = n + 1
			}
		}
	}
	var b strings.Builder
	last := 0
	numbered := map[string]int{}
	for i, t := range toks {
		if !t.isPunct("@") || i+1 >= len(toks) || toks[i+1].kind != 'w' || toks[i+1].start != t.end {
			continue
		}
		name := toks[i+1].lower
		b.WriteString(sqlText[last:t.start])
		if dialect == DialectSQLite {
			b.WriteString("?")
		} else {
			n, seen := numbered[name]
			if !seen {
				n = next
				next++
				numbered[name] = n
			}
			fmt.Fprintf(&b, "$%d", n)
		}
		last = toks[i+1].end
	}
	b.WriteString(sqlText[last:])
	return b.String(), nil
}

// placeholderCount returns how many positional parameters a SQLite statement
// takes: the number of "?" markers, or the highest "?N".
func placeholderCount(statement string) int {
	toks, err := tokenize(statement)
	if err != nil {
		return 0
	}
	auto, highest := 0, 0
	for i, t := range toks {
		if !t.isPunct("?") {
			continue
		}
		if i+1 < len(toks) && toks[i+1].kind == 'n' && toks[i+1].start == t.end {
			if n, err := strconv.Atoi(toks[i+1].text); err == nil && n > highest {
				highest = n
			}
			continue
		}
		auto++
	}
	return max(auto, highest)
}
