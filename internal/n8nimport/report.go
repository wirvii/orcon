// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// ReportFormat tags the JSON report.
const ReportFormat = "orcon/n8n-coverage@1"

// maxBlockerRows bounds the blocker rows of the report.
const maxBlockerRows = 200

// IDHash returns the stable hash that stands for a template id in the
// report: the committed figures never name a template.
func IDHash(id int) string {
	sum := sha256.Sum256([]byte("orcon/n8n-template/" + strconv.Itoa(id)))
	return hex.EncodeToString(sum[:8])
}

func obj(members ...jsonvalue.Member) jsonvalue.Value {
	v, err := jsonvalue.Object(members...)
	if err != nil {
		// Every caller passes distinct literal keys.
		return jsonvalue.Value{}
	}
	return v
}

func m(key string, v jsonvalue.Value) jsonvalue.Member { return jsonvalue.Member{Key: key, Value: v} }

func num(n int) jsonvalue.Value { return jsonvalue.Int(int64(n)) }

func score(f float64) jsonvalue.Value {
	v, err := jsonvalue.Float(f)
	if err != nil {
		return jsonvalue.Int(0)
	}
	return v
}

// JSON renders the report, and the selection when sel is not nil, as the
// machine-readable file of ADR-0037 section 3. It holds hashes, counts and
// scores only.
func (r *Report) JSON(sel *Selection) ([]byte, error) {
	c := r.Counts
	seenIntegration, mappedIntegration := 0, 0
	for _, t := range r.Types {
		if t.Class == ClassIntegration {
			seenIntegration++
			if t.Mapped {
				mappedIntegration++
			}
		}
	}
	coreSeen, coreMapped := r.CoreCoverage()

	var types []jsonvalue.Value
	aiTypes, aiNodes := 0, 0
	for _, t := range r.Types {
		if t.Class == ClassAI {
			aiTypes++
			aiNodes += t.Occurrences
			continue
		}
		types = append(types, obj(m("type", jsonvalue.String(t.Type)), m("class", jsonvalue.String(string(t.Class))),
			m("occurrences", num(t.Occurrences)), m("templates", num(t.Templates)), m("mapped", jsonvalue.Bool(t.Mapped))))
	}
	var blockers []jsonvalue.Value
	for _, b := range r.Blockers {
		if len(blockers) == maxBlockerRows {
			break
		}
		if ClassOf(b.Type) == ClassAI {
			continue
		}
		blockers = append(blockers, obj(m("reason", jsonvalue.String(string(b.Reason))), m("type", jsonvalue.String(b.Type)), m("templates", num(b.Templates))))
	}
	statuses := make([]jsonvalue.Value, len(r.Results))
	for i := range r.Results {
		res := &r.Results[i]
		statuses[i] = obj(m("idHash", jsonvalue.String(IDHash(res.ID))), m("views", num(res.Views)), m("ai", jsonvalue.Bool(res.HasAI)),
			m("importable", jsonvalue.Bool(res.Importable())), m("blockers", num(len(res.Blockers))))
	}

	members := []jsonvalue.Member{
		m("format", jsonvalue.String(ReportFormat)),
		m("n8nVersion", jsonvalue.String(r.Manifest.N8nVersion)),
		m("source", jsonvalue.String(r.Manifest.Source)),
		m("fetchedAt", jsonvalue.String(r.Manifest.FetchedAt)),
		m("corpusSha256", jsonvalue.String(r.Manifest.CorpusSHA256)),
		m("catalog", jsonvalue.String(r.Catalog)),
		m("templates", num(c.Templates)),
		m("unreadable", num(r.Unreadable)),
		m("stickyNotes", num(c.StickyNotes)),
		m("metrics", obj(
			m("unweighted", obj(
				m("importable", score(Pct(int64(c.Importable), int64(c.Templates)))),
			)),
			m("viewWeighted", obj(
				m("importable", score(Pct(c.ViewsImportable, c.Views))),
			)),
			m("withoutAi", obj(
				m("templates", num(c.WithoutAI)),
				m("importable", score(Pct(int64(c.WithoutAIImportable), int64(c.WithoutAI)))),
			)),
			m("withAi", obj(
				m("templates", num(c.WithAI)),
				m("importable", score(Pct(int64(c.WithAIImportable), int64(c.WithAI)))),
			)),
			m("integrationTypes", obj(m("seen", num(seenIntegration)), m("mapped", num(mappedIntegration)),
				m("percent", score(Pct(int64(mappedIntegration), int64(seenIntegration)))))),
			m("coreTypes", obj(m("seen", num(coreSeen)), m("mapped", num(coreMapped)),
				m("percent", score(Pct(int64(coreMapped), int64(coreSeen)))))),
			m("operations", obj(m("used", num(r.Units.Used)), m("mapped", num(r.Units.Mapped)),
				m("percent", score(Pct(int64(r.Units.Mapped), int64(r.Units.Used)))))),
		)),
		m("aiNodes", obj(m("types", num(aiTypes)), m("nodes", num(aiNodes)))),
		m("nodeTypes", jsonvalue.Array(types...)),
		m("blockers", jsonvalue.Array(blockers...)),
		m("templateStatus", jsonvalue.Array(statuses...)),
	}
	if sel != nil {
		members = append(members, m("selection", sel.value()))
	}
	return jsonvalue.Marshal(obj(members...))
}

