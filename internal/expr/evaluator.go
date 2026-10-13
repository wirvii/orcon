// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed prelude/*.js
var preludeFS embed.FS

// Default limits of an evaluation (ADR-0018 section 3).
const (
	// DefaultTimeout is the time limit of one expression.
	DefaultTimeout = 100 * time.Millisecond
	// DefaultMemoryLimit is the memory cap of one runtime, in bytes.
	DefaultMemoryLimit = 64 << 20
	// DefaultStackLimit is the JavaScript call-stack cap, in bytes.
	DefaultStackLimit = 512 << 10
	// MaxSourceSize bounds the code of one {{ }} segment.
	MaxSourceSize = 64 << 10
)

// Errors of the evaluator.
var (
	// ErrNoSandbox means an expression needs the sandbox and the evaluator was
	// built without one.
	ErrNoSandbox = errors.New("expr: the expression needs the sandbox, which is not configured")
	// ErrNoActivation means an expression needs the recorded activation time
	// and random stream, and the Env carries none.
	ErrNoActivation = errors.New("expr: the evaluation needs the recorded activation of the node")
	// ErrTooLarge means the code of a {{ }} segment exceeds [MaxSourceSize].
	ErrTooLarge = errors.New("expr: the expression is too large")
)

// EvalError is an error thrown by the expression itself (a TypeError, a
// ReferenceError, a syntax error, an error raised by the runtime such as
// "Referenced node is unexecuted"). It is the author's expression that failed;
// the sandbox did its job. Sandbox failures are [ErrTimeout], [ErrMemory],
// [ErrStack] and [ErrSandbox].
type EvalError struct {
	// Name is the JavaScript error name, for example "TypeError".
	Name    string
	Message string
	// Compile is set when the error was raised while compiling the expression
	// (a syntax error), before any of it ran.
	Compile bool
}

func (e *EvalError) Error() string { return e.Name + ": " + e.Message }

// Is reports [ErrStack] for the RangeError the runtime throws when the
// JavaScript stack limit is reached, so callers can tell a limit from a bug in
// the expression.
func (e *EvalError) Is(target error) bool {
	return target == ErrStack && e.Name == "RangeError" && strings.Contains(e.Message, "call stack")
}

// Options configure an [Evaluator].
type Options struct {
	// Sandbox runs the expressions the fast path cannot. Nil means such
	// expressions fail with [ErrNoSandbox].
	Sandbox Sandbox
	// Timeout is the time limit of one sandbox evaluation; zero means
	// [DefaultTimeout].
	Timeout time.Duration
	// MemoryLimit is the memory cap of a sandbox runtime in bytes; zero means
	// [DefaultMemoryLimit].
	MemoryLimit uint32
	// StackLimit is the JavaScript stack cap in bytes; zero means
	// [DefaultStackLimit].
	StackLimit uint32
	// DisableFastPath sends every expression to the sandbox. It exists for the
	// differential tests and the measurements.
	DisableFastPath bool
	// DisableProjection sends the sandbox all of $json even when the code reads
	// a few properties. It exists for the differential tests.
	DisableProjection bool
}

// Stats counts what an evaluator has done.
type Stats struct {
	// Fast is the number of code segments evaluated by the fast path.
	Fast uint64
	// Bailed is the number of segments the fast path started and handed to
	// the sandbox because a value was outside what it reproduces exactly.
	Bailed uint64
	// Sandboxed is the number of segments evaluated in the sandbox (including
	// the bailed ones).
	Sandboxed uint64
	// Opened is the number of sandbox runtimes opened.
	Opened uint64
}

// Evaluator compiles and evaluates expressions. It is safe for concurrent
// use; a [Session] is bound to one execution.
type Evaluator struct {
	opts    Options
	scripts []Script

	fast, bailed, sandboxed, opened atomic.Uint64
}

// New returns an evaluator.
func New(opts Options) *Evaluator {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.MemoryLimit == 0 {
		opts.MemoryLimit = DefaultMemoryLimit
	}
	if opts.StackLimit == 0 {
		opts.StackLimit = DefaultStackLimit
	}
	return &Evaluator{opts: opts, scripts: preludeScripts()}
}

// Stats returns the counters of the evaluator.
func (e *Evaluator) Stats() Stats {
	return Stats{Fast: e.fast.Load(), Bailed: e.bailed.Load(), Sandboxed: e.sandboxed.Load(), Opened: e.opened.Load()}
}

// preludeScripts returns the scripts every runtime loads, in order.
func preludeScripts() []Script {
	var out []Script
	for _, name := range []string{"intl.js", "luxon.min.js", "runtime.js", "jmespath.js", "extensions.js", "warm.js", "harden.js"} {
		src, err := preludeFS.ReadFile("prelude/" + name)
		if err != nil {
			panic(fmt.Sprintf("expr: the embedded prelude lacks %s: %v", name, err)) // a build defect, not a runtime condition
		}
		out = append(out, Script{Name: name, Source: string(src)})
	}
	return out
}

// SandboxConfig returns the configuration of the runtimes the evaluator opens,
// for tools that open runtimes themselves.
func (e *Evaluator) SandboxConfig() SandboxConfig {
	return SandboxConfig{
		Scripts:     e.scripts,
		Entry:       "__orcon_run",
		Setup:       setupGuest{},
		MemoryLimit: e.opts.MemoryLimit,
		StackLimit:  e.opts.StackLimit,
	}
}

