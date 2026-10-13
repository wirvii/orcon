// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bench

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/expr"
)

// Category classifies why an expression does not run unchanged.
type Category string

// Categories of reasons.
const (
	// CategorySyntax means the code does not parse.
	CategorySyntax Category = "SYNTAX"
	// CategoryUnknownIdentifier means a free identifier the runtime does not
	// define.
	CategoryUnknownIdentifier Category = "UNKNOWN_IDENTIFIER"
	// CategoryUnknownMethod means a method name that neither JavaScript, Luxon
	// nor the supported extension methods define.
	CategoryUnknownMethod Category = "UNKNOWN_METHOD"
	// CategoryDeferred means an identifier owned by a later specification, for
	// example $fromAI (agents).
	CategoryDeferred Category = "DEFERRED"
	// CategoryPolicy means an identifier the runtime refuses on purpose, for
	// example $env (disabled by default).
	CategoryPolicy Category = "POLICY"
	// CategoryRuntime means the sandbox raised an error while running the code
	// against a lenient context.
	CategoryRuntime Category = "RUNTIME"
)

// deferred identifiers belong to a later specification that defines them by
// name. They are reported as not running unchanged, with the specification
// that owns them, so the figure can be read with and without them. The list is
// closed: only an identifier a later specification will define belongs here,
// and the adapter test TestDeferredIdentifiersAreStillUndefined fails once the
// runtime defines one, so the exclusion cannot outlive its reason.
var deferred = map[string]string{
	"$fromAI": "SPEC-006 (agent tools)",
}

// DeferredIdentifiers returns the identifiers the compatibility gate of cut C5
// leaves out because a later specification defines them, with the owner of
// each. The launch gate (ADR-0019 section 5) counts them: it uses the full
// reading.
func DeferredIdentifiers() map[string]string { return maps.Clone(deferred) }

// policy identifiers are refused on purpose.
var policy = map[string]string{
	"$env": "disabled by default (ADR-0018 section 1)",
}

// Reason is one cause for which an expression does not run unchanged.
type Reason struct {
	Category Category
	// Name is the identifier, method or error concerned.
	Name string
}

// Verdict is the outcome for one expression.
type Verdict struct {
	Entry   Entry
	Reasons []Reason
	// FastSegments and CodeSegments count the {{ }} segments of the expression
	// and those the fast path evaluates.
	FastSegments, CodeSegments int
	// LuxonSegments counts the code segments that use Luxon (DateTime,
	// Duration, Interval, $now, $today), and SandboxLuxon those of them that
	// need the sandbox.
	LuxonSegments, SandboxSegments, SandboxLuxon int
	// AnalysisGap is set when the static parser rejected code that the sandbox
	// accepts: the analysis, not the expression, is the limit.
	AnalysisGap bool
}

// OK reports whether the expression runs unchanged.
func (v Verdict) OK() bool { return len(v.Reasons) == 0 }

// Prober runs code against a lenient context. *expr.Session implements it.
type Prober interface {
	Probe(ctx context.Context, code string, env *expr.Env) error
}

// CompatInput is what [Compat] needs besides the corpus.
type CompatInput struct {
	Evaluator *expr.Evaluator
	Prober    Prober
	// Env is the context the code is probed with. It needs an activation.
	Env *expr.Env
	// Timeout bounds one probe; zero means one second.
	Timeout time.Duration
}

