// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func jv(tb testing.TB, text string) jsonvalue.Value {
	tb.Helper()
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		tb.Fatalf("Parse(%s): %v", text, err)
	}
	return v
}

var epoch = time.Date(2026, 10, 13, 12, 30, 0, 0, time.UTC)

func newEnv(tb testing.TB, item string) *expr.Env {
	tb.Helper()
	var seed [expr.SeedSize]byte
	seed[0] = 7
	return &expr.Env{
		Input:      []expr.Item{{JSON: jv(tb, item)}},
		Activation: expr.NewActivation(epoch, seed),
	}
}

// canon renders a result the way tests compare them.
func canon(tb testing.TB, r expr.Result) string {
	tb.Helper()
	if r.Undefined {
		return "undefined"
	}
	b, err := jsonvalue.Canonical(r.Value)
	if err != nil {
		tb.Fatalf("Canonical: %v", err)
	}
	return string(b)
}

// eval compiles and evaluates source with an evaluator that has no sandbox.
func eval(tb testing.TB, source, item string) (expr.Result, error) {
	tb.Helper()
	ev := expr.New(expr.Options{})
	p, err := ev.Compile(source)
	if err != nil {
		tb.Fatalf("Compile(%q): %v", source, err)
	}
	s := ev.Open()
	defer func() { _ = s.Close(tb.Context()) }()
	return s.Eval(tb.Context(), p, newEnv(tb, item))
}

// fakeSandbox is a sandbox whose runtimes answer from a script and can call
// back into the guest.
type fakeSandbox struct {
	mu       sync.Mutex
	opened   int
	openErr  error
	config   expr.SandboxConfig
	runtimes []*fakeRuntime
	// answer decides the reply of every Call of runtimes opened from now on.
	answer func(req string, g expr.Guest) ([]byte, error)
}

func (f *fakeSandbox) Open(_ context.Context, cfg expr.SandboxConfig) (expr.Runtime, error) { //nolint:ireturn // implements the expr.Sandbox port
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.openErr != nil {
		return nil, f.openErr
	}
	f.opened++
	f.config = cfg
	rt := &fakeRuntime{answer: f.answer}
	f.runtimes = append(f.runtimes, rt)
	return rt, nil
}

type fakeRuntime struct {
	mu       sync.Mutex
	answer   func(req string, g expr.Guest) ([]byte, error)
	requests []string
	closed   bool
	usage    int64
}

func (r *fakeRuntime) Call(ctx context.Context, request []byte, g expr.Guest) ([]byte, error) {
	r.mu.Lock()
	r.requests = append(r.requests, string(request))
	r.mu.Unlock()
	if r.answer == nil {
		return nil, errors.New("fakeRuntime: no script")
	}
	return r.answer(string(request), g)
}

func (r *fakeRuntime) MemoryUsage(context.Context) int64 { return r.usage }

func (r *fakeRuntime) Close(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return nil
}

func replying(text string) func(string, expr.Guest) ([]byte, error) {
	return func(string, expr.Guest) ([]byte, error) { return []byte(text), nil }
}
