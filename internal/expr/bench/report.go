// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bench

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

// Targets of the measurement (ADR-0018 section 4).
const (
	// FastPathTarget is the share of usage-weighted code segments, in percent,
	// that the fast path must evaluate.
	FastPathTarget = 70.0
	// WarmP99Target is the p99 latency a warm sandbox must stay under for
	// expressions of median complexity that do not run Luxon.
	WarmP99Target = 200 * time.Microsecond
	// WarmP99TargetLuxon is the same limit for the expressions that run Luxon
	// (ADR-0018 section 4 as amended by SPEC-001 amendment 10, B.2).
	WarmP99TargetLuxon = 500 * time.Microsecond
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
	// The first is the production stop of ADR-0018 section 3 as amended; the
	// verdict of the decision is its verdict, and the others are measured for
	// comparison only.
	Modes []*ModeResult
	// QuickJS, Luxon and Wazero name the pinned versions measured.
	QuickJS, Luxon, Wazero string
	// Previous, when set, is the measurement before the patch of section C.
	Previous *Previous
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
	// PlainP99Runs and LuxonP99Runs are the p99 of the two bands of the median
	// complexity in every run. The figures judged against the targets are the
	// medians of these (see [ModeResult.PlainP99] and [ModeResult.LuxonP99]).
	PlainP99Runs, LuxonP99Runs []time.Duration
	// Abandon holds what only the mode that answers at the deadline measures;
	// nil for the others.
	Abandon *AbandonResult
	// Load is the measurement under load (SPEC-001 amendment 10, section D);
	// nil for the modes without execution slots.
	Load *LoadResult
}

// LoadResult is the behavior of the execution slots (amendment 10, D-3.3).
type LoadResult struct {
	// Throughput has one row per processor count and slot count.
	Throughput []LoadRow
	// Hostile has one row per configuration of the hostile-call run.
	Hostile []HostileRow
}

// LoadRow is the throughput of trivial warm calls, one goroutine per
// processor, with a number of slots.
type LoadRow struct {
	Procs, Slots int
	CallsPerSec  float64
	// WaitP50 and WaitP99 are the percentiles of the wait for a slot over all
	// the calls, the ones that did not wait included.
	WaitP50, WaitP99 time.Duration
}

// HostileRow is a run of 2*slots+2 hostile calls with the garbage collector
// running all the time.
type HostileRow struct {
	Procs, Slots, Calls, Holders, Refused int
	HeldMax, RefusedMax                   time.Duration
	Heartbeat, GCPause                    time.Duration
}

// AbandonResult is the cost and the limits of answering at the deadline by
// abandoning the call (SPEC-001 amendment 10, A.4).
type AbandonResult struct {
	// Functions are the built-ins covered by the interrupt-poll patch, each
	// with how long the caller waited and how long the abandoned call ran on.
	Functions []AbandonRow
	// CallerSlack is the largest wait past the deadline seen by any caller;
	// MaxOvershoot, the longest an abandoned call ran after its caller left.
	CallerSlack, MaxOvershoot time.Duration
	// Deadline is the limit the functions were run under.
	Deadline time.Duration
	// ExtraNs, ExtraAllocs and ExtraBytes are the cost per call of the
	// goroutine and channels, against the same call without them.
	ExtraNs, ExtraAllocs, ExtraBytes float64
	// Natives summarizes the native functions of the inventory, each driven
	// past its deadline as one large call and inside a loop.
	Natives NativeSummary
}

// NativeSummary is the outcome of driving every covered native function of the
// guest past its deadline (SPEC-001 amendment 10, section C).
type NativeSummary struct {
	// Total is the number of native functions of the guest, Functions the
	// covered ones that were driven and Cases the number of runs.
	Total, Functions, Cases int
	// Max and P99 are the longest and the 99th percentile of how long an
	// abandoned call ran on, over the cases.
	Max, P99 time.Duration
	// Over counts the cases above the bound of the tests (100 ms).
	Over int
	// Worst lists the slowest cases to stop, longest first.
	Worst []AbandonRow
	// StretchMax and StretchP99 are the longest and the 99th percentile of the
	// longest stretch without a host call, over the cases; WorstStretch lists
	// the cases with the longest, longest first (Overshoot holds the stretch).
	StretchMax, StretchP99 time.Duration
	WorstStretch           []AbandonRow
}

// Previous is the measurement before the patch of section C, kept to show what
// the patch cost and gained.
type Previous struct {
	// Commit identifies the measured tree.
	Commit string
	// Plain, Luxon and Combined are the p99 of the bands of the production stop.
	Plain, Luxon, Combined time.Duration
}

