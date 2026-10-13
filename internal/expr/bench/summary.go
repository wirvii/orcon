// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bench

import (
	"cmp"
	"slices"
)

// Tally counts the expressions that share a reason.
type Tally struct {
	Reason Reason
	Count  int
	// Weight is the total views of the templates of those expressions.
	Weight float64
}

// Rate is a count out of a total, plain and weighted by template usage.
type Rate struct {
	Count, Total            int
	Weighted, WeightedTotal float64
}

// Percent returns the unweighted share in percent.
func (r Rate) Percent() float64 { return percent(float64(r.Count), float64(r.Total)) }

// WeightedPercent returns the usage-weighted share in percent.
func (r Rate) WeightedPercent() float64 { return percent(r.Weighted, r.WeightedTotal) }

func percent(part, whole float64) float64 {
	if whole == 0 {
		return 0
	}
	return 100 * part / whole
}

// Summary aggregates the verdicts of a corpus.
type Summary struct {
	// Expressions is the number of expressions with at least one {{ }} segment.
	Expressions int
	// Templates is the number of distinct templates they come from.
	Templates int
	// All is the share that runs unchanged out of every expression.
	All Rate
	// WithoutDeferred leaves out the expressions whose only reasons are
	// identifiers owned by a later specification ($fromAI).
	WithoutDeferred Rate
	// WithoutDeferredOrPolicy also leaves out those refused on purpose ($env).
	WithoutDeferredOrPolicy Rate
	// Fast is the share of {{ }} segments the fast path evaluates, weighted by
	// the views of the template that holds them.
	Fast Rate
	// Luxon is the share of code segments that use Luxon, plain and weighted;
	// SandboxLuxon the share of Luxon segments among those that need the
	// sandbox.
	Luxon, SandboxLuxon Rate
	// AnalysisGaps counts expressions the static parser rejected and the
	// sandbox accepted.
	AnalysisGaps int
	// Reasons lists every reason with the expressions it affects, most
	// frequent first.
	Reasons []Tally
}

// Summarize aggregates verdicts.
func Summarize(verdicts []Verdict) Summary {
	var s Summary
	s.Expressions = len(verdicts)
	templates := map[int]struct{}{}
	tallies := map[Reason]*Tally{}
	for i := range verdicts {
		v := &verdicts[i]
		templates[v.Entry.Template] = struct{}{}
		w := float64(v.Entry.Views)
		add(&s.All, v.OK(), w)
		if !onlyCategories(v, CategoryDeferred) {
			add(&s.WithoutDeferred, v.OK(), w)
		}
		if !onlyCategories(v, CategoryDeferred, CategoryPolicy) {
			add(&s.WithoutDeferredOrPolicy, v.OK(), w)
		}
		s.Fast.Count += v.FastSegments
		s.Fast.Total += v.CodeSegments
		s.Fast.Weighted += w * float64(v.FastSegments)
		s.Fast.WeightedTotal += w * float64(v.CodeSegments)
		s.Luxon.Count += v.LuxonSegments
		s.Luxon.Total += v.CodeSegments
		s.Luxon.Weighted += w * float64(v.LuxonSegments)
		s.Luxon.WeightedTotal += w * float64(v.CodeSegments)
		s.SandboxLuxon.Count += v.SandboxLuxon
		s.SandboxLuxon.Total += v.SandboxSegments
		s.SandboxLuxon.Weighted += w * float64(v.SandboxLuxon)
		s.SandboxLuxon.WeightedTotal += w * float64(v.SandboxSegments)
		if v.AnalysisGap {
			s.AnalysisGaps++
		}
		for _, r := range v.Reasons {
			t := tallies[r]
			if t == nil {
				t = &Tally{Reason: r}
				tallies[r] = t
			}
			t.Count++
			t.Weight += w
		}
	}
	s.Templates = len(templates)
	for _, t := range tallies {
		s.Reasons = append(s.Reasons, *t)
	}
	slices.SortFunc(s.Reasons, func(a, b Tally) int {
		if c := cmp.Compare(b.Count, a.Count); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Reason.Category, b.Reason.Category); c != 0 {
			return c
		}
		return cmp.Compare(a.Reason.Name, b.Reason.Name)
	})
	return s
}

func add(r *Rate, ok bool, w float64) {
	r.Total++
	r.WeightedTotal += w
	if ok {
		r.Count++
		r.Weighted += w
	}
}

// onlyCategories reports whether the verdict has reasons and all of them are
// in the given categories.
func onlyCategories(v *Verdict, cats ...Category) bool {
	if len(v.Reasons) == 0 {
		return false
	}
	for _, r := range v.Reasons {
		if !slices.Contains(cats, r.Category) {
			return false
		}
	}
	return true
}
