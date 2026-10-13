// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bench

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/expr"
)

// Latency summarizes a sample of durations.
type Latency struct {
	N                        int
	Mean, P50, P90, P99, Max time.Duration
}

// NewLatency computes the summary of a sample. The slice is sorted in place.
func NewLatency(sample []time.Duration) Latency {
	if len(sample) == 0 {
		return Latency{}
	}
	slices.Sort(sample)
	var sum time.Duration
	for _, d := range sample {
		sum += d
	}
	return Latency{
		N: len(sample), Mean: sum / time.Duration(len(sample)),
		P50: Quantile(sample, 0.50), P90: Quantile(sample, 0.90), P99: Quantile(sample, 0.99), Max: sample[len(sample)-1],
	}
}

// Quantile returns the q-quantile (0..1) of a sorted sample by the nearest
// rank method.
func Quantile(sorted []time.Duration, q float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(q*float64(len(sorted))+0.999999999) - 1
	return sorted[min(max(rank, 0), len(sorted)-1)]
}

// CaseResult is the measurement of one case.
type CaseResult struct {
	Case Case
	// Fast reports that the fast path evaluates the whole case.
	Fast bool
	// Size is the complexity of the case, the number of expression nodes.
	Size    int
	Latency Latency
	sample  []time.Duration
}

// SuiteInput configures [RunSuite].
type SuiteInput struct {
	// Evaluator has the fast path enabled and the sandbox under test.
	Evaluator *expr.Evaluator
	// Sandbox is the same sandbox, used to time runtime creation directly.
	Sandbox expr.Sandbox
	// Config is the runtime configuration the evaluator uses.
	Config expr.SandboxConfig
	// Iterations per case after Warmup evaluations that are not timed.
	Iterations, Warmup int
	// Opens is how many runtimes are opened to time creation and size them.
	Opens int
	// Parallel is the number of concurrent sessions of the throughput test.
	Parallel int
	// Window is how long each throughput measurement runs; zero means 500 ms.
	Window time.Duration
	// CorpusMedianSize is the median complexity of the corpus expressions; zero
	// means unknown, and the median of the cases is used.
	CorpusMedianSize int
}

// SuiteResult is everything [RunSuite] measures.
type SuiteResult struct {
	// Iterations and Warmup are the timed and untimed evaluations per case.
	Iterations, Warmup int
	Cases              []*CaseResult
	// Fast pools the cases the fast path evaluates; Warm pools the cases that
	// need the sandbox, evaluated by a runtime that already served requests.
	Fast, Warm Latency
	// MedianBand pools the sandbox cases whose complexity is within a factor of
	// two of the median; BandSize is that median and BandCases their names.
	MedianBand Latency
	BandSize   int
	BandCases  []string
	// BandPlain and BandLuxon split MedianBand into the cases that do not run
	// Luxon and those that do.
	BandPlain, BandLuxon Latency
	// ColdEval times a new session's first sandbox evaluation, which opens a
	// runtime from the snapshot; OpenOnly times opening and closing a runtime.
	ColdEval, OpenOnly Latency
	// LinearMemory is the size of the WebAssembly memory of a new runtime, in
	// bytes; JSHeap the bytes the JavaScript heap holds after its first
	// evaluation; GoHeap the growth of the Go heap per open runtime.
	LinearMemory uint32
	JSHeap       int64
	GoHeap       int64
	// Throughput is evaluations per second of a sandbox case on one session
	// (one core busy) and of Parallel sessions on as many goroutines.
	Throughput         float64
	ParallelThroughput float64
	Parallel           int
}

// RunSuite measures the fast path and the sandbox on [Cases].
func RunSuite(ctx context.Context, in SuiteInput) (SuiteResult, error) {
	res := SuiteResult{Iterations: in.Iterations, Warmup: in.Warmup}
	if err := measureCases(ctx, in, &res); err != nil {
		return res, err
	}
	summarizeBand(in, &res)
	if err := measureOpening(ctx, in, &res); err != nil {
		return res, err
	}
	if err := measureThroughput(ctx, in, &res); err != nil {
		return res, err
	}
	return res, nil
}

// measureCases evaluates every case on one session and records its latencies.
func measureCases(ctx context.Context, in SuiteInput, res *SuiteResult) error {
	sess := in.Evaluator.Open()
	defer func() { _ = sess.Close(ctx) }()
	for _, c := range Cases() {
		env, err := c.Env()
		if err != nil {
			return err
		}
		prog, err := in.Evaluator.Compile(c.Template)
		if err != nil {
			return fmt.Errorf("bench: compile case %q: %w", c.Name, err)
		}
		cr := &CaseResult{Case: c, Fast: prog.FastSegments() == prog.CodeSegments(), Size: expr.AnalyzeTemplate(c.Template).Size}
		for range in.Warmup {
			if _, err := sess.Eval(ctx, prog, env); err != nil {
				return fmt.Errorf("bench: case %q: %w", c.Name, err)
			}
		}
		cr.sample = make([]time.Duration, in.Iterations)
		for i := range cr.sample {
			start := time.Now()
			_, err := sess.Eval(ctx, prog, env)
			cr.sample[i] = time.Since(start)
			if err != nil {
				return fmt.Errorf("bench: case %q: %w", c.Name, err)
			}
		}
		cr.Latency = NewLatency(slices.Clone(cr.sample))
		res.Cases = append(res.Cases, cr)
	}
	return nil
}