// AbandonRow is one built-in driven past its deadline.
type AbandonRow struct {
	Name string
	// Wait is the caller's wait beyond the deadline; Overshoot and P99 are the
	// longest and the 99th percentile of how long the abandoned call ran after
	// the caller stopped waiting, over the runs; Before is the same measure
	// before the patch of section C (zero when it was not measured).
	Wait, Overshoot, P99, Before time.Duration
	// Stretch is the longest the guest went without calling the host.
	Stretch time.Duration
}

// medianOf returns the median of runs, or fallback when there are none.
func medianOf(runs []time.Duration, fallback time.Duration) time.Duration {
	if len(runs) == 0 {
		return fallback
	}
	s := slices.Clone(runs)
	slices.Sort(s)
	return s[len(s)/2]
}

// PlainP99 is the p99 of the median-complexity expressions that do not run
// Luxon: the median of the runs of the suite.
func (r *ModeResult) PlainP99() time.Duration { return medianOf(r.PlainP99Runs, r.Suite.BandPlain.P99) }

// LuxonP99 is the p99 of the median-complexity expressions that run Luxon: the
// median of the runs of the suite.
func (r *ModeResult) LuxonP99() time.Duration { return medianOf(r.LuxonP99Runs, r.Suite.BandLuxon.P99) }

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

	band := func(name string, n int, p99, target time.Duration) Check {
		c := Check{Name: name, Target: "<= " + target.String()}
		switch {
		case n == 0:
			c.Measured, c.Status = "no sandbox case in the band", "PENDING"
			decision = DecisionIncomplete
		case p99 <= target:
			c.Measured, c.Status = p99.String(), "PASS"
		default:
			c.Measured, c.Status = p99.String(), "FAIL"
			decision = DecisionReversalCriterionMet
		}
		return c
	}
	plain := band("Warm sandbox p99, median-complexity expressions without Luxon", mode.Suite.BandPlain.N, mode.PlainP99(), WarmP99Target)
	luxon := band("Warm sandbox p99, median-complexity expressions with Luxon", mode.Suite.BandLuxon.N, mode.LuxonP99(), WarmP99TargetLuxon)
	checks = append(checks, fast, plain, luxon, Check{
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
	case d < time.Second:
		return fmt.Sprintf("%.2f ms", float64(d.Nanoseconds())/1e6)
	default:
		return fmt.Sprintf("%.2f s", d.Seconds())
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
		w("**Outcome, with the production stop of ADR-0018 section 3 as amended by SPEC-001 amendment 10 (%s): %s.**\n\n", m.Modes[0].Name, decision)
		if !strings.Contains(m.Hardware, "ORCON-REF-1") {
			w("This run is informational: the verdict is taken from a run on the reference machine ORCON-REF-1 (amendment 10, B.5), and this one ran on %s.\n\n", m.Hardware)
		}
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
		for _, c := range alt[1:3] {
			w("| %s, %s (comparison only) | %s | %s | %s |\n", c.Name, mode.Name, c.Target, c.Measured, c.Status)
		}
	}
	for _, mode := range m.Modes {
		w("| &nbsp;&nbsp;combined band, %s (no target) | | %s | |\n", mode.Name, dur(medianOf(mode.BandP99Runs, mode.Suite.MedianBand.P99)))
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
		if mode.Abandon != nil {
			writeAbandon(&b, heading, mode.Abandon)
		}
		if mode.Load != nil {
			writeLoad(&b, heading, mode.Load)
		}
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
	m.writePrevious(&b)

	w("## Isolation and determinism\n\nThese are tests, not measurements: `TestSandboxIsolation` (memory bombs, infinite loops, regular-expression backtracking, runaway recursion, attempts to reach the host, state left behind; run in every mode), `TestSandboxContainsABadNeighbour` (one failing execution among many; every mode) and `TestDeterminism` (recorded time and seeded randomness). They run in `make check`.\n\n")
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
		w("- %s: median-complexity expressions without Luxon have a p99 of %s (%s the %s target); expressions that run Luxon, %s (%s the %s target).\n",
			mode.Name, dur(mode.PlainP99()), verb(mode.PlainP99() <= WarmP99Target), WarmP99Target,
			dur(mode.LuxonP99()), verb(mode.LuxonP99() <= WarmP99TargetLuxon), WarmP99TargetLuxon)
	}
	w("- \"Runs Luxon\" means the analysis of the expression names `DateTime`, `Duration`, `Interval`, `$now`, `$today`, a date extension method (`beginningOf`, `diffTo`, `endOfMonth`, `extract`, `format`, `isDst`, `isInLast`, `isWeekend`, `minus`, `plus`) or `toDateTime` (SPEC-001 amendment 10, B.2). The classifier of this harness uses exactly that list, by name: a call to `format`, `plus` or `minus` on a value that is not a date counts as running Luxon here, which can only move a case into the band with the looser target.\n")
	if m.Compat.Expressions > 0 {
		l := m.Compat.Luxon
		sl := m.Compat.SandboxLuxon
		w("- Luxon is used by %.1f %% of the code segments of the corpus (%.1f %% weighted by views) and by %.1f %% of the segments that need the sandbox. It is a large library that the interpreter runs operation by operation, so a single date operation costs tens to hundreds of microseconds however fast the call into the sandbox is.\n",
			l.Percent(), l.WeightedPercent(), sl.Percent())
	}
	w("- The cost of the time limit is the difference between the modes: wazero's close-on-context-done adds a check to every function and loop of the module, and the interpreter runs several times slower with it.\n")
	w("- A coarser outer deadline saves nothing. The cost is not paid when the deadline is reached; it is the check wazero compiles into every function and loop whenever close-on-context-done is on, and it is paid on every operation whatever the length of the deadline. wazero cannot stop, from outside, a module that was compiled without those checks. The only way to answer exactly at the deadline without them is to stop waiting for the code rather than to stop the code: run the call on its own goroutine and abandon it at the deadline, which is the production stop measured first in this report.\n")
	w("- The cooperative modes stop the code at the next poll of the interrupt handler. Without the patch, code that never polls (a native string search over megabytes, a deep `JSON.stringify`) outlives its deadline by seconds and is bounded by the memory cap only. The patched module polls inside the built-ins whose work grows with the input, so an abandoned call ends within the overshoot table above.\n")
	w("- The production stop of expression evaluations is the first mode of this report. The hard limit and the cooperative limit without abandon are measured for comparison and for the tests; the adapter selects the production one by default, and an architecture test (`TestProductionStopMode`) keeps the other two out of every other package, of the configuration and of the flags (SPEC-001 amendment 10, B.1). The hard limit stays for the Code node and WASM connectors until their own cuts measure them.\n")
	w("- Compatibility gate: the C5 gate of %.0f %% excludes only identifiers a later specification defines by name (`$fromAI`, SPEC-006), from a closed list in code; `$env` counts as failing. The launch gate of ADR-0019 section 5 uses the full count, the first row of the compatibility table.\n", CompatTarget)
	w("- The targets are 200 µs without Luxon and 500 µs with it, each the p99 of the median of three runs, measured with the production stop (ADR-0018 section 4 as amended). The verdict is the run on ORCON-REF-1; this report does not amend the reversal criterion, which is unchanged.\n\n")
}

