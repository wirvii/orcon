// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
)

// echoGuest answers host calls with the payload prefixed by the op.
type echoGuest struct{ at time.Time }

func (g echoGuest) HostCall(_ context.Context, op int32, payload []byte) ([]byte, error) {
	return append([]byte{byte('0' + op)}, payload...), nil //nolint:gosec // op is a small test constant
}
func (g echoGuest) Now() time.Time { return g.at }
func (g echoGuest) Read(p []byte)  { clear(p) }

const echoScript = `
globalThis.__orcon_run = function (req) {
  if (req === "host") return __host(3, "abc");
  if (req === "now") return String(Date.now());
  if (req === "throw") throw new Error("boom");
  return req.toUpperCase();
};
`

func open(t *testing.T, sb *wazero.Sandbox, memory uint32) expr.Runtime { //nolint:ireturn // the helper hands out the port under test
	t.Helper()
	rt, err := sb.Open(t.Context(), expr.SandboxConfig{
		Scripts:     []expr.Script{{Name: "echo.js", Source: echoScript}},
		Entry:       "__orcon_run",
		MemoryLimit: memory,
		StackLimit:  512 << 10,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close(context.WithoutCancel(t.Context())) })
	return rt
}

func TestSmoke(t *testing.T) {
	rt := open(t, sharedSandbox(wazero.ModeHardLimit), 64<<20)
	guest := echoGuest{at: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)}
	for req, want := range map[string]string{
		"hello": "HELLO",
		"host":  "3abc",
		"now":   "1791892800000",
	} {
		out, err := rt.Call(t.Context(), []byte(req), guest)
		if err != nil {
			t.Fatalf("Call(%q): %v", req, err)
		}
		if string(out) != want {
			t.Errorf("Call(%q) = %q, want %q", req, out, want)
		}
	}
	_, err := rt.Call(t.Context(), []byte("throw"), guest)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("Call(throw) error = %v, want one naming boom", err)
	}
	if !errors.Is(err, expr.ErrSandbox) {
		t.Errorf("Call(throw) error = %v, want ErrSandbox", err)
	}
}
