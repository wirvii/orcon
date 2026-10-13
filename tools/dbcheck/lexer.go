// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tokKind classifies one lexical token of an SQL file.
type tokKind int

const (
	tokWord    tokKind = iota // keyword or unquoted identifier
	tokQuoted                 // "identifier", `identifier` or (SQLite) [identifier]
	tokString                 // 'text', E'text' or a PostgreSQL dollar-quoted body
	tokNumber                 // numeric literal
	tokParam                  // $1, ?, ?1, :name, @name, $name
	tokOp                     // operator run such as -> ->> #>> @? ?| *
	tokPunct                  // ( ) , ; . :: [ ]
	tokComment                // -- line or /* block */ comment; text is the content
)

// token is one lexical element. Code tokens exclude comments; comments keep
// their trimmed content in text so annotations (-- orcon:...) can be read.
type token struct {
	kind  tokKind
	text  string // source text; for comments, the trimmed content
	lower string // lowercase text for words, the unquoted name for quoted identifiers
	line  int
	depth int // parenthesis depth; "(" and its matching ")" share one depth
}

func (t token) isWord(w string) bool { return t.kind == tokWord && t.lower == w }
func (t token) isPunct(p string) bool {
	return t.kind == tokPunct && t.text == p
}
func (t token) isOp(o string) bool { return t.kind == tokOp && t.text == o }

// name returns the identifier a word or quoted identifier stands for.
func (t token) name() (string, bool) {
	if t.kind == tokWord || t.kind == tokQuoted {
		return t.lower, true
	}
	return "", false
}

// lex splits SQL text into tokens. dialect is "sqlite" or "postgres": it
// decides what "?" (a parameter in SQLite, an operator in PostgreSQL), "$"
// (a parameter or a dollar-quoted string), "[" and ":" mean. Comments are kept
// in the stream so callers can read annotations; use codeTokens to drop them.
func lex(src, dialect string) ([]token, error) {
	l := &lexer{src: src, dialect: dialect, line: 1}
	if err := l.run(); err != nil {
		return nil, err
	}
	depth := 0
	for i := range l.toks {
		t := &l.toks[i]
		switch {
		case t.isPunct("("):
			t.depth = depth
			depth++
		case t.isPunct(")"):
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("line %d: unbalanced closing parenthesis", t.line)
			}
			t.depth = depth
		default:
			t.depth = depth
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("line %d: unbalanced parentheses", l.line)
	}
	return l.toks, nil
}

// codeTokens returns the tokens that are not comments.
func codeTokens(toks []token) []token {
	out := make([]token, 0, len(toks))
	for _, t := range toks {
		if t.kind != tokComment {
			out = append(out, t)
		}
	}
	return out
}

type lexer struct {
	src     string
	dialect string
	pos     int
	line    int
	toks    []token
}

func (l *lexer) emit(kind tokKind, text, lower string, line int) {
	l.toks = append(l.toks, token{kind: kind, text: text, lower: lower, line: line})
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || r >= 0x80
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r) || r == '$'
}

func isOpChar(dialect string, r rune) bool {
	if r == '?' {
		return dialect == dialectPostgres
	}
	return strings.ContainsRune("+-*/<>=~!@#%^&|", r)
}

func (l *lexer) peekRune(off int) (rune, int) {
	if l.pos+off >= len(l.src) {
		return 0, 0
	}
	return utf8.DecodeRuneInString(l.src[l.pos+off:])
}

func (l *lexer) run() error {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			l.line++
			l.pos++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
			l.pos++
		case c == '-' && strings.HasPrefix(l.src[l.pos:], "--"):
			l.lineComment()
		case c == '/' && strings.HasPrefix(l.src[l.pos:], "/*"):
			if err := l.blockComment(); err != nil {
				return err
			}
		case c == '\'':
			if err := l.quoted('\'', tokString, false); err != nil {
				return err
			}
		case c == '"':
			if err := l.quoted('"', tokQuoted, true); err != nil {
				return err
			}
		case c == '`':
			if err := l.quoted('`', tokQuoted, true); err != nil {
				return err
			}
		case c == '[' && l.dialect == dialectSQLite:
			if err := l.bracketIdent(); err != nil {
				return err
			}
		case c == '$':
			if err := l.dollar(); err != nil {
				return err
			}
		case c >= '0' && c <= '9':
			l.number()
		case c == '.' && l.pos+1 < len(l.src) && l.src[l.pos+1] >= '0' && l.src[l.pos+1] <= '9':
			l.number()
		case c == ':' && strings.HasPrefix(l.src[l.pos:], "::"):
			l.emit(tokPunct, "::", "::", l.line)
			l.pos += 2
		case c == ':' && l.dialect == dialectSQLite && l.nextIsIdentStart(1):
			l.param(1)
		case c == '@' && l.nextIsIdentStart(1):
			l.param(1)
		case c == '?' && l.dialect == dialectSQLite:
			start := l.pos
			l.pos++
			for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
				l.pos++
			}
			l.emit(tokParam, l.src[start:l.pos], l.src[start:l.pos], l.line)
		case strings.IndexByte("(),;.[]:", c) >= 0:
			l.emit(tokPunct, string(c), string(c), l.line)
			l.pos++
		default:
			r, _ := l.peekRune(0)
			switch {
			case isIdentStart(r):
				l.word()
			case isOpChar(l.dialect, r):
				l.operator()
			default:
				return fmt.Errorf("line %d: unexpected character %q", l.line, r)
			}
		}
	}
	return nil
}