// Prelude returns the scripts a sandbox runtime loads before it serves
// expressions. It exists for tools that open runtimes directly.
func Prelude() []Script { return preludeScripts() }

// Program is a compiled expression template. It is immutable and safe for
// concurrent use; compile it once per flow version.
type Program struct {
	source string
	segs   []segment
}

type segment struct {
	Segment
	fast *fastProgram
	// run is the code as the sandbox runs it (see prepareCode), and need what
	// the sandbox must be given for it (see needsOf).
	run  string
	need needs
}

// Compile prepares a parameter value. A value that does not start with "=" is
// literal text; otherwise the {{ }} segments are classified for the fast path.
func (e *Evaluator) Compile(source string) (*Program, error) {
	p := &Program{source: source}
	if !IsTemplate(source) {
		p.segs = []segment{{Segment: Segment{Text: source}}}
		return p, nil
	}
	for _, s := range Segments(source[1:]) {
		if s.Code && len(s.Text) > MaxSourceSize {
			return nil, fmt.Errorf("%w: a {{ }} segment of %d bytes exceeds %d", ErrTooLarge, len(s.Text), MaxSourceSize)
		}
		seg := segment{Segment: s}
		if s.Code {
			seg.run = prepareCode(s.Text)
			seg.need = needsOf(s.Text, e.opts.DisableProjection)
			if !e.opts.DisableFastPath {
				seg.fast = compileFast(s.Text)
			}
		}
		p.segs = append(p.segs, seg)
	}
	return p, nil
}

// Source returns the text the program was compiled from.
func (p *Program) Source() string { return p.source }

// SegmentInfo describes one segment of a compiled program.
type SegmentInfo struct {
	Segment
	// Fast reports that the fast path can evaluate the code segment.
	Fast bool
}

// Segments returns the literal and code segments of the program in order.
func (p *Program) Segments() []SegmentInfo {
	out := make([]SegmentInfo, len(p.segs))
	for i, s := range p.segs {
		out[i] = SegmentInfo{Segment: s.Segment, Fast: s.fast != nil}
	}
	return out
}

// CodeSegments returns the number of {{ }} segments of the program.
func (p *Program) CodeSegments() int {
	n := 0
	for _, s := range p.segs {
		if s.Code {
			n++
		}
	}
	return n
}

// FastSegments returns how many code segments the fast path can evaluate. The
// publication step records it; the measurements report the ratio.
func (p *Program) FastSegments() int {
	n := 0
	for _, s := range p.segs {
		if s.Code && s.fast != nil {
			n++
		}
	}
	return n
}

// Session evaluates the expressions of one execution. It holds at most one
// sandbox runtime, opened on the first expression that needs it and never
// shared with another execution (ADR-0018 section 3). Close it when the
// execution ends. A Session serializes its evaluations.
type Session struct {
	ev *Evaluator

	mu     sync.Mutex
	rt     Runtime
	closed bool
}

// Open returns a session for one execution.
func (e *Evaluator) Open() *Session { return &Session{ev: e} }

// Close releases the sandbox runtime of the session, if one was opened.
func (s *Session) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return s.dropLocked(ctx)
}

func (s *Session) dropLocked(ctx context.Context) error {
	if s.rt == nil {
		return nil
	}
	err := s.rt.Close(ctx)
	s.rt = nil
	return err
}

// MemoryUsage reports the bytes the sandbox runtime of the session holds, or 0
// when none is open.
func (s *Session) MemoryUsage(ctx context.Context) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rt == nil {
		return 0
	}
	return s.rt.MemoryUsage(ctx)
}

// Eval evaluates the program for env. A program that is a single {{ }}
// segment returns the typed value; any other template returns a string, the
// segments joined with JavaScript's string conversion.
func (s *Session) Eval(ctx context.Context, p *Program, env *Env) (Result, error) {
	if len(p.segs) == 1 && p.segs[0].Code {
		return s.evalValue(ctx, p.segs[0], env)
	}
	var b strings.Builder
	for _, seg := range p.segs {
		if !seg.Code {
			b.WriteString(seg.Text)
			continue
		}
		text, err := s.evalString(ctx, seg, env)
		if err != nil {
			return Result{}, err
		}
		b.WriteString(text)
	}
	return stringResult(b.String()), nil
}

func (s *Session) evalValue(ctx context.Context, seg segment, env *Env) (Result, error) {
	if seg.fast != nil {
		v, err := seg.fast.eval(env)
		if err == nil {
			s.ev.fast.Add(1)
			return v.result()
		}
		if !errors.Is(err, errBail) {
			return Result{}, err
		}
		s.ev.bailed.Add(1)
	}
	return s.sandbox(ctx, seg.run, seg.need, env, modeValue)
}

func (s *Session) evalString(ctx context.Context, seg segment, env *Env) (string, error) {
	if seg.fast != nil {
		v, err := seg.fast.eval(env)
		if err == nil {
			if text, terr := toStr(v); terr == nil {
				s.ev.fast.Add(1)
				return text, nil
			}
		}
		if err != nil && !errors.Is(err, errBail) {
			return "", err
		}
		s.ev.bailed.Add(1)
	}
	res, err := s.sandbox(ctx, seg.run, seg.need, env, modeString)
	if err != nil {
		return "", err
	}
	text, _ := res.Value.Str()
	return text, nil
}
