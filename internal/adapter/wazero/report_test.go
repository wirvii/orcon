// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"os"
	"path/filepath"
	"runtime"
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

	cfg := expr.SandboxConfig{
		Scripts: expr.Prelude(), Entry: "__orcon_run",
		MemoryLimit: expr.DefaultMemoryLimit, StackLimit: expr.DefaultStackLimit,
	}
	modes := []struct {
		mode        wazero.Mode
		description string
	}{
		{wazero.ModeHardLimit, "The deadline closes the WebAssembly module when the context is done (wazero close-on-context-done), as ADR-0018 section 3 prescribes. wazero compiles a check into every function and loop of the module for this; the QuickJS interpreter loop pays it on every bytecode operation."},
		{wazero.ModeCooperativeLimit, "The deadline is enforced by the QuickJS interrupt handler, polled by the interpreter every ten thousand operations and inside long built-ins (regular expressions, sorting, array methods); the wazero checks are not compiled in. Code that never polls, for example a native string search over megabytes, is bounded by the memory cap only: the test `TestSandboxIsolation` records a crafted `JSON.stringify` of a 100000-deep array that outlives its 150 ms limit by about 2.7 s. This is a deviation from ADR-0018 section 3 and would need an amendment."},
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
		}
		m.Modes = append(m.Modes, mr)
	}
	decision, checks := m.Verdict()
	t.Logf("decision with the ADR-prescribed limit (%s): %s", m.Modes[0].Name, decision)
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
