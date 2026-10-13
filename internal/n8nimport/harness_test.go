// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport_test

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/n8nimport"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// tpl builds a template document as the public library serves it.
func tpl(id, views int, nodes ...string) string {
	return fmt.Sprintf(`{"workflow":{"id":%d,"totalViews":%d,"workflow":{"nodes":[%s],"connections":{}}}}`,
		id, views, strings.Join(nodes, ","))
}

func node(name, typ string, version int, params string) string {
	if params == "" {
		params = "{}"
	}
	return fmt.Sprintf(`{"id":"id-%s","name":%q,"type":%q,"typeVersion":%d,"parameters":%s}`, name, name, typ, version, params)
}

const (
	manual  = "n8n-nodes-base.manualTrigger"
	setNode = "n8n-nodes-base.set"
	ifNode  = "n8n-nodes-base.if"
	code    = "n8n-nodes-base.code"
	slack   = "n8n-nodes-base.slack"
	gmail   = "n8n-nodes-base.gmail"
	mongo   = "n8n-nodes-base.mongoDb"
	chat    = "@n8n/n8n-nodes-langchain.lmChatOpenAi"
)

// fixture is a corpus of nine templates whose outcome is known by hand.
//
//	1 core only                                  importable
//	2 slack post                                 needs slack
//	3 slack post + gmail getAll                  needs slack and gmail
//	4 AI chat model + set                        only the AI design blocks it
//	5 community node                             no connector fixes it
//	6 slack with an operation chosen by expression  no connector fixes it
//	7 code with an expression the analyzer rejects  no connector fixes it
//	8 $fromAI expression in a set node           only the AI design blocks it
//	9 mongoDb find                               needs mongoDb (native)
func fixture() fstest.MapFS {
	slackPost := `{"resource":"message","operation":"post"}`
	return fstest.MapFS{
		"manifest.json":    {Data: []byte(`{"n8nVersion":"9.9.9","source":"fixture","fetchedAt":"2026-01-01T00:00:00Z","templates":9,"corpusSha256":"abc"}`)},
		"templates/1.json": {Data: []byte(tpl(1, 100, node("Start", manual, 1, ""), node("Set", setNode, 3, ""), node("If", ifNode, 2, ""), node("Note", "n8n-nodes-base.stickyNote", 1, "")))},
		"templates/2.json": {Data: []byte(tpl(2, 50, node("Start", manual, 1, ""), node("Post", slack, 2, slackPost)))},
		"templates/3.json": {Data: []byte(tpl(3, 30, node("Post", slack, 2, slackPost), node("Read", gmail, 2, `{"resource":"message","operation":"getAll"}`)))},
		"templates/4.json": {Data: []byte(tpl(4, 20, node("Model", chat, 1, ""), node("Set", setNode, 3, "")))},
		"templates/5.json": {Data: []byte(tpl(5, 10, node("Odd", "n8n-nodes-someone.odd", 1, "")))},
		"templates/6.json": {Data: []byte(tpl(6, 10, node("Post", slack, 2, `{"resource":"message","operation":"={{ $json.op }}"}`)))},
		"templates/7.json": {Data: []byte(tpl(7, 10, node("Code", code, 2, `{"value":"={{ $json.x.notARealMethod() }}"}`)))},
		"templates/8.json": {Data: []byte(tpl(8, 5, node("Set", setNode, 3, `{"value":"={{ $fromAI('city') }}"}`)))},
		"templates/9.json": {Data: []byte(tpl(9, 5, node("Find", mongo, 1, `{"operation":"find"}`)))},
	}
}

