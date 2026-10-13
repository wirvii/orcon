// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dbrules

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// sqlToken is one token of SQL text read from the catalog or from a file.
type sqlToken struct {
	kind  byte   // 'w' word, 'q' quoted identifier, 's' string, 'n' number, 'p' punctuation or operator
	text  string // source text
	lower string // lowercase word, or the unquoted identifier
	depth int    // parenthesis depth; "(" and its matching ")" share one depth
	line  int
	start int // byte offset of the token in the source
	end   int // byte offset after the token
}

func (t sqlToken) isWord(w string) bool { return t.kind == 'w' && t.lower == w }
func (t sqlToken) isPunct(p string) bool {
	return t.kind == 'p' && t.text == p
}

// ident returns the identifier a word or quoted identifier stands for.
func (t sqlToken) ident() (string, bool) {
	if t.kind == 'w' || t.kind == 'q' {
		return t.lower, true
	}
	return "", false
}

var errUnterminated = errors.New("unterminated quoted text or comment")

// tokenize splits SQL text into tokens, dropping comments. It understands
// single-quoted strings, double-quoted, backtick and bracketed identifiers and
// PostgreSQL dollar-quoted strings.
func tokenize(src string) ([]sqlToken, error) {
	return tokenizeText(src, true)
}

// tokenizePostgres is tokenize for PostgreSQL text, where "[" and "]" delimit
// array subscripts and ARRAY[...] and never quote an identifier.
func tokenizePostgres(src string) ([]sqlToken, error) {
	return tokenizeText(src, false)
}

func tokenizeText(src string, brackets bool) ([]sqlToken, error) {
	s := &scanner{src: src, line: 1, brackets: brackets}
	for s.pos < len(s.src) {
		if err := s.next(); err != nil {
			return nil, fmt.Errorf("line %d: %w", s.line, err)
		}
	}
	if s.depth != 0 {
		return nil, fmt.Errorf("line %d: unbalanced parentheses", s.line)
	}
	return s.toks, nil
}

type scanner struct {
	src   string
	pos   int
	line  int
	depth int
	toks  []sqlToken
	// brackets makes [name] an identifier (SQLite).
	brackets bool
}

func (s *scanner) add(kind byte, start, end int, lower string) {
	s.toks = append(s.toks, sqlToken{kind: kind, text: s.src[start:end], lower: lower, depth: s.depth, line: s.line, start: start, end: end})
}

// next reads one token, comment or run of white space.
func (s *scanner) next() error {
	c := s.src[s.pos]
	switch {
	case c == '\n':
		s.line++
		s.pos++
	case c == ' ' || c == '\t' || c == '\r' || c == '\f':
		s.pos++
	case strings.HasPrefix(s.src[s.pos:], "--"):
		if end := strings.IndexByte(s.src[s.pos:], '\n'); end < 0 {
			s.pos = len(s.src)
		} else {
			s.pos += end
		}
	case strings.HasPrefix(s.src[s.pos:], "/*"):
		end := strings.Index(s.src[s.pos+2:], "*/")
		if end < 0 {
			return errUnterminated
		}
		s.line += strings.Count(s.src[s.pos:s.pos+2+end+2], "\n")
		s.pos += 2 + end + 2
	case c == '\'' || c == '"' || c == '`':
		return s.quoted(c)
	case c == '[' && s.brackets:
		return s.bracketed()
	case c == '$':
		s.dollar()
	case c >= '0' && c <= '9':
		s.run('n', func(b byte) bool { return b >= '0' && b <= '9' || b == '.' })
	case c == '_' || c >= 0x80 || unicode.IsLetter(rune(c)):
		s.run('w', isWordByte)
	default:
		return s.punct(c)
	}
	return nil
}

func isWordByte(b byte) bool {
	return b == '_' || b == '$' || b >= 0x80 || unicode.IsLetter(rune(b)) || unicode.IsDigit(rune(b))
}

// run reads the longest run of bytes accepted by in as one token.
func (s *scanner) run(kind byte, in func(byte) bool) {
	start := s.pos
	for s.pos < len(s.src) && in(s.src[s.pos]) {
		s.pos++
	}
	lower := strings.ToLower(s.src[start:s.pos])
	s.add(kind, start, s.pos, lower)
}

func (s *scanner) quoted(q byte) error {
	end, err := closingQuote(s.src, s.pos)
	if err != nil {
		return err
	}
	text := s.src[s.pos : end+1]
	if q == '\'' {
		s.add('s', s.pos, end+1, "")
	} else {
		s.add('q', s.pos, end+1, strings.ReplaceAll(text[1:len(text)-1], string(q)+string(q), string(q)))
	}
	s.line += strings.Count(text, "\n")
	s.pos = end + 1
	return nil
}

