// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // expressions resolve time zones the same way on every host.

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

const (
	modeValue  = "v"
	modeString = "s"
)

func stringResult(s string) Result { return Result{Value: jsonvalue.String(s)} }

// Probe runs code in the sandbox against a lenient context: every name of the
// n8n context, such as $json or $('Node'), answers with a value that accepts
// any property access, call or conversion. It reports whether the code itself
// is runnable. Unknown identifiers, unknown methods of JavaScript values and
// syntax errors surface as an [*EvalError]; the data does not matter. The
// compatibility measurement uses it (ADR-0018 section 6).
func (s *Session) Probe(ctx context.Context, code string, env *Env) error {
	_, err := s.sandboxMode(ctx, prepareCode(code), needs{all: true}, env, modeValue, true)
	var ee *EvalError
	if errors.As(err, &ee) && !ee.Compile && !isCodeDefect(ee) {
		return nil // an effect of the lenient data, not of the code
	}
	return err
}

// isCodeDefect reports whether an error thrown while running lenient code
// shows a defect of the code rather than of the data it ran on.
func isCodeDefect(ee *EvalError) bool {
	return ee.Name == "ReferenceError" || (ee.Name == "TypeError" && strings.Contains(ee.Message, "not a function"))
}

func (s *Session) sandbox(ctx context.Context, code string, need needs, env *Env, mode string) (Result, error) {
	return s.sandboxMode(ctx, code, need, env, mode, false)
}

func (s *Session) sandboxMode(ctx context.Context, code string, need needs, env *Env, mode string, lenient bool) (Result, error) {
	if s.ev.opts.Sandbox == nil {
		return Result{}, ErrNoSandbox
	}
	if env.Activation == nil {
		return Result{}, ErrNoActivation
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Result{}, errors.New("expr: the session is closed")
	}
	if s.rt == nil {
		rt, err := s.ev.opts.Sandbox.Open(ctx, SandboxConfig{
			Scripts:     s.ev.scripts,
			Entry:       "__orcon_run",
			MemoryLimit: s.ev.opts.MemoryLimit,
			StackLimit:  s.ev.opts.StackLimit,
		})
		if err != nil {
			return Result{}, fmt.Errorf("expr: open the sandbox: %w", err)
		}
		s.rt = rt
		s.ev.opened.Add(1)
	}
	s.ev.sandboxed.Add(1)
	req := buildRequest(code, mode, lenient, env, need)
	callCtx, cancel := context.WithTimeout(ctx, s.ev.opts.Timeout)
	defer cancel()
	out, err := s.rt.Call(callCtx, req, &guest{env: env})
	if err != nil {
		// A runtime that failed is discarded: the next evaluation of the
		// session starts from a pristine one.
		_ = s.dropLocked(context.WithoutCancel(ctx))
		return Result{}, err
	}
	res, err := parseAnswer(out)
	if err != nil {
		var ee *EvalError
		if errors.As(err, &ee) && (ee.Name == "InternalError") {
			// QuickJS reports exhausted limits as InternalError; the runtime is
			// discarded, as for any sandbox stop.
			_ = s.dropLocked(context.WithoutCancel(ctx))
			switch {
			case strings.Contains(ee.Message, "out of memory"):
				return Result{}, fmt.Errorf("%w: %s", ErrMemory, ee.Message)
			case strings.Contains(ee.Message, "stack overflow"):
				return Result{}, fmt.Errorf("%w: %s", ErrStack, ee.Message)
			}
		}
		return Result{}, err
	}
	return res, nil
}

// parseAnswer decodes the envelope of the runtime entry point.
func parseAnswer(out []byte) (Result, error) {
	v, err := jsonvalue.Parse(out)
	if err != nil {
		return Result{}, fmt.Errorf("%w: unreadable answer: %w", ErrSandbox, err)
	}
	if x, ok := v.Get("x"); ok {
		name, msg := member1(x, "n"), member1(x, "m")
		_, compile := x.Get("c")
		return Result{}, &EvalError{Name: name, Message: msg, Compile: compile}
	}
	if _, ok := v.Get("u"); ok {
		return Result{Value: jsonvalue.Null(), Undefined: true}, nil
	}
	if s, ok := v.Get("s"); ok {
		return Result{Value: s}, nil
	}
	if r, ok := v.Get("v"); ok {
		return Result{Value: r}, nil
	}
	return Result{}, fmt.Errorf("%w: answer has no result", ErrSandbox)
}

// member1 returns the string member key of v, or "" when there is none.
func member1(v jsonvalue.Value, key string) string {
	m, ok := v.Get(key)
	if !ok {
		return ""
	}
	s, _ := m.Str()
	return s
}

// ---- request ---------------------------------------------------------------

