// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bench

import (
	"fmt"
	"strings"
	"time"
)

// Targets of the measurement (ADR-0018 section 4).
const (
	// FastPathTarget is the share of usage-weighted code segments, in percent,
	// that the fast path must evaluate.
	FastPathTarget = 70.0
	// WarmP99Target is the p99 latency a warm sandbox must stay under for
	// expressions of median complexity.
	WarmP99Target = 200 * time.Microsecond
	// CompatTarget is the share of corpus expressions, in percent, that must
	// run unchanged (ADR-0018 section 6).
	CompatTarget = 95.0
)

// Measurements is the raw material of the report.
type Measurements struct {
	Date, Commit, Toolchain, Hardware string
	GOOS, GOARCH                      string
	NumCPU                            int

	// N8nVersion, Templates and CorpusSHA256 identify the corpus.
	N8nVersion   string
	Templates    int
	CorpusSHA256 string
	// Compat is the corpus summary; Compat.Expressions is zero when no corpus
	// was measured.
	Compat Summary
	// CorpusMedianSize is the median complexity of the corpus segments.
	CorpusMedianSize int
	// Modes are the suites measured, one per way of enforcing the time limit.
	// The first is the one ADR-0018 section 3 prescribes; the verdict of the
	// decision is its verdict, and the others are alternatives with the
	// numbers that would justify an amendment.
	Modes []*ModeResult
	// QuickJS, Luxon and Wazero name the pinned versions measured.
	QuickJS, Luxon, Wazero string
}

// ModeResult is the measurement of one way of enforcing the time limit.
type ModeResult struct {
	// Name is a short label ("hard limit"), and Description says how the limit
	// is enforced and what it costs.
	Name, Description string
	Suite             SuiteResult
	// BandP99Runs are the p99 of the median-complexity band in every run of
	// the suite; Suite is the run with the median value.
	BandP99Runs []time.Duration
}

// Check is one target and what was measured against it.
type Check struct {
	Name     string
	Target   string
	Measured string
	// Status is "PASS", "FAIL" or "PENDING" (not measurable in this cut).
	Status string
}

// Decision is the outcome of the measurement for ADR-0018 section 3.
type Decision string

// Decisions.
const (
	// DecisionConfirmed means every measured target is met.
	DecisionConfirmed Decision = "CONFIRMED"
	// DecisionReversalCriterionMet means a measured target is missed: the
	// reversal criterion of ADR-0018 section 4 applies, and amending the ADR is
	// the owner's decision.
	DecisionReversalCriterionMet Decision = "REVERSAL CRITERION MET"
	// DecisionIncomplete means a target could not be measured.
	DecisionIncomplete Decision = "INCOMPLETE"
)

// Verdict applies the targets to the measurements of the prescribed mode, the
// first of m.Modes.
func (m *Measurements) Verdict() (Decision, []Check) {
	if len(m.Modes) == 0 {
		return DecisionIncomplete, nil
	}
	return m.VerdictOf(m.Modes[0])
}

// VerdictOf applies the targets to the measurements of one mode.
func (m *Measurements) VerdictOf(mode *ModeResult) (Decision, []Check) {
	var checks []Check
	decision := DecisionConfirmed

	fast := Check{Name: "Fast path coverage (usage-weighted)", Target: fmt.Sprintf(">= %.0f %%", FastPathTarget)}
	switch {
	case m.Compat.Expressions == 0:
		fast.Measured, fast.Status = "no corpus measured", "PENDING"
		decision = DecisionIncomplete
	case m.Compat.Fast.WeightedPercent() >= FastPathTarget:
		fast.Measured, fast.Status = fmt.Sprintf("%.1f %%", m.Compat.Fast.WeightedPercent()), "PASS"
	default:
		fast.Measured, fast.Status = fmt.Sprintf("%.1f %%", m.Compat.Fast.WeightedPercent()), "FAIL"
		decision = DecisionReversalCriterionMet
	}

	warm := Check{Name: "Warm sandbox p99, median-complexity expressions", Target: "<= " + WarmP99Target.String()}
	switch {
	case mode.Suite.MedianBand.N == 0:
		warm.Measured, warm.Status = "no sandbox case in the band", "PENDING"
		decision = DecisionIncomplete
	case mode.Suite.MedianBand.P99 <= WarmP99Target:
		warm.Measured, warm.Status = mode.Suite.MedianBand.P99.String(), "PASS"
	default:
		warm.Measured, warm.Status = mode.Suite.MedianBand.P99.String(), "FAIL"
		decision = DecisionReversalCriterionMet
	}
	checks = append(checks, fast, warm, Check{
		Name: "D20 scenario S1", Target: "passes", Status: "PENDING",
		Measured: "the benchmark harness of cut C16 does not exist yet; the sandbox can only be reversed on this ground when it does",
	})
	return decision, checks
}

