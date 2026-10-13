// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/expr/bench"
)

// TestExprCompat is the compatibility measurement of ADR-0018 section 6
// (criterion E-EXPR-3). It runs only under `make expr-compat`, which downloads
// the corpus and sets ORCON_EXPR_CORPUS to its directory.
func TestExprCompat(t *testing.T) {
	dir := os.Getenv("ORCON_EXPR_CORPUS")
	if dir == "" {
		t.Skip("ORCON_EXPR_CORPUS is not set; run `make expr-compat`")
	}
	f, err := os.Open(filepath.Join(dir, "expressions.jsonl")) //nolint:gosec // the directory is named by the make target
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	entries, err := bench.ReadCorpus(f)
	if err != nil {
		t.Fatal(err)
	}
	ev := evaluator(t, expr.Options{Timeout: time.Second})
	s := ev.Open()
	t.Cleanup(func() { _ = s.Close(t.Context()) })
	verdicts, err := bench.Compat(t.Context(), bench.CompatInput{Evaluator: ev, Prober: s, Env: newEnv(t, `{}`)}, entries)
	if err != nil {
		t.Fatal(err)
	}
	sum := bench.Summarize(verdicts)
	t.Logf("expressions with code: %d from %d templates", sum.Expressions, sum.Templates)
	t.Logf("run unchanged (all):                  %d/%d = %.2f%% (usage-weighted %.2f%%)", sum.All.Count, sum.All.Total, sum.All.Percent(), sum.All.WeightedPercent())
	t.Logf("run unchanged (without deferred):     %d/%d = %.2f%% (usage-weighted %.2f%%)", sum.WithoutDeferred.Count, sum.WithoutDeferred.Total, sum.WithoutDeferred.Percent(), sum.WithoutDeferred.WeightedPercent())
	t.Logf("run unchanged (without deferred/policy): %d/%d = %.2f%% (usage-weighted %.2f%%)", sum.WithoutDeferredOrPolicy.Count, sum.WithoutDeferredOrPolicy.Total, sum.WithoutDeferredOrPolicy.Percent(), sum.WithoutDeferredOrPolicy.WeightedPercent())
	t.Logf("fast path: %d/%d segments = %.2f%% (usage-weighted %.2f%%)", sum.Fast.Count, sum.Fast.Total, sum.Fast.Percent(), sum.Fast.WeightedPercent())
	t.Logf("analysis gaps: %d", sum.AnalysisGaps)
	for i, r := range sum.Reasons {
		if i >= 40 {
			break
		}
		t.Logf("  %-18s %-60s %5d expressions", r.Reason.Category, r.Reason.Name, r.Count)
	}
	var b strings.Builder
	for _, v := range verdicts {
		if v.OK() {
			continue
		}
		var rs []string
		for _, r := range v.Reasons {
			rs = append(rs, string(r.Category)+":"+r.Name)
		}
		fmt.Fprintf(&b, "%s | %s | %s\n", v.Entry.Where(), strings.Join(rs, ", "), strings.ReplaceAll(v.Entry.Expression, "\n", "\\n"))
	}
	if err := os.WriteFile(filepath.Join(dir, "unsupported.txt"), []byte(b.String()), 0o600); err != nil { //nolint:gosec // the directory is named by the make target
		t.Fatal(err)
	}
	t.Logf("every expression that does not run unchanged, with its node and field, is listed in %s", filepath.Join(dir, "unsupported.txt"))

	// The gate (E-EXPR-3) of cut C5: at least 95 % run unchanged, both plain and
	// weighted by usage, leaving out only the identifiers a later specification
	// defines by name (bench.DeferredIdentifiers, today $fromAI of SPEC-006).
	// $env counts as a failure: it is refused on purpose. The launch gate
	// (ADR-0019 section 5) uses the full reading logged above.
	gate := 95.0
	if v := os.Getenv("ORCON_EXPR_COMPAT_MIN"); v != "" {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			t.Fatalf("ORCON_EXPR_COMPAT_MIN: %v", err)
		}
		gate = parsed
	}
	if got := sum.WithoutDeferred.Percent(); got < gate {
		t.Errorf("only %.2f %% of the expressions run unchanged (identifiers owned by later specifications left out), need %.0f %%", got, gate)
	}
	if got := sum.WithoutDeferred.WeightedPercent(); got < gate {
		t.Errorf("only %.2f %% of the expressions run unchanged by usage, need %.0f %%", got, gate)
	}
}

// propertyNames lists the property names an expression mentions.
var propertyNames = regexp.MustCompile(`\.([A-Za-z_$][A-Za-z0-9_$]*)|\[\s*['"]([^'"]+)['"]\s*\]`)