// jsonText renders a JSON value as text; an absent or unencodable value is {},
// the empty object the sandbox sees for a missing section.
func jsonText(v jsonvalue.Value) string {
	if v.IsZero() {
		return "{}"
	}
	b, err := jsonvalue.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func quote(s string) string {
	b, err := jsonvalue.Marshal(jsonvalue.String(s))
	if err != nil {
		// The string is not valid UTF-8: replace the offending bytes, as the
		// runtime would see them.
		b, _ = jsonvalue.Marshal(jsonvalue.String(strings.ToValidUTF8(s, "�")))
	}
	return string(b)
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// ---- guest ---------------------------------------------------------------------

// guest serves the host operations of a running evaluation from its Env.
type guest struct {
	env *Env
}

var _ Guest = (*guest)(nil)

func (g *guest) Now() time.Time { return g.env.Activation.Time() }

func (g *guest) Read(p []byte) { g.env.Activation.Read(p) }

func itemJSON(it Item) string {
	var b strings.Builder
	b.WriteString(`{"json":`)
	b.WriteString(jsonText(it.JSON))
	if !it.Binary.IsZero() {
		b.WriteString(`,"binary":`)
		b.WriteString(jsonText(it.Binary))
	}
	b.WriteByte('}')
	return b.String()
}

func itemsJSON(items []Item) []byte {
	var b strings.Builder
	b.WriteByte('[')
	for i := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(itemJSON(items[i]))
	}
	b.WriteByte(']')
	return []byte(b.String())
}

// HostCall implements [Guest].
func (g *guest) HostCall(_ context.Context, op int32, payload []byte) ([]byte, error) {
	switch op {
	case OpInputAll:
		return itemsJSON(g.env.Input), nil
	case OpNodeMeta:
		return g.nodeMeta(string(payload)), nil
	case OpNodeItems:
		out, ok := g.node(string(payload))
		if !ok {
			return nil, nil
		}
		return itemsJSON(out.Items), nil
	case OpNodePaired:
		return g.nodePaired(payload)
	case OpRandom:
		return []byte(strconv.FormatFloat(g.env.Activation.Float64(), 'g', -1, 64)), nil
	case OpRandomBytes:
		n, err := strconv.Atoi(string(payload))
		if err != nil || n < 0 || n > 1<<16 {
			return nil, fmt.Errorf("expr: invalid random byte count %q", payload)
		}
		buf := make([]byte, n)
		g.env.Activation.Read(buf)
		return []byte(hex.EncodeToString(buf)), nil
	case OpZoneFields:
		return zoneFields(string(payload)), nil
	case OpHash:
		return hashText(string(payload)), nil
	default:
		return nil, fmt.Errorf("expr: unknown host operation %d", op)
	}
}

func (g *guest) node(name string) (NodeOutput, bool) {
	if g.env.Nodes == nil {
		return NodeOutput{}, false
	}
	return g.env.Nodes.Node(name)
}

func (g *guest) nodeMeta(name string) []byte {
	out, ok := g.node(name)
	if !ok {
		return []byte(`{"exists":false}`)
	}
	var b strings.Builder
	b.WriteString(`{"exists":true,"executed":`)
	b.WriteString(strconv.FormatBool(out.Executed))
	b.WriteString(`,"params":`)
	b.WriteString(jsonText(out.Params))
	b.WriteString(`,"count":`)
	b.WriteString(strconv.Itoa(len(out.Items)))
	b.WriteByte('}')
	return []byte(b.String())
}

func (g *guest) nodePaired(payload []byte) ([]byte, error) {
	req, err := jsonvalue.Parse(payload)
	if err != nil {
		return nil, fmt.Errorf("expr: unreadable paired-item request: %w", err)
	}
	name := member1(req, "n")
	idx := 0
	if m, ok := req.Get("i"); ok {
		if n, ok := m.Int64(); ok {
			idx = int(n)
		}
	}
	out, ok := g.node(name)
	if !ok || out.Paired == nil {
		return nil, nil
	}
	it, ok := out.Paired(idx)
	if !ok {
		return nil, nil
	}
	return []byte(itemJSON(it)), nil
}

var (
	zoneMu    sync.Mutex
	zoneCache = map[string]*time.Location{}
)

// zoneFields answers OpZoneFields: "<zone>\t<epoch ms>" becomes
// "year month day hour minute second" in that zone, or nothing for a zone the
// time zone database does not know.
func zoneFields(payload string) []byte {
	name, ms, ok := strings.Cut(payload, "\t")
	if !ok || name == "" || name == "Local" {
		return nil
	}
	zoneMu.Lock()
	loc, cached := zoneCache[name]
	if !cached {
		l, err := time.LoadLocation(name)
		if err == nil && len(zoneCache) < 1024 {
			zoneCache[name] = l
		}
		if err != nil {
			l = nil
		}
		loc = l
	}
	zoneMu.Unlock()
	if loc == nil {
		return nil
	}
	n, err := strconv.ParseInt(ms, 10, 64)
	if err != nil {
		return nil
	}
	t := time.UnixMilli(n).In(loc)
	return []byte(fmt.Sprintf("%d %d %d %d %d %d", t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second()))
}