func (l *lexer) nextIsIdentStart(off int) bool {
	r, size := l.peekRune(off)
	return size > 0 && isIdentStart(r)
}

func (l *lexer) lineComment() {
	start := l.pos + 2
	end := strings.IndexByte(l.src[start:], '\n')
	if end < 0 {
		end = len(l.src) - start
	}
	text := strings.TrimSpace(l.src[start : start+end])
	l.emit(tokComment, text, text, l.line)
	l.pos = start + end
}

func (l *lexer) blockComment() error {
	startLine := l.line
	depth := 0
	i := l.pos
	for i < len(l.src) {
		switch {
		case strings.HasPrefix(l.src[i:], "/*"):
			// PostgreSQL block comments nest; SQLite's do not.
			if l.dialect == dialectPostgres || depth == 0 {
				depth++
			}
			i += 2
		case strings.HasPrefix(l.src[i:], "*/"):
			depth--
			i += 2
			if depth == 0 {
				text := strings.TrimSpace(l.src[l.pos+2 : i-2])
				l.emit(tokComment, text, text, startLine)
				l.line += strings.Count(l.src[l.pos:i], "\n")
				l.pos = i
				return nil
			}
		default:
			i++
		}
	}
	return fmt.Errorf("line %d: unterminated block comment", startLine)
}

// quoted reads a '...' string or a "..." / `...` identifier; a doubled quote
// is an escaped quote. In a PostgreSQL E'...' string a backslash escapes the
// next character.
func (l *lexer) quoted(q byte, kind tokKind, ident bool) error {
	startLine := l.line
	escapes := false
	if kind == tokString && len(l.toks) > 0 && l.dialect == dialectPostgres {
		prev := l.toks[len(l.toks)-1]
		if prev.kind == tokWord && (prev.lower == "e") && l.pos > 0 && l.src[l.pos-1] != ' ' {
			escapes = true
			l.toks = l.toks[:len(l.toks)-1]
		}
	}
	start := l.pos
	i := l.pos + 1
	var b strings.Builder
	for i < len(l.src) {
		c := l.src[i]
		switch {
		case escapes && c == '\\' && i+1 < len(l.src):
			if l.src[i+1] == '\n' {
				l.line++
			}
			b.WriteByte(l.src[i+1])
			i += 2
		case c == q && i+1 < len(l.src) && l.src[i+1] == q:
			b.WriteByte(q)
			i += 2
		case c == q:
			i++
			text := l.src[start:i]
			lower := ""
			if ident {
				lower = b.String()
			}
			l.emit(kind, text, lower, startLine)
			l.pos = i
			return nil
		default:
			if c == '\n' {
				l.line++
			}
			b.WriteByte(c)
			i++
		}
	}
	return fmt.Errorf("line %d: unterminated quoted text", startLine)
}

func (l *lexer) bracketIdent() error {
	end := strings.IndexByte(l.src[l.pos:], ']')
	if end < 0 {
		return fmt.Errorf("line %d: unterminated [identifier]", l.line)
	}
	text := l.src[l.pos : l.pos+end+1]
	l.emit(tokQuoted, text, text[1:len(text)-1], l.line)
	l.pos += end + 1
	return nil
}

// dollar handles $1 / $name parameters and PostgreSQL dollar-quoted strings.
func (l *lexer) dollar() error {
	i := l.pos + 1
	for i < len(l.src) && l.src[i] >= '0' && l.src[i] <= '9' {
		i++
	}
	if i > l.pos+1 { // $1
		l.emit(tokParam, l.src[l.pos:i], l.src[l.pos:i], l.line)
		l.pos = i
		return nil
	}
	if l.dialect == dialectPostgres {
		// $tag$ ... $tag$
		j := l.pos + 1
		for j < len(l.src) && (isIdentPart(rune(l.src[j])) && l.src[j] != '$') {
			j++
		}
		if j < len(l.src) && l.src[j] == '$' {
			tag := l.src[l.pos : j+1]
			end := strings.Index(l.src[j+1:], tag)
			if end < 0 {
				return fmt.Errorf("line %d: unterminated dollar-quoted string %s", l.line, tag)
			}
			stop := j + 1 + end + len(tag)
			l.emit(tokString, l.src[l.pos:stop], "", l.line)
			l.line += strings.Count(l.src[l.pos:stop], "\n")
			l.pos = stop
			return nil
		}
		return fmt.Errorf("line %d: unexpected '$'", l.line)
	}
	if l.nextIsIdentStart(1) {
		l.param(1)
		return nil
	}
	return fmt.Errorf("line %d: unexpected '$'", l.line)
}