func (s *scanner) bracketed() error {
	end := strings.IndexByte(s.src[s.pos:], ']')
	if end < 0 {
		return errUnterminated
	}
	s.add('q', s.pos, s.pos+end+1, s.src[s.pos+1:s.pos+end])
	s.pos += end + 1
	return nil
}

// dollar reads a PostgreSQL dollar-quoted string, or a $n parameter.
func (s *scanner) dollar() {
	if end, ok := dollarQuoteEnd(s.src, s.pos); ok {
		s.add('s', s.pos, end, "")
		s.line += strings.Count(s.src[s.pos:end], "\n")
		s.pos = end
		return
	}
	start := s.pos
	s.pos++
	for s.pos < len(s.src) && s.src[s.pos] >= '0' && s.src[s.pos] <= '9' {
		s.pos++
	}
	s.add('n', start, s.pos, s.src[start:s.pos])
}

func (s *scanner) punct(c byte) error {
	switch c {
	case '(':
		s.add('p', s.pos, s.pos+1, "(")
		s.depth++
	case ')':
		s.depth--
		if s.depth < 0 {
			return errors.New("unbalanced parenthesis")
		}
		s.add('p', s.pos, s.pos+1, ")")
	default:
		s.add('p', s.pos, s.pos+1, string(c))
	}
	s.pos++
	return nil
}

// closingQuote returns the index of the quote closing the one at src[start];
// a doubled quote is an escaped quote.
func closingQuote(src string, start int) (int, error) {
	q := src[start]
	for i := start + 1; i < len(src); i++ {
		if src[i] != q {
			continue
		}
		if i+1 < len(src) && src[i+1] == q {
			i++
			continue
		}
		return i, nil
	}
	return 0, errUnterminated
}

// dollarQuoteEnd returns the index after a $tag$...$tag$ string starting at
// src[start], and whether one starts there.
func dollarQuoteEnd(src string, start int) (int, bool) {
	j := start + 1
	for j < len(src) && (src[j] == '_' || unicode.IsLetter(rune(src[j])) || (j > start+1 && unicode.IsDigit(rune(src[j])))) {
		j++
	}
	if j >= len(src) || src[j] != '$' {
		return 0, false
	}
	tag := src[start : j+1]
	end := strings.Index(src[j+1:], tag)
	if end < 0 {
		return 0, false
	}
	return j + 1 + end + len(tag), true
}

// splitStatements cuts SQL text at the semicolons that are outside strings,
// comments and parentheses. Empty statements are dropped.
func splitStatements(src string) ([]string, error) {
	toks, err := tokenize(src)
	if err != nil {
		return nil, err
	}
	var out []string
	startByte := 0
	for _, t := range toks {
		if t.isPunct(";") && t.depth == 0 {
			if stmt := strings.TrimSpace(src[startByte:t.start]); hasCode(stmt) {
				out = append(out, stmt)
			}
			startByte = t.end
		}
	}
	if stmt := strings.TrimSpace(src[startByte:]); hasCode(stmt) {
		out = append(out, stmt)
	}
	return out, nil
}

// hasCode reports whether SQL text holds anything besides comments.
func hasCode(stmt string) bool {
	toks, err := tokenize(stmt)
	return err != nil || len(toks) > 0
}

// splitTopLevel splits tokens at the commas of depth base.
func splitTopLevel(toks []sqlToken, base int) [][]sqlToken {
	var out [][]sqlToken
	var cur []sqlToken
	for _, t := range toks {
		if t.isPunct(",") && t.depth == base {
			out = append(out, cur)
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// parenGroup returns the tokens inside the parentheses that open at toks[i]
// and the index of the closing one.
func parenGroup(toks []sqlToken, i int) (inside []sqlToken, closeIdx int, ok bool) {
	if i >= len(toks) || !toks[i].isPunct("(") {
		return nil, 0, false
	}
	for j := i + 1; j < len(toks); j++ {
		if toks[j].isPunct(")") && toks[j].depth == toks[i].depth {
			return toks[i+1 : j], j, true
		}
	}
	return nil, 0, false
}

// identList reads "(a, b, c)" at toks[i] as identifier names.
func identList(toks []sqlToken, i int) ([]string, int) {
	inside, closeIdx, ok := parenGroup(toks, i)
	if !ok {
		return nil, i
	}
	var names []string
	for _, part := range splitTopLevel(inside, toks[i].depth+1) {
		if len(part) == 0 {
			continue
		}
		if n, ok := part[0].ident(); ok {
			names = append(names, n)
		}
	}
	return names, closeIdx
}

// words returns the lowercase words and identifier names of toks.
func words(toks []sqlToken) []string {
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		if n, ok := t.ident(); ok {
			out = append(out, n)
		}
	}
	return out
}