func loadFixture(t *testing.T) *n8nimport.Corpus {
	t.Helper()
	c, err := n8nimport.LoadCorpus(fixture())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCoverageHarness(t *testing.T) {
	c := loadFixture(t)
	if got := len(c.Templates); got != 9 {
		t.Fatalf("templates = %d, want 9", got)
	}
	if c.Manifest.N8nVersion != "9.9.9" || c.Manifest.CorpusSHA256 != "abc" {
		t.Fatalf("manifest = %+v", c.Manifest)
	}

	t.Run("core only", func(t *testing.T) {
		r := n8nimport.Measure(c, nil, "core")
		// Templates 1, 4 and 8 have no blocker besides the AI design; 4 and 8
		// use it. Views: 100 + 20 + 5 = 125 ignoring the AI design.
		want := n8nimport.Counts{
			Templates: 9, Importable: 1, ImportableIgnoringAI: 3, WithAI: 2, WithoutAI: 7,
			WithoutAIImportable: 1, WithAIImportableIgnoringAI: 2,
			Views: 240, ViewsImportable: 100, ViewsImportableIgnoringAI: 125,
		}
		if r.Counts != want {
			t.Fatalf("counts = %+v\nwant     %+v", r.Counts, want)
		}
		if got := n8nimport.Pct(int64(r.Counts.Importable), int64(r.Counts.Templates)); got != 11.11 {
			t.Errorf("unweighted = %v, want 11.11", got)
		}
		seen, mapped := r.CoreCoverage()
		if seen != mapped || seen == 0 {
			t.Errorf("core types seen %d mapped %d: every core type used must map", seen, mapped)
		}
		byReason := map[n8nimport.Reason]int{}
		for _, b := range r.Blockers {
			byReason[b.Reason] += b.Templates
		}
		for reason, want := range map[n8nimport.Reason]int{
			n8nimport.ReasonUnmappedType: 4, n8nimport.ReasonSelectorExpression: 1,
			n8nimport.ReasonCommunity: 1, n8nimport.ReasonExpression: 1,
		} {
			if byReason[reason] != want {
				t.Errorf("blockers %s = %d, want %d", reason, byReason[reason], want)
			}
		}
	})

	t.Run("a catalog with slack", func(t *testing.T) {
		cat, err := n8nimport.NewCatalog(n8nimport.Connector{
			ID: "slack", Effort: n8nimport.EffortDeclarative,
			Units: []n8nimport.UnitMap{{Unit: n8nimport.Unit{Type: slack, TypeVersion: "2", Resource: "message", Operation: "post"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		r := n8nimport.Measure(c, cat, "file")
		if r.Counts.Importable != 2 {
			t.Errorf("importable = %d, want 2 (templates 1 and 2)", r.Counts.Importable)
		}
		if r.Units.Used != 3 || r.Units.Mapped != 1 {
			t.Errorf("units used %d mapped %d, want 3 and 1", r.Units.Used, r.Units.Mapped)
		}
	})
}

func TestCoverageHarnessReportHoldsNoContent(t *testing.T) {
	c := loadFixture(t)
	sel, err := n8nimport.SelectWave(c, nil, n8nimport.SelectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r := n8nimport.Measure(c, sel.Catalog, "planned")
	data, err := r.JSON(sel)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data) + r.Markdown() + sel.RoadmapMarkdown(r, c.Manifest)
	// Names, expressions, parameter values and raw ids must not reach a
	// committed file.
	for _, leak := range []string{"Start", "Post", "notARealMethod", "$fromAI", "city", "\"id\":3,", "id-Post"} {
		if strings.Contains(string(data), leak) {
			t.Errorf("the JSON report contains %q", leak)
		}
	}
	if strings.Contains(text, "notARealMethod") || strings.Contains(text, "city") {
		t.Error("a rendered page contains expression text")
	}
	for _, want := range []string{n8nimport.ReportFormat, n8nimport.IDHash(1), "9.9.9", "abc"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("the JSON report lacks %q", want)
		}
	}
	again, err := r.JSON(sel)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, again) {
		t.Error("two renderings differ")
	}
}

func TestWaveSelection(t *testing.T) {
	c := loadFixture(t)
	sel, err := n8nimport.SelectWave(c, nil, n8nimport.SelectOptions{Target: 100})
	if err != nil {
		t.Fatal(err)
	}
	// Templates 1, 4 and 8 import apart from the AI design; 2, 3 and 9 are
	// reachable; 5, 6 and 7 hold a blocker no connector fixes.
	if sel.Start != 3 || sel.Ceiling != 6 || sel.Templates != 9 {
		t.Fatalf("start %d ceiling %d templates %d, want 3, 6, 9", sel.Start, sel.Ceiling, sel.Templates)
	}
	var order []string
	for _, st := range sel.Steps {
		order = append(order, strings.TrimPrefix(st.Type, "n8n-nodes-base."))
	}
	// slack completes template 2 (gain 1, effort 1); mongoDb completes
	// template 9 (gain 1, effort 3); gmail alone completes nothing until
	// slack is in, and then completes template 3.
	if want := "slack,gmail,mongoDb"; strings.Join(order, ",") != want {
		t.Fatalf("order = %v, want %s", order, want)
	}
	last := sel.Steps[len(sel.Steps)-1]
	if last.Importable != 6 || last.CumulativeEffort != 5 {
		t.Errorf("last step importable %d effort %d, want 6 and 5", last.Importable, last.CumulativeEffort)
	}
	if sel.Stop != n8nimport.StopNoProgress {
		t.Errorf("stop = %s, want %s", sel.Stop, n8nimport.StopNoProgress)
	}
	if got := sel.Steps[0]; got.Newly != 1 || got.Units != 1 || got.Effort != n8nimport.EffortDeclarative {
		t.Errorf("first step = %+v", got)
	}
	if got := sel.Steps[2]; got.Effort != n8nimport.EffortNative {
		t.Errorf("mongoDb effort = %s, want NATIVE", got.Effort)
	}

	// Measuring against the planned catalog gives the same totals.
	r := n8nimport.Measure(c, sel.Catalog, "planned")
	if r.Counts.ImportableIgnoringAI != last.Importable {
		t.Errorf("measured %d, selection says %d", r.Counts.ImportableIgnoringAI, last.Importable)
	}

	// The selection is deterministic.
	again, err := n8nimport.SelectWave(c, nil, n8nimport.SelectOptions{Target: 100})
	if err != nil {
		t.Fatal(err)
	}
	for i := range sel.Steps {
		if sel.Steps[i] != again.Steps[i] {
			t.Fatalf("step %d differs between runs", i)
		}
	}
}

func TestWaveSelectionStops(t *testing.T) {
	c := loadFixture(t)
	// 3 of 9 templates import at the start and 4 of 9 (44.44 %) once slack is in.
	sel, err := n8nimport.SelectWave(c, nil, n8nimport.SelectOptions{Target: 40})
	if err != nil {
		t.Fatal(err)
	}
	if sel.Stop != n8nimport.StopTarget || len(sel.Steps) != 1 {
		t.Errorf("stop %s after %d steps, want TARGET_REACHED after 1", sel.Stop, len(sel.Steps))
	}
	limited, err := n8nimport.SelectWave(c, nil, n8nimport.SelectOptions{Target: 100, MaxConnectors: 1})
	if err != nil {
		t.Fatal(err)
	}
	if limited.Stop != n8nimport.StopLimit || len(limited.Steps) != 1 {
		t.Errorf("stop %s after %d steps, want LIMIT_REACHED after 1", limited.Stop, len(limited.Steps))
	}
}

func TestWaveSelectionOwnEffort(t *testing.T) {
	c := loadFixture(t)
	// Making slack expensive moves mongoDb ahead of it.
	sel, err := n8nimport.SelectWave(c, nil, n8nimport.SelectOptions{
		Target: 100,
		EffortOf: func(typ string) n8nimport.Effort {
			if typ == slack {
				return n8nimport.EffortTriggerFamily
			}
			return n8nimport.EffortDeclarative
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := sel.Steps[0].Type; got != mongo {
		t.Errorf("first connector = %s, want %s", got, mongo)
	}
}

func TestWaveSelectionRunsOnTopOfACatalog(t *testing.T) {
	c := loadFixture(t)
	base, err := n8nimport.NewCatalog(n8nimport.Connector{
		ID: "slack", Effort: n8nimport.EffortDeclarative,
		Units: []n8nimport.UnitMap{{Unit: n8nimport.Unit{Type: slack, TypeVersion: "2", Resource: "message", Operation: "post"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sel, err := n8nimport.SelectWave(c, base, n8nimport.SelectOptions{Target: 100})
	if err != nil {
		t.Fatal(err)
	}
	if sel.Start != 4 {
		t.Errorf("start = %d, want 4", sel.Start)
	}
	for _, st := range sel.Steps {
		if st.Type == slack {
			t.Error("slack is already in the catalog and must not be selected again")
		}
	}
}

// TestCoverageHarnessRealCorpus runs the harness on a corpus fetched by
// tools/n8ncorpus when ORCON_N8N_CORPUS names its directory. Without it the
// test is skipped: the corpus is community content kept outside the
// repository.
func TestCoverageHarnessRealCorpus(t *testing.T) {
	dir := os.Getenv("ORCON_N8N_CORPUS")
	if dir == "" {
		t.Skip("ORCON_N8N_CORPUS is not set")
	}
	c, err := n8nimport.LoadCorpus(os.DirFS(dir))
	if err != nil {
		t.Fatal(err)
	}
	sel, err := n8nimport.SelectWave(c, nil, n8nimport.SelectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r := n8nimport.Measure(c, sel.Catalog, "planned")
	// A core type used by the corpus that has no counterpart blocks the gate.
	for _, ts := range r.Types {
		if ts.Class == n8nimport.ClassCore && !ts.Mapped && ts.Type != "n8n-nodes-base.dataTable" {
			t.Errorf("core type %s has no counterpart", ts.Type)
		}
	}
	if r.Counts.ImportableIgnoringAI < sel.Steps[len(sel.Steps)-1].Importable {
		t.Errorf("measuring the planned catalog gives %d, the selection says %d", r.Counts.ImportableIgnoringAI, sel.Steps[len(sel.Steps)-1].Importable)
	}
	if sel.Stop == n8nimport.StopTarget && float64(r.Counts.ImportableIgnoringAI)*100 < n8nimport.DefaultTarget*float64(r.Counts.Templates) {
		t.Errorf("selection claims the target with %d of %d templates", r.Counts.ImportableIgnoringAI, r.Counts.Templates)
	}
	t.Logf("%d templates: %.2f %% importable today, %.2f %% with the AI nodes mapped; wave 1 has %d connectors",
		r.Counts.Templates, n8nimport.Pct(int64(r.Counts.Importable), int64(r.Counts.Templates)),
		n8nimport.Pct(int64(r.Counts.ImportableIgnoringAI), int64(r.Counts.Templates)), len(sel.Steps))
}