func (s *Selection) value() jsonvalue.Value {
	steps := make([]jsonvalue.Value, len(s.Steps))
	for i, st := range s.Steps {
		steps[i] = obj(m("rank", num(st.Rank)), m("type", jsonvalue.String(st.Type)), m("effort", jsonvalue.String(string(st.Effort))),
			m("units", num(st.Units)), m("newlyImportable", num(st.Newly)), m("templates", num(st.Templates)),
			m("occurrences", num(st.Occurrences)), m("importable", num(st.Importable)),
			m("percent", score(st.Percent)), m("cumulativeEffort", num(st.CumulativeEffort)))
	}
	return obj(m("target", score(s.Target)), m("start", num(s.Start)), m("ceiling", num(s.Ceiling)),
		m("ceilingPercent", score(Pct(int64(s.Ceiling), int64(s.Templates)))),
		m("stop", jsonvalue.String(string(s.Stop))), m("steps", jsonvalue.Array(steps...)))
}

const spdxMarkdown = "<!-- SPDX-FileCopyrightText: The Orcon Authors -->\n<!-- SPDX-License-Identifier: Apache-2.0 -->\n"

// Markdown renders the coverage page (docs/reference/connector-coverage.md).
// The figures are those of the catalog the report was measured against.
func (r *Report) Markdown() string {
	c := r.Counts
	var b strings.Builder
	b.WriteString(spdxMarkdown)
	b.WriteString("# n8n connector coverage\n\n")
	b.WriteString("Generated by `orcon n8n coverage`; do not edit by hand. Method: ADR-0037 sections 2 and 3.\n\n")
	fmt.Fprintf(&b, "Corpus: %d templates of the public n8n template library (most viewed first), measured against n8n %s, fetched %s, corpus digest `%s`.", c.Templates, orNone(r.Manifest.N8nVersion), orNone(r.Manifest.FetchedAt), orNone(r.Manifest.CorpusSHA256))
	if r.Unreadable > 0 {
		fmt.Fprintf(&b, " %d files could not be read and are left out.", r.Unreadable)
	}
	fmt.Fprintf(&b, "\n\nCatalog measured: **%s**. Only counts, hashes and scores are published; the templates are community content and stay outside the repository.\n\n", r.Catalog)
	b.WriteString("A template is fully importable when every node maps, every operation it selects is in the catalog, no selector is an expression and every expression passes the analyzer of ADR-0018. An AI (LangChain) node counts as mapped only if the measured catalog maps it. Core nodes are counted at the type level: the translation of their parameters belongs to the cut that implements each node. Sticky notes are editor data and are not counted as nodes or types.\n\n")
	b.WriteString("| Measure | Value |\n|---|---|\n")
	row := func(name, value string) { fmt.Fprintf(&b, "| %s | %s |\n", name, value) }
	label := "Fully importable (release gate figure, unweighted)"
	if r.Catalog != "installed" {
		label = "Fully importable with the " + r.Catalog + " catalog (not a release figure, unweighted)"
	}
	row(label, pctStr(Pct(int64(c.Importable), int64(c.Templates)))+fmt.Sprintf(" (%d of %d)", c.Importable, c.Templates))
	row("Fully importable, weighted by views (information only)", pctStr(Pct(c.ViewsImportable, c.Views)))
	row("Templates without AI nodes", fmt.Sprintf("%d, of which %s importable", c.WithoutAI, pctStr(Pct(int64(c.WithoutAIImportable), int64(c.WithoutAI)))))
	row("Templates with AI nodes", fmt.Sprintf("%d, of which %s importable", c.WithAI, pctStr(Pct(int64(c.WithAIImportable), int64(c.WithAI)))))
	seen, mapped := 0, 0
	for _, t := range r.Types {
		if t.Class == ClassIntegration {
			seen++
			if t.Mapped {
				mapped++
			}
		}
	}
	row("Built-in integration node types used, with a connector", fmt.Sprintf("%d of %d (%s)", mapped, seen, pctStr(Pct(int64(mapped), int64(seen)))))
	coreSeen, coreMapped := r.CoreCoverage()
	row("Core node types used, with a counterpart", fmt.Sprintf("%d of %d (%s)", coreMapped, coreSeen, pctStr(Pct(int64(coreMapped), int64(coreSeen)))))
	row("Operations used (type, version, resource, operation), mapped", fmt.Sprintf("%d of %d (%s)", r.Units.Mapped, r.Units.Used, pctStr(Pct(int64(r.Units.Mapped), int64(r.Units.Used)))))
	row("Sticky notes (editor data)", strconv.Itoa(c.StickyNotes))
	aiTypes, aiNodes, aiMapped := 0, 0, 0
	b.WriteString("\n## Node types by use\n\n| Type | Class | Nodes | Templates | Mapped |\n|---|---|---|---|---|\n")
	shown := 0
	for _, t := range r.Types {
		if t.Class == ClassAI {
			aiTypes++
			aiNodes += t.Occurrences
			if t.Mapped {
				aiMapped++
			}
			continue
		}
		if shown < 60 {
			shown++
			fmt.Fprintf(&b, "| `%s` | %s | %d | %d | %s |\n", t.Type, t.Class, t.Occurrences, t.Templates, yesNo(t.Mapped))
		}
	}
	fmt.Fprintf(&b, "\nThe AI nodes (%d types, %d nodes, %d types mapped by this catalog) are not listed: their mapping is in `n8n-ai-coverage.md`.\n", aiTypes, aiNodes, aiMapped)
	b.WriteString("\n## What blocks templates most\n\n| Reason | Type | Templates |\n|---|---|---|\n")
	shown = 0
	for _, bl := range r.Blockers {
		if shown == 40 {
			break
		}
		if ClassOf(bl.Type) == ClassAI {
			continue
		}
		shown++
		fmt.Fprintf(&b, "| %s | `%s` | %d |\n", bl.Reason, bl.Type, bl.Templates)
	}
	return b.String()
}

