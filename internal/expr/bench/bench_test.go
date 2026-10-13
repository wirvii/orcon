// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bench_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/expr/bench"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

const corpusText = `{"template":1,"views":100,"node":"Set","type":"n8n-nodes-base.set","path":"parameters.a","expression":"={{ $json.a }}"}
{"template":1,"views":100,"node":"Set","type":"n8n-nodes-base.set","path":"parameters.b","expression":"=plain text"}
{"template":2,"views":10,"node":"Agent","type":"tool","path":"parameters.c","expression":"={{ $fromAI('x') }}"}
{"template":2,"views":10,"node":"Code","type":"n8n-nodes-base.set","path":"parameters.d","expression":"={{ $env.HOME }}"}
{"template":3,"views":1,"node":"HTTP","type":"n8n-nodes-base.httpRequest","path":"parameters.e","expression":"={{ $json.name.toTitleCase().nopeMethod() }}"}
{"template":3,"views":1,"node":"HTTP","type":"n8n-nodes-base.httpRequest","path":"parameters.f","expression":"={{ mystery }}"}
{"template":3,"views":1,"node":"HTTP","type":"n8n-nodes-base.httpRequest","path":"parameters.g","expression":"={{ $json. }}"}
{"template":3,"views":1,"node":"HTTP","type":"n8n-nodes-base.httpRequest","path":"parameters.h","expression":"={{ $json.a.trim() }}"}
`

// probeFunc adapts a function to bench.Prober.
type probeFunc func(code string) error

func (f probeFunc) Probe(_ context.Context, code string, _ *expr.Env) error { return f(code) }

func testEnv() *expr.Env {
	var seed [expr.SeedSize]byte
	return &expr.Env{Activation: expr.NewActivation(time.Unix(0, 0), seed)}
}

func TestReadCorpus(t *testing.T) {
	t.Parallel()
	entries, err := bench.ReadCorpus(strings.NewReader(corpusText))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 8 || entries[0].Template != 1 || entries[0].Views != 100 || entries[0].Node != "Set" || entries[0].Expression != "={{ $json.a }}" {
		t.Errorf("entries = %+v", entries)
	}
	if got := entries[0].Where(); !strings.Contains(got, "template 1") || !strings.Contains(got, `"Set"`) || !strings.Contains(got, "parameters.a") {
		t.Errorf("Where() = %q", got)
	}
	for name, bad := range map[string]string{
		"not json":   "{",
		"missing":    `{"template":1}`,
		"wrong type": `{"template":"x","views":1,"node":"n","type":"t","path":"p","expression":"="}`,
		"views":      `{"template":1,"views":"x","node":"n","type":"t","path":"p","expression":"="}`,
		"node":       `{"template":1,"views":1,"node":1,"type":"t","path":"p","expression":"="}`,
		"no type":    `{"template":1,"views":1,"node":"n","path":"p","expression":"="}`,
		"no path":    `{"template":1,"views":1,"node":"n","type":"t","expression":"="}`,
		"no expr":    `{"template":1,"views":1,"node":"n","type":"t","path":"p"}`,
		"no views":   `{"template":1,"node":"n","type":"t","path":"p","expression":"="}`,
	} {
		if _, err := bench.ReadCorpus(strings.NewReader(bad)); err == nil {
			t.Errorf("%s: ReadCorpus accepted %q", name, bad)
		}
	}
}

