// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"cmp"
	"slices"
	"strings"
)

// TypeClass groups n8n node types for the report.
type TypeClass string

// Type classes (UPPER_SNAKE, they appear in the report).
const (
	// ClassCore is a node of the core table.
	ClassCore TypeClass = "CORE"
	// ClassIntegration is a built-in node of an application or protocol.
	ClassIntegration TypeClass = "INTEGRATION"
	// ClassAI is a node of the LangChain package.
	ClassAI TypeClass = "AI"
	// ClassCommunity is a node of a community package.
	ClassCommunity TypeClass = "COMMUNITY"
)

// ClassOf returns the class of a full n8n node type.
func ClassOf(nodeType string) TypeClass {
	typ, _ := normalizeType(nodeType)
	switch {
	case IsAIType(typ):
		return ClassAI
	case !IsBuiltin(typ):
		return ClassCommunity
	}
	if _, ok := LookupCore(typ); ok {
		return ClassCore
	}
	return ClassIntegration
}

// TypeStat counts the use of one node type in the corpus.
type TypeStat struct {
	Type        string
	Class       TypeClass
	Occurrences int
	Templates   int
	// Mapped is true for a core type with a counterpart and for an
	// integration type that has at least one unit in the catalog.
	Mapped bool
}

// BlockerStat counts the templates a reason on a type keeps from importing.
type BlockerStat struct {
	Reason    Reason
	Type      string
	Templates int
}

// Counts are the figures from which every percentage of the report derives.
type Counts struct {
	Templates int
	// Importable has no blocker with the measured catalog: an AI node counts
	// only if that catalog maps it.
	Importable int
	// WithAI and WithoutAI split the templates by whether they use an AI node,
	// a Tool variant or an AI-only expression. They are information only.
	WithAI    int
	WithoutAI int
	// WithAIImportable and WithoutAIImportable are the importable figure
	// inside each group.
	WithAIImportable    int
	WithoutAIImportable int
	Views               int64
	ViewsImportable     int64
	// StickyNotes is the number of sticky notes, which are editor data and
	// are not counted as nodes or types.
	StickyNotes int
}

// UnitCounts count the operations the corpus uses.
type UnitCounts struct {
	Used   int
	Mapped int
}

// Report is the outcome of measuring a corpus against a catalog. It is the
// source of the committed figures; nothing in it can reproduce a template.
type Report struct {
	Manifest   Manifest
	Catalog    string
	Unreadable int
	Counts     Counts
	Types      []TypeStat
	Units      UnitCounts
	Blockers   []BlockerStat
	Results    []Result
}

// Pct returns 100*num/den rounded to two decimals, or 0 when den is 0.
func Pct(num, den int64) float64 {
	if den == 0 {
		return 0
	}
	return float64(num*10000/den+boolInt64(num*10000%den*2 >= den)) / 100
}

func boolInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// Measure classifies every template of the corpus against the catalog.
func Measure(c *Corpus, cat *Catalog, catalogLabel string) *Report {
	r := &Report{Manifest: c.Manifest, Catalog: catalogLabel, Unreadable: c.Unreadable}
	r.Results = make([]Result, len(c.Templates))
	for i := range c.Templates {
		r.Results[i] = Classify(&c.Templates[i], cat)
	}
	r.Counts = countResults(r.Results)
	r.Types = typeStats(c, cat)
	r.Units = unitCounts(r.Results, cat)
	r.Blockers = blockerStats(r.Results)
	return r
}

func countResults(results []Result) Counts {
	var n Counts
	for i := range results {
		res := &results[i]
		n.Templates++
		n.Views += int64(res.Views)
		n.StickyNotes += res.StickyNotes
		if res.HasAI {
			n.WithAI++
		} else {
			n.WithoutAI++
		}
		if res.Importable() {
			n.Importable++
			n.ViewsImportable += int64(res.Views)
			if res.HasAI {
				n.WithAIImportable++
			} else {
				n.WithoutAIImportable++
			}
		}
	}
	return n
}

func typeStats(c *Corpus, cat *Catalog) []TypeStat {
	byType := map[string]*TypeStat{}
	for i := range c.Templates {
		seen := map[string]bool{}
		for j := range c.Templates[i].Nodes {
			typ, _ := normalizeType(c.Templates[i].Nodes[j].Type)
			if IsStickyNote(typ) {
				continue
			}
			s := byType[typ]
			if s == nil {
				s = &TypeStat{Type: typ, Class: ClassOf(typ)}
				switch s.Class {
				case ClassCore:
					row, _ := LookupCore(typ)
					s.Mapped = row.Mapped()
				case ClassIntegration:
					s.Mapped = cat.Has(typ)
				case ClassAI:
					s.Mapped = cat.MapsAI(typ)
				case ClassCommunity:
				}
				byType[typ] = s
			}
			s.Occurrences++
			if !seen[typ] {
				seen[typ] = true
				s.Templates++
			}
		}
	}
	out := make([]TypeStat, 0, len(byType))
	for _, s := range byType {
		out = append(out, *s)
	}
	slices.SortFunc(out, func(a, b TypeStat) int {
		return cmp.Or(cmp.Compare(b.Occurrences, a.Occurrences), strings.Compare(a.Type, b.Type))
	})
	return out
}

func unitCounts(results []Result, cat *Catalog) UnitCounts {
	seen := map[Unit]bool{}
	var u UnitCounts
	for i := range results {
		for _, unit := range results[i].Units {
			if seen[unit] {
				continue
			}
			seen[unit] = true
			u.Used++
			if _, _, ok := cat.Lookup(unit); ok {
				u.Mapped++
			}
		}
	}
	return u
}

func blockerStats(results []Result) []BlockerStat {
	type key struct {
		reason Reason
		typ    string
	}
	count := map[key]int{}
	for i := range results {
		seen := map[key]bool{}
		for _, b := range results[i].Blockers {
			k := key{b.Reason, b.Unit.Type}
			if !seen[k] {
				seen[k] = true
				count[k]++
			}
		}
	}
	out := make([]BlockerStat, 0, len(count))
	for k, n := range count {
		out = append(out, BlockerStat{Reason: k.reason, Type: k.typ, Templates: n})
	}
	slices.SortFunc(out, func(a, b BlockerStat) int {
		return cmp.Or(cmp.Compare(b.Templates, a.Templates), strings.Compare(string(a.Reason), string(b.Reason)), strings.Compare(a.Type, b.Type))
	})
	return out
}

// CoreCoverage reports, for the core types seen in the corpus, how many have
// a counterpart; the release gate wants all of them.
func (r *Report) CoreCoverage() (seen, mapped int) {
	for _, t := range r.Types {
		if t.Class == ClassCore {
			seen++
			if t.Mapped {
				mapped++
			}
		}
	}
	return seen, mapped
}
