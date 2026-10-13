// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/expr"
)

func TestSegments(t *testing.T) {
	t.Parallel()
	type seg = expr.Segment
	tests := []struct {
		name string
		body string
		want []seg
	}{
		{"empty", "", nil},
		{"plain text", "hello", []seg{{Text: "hello"}}},
		{"one expression", "{{ $json.a }}", []seg{{Code: true, Text: " $json.a ", Offset: 2}}},
		{"text around", "a {{ 1 }} b", []seg{{Text: "a ", Offset: 0}, {Code: true, Text: " 1 ", Offset: 4}, {Text: " b", Offset: 9}}},
		{"two expressions", "{{a}}{{b}}", []seg{{Code: true, Text: "a", Offset: 2}, {Code: true, Text: "b", Offset: 7}}},
		{"unterminated is text", "x {{ 1 + ", []seg{{Text: "x {{ 1 + "}}},
		{"braces inside code", "{{ {a: {b: 1}}.a.b }}", []seg{{Code: true, Text: " {a: {b: 1}}.a.b ", Offset: 2}}},
		{"closing braces in a string", `{{ "}}" + 'x' }}`, []seg{{Code: true, Text: ` "}}" + 'x' `, Offset: 2}}},
		{"closing braces in a template literal", "{{ `}}${1}` }}", []seg{{Code: true, Text: " `}}${1}` ", Offset: 2}}},
		{"closing braces in a comment", "{{ 1 /* }} */ }}", []seg{{Code: true, Text: " 1 /* }} */ ", Offset: 2}}},
		{"line comment", "{{ 1 // }}\n }}", []seg{{Code: true, Text: " 1 // }}\n ", Offset: 2}}},
		{"unterminated string falls back to the first closing braces", `{{ 'a }} b`, []seg{{Code: true, Text: " 'a ", Offset: 2}, {Text: " b", Offset: 8}}},
		{"unterminated comment falls back", "{{ 1 /* }} x", []seg{{Code: true, Text: " 1 /* ", Offset: 2}, {Text: " x", Offset: 10}}},
		{"a single closing brace stays in the code", "{{ {a:1}.a }}", []seg{{Code: true, Text: " {a:1}.a ", Offset: 2}}},
		{"division is not a comment", "{{ 4 / 2 }}", []seg{{Code: true, Text: " 4 / 2 ", Offset: 2}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tc.want, expr.Segments(tc.body)); diff != "" {
				t.Errorf("Segments(%q) mismatch (-want +got):\n%s", tc.body, diff)
			}
		})
	}
}

func TestIsTemplate(t *testing.T) {
	t.Parallel()
	for s, want := range map[string]bool{"": false, "x": false, "=": true, "={{ 1 }}": true, " =x": false} {
		if got := expr.IsTemplate(s); got != want {
			t.Errorf("IsTemplate(%q) = %v, want %v", s, got, want)
		}
	}
}

// FuzzSegments checks that the segments of any text tile the text: literal
// pieces and the code between their braces, in order, without losing a byte.
func FuzzSegments(f *testing.F) {
	for _, s := range []string{"", "a{{b}}c", "{{ {a:{b:1}} }}", "{{ '}}' }}", "{{", "}}", "{{{{ }}}}", "{{ `x${1}` }}", "{{ // }}\n}}", "é{{ ü }}"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		var rebuilt strings.Builder
		pos := 0
		for _, seg := range expr.Segments(body) {
			if seg.Offset < pos || seg.Offset > len(body) || !strings.HasPrefix(body[seg.Offset:], seg.Text) {
				t.Fatalf("segment %+v does not match the text at its offset (previous end %d)", seg, pos)
			}
			rebuilt.WriteString(body[pos:seg.Offset]) // the braces that delimit the code
			rebuilt.WriteString(seg.Text)
			pos = seg.Offset + len(seg.Text)
		}
		rebuilt.WriteString(body[pos:])
		if rebuilt.String() != body {
			t.Fatalf("segments do not tile the text: rebuilt %q from %q", rebuilt.String(), body)
		}
	})
}
