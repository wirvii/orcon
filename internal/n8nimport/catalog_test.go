// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport_test

import (
	"bytes"
	"testing"
	"testing/fstest"

	"github.com/wirvii/orcon/internal/n8nimport"
)

func TestCatalogRoundTrip(t *testing.T) {
	cat := slackCatalog(t)
	data, err := cat.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := n8nimport.ParseCatalog(data)
	if err != nil {
		t.Fatal(err)
	}
	again, err := back.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, again) {
		t.Fatalf("encoding is not stable:\n%s\n%s", data, again)
	}
	u := n8nimport.Unit{Type: slack, TypeVersion: "2.2", Resource: "message", Operation: "post"}
	conn, op, ok := back.Lookup(u)
	if !ok || conn != "slack" || op != "message.send" {
		t.Errorf("lookup = %q %q %v", conn, op, ok)
	}
	if k, ok := back.Connector(slack); !ok || k.OrconType != "slack.action@1" {
		t.Errorf("connector = %+v %v", k, ok)
	}
	if o, ok := back.Credential("slackApi"); !ok || o != "slack.token" {
		t.Errorf("credential = %q %v", o, ok)
	}
}

func TestCatalogRejects(t *testing.T) {
	tests := map[string]string{
		"not json":         `{`,
		"no connectors":    `{}`,
		"bad effort":       `{"connectors":[{"id":"x","effort":"HUGE","units":[]}]}`,
		"no id":            `{"connectors":[{"effort":"NATIVE","units":[]}]}`,
		"unit without ver": `{"connectors":[{"id":"x","effort":"NATIVE","units":[{"type":"n8n-nodes-base.x"}]}]}`,
		"duplicate unit": `{"connectors":[
		  {"id":"a","effort":"NATIVE","units":[{"type":"n8n-nodes-base.x","typeVersion":"1"}]},
		  {"id":"b","effort":"NATIVE","units":[{"type":"n8n-nodes-base.x","typeVersion":"1"}]}]}`,
	}
	for name, doc := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := n8nimport.ParseCatalog([]byte(doc)); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestCatalogNilAndMerge(t *testing.T) {
	var nilCat *n8nimport.Catalog
	u := n8nimport.Unit{Type: slack, TypeVersion: "2.2"}
	if _, _, ok := nilCat.Lookup(u); ok || nilCat.Has(slack) || nilCat.HasVersion(slack, "2.2") {
		t.Error("a nil catalog holds nothing")
	}
	if _, ok := nilCat.Connector(slack); ok {
		t.Error("a nil catalog has no connector")
	}
	if _, ok := nilCat.Credential("slackApi"); ok {
		t.Error("a nil catalog has no credential")
	}
	merged, err := n8nimport.Merge(nilCat, slackCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	if !merged.Has(slack) {
		t.Error("the merge lost slack")
	}
	if _, err := n8nimport.Merge(slackCatalog(t), slackCatalog(t)); err == nil {
		t.Error("merging a catalog with itself must fail: a unit has two mappings")
	}
	if got := n8nimport.EffortNative.Weight() + n8nimport.EffortTriggerFamily.Weight() + n8nimport.EffortDeclarative.Weight(); got != 7 {
		t.Errorf("weights add up to %d, want 3+3+1", got)
	}
}

func TestParseTemplateShapes(t *testing.T) {
	nodes := `"nodes":[{"name":"A","type":"n8n-nodes-base.set","typeVersion":3.4,"disabled":true,"parameters":{"a":1}}]`
	tests := []struct {
		name  string
		doc   string
		id    int
		views int
	}{
		{"library", `{"workflow":{"id":7,"totalViews":9,"workflow":{` + nodes + `}}}`, 7, 9},
		{"wrapper without the inner workflow", `{"workflow":{"id":8,"totalViews":1,` + nodes + `}}`, 8, 1},
		{"plain export", `{"id":5,` + nodes + `}`, 5, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tp, err := n8nimport.ParseTemplate([]byte(tc.doc))
			if err != nil {
				t.Fatal(err)
			}
			if tp.ID != tc.id || tp.Views != tc.views || len(tp.Nodes) != 1 {
				t.Fatalf("template = %+v", tp)
			}
			n := tp.Nodes[0]
			if n.TypeVersion != "3.4" || !n.Disabled || n.Name != "A" || n.IsAI() {
				t.Errorf("node = %+v", n)
			}
		})
	}
	for name, doc := range map[string]string{
		"not json":     `{`,
		"no nodes":     `{"workflow":{"workflow":{}}}`,
		"node no type": `{"nodes":[{"name":"A"}]}`,
		"node scalar":  `{"nodes":[3]}`,
	} {
		if _, err := n8nimport.ParseTemplate([]byte(doc)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestLoadCorpus(t *testing.T) {
	good := tpl(2, 1, node("A", manual, 1, ""))
	t.Run("sorted, with unreadable files counted", func(t *testing.T) {
		c, err := n8nimport.LoadCorpus(fstest.MapFS{
			"templates/10.json":   {Data: []byte(tpl(0, 1, node("A", manual, 1, "")))},
			"templates/2.json":    {Data: []byte(good)},
			"templates/bad.json":  {Data: []byte(`{`)},
			"templates/notes.txt": {Data: []byte(`ignored`)},
			"manifest.json":       {Data: []byte(`not json`)},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Templates) != 2 || c.Unreadable != 1 {
			t.Fatalf("templates %d unreadable %d", len(c.Templates), c.Unreadable)
		}
		// The id comes from the file name when the document has none.
		if c.Templates[0].ID != 2 || c.Templates[1].ID != 10 {
			t.Errorf("ids = %d, %d, want 2, 10", c.Templates[0].ID, c.Templates[1].ID)
		}
		if c.Manifest.N8nVersion != "" {
			t.Errorf("an unreadable manifest must leave the manifest empty, got %+v", c.Manifest)
		}
	})
	t.Run("empty", func(t *testing.T) {
		_, err := n8nimport.LoadCorpus(fstest.MapFS{"templates/x.json": {Data: []byte(`{`)}})
		if err == nil {
			t.Fatal("a corpus with no readable template must fail")
		}
	})
	t.Run("no templates directory", func(t *testing.T) {
		if _, err := n8nimport.LoadCorpus(fstest.MapFS{}); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestClassification(t *testing.T) {
	tests := []struct {
		typ   string
		class n8nimport.TypeClass
	}{
		{"n8n-nodes-base.if", n8nimport.ClassCore},
		{"n8n-nodes-base.httpRequestTool", n8nimport.ClassCore},
		{"n8n-nodes-base.slack", n8nimport.ClassIntegration},
		{"n8n-nodes-base.gmailTool", n8nimport.ClassIntegration},
		{"n8n-nodes-base.openAi", n8nimport.ClassAI},
		{"@n8n/n8n-nodes-langchain.agent", n8nimport.ClassAI},
		{"n8n-nodes-someone.odd", n8nimport.ClassCommunity},
	}
	for _, tc := range tests {
		if got := n8nimport.ClassOf(tc.typ); got != tc.class {
			t.Errorf("ClassOf(%s) = %s, want %s", tc.typ, got, tc.class)
		}
	}
}

func TestClassifyToolVariantCountsAsAI(t *testing.T) {
	tp, err := n8nimport.ParseTemplate([]byte(`{"nodes":[
	  {"name":"Tool","type":"n8n-nodes-base.httpRequestTool","typeVersion":4,"parameters":{}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	res := n8nimport.Classify(&tp, nil)
	if !res.HasAI || !res.ImportableIgnoringAI() || res.Importable() || res.Fixed() {
		t.Errorf("result = %+v", res)
	}
}

func TestPctRounds(t *testing.T) {
	tests := []struct {
		n, d int64
		want float64
	}{{1, 3, 33.33}, {2, 3, 66.67}, {0, 0, 0}, {9, 10, 90}, {1, 8, 12.5}, {1, 200, 0.5}}
	for _, tc := range tests {
		if got := n8nimport.Pct(tc.n, tc.d); got != tc.want {
			t.Errorf("Pct(%d, %d) = %v, want %v", tc.n, tc.d, got, tc.want)
		}
	}
}

func TestIDHashIsStableAndDoesNotRevealTheID(t *testing.T) {
	first, second, other := n8nimport.IDHash(1234), n8nimport.IDHash(1234), n8nimport.IDHash(1235)
	if first != second || first == other {
		t.Fatal("hash is not a function of the id")
	}
	if len(n8nimport.IDHash(1234)) != 16 {
		t.Errorf("hash %q is not 16 hex digits", n8nimport.IDHash(1234))
	}
}