func TestCompatClassifiesEveryExpression(t *testing.T) {
	t.Parallel()
	entries, err := bench.ReadCorpus(strings.NewReader(corpusText))
	if err != nil {
		t.Fatal(err)
	}
	entries = append(entries, bench.Entry{Template: 9, Views: 5, Node: "Big", Expression: "={{ " + strings.Repeat("1+", expr.MaxSourceSize) + "1 }}"})
	ev := expr.New(expr.Options{})
	probes := 0
	prober := probeFunc(func(code string) error {
		probes++
		switch {
		case strings.Contains(code, "$json.a.trim()"):
			return &expr.EvalError{Name: "TypeError", Message: "not a function"}
		case strings.TrimSpace(code) == "$json.":
			return &expr.EvalError{Name: "SyntaxError", Message: "unexpected token", Compile: true}
		case strings.Contains(code, "mystery"):
			return &expr.EvalError{Name: "ReferenceError", Message: "mystery is not defined"}
		case strings.Contains(code, "weird"):
			return errors.New("sandbox failure")
		}
		return nil
	})
	verdicts, err := bench.Compat(t.Context(), bench.CompatInput{Evaluator: ev, Prober: prober, Env: testEnv()}, entries)
	if err != nil {
		t.Fatal(err)
	}
	// The text-only entry is not an expression with code.
	if len(verdicts) != 8 {
		t.Fatalf("%d verdicts, want 8 (one entry holds no code)", len(verdicts))
	}
	byPath := map[string]bench.Verdict{}
	for _, v := range verdicts {
		byPath[v.Entry.Path+v.Entry.Node] = v
	}
	reasons := func(key string) string {
		var out []string
		for _, r := range byPath[key].Reasons {
			out = append(out, string(r.Category)+":"+r.Name)
		}
		return strings.Join(out, ",")
	}
	for key, want := range map[string]string{
		"parameters.aSet":   "",
		"parameters.cAgent": "DEFERRED:$fromAI",
		"parameters.dCode":  "POLICY:$env",
		"parameters.eHTTP":  "UNKNOWN_METHOD:nopeMethod",
		"parameters.fHTTP":  "UNKNOWN_IDENTIFIER:mystery",
		"parameters.gHTTP":  "SYNTAX:unexpected token",
		"parameters.hHTTP":  "RUNTIME:TypeError: not a function",
		"Big":               "RUNTIME:code segment too large",
	} {
		if got := reasons(key); got != want {
			t.Errorf("%q: reasons = %q, want %q", key, got, want)
		}
	}
	if !byPath["parameters.aSet"].OK() || byPath["parameters.cAgent"].OK() {
		t.Error("OK() does not follow the reasons")
	}
	// An expression with a known name and a method on a lenient value does not need the probe to fail.
	if probes == 0 {
		t.Error("the prober was never used")
	}
}

