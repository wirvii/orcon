// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// normalizeForParser rewrites a statement of either dialect into text the
// PostgreSQL parser reads, without changing its meaning for the isolation
// check: every parameter spelling ("?", "?3", ":name", "@name", sqlc.arg(...),
// sqlc.narg(...), sqlc.slice(...)) becomes $n, and "INSERT OR IGNORE" becomes
// "INSERT" (the conflict clause stays visible to the check through ON CONFLICT
// only). Line breaks are kept, so a position in the result is a line of the
// file. Text inside strings, quoted identifiers and comments is left alone.
func normalizeForParser(src, dialect string) string {
	var out strings.Builder
	rs := []rune(src)
	next := 0
	param := func() string {
		next++
		return fmt.Sprintf("$%d", next)
	}
	for i := 0; i < len(rs); {
		c := rs[i]
		switch {
		case c == '-' && i+1 < len(rs) && rs[i+1] == '-':
			for i < len(rs) && rs[i] != '\n' {
				out.WriteRune(' ')
				i++
			}
		case c == '/' && i+1 < len(rs) && rs[i+1] == '*':
			i = copyBlockComment(&out, rs, i)
		case c == '\'' || c == '"' || c == '`':
			i = copyQuoted(&out, rs, i)
		case c == '?' && dialect == "sqlite":
			i++
			for i < len(rs) && unicode.IsDigit(rs[i]) {
				i++
			}
			out.WriteString(param())
		case (c == '@' || (c == ':' && dialect == "sqlite")) && i+1 < len(rs) && (unicode.IsLetter(rs[i+1]) || rs[i+1] == '_') && !(i > 0 && (rs[i-1] == c || rs[i-1] == ':')):
			i++
			for i < len(rs) && (unicode.IsLetter(rs[i]) || unicode.IsDigit(rs[i]) || rs[i] == '_') {
				i++
			}
			out.WriteString(param())
		case c == 's' && hasSQLCPrefix(rs, i):
			i = skipSQLCCall(&out, rs, i, param())
		default:
			out.WriteRune(c)
			i++
		}
	}
	return out.String()
}

func copyBlockComment(out *strings.Builder, rs []rune, i int) int {
	for i < len(rs) {
		if rs[i] == '*' && i+1 < len(rs) && rs[i+1] == '/' {
			out.WriteString("  ")
			return i + 2
		}
		if rs[i] == '\n' {
			out.WriteRune('\n')
		} else {
			out.WriteRune(' ')
		}
		i++
	}
	return i
}

func copyQuoted(out *strings.Builder, rs []rune, i int) int {
	q := rs[i]
	out.WriteRune(q)
	i++
	for i < len(rs) {
		out.WriteRune(rs[i])
		if rs[i] == q {
			if i+1 < len(rs) && rs[i+1] == q {
				out.WriteRune(q)
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return i
}

var sqlcCall = regexp.MustCompile(`^sqlc\s*\.\s*(arg|narg|slice)\s*\(`)

func hasSQLCPrefix(rs []rune, i int) bool {
	if i > 0 && (unicode.IsLetter(rs[i-1]) || unicode.IsDigit(rs[i-1]) || rs[i-1] == '_' || rs[i-1] == '.') {
		return false
	}
	end := min(i+40, len(rs))
	return sqlcCall.MatchString(string(rs[i:end]))
}

// skipSQLCCall replaces sqlc.arg(...) by the parameter text, keeping line breaks.
func skipSQLCCall(out *strings.Builder, rs []rune, i int, replacement string) int {
	depth := 0
	breaks := 0
	for ; i < len(rs); i++ {
		switch rs[i] {
		case '(':
			depth++
		case ')':
			depth--
		case '\n':
			breaks++
		}
		if depth == 0 && rs[i] == ')' {
			i++
			break
		}
	}
	out.WriteString(replacement)
	out.WriteString(strings.Repeat("\n", breaks))
	return i
}

var replaceStatement = regexp.MustCompile(`(?is)^\s*(insert\s+or\s+replace\b|replace\s+into\b)`)
var insertOr = regexp.MustCompile(`(?is)^(\s*insert)\s+or\s+(ignore|abort|fail|rollback)\b`)

// replacesRows reports SQLite's REPLACE, which deletes any row that conflicts on
// a unique key and so can remove a row the statement never named.
func replacesRows(normalized string) bool { return replaceStatement.MatchString(normalized) }

// dropInsertOr turns "INSERT OR IGNORE INTO" into "INSERT INTO".
func dropInsertOr(normalized string) string { return insertOr.ReplaceAllString(normalized, "$1") }