// summarizeBand pools the samples of the cases into the fast, warm and median
// complexity figures. The median band holds the sandbox cases within a factor
// of two of the median complexity of the corpus (or of the cases when no
// corpus is given).
func summarizeBand(in SuiteInput, res *SuiteResult) {
	pool := func(keep func(*CaseResult) bool) Latency {
		var all []time.Duration
		for _, cr := range res.Cases {
			if keep(cr) {
				all = append(all, cr.sample...)
			}
		}
		return NewLatency(all)
	}
	res.Fast = pool(func(c *CaseResult) bool { return c.Fast })
	res.Warm = pool(func(c *CaseResult) bool { return !c.Fast })

	var sizes []int
	for _, cr := range res.Cases {
		if !cr.Fast {
			sizes = append(sizes, cr.Size)
		}
	}
	slices.Sort(sizes)
	res.BandSize = in.CorpusMedianSize
	if res.BandSize == 0 && len(sizes) > 0 {
		res.BandSize = sizes[len(sizes)/2]
	}
	inBand := func(c *CaseResult) bool {
		return !c.Fast && 2*c.Size >= res.BandSize && c.Size <= 2*res.BandSize
	}
	for _, cr := range res.Cases {
		if inBand(cr) {
			res.BandCases = append(res.BandCases, cr.Case.Name)
		}
	}
	res.MedianBand = pool(inBand)
	res.BandPlain = pool(func(c *CaseResult) bool { return inBand(c) && !c.Case.Luxon })
	res.BandLuxon = pool(func(c *CaseResult) bool { return inBand(c) && c.Case.Luxon })
}

// memorySizer is implemented by runtimes that can report the size of their
// linear memory.
type memorySizer interface{ MemorySize() uint32 }

func measureOpening(ctx context.Context, in SuiteInput, res *SuiteResult) error {
	opens := max(in.Opens, 1)
	// Cold: a new session's first sandbox evaluation, including opening the runtime.
	var sandboxCase Case
	for _, cr := range res.Cases {
		if !cr.Fast {
			sandboxCase = cr.Case
			break
		}
	}
	if sandboxCase.Template == "" {
		return nil
	}
	env, err := sandboxCase.Env()
	if err != nil {
		return err
	}
	prog, err := in.Evaluator.Compile(sandboxCase.Template)
	if err != nil {
		return err
	}
	cold := make([]time.Duration, opens)
	for i := range cold {
		s := in.Evaluator.Open()
		start := time.Now()
		_, err := s.Eval(ctx, prog, env)
		cold[i] = time.Since(start)
		_ = s.Close(ctx)
		if err != nil {
			return fmt.Errorf("bench: cold evaluation: %w", err)
		}
	}
	res.ColdEval = NewLatency(cold)

	open := make([]time.Duration, opens)
	for i := range open {
		start := time.Now()
		rt, err := in.Sandbox.Open(ctx, in.Config)
		open[i] = time.Since(start)
		if err != nil {
			return fmt.Errorf("bench: open a runtime: %w", err)
		}
		_ = rt.Close(ctx)
	}
	res.OpenOnly = NewLatency(open)

	// Memory: hold several runtimes open and measure what they cost.
	var runtimes []expr.Runtime
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	for range opens {
		rt, err := in.Sandbox.Open(ctx, in.Config)
		if err != nil {
			return err
		}
		runtimes = append(runtimes, rt)
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	res.GoHeap = (int64(after.HeapAlloc) - int64(before.HeapAlloc)) / int64(opens) //nolint:gosec // heap sizes fit int64.
	if m, ok := runtimes[0].(memorySizer); ok {
		res.LinearMemory = m.MemorySize()
	}
	res.JSHeap = runtimes[0].MemoryUsage(ctx)
	for _, rt := range runtimes {
		_ = rt.Close(ctx)
	}
	return nil
}

func measureThroughput(ctx context.Context, in SuiteInput, res *SuiteResult) error {
	var target Case
	for _, cr := range res.Cases {
		if slices.Contains(res.BandCases, cr.Case.Name) {
			target = cr.Case
			break
		}
	}
	if target.Template == "" {
		return nil
	}
	prog, err := in.Evaluator.Compile(target.Template)
	if err != nil {
		return err
	}
	run := func(d time.Duration) (int, error) {
		env, err := target.Env()
		if err != nil {
			return 0, err
		}
		s := in.Evaluator.Open()
		defer func() { _ = s.Close(ctx) }()
		deadline := time.Now().Add(d)
		n := 0
		for time.Now().Before(deadline) {
			if _, err := s.Eval(ctx, prog, env); err != nil {
				return n, err
			}
			n++
		}
		return n, nil
	}
	window := in.Window
	if window <= 0 {
		window = 500 * time.Millisecond
	}
	n, err := run(window)
	if err != nil {
		return err
	}
	res.Throughput = float64(n) / window.Seconds()

	res.Parallel = max(in.Parallel, 1)
	counts := make([]int, res.Parallel)
	errs := make([]error, res.Parallel)
	var wg sync.WaitGroup
	for i := range res.Parallel {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counts[i], errs[i] = run(window)
		}()
	}
	wg.Wait()
	total := 0
	for i, c := range counts {
		if errs[i] != nil {
			return errs[i]
		}
		total += c
	}
	res.ParallelThroughput = float64(total) / window.Seconds()
	return nil
}

// MedianSize returns the median complexity (expression nodes) of the code
// segments of the corpus, or 0 for an empty corpus.
func MedianSize(entries []Entry) int {
	var sizes []int
	for _, e := range entries {
		if !expr.IsTemplate(e.Expression) {
			continue
		}
		for _, seg := range expr.Segments(e.Expression[1:]) {
			if seg.Code {
				sizes = append(sizes, expr.Analyze(seg.Text).Size)
			}
		}
	}
	if len(sizes) == 0 {
		return 0
	}
	slices.Sort(sizes)
	return sizes[len(sizes)/2]
}
