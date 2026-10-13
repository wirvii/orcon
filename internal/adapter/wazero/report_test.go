// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/expr/bench"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// TestExprMeasurementReport runs the measurements of ADR-0018 section 4 and
// writes the report. It runs only when ORCON_EXPR_REPORT names the file to
// write (`make expr-compat` and `make expr-report` set it); with
// ORCON_EXPR_CORPUS it also measures coverage and compatibility on the
// downloaded corpus.
func TestExprMeasurementReport(t *testing.T) {
	out := os.Getenv("ORCON_EXPR_REPORT")
	if out == "" {
		t.Skip("ORCON_EXPR_REPORT is not set; run `make expr-report`")
	}
	m := bench.Measurements{
		Date:      time.Now().UTC().Format("2006-01-02"),
		Commit:    os.Getenv("ORCON_EXPR_COMMIT"),
		Toolchain: runtime.Version(),
		Hardware:  os.Getenv("ORCON_EXPR_HARDWARE"),
		GOOS:      runtime.GOOS, GOARCH: runtime.GOARCH, NumCPU: runtime.NumCPU(),
		QuickJS: "QuickJS-ng 0.17.0", Luxon: "Luxon 3.7.2", Wazero: "wazero v1.11.0",
		N8nVersion: os.Getenv("N8N_VERSION"),
	}
	m.Previous = &bench.Previous{Commit: "b5b8642", Plain: 56900 * time.Nanosecond, Luxon: 236600 * time.Nanosecond, Combined: 217900 * time.Nanosecond}
	if m.Hardware == "" {
		m.Hardware = "unspecified"
	}
	if m.Commit == "" {
		m.Commit = "unspecified"
	}

	if dir := os.Getenv("ORCON_EXPR_CORPUS"); dir != "" {
		f, err := os.Open(filepath.Join(dir, "expressions.jsonl")) //nolint:gosec // the directory is named by the make target
		if err != nil {
			t.Fatal(err)
		}
		entries, err := bench.ReadCorpus(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		cev := evaluator(t, expr.Options{Timeout: time.Second})
		s := cev.Open()
		verdicts, err := bench.Compat(t.Context(), bench.CompatInput{Evaluator: cev, Prober: s, Env: newEnv(t, `{}`)}, entries)
		_ = s.Close(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		m.Compat = bench.Summarize(verdicts)
		m.CorpusMedianSize = bench.MedianSize(entries)
		if mf, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil { //nolint:gosec // the directory is named by the make target
			if v, err := jsonvalue.Parse(mf); err == nil {
				if s, ok := v.Get("corpusSha256"); ok {
					m.CorpusSHA256, _ = s.Str()
				}
				if n, ok := v.Get("templates"); ok {
					if i, ok := n.Int64(); ok {
						m.Templates = int(i)
					}
				}
			}
		}
	}

	cfg := expr.New(expr.Options{}).SandboxConfig()
	modes := []struct {
		mode        wazero.Mode
		description string
	}{
		{wazero.ModeCooperativeAbandon, "The production stop of expression evaluations (ADR-0018 section 3 as amended by SPEC-001 amendment 10): the QuickJS interrupt handler checks the deadline every ten thousand operations and, in the patched module (QuickJS-ng 0.17.0 plus the interrupt-poll patch), inside the built-ins whose work grows with the input; every call runs on its own goroutine and the caller stops waiting when the context is done, so it gets its timeout within the deadline plus 5 ms whatever the guest is doing. The abandoned call stops at its next check, refuses further requests to the host, closes its runtime, and the runtime is never reused. At most eight abandoned calls may still be running in a process."},
		{wazero.ModeHardLimit, "Measured for comparison only; not a production mode for expressions. The deadline closes the WebAssembly module when the context is done (wazero close-on-context-done), as ADR-0018 section 3 prescribed before the amendment. wazero compiles a check into every function and loop of the module for this; the QuickJS interpreter loop pays it on every bytecode operation. It stays for the Code node and WASM connectors until their cuts measure their own stop."},
		{wazero.ModeCooperativeLimit, "Measured for comparison only. The cooperative stop of the production mode without the answer at the deadline: the caller waits for the code to reach a check. Code in a built-in the patch does not cover is bounded by the memory cap only (upstream, a crafted `JSON.stringify` of a 100000-deep array outlived a 150 ms limit by about 2.7 s)."},
	}
	for _, md := range modes {
		sb := wazero.New(wazero.WithMode(md.mode))
		ev := expr.New(expr.Options{Sandbox: sb})
		var runs []bench.SuiteResult
		for range 3 {
			res, err := bench.RunSuite(t.Context(), bench.SuiteInput{
				Evaluator: ev, Sandbox: sb, Config: cfg,
				Iterations: 2000, Warmup: 300, Opens: 50, Parallel: min(runtime.NumCPU(), 8),
				CorpusMedianSize: m.CorpusMedianSize,
			})
			if err != nil {
				t.Fatal(err)
			}
			runs = append(runs, res)
		}
		_ = sb.Close(t.Context())
		sort.SliceStable(runs, func(i, j int) bool { return runs[i].MedianBand.P99 < runs[j].MedianBand.P99 })
		mr := &bench.ModeResult{Name: md.mode.String(), Description: md.description, Suite: runs[len(runs)/2]}
		for _, r := range runs {
			mr.BandP99Runs = append(mr.BandP99Runs, r.MedianBand.P99)
			mr.PlainP99Runs = append(mr.PlainP99Runs, r.BandPlain.P99)
			mr.LuxonP99Runs = append(mr.LuxonP99Runs, r.BandLuxon.P99)
		}
		if md.mode == wazero.ModeCooperativeAbandon {
			mr.Abandon = measureAbandon(t, cfg)
		}
		m.Modes = append(m.Modes, mr)
	}
	decision, checks := m.Verdict()
	t.Logf("decision with the production stop (%s): %s", m.Modes[0].Name, decision)
	for _, mr := range m.Modes {
		d, cs := m.VerdictOf(mr)
		t.Logf("%s: %s", mr.Name, d)
		for _, c := range cs {
			t.Logf("  %-52s target %-12s measured %-40s %s", c.Name, c.Target, c.Measured, c.Status)
		}
	}
	_ = checks
	if err := os.WriteFile(out, []byte(m.Markdown()), 0o600); err != nil { //nolint:gosec // the file is named by the make target
		t.Fatal(err)
	}
}

// overrunBefore is how long an abandoned call ran on after its caller left,
// measured at commit b5b8642 (the patch before section C of amendment 10), for
// the cases that existed then. The BL-030 cases are the ones measured there.
var overrunBefore = map[string]time.Duration{
	"indexOf": 4090 * time.Microsecond, "lastIndexOf": 425 * time.Microsecond, "includes": 5980 * time.Microsecond,
	"replaceAll": 353 * time.Microsecond, "replace with a pattern": 9970 * time.Microsecond, "split": 9010 * time.Microsecond,
	"repeat": 365 * time.Microsecond, "padStart": 397 * time.Microsecond, "JSON.parse": 2460 * time.Microsecond,
	"JSON.stringify": 8500 * time.Microsecond, "JSON.stringify nested": 7360 * time.Microsecond, "join": 41660 * time.Microsecond,
	"concat": 17090 * time.Microsecond, "flat": 22260 * time.Microsecond, "toUpperCase": 465 * time.Microsecond,
	"encodeURIComponent": 581 * time.Microsecond, "loop": 82 * time.Microsecond,
	"unshift in a loop": 21654 * time.Millisecond, "string comparison in a loop": 8563 * time.Millisecond,
	"copyWithin in a loop": 11515 * time.Millisecond, "array spread in a loop": 2473 * time.Millisecond,
	"Object.keys in a loop": 4522 * time.Millisecond, "trim": 137 * time.Millisecond, "escape": 247 * time.Millisecond,
	"Array.from": 220 * time.Millisecond, "includes inside filter": 1187 * time.Millisecond,
}

// percentile returns the p-th percentile (0 to 1) of ds, which it sorts.
func percentile(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	slices.Sort(ds)
	return ds[min(int(float64(len(ds))*p), len(ds)-1)]
}

// measureAbandon drives every patched built-in past its deadline in the
// abandon mode, summarizes every covered native of the inventory, and prices
// the goroutine against the cooperative mode.
func measureAbandon(t *testing.T, cfg expr.SandboxConfig) *bench.AbandonResult {
	t.Helper()
	res := &bench.AbandonResult{Deadline: abandonDeadline}
	sb := wazero.New(wazero.WithMode(wazero.ModeCooperativeAbandon))
	defer func() { _ = sb.Close(context.WithoutCancel(t.Context())) }()
	for _, tc := range abandonCases {
		// Five runs: the figures are limits, so the longest is the one reported.
		row := bench.AbandonRow{Name: tc.name, Before: overrunBefore[tc.name]}
		var runs []time.Duration
		for range 5 {
			out, deadline := driveCase(t, sb, cfg, tc.src, false)
			if !errors.Is(out.err, expr.ErrTimeout) {
				t.Fatalf("%s: got %v, want ErrTimeout", tc.name, out.err)
			}
			row.Wait = max(row.Wait, max(out.took-deadline, 0))
			row.Overshoot = max(row.Overshoot, out.overshoot)
			runs = append(runs, out.overshoot)
		}
		row.P99 = percentile(runs, 0.99)
		res.Functions = append(res.Functions, row)
		res.CallerSlack = max(res.CallerSlack, row.Wait)
		res.MaxOvershoot = max(res.MaxOvershoot, row.Overshoot)
	}
	var all []time.Duration
	natives := map[string]bool{}
	for _, tc := range tableCases() {
		out, deadline := driveCase(t, sb, cfg, tc.src, true)
		if !errors.Is(out.err, expr.ErrTimeout) {
			// A single call that finishes inside the deadline has nothing to stop.
			continue
		}
		natives[tc.name] = true
		res.CallerSlack = max(res.CallerSlack, max(out.took-deadline, 0))
		res.MaxOvershoot = max(res.MaxOvershoot, out.overshoot)
		res.Natives.Cases++
		all = append(all, out.overshoot)
		if out.overshoot > overshootBound {
			res.Natives.Over++
		}
		res.Natives.Worst = append(res.Natives.Worst, bench.AbandonRow{Name: tc.name + " (" + tc.kind + ")", Overshoot: out.overshoot})
	}
	res.Natives.Total = len(guestNatives(t))
	res.Natives.Functions = len(natives)
	res.Natives.Max = percentile(slices.Clone(all), 1)
	res.Natives.P99 = percentile(all, 0.99)
	slices.SortFunc(res.Natives.Worst, func(a, b bench.AbandonRow) int { return int(b.Overshoot - a.Overshoot) })
	res.Natives.Worst = res.Natives.Worst[:min(10, len(res.Natives.Worst))]
	base := callCost(t, wazero.ModeCooperativeLimit, cfg)
	with := callCost(t, wazero.ModeCooperativeAbandon, cfg)
	res.ExtraNs, res.ExtraAllocs, res.ExtraBytes = with.ns-base.ns, with.allocs-base.allocs, with.bytes-base.bytes
	return res
}

type perCall struct{ ns, allocs, bytes float64 }

// callCost is the mean cost of a trivial call on a warm runtime.
func callCost(t *testing.T, mode wazero.Mode, cfg expr.SandboxConfig) perCall {
	t.Helper()
	sb := wazero.New(wazero.WithMode(mode))
	defer func() { _ = sb.Close(context.WithoutCancel(t.Context())) }()
	rt, err := sb.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Close(context.WithoutCancel(t.Context())) }()
	req := []byte(`{"e":"1 + 1","m":"v","i":0,"r":0,"n":1791913530250,"z":"UTC"}` + "\x01\x01\x01\x01")
	g := &countingGuest{ops: map[int32]int{}}
	const warm, n = 500, 5000
	for range warm {
		if _, err := rt.Call(t.Context(), req, g); err != nil {
			t.Fatal(err)
		}
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	for range n {
		if _, err := rt.Call(t.Context(), req, g); err != nil {
			t.Fatal(err)
		}
	}
	took := time.Since(start)
	runtime.ReadMemStats(&after)
	return perCall{
		ns:     float64(took.Nanoseconds()) / n,
		allocs: float64(after.Mallocs-before.Mallocs) / n,
		bytes:  float64(after.TotalAlloc-before.TotalAlloc) / n,
	}
}
