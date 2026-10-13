// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import "strings"

// Segment is a piece of a template: literal text or the code of a {{ }}
// segment.
type Segment struct {
	// Code is true when Text holds the JavaScript between {{ and }}.
	Code bool
	Text string
	// Offset is the position of Text in the template body (the string after
	// the leading "=").
	Offset int
}

// IsTemplate reports whether a parameter value is an expression: a string
// that starts with "=" (ADR-0007 section 3).
func IsTemplate(s string) bool { return strings.HasPrefix(s, "=") }

// Segments splits the body of a template (the parameter value without its
// leading "=") into literal text and code segments. A "{{" without a closing
// "}}" is literal text, as in n8n. The closing "}}" is found with a light
// JavaScript scan, so braces and quotes inside the code do not end the
// segment early: {{ {a: {b: 1}}.a }} is one segment.
func Segments(body string) []Segment {
	var out []Segment
	pos := 0
	for pos < len(body) {
		open := strings.Index(body[pos:], "{{")
		if open < 0 {
			break
		}
		open += pos
		end := closingBraces(body, open+2)
		if end < 0 {
			break
		}
		if open > pos {
			out = append(out, Segment{Text: body[pos:open], Offset: pos})
		}
		out = append(out, Segment{Code: true, Text: body[open+2 : end], Offset: open + 2})
		pos = end + 2
	}
	if pos < len(body) {
		out = append(out, Segment{Text: body[pos:], Offset: pos})
	}
	return out
}

// closingBraces returns the index of the "}}" that closes the code starting at
// from, or -1.
func closingBraces(s string, from int) int {
	depth := 0
	for i := from; i < len(s); i++ {
		switch c := s[i]; c {
		case '\'', '"', '`':
			i = skipQuoted(s, i)
			if i < 0 {
				return plainClosing(s, from)
			}
		case '/':
			next, ok := skipComment(s, i)
			if !ok {
				return plainClosing(s, from)
			}
			i = next
		case '{':
			depth++
		case '}':
			if depth == 0 {
				if i+1 < len(s) && s[i+1] == '}' {
					return i
				}
				continue
			}
			depth--
		}
	}
	return plainClosing(s, from)
}

// skipComment returns the index of the last byte of the comment that starts at
// i, or i when there is none (a division). It reports false when a comment
// never ends.
func skipComment(s string, i int) (int, bool) {
	if i+1 >= len(s) {
		return i, true
	}
	switch s[i+1] {
	case '/':
		nl := strings.IndexByte(s[i:], '\n')
		if nl < 0 {
			return i, false
		}
		return i + nl, true
	case '*':
		end := strings.Index(s[i+2:], "*/")
		if end < 0 {
			return i, false
		}
		return i + end + 3, true
	}
	return i, true
}

// plainClosing is the fallback when the scan cannot make sense of the code:
// the first "}}".
func plainClosing(s string, from int) int {
	i := strings.Index(s[from:], "}}")
	if i < 0 {
		return -1
	}
	return from + i
}

// skipQuoted returns the index of the closing quote of the string or template
// literal that opens at i, or -1 when it never closes.
func skipQuoted(s string, i int) int {
	quote := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case quote:
			return j
		}
	}
	return -1
}
