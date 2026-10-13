// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func parse(tb testing.TB, s string) jsonvalue.Value {
	tb.Helper()
	v, err := jsonvalue.Parse([]byte(s))
	if err != nil {
		tb.Fatalf("Parse(%s): %v", s, err)
	}
	return v
}

func newEnv(tb testing.TB, json string) *expr.Env {
	tb.Helper()
	var seed [expr.SeedSize]byte
	seed[0] = 7
	return &expr.Env{
		Input:      []expr.Item{{JSON: parse(tb, json)}},
		Activation: expr.NewActivation(time.Date(2026, 10, 13, 12, 30, 0, 0, time.UTC), seed),
	}
}

func asEvalError(err error, target **expr.EvalError) bool { return errors.As(err, target) }

// evaluator builds an evaluator with the production stop; tests that compare
// modes use evaluatorIn.
func evaluator(tb testing.TB, opts expr.Options) *expr.Evaluator {
	tb.Helper()
	return evaluatorIn(tb, wazero.ModeCooperativeAbandon, opts)
}

var (
	sharedMu        sync.Mutex
	sharedSandboxes = map[wazero.Mode]*wazero.Sandbox{}
)

// sharedSandbox returns the sandbox of the test binary for a mode. Compiling
// the module is the slowest step of a test (seconds under the race detector),
// so the tests share the compiled code; a sandbox keeps no state of an
// execution, runtimes do. It is never closed: the process ends with the tests.
func sharedSandbox(mode wazero.Mode) *wazero.Sandbox {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if sb, ok := sharedSandboxes[mode]; ok {
		return sb
	}
	sb := wazero.New(wazero.WithMode(mode))
	sharedSandboxes[mode] = sb
	return sb
}

// evaluatorIn builds an evaluator over the shared sandbox that enforces the
// time limit in the given mode.
func evaluatorIn(tb testing.TB, mode wazero.Mode, opts expr.Options) *expr.Evaluator {
	tb.Helper()
	opts.Sandbox = sharedSandbox(mode)
	if opts.Timeout == 0 {
		opts.Timeout = defaultTestTimeout
	}
	return expr.New(opts)
}

func TestEvalSmoke(t *testing.T) {
	ev := evaluator(t, expr.Options{DisableFastPath: true})
	s := ev.Open()
	t.Cleanup(func() { _ = s.Close(context.WithoutCancel(t.Context())) })
	env := newEnv(t, `{"name":"Ada","n":[1,2,3]}`)
	for src, want := range map[string]string{
		`={{ $json.name }}`:              `"Ada"`,
		`={{ $json.n.map(x => x * 2) }}`: `[2,4,6]`,
		`={{ $now.toISO() }}`:            `"2026-10-13T12:30:00.000Z"`,
		`={{ $now.setZone('America/Bogota').toFormat('yyyy-MM-dd HH:mm') }}`: `"2026-10-13 07:30"`,
		`={{ 1 + 1 }}`:     `2`,
		`={{ undefined }}`: `null`,
	} {
		p, err := ev.Compile(src)
		if err != nil {
			t.Fatal(err)
		}
		res, err := s.Eval(t.Context(), p, env)
		if err != nil {
			t.Errorf("Eval(%s): %v", src, err)
			continue
		}
		got, _ := jsonvalue.Canonical(res.Value)
		if string(got) != want {
			t.Errorf("Eval(%s) = %s, want %s", src, got, want)
		}
	}
}
