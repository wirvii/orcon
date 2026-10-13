// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package jsonvalue_test

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func TestCanonicalNumbers(t *testing.T) {
	t.Parallel()
	// Vectors from RFC 8785 appendix B plus the boundaries of the three
	// ECMAScript notations.
	tests := []struct{ in, want string }{
		{"0", "0"},
		{"-0", "0"},
		{"5e-324", "5e-324"},
		{"1.7976931348623157e308", "1.7976931348623157e+308"},
		{"295147905179352830000", "295147905179352830000"},
		{"9.999999999999997e22", "9.999999999999997e+22"},
		{"1e23", "1e+23"},
		{"1.0e22", "1e+22"},
		{"1e21", "1e+21"},
		{"1e20", "100000000000000000000"},
		{"1e-6", "0.000001"},
		{"1e-7", "1e-7"},
		{"4.5", "4.5"},
		{"2e-3", "0.002"},
		{"100", "100"},
		{"-1.5E+2", "-150"},
		{"123456789012345680000", "123456789012345680000"},
		{"0.1", "0.1"},
		{"1.5e300", "1.5e+300"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			v, err := jsonvalue.Parse([]byte(tt.in))
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.in, err)
			}
			got, err := jsonvalue.Canonical(v)
			if err != nil {
				t.Fatalf("Canonical(%q) error: %v", tt.in, err)
			}
			if string(got) != tt.want {
				t.Errorf("Canonical(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanonicalObjectsAndStrings(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, in, want string }{
		{"sorted keys", `{"b":1,"a":2,"c":{"z":null,"y":[true,false]}}`, `{"a":2,"b":1,"c":{"y":[true,false],"z":null}}`},
		{"whitespace", " { \"a\" : [ 1 , 2 ] } \n", `{"a":[1,2]}`},
		{"utf16 order", "{\"\U0001F600\":1,\"דּ\":2}", "{\"\U0001F600\":1,\"דּ\":2}"},
		{"minimal escapes", `"A\/\u007f\u0001\n\"\\"`, `"A/` + "\x7f" + `\u0001\n\"\\"`},
		{"control chars", `"\b\t\f\r"`, `"\b\t\f\r"`},
		{"empty containers", `{"a":[],"b":{}}`, `{"a":[],"b":{}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, err := jsonvalue.Parse([]byte(tt.in))
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.in, err)
			}
			got, err := jsonvalue.Canonical(v)
			if err != nil {
				t.Fatalf("Canonical error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Canonical(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in string
		want     error
	}{
		{"duplicate key", `{"a":1,"a":2}`, jsonvalue.ErrDuplicateKey},
		{"nested duplicate key", `{"x":{"a":1,"a":2}}`, jsonvalue.ErrDuplicateKey},
		{"trailing data", `{"a":1} 2`, jsonvalue.ErrSyntax},
		{"empty", ``, jsonvalue.ErrSyntax},
		{"truncated", `{"a":`, jsonvalue.ErrSyntax},
		{"invalid utf-8", "\"\xff\"", jsonvalue.ErrSyntax},
		{"bad token", `{a:1}`, jsonvalue.ErrSyntax},
		{"too deep", strings.Repeat("[", jsonvalue.MaxDepth+1) + strings.Repeat("]", jsonvalue.MaxDepth+1), jsonvalue.ErrTooDeep},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := jsonvalue.Parse([]byte(tt.in)); !errors.Is(err, tt.want) {
				t.Errorf("Parse(%q) error = %v, want %v", tt.in, err, tt.want)
			}
		})
	}
}

func TestMarshalPreservesOrderAndNumbers(t *testing.T) {
	t.Parallel()
	in := `{"b":1.0,"a":[10000000000000000000000001,"x"],"c":null,"d":true}`
	v, err := jsonvalue.Parse([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	got, err := jsonvalue.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != in {
		t.Errorf("Marshal = %s, want %s", got, in)
	}
}

func TestCanonicalRejectsHugeNumberAndAbsent(t *testing.T) {
	t.Parallel()
	v, err := jsonvalue.Number("1e999")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jsonvalue.Canonical(v); !errors.Is(err, jsonvalue.ErrNotEncodable) {
		t.Errorf("Canonical(1e999) error = %v, want ErrNotEncodable", err)
	}
	if _, err := jsonvalue.Marshal(jsonvalue.Value{}); !errors.Is(err, jsonvalue.ErrNotEncodable) {
		t.Errorf("Marshal(zero) error = %v, want ErrNotEncodable", err)
	}
	if _, err := jsonvalue.Marshal(jsonvalue.Array(jsonvalue.Value{})); !errors.Is(err, jsonvalue.ErrNotEncodable) {
		t.Errorf("Marshal([absent]) error = %v, want ErrNotEncodable", err)
	}
}

func TestConstructorsAndAccessors(t *testing.T) {
	t.Parallel()
	obj, err := jsonvalue.Object(
		jsonvalue.Member{Key: "n", Value: jsonvalue.Int(7)},
		jsonvalue.Member{Key: "s", Value: jsonvalue.String("x")},
		jsonvalue.Member{Key: "b", Value: jsonvalue.Bool(true)},
		jsonvalue.Member{Key: "l", Value: jsonvalue.Array(jsonvalue.Null())},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jsonvalue.Object(jsonvalue.Member{Key: "k"}, jsonvalue.Member{Key: "k"}); err == nil {
		t.Error("Object with duplicate key succeeded, want error")
	}
	n, _ := obj.Get("n")
	if i, ok := n.Int64(); !ok || i != 7 {
		t.Errorf("Int64 = %d, %v, want 7, true", i, ok)
	}
	if f, ok := n.Float64(); !ok || f != 7 {
		t.Errorf("Float64 = %v, %v, want 7, true", f, ok)
	}
	if text, ok := n.NumberText(); !ok || text != "7" {
		t.Errorf("NumberText = %q, %v", text, ok)
	}
	s, _ := obj.Get("s")
	if str, ok := s.Str(); !ok || str != "x" {
		t.Errorf("Str = %q, %v", str, ok)
	}
	if _, ok := n.Str(); ok {
		t.Error("Str on a number reported ok")
	}
	b, _ := obj.Get("b")
	if v, ok := b.Bool(); !ok || !v {
		t.Errorf("Bool = %v, %v", v, ok)
	}
	if obj.Len() != 4 || len(obj.Members()) != 4 {
		t.Errorf("Len = %d, want 4", obj.Len())
	}
	l, _ := obj.Get("l")
	if l.Len() != 1 || len(l.Items()) != 1 || l.Items()[0].Kind() != jsonvalue.KindNull {
		t.Errorf("array accessors wrong: %v", l.Items())
	}
	if obj.Items() != nil || l.Members() != nil || n.Len() != 0 {
		t.Error("accessor for the wrong kind returned data")
	}
	if _, ok := n.Get("x"); ok {
		t.Error("Get on a number reported ok")
	}
	if !(jsonvalue.Value{}).IsZero() || obj.IsZero() {
		t.Error("IsZero wrong")
	}
	if got := obj.Without("n").Len(); got != 3 {
		t.Errorf("Without removed to Len %d, want 3", got)
	}
	if obj.Len() != 4 {
		t.Error("Without mutated the receiver")
	}
	replaced := obj.With("n", jsonvalue.Int(8)).With("new", jsonvalue.Null())
	if got, _ := replaced.Get("n"); !jsonvalue.Equal(got, jsonvalue.Int(8)) || replaced.Len() != 5 {
		t.Errorf("With produced %v", replaced)
	}
	if got, _ := obj.Get("n"); !jsonvalue.Equal(got, jsonvalue.Int(7)) {
		t.Error("With mutated the receiver")
	}
	if n.Without("x").Kind() != jsonvalue.KindNumber || n.With("x", n).Kind() != jsonvalue.KindNumber {
		t.Error("Without/With on a non-object changed the value")
	}
}

func TestNumberAndFloat(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"0", "-1", "1.5", "1e5", "1E+5", "-0.0e-1", "10"} {
		if _, err := jsonvalue.Number(ok); err != nil {
			t.Errorf("Number(%q) error: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-", "01", "1.", ".5", "1e", "1e+", "+1", "0x1", "1 ", "NaN", "--1"} {
		if _, err := jsonvalue.Number(bad); err == nil {
			t.Errorf("Number(%q) succeeded, want error", bad)
		}
	}
	if v, err := jsonvalue.Float(2.5); err != nil {
		t.Errorf("Float(2.5) error: %v", err)
	} else if text, _ := v.NumberText(); text != "2.5" {
		t.Errorf("Float(2.5) text = %q", text)
	}
	if _, err := jsonvalue.Float(math.Inf(1)); err == nil {
		t.Error("Float(+Inf) succeeded, want error")
	}
	if _, ok := mustNumber(t, "1e999").Float64(); ok {
		t.Error("Float64 of 1e999 reported ok")
	}
	if _, ok := mustNumber(t, "1.5").Int64(); ok {
		t.Error("Int64 of 1.5 reported ok")
	}
}

func TestEqual(t *testing.T) {
	t.Parallel()
	parse := func(s string) jsonvalue.Value {
		v, err := jsonvalue.Parse([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	tests := []struct {
		a, b string
		want bool
	}{
		{`{"a":1,"b":[1,2]}`, `{"b":[1,2],"a":1}`, true},
		{`{"a":1}`, `{"a":2}`, false},
		{`{"a":1}`, `{"a":1,"b":1}`, false},
		{`[1,2]`, `[2,1]`, false},
		{`[1]`, `[1,2]`, false},
		{`"x"`, `"x"`, true},
		{`1`, `"1"`, false},
		{`true`, `false`, false},
		{`null`, `null`, true},
		{`1.0`, `1`, false},
	}
	for _, tt := range tests {
		if got := jsonvalue.Equal(parse(tt.a), parse(tt.b)); got != tt.want {
			t.Errorf("Equal(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
	if !jsonvalue.Equal(jsonvalue.Value{}, jsonvalue.Value{}) {
		t.Error("two absent values are not equal")
	}
}

func TestKindString(t *testing.T) {
	t.Parallel()
	kinds := []jsonvalue.Kind{
		jsonvalue.KindInvalid, jsonvalue.KindNull, jsonvalue.KindBool, jsonvalue.KindNumber,
		jsonvalue.KindString, jsonvalue.KindArray, jsonvalue.KindObject, jsonvalue.Kind(99),
	}
	want := []string{"absent", "null", "boolean", "number", "string", "array", "object", "kind(99)"}
	for i, k := range kinds {
		if k.String() != want[i] {
			t.Errorf("Kind(%d).String() = %q, want %q", k, k.String(), want[i])
		}
	}
}

func FuzzParseCanonical(f *testing.F) {
	for _, seed := range []string{
		`{"a":[1,2.5,"x",null,true],"b":{"c":-0}}`, `[]`, `"😀"`, `1e400`, `{"":""}`, `{"a":1,"a":2}`, `[[[[1]]]]`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		v, err := jsonvalue.Parse(data)
		if err != nil {
			return
		}
		// Marshal of a parsed value must parse back to an equal value.
		text, err := jsonvalue.Marshal(v)
		if err != nil {
			t.Fatalf("Marshal of parsed value: %v", err)
		}
		back, err := jsonvalue.Parse(text)
		if err != nil {
			t.Fatalf("Parse of marshaled value %q: %v", text, err)
		}
		if !jsonvalue.Equal(v, back) {
			t.Fatalf("round trip changed the value: %q", text)
		}
		// Canonical form is a fixed point: canonicalizing it changes nothing.
		canon, err := jsonvalue.Canonical(v)
		if err != nil {
			return // a number outside the double range has no canonical form
		}
		again, err := jsonvalue.Parse(canon)
		if err != nil {
			t.Fatalf("Parse of canonical %q: %v", canon, err)
		}
		canon2, err := jsonvalue.Canonical(again)
		if err != nil || !bytes.Equal(canon2, canon) {
			t.Fatalf("canonical form is not stable: %q then %q (%v)", canon, canon2, err)
		}
	})
}

func mustNumber(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Number(text)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
