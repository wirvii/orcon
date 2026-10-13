// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// header splits a request of the sandbox protocol into its header (parsed) and
// its four sections.
func splitRequest(t *testing.T, req string) (jsonvalue.Value, []string) {
	t.Helper()
	parts := strings.Split(req, "\x01")
	if len(parts) != 5 {
		t.Fatalf("request has %d parts, want 5: %q", len(parts), req)
	}
	return jv(t, parts[0]), parts[1:]
}

func headerString(t *testing.T, h jsonvalue.Value, key string) string {
	t.Helper()
	m, ok := h.Get(key)
	if !ok {
		return ""
	}
	if s, ok := m.Str(); ok {
		return s
	}
	b, _ := jsonvalue.Canonical(m)
	return string(b)
}

func sandboxEvaluator(sb *fakeSandbox, opts expr.Options) *expr.Evaluator {
	opts.Sandbox = sb
	return expr.New(opts)
}

func TestSessionSandboxRequestAndAnswer(t *testing.T) {
	t.Parallel()
	sb := &fakeSandbox{answer: replying(`{"v":{"b":[1,2]}}`)}
	ev := sandboxEvaluator(sb, expr.Options{DisableFastPath: true})
	s := ev.Open()
	defer func() { _ = s.Close(t.Context()) }()
	env := newEnv(t, `{"a":{"x":1},"big":"padding","n":5}`)
	env.ItemIndex = 0
	env.RunIndex = 3
	env.Timezone = "Europe/Madrid"
	env.Vars = jv(t, `{"stage":"prod"}`)
	env.Parameters = jv(t, `{"p":1}`)
	env.Execution = expr.ExecutionInfo{ID: "ex-1", ResumeURL: "https://example.test/r"}
	env.Workflow = expr.WorkflowInfo{ID: "wf-1", Name: "Flow", Active: true}
	env.PrevNode = expr.PrevNode{Name: "Prev", OutputIndex: 2, RunIndex: 1}

	p, err := ev.Compile(`={{ $json.a.x + $execution.id + $workflow.name + $prevNode.name }}`)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Eval(t.Context(), p, env)
	if err != nil {
		t.Fatal(err)
	}
	if got := canon(t, res); got != `{"b":[1,2]}` {
		t.Errorf("result = %s", got)
	}
	if sb.opened != 1 || len(sb.config.Scripts) != 6 || sb.config.Entry != "__orcon_run" ||
		sb.config.MemoryLimit != expr.DefaultMemoryLimit || sb.config.StackLimit != expr.DefaultStackLimit {
		t.Errorf("sandbox config = %+v opened %d", sb.config, sb.opened)
	}
	req := sb.runtimes[0].requests[0]
	h, sections := splitRequest(t, req)
	for key, want := range map[string]string{"m": "v", "i": "0", "r": "3", "n": "1791894600000", "z": "Europe/Madrid"} {
		if got := headerString(t, h, key); got != want {
			t.Errorf("header %s = %q, want %q", key, got, want)
		}
	}
	if got := headerString(t, h, "e"); got != "$json.a.x + $execution.id + $workflow.name + $prevNode.name" {
		t.Errorf("header e = %q", got)
	}
	x, _ := h.Get("x")
	if id, _ := x.Get("id"); jsonString(id) != "ex-1" {
		t.Errorf("execution header = %v", x)
	}
	if mode, _ := x.Get("mode"); jsonString(mode) != "production" {
		t.Errorf("execution mode defaults to production, got %v", mode)
	}
	w, _ := h.Get("w")
	if name, _ := w.Get("name"); jsonString(name) != "Flow" {
		t.Errorf("workflow header = %v", w)
	}
	// Only $json.a is read, so only that path of the item is sent.
	if sections[0] != `{"a":{"x":1}}` {
		t.Errorf("$json section = %q, want the projection {\"a\":{\"x\":1}}", sections[0])
	}
	if sections[1] != "" || sections[2] != "" || sections[3] != "" {
		t.Errorf("unused sections were sent: %q", sections[1:])
	}
	if st := ev.Stats(); st.Sandboxed != 1 || st.Opened != 1 || st.Fast != 0 {
		t.Errorf("stats = %+v", st)
	}
}

func jsonString(v jsonvalue.Value) string {
	s, _ := v.Str()
	return s
}