func dur(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%d ns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.1f µs", float64(d.Nanoseconds())/1e3)
	default:
		return fmt.Sprintf("%.2f ms", float64(d.Nanoseconds())/1e6)
	}
}

func latencyRow(name string, l Latency) string {
	return fmt.Sprintf("| %s | %d | %s | %s | %s | %s | %s |\n", name, l.N, dur(l.Mean), dur(l.P50), dur(l.P90), dur(l.P99), dur(l.Max))
}

func mib(n int64) string { return fmt.Sprintf("%.2f MiB", float64(n)/(1<<20)) }

// Markdown renders the report.
func (m *Measurements) Markdown() string {
	var b strings.Builder
	decision, checks := m.Verdict()
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("<!-- SPDX-FileCopyrightText: The Orcon Authors -->\n<!-- SPDX-License-Identifier: Apache-2.0 -->\n")
	w("# Expression engine measurement report\n\n")
	w("Generated by `make expr-report` (`go test ./internal/adapter/wazero -run TestExprMeasurementReport`; see \"Reproducing\"). Evidence for SPEC-001 criteria E-EXPR-2 and E-EXPR-3 and for the decision of ADR-0018 sections 3 and 4.\n\n")
	w("**Decision under review:** run non-trivial expressions in QuickJS compiled to WebAssembly on wazero, with a Go fast path for simple expressions (ADR-0018 sections 2 and 3).\n\n")
	if len(m.Modes) > 0 {
		w("**Outcome, with the time limit enforced as ADR-0018 section 3 prescribes (%s): %s.**\n\n", m.Modes[0].Name, decision)
	}
	switch decision {
	case DecisionConfirmed:
		w("Every target that can be measured in this cut is met. Scenario S1 of the D20 gate (ADR-0019 section 4) depends on the benchmark harness of cut C16 and is still pending; if it fails because of the sandbox, ADR-0018 section 4 names the fallback (goja in a separate worker process pool, never in the host process).\n\n")
	case DecisionReversalCriterionMet:
		w("A measured target is missed. This report does not amend the ADR: ADR-0018 section 4 says any change requires amending it, which is the owner's decision.\n\n")
	case DecisionIncomplete:
		w("A target could not be measured; the decision is open.\n\n")
	}
	for _, mode := range m.Modes[min(1, len(m.Modes)):] {
		alt, _ := m.VerdictOf(mode)
		w("**Alternative measured, %s: %s.** %s\n\n", mode.Name, alt, mode.Description)
	}

	w("## Targets\n\n| Target | Required | Measured | Status |\n|---|---|---|---|\n")
	for i := range checks {
		c := &checks[i]
		w("| %s | %s | %s | %s |\n", c.Name, c.Target, c.Measured, c.Status)
	}
	for _, mode := range m.Modes[min(1, len(m.Modes)):] {
		_, alt := m.VerdictOf(mode)
		c := &alt[1]
		w("| %s, %s | %s | %s | %s |\n", c.Name, mode.Name, c.Target, c.Measured, c.Status)
	}
	for _, mode := range m.Modes {
		w("| &nbsp;&nbsp;of which without Luxon, %s | <= %s | %s | %s |\n", mode.Name, WarmP99Target, mode.Suite.BandPlain.P99, passFail(mode.Suite.BandPlain.P99 <= WarmP99Target))
		w("| &nbsp;&nbsp;of which running Luxon, %s | <= %s | %s | %s |\n", mode.Name, WarmP99Target, mode.Suite.BandLuxon.P99, passFail(mode.Suite.BandLuxon.P99 <= WarmP99Target))
	}
	if m.Compat.Expressions > 0 {
		w("| Expressions that run unchanged (E-EXPR-3) | >= %.0f %% | %.2f %% (usage-weighted %.2f %%); %.2f %% (%.2f %%) leaving out identifiers owned by later specifications | see below |\n",
			CompatTarget, m.Compat.All.Percent(), m.Compat.All.WeightedPercent(), m.Compat.WithoutDeferred.Percent(), m.Compat.WithoutDeferred.WeightedPercent())
	}
	w("\n## Environment\n\n")
	w("- Date: %s; commit `%s`; %s\n", m.Date, m.Commit, m.Toolchain)
	w("- Hardware: %s (%s/%s, %d logical CPUs). This is a development machine, not ORCON-REF-1: absolute latencies on the reference hardware (performance.md PERF-4) will differ; the ratios between paths and modes and the pass/fail of the targets are what this report establishes.\n", m.Hardware, m.GOOS, m.GOARCH, m.NumCPU)
	w("- Runtime: %s, %s, %s (wazero compiler mode where the platform supports it).\n\n", m.QuickJS, m.Luxon, m.Wazero)

	for i, mode := range m.Modes {
		heading := "##"
		if len(m.Modes) > 1 {
			w("## Mode %d: %s\n\n%s\n\n", i+1, mode.Name, mode.Description)
			heading = "###"
		}
		m.writeSuite(&b, heading, mode)
	}

	if m.Compat.Expressions > 0 {
		c := m.Compat
		w("## Compatibility with the public template corpus (E-EXPR-3)\n\n")
		w("- Corpus: the %d most viewed templates of the public n8n template library (%d with code), pinned n8n release %s, corpus digest `%s`. The corpus is downloaded at test time and never committed; this report holds only counts, the digest and scores (SPEC-001 decision P8).\n", m.Templates, c.Templates, m.N8nVersion, m.CorpusSHA256)
		w("- Expressions with at least one `{{ }}` segment: %d.\n", c.Expressions)
		w("- \"Runs unchanged\" means the code parses, uses only identifiers and method names the runtime defines, and runs in the sandbox against a lenient context without a reference, syntax or missing-function error. No reference output from a running n8n was captured, so output equality is not measured.\n\n")
		w("| Reading | Expressions | Share | Usage-weighted |\n|---|---|---|---|\n")
		w("| All expressions | %d / %d | %.2f %% | %.2f %% |\n", c.All.Count, c.All.Total, c.All.Percent(), c.All.WeightedPercent())
		w("| Leaving out `$fromAI` (owned by SPEC-006) | %d / %d | %.2f %% | %.2f %% |\n", c.WithoutDeferred.Count, c.WithoutDeferred.Total, c.WithoutDeferred.Percent(), c.WithoutDeferred.WeightedPercent())
		w("| Also leaving out `$env` (refused by policy) | %d / %d | %.2f %% | %.2f %% |\n\n", c.WithoutDeferredOrPolicy.Count, c.WithoutDeferredOrPolicy.Total, c.WithoutDeferredOrPolicy.Percent(), c.WithoutDeferredOrPolicy.WeightedPercent())
		w("Fast path: %d of %d code segments (%.1f %%), %.1f %% weighted by template views.\n\n", c.Fast.Count, c.Fast.Total, c.Fast.Percent(), c.Fast.WeightedPercent())
		w("Reasons an expression does not run unchanged (an expression can have several):\n\n| Category | Name | Expressions |\n|---|---|---|\n")
		for i, r := range c.Reasons {
			if i >= 25 {
				w("| ... | %d more reasons | |\n", len(c.Reasons)-i)
				break
			}
			w("| %s | `%s` | %d |\n", r.Reason.Category, strings.ReplaceAll(r.Reason.Name, "|", "/"), r.Count)
		}
		w("\n")
	}

	m.writeReading(&b)

	w("## Isolation and determinism\n\nThese are tests, not measurements: `TestSandboxIsolation` (memory bombs, infinite loops, regular-expression backtracking, runaway recursion, attempts to reach the host, state left behind; run in both modes), `TestSandboxContainsABadNeighbour` (one failing execution among many; both modes) and `TestDeterminism` (recorded time and seeded randomness). They run in `make check`.\n\n")
	w("## Reproducing\n\n```sh\nmake expr-compat N8N_VERSION=<pinned n8n release>   # corpus download, compatibility gate, this report\nmake expr-report                                   # this report from the last corpus, without the gate\n```\n")
	return b.String()
}

