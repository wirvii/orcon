// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package configyaml_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/adapter/codec/configyaml"
	"github.com/wirvii/orcon/internal/platform/config"
)

func decode(t *testing.T, doc string) ([]config.FileEntry, error) {
	t.Helper()
	return configyaml.Decode([]byte(doc), config.Default().IsLeaf)
}

func TestDecodeKeepsLinesAndFlattensSections(t *testing.T) {
	t.Parallel()

	doc := `# comment
role: api
store:
  driver: postgres
  postgres:
    dsnFile: /run/secrets/dsn
    maxConnections: 30
http:
  trustedProxies: [10.0.0.0/8, "192.168.0.0/16"]
observability:
  otlp:
    headers:
      Authorization: Bearer t
log:
  level:
`
	got, err := decode(t, doc)
	if err != nil {
		t.Fatal(err)
	}
	want := []config.FileEntry{
		{Path: "role", Line: 2, Scalar: "api"},
		{Path: "store.driver", Line: 4, Scalar: "postgres"},
		{Path: "store.postgres.dsnFile", Line: 6, Scalar: "/run/secrets/dsn"},
		{Path: "store.postgres.maxConnections", Line: 7, Scalar: "30"},
		{Path: "http.trustedProxies", Line: 9, Kind: config.EntryList, List: []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{Path: "observability.otlp.headers", Line: 12, Kind: config.EntryMap, Map: map[string]string{"Authorization": "Bearer t"}},
		{Path: "log.level", Line: 15, Kind: config.EntryNull},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("entries mismatch (-want +got):\n%s", diff)
	}
}

func TestDecodeKeepsScalarTextAsWritten(t *testing.T) {
	t.Parallel()

	got, err := decode(t, "updateCheck:\n  enabled: true\n  interval: 24h\nruntime:\n  memoryLimitRatio: 0.90\n")
	if err != nil {
		t.Fatal(err)
	}
	texts := map[string]string{}
	for _, e := range got {
		texts[e.Path] = e.Scalar
	}
	want := map[string]string{"updateCheck.enabled": "true", "updateCheck.interval": "24h", "runtime.memoryLimitRatio": "0.90"}
	if diff := cmp.Diff(want, texts); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestDecodeEmptyDocuments(t *testing.T) {
	t.Parallel()

	for _, doc := range []string{"", "\n", "# only a comment\n", "---\n", "null\n"} {
		got, err := decode(t, doc)
		if err != nil || len(got) != 0 {
			t.Errorf("Decode(%q) = %v, %v; want no entries and no error", doc, got, err)
		}
	}
}

func TestDecodeRejectsUnsafeOrBrokenDocuments(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, doc, match string }{
		{"syntax", "role: [api\n", "invalid YAML"},
		{"not a mapping", "- a\n- b\n", "must be a mapping"},
		{"scalar document", "hello\n", "must be a mapping"},
		{"two documents", "role: api\n---\nrole: worker\n", "more than one"},
		{"duplicate key", "role: api\nrole: worker\n", "set twice"},
		{"anchor", "base: &b {role: api}\nrole: api\n", "anchors and aliases"},
		{"alias", "a: &x 1\nb: *x\n", "anchors and aliases"},
		{"merge key", "<<: {role: api}\n", "merge keys"},
		{"dotted name", "store.driver: sqlite\n", "not a valid setting name"},
		{"complex key", "? [a, b]\n: c\n", "plain words"},
		{"list of maps", "http:\n  trustedProxies:\n    - {a: b}\n", "plain values"},
		{"nested map value", "observability:\n  otlp:\n    headers:\n      a: {b: c}\n", "plain name: value pairs"},
		{"too deep", deepDocument(40), "nest too deep"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := decode(t, tt.doc)
			if err == nil || !strings.Contains(err.Error(), tt.match) {
				t.Fatalf("Decode = %v, %v; want an error containing %q", got, err, tt.match)
			}
			if got != nil {
				t.Errorf("entries returned with an error: %v", got)
			}
		})
	}
}

func TestDecodeReportsUnknownKeysToTheLoader(t *testing.T) {
	t.Parallel()

	got, err := decode(t, "bogus:\n  deep:\n    x: 1\nhttp: oops\n")
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, e := range got {
		paths = append(paths, e.Path)
	}
	if diff := cmp.Diff([]string{"bogus.deep.x", "http"}, paths); diff != "" {
		t.Fatalf("paths mismatch (-want +got):\n%s", diff)
	}
}

func TestLoadWithTheRealCodec(t *testing.T) {
	t.Parallel()

	files := memFS{"/c.yaml": "store:\n  driver: postgres\n  postgres:\n    dsnFile: /s\nunknown: 1\npublicUrl: https://o.example.com\n", "/s": "postgres://h/db\n"}
	res := config.Default().Load(config.Input{
		Flags:   &config.FlagValues{Config: "/c.yaml"},
		Environ: []string{"HOME=/h", "ORCON_ROLE=api"},
		FS:      files,
		Decode:  configyaml.Decode,
		GOOS:    "linux",
	})
	var msgs []string
	for _, i := range res.Issues {
		msgs = append(msgs, i.Key+": "+i.Message)
	}
	if len(res.Issues) != 1 || !strings.Contains(msgs[0], "/c.yaml:5: unknown key") {
		t.Fatalf("issues = %v, want only the unknown key at line 5", msgs)
	}
	if got := res.Config.Secret("store.postgres.dsn").Reveal(); got != "postgres://h/db" {
		t.Errorf("dsn = %q", got)
	}
}

func TestMarshal(t *testing.T) {
	t.Parallel()

	out, err := configyaml.Marshal(map[string]any{"b": []string{"x"}, "a": map[string]int{"count": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if want := "a:\n  count: 1\nb:\n  - x\n"; string(out) != want {
		t.Fatalf("Marshal = %q, want %q", out, want)
	}
}

func FuzzDecode(f *testing.F) {
	for _, seed := range []string{"role: api\n", "a:\n  b: [1, 2]\n", "x: &a 1\ny: *a\n", "? [a]\n: b\n", "---\na: 1\n---\nb: 2\n", "a: {b: c}\n"} {
		f.Add([]byte(seed))
	}
	isLeaf := config.Default().IsLeaf
	f.Fuzz(func(t *testing.T, data []byte) {
		entries, err := configyaml.Decode(data, isLeaf)
		if err != nil && entries != nil {
			t.Fatalf("entries returned with an error: %v", err)
		}
		for _, e := range entries {
			if e.Path == "" || e.Line < 1 {
				t.Fatalf("bad entry %+v", e)
			}
		}
	})
}

// deepDocument nests n sections, one level of indentation each.
func deepDocument(n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteString(strings.Repeat(" ", i))
		b.WriteString("a:\n")
	}
	return b.String()
}
