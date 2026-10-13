// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func activation(at time.Time, seedByte byte) *expr.Activation {
	var seed [expr.SeedSize]byte
	seed[0] = seedByte
	return expr.NewActivation(at, seed)
}

func evalWith(t *testing.T, ev *expr.Evaluator, s *expr.Session, act *expr.Activation, src string) string {
	t.Helper()
	env := newEnv(t, `{}`)
	env.Activation = act
	res, err := s.Eval(t.Context(), mustCompile(t, ev, src), env)
	if err != nil {
		t.Fatalf("Eval(%s): %v", src, err)
	}
	return outcome(res, nil)
}

// TestDeterminism checks ADR-0018 section 5: during one node activation $now,
// $today and Date.now() are the recorded activation time, and Math.random and
// crypto.randomUUID draw from a ChaCha8 stream seeded by the recorded seed, so
// recomputing the activation gives identical results.
func TestDeterminism(t *testing.T) {
	ev := evaluator(t, expr.Options{})
	at := time.Date(2026, 10, 13, 17, 45, 30, 250_000_000, time.UTC)

	t.Run("the clock is the activation time", func(t *testing.T) {
		s := ev.Open()
		defer func() { _ = s.Close(t.Context()) }()
		act := activation(at, 1)
		first := evalWith(t, ev, s, act, `={{ [Date.now(), new Date().getTime(), $now.toMillis(), $today.toISO(), new Date().toISOString(), $now.toISO()] }}`)
		time.Sleep(30 * time.Millisecond)
		second := evalWith(t, ev, s, act, `={{ [Date.now(), new Date().getTime(), $now.toMillis(), $today.toISO(), new Date().toISOString(), $now.toISO()] }}`)
		if first != second {
			t.Errorf("the clock moved during the activation:\n  %s\n  %s", first, second)
		}
		want := `[1791913530250,1791913530250,1791913530250,"2026-10-13T00:00:00.000Z","2026-10-13T17:45:30.250Z","2026-10-13T17:45:30.250Z"]`
		if first != want {
			t.Errorf("clock values = %s, want %s", first, want)
		}
	})

	t.Run("the zone moves $today but not the instant", func(t *testing.T) {
		s := ev.Open()
		defer func() { _ = s.Close(t.Context()) }()
		env := newEnv(t, `{}`)
		env.Activation = activation(at, 1)
		env.Timezone = "Asia/Tokyo"
		res, err := s.Eval(t.Context(), mustCompile(t, ev, `={{ [$now.toISO(), $today.toISO(), $now.toMillis()] }}`), env)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := outcome(res, nil), `["2026-10-14T02:45:30.250+09:00","2026-10-14T00:00:00.000+09:00",1791913530250]`; got != want {
			t.Errorf("= %s, want %s", got, want)
		}
	})

	const draws = `={{ [Math.random(), Math.random(), crypto.randomUUID(), Math.random(), crypto.randomUUID()] }}`
	t.Run("random values replay from the seed", func(t *testing.T) {
		// A different runtime, a different session and a new Activation built from
		// the same record give the same stream: this is the recomputation after a crash.
		a, b := ev.Open(), ev.Open()
		defer func() { _ = a.Close(t.Context()); _ = b.Close(t.Context()) }()
		first := evalWith(t, ev, a, activation(at, 9), draws)
		replay := evalWith(t, ev, b, activation(at, 9), draws)
		if first != replay {
			t.Errorf("recomputation differs:\n  %s\n  %s", first, replay)
		}
		other := evalWith(t, ev, a, activation(at, 10), draws)
		if other == first {
			t.Error("a different seed gave the same stream")
		}
		// The stream advances within an activation and restarts with a new one.
		act := activation(at, 9)
		one := evalWith(t, ev, a, act, draws)
		two := evalWith(t, ev, a, act, draws)
		if one != first || two == one {
			t.Errorf("stream state: first draw %s (want %s), second draw equal to the first: %v", one, first, two == one)
		}
	})

	t.Run("random values have the documented shape", func(t *testing.T) {
		s := ev.Open()
		defer func() { _ = s.Close(t.Context()) }()
		act := activation(at, 3)
		res, err := s.Eval(t.Context(), mustCompile(t, ev, `={{ [Math.random(), crypto.randomUUID()] }}`), withAct(newEnv(t, `{}`), act))
		if err != nil {
			t.Fatal(err)
		}
		items := res.Value.Items()
		f, ok := items[0].Float64()
		if !ok || f < 0 || f >= 1 {
			t.Errorf("Math.random() = %v, want a number in [0, 1)", items[0])
		}
		id, _ := items[1].Str()
		if len(id) != 36 || strings.Count(id, "-") != 4 || id[14] != '4' || !strings.ContainsRune("89ab", rune(id[19])) {
			t.Errorf("crypto.randomUUID() = %q, want a version 4 UUID", id)
		}
	})

	t.Run("the activation time is what the sandbox sees, not the wall clock", func(t *testing.T) {
		s := ev.Open()
		defer func() { _ = s.Close(t.Context()) }()
		old := time.Date(1999, 12, 31, 23, 59, 59, 0, time.UTC)
		if got := evalWith(t, ev, s, activation(old, 1), `={{ new Date().getFullYear() }}`); got != "1999" {
			t.Errorf("year = %s, want 1999", got)
		}
	})
}

func withAct(env *expr.Env, act *expr.Activation) *expr.Env {
	env.Activation = act
	return env
}

var _ = jsonvalue.Null