func writeAbandon(b *strings.Builder, heading string, a *AbandonResult) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	w("%s Answering at the deadline\n\n", heading)
	w("Each function below is made to run far past a %s deadline on a runtime that already served a request. \"Caller wait\" is how long after the deadline the caller got its `ErrTimeout`; \"call ran on\" is how long the abandoned call kept running before its next poll of the interrupt handler stopped it and its runtime was closed. The largest caller wait was %s and the largest overshoot %s (the tests hold them to 5 ms and 100 ms: `TestSandboxIsolationAbandon`).\n\n", dur(a.Deadline), dur(a.CallerSlack), dur(a.MaxOvershoot))
	w("| Function | Caller wait past the deadline | Call ran on, before the patch of section C | Call ran on, now (max) | Call ran on, now (p99) | Longest without a host call |\n|---|---|---|---|---|---|\n")
	for _, r := range a.Functions {
		before := "not measured"
		if r.Before > 0 {
			before = dur(r.Before)
		}
		w("| %s | %s | %s | %s | %s | %s |\n", r.Name, dur(r.Wait), before, dur(r.Overshoot), dur(r.P99), dur(r.Stretch))
	}
	if n := a.Natives; n.Cases > 0 {
		w("\n%s Every native function of the guest\n\n", heading+"#")
		w("`TestSandboxIsolationBuiltinInventory` classifies the %d native functions reachable from the global object; the %d covered ones are each driven past the deadline as one large call and inside a loop of calls (`TestSandboxIsolationAbandonNatives`, %d runs). The abandoned call ran on for at most %s after its caller left (p99 %s); %d runs were above 100 ms.\n\n", n.Total, n.Functions, n.Cases, dur(n.Max), dur(n.P99), n.Over)
		w("| Slowest to stop | Call ran on |\n|---|---|\n")
		for _, r := range n.Worst {
			w("| %s | %s |\n", r.Name, dur(r.Overshoot))
		}
		w("\nThe guest returns to the host at least every millisecond of native work, so that Go can run a timer or stop the world; the longest it went without doing so over these runs was %s (p99 %s). The tests hold the cases of the first table to 2 ms and the natives to 5 ms.\n\n", dur(n.StretchMax), dur(n.StretchP99))
		w("| Longest without a host call | Stretch |\n|---|---|\n")
		for _, r := range n.WorstStretch {
			w("| %s | %s |\n", r.Name, dur(r.Overshoot))
		}
	}
	w("\nGoroutine cost per call, against the same call without the goroutine: %+.0f ns, %+.1f allocations, %+.0f bytes. A call that does not reach its deadline pays only this; an abandoned call also costs its runtime, which is closed when the call ends and never reused, so the next call opens a new one (see the cold open above).\n\n", a.ExtraNs, a.ExtraAllocs, a.ExtraBytes)
}

