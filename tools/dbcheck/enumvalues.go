// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"regexp"
	"strings"
)

// enumTok is a token reduced to what the enumeration-value rule (CK-24) reads.
// The algorithm is the same in tools/dbcheck and in dbrules; each has its own
// conversion from its lexer to this form.
type enumTok struct {
	kind byte // 'i' identifier or word, 's' string (text is the value), 'e' = <> !=, 'o' other operator, 'c' cast "::", 'd' ".", '(' ')' '[' ']' ',' and 'n' anything else
	text string
}

var upperSnakeRE = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)

var nonAlnumRE = regexp.MustCompile(`[^A-Za-z0-9]+`)

// upperSnakeFix is the value uppercased with every run of other characters
// replaced by "_".
func upperSnakeFix(value string) string {
	return strings.ToUpper(nonAlnumRE.ReplaceAllString(value, "_"))
}

// enumLiteral is a string literal CK-24 judges.
type enumLiteral struct {
	value string
	index int
}

// badEnumLiterals returns the literals of one CHECK expression that must be
// UPPER_SNAKE and are not. words are the enumeration words of NM-8.
func badEnumLiterals(t []enumTok, words []string) []enumLiteral {
	var out []enumLiteral
	for i, tok := range t {
		if tok.kind != 's' {
			continue
		}
		col, inList := listColumn(t, i)
		if !inList {
			col = comparisonColumn(t, i)
			if col == "" || !isEnumColumn(col, words) {
				continue
			}
		}
		if col == "format" || strings.HasSuffix(col, "_format") {
			continue
		}
		if !upperSnakeRE.MatchString(tok.text) {
			out = append(out, enumLiteral{value: tok.text, index: i})
		}
	}
	return out
}

func isEnumColumn(col string, words []string) bool {
	for _, w := range words {
		if col == w || strings.HasSuffix(col, "_"+w) {
			return true
		}
	}
	return false
}

// listColumn reports whether the literal at i belongs to an IN or NOT IN list,
// or to = ANY (ARRAY[...]) or <> ALL (ARRAY[...]), and which column the list is
// compared with ("" when it cannot be told).
func listColumn(t []enumTok, i int) (string, bool) {
	depth := 0
	inArray := false
	for j := i - 1; j >= 0; j-- {
		switch t[j].kind {
		case ')', ']':
			depth++
		case '(', '[':
			if depth > 0 {
				depth--
				continue
			}
			prev := ""
			if j > 0 && t[j-1].kind == 'i' {
				prev = t[j-1].text
			}
			switch {
			case t[j].kind == '[' && prev == "array":
				inArray = true
			case t[j].kind == '(' && prev == "in":
				return leftOperand(t, j-2), true
			case t[j].kind == '(' && (prev == "any" || prev == "all") && j >= 2:
				return leftOperand(t, j-3), true
			}
		}
	}
	return "", inArray
}

// leftOperand returns the column name that ends at or before t[k], skipping NOT,
// closing parentheses and casts.
func leftOperand(t []enumTok, k int) string {
	for k >= 0 {
		switch {
		case t[k].kind == ')' || t[k].kind == '(':
			k--
		case t[k].kind == 'i' && t[k].text == "not":
			k--
		case t[k].kind == 'i':
			j := k
			for j > 0 && t[j-1].kind == 'i' {
				j--
			}
			if j > 0 && t[j-1].kind == 'c' { // a cast type, possibly of several words
				k = j - 2
				continue
			}
			return t[k].text
		default:
			return ""
		}
	}
	return ""
}

// comparisonColumn returns the column a literal at i is compared with by =, <>
// or !=, or "".
func comparisonColumn(t []enumTok, i int) string {
	p := i - 1
	for p >= 0 && t[p].kind == '(' {
		p--
	}
	if p >= 0 && t[p].kind == 'e' {
		return leftOperand(t, p-1)
	}
	q := i + 1
	for q < len(t) {
		switch {
		case t[q].kind == 'c':
			q++
			if q < len(t) && t[q].kind == 'i' { // the cast type: one word, or "character varying"
				q++
				if q < len(t) && t[q].kind == 'i' && (t[q].text == "varying" || t[q].text == "precision") {
					q++
				}
			}
			continue
		case t[q].kind == ')':
			q++
			continue
		}
		break
	}
	if q < len(t) && t[q].kind == 'e' {
		return rightOperand(t, q+1)
	}
	return ""
}

func rightOperand(t []enumTok, k int) string {
	for k < len(t) && t[k].kind == '(' {
		k++
	}
	name := ""
	for k < len(t) && t[k].kind == 'i' {
		name = t[k].text
		if k+2 < len(t) && t[k+1].kind == 'd' && t[k+2].kind == 'i' {
			k += 2
			continue
		}
		break
	}
	return name
}

// toEnumToks converts lexer tokens of one expression.
func toEnumToks(toks []token) []enumTok {
	out := make([]enumTok, 0, len(toks))
	for _, t := range toks {
		switch t.kind {
		case tokWord, tokQuoted:
			out = append(out, enumTok{kind: 'i', text: t.lower})
		case tokString:
			if strings.HasPrefix(t.text, "'") {
				out = append(out, enumTok{kind: 's', text: strings.ReplaceAll(t.text[1:len(t.text)-1], "''", "'")})
			} else {
				out = append(out, enumTok{kind: 'n', text: t.text})
			}
		case tokOp:
			if t.text == "=" || t.text == "<>" || t.text == "!=" || t.text == "==" {
				out = append(out, enumTok{kind: 'e', text: t.text})
			} else {
				out = append(out, enumTok{kind: 'o', text: t.text})
			}
		case tokPunct:
			switch t.text {
			case "::":
				out = append(out, enumTok{kind: 'c', text: t.text})
			case ".":
				out = append(out, enumTok{kind: 'd', text: t.text})
			case "(", ")", "[", "]", ",":
				out = append(out, enumTok{kind: t.text[0], text: t.text})
			default:
				out = append(out, enumTok{kind: 'o', text: t.text})
			}
		default:
			out = append(out, enumTok{kind: 'n', text: t.text})
		}
	}
	return out
}