// Compat checks every expression of the corpus: it parses the code, matches
// its identifiers and method names against what the runtime defines, and runs
// it against a lenient context in the sandbox (ADR-0018 section 6). Entries
// without a {{ }} segment hold plain text and are not returned.
func Compat(ctx context.Context, in CompatInput, entries []Entry) ([]Verdict, error) {
	if in.Timeout == 0 {
		in.Timeout = time.Second
	}
	var out []Verdict
	for _, e := range entries {
		prog, err := in.Evaluator.Compile(e.Expression)
		if errors.Is(err, expr.ErrTooLarge) {
			out = append(out, Verdict{Entry: e, CodeSegments: 1, Reasons: []Reason{{CategoryRuntime, "code segment too large"}}})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("bench: compile %s: %w", e.Where(), err)
		}
		if prog.CodeSegments() == 0 {
			continue
		}
		v := Verdict{Entry: e, CodeSegments: prog.CodeSegments(), FastSegments: prog.FastSegments()}
		for _, seg := range prog.Segments() {
			if !seg.Code {
				continue
			}
			luxon := usesLuxon(seg.Text)
			if luxon {
				v.LuxonSegments++
			}
			if !seg.Fast {
				v.SandboxSegments++
				if luxon {
					v.SandboxLuxon++
				}
			}
			reasons, gap := checkCode(ctx, in, seg.Text)
			v.Reasons = append(v.Reasons, reasons...)
			v.AnalysisGap = v.AnalysisGap || gap
		}
		v.Reasons = dedupe(v.Reasons)
		out = append(out, v)
	}
	return out, nil
}

// usesLuxon reports whether the code uses a Luxon name of the context.
func usesLuxon(code string) bool {
	a := expr.Analyze(code)
	for _, name := range []string{"DateTime", "Duration", "Interval", "$now", "$today"} {
		if a.Uses(name) {
			return true
		}
	}
	return false
}

func checkCode(ctx context.Context, in CompatInput, code string) (reasons []Reason, gap bool) {
	a := expr.Analyze(code)
	syntax := a.Syntax()
	for _, f := range a.Findings {
		switch f.Kind {
		case expr.FindingSyntax:
			continue // decided below against the sandbox's own parser
		case expr.FindingUnknownIdentifier:
			switch {
			case deferred[f.Name] != "":
				reasons = append(reasons, Reason{CategoryDeferred, f.Name})
			case policy[f.Name] != "":
				reasons = append(reasons, Reason{CategoryPolicy, f.Name})
			default:
				reasons = append(reasons, Reason{CategoryUnknownIdentifier, f.Name})
			}
		case expr.FindingUnknownMethod:
			reasons = append(reasons, Reason{CategoryUnknownMethod, f.Name})
		case expr.FindingDynamicReference:
			// A computed node name is still runnable; the importer reports it.
		}
	}
	for _, id := range a.Identifiers {
		if why, ok := policy[id]; ok && why != "" && !slices.ContainsFunc(reasons, func(r Reason) bool { return r.Name == id }) {
			reasons = append(reasons, Reason{CategoryPolicy, id})
		}
	}
	if len(reasons) > 0 && !syntax {
		return reasons, false // already known not to run unchanged; no need to probe
	}
	probeCtx, cancel := context.WithTimeout(ctx, in.Timeout+time.Second)
	defer cancel()
	err := in.Prober.Probe(probeCtx, code, in.Env)
	if err == nil {
		return reasons, syntax
	}
	var ee *expr.EvalError
	switch {
	case errors.As(err, &ee):
		switch ee.Name {
		case "SyntaxError":
			return append(reasons, Reason{CategorySyntax, firstWords(ee.Message)}), false
		case "ReferenceError":
			return append(reasons, Reason{CategoryUnknownIdentifier, firstWords(ee.Message)}), false
		default:
			return append(reasons, Reason{CategoryRuntime, ee.Name + ": " + firstWords(ee.Message)}), false
		}
	default:
		return append(reasons, Reason{CategoryRuntime, firstWords(err.Error())}), false
	}
}

func firstWords(s string) string {
	const limit = 80
	s, _, _ = strings.Cut(s, "\n")
	if len(s) > limit {
		s = s[:limit]
	}
	return s
}

func dedupe(rs []Reason) []Reason {
	slices.SortFunc(rs, func(a, b Reason) int {
		if c := strings.Compare(string(a.Category), string(b.Category)); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return slices.Compact(rs)
}
