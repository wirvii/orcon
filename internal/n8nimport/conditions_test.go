// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// filterOf builds one filter with a single condition, the common IF shape.
func filterOf(t *testing.T, cond string, caseSensitive bool) jsonvalue.Value {
	t.Helper()
	cs := "false"
	if caseSensitive {
		cs = "true"
	}
	data := `{"options": {"caseSensitive": ` + cs + `}, "combinator": "and", "conditions": [` + cond + `]}`
	v, err := jsonvalue.Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestFilterJSOperators(t *testing.T) {
	tests := []struct {
		name string
		cond string
		cs   bool
		want string
	}{
		{
			name: "string equals against a literal",
			cond: `{"leftValue": "={{ $json.type }}", "rightValue": "text", "operator": {"type": "string", "operation": "equals"}}`,
			cs:   true,
			want: `($json.type) === "text"`,
		},
		{
			name: "string not equals",
			cond: `{"leftValue": "={{ $json.a }}", "rightValue": "x", "operator": {"type": "string", "operation": "notEquals"}}`,
			cs:   true,
			want: `($json.a) !== "x"`,
		},
		{
			name: "contains ignores case when the filter says so",
			cond: `{"leftValue": "={{ $json.text }}", "rightValue": "PRECIO", "operator": {"type": "string", "operation": "contains"}}`,
			cs:   false,
			want: `String(String(($json.text)).toLowerCase()).includes(String(String("PRECIO").toLowerCase()))`,
		},
		{
			name: "not contains",
			cond: `{"leftValue": "={{ $json.text }}", "rightValue": "x", "operator": {"type": "string", "operation": "notContains"}}`,
			cs:   true,
			want: `!String(($json.text)).includes(String("x"))`,
		},
		{
			name: "starts with",
			cond: `{"leftValue": "=abc", "rightValue": "a", "operator": {"type": "string", "operation": "startsWith"}}`,
			cs:   true,
			want: `String("abc").startsWith(String("a"))`,
		},
		{
			name: "is not empty takes no right value",
			cond: `{"leftValue": "={{ $json.body }}", "operator": {"type": "string", "operation": "isNotEmpty", "singleValue": true}}`,
			cs:   true,
			want: `!((($json.body) === undefined || ($json.body) === null || ($json.body) === ""))`,
		},
		{
			name: "number greater than compares as numbers",
			cond: `{"leftValue": "={{ $json.total }}", "rightValue": 100, "operator": {"type": "number", "operation": "gt"}}`,
			cs:   true,
			want: `Number(($json.total)) > Number(100)`,
		},
		{
			name: "boolean true",
			cond: `{"leftValue": "={{ $json.ok }}", "operator": {"type": "boolean", "operation": "true", "singleValue": true}}`,
			cs:   true,
			want: `($json.ok) === true`,
		},
		{
			name: "any exists",
			cond: `{"leftValue": "={{ $json.id }}", "operator": {"type": "any", "operation": "exists", "singleValue": true}}`,
			cs:   true,
			want: `(($json.id) !== undefined && ($json.id) !== null)`,
		},
		{
			name: "literal operands are quoted",
			cond: `{"leftValue": "plain text", "rightValue": "other", "operator": {"type": "string", "operation": "equals"}}`,
			cs:   true,
			want: `"plain text" === "other"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterJS(filterOf(t, tt.cond, tt.cs))
			if err != nil {
				t.Fatalf("filterJS: %v", err)
			}
			if got != "("+tt.want+")" {
				t.Errorf("filterJS =\n  %s\nwant\n  (%s)", got, tt.want)
			}
		})
	}
}

func TestFilterJSCombinatorAndFailures(t *testing.T) {
	two := `{"leftValue": "={{ $json.a }}", "rightValue": "1", "operator": {"type": "string", "operation": "equals"}},
		{"leftValue": "={{ $json.b }}", "rightValue": "2", "operator": {"type": "string", "operation": "equals"}}`
	v, err := jsonvalue.Parse([]byte(`{"combinator": "or", "conditions": [` + two + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := filterJS(v)
	if err != nil {
		t.Fatal(err)
	}
	if want := `(($json.a) === "1") || (($json.b) === "2")`; got != want {
		t.Errorf("or filter = %s, want %s", got, want)
	}

	failures := []struct {
		name   string
		filter string
	}{
		{"no conditions", `{"conditions": []}`},
		{"unknown combinator", `{"combinator": "xor", "conditions": [{"leftValue": "a", "rightValue": "b", "operator": {"type": "string", "operation": "equals"}}]}`},
		{"date operator is not converted", `{"conditions": [{"leftValue": "={{ $json.d }}", "rightValue": "2026", "operator": {"type": "dateTime", "operation": "after"}}]}`},
		{"regex operator is not converted", `{"conditions": [{"leftValue": "a", "rightValue": "b", "operator": {"type": "string", "operation": "regex"}}]}`},
		{"condition without operator", `{"conditions": [{"leftValue": "a", "rightValue": "b"}]}`},
		{"unclosed expression", `{"conditions": [{"leftValue": "={{ $json.a", "rightValue": "b", "operator": {"type": "string", "operation": "equals"}}]}`},
		{"empty expression", `{"conditions": [{"leftValue": "={{   }}", "rightValue": "b", "operator": {"type": "string", "operation": "equals"}}]}`},
		{"array operand", `{"conditions": [{"leftValue": [1], "rightValue": "b", "operator": {"type": "string", "operation": "equals"}}]}`},
	}
	for _, tt := range failures {
		t.Run(tt.name, func(t *testing.T) {
			v, err := jsonvalue.Parse([]byte(tt.filter))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := filterJS(v); !errors.Is(err, errNotConverted) {
				t.Errorf("error %v, want a not-converted error", err)
			}
		})
	}
}

func TestTemplateJS(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"plain", `"plain"`},
		{"=literal without braces", `"literal without braces"`},
		{"={{ $json.a }}", `($json.a)`},
		{"=Hola {{ $json.name }}!", `("" + "Hola " + ($json.name) + "!")`},
		{"={{ $json.a }}{{ $json.b }}", `("" + ($json.a) + ($json.b))`},
	}
	for _, tt := range tests {
		got, err := templateJS(tt.in)
		if err != nil {
			t.Fatalf("templateJS(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("templateJS(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestWhenParamWrapsTemplate(t *testing.T) {
	if got := whenParam("a === 1"); got != "={{ a === 1 }}" {
		t.Errorf("whenParam = %q", got)
	}
}