func TestSessionSectionsFollowTheCode(t *testing.T) {
	t.Parallel()
	whole := `{"a":1,"b":{"c":2},"list":[1,2]}`
	tests := []struct {
		code                          string
		json, binary, vars, params    string
		lenient                       bool
		projectionDisabled, hasHeader bool
	}{
		{code: `={{ $json.b.c }}`, json: `{"b":{"c":2}}`},
		{code: `={{ $json.list[0] }}`, json: `{"list":[1,2]}`},
		{code: `={{ $json.b.c + $json.a }}`, json: `{"a":1,"b":{"c":2}}`},
		{code: `={{ JSON.stringify($json) }}`, json: whole},
		{code: `={{ $json.b.c }}`, json: whole, projectionDisabled: true},
		{code: `={{ $input.item.json.a }}`, json: whole, binary: `{"f":1}`, params: `{"p":1}`},
		{code: `={{ $binary.f }}`, binary: `{"f":1}`},
		{code: `={{ $vars.stage }}`, vars: `{"stage":"prod"}`},
		{code: `={{ $parameter.p }}`, params: `{"p":1}`},
		{code: `={{ eval('1') }}`, json: whole, binary: `{"f":1}`, vars: `{"stage":"prod"}`, params: `{"p":1}`},
		{code: `={{ 1 + }}`, json: whole, binary: `{"f":1}`, vars: `{"stage":"prod"}`, params: `{"p":1}`},
		{code: `={{ 1 }}`},
	}
	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			t.Parallel()
			sb := &fakeSandbox{answer: replying(`{"v":1}`)}
			ev := sandboxEvaluator(sb, expr.Options{DisableFastPath: true, DisableProjection: tc.projectionDisabled})
			s := ev.Open()
			defer func() { _ = s.Close(t.Context()) }()
			env := newEnv(t, whole)
			env.Input[0].Binary = jv(t, `{"f":1}`)
			env.Vars = jv(t, `{"stage":"prod"}`)
			env.Parameters = jv(t, `{"p":1}`)
			p, err := ev.Compile(tc.code)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Eval(t.Context(), p, env); err != nil {
				t.Fatal(err)
			}
			_, sections := splitRequest(t, sb.runtimes[0].requests[0])
			if diff := cmp.Diff([]string{tc.json, tc.binary, tc.vars, tc.params}, sections); diff != "" {
				t.Errorf("sections (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSessionAnswersAndErrors(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	tests := []struct {
		name   string
		answer string
		want   string
		check  func(t *testing.T, err error)
	}{
		{name: "undefined", answer: `{"u":1}`, want: "undefined"},
		{name: "string mode answer in value mode", answer: `{"s":"text"}`, want: `"text"`},
		{name: "expression error", answer: `{"x":{"n":"TypeError","m":"boom"}}`, check: func(t *testing.T, err error) {
			t.Helper()
			var ee *expr.EvalError
			if !errors.As(err, &ee) || ee.Name != "TypeError" || ee.Message != "boom" || ee.Compile || err.Error() != "TypeError: boom" {
				t.Errorf("error = %v", err)
			}
		}},
		{name: "compile error", answer: `{"x":{"n":"SyntaxError","m":"bad","c":1}}`, check: func(t *testing.T, err error) {
			t.Helper()
			var ee *expr.EvalError
			if !errors.As(err, &ee) || !ee.Compile {
				t.Errorf("error = %v, want a compile-time EvalError", err)
			}
		}},
		{name: "out of memory", answer: `{"x":{"n":"InternalError","m":"out of memory"}}`, check: func(t *testing.T, err error) {
			t.Helper()
			if !errors.Is(err, expr.ErrMemory) {
				t.Errorf("error = %v, want ErrMemory", err)
			}
		}},
		{name: "stack overflow", answer: `{"x":{"n":"InternalError","m":"stack overflow"}}`, check: func(t *testing.T, err error) {
			t.Helper()
			if !errors.Is(err, expr.ErrStack) {
				t.Errorf("error = %v, want ErrStack", err)
			}
		}},
		{name: "other internal error", answer: `{"x":{"n":"InternalError","m":"string too long"}}`, check: func(t *testing.T, err error) {
			t.Helper()
			var ee *expr.EvalError
			if !errors.As(err, &ee) || errors.Is(err, expr.ErrMemory) {
				t.Errorf("error = %v", err)
			}
		}},
		{name: "call stack range error is a stack limit", answer: `{"x":{"n":"RangeError","m":"Maximum call stack size exceeded"}}`, check: func(t *testing.T, err error) {
			t.Helper()
			if !errors.Is(err, expr.ErrStack) {
				t.Errorf("error = %v, want ErrStack", err)
			}
		}},
		{name: "unreadable answer", answer: `not json`, check: func(t *testing.T, err error) {
			t.Helper()
			if !errors.Is(err, expr.ErrSandbox) {
				t.Errorf("error = %v, want ErrSandbox", err)
			}
		}},
		{name: "answer without a result", answer: `{}`, check: func(t *testing.T, err error) {
			t.Helper()
			if !errors.Is(err, expr.ErrSandbox) {
				t.Errorf("error = %v, want ErrSandbox", err)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sb := &fakeSandbox{answer: replying(tc.answer)}
			ev := sandboxEvaluator(sb, expr.Options{DisableFastPath: true})
			s := ev.Open()
			defer func() { _ = s.Close(ctx) }()
			res, err := s.Eval(ctx, mustProgram(t, ev, `={{ x }}`), newEnv(t, `{}`))
			if tc.check != nil {
				tc.check(t, err)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := canon(t, res); got != tc.want {
				t.Errorf("result = %s, want %s", got, tc.want)
			}
		})
	}
}

func mustProgram(t *testing.T, ev *expr.Evaluator, src string) *expr.Program {
	t.Helper()
	p, err := ev.Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSessionDiscardsAFailedRuntime(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	calls := 0
	sb := &fakeSandbox{answer: func(string, expr.Guest) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, expr.ErrTimeout
		}
		return []byte(`{"v":2}`), nil
	}}
	ev := sandboxEvaluator(sb, expr.Options{DisableFastPath: true})
	s := ev.Open()
	defer func() { _ = s.Close(ctx) }()
	p := mustProgram(t, ev, `={{ 1 }}`)
	if _, err := s.Eval(ctx, p, newEnv(t, `{}`)); !errors.Is(err, expr.ErrTimeout) {
		t.Fatalf("first evaluation error = %v, want ErrTimeout", err)
	}
	if !sb.runtimes[0].closed {
		t.Error("the failed runtime was not closed")
	}
	res, err := s.Eval(ctx, p, newEnv(t, `{}`))
	if err != nil || canon(t, res) != "2" {
		t.Fatalf("second evaluation = %v, %v", res, err)
	}
	if sb.opened != 2 {
		t.Errorf("runtimes opened = %d, want a fresh one for the second evaluation", sb.opened)
	}
}

func TestSessionPreconditions(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	// No sandbox configured.
	ev := expr.New(expr.Options{DisableFastPath: true})
	if _, err := ev.Open().Eval(ctx, mustProgram(t, ev, `={{ 1 }}`), newEnv(t, `{}`)); !errors.Is(err, expr.ErrNoSandbox) {
		t.Errorf("without a sandbox: %v", err)
	}
	// No activation.
	sb := &fakeSandbox{answer: replying(`{"v":1}`)}
	ev = sandboxEvaluator(sb, expr.Options{DisableFastPath: true})
	env := newEnv(t, `{}`)
	env.Activation = nil
	if _, err := ev.Open().Eval(ctx, mustProgram(t, ev, `={{ 1 }}`), env); !errors.Is(err, expr.ErrNoActivation) {
		t.Errorf("without an activation: %v", err)
	}
	// Sandbox fails to open.
	boom := errors.New("no memory")
	failing := &fakeSandbox{openErr: boom}
	ev = sandboxEvaluator(failing, expr.Options{DisableFastPath: true})
	if _, err := ev.Open().Eval(ctx, mustProgram(t, ev, `={{ 1 }}`), newEnv(t, `{}`)); !errors.Is(err, boom) {
		t.Errorf("open failure: %v", err)
	}
	// A closed session refuses work.
	ev = sandboxEvaluator(sb, expr.Options{DisableFastPath: true})
	s := ev.Open()
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Eval(ctx, mustProgram(t, ev, `={{ 1 }}`), newEnv(t, `{}`)); err == nil {
		t.Error("a closed session evaluated")
	}
	if err := s.Close(ctx); err != nil {
		t.Errorf("closing twice: %v", err)
	}
	// Too large a segment.
	if _, err := ev.Compile("={{ " + strings.Repeat("1+", expr.MaxSourceSize) + "1 }}"); !errors.Is(err, expr.ErrTooLarge) {
		t.Errorf("oversize segment: %v", err)
	}
	// A large literal text around small code is fine.
	if _, err := ev.Compile("=" + strings.Repeat("x", 2*expr.MaxSourceSize) + "{{ 1 }}"); err != nil {
		t.Errorf("large text: %v", err)
	}
}

func TestSessionMemoryUsageAndClose(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	sb := &fakeSandbox{answer: replying(`{"v":1}`)}
	ev := sandboxEvaluator(sb, expr.Options{DisableFastPath: true})
	s := ev.Open()
	if s.MemoryUsage(ctx) != 0 {
		t.Error("MemoryUsage before any runtime is not zero")
	}
	if _, err := s.Eval(ctx, mustProgram(t, ev, `={{ 1 }}`), newEnv(t, `{}`)); err != nil {
		t.Fatal(err)
	}
	sb.runtimes[0].usage = 1234
	if got := s.MemoryUsage(ctx); got != 1234 {
		t.Errorf("MemoryUsage = %d", got)
	}
	if err := s.Close(ctx); err != nil || !sb.runtimes[0].closed {
		t.Errorf("Close: %v, runtime closed %v", err, sb.runtimes[0].closed)
	}
}

func TestSessionTemplatesJoinSegments(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	sb := &fakeSandbox{answer: func(req string, _ expr.Guest) ([]byte, error) {
		h, _ := splitRequest(t, req)
		if headerString(t, h, "m") != "s" {
			t.Errorf("a segment inside text must be requested as a string: %s", req)
		}
		return []byte(`{"s":"[` + headerString(t, h, "e") + `]"}`), nil
	}}
	ev := sandboxEvaluator(sb, expr.Options{})
	s := ev.Open()
	defer func() { _ = s.Close(ctx) }()
	res, err := s.Eval(ctx, mustProgram(t, ev, `=a {{ $json.name.toUpperCase() }} b {{ 1 + 1 }} c {{ $json.q.trim() }}`), newEnv(t, `{"name":"x","q":" y "}`))
	if err != nil {
		t.Fatal(err)
	}
	// 1 + 1 is on the fast path; the two method calls go to the sandbox.
	if got := canon(t, res); got != `"a [$json.name.toUpperCase()] b 2 c [$json.q.trim()]"` {
		t.Errorf("result = %s", got)
	}
	st := ev.Stats()
	if st.Fast != 1 || st.Sandboxed != 2 {
		t.Errorf("stats = %+v, want one fast and two sandboxed segments", st)
	}
	// A segment the fast path starts and cannot finish is counted as bailed.
	res, err = s.Eval(ctx, mustProgram(t, ev, `=v: {{ $json.o }}`), newEnv(t, `{"o":{"a":1}}`))
	if err != nil || ev.Stats().Bailed == 0 {
		t.Errorf("bail: %v %v %+v", res, err, ev.Stats())
	}
	// An error of a fast-path-eligible segment surfaces as the sandbox's answer.
	failing := &fakeSandbox{answer: replying(`{"x":{"n":"TypeError","m":"nope"}}`)}
	ev2 := sandboxEvaluator(failing, expr.Options{})
	s2 := ev2.Open()
	defer func() { _ = s2.Close(ctx) }()
	if _, err := s2.Eval(ctx, mustProgram(t, ev2, `=t {{ $json.a.b }}`), newEnv(t, `{}`)); err == nil {
		t.Error("an error of the sandbox did not surface")
	}
}

func TestProbeIgnoresEffectsOfLenientData(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	for _, tc := range []struct {
		answer string
		fail   bool
	}{
		{`{"v":1}`, false},
		{`{"x":{"n":"RangeError","m":"Invalid unit value NaN"}}`, false},
		{`{"x":{"n":"TypeError","m":"cannot read property 'json' of undefined"}}`, false},
		{`{"x":{"n":"TypeError","m":"x.y is not a function"}}`, true},
		{`{"x":{"n":"ReferenceError","m":"foo is not defined"}}`, true},
		{`{"x":{"n":"SyntaxError","m":"unexpected token","c":1}}`, true},
	} {
		sb := &fakeSandbox{answer: replying(tc.answer)}
		ev := sandboxEvaluator(sb, expr.Options{})
		s := ev.Open()
		err := s.Probe(ctx, "x", newEnv(t, `{}`))
		_ = s.Close(ctx)
		if (err != nil) != tc.fail {
			t.Errorf("Probe with %s: error = %v, want failure %v", tc.answer, err, tc.fail)
		}
		if len(sb.runtimes) > 0 && len(sb.runtimes[0].requests) > 0 && !strings.Contains(sb.runtimes[0].requests[0], `"l":true`) {
			t.Errorf("Probe did not ask for the lenient context: %s", sb.runtimes[0].requests[0])
		}
	}
}

func TestProgramSegments(t *testing.T) {
	t.Parallel()
	ev := expr.New(expr.Options{})
	p := mustProgram(t, ev, `=a {{ $json.x }} b {{ foo.bar() }}`)
	if p.Source() != `=a {{ $json.x }} b {{ foo.bar() }}` || p.CodeSegments() != 2 || p.FastSegments() != 1 {
		t.Errorf("program = %q code %d fast %d", p.Source(), p.CodeSegments(), p.FastSegments())
	}
	segs := p.Segments()
	if len(segs) != 4 || !segs[1].Code || !segs[1].Fast || segs[3].Fast || segs[0].Code {
		t.Errorf("segments = %+v", segs)
	}
	lit := mustProgram(t, ev, `just text`)
	if lit.CodeSegments() != 0 || len(lit.Segments()) != 1 {
		t.Errorf("literal program = %+v", lit.Segments())
	}
	res, err := ev.Open().Eval(t.Context(), lit, newEnv(t, `{}`))
	if err != nil || canon(t, res) != `"just text"` {
		t.Errorf("literal evaluation = %v, %v", res, err)
	}
}

func TestActivation(t *testing.T) {
	t.Parallel()
	var seed [expr.SeedSize]byte
	seed[3] = 9
	at := time.Date(2026, 1, 2, 3, 4, 5, 6, time.FixedZone("x", 3600))
	a, b := expr.NewActivation(at, seed), expr.NewActivation(at, seed)
	if !a.Time().Equal(at) || a.Time().Location() != time.UTC {
		t.Errorf("Time() = %v", a.Time())
	}
	for range 100 {
		x, y := a.Float64(), b.Float64()
		if x != y || x < 0 || x >= 1 {
			t.Fatalf("draws differ or leave [0,1): %v %v", x, y)
		}
	}
	pa, pb := make([]byte, 40), make([]byte, 40)
	a.Read(pa)
	b.Read(pb)
	if !bytes.Equal(pa, pb) {
		t.Error("Read differs for the same seed")
	}
	seed[3] = 10
	c := expr.NewActivation(at, seed)
	pc := make([]byte, 40)
	c.Read(pc)
	if bytes.Equal(pa, pc) {
		t.Error("a different seed gave the same bytes")
	}
}

// guestHarness evaluates through a fake runtime whose answer calls the guest.
func guestOps(t *testing.T, env *expr.Env, calls func(g expr.Guest)) {
	t.Helper()
	sb := &fakeSandbox{answer: func(_ string, g expr.Guest) ([]byte, error) {
		calls(g)
		return []byte(`{"v":1}`), nil
	}}
	ev := sandboxEvaluator(sb, expr.Options{DisableFastPath: true})
	s := ev.Open()
	defer func() { _ = s.Close(t.Context()) }()
	if _, err := s.Eval(t.Context(), mustProgram(t, ev, `={{ x }}`), env); err != nil {
		t.Fatal(err)
	}
}

func TestGuestOperations(t *testing.T) {
	t.Parallel()
	items := []expr.Item{{JSON: jv(t, `{"id":1}`), Binary: jv(t, `{"f":{}}`)}, {JSON: jv(t, `{"id":2}`)}}
	env := newEnv(t, `{"id":0}`)
	env.Input = items
	env.Nodes = fakeNodes{
		"Fetch": {Executed: true, Items: items, Params: jv(t, `{"url":"u"}`), Paired: func(i int) (expr.Item, bool) {
			if i >= 0 && i < len(items) {
				return items[i], true
			}
			return expr.Item{}, false
		}},
		"Skipped": {Executed: false},
		"NoPair":  {Executed: true, Items: items},
	}
	ctx := t.Context()
	guestOps(t, env, func(g expr.Guest) {
		call := func(op int32, payload string) string {
			out, err := g.HostCall(ctx, op, []byte(payload))
			if err != nil {
				t.Errorf("HostCall(%d, %q): %v", op, payload, err)
			}
			return string(out)
		}
		if got := call(expr.OpInputAll, ""); got != `[{"json":{"id":1},"binary":{"f":{}}},{"json":{"id":2}}]` {
			t.Errorf("OpInputAll = %s", got)
		}
		if got := call(expr.OpNodeMeta, "Fetch"); got != `{"exists":true,"executed":true,"params":{"url":"u"},"count":2}` {
			t.Errorf("OpNodeMeta = %s", got)
		}
		if got := call(expr.OpNodeMeta, "Skipped"); !strings.Contains(got, `"executed":false`) {
			t.Errorf("OpNodeMeta(skipped) = %s", got)
		}
		if got := call(expr.OpNodeMeta, "Nope"); got != `{"exists":false}` {
			t.Errorf("OpNodeMeta(missing) = %s", got)
		}
		if got := call(expr.OpNodeItems, "Fetch"); got != `[{"json":{"id":1},"binary":{"f":{}}},{"json":{"id":2}}]` {
			t.Errorf("OpNodeItems = %s", got)
		}
		if got := call(expr.OpNodeItems, "Nope"); got != "" {
			t.Errorf("OpNodeItems(missing) = %q", got)
		}
		if got := call(expr.OpNodePaired, `{"n":"Fetch","i":1}`); got != `{"json":{"id":2}}` {
			t.Errorf("OpNodePaired = %s", got)
		}
		for _, bad := range []string{`{"n":"Fetch","i":9}`, `{"n":"NoPair","i":0}`, `{"n":"Nope","i":0}`} {
			if got := call(expr.OpNodePaired, bad); got != "" {
				t.Errorf("OpNodePaired(%s) = %q, want nothing", bad, got)
			}
		}
		if _, err := g.HostCall(ctx, expr.OpNodePaired, []byte(`not json`)); err == nil {
			t.Error("OpNodePaired accepted unreadable input")
		}
		if got := call(expr.OpRandom, ""); !strings.HasPrefix(got, "0.") {
			t.Errorf("OpRandom = %s", got)
		}
		if got := call(expr.OpRandomBytes, "4"); len(got) != 8 {
			t.Errorf("OpRandomBytes(4) = %q, want 8 hex digits", got)
		}
		for _, bad := range []string{"x", "-1", "99999999"} {
			if _, err := g.HostCall(ctx, expr.OpRandomBytes, []byte(bad)); err == nil {
				t.Errorf("OpRandomBytes(%q) succeeded", bad)
			}
		}
		if got := call(expr.OpZoneFields, "America/Bogota\t1791913530250"); got != "2026 10 13 12 45 30" {
			t.Errorf("OpZoneFields = %q", got)
		}
		if got := call(expr.OpZoneFields, "America/Bogota\t1791913530250"); got != "2026 10 13 12 45 30" { // cached zone
			t.Errorf("OpZoneFields (cached) = %q", got)
		}
		for _, bad := range []string{"Mars/Base\t0", "Local\t0", "\t0", "UTC", "UTC\tabc"} {
			if got := call(expr.OpZoneFields, bad); got != "" {
				t.Errorf("OpZoneFields(%q) = %q, want nothing", bad, got)
			}
		}
		hashes := map[string]string{
			"md5\tabc":    "900150983cd24fb0d6963f7d28e17f72",
			"sha1\tabc":   "a9993e364706816aba3e25717850c26c9cd0d89d",
			"sha224\tabc": "23097d223405d8228642a477bda255b32aadbce4bda0b3f7e36c9da7",
			"sha256\tabc": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
			"sha384\tabc": "cb00753f45a35e8bb5a03d699ac65007272c32ab0eded1631a8b605a43ff5bed8086072ba1e7cc2358baeca134c825a7",
			"sha512\tabc": "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
			"sha3\tabc":   "3a985da74fe225b2045c172d6bd390bd855f086e3e9d525b46bfe24511431532",
			"base64\tabc": "YWJj",
		}
		for in, want := range hashes {
			if got := call(expr.OpHash, in); got != want {
				t.Errorf("OpHash(%q) = %s, want %s", in, got, want)
			}
		}
		for _, bad := range []string{"crc32\tabc", "noseparator"} {
			if got := call(expr.OpHash, bad); got != "" {
				t.Errorf("OpHash(%q) = %q, want nothing", bad, got)
			}
		}
		if _, err := g.HostCall(ctx, 99, nil); err == nil {
			t.Error("an unknown operation succeeded")
		}
		if !g.Now().Equal(epoch) {
			t.Errorf("Now() = %v", g.Now())
		}
		buf := make([]byte, 8)
		g.Read(buf)
	})

	// Without a node source every node is missing.
	bare := newEnv(t, `{}`)
	guestOps(t, bare, func(g expr.Guest) {
		if out, _ := g.HostCall(ctx, expr.OpNodeMeta, []byte("A")); string(out) != `{"exists":false}` {
			t.Errorf("OpNodeMeta without nodes = %s", out)
		}
	})
}
