// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// withSensitivePaths returns the JSON of a conformance flow whose node "fetch"
// carries the given value as its sensitivePaths member.
func withSensitivePaths(t *testing.T, member jsonvalue.Value) jsonvalue.Value {
	t.Helper()
	v := conformanceValue(t, "linear")
	nodes, _ := v.Get("nodes")
	items := nodes.Items()
	for i, n := range items {
		id, _ := n.Get("id")
		if s, _ := id.Str(); s == "fetch" {
			items[i] = n.With("sensitivePaths", member)
		}
	}
	return v.With("nodes", jsonvalue.Array(items...))
}

func stringArray(items ...string) jsonvalue.Value {
	vs := make([]jsonvalue.Value, len(items))
	for i, s := range items {
		vs[i] = jsonvalue.String(s)
	}
	return jsonvalue.Array(vs...)
}

func TestSensitivePathsValidation(t *testing.T) {
	t.Parallel()
	tooMany := make([]string, flow.MaxSensitivePaths+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("/f%d", i)
	}
	atLimit := tooMany[:flow.MaxSensitivePaths]
	cases := []struct {
		name  string
		paths []string
		code  string // empty means valid
	}{
		{"one path", []string{"/password"}, ""},
		{"nested path", []string{"/user/card/number"}, ""},
		{"array index", []string{"/items/0/token"}, ""},
		{"wildcard segment", []string{"/items/*/token", "/*/secret"}, ""},
		{"escaped tilde and slash", []string{"/a~0b", "/a~1b", "/~0~1"}, ""},
		{"empty key segment", []string{"/"}, ""},
		{"non-ASCII", []string{"/contraseña/日本"}, ""},
		{"exactly 64 paths", atLimit, ""},
		{"exactly 512 code points", []string{"/" + strings.Repeat("é", 511)}, ""},
		{"no leading slash", []string{"password"}, flow.CodeFormat},
		{"empty pointer", []string{""}, flow.CodeFormat},
		{"relative root", []string{"user/password"}, flow.CodeFormat},
		{"tilde not followed by 0 or 1", []string{"/a~2b"}, flow.CodeFormat},
		{"trailing tilde", []string{"/a~"}, flow.CodeFormat},
		{"513 code points", []string{"/" + strings.Repeat("é", 512)}, flow.CodeFormat},
		{"65 paths", tooMany, flow.CodeValue},
		{"duplicate", []string{"/a", "/b", "/a"}, flow.CodeValue},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc := load(t, "linear")
			node(&doc, "fetch").SensitivePaths = tt.paths
			r := flow.Validate(&doc)
			if tt.code == "" {
				if errs := r.Errors(); len(errs) > 0 {
					t.Fatalf("Validate errors = %v, want none", errs)
				}
				return
			}
			found := false
			for _, issue := range r.Errors() {
				if issue.Code == tt.code && strings.HasPrefix(issue.Path, "/nodes/1/sensitivePaths") {
					found = true
				}
			}
			if !found {
				t.Errorf("Validate issues = %v, want error %s under /nodes/1/sensitivePaths", r.Issues, tt.code)
			}
		})
	}
	t.Run("absent member is valid and stays absent", func(t *testing.T) {
		t.Parallel()
		doc := load(t, "linear")
		if node(&doc, "fetch").SensitivePaths != nil {
			t.Fatal("conformance flow unexpectedly carries sensitive paths")
		}
		if _, ok := nodeValue(t, doc.Value(), "fetch").Get("sensitivePaths"); ok {
			t.Error("the writer emitted sensitivePaths for a node without marks")
		}
	})
}

func nodeValue(t *testing.T, doc jsonvalue.Value, id string) jsonvalue.Value {
	t.Helper()
	nodes, _ := doc.Get("nodes")
	for _, n := range nodes.Items() {
		if v, _ := n.Get("id"); !v.IsZero() {
			if s, _ := v.Str(); s == id {
				return n
			}
		}
	}
	t.Fatalf("no node %q", id)
	return jsonvalue.Value{}
}

func TestSensitivePathsReader(t *testing.T) {
	t.Parallel()
	t.Run("reads the member in document order", func(t *testing.T) {
		t.Parallel()
		doc, err := flow.Read(withSensitivePaths(t, stringArray("/b", "/a/*", "/c~1d")))
		if err != nil {
			t.Fatal(err)
		}
		got := node(&doc, "fetch").SensitivePaths
		want := []string{"/b", "/a/*", "/c~1d"}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("SensitivePaths = %q, want %q", got, want)
		}
	})
	for _, tt := range []struct {
		name   string
		member jsonvalue.Value
	}{
		{"empty array", jsonvalue.Array()},
		{"not an array", jsonvalue.String("/a")},
		{"element is a number", jsonvalue.Array(jsonvalue.Int(1))},
		{"element is an empty string", stringArray("")},
		{"null", jsonvalue.Null()},
	} {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := flow.Read(withSensitivePaths(t, tt.member)); err == nil {
				t.Error("Read accepted a malformed sensitivePaths member")
			}
		})
	}
	t.Run("round trip is byte for byte", func(t *testing.T) {
		t.Parallel()
		in := withSensitivePaths(t, stringArray("/z", "/a/*/b", "/m~0n"))
		doc, err := flow.Read(in)
		if err != nil {
			t.Fatal(err)
		}
		a, err := jsonvalue.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		b, err := jsonvalue.Marshal(doc.Value())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("writer output differs from the input:\n%s\n%s", a, b)
		}
	})
}

func TestSensitivePathsContentHash(t *testing.T) {
	t.Parallel()
	hash := func(t *testing.T, d *flow.Document) flow.Hash {
		t.Helper()
		h, err := flow.ContentHash(d)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	t.Run("adding a mark changes the hash", func(t *testing.T) {
		t.Parallel()
		a := load(t, "linear")
		b := load(t, "linear")
		node(&b, "fetch").SensitivePaths = []string{"/password"}
		if hash(t, &a) == hash(t, &b) {
			t.Error("a sensitive-path mark left contentHash unchanged")
		}
	})
	t.Run("a different mark changes the hash", func(t *testing.T) {
		t.Parallel()
		a := load(t, "linear")
		b := load(t, "linear")
		node(&a, "fetch").SensitivePaths = []string{"/password"}
		node(&b, "fetch").SensitivePaths = []string{"/token"}
		if hash(t, &a) == hash(t, &b) {
			t.Error("two different marks share a contentHash")
		}
	})
	t.Run("changing a node note does not change the hash", func(t *testing.T) {
		t.Parallel()
		a := load(t, "linear")
		b := load(t, "linear")
		node(&a, "fetch").SensitivePaths = []string{"/password"}
		node(&b, "fetch").SensitivePaths = []string{"/password"}
		edNodes, _ := b.Editor.Get("nodes")
		pos, _ := edNodes.Get("fetch")
		b.Editor = b.Editor.With("nodes", edNodes.With("fetch", pos.With("note", jsonvalue.String("calls the billing API"))))
		if _, ok := b.Editor.Get("nodes"); !ok {
			t.Fatal("editor lost its nodes member")
		}
		if hash(t, &a) != hash(t, &b) {
			t.Error("a node note moved contentHash")
		}
	})
}