// writeReading explains, from the numbers, which expressions miss the latency
// target and what the owner can do about it.
func (m *Measurements) writeReading(b *strings.Builder) {
	if len(m.Modes) == 0 {
		return
	}
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	w("## Reading the result\n\n")
	for _, mode := range m.Modes {
		s := &mode.Suite
		w("- %s: median-complexity expressions without Luxon have a p99 of %s (%s the %s target); expressions that run Luxon, %s (%s).\n",
			mode.Name, dur(s.BandPlain.P99), verb(s.BandPlain.P99 <= WarmP99Target), WarmP99Target, dur(s.BandLuxon.P99), verb(s.BandLuxon.P99 <= WarmP99Target))
	}
	if m.Compat.Expressions > 0 {
		l := m.Compat.Luxon
		sl := m.Compat.SandboxLuxon
		w("- Luxon is used by %.1f %% of the code segments of the corpus (%.1f %% weighted by views) and by %.1f %% of the segments that need the sandbox. It is a large library that the interpreter runs operation by operation, so a single date operation costs tens to hundreds of microseconds however fast the call into the sandbox is.\n",
			l.Percent(), l.WeightedPercent(), sl.Percent())
	}
	w("- The cost of the time limit is the difference between the modes: wazero's close-on-context-done adds a check to every function and loop of the module, and the interpreter runs several times slower with it. The alternative mode gives the interpreter its full speed and stops runaway code at the next poll of the interrupt handler, but code that never polls is bounded by the memory cap only.\n")
	w("- This report applies the targets as written and does not amend ADR-0018. Whether to keep the prescribed mechanism, adopt the cooperative one, restate the target for date expressions, or give the most common date operations a fast path is the owner's decision.\n\n")
}

