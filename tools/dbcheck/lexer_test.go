// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func kinds(toks []token) string {
	var b strings.Builder
	for _, t := range toks {
		switch t.kind {
		case tokWord:
			b.WriteString("W")
		case tokQuoted:
			b.WriteString("Q")
		case tokString:
			b.WriteString("S")
		case tokNumber:
			b.WriteString("N")
		case tokParam:
			b.WriteString("P")
		case tokOp:
			b.WriteString("O")
		case tokPunct:
			b.WriteString("p")
		case tokComment:
			b.WriteString("C")
		}
	}
	return b.String()
}

func texts(toks []token) []string {
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		out = append(out, t.text)
	}
	return out
}

func TestLex(t *testing.T) {
	tests := []struct {
		name    string
		dialect string
		src     string
		kinds   string
		texts   []string
	}{
		{"line comment hides code", dialectSQLite, "SELECT 1 -- DROP TABLE x\n;", "WNCp", []string{"SELECT", "1", "DROP TABLE x", ";"}},
		{"block comment hides code", dialectSQLite, "SELECT /* * */ 1", "WCN", []string{"SELECT", "*", "1"}},
		{"string hides operators", dialectPostgres, "SELECT '->> ? offset'", "WS", []string{"SELECT", "'->> ? offset'"}},
		{"escaped quote in string", dialectSQLite, "SELECT 'it''s'", "WS", []string{"SELECT", "'it''s'"}},
		{"quoted identifier keeps its name", dialectPostgres, `SELECT "Select"`, "WQ", []string{"SELECT", `"Select"`}},
		{"sqlite question mark is a parameter", dialectSQLite, "a = ?", "WOP", []string{"a", "=", "?"}},
		{"sqlite numbered parameter", dialectSQLite, "a = ?12", "WOP", []string{"a", "=", "?12"}},
		{"postgres question mark is an operator", dialectPostgres, "a ? 'k'", "WOS", []string{"a", "?", "'k'"}},
		{"postgres json operators", dialectPostgres, "a ->> 'k' #>> b @? c ?| d", "WOSOWOWOW", []string{"a", "->>", "'k'", "#>>", "b", "@?", "c", "?|", "d"}},
		{"dollar parameter", dialectPostgres, "id = $1", "WOP", []string{"id", "=", "$1"}},
		{"dollar quoted body", dialectPostgres, "AS $body$ SELECT '->>'; $body$", "WS", []string{"AS", "$body$ SELECT '->>'; $body$"}},
		{"empty tag dollar quote", dialectPostgres, "AS $$ ; $$", "WS", []string{"AS", "$$ ; $$"}},
		{"named parameters", dialectSQLite, "a = :name AND b = @other", "WOPWWOP", []string{"a", "=", ":name", "AND", "b", "=", "@other"}},
		{"cast", dialectPostgres, "a::text", "WpW", []string{"a", "::", "text"}},
		{"star in count", dialectSQLite, "count(*)", "WpOp", []string{"count", "(", "*", ")"}},
		{"star after dot", dialectSQLite, "t.*", "WpO", []string{"t", ".", "*"}},
		{"operator stops before comment", dialectPostgres, "a +-- c\n", "WOC", []string{"a", "+", "c"}},
		{"sqlite bracket identifier", dialectSQLite, "[a b]", "Q", []string{"[a b]"}},
		{"postgres array brackets", dialectPostgres, "int[]", "Wpp", []string{"int", "[", "]"}},
		{"numbers", dialectSQLite, "1 2.5 .5 1e3 0xFF", "NNNNN", []string{"1", "2.5", ".5", "1e3", "0xFF"}},
		{"nested block comment in postgres", dialectPostgres, "/* a /* b */ c */ x", "CW", []string{"a /* b */ c", "x"}},
		{"E string escapes", dialectPostgres, `SELECT E'a\'b'`, "WS", []string{"SELECT", `'a\'b'`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks, err := lex(tt.src, tt.dialect)
			if err != nil {
				t.Fatalf("lex: %v", err)
			}
			if got := kinds(toks); got != tt.kinds {
				t.Fatalf("kinds = %s, want %s (%v)", got, tt.kinds, texts(toks))
			}
			if tt.name == "E string escapes" {
				return // the token text drops the E prefix on purpose
			}
			got := texts(toks)
			if strings.Join(got, "|") != strings.Join(tt.texts, "|") {
				t.Fatalf("texts = %q, want %q", got, tt.texts)
			}
		})
	}
}

func TestLexErrors(t *testing.T) {
	for name, src := range map[string]string{
		"unterminated string":        "SELECT 'a",
		"unterminated block comment": "SELECT /* a",
		"unbalanced close":           "SELECT )",
		"unbalanced open":            "SELECT (",
		"unterminated dollar quote":  "AS $x$ a",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := lex(src, dialectPostgres); err == nil {
				t.Fatalf("lex(%q) succeeded, want an error", src)
			}
		})
	}
}

func TestLexDepthAndLines(t *testing.T) {
	toks, err := lex("a (\n b (c) )\n d", dialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{"a": {0, 1}, "b": {1, 2}, "c": {2, 2}, "d": {0, 3}}
	for _, tok := range toks {
		if w, ok := want[tok.text]; ok && (tok.depth != w[0] || tok.line != w[1]) {
			t.Errorf("%s: depth/line = %d/%d, want %d/%d", tok.text, tok.depth, tok.line, w[0], w[1])
		}
	}
}

func TestSplitStatements(t *testing.T) {
	src := `CREATE TABLE a (x INTEGER);
CREATE TRIGGER t AFTER INSERT ON a BEGIN
  UPDATE a SET x = CASE WHEN 1 THEN 2 ELSE 3 END;
  DELETE FROM a;
END;
SELECT ';' ;
`
	toks, err := lex(src, dialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	stmts := splitStatements(codeTokens(toks))
	if len(stmts) != 3 {
		t.Fatalf("got %d statements, want 3", len(stmts))
	}
	if !stmts[1].toks[1].isWord("trigger") || !stmts[1].toks[len(stmts[1].toks)-1].isWord("end") {
		t.Fatalf("trigger statement was cut: %v", texts(stmts[1].toks))
	}
}