func (l *lexer) param(skip int) {
	start := l.pos
	l.pos += skip
	for l.pos < len(l.src) {
		r, size := l.peekRune(0)
		if !isIdentPart(r) {
			break
		}
		l.pos += size
	}
	l.emit(tokParam, l.src[start:l.pos], l.src[start:l.pos], l.line)
}

func (l *lexer) number() {
	start := l.pos
	if strings.HasPrefix(l.src[l.pos:], "0x") || strings.HasPrefix(l.src[l.pos:], "0X") {
		l.pos += 2
		for l.pos < len(l.src) && strings.IndexByte("0123456789abcdefABCDEF", l.src[l.pos]) >= 0 {
			l.pos++
		}
		l.emit(tokNumber, l.src[start:l.pos], l.src[start:l.pos], l.line)
		return
	}
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c >= '0' && c <= '9', c == '.':
			l.pos++
		case (c == 'e' || c == 'E') && l.exponentFollows():
			l.pos += 2
		default:
			l.emit(tokNumber, l.src[start:l.pos], l.src[start:l.pos], l.line)
			return
		}
	}
	l.emit(tokNumber, l.src[start:l.pos], l.src[start:l.pos], l.line)
}

// exponentFollows reports whether the "e" at the current position starts a
// numeric exponent (a digit or a signed digit follows).
func (l *lexer) exponentFollows() bool {
	rest := l.src[l.pos+1:]
	if rest == "" {
		return false
	}
	if rest[0] >= '0' && rest[0] <= '9' {
		return true
	}
	return (rest[0] == '+' || rest[0] == '-') && len(rest) > 1 && rest[1] >= '0' && rest[1] <= '9'
}

func (l *lexer) word() {
	start := l.pos
	for l.pos < len(l.src) {
		r, size := l.peekRune(0)
		if !isIdentPart(r) {
			break
		}
		l.pos += size
	}
	text := l.src[start:l.pos]
	l.emit(tokWord, text, strings.ToLower(text), l.line)
}

// operator reads a maximal run of operator characters, stopping before a
// comment opener so "a --comment" and "a /* c */" lex as the author meant.
func (l *lexer) operator() {
	start := l.pos
	for l.pos < len(l.src) {
		r, size := l.peekRune(0)
		if !isOpChar(l.dialect, r) {
			break
		}
		rest := l.src[l.pos:]
		if l.pos > start && (strings.HasPrefix(rest, "--") || strings.HasPrefix(rest, "/*")) {
			break
		}
		l.pos += size
	}
	text := l.src[start:l.pos]
	l.emit(tokOp, text, text, l.line)
}

// stmt is one SQL statement of a file: its code tokens without the final ";".
type stmt struct {
	toks []token
	line int
}

// words returns the lowercase words and identifier names of the statement,
// without literals, operators and punctuation.
func (s stmt) words() []string {
	out := make([]string, 0, len(s.toks))
	for _, t := range s.toks {
		if t.kind == tokWord || t.kind == tokQuoted {
			out = append(out, t.lower)
		}
	}
	return out
}

// splitStatements cuts the code tokens at every ";" outside parentheses. A
// CREATE TRIGGER body (BEGIN ... END) keeps its inner semicolons.
func splitStatements(code []token) []stmt {
	var out []stmt
	var cur []token
	trigger, inBody, nest := false, false, 0
	flush := func() {
		if len(cur) > 0 {
			out = append(out, stmt{toks: cur, line: cur[0].line})
		}
		cur, trigger, inBody, nest = nil, false, false, 0
	}
	for _, t := range code {
		if t.isPunct(";") && t.depth == 0 && (!trigger || !inBody || nest < 0) {
			flush()
			continue
		}
		cur = append(cur, t)
		if len(cur) == 3 && cur[0].isWord("create") {
			for _, c := range cur[1:] {
				if c.isWord("trigger") {
					trigger = true
				}
			}
		}
		if trigger && t.kind == tokWord {
			switch t.lower {
			case "begin":
				inBody = true
			case "case":
				if inBody {
					nest++
				}
			case "end":
				if inBody {
					if nest > 0 {
						nest--
					} else {
						nest = -1
					}
				}
			}
		}
	}
	flush()
	return out
}