// corpusData builds a data set where every property the code mentions exists
// at every depth, filled with values of one kind per variant.
func corpusData(code string, variant int) string {
	names := map[string]bool{}
	for _, m := range propertyNames.FindAllStringSubmatch(code, -1) {
		name := m[1] + m[2]
		if name != "" && name != "json" && name != "item" && name != "first" && name != "last" && name != "all" && name != "isExecuted" {
			names[name] = true
		}
	}
	leaf := []string{`"abc"`, `42`, `null`, `[1,2,"x"]`, `true`, `""`}[variant%6]
	var build func(depth int) string
	build = func(depth int) string {
		if depth == 0 {
			return leaf
		}
		var parts []string
		for name := range names {
			parts = append(parts, strconv.Quote(name)+":"+build(depth-1))
		}
		sort.Strings(parts)
		return "{" + strings.Join(parts, ",") + "}"
	}
	if variant >= 6 {
		return `{}`
	}
	return build(3)
}

// TestExprCorpusParity is the differential test of ADR-0018 section 2 on the
// corpus: every expression of the downloaded corpus that the fast path takes
// is evaluated by both paths against several synthetic data sets and must
// give the same JSON. It runs under `make expr-compat`.
func TestExprCorpusParity(t *testing.T) {
	dir := os.Getenv("ORCON_EXPR_CORPUS")
	if dir == "" {
		t.Skip("ORCON_EXPR_CORPUS is not set; run `make expr-compat`")
	}
	f, err := os.Open(filepath.Join(dir, "expressions.jsonl")) //nolint:gosec // the directory is named by the make target
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	entries, err := bench.ReadCorpus(f)
	if err != nil {
		t.Fatal(err)
	}
	fast := evaluator(t, expr.Options{})
	slow := evaluator(t, expr.Options{DisableFastPath: true})
	fs, ss := fast.Open(), slow.Open()
	defer func() { _ = fs.Close(t.Context()); _ = ss.Close(t.Context()) }()

	compared, segments, mismatches := 0, 0, 0
	for _, e := range entries {
		if !expr.IsTemplate(e.Expression) {
			continue
		}
		for _, seg := range expr.Segments(e.Expression[1:]) {
			if !seg.Code {
				continue
			}
			src := "={{" + seg.Text + "}}"
			fp, err := fast.Compile(src)
			if err != nil || fp.FastSegments() != 1 {
				continue
			}
			sp, err := slow.Compile(src)
			if err != nil {
				continue
			}
			segments++
			for variant := range 7 {
				data := corpusData(seg.Text, variant)
				env := newEnv(t, data)
				env.Nodes = corpusNodes(t, seg.Text, data)
				fr, ferr := fs.Eval(t.Context(), fp, env)
				sr, serr := ss.Eval(t.Context(), sp, env)
				compared++
				if got, want := outcome(fr, ferr), outcome(sr, serr); got != want {
					mismatches++
					if mismatches <= 20 {
						t.Errorf("parity: %s on %s: fast path %s, interpreter %s", seg.Text, data, got, want)
					}
				}
			}
		}
	}
	t.Logf("fast-path parity on the corpus: %d expressions, %d evaluations compared, %d differences", segments, compared, mismatches)
	if segments == 0 {
		t.Error("no fast-path expression found in the corpus")
	}
}

// corpusNodes gives every node the code names the same items as the current one.
func corpusNodes(t *testing.T, code, data string) corpusNodeSet {
	t.Helper()
	items := []expr.Item{{JSON: parse(t, data)}}
	nodes := corpusNodeSet{}
	for _, ref := range expr.Analyze(code).NodeRefs {
		nodes[ref.Name] = expr.NodeOutput{
			Executed: true, Items: items,
			Paired: func(int) (expr.Item, bool) { return items[0], true },
		}
	}
	return nodes
}

type corpusNodeSet map[string]expr.NodeOutput

func (c corpusNodeSet) Node(name string) (expr.NodeOutput, bool) {
	o, ok := c[name]
	return o, ok
}

// TestDeferredIdentifiersAreStillUndefined keeps the exclusion of the C5
// compatibility gate honest: an identifier is left out only while the runtime
// does not define it. When the specification that owns it lands, this test
// fails and the entry must go.
func TestDeferredIdentifiersAreStillUndefined(t *testing.T) {
	ev := evaluator(t, expr.Options{DisableFastPath: true})
	s := ev.Open()
	t.Cleanup(func() { _ = s.Close(context.WithoutCancel(t.Context())) })
	for name, owner := range bench.DeferredIdentifiers() {
		if owner == "" {
			t.Errorf("%s has no owning specification", name)
		}
		res, err := evalOne(t, ev, s, "={{ typeof "+name+" }}")
		if err != nil || outcome(res, err) != `"undefined"` {
			t.Errorf("%s is now defined by the runtime (%s %v): remove it from the deferred list of internal/expr/bench", name, outcome(res, err), err)
		}
		if slices.Contains(expr.ContextNames(), name) || slices.Contains(expr.GlobalNames(), name) {
			t.Errorf("%s is listed as a known name: remove it from the deferred list", name)
		}
	}
}
