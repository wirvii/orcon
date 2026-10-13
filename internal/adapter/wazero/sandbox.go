// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero

import (
	"context"
	_ "embed" // quickjs.wasm is embedded.
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"

	"github.com/wirvii/orcon/internal/expr"
	"github.com/wirvii/orcon/internal/platform/config"
)

//go:embed assets/quickjs.wasm
var quickjsWasm []byte

// Sizes of the WebAssembly memory.
const (
	// The size of a WebAssembly memory page.
	pageSize = 65536
	// The smallest memory cap that can hold the runtime and the prelude scripts.
	minMemoryLimit = 16 << 20
	// What the WebAssembly memory keeps beyond the JavaScript heap limit: the C
	// stack, the allocator's own bookkeeping and the buffers of a call. The
	// JavaScript heap hits its limit and throws a catchable error before the
	// memory cap makes an allocation fail.
	reserve = 8 << 20
)

// Mode selects how the time limit of a call is enforced.
type Mode uint8

const (
	// ModeHardLimit enforces the deadline by closing the module when the
	// context is done (wazero's close-on-context-done), as ADR-0018 section 3
	// prescribes. It stops any code, at the price of checks that wazero compiles
	// into every function and loop of the module: the QuickJS interpreter runs
	// about six times slower with them.
	ModeHardLimit Mode = iota
	// ModeCooperativeLimit enforces the deadline through the QuickJS interrupt
	// handler, which the interpreter polls in loops, calls and long built-ins
	// (every ten thousand operations, about a millisecond), so a runaway
	// expression stops promptly and the interpreter runs at full speed. Native
	// code that never polls, such as a quadratic string search over megabytes,
	// is only bounded by the memory cap. See doc.go and the measurement report.
	ModeCooperativeLimit
	// ModeCooperativeAbandon is ModeCooperativeLimit plus an answer at the
	// deadline: the call runs on its own goroutine and the caller stops waiting
	// when the context is done, whatever the module is doing. The abandoned call
	// still runs until its next poll of the interrupt handler or its end, then
	// its runtime is closed and never used again. It is the production mode of
	// expression evaluations (ADR-0018 section 3, SPEC-001 amendment 10) and
	// the default of [New]. ModeHardLimit stays for the measurement and for the
	// slices that run other guests (the Code node, WASM connectors) until each
	// measures its own stop.
	ModeCooperativeAbandon
)

// String names the mode.
func (m Mode) String() string {
	switch m {
	case ModeCooperativeLimit:
		return "cooperative limit"
	case ModeCooperativeAbandon:
		return "cooperative limit with abandon"
	case ModeHardLimit:
		return "hard limit"
	}
	return "unknown mode"
}

// Option configures a [Sandbox].
type Option func(*Sandbox)

// WithMode selects how the time limit is enforced. The default is
// [ModeCooperativeAbandon]. The other two modes exist for the measurement
// report and the tests; the composition root must not select them
// (TestProductionStopMode).
func WithMode(m Mode) Option { return func(s *Sandbox) { s.mode = m } }

// WithMaxAbandoned sets the limit of abandoned calls still running. The
// default is [config.SandboxMaxAbandonedDefault]; a value outside the range
// 1 to 64 is moved to the nearest bound (the composition root rejects it
// earlier, with [config.ValidSandboxMaxAbandoned]).
func WithMaxAbandoned(n int) Option { return func(s *Sandbox) { s.maxAbandoned = n } }

// WithSlots sets how many calls may execute guest code at once (default
// GOMAXPROCS - 1, at least 1). Every live call holds a slot until its guest
// returns, abandoned calls included.
func WithSlots(n int) Option { return func(s *Sandbox) { s.slots = n } }

// WithSlotWait bounds the wait for a slot; the default is [DefaultSlotWait].
func WithSlotWait(d time.Duration) Option { return func(s *Sandbox) { s.slotWait = d } }

// WithStuckAfter sets how long an abandoned call may keep running before it
// counts as stuck. The default is [DefaultStuckAfter].
func WithStuckAfter(d time.Duration) Option { return func(s *Sandbox) { s.stuckAfter = d } }

