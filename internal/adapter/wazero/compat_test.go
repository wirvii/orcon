// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"fmt"
	"os"
	"path/filepath"
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

	// The gate (E-EXPR-3): at least 95 % run unchanged, both plain and weighted
	// by usage. $fromAI belongs to the agent specification (SPEC-006) and is
	// reported with its own reason; the strict reading, which counts it as not
	// running, is logged above and is a question for the owner.
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