// RoadmapMarkdown renders the wave-1 selection page
// (docs/reference/connector-roadmap.md). Every figure on it is planned: the
// report passed in was measured against the planned catalog, which maps the
// AI nodes as SPEC-006 Annex B commits and the connectors this selection
// chooses.
func (s *Selection) RoadmapMarkdown(planned *Report, manifest Manifest) string {
	var b strings.Builder
	b.WriteString(spdxMarkdown)
	b.WriteString("# Connector roadmap: wave 1\n\n")
	b.WriteString("Generated by `orcon n8n coverage --catalog planned`; do not edit by hand. Method: ADR-0037 section 4 (greedy selection by marginal templates that become fully importable, per unit of estimated effort). Every figure on this page is **planned**, not the coverage of a release: the actual coverage is in `connector-coverage.md`.\n\n")
	fmt.Fprintf(&b, "Corpus: %d templates, n8n %s, digest `%s`. Target: %s of the templates fully importable under the planned catalog. The planned catalog maps the AI nodes that SPEC-006 Annex B names (rows S and A); LangChain Code and the AI types the annex does not name stay unmapped and are counted below.\n\n",
		s.Templates, orNone(manifest.N8nVersion), orNone(manifest.CorpusSHA256), pctStr(s.Target))
	fmt.Fprintf(&b, "- Planned, before any connector (core nodes and planned AI mapping): %d templates (%s).\n", s.Start, pctStr(Pct(int64(s.Start), int64(s.Templates))))
	fmt.Fprintf(&b, "- Planned ceiling for connectors: %d templates (%s). The rest hold a node no connector fixes: a community node, an AI type without a planned mapping, an expression the analyzer rejects, an operation chosen by expression, or a core node without a counterpart.\n", s.Ceiling, pctStr(Pct(int64(s.Ceiling), int64(s.Templates))))
	fmt.Fprintf(&b, "- Selection stopped because: %s.\n", stopText(s.Stop))
	last := s.Start
	if n := len(s.Steps); n > 0 {
		last = s.Steps[n-1].Importable
	}
	fmt.Fprintf(&b, "- Connectors selected: %d, reaching %d templates (planned %s) for an estimated effort of %d.\n", len(s.Steps), last, pctStr(Pct(int64(last), int64(s.Templates))), cumulative(s))
	if planned != nil {
		c := planned.Counts
		fmt.Fprintf(&b, "- Planned, weighted by views: %s. Planned, templates without AI nodes: %s importable; with AI nodes: %s importable.\n",
			pctStr(Pct(c.ViewsImportable, c.Views)), pctStr(Pct(int64(c.WithoutAIImportable), int64(c.WithoutAI))), pctStr(Pct(int64(c.WithAIImportable), int64(c.WithAI))))
	}
	b.WriteString("\nEffort is an estimate for ordering only: declarative connector 1, native connector or new trigger family 3. The connector author decides the real class.\n\n")
	b.WriteString("| # | n8n type | Effort | Operations | New templates | Templates using it | Planned cumulative importable | Cumulative effort |\n|---|---|---|---|---|---|---|---|\n")
	for _, st := range s.Steps {
		fmt.Fprintf(&b, "| %d | `%s` | %s | %d | %d | %d | %s | %d |\n", st.Rank, st.Type, st.Effort, st.Units, st.Newly, st.Templates, pctStr(st.Percent), st.CumulativeEffort)
	}
	if planned != nil {
		unmapped := 0
		for _, t := range planned.Types {
			if t.Class == ClassAI && !t.Mapped {
				unmapped++
			}
		}
		fmt.Fprintf(&b, "\n%d AI node types of the corpus have no planned mapping (SPEC-006 Annex B does not name them, or marks them unsupported); they are not listed here.\n", unmapped)
		b.WriteString("\n## What stays blocked under the planned catalog\n\n| Reason | Type | Templates |\n|---|---|---|\n")
		n := 0
		for _, bl := range planned.Blockers {
			if n == 25 {
				break
			}
			if ClassOf(bl.Type) == ClassAI {
				continue
			}
			n++
			fmt.Fprintf(&b, "| %s | `%s` | %d |\n", bl.Reason, bl.Type, bl.Templates)
		}
	}
	return b.String()
}

func cumulative(s *Selection) int {
	if n := len(s.Steps); n > 0 {
		return s.Steps[n-1].CumulativeEffort
	}
	return 0
}

func stopText(r StopReason) string {
	switch r {
	case StopTarget:
		return "the target was reached"
	case StopNoProgress:
		return "no connector brings any template closer to importing"
	case StopLimit:
		return "the connector limit was reached"
	default:
		return string(r)
	}
}

func pctStr(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) + " %" }

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func orNone(s string) string {
	if s == "" {
		return "(not stated)"
	}
	return s
}