// WithLogger sets the logger of the abandon accounting.
func WithLogger(l *slog.Logger) Option { return func(s *Sandbox) { s.log = l } }

// WithMetrics sets the receiver of the abandon metrics.
func WithMetrics(m Metrics) Option { return func(s *Sandbox) { s.metrics = m } }

// Sandbox implements [expr.Sandbox]. It compiles the QuickJS module once and
// instantiates it for every [Sandbox.Open]. It is safe for concurrent use.
type Sandbox struct {
	mode  Mode
	cache wazero.CompilationCache

	maxAbandoned int
	slots        int
	slotWait     time.Duration
	stuckAfter   time.Duration
	log          *slog.Logger
	metrics      Metrics
	track        *tracker

	mu       sync.Mutex
	closed   bool
	runtimes map[uint32]*engine
}

// engine is one wazero runtime: the memory cap is a property of the runtime,
// so each distinct cap gets its own, sharing the compilation cache.
type engine struct {
	mode     Mode
	track    *tracker
	rt       wazero.Runtime
	compiled wazero.CompiledModule

	// mu guards snapshots and serializes the first Open of each configuration,
	// which builds the snapshot.
	mu        sync.Mutex
	snapshots map[snapshotKey]*snapshot
}

var _ expr.Sandbox = (*Sandbox)(nil)

// Mode reports how the sandbox enforces the time limit of a call.
func (s *Sandbox) Mode() Mode { return s.mode }

// New returns a sandbox. It compiles the module on the first Open of each
// memory cap; the compiled code is shared.
func New(opts ...Option) *Sandbox {
	s := &Sandbox{
		mode: ModeCooperativeAbandon, cache: wazero.NewCompilationCache(), runtimes: map[uint32]*engine{},
		maxAbandoned: config.SandboxMaxAbandonedDefault,
	}
	for _, o := range opts {
		o(s)
	}
	if s.slots <= 0 {
		s.slots = max(runtime.GOMAXPROCS(0)-1, 1)
	}
	s.track = newTracker(s.maxAbandoned, s.slots, s.slotWait, s.stuckAfter, s.log, s.metrics)
	return s
}

// Close releases every runtime. Runtimes opened from the sandbox must be
// closed first.
func (s *Sandbox) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var errs []error
	for pages, e := range s.runtimes {
		errs = append(errs, e.rt.Close(ctx))
		delete(s.runtimes, pages)
	}
	errs = append(errs, s.cache.Close(ctx))
	return errors.Join(errs...)
}

// engineFor returns the runtime that enforces the given memory cap in pages.
func (s *Sandbox) engineFor(ctx context.Context, pages uint32) (*engine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("wazero: sandbox is closed")
	}
	if e, ok := s.runtimes[pages]; ok {
		return e, nil
	}
	cfg := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(s.mode == ModeHardLimit).
		WithMemoryLimitPages(pages).
		WithCompilationCache(s.cache)
	rt := wazero.NewRuntimeWithConfig(ctx, cfg)
	if err := instantiateHostModules(ctx, rt); err != nil {
		_ = rt.Close(ctx)
		return nil, err
	}
	compiled, err := rt.CompileModule(ctx, quickjsWasm)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("wazero: compile the expression runtime: %w", err)
	}
	e := &engine{mode: s.mode, track: s.track, rt: rt, compiled: compiled, snapshots: map[snapshotKey]*snapshot{}}
	s.runtimes[pages] = e
	return e, nil
}

// Open implements [expr.Sandbox].
func (s *Sandbox) Open(ctx context.Context, cfg expr.SandboxConfig) (expr.Runtime, error) { //nolint:ireturn // implements the expr.Sandbox port
	limit := max(cfg.MemoryLimit, minMemoryLimit)
	pages := (limit + pageSize - 1) / pageSize
	e, err := s.engineFor(ctx, pages)
	if err != nil {
		return nil, err
	}
	if s.mode == ModeCooperativeAbandon {
		// Opening runs a little guest code, so it takes a slot like a call.
		if err := s.track.acquireSlot(ctx); err != nil {
			return nil, err
		}
		defer s.track.releaseSlot()
	}
	return e.open(ctx, cfg, limit)
}
