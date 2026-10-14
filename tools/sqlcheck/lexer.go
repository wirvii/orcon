// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
	"unicode"
)

// kind classifies a lexical token.
type kind int

const (
	kindWord   kind = iota // keyword or identifier (quoted identifiers included, unquoted)
	kindString             // 'text'
	kindNumber
	kindParam // $1, ?, ?1, :name, @name, sqlc.arg(...), sqlc.narg(...)
	kindOp
	kindPunct // ( ) , ; . ::
)

// token is one lexical element of a statement. Comments are dropped.
type token struct {
	kind kind
	text string // lowercase for words
	line int    // line of the file
}

func (t token) isWord(w string) bool  { return t.kind == kindWord && t.text == w }
func (t token) isPunct(p string) bool { return t.kind == kindPunct && t.text == p }
func (t token) isOp(o string) bool    { return t.kind == kindOp && t.text == o }

// lex splits one statement into tokens. firstLine is the file line of the first
// character. sqlc.arg(x), sqlc.narg(x) and sqlc.slice(x) become one parameter
// token, which is how sqlc spells a named parameter.
func lex(src string, firstLine int) ([]token, error) {
	l := &lexer{src: []rune(src), line: firstLine}
	if err := l.run(); err != nil {
		return nil, err
	}
	return mergeSQLCParams(l.toks), nil
}

type lexer struct {
	src  []rune
	pos  int
	line int
	toks []token
}

func (l *lexer) emit(k kind, text string, line int) {
	l.toks = append(l.toks, token{kind: k, text: text, line: line})
}

func (l *lexer) peek(off int) rune {
	if l.pos+off >= len(l.src) {
		return 0
	}
	return l.src[l.pos+off]
}

func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isIdentPart(r rune) bool  { return isIdentStart(r) || unicode.IsDigit(r) || r == '$' }

func (l *lexer) run() error {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			l.line++
			l.pos++
		case unicode.IsSpace(c):
			l.pos++
		case c == '-' && l.peek(1) == '-':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case c == '/' && l.peek(1) == '*':
			if err := l.blockComment(); err != nil {
				return err
			}
		case c == '\'':
			if err := l.quoted('\'', kindString); err != nil {
				return err
			}
		case c == '"' || c == '`':
			if err := l.quoted(c, kindWord); err != nil {
				return err
			}
		case c == '$' && unicode.IsDigit(l.peek(1)):
			l.number('$', kindParam)
		case c == '?':
			l.number('?', kindParam)
		case (c == ':' && isIdentStart(l.peek(1))) || (c == '@' && isIdentStart(l.peek(1))):
			l.namedParam()
		case c == ':' && l.peek(1) == ':':
			l.emit(kindPunct, "::", l.line)
			l.pos += 2
		case strings.ContainsRune("(),;.", c):
			l.emit(kindPunct, string(c), l.line)
			l.pos++
		case unicode.IsDigit(c):
			l.number(0, kindNumber)
		case isIdentStart(c):
			start := l.pos
			for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
				l.pos++
			}
			l.emit(kindWord, strings.ToLower(string(l.src[start:l.pos])), l.line)
		default:
			l.operator()
		}
	}
	return nil
}

func (l *lexer) blockComment() error {
	startLine := l.line
	l.pos += 2
	for l.pos+1 < len(l.src) {
		if l.src[l.pos] == '*' && l.src[l.pos+1] == '/' {
			l.pos += 2
			return nil
		}
		if l.src[l.pos] == '\n' {
			l.line++
		}
		l.pos++
	}
	return fmt.Errorf("line %d: unterminated block comment", startLine)
}

// quoted reads 'text', "identifier" or `identifier`; a doubled quote is an escape.
func (l *lexer) quoted(q rune, k kind) error {
	startLine := l.line
	l.pos++
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == q {
			if l.peek(1) == q {
				b.WriteRune(q)
				l.pos += 2
				continue
			}
			l.pos++
			text := b.String()
			if k == kindWord {
				text = strings.ToLower(text)
			}
			l.emit(k, text, startLine)
			return nil
		}
		if c == '\n' {
			l.line++
		}
		b.WriteRune(c)
		l.pos++
	}
	return fmt.Errorf("line %d: unterminated quoted text", startLine)
}

// number reads digits after an optional one-character prefix.
func (l *lexer) number(prefix rune, k kind) {
	start := l.pos
	if prefix != 0 {
		l.pos++
	}
	for l.pos < len(l.src) && (unicode.IsDigit(l.src[l.pos]) || (prefix == 0 && (l.src[l.pos] == '.' || l.src[l.pos] == 'e'))) {
		l.pos++
	}
	l.emit(k, string(l.src[start:l.pos]), l.line)
}

func (l *lexer) namedParam() {
	start := l.pos
	l.pos++
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		l.pos++
	}
	l.emit(kindParam, string(l.src[start:l.pos]), l.line)
}

func (l *lexer) operator() {
	start := l.pos
	for l.pos < len(l.src) && strings.ContainsRune("+-*/<>=~!@#%^&|[]", l.src[l.pos]) {
		l.pos++
	}
	if l.pos == start {
		l.pos++ // an unknown character: keep it as an operator so the scan goes on
	}
	l.emit(kindOp, string(l.src[start:l.pos]), l.line)
}

// mergeSQLCParams folds sqlc . arg|narg|slice ( ... ) into one parameter token.
func mergeSQLCParams(toks []token) []token {
	out := make([]token, 0, len(toks))
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.isWord("sqlc") && i+3 < len(toks) && toks[i+1].isPunct(".") && toks[i+3].isPunct("(") {
			switch toks[i+2].text {
			case "arg", "narg", "slice":
				depth, j := 0, i+3
				for ; j < len(toks); j++ {
					if toks[j].isPunct("(") {
						depth++
					}
					if toks[j].isPunct(")") {
						depth--
						if depth == 0 {
							break
						}
					}
				}
				out = append(out, token{kind: kindParam, text: "sqlc." + toks[i+2].text, line: t.line})
				i = j
				continue
			}
		}
		out = append(out, t)
	}
	return out
}