// writeLoad renders the behavior of the execution slots.
func writeLoad(b *strings.Builder, heading string, l *LoadResult) {
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	w("%s Under load\n\n", heading)
	w("At most GOMAXPROCS - 1 calls run guest code at once (configurable with `--expr-sandbox-slots`), and an abandoned call keeps its slot until its guest returns, so the process always has a scheduler slot for timers and requests. Waiting for a slot does not use the deadline of the expression; a call that waits longer than the slot-wait limit (1 s by default) fails with `ErrSandboxSaturated`.\n\n")
	w("Throughput of trivial warm calls, one goroutine per processor, 400 ms per row. \"Slot wait\" is the wait of each call for a slot, the calls that found one free included.\n\n")
	w("| GOMAXPROCS | Slots | Calls per second | Slot wait p50 | Slot wait p99 |\n|---|---|---|---|---|\n")
	for _, r := range l.Throughput {
		w("| %d | %d | %.0f | %s | %s |\n", r.Procs, r.Slots, r.CallsPerSec, dur(r.WaitP50), dur(r.WaitP99))
	}
	w("\nHostile calls: 2 x slots + 2 calls that run for seconds unstopped, started together, with a deadline of 100 ms, at most 2 abandoned calls, a slot wait of 50 ms, the garbage collector running in a loop and a heartbeat of 1 ms. \"Held\" is the longest wait of a call that had a slot and \"refused\" of one that did not; the heartbeat is how late the 1 ms timer ran at worst. The tests hold the held wait to the deadline plus 5 ms, the refused wait to the slot wait plus 5 ms, and the heartbeat and the pauses to 5 ms (`TestAbandonUnderLoad`).\n\n")
	w("| GOMAXPROCS | Slots | Calls | Held | Refused | Held wait (max) | Refused wait (max) | Heartbeat late (max) | GC pause (max) |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range l.Hostile {
		w("| %d | %d | %d | %d | %d | %s | %s | %s | %s |\n", r.Procs, r.Slots, r.Calls, r.Holders, r.Refused, dur(r.HeldMax), dur(r.RefusedMax), dur(r.Heartbeat), dur(r.GCPause))
	}
	w("\n")
}

// writePrevious compares the production stop with its measurement before the
// patch of section C, whose charges sit in the allocator and the string
// comparison, the hot paths of the interpreter.
func (m *Measurements) writePrevious(b *strings.Builder) {
	if m.Previous == nil || len(m.Modes) == 0 {
		return
	}
	w := func(format string, args ...any) { fmt.Fprintf(b, format, args...) }
	mode := m.Modes[0]
	w("## Cost of charging native work\n\n")
	w("The patch of section C charges the interrupt counter in the allocator, in string comparison and in the bulk copies, which sit on hot paths. p99 of the median-complexity band of the production stop, median of three runs, before (commit `%s`) and after:\n\n", m.Previous.Commit)
	w("| Band | Before | After | Target |\n|---|---|---|---|\n")
	w("| Without Luxon | %s | %s | <= %s |\n", dur(m.Previous.Plain), dur(mode.PlainP99()), WarmP99Target)
	w("| With Luxon | %s | %s | <= %s |\n", dur(m.Previous.Luxon), dur(mode.LuxonP99()), WarmP99TargetLuxon)
	w("| Combined | %s | %s | no target |\n\n", dur(m.Previous.Combined), dur(medianOf(mode.BandP99Runs, mode.Suite.MedianBand.P99)))
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
	if len(mode.PlainP99Runs) > 1 {
		w(" Without Luxon, per run: %s; with Luxon, per run: %s. The figures judged against the targets are the medians of these.", durList(mode.PlainP99Runs), durList(mode.LuxonP99Runs))
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

func durList(runs []time.Duration) string {
	parts := make([]string, 0, len(runs))
	for _, d := range runs {
		parts = append(parts, dur(d))
	}
	return strings.Join(parts, ", ")
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
