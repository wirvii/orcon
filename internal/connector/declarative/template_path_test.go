// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import (
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestRenderSubstitutesParametersAndCredentials(t *testing.T) {
	s := scope{
		params:      map[string]jsonvalue.Value{"id": jsonvalue.Int(5), "ok": jsonvalue.Bool(true), "none": jsonvalue.Null()},
		credentials: map[string]string{"tok": "secret"},
	}
	got, err := s.render("/a/{{ $parameter.id }}/{{$credentials.tok}}?ok={{$parameter.ok}}&n={{$parameter.none}}")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if want := "/a/5/secret?ok=true&n="; got != want {
		t.Fatalf("render = %q, want %q", got, want)
	}
}

func TestRenderRefusesBrokenTemplates(t *testing.T) {
	withCreds := scope{params: map[string]jsonvalue.Value{}, credentials: map[string]string{}}
	noCreds := scope{params: map[string]jsonvalue.Value{}}
	arr := jsonvalue.Array(jsonvalue.Int(1))
	cases := []struct {
		name string
		s    scope
		tpl  string
	}{
		{"unclosed placeholder", withCreds, "/a/{{$parameter.id"},
		{"unknown placeholder kind", withCreds, "{{$env.HOME}}"},
		{"parameter missing", withCreds, "{{$parameter.id}}"},
		{"credential field missing", withCreds, "{{$credentials.tok}}"},
		{"credentials not visible here", noCreds, "{{$credentials.tok}}"},
		{"array used as text", scope{params: map[string]jsonvalue.Value{"a": arr}}, "{{$parameter.a}}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.s.render(tc.tpl); !errors.Is(err, ErrTemplate) {
				t.Fatalf("err = %v, want ErrTemplate", err)
			}
		})
	}
}

func TestRenderScalarTextRules(t *testing.T) {
	s := scope{params: map[string]jsonvalue.Value{
		"t": jsonvalue.Bool(true), "f": jsonvalue.Bool(false), "x": jsonvalue.String("a"),
	}}
	got, err := s.render("{{$parameter.t}}-{{$parameter.f}}-{{$parameter.x}}")
	if err != nil || got != "true-false-a" {
		t.Fatalf("render = %q, err = %v", got, err)
	}
}

func TestRenderValueKeepsASinglePlaceholderTyped(t *testing.T) {
	obj, _ := jsonvalue.Object(jsonvalue.Member{Key: "k", Value: jsonvalue.Int(1)})
	s := scope{
		params:      map[string]jsonvalue.Value{"flag": jsonvalue.Bool(true), "obj": obj, "id": jsonvalue.Int(9)},
		credentials: map[string]string{"tok": "secret"},
	}
	if v, err := s.renderValue("{{$parameter.flag}}"); err != nil || v.Kind() != jsonvalue.KindBool {
		t.Fatalf("flag: kind=%v err=%v", v.Kind(), err)
	}
	if v, err := s.renderValue(" {{$parameter.obj}} "); err != nil || v.Kind() != jsonvalue.KindObject {
		t.Fatalf("object: kind=%v err=%v", v.Kind(), err)
	}
	if v, err := s.renderValue("id-{{$parameter.id}}"); err != nil || v.Kind() != jsonvalue.KindString {
		t.Fatalf("mixed text: kind=%v err=%v", v.Kind(), err)
	} else if str, _ := v.Str(); str != "id-9" {
		t.Fatalf("mixed text = %q", str)
	}
	if v, err := s.renderValue("{{$credentials.tok}}"); err != nil {
		t.Fatalf("credential: %v", err)
	} else if str, _ := v.Str(); str != "secret" {
		t.Fatalf("credential text = %q", str)
	}
	if _, err := s.renderValue("{{$parameter.missing}}"); !errors.Is(err, ErrTemplate) {
		t.Fatalf("missing parameter: %v", err)
	}
	// Two placeholders are not one: the result is text, never the first value.
	if v, err := s.renderValue("{{$parameter.id}}{{$parameter.flag}}"); err != nil || v.Kind() != jsonvalue.KindString {
		t.Fatalf("two placeholders: kind=%v err=%v", v.Kind(), err)
	}
}

func TestParsePathSupportedSubset(t *testing.T) {
	doc, err := jsonvalue.Parse([]byte(`{"data":[{"name":"a"},{"name":"b"}],"meta":{"n":1}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cases := []struct {
		path string
		want string
		ok   bool
	}{
		{"$", `{"data":[{"name":"a"},{"name":"b"}],"meta":{"n":1}}`, true},
		{"$.data[1].name", `"b"`, true},
		{"$.meta.n", `1`, true},
		{"$.data[9]", "", false},
		{"$.absent", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			v, ok, err := lookup(doc, tc.path)
			if err != nil || ok != tc.ok {
				t.Fatalf("lookup ok=%v err=%v", ok, err)
			}
			if ok {
				got, _ := jsonvalue.Canonical(v)
				if string(got) != tc.want {
					t.Fatalf("value = %s, want %s", got, tc.want)
				}
			}
		})
	}
}

func TestParsePathRefusesUnsupportedSyntax(t *testing.T) {
	for _, path := range []string{"data", "$.", "$.*", "$..name", "$.a[", "$.a[x]", "$.a[-1]", "$a"} {
		if _, err := parsePath(path); !errors.Is(err, ErrTemplate) {
			t.Errorf("parsePath(%q) = %v, want ErrTemplate", path, err)
		}
	}
	if _, err := parsePath("$.data[0].name"); err != nil {
		t.Errorf("a valid path was refused: %v", err)
	}
}