func TestCompatSeparatesAnalysisGaps(t *testing.T) {
	t.Parallel()
	// The static parser rejects code the sandbox accepts: the analysis, not the
	// expression, is the limit.
	entries := []bench.Entry{{Template: 1, Views: 1, Expression: "={{ $json.a }} {{ $json. }}"}}
	verdicts, err := bench.Compat(t.Context(), bench.CompatInput{
		Evaluator: expr.New(expr.Options{}), Env: testEnv(),
		Prober: probeFunc(func(string) error { return nil }),
	}, entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(verdicts) != 1 || !verdicts[0].OK() || !verdicts[0].AnalysisGap || verdicts[0].CodeSegments != 2 || verdicts[0].FastSegments != 1 {
		t.Errorf("verdicts = %+v", verdicts)
	}
}

func TestCompatCountsLuxon(t *testing.T) {
	t.Parallel()
	entries := []bench.Entry{
		{Template: 1, Views: 10, Expression: "={{ $now.toISO() }} {{ $json.a }}"},
		{Template: 1, Views: 10, Expression: "={{ DateTime.now().plus({days: 1}) }}"},
		{Template: 2, Views: 30, Expression: "={{ $json.a.trim() }}"},
	}
	verdicts, err := bench.Compat(t.Context(), bench.CompatInput{
		Evaluator: expr.New(expr.Options{}), Env: testEnv(),
		Prober: probeFunc(func(string) error { return nil }),
	}, entries)
	if err != nil {
		t.Fatal(err)
	}
	sum := bench.Summarize(verdicts)
	// Four code segments: two use Luxon (both need the sandbox), one is fast, one needs it without Luxon.
	if sum.Luxon.Count != 2 || sum.Luxon.Total != 4 || sum.SandboxLuxon.Count != 2 || sum.SandboxLuxon.Total != 3 {
		t.Errorf("luxon = %+v, sandbox luxon = %+v", sum.Luxon, sum.SandboxLuxon)
	}
	if got := sum.Luxon.WeightedPercent(); got < 30 || got > 60 {
		t.Errorf("weighted share of Luxon = %.1f", got)
	}
}

func TestSummarize(t *testing.T) {
	t.Parallel()
	entries, err := bench.ReadCorpus(strings.NewReader(corpusText))
	if err != nil {
		t.Fatal(err)
	}
	verdicts, err := bench.Compat(t.Context(), bench.CompatInput{
		Evaluator: expr.New(expr.Options{}), Env: testEnv(),
		Prober: probeFunc(func(string) error { return nil }),
	}, entries)
	if err != nil {
		t.Fatal(err)
	}
	sum := bench.Summarize(verdicts)
	if sum.Expressions != 7 || sum.Templates != 3 {
		t.Errorf("expressions %d templates %d", sum.Expressions, sum.Templates)
	}
	// Passing: $json.a (100 views). Failing: fromAI (10), env (10), method (1), unknown (1), syntax or probe-clean (1), $json.a.b() (1).
	if sum.All.Count != 3 || sum.All.Total != 7 {
		t.Errorf("All = %+v", sum.All)
	}
	if sum.WithoutDeferred.Total != 6 || sum.WithoutDeferredOrPolicy.Total != 5 {
		t.Errorf("leaving out deferred/policy: %+v %+v", sum.WithoutDeferred, sum.WithoutDeferredOrPolicy)
	}
	if got := sum.All.WeightedPercent(); got < 80 || got > 100 {
		t.Errorf("weighted share = %.2f", got)
	}
	if sum.Fast.Total == 0 || sum.Fast.WeightedPercent() <= 0 {
		t.Errorf("fast = %+v", sum.Fast)
	}
	if len(sum.Reasons) == 0 || sum.Reasons[0].Count < sum.Reasons[len(sum.Reasons)-1].Count {
		t.Errorf("reasons are not sorted by frequency: %+v", sum.Reasons)
	}
	if (bench.Rate{}).Percent() != 0 || (bench.Rate{}).WeightedPercent() != 0 {
		t.Error("an empty rate is not zero")
	}
	if got := bench.MedianSize(entries); got <= 0 {
		t.Errorf("MedianSize = %d", got)
	}
	if bench.MedianSize(nil) != 0 {
		t.Error("MedianSize of nothing is not zero")
	}
}

func TestLatency(t *testing.T) {
	t.Parallel()
	var sample []time.Duration
	for i := 100; i >= 1; i-- {
		sample = append(sample, time.Duration(i)*time.Microsecond)
	}
	l := bench.NewLatency(sample)
	if l.N != 100 || l.P50 != 50*time.Microsecond || l.P90 != 90*time.Microsecond || l.P99 != 99*time.Microsecond || l.Max != 100*time.Microsecond || l.Mean != 50500*time.Nanosecond {
		t.Errorf("latency = %+v", l)
	}
	if (bench.NewLatency(nil) != bench.Latency{}) {
		t.Error("an empty sample has a latency")
	}
	if bench.Quantile(nil, 0.5) != 0 || bench.Quantile([]time.Duration{7}, 0.99) != 7 || bench.Quantile([]time.Duration{1, 2}, 0) != 1 {
		t.Error("Quantile edge cases")
	}
}

// answeringSandbox is a sandbox whose runtimes answer every call with 1 and
// report a memory size.
type answeringSandbox struct {
	mu     sync.Mutex
	opened int
}

func (a *answeringSandbox) Open(context.Context, expr.SandboxConfig) (expr.Runtime, error) { //nolint:ireturn // implements the expr.Sandbox port
	a.mu.Lock()
	defer a.mu.Unlock()
	a.opened++
	return &answeringRuntime{}, nil
}

type answeringRuntime struct{}

func (*answeringRuntime) Call(context.Context, []byte, expr.Guest) ([]byte, error) {
	time.Sleep(20 * time.Microsecond)
	return []byte(`{"v":1}`), nil
}
func (*answeringRuntime) MemoryUsage(context.Context) int64 { return 4096 }
func (*answeringRuntime) MemorySize() uint32                { return 1 << 20 }
func (*answeringRuntime) Close(context.Context) error       { return nil }

func TestCasesAreValidAndMeasured(t *testing.T) {
	t.Parallel()
	cases := bench.Cases()
	if len(cases) < 20 {
		t.Fatalf("only %d cases", len(cases))
	}
	seen := map[string]bool{}
	luxon := 0
	for _, c := range cases {
		if seen[c.Name] {
			t.Errorf("duplicate case %q", c.Name)
		}
		seen[c.Name] = true
		env, err := c.Env()
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if !expr.IsTemplate(c.Template) || len(env.Input) != 1 || env.Activation == nil {
			t.Errorf("%s: malformed case", c.Name)
		}
		if c.Luxon {
			luxon++
		}
		if a := expr.AnalyzeTemplate(c.Template); !a.Supported() {
			t.Errorf("%s: the analysis reports %v", c.Name, a.Findings)
		}
		// Every node a case reads exists, and pairs with the item.
		for _, ref := range expr.AnalyzeTemplate(c.Template).NodeRefs {
			out, ok := env.Nodes.Node(ref.Name)
			if !ok || !out.Executed {
				t.Errorf("%s: node %q is not defined", c.Name, ref.Name)
			}
			if _, ok := out.Paired(0); !ok {
				t.Errorf("%s: node %q has no paired item 0", c.Name, ref.Name)
			}
			if _, ok := out.Paired(99); ok {
				t.Errorf("%s: node %q pairs item 99", c.Name, ref.Name)
			}
		}
	}
	if luxon == 0 {
		t.Error("no case is marked as running Luxon")
	}
	bad := bench.Case{Name: "x", Item: "{"}
	if _, err := bad.Env(); err == nil {
		t.Error("Env accepted an unreadable item")
	}
	bad = bench.Case{Name: "x", Item: "{}", Nodes: map[string]string{"N": "["}}
	if _, err := bad.Env(); err == nil {
		t.Error("Env accepted unreadable node data")
	}
}

func TestRunSuiteAndReport(t *testing.T) {
	t.Parallel()
	sb := &answeringSandbox{}
	ev := expr.New(expr.Options{Sandbox: sb})
	cfg := expr.SandboxConfig{Scripts: expr.Prelude(), Entry: "__orcon_run"}
	res, err := bench.RunSuite(t.Context(), bench.SuiteInput{
		Evaluator: ev, Sandbox: sb, Config: cfg,
		Iterations: 20, Warmup: 2, Opens: 3, Parallel: 2, Window: 15 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Cases) != len(bench.Cases()) || res.Fast.N == 0 || res.Warm.N == 0 || res.MedianBand.N == 0 {
		t.Fatalf("suite = fast %d warm %d band %d cases %d", res.Fast.N, res.Warm.N, res.MedianBand.N, len(res.Cases))
	}
	if res.BandPlain.N == 0 || res.BandPlain.N+res.BandLuxon.N != res.MedianBand.N {
		t.Errorf("band split: plain %d luxon %d total %d", res.BandPlain.N, res.BandLuxon.N, res.MedianBand.N)
	}
	if res.LinearMemory != 1<<20 || res.JSHeap != 4096 || res.OpenOnly.N != 3 || res.ColdEval.N != 3 {
		t.Errorf("opening measurements = %+v", res)
	}
	if res.Throughput <= 0 || res.ParallelThroughput <= 0 || res.Parallel != 2 || res.Iterations != 20 || res.Warmup != 2 {
		t.Errorf("throughput = %v / %v", res.Throughput, res.ParallelThroughput)
	}
	if res.BandSize == 0 || len(res.BandCases) == 0 {
		t.Errorf("band = %d %v", res.BandSize, res.BandCases)
	}

	// An explicit corpus median moves the band.
	res2, err := bench.RunSuite(t.Context(), bench.SuiteInput{
		Evaluator: ev, Sandbox: sb, Config: cfg, Iterations: 5, Opens: 1, Parallel: 1, Window: 5 * time.Millisecond, CorpusMedianSize: 4,
	})
	if err != nil || res2.BandSize != 4 {
		t.Fatalf("band size = %d, %v", res2.BandSize, err)
	}

	for _, tc := range []struct {
		name      string
		suite     bench.SuiteResult
		compat    bool
		wantDec   bench.Decision
		wantWords []string
	}{
		{"confirmed", passing(&res), true, bench.DecisionConfirmed, []string{"Every target that can be measured", "PENDING"}},
		{"slow sandbox", slowSandbox(&res), true, bench.DecisionReversalCriterionMet, []string{"does not amend the ADR", "FAIL"}},
		{"no corpus", passing(&res), false, bench.DecisionIncomplete, []string{"could not be measured", "no corpus measured"}},
	} {
		m := bench.Measurements{
			Date: "2026-10-13", Commit: "abc", Toolchain: "go1.x", Hardware: "test box", GOOS: "darwin", GOARCH: "arm64", NumCPU: 4,
			N8nVersion: "9.9.9", Templates: 3, CorpusSHA256: "deadbeef",
			QuickJS: "QuickJS", Luxon: "Luxon", Wazero: "wazero",
			Modes: []*bench.ModeResult{{Name: "hard limit", Description: "closes the module", Suite: tc.suite, BandP99Runs: []time.Duration{time.Microsecond, 2 * time.Microsecond}}},
		}
		if tc.compat {
			m.Compat = compatSummary(t)
			m.CorpusMedianSize = 4
		}
		dec, checks := m.Verdict()
		if dec != tc.wantDec || len(checks) != 4 {
			t.Errorf("%s: decision %s with %d checks, want %s", tc.name, dec, len(checks), tc.wantDec)
		}
		md := m.Markdown()
		for _, w := range append(tc.wantWords, "Expression engine measurement report", "hard limit", "Per case", "Reproducing") {
			if !strings.Contains(md, w) {
				t.Errorf("%s: report lacks %q", tc.name, w)
			}
		}
	}

	// A second mode is reported as an alternative.
	m := bench.Measurements{Modes: []*bench.ModeResult{
		{Name: "hard limit", Suite: slowSandbox(&res)},
		{Name: "cooperative limit", Description: "polls", Suite: passing(&res)},
	}, Compat: compatSummary(t), CorpusMedianSize: 4}
	md := m.Markdown()
	if !strings.Contains(md, "Alternative measured, cooperative limit: ") || !strings.Contains(md, "## Mode 2: cooperative limit") {
		t.Error("the alternative mode is not reported")
	}
	var empty bench.Measurements
	if dec, _ := empty.Verdict(); dec != bench.DecisionIncomplete {
		t.Errorf("a report with no measurements is %s", dec)
	}
}

func passing(res *bench.SuiteResult) bench.SuiteResult { return withBandP99(res, 50*time.Microsecond) }

func slowSandbox(res *bench.SuiteResult) bench.SuiteResult {
	return withBandP99(res, 900*time.Microsecond)
}

func withBandP99(res *bench.SuiteResult, p99 time.Duration) bench.SuiteResult {
	c := *res
	c.MedianBand.P99, c.BandPlain.P99, c.BandLuxon.P99 = p99, p99, p99
	c.BandPlain.N, c.BandLuxon.N = max(c.BandPlain.N, 1), max(c.BandLuxon.N, 1)
	return c
}

// TestLuxonBandHasItsOwnTarget checks SPEC-001 amendment 10, B.2: p99 is
// judged at 200 us without Luxon and at 500 us with it, on the median of the
// runs.
func TestLuxonBandHasItsOwnTarget(t *testing.T) {
	t.Parallel()
	mk := func(plain, luxon []time.Duration) bench.Decision {
		suite := bench.SuiteResult{}
		suite.BandPlain.N, suite.BandLuxon.N, suite.MedianBand.N = 1, 1, 2
		m := bench.Measurements{Compat: bench.Summary{Expressions: 1}, Modes: []*bench.ModeResult{{Suite: suite, PlainP99Runs: plain, LuxonP99Runs: luxon}}}
		m.Compat.Fast = bench.Rate{Count: 1, Total: 1, Weighted: 1, WeightedTotal: 1}
		dec, _ := m.Verdict()
		return dec
	}
	us := func(v ...int) []time.Duration {
		out := make([]time.Duration, len(v))
		for i, x := range v {
			out[i] = time.Duration(x) * time.Microsecond
		}
		return out
	}
	for _, tc := range []struct {
		name         string
		plain, luxon []time.Duration
		want         bench.Decision
	}{
		{"both inside their targets", us(150, 160, 170), us(450, 460, 470), bench.DecisionConfirmed},
		{"luxon between 200 and 500 passes", us(100, 100, 100), us(499, 499, 499), bench.DecisionConfirmed},
		{"luxon above 500 fails", us(100, 100, 100), us(501, 502, 503), bench.DecisionReversalCriterionMet},
		{"plain above 200 fails", us(201, 202, 203), us(100, 100, 100), bench.DecisionReversalCriterionMet},
		{"the median decides, not the worst run", us(100, 100, 900), us(100, 100, 900), bench.DecisionConfirmed},
		{"one good run does not hide two bad ones", us(100, 900, 900), us(100, 100, 100), bench.DecisionReversalCriterionMet},
	} {
		if got := mk(tc.plain, tc.luxon); got != tc.want {
			t.Errorf("%s: decision %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestUsesLuxonFollowsTheNamedList(t *testing.T) {
	t.Parallel()
	for code, want := range map[string]bool{
		"$json.a + 1": false, "$now.toISO()": true, "$today": true, "DateTime.now()": true,
		"Duration.fromObject({a: 1})": true, "Interval.after(a, 1)": true,
		"$json.d.toDateTime()": true, "$json.d.beginningOf('day')": true, "$json.d.isWeekend()": true,
		"$json.s.toUpperCase()": false, "$json.tags.join(',')": false,
	} {
		if got := bench.UsesLuxon(code); got != want {
			t.Errorf("UsesLuxon(%s) = %v, want %v", code, got, want)
		}
	}
}

func compatSummary(t *testing.T) bench.Summary {
	t.Helper()
	entries, err := bench.ReadCorpus(strings.NewReader(corpusText))
	if err != nil {
		t.Fatal(err)
	}
	verdicts, err := bench.Compat(t.Context(), bench.CompatInput{
		Evaluator: expr.New(expr.Options{}), Env: testEnv(),
		Prober: probeFunc(func(string) error { return nil }),
	}, entries)
	if err != nil {
		t.Fatal(err)
	}
	return bench.Summarize(verdicts)
}

func TestRunSuiteReportsAFailingSandbox(t *testing.T) {
	t.Parallel()
	failing := &failingSandbox{}
	ev := expr.New(expr.Options{Sandbox: failing})
	_, err := bench.RunSuite(t.Context(), bench.SuiteInput{Evaluator: ev, Sandbox: failing, Iterations: 2, Opens: 1})
	if err == nil {
		t.Error("RunSuite hid a sandbox failure")
	}
	_ = jsonvalue.Null
}

type failingSandbox struct{}

func (*failingSandbox) Open(context.Context, expr.SandboxConfig) (expr.Runtime, error) { //nolint:ireturn // implements the expr.Sandbox port
	return nil, errors.New("no sandbox today")
}