func verb(ok bool) string {
	if ok {
		return "meets"
	}
	return "misses"
}

func (m *Measurements) writeSuite(b *strings.Builder, heading string, mode *ModeResult) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	s := &mode.Suite
	w("%s Latency\n\nEach case is evaluated %d times after %d untimed warm-up evaluations, one session, one goroutine. Times include building the request, the call into the sandbox and decoding the answer. A \"warm\" sandbox is a runtime that already served requests.", heading, s.Iterations, s.Warmup)
	if len(mode.BandP99Runs) > 1 {
		var runs []string
		for _, d := range mode.BandP99Runs {
			runs = append(runs, dur(d))
		}
		w(" The suite ran %d times; the tables show the run with the median p99 of the median-complexity band. That p99 in every run: %s.", len(mode.BandP99Runs), strings.Join(runs, ", "))
	}
	w("\n\n| Path | Samples | Mean | p50 | p90 | p99 | Max |\n|---|---|---|---|---|---|---|\n")
	w("%s", latencyRow("Fast path (all fast cases)", s.Fast))
	w("%s", latencyRow("Warm sandbox (all sandbox cases)", s.Warm))
	w("%s", latencyRow(fmt.Sprintf("Warm sandbox, median complexity (nodes %d..%d)", (s.BandSize+1)/2, 2*s.BandSize), s.MedianBand))
	w("%s", latencyRow("&nbsp;&nbsp;of which without Luxon", s.BandPlain))
	w("%s", latencyRow("&nbsp;&nbsp;of which running Luxon (dates)", s.BandLuxon))
	w("%s", latencyRow("Cold: first evaluation of a new session (opens a runtime)", s.ColdEval))
	w("%s", latencyRow("Runtime creation alone", s.OpenOnly))
	w("\nCold start restores a memory snapshot taken after the prelude (Luxon, the runtime scripts, the extension methods) loaded once per configuration, instead of running the prelude again; building that snapshot is a one-time cost per process.\n\n")
	if m.Compat.Expressions > 0 {
		w("Median complexity is the median number of expression nodes of a code segment of the corpus: %d. The band is the sandbox cases within a factor of two of it: %s.\n\n", m.CorpusMedianSize, strings.Join(s.BandCases, ", "))
	} else {
		w("No corpus was measured, so the median complexity is that of the sandbox cases: %d nodes. Cases in the band: %s.\n\n", s.BandSize, strings.Join(s.BandCases, ", "))
	}

	w("%s Per case\n\n| Case | Path | Nodes | Mean | p50 | p99 |\n|---|---|---|---|---|---|\n", heading+"#")
	for _, c := range s.Cases {
		path := "sandbox"
		if c.Fast {
			path = "fast"
		}
		w("| %s | %s | %d | %s | %s | %s |\n", c.Case.Name, path, c.Size, dur(c.Latency.Mean), dur(c.Latency.P50), dur(c.Latency.P99))
	}

	w("\n%s Instance cost and memory\n\n", heading)
	w("- Creating a runtime from the snapshot: mean %s, p99 %s.\n", dur(s.OpenOnly.Mean), dur(s.OpenOnly.P99))
	w("- WebAssembly linear memory of a new runtime: %s (the per-expression cap is 64 MiB; memory is allocated on demand).\n", mib(int64(s.LinearMemory)))
	w("- JavaScript heap in use after loading the prelude: %s.\n", mib(s.JSHeap))
	w("- Growth of the Go heap per open runtime: %s.\n", mib(s.GoHeap))
	w("- Throughput of one session on one core: %.0f evaluations/s (a median-complexity sandbox case); %d sessions in parallel: %.0f evaluations/s in total (%.0f per session).\n\n",
		s.Throughput, s.Parallel, s.ParallelThroughput, safeDiv(s.ParallelThroughput, float64(s.Parallel)))
}

func passFail(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
