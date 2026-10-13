// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"

	"github.com/wirvii/orcon/internal/expr"
)

// instance is one QuickJS runtime inside one WebAssembly module instance.
type instance struct {
	mod api.Module
	mem api.Memory

	alloc, free, eval, outPtr, outLen, memUsed api.Function

	mu     sync.Mutex
	broken bool
	// reqPtr and reqCap are a request buffer reused across calls.
	reqPtr, reqCap uint32

	// answerAtDeadline makes Call run the module on its own goroutine and stop
	// waiting when the context is done (ModeCooperativeAbandon). stateMu guards
	// the hand-over between the caller and that goroutine.
	answerAtDeadline bool
	track            *tracker
	// abandoned is set, before the caller returns, once the call was given up;
	// the host functions read it to refuse the guest from then on.
	abandoned atomic.Bool
	// stretch is the longest time between two calls of the guest to the host
	// in the last call, in nanoseconds.
	stretch     atomic.Int64
	tail        atomic.Int64
	first       atomic.Int64
	stateMu     sync.Mutex
	callDone    bool
	abandonedAt time.Time
	endedAt     time.Time
	ended       chan struct{}
}

// refuses reports whether the host must stop serving the guest of the call:
// the call was abandoned or its context is done. An abandoned runtime reads no
// more execution data and draws no more random numbers.
func (in *instance) refuses(ctx context.Context) bool {
	return in != nil && in.abandoned.Load() || ctx.Err() != nil
}

var _ expr.Runtime = (*instance)(nil)

// snapshot is the linear memory of a runtime right after its scripts loaded.
// A new instance starts from a copy of it instead of running the scripts
// again: loading Luxon and the prelude takes tens of milliseconds, the copy
// well under one. Nothing else carries state: the module's mutable globals
// are back at their initial values once initialization returns.
type snapshot struct {
	memory []byte
}

type snapshotKey [sha256.Size]byte

func keyOf(cfg expr.SandboxConfig, memoryLimit uint32) snapshotKey {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%d/%d/%d/%s", memoryLimit, cfg.StackLimit, len(cfg.Scripts), cfg.Entry)
	for _, s := range cfg.Scripts {
		_, _ = fmt.Fprintf(h, "\x00%s\x00%d\x00", s.Name, len(s.Source))
		_, _ = h.Write([]byte(s.Source))
	}
	var k snapshotKey
	h.Sum(k[:0])
	return k
}

// open returns an instance with the scripts of cfg loaded.
func (e *engine) open(ctx context.Context, cfg expr.SandboxConfig, memoryLimit uint32) (*instance, error) {
	key := keyOf(cfg, memoryLimit)
	e.mu.Lock()
	snap, ok := e.snapshots[key]
	if ok {
		e.mu.Unlock()
		return e.restore(ctx, snap)
	}
	// The first open of a configuration builds the snapshot while holding the
	// lock, so concurrent first opens do not each pay for it.
	defer e.mu.Unlock()
	in, err := e.build(ctx, cfg, memoryLimit)
	if err != nil {
		return nil, err
	}
	mem, ok := in.mem.Read(0, in.mem.Size())
	if !ok {
		_ = in.mod.Close(ctx)
		return nil, errors.New("wazero: cannot read the memory of the new runtime")
	}
	end := len(mem)
	for end > 0 && mem[end-1] == 0 {
		end--
	}
	e.snapshots[key] = &snapshot{memory: bytes.Clone(mem[:end])}
	return in, nil
}

// restore instantiates the module without running its initializers and loads
// the snapshot over its memory.
func (e *engine) restore(ctx context.Context, snap *snapshot) (*instance, error) {
	mod, err := e.rt.InstantiateModule(ctx, e.compiled, wazero.NewModuleConfig().WithName("").WithStartFunctions())
	if err != nil {
		return nil, fmt.Errorf("wazero: instantiate the expression runtime: %w", err)
	}
	in := &instance{mod: mod, mem: mod.Memory(), answerAtDeadline: e.mode == ModeCooperativeAbandon, track: e.track}
	have := in.mem.Size() / pageSize
	if need := uint32((len(snap.memory) + pageSize - 1) / pageSize); need > have { //nolint:gosec // bounded by the memory cap.
		if _, ok := in.mem.Grow(need - have); !ok {
			_ = mod.Close(ctx)
			return nil, fmt.Errorf("%w: the snapshot does not fit the memory cap", expr.ErrMemory)
		}
	}
	if !in.mem.Write(0, snap.memory) {
		_ = mod.Close(ctx)
		return nil, errors.New("wazero: cannot load the snapshot into the new runtime")
	}
	if err := in.bind(); err != nil {
		_ = mod.Close(ctx)
		return nil, err
	}
	return in, nil
}

// build instantiates the module, creates the QuickJS runtime and loads the
// scripts.
func (e *engine) build(ctx context.Context, cfg expr.SandboxConfig, memoryLimit uint32) (*instance, error) {
	// The reactor module's _initialize export runs the C library constructors.
	mod, err := e.rt.InstantiateModule(ctx, e.compiled, newModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("wazero: instantiate the expression runtime: %w", err)
	}
	in := &instance{mod: mod, mem: mod.Memory(), answerAtDeadline: e.mode == ModeCooperativeAbandon, track: e.track}
	if err := in.bind(); err != nil {
		_ = mod.Close(ctx)
		return nil, err
	}
	// The call runs under a state whose guest is a no-op: loading scripts
	// needs the clock and nothing else.
	var setup expr.Guest = noGuest{}
	if cfg.Setup != nil {
		setup = cfg.Setup
	}
	loadCtx := withCall(ctx, &callState{guest: setup, inst: in})
	jsLimit := memoryLimit - reserve
	var flags int32
	if e.mode != ModeHardLimit {
		flags |= 1 // install the QuickJS interrupt handler
	}
	if err := in.start(loadCtx, int32(jsLimit), int32(cfg.StackLimit), flags); err != nil { //nolint:gosec // limits are far below 2 GiB.
		_ = mod.Close(ctx)
		return nil, err
	}
	for _, s := range cfg.Scripts {
		if err := in.load(loadCtx, s); err != nil {
			_ = mod.Close(ctx)
			return nil, err
		}
	}
	if err := in.bindEntry(loadCtx, cfg.Entry); err != nil {
		_ = mod.Close(ctx)
		return nil, err
	}
	return in, nil
}

func (in *instance) bind() error {
	names := map[string]*api.Function{
		"orcon_alloc": &in.alloc, "orcon_free": &in.free, "orcon_eval": &in.eval,
		"orcon_out_ptr": &in.outPtr, "orcon_out_len": &in.outLen, "orcon_memory_used": &in.memUsed,
	}
	for name, dst := range names {
		fn := in.mod.ExportedFunction(name)
		if fn == nil {
			return fmt.Errorf("wazero: the expression runtime does not export %s", name)
		}
		*dst = fn
	}
	return nil
}

func (in *instance) call(ctx context.Context, fn api.Function, args ...uint64) (uint64, error) {
	var stack [4]uint64
	copy(stack[:], args)
	if err := fn.CallWithStack(ctx, stack[:]); err != nil {
		return 0, err
	}
	return stack[0], nil
}

func (in *instance) start(ctx context.Context, memoryLimit, stackLimit, flags int32) error {
	fn := in.mod.ExportedFunction("orcon_init")
	if fn == nil {
		return errors.New("wazero: the expression runtime does not export orcon_init")
	}
	res, err := in.call(ctx, fn, api.EncodeI32(memoryLimit), api.EncodeI32(stackLimit), api.EncodeI32(flags))
	if err != nil {
		return classify(err)
	}
	if code := api.DecodeI32(res); code != 0 {
		return fmt.Errorf("%w: orcon_init returned %d", expr.ErrSandbox, code)
	}
	return nil
}

// put copies b into guest memory and returns its address.
func (in *instance) put(ctx context.Context, b []byte) (uint32, error) {
	res, err := in.call(ctx, in.alloc, uint64(len(b)))
	if err != nil {
		return 0, err
	}
	ptr := api.DecodeU32(res)
	if ptr == 0 || !in.mem.Write(ptr, b) {
		return 0, fmt.Errorf("%w: guest allocation of %d bytes failed", expr.ErrMemory, len(b))
	}
	return ptr, nil
}

func (in *instance) load(ctx context.Context, s expr.Script) error {
	fn := in.mod.ExportedFunction("orcon_load")
	// QuickJS requires the script to be NUL-terminated past its length.
	src, err := in.put(ctx, append([]byte(s.Source), 0))
	if err != nil {
		return classify(err)
	}
	name, err := in.put(ctx, append([]byte(s.Name), 0))
	if err != nil {
		return classify(err)
	}
	res, err := in.call(ctx, fn, uint64(src), uint64(len(s.Source)), uint64(name))
	_, _ = in.call(ctx, in.free, uint64(src))
	_, _ = in.call(ctx, in.free, uint64(name))
	if err != nil {
		return classify(err)
	}
	if api.DecodeI32(res) != 0 {
		return fmt.Errorf("wazero: loading %s: %s", s.Name, in.output(ctx))
	}
	return nil
}

func (in *instance) bindEntry(ctx context.Context, entry string) error {
	// The entry point is looked up by the fixed name __orcon_run (see
	// orcon_qjs.c); the configured name is checked against it so a prelude
	// that renames it fails loudly.
	if entry != "__orcon_run" {
		return fmt.Errorf("wazero: unsupported entry point %q; the runtime binds __orcon_run", entry)
	}
	res, err := in.call(ctx, in.mod.ExportedFunction("orcon_bind"))
	if err != nil {
		return classify(err)
	}
	if api.DecodeI32(res) != 0 {
		return fmt.Errorf("wazero: the scripts did not define %s", entry)
	}
	return nil
}

// output reads the output buffer of the last call. It runs detached from the
// cancellation of ctx: the answer of a call that finished is read even if its
// deadline passed an instant later.
func (in *instance) output(ctx context.Context) string {
	ctx = context.WithoutCancel(ctx)
	p, err := in.call(ctx, in.outPtr)
	if err != nil {
		return ""
	}
	n, err := in.call(ctx, in.outLen)
	if err != nil || n == 0 {
		return ""
	}
	b, ok := in.mem.Read(api.DecodeU32(p), api.DecodeU32(n))
	if !ok {
		return ""
	}
	return string(b)
}

// Call implements [expr.Runtime].
func (in *instance) Call(ctx context.Context, request []byte, guest expr.Guest) ([]byte, error) {
	if in.answerAtDeadline {
		return in.callAndAbandon(ctx, request, guest)
	}
	if d := expr.TimeoutFrom(ctx); d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.broken {
		return nil, fmt.Errorf("%w: the runtime failed earlier and must be reopened", expr.ErrSandbox)
	}
	st := &callState{guest: guest, inst: in, begun: time.Now()}
	st.deadline, st.hasDeadline = ctx.Deadline()
	out, err := in.callLocked(withCall(ctx, st), request)
	st.finish()
	in.stretch.Store(int64(st.longest))
	in.tail.Store(int64(st.tail))
	in.first.Store(int64(st.first))
	if err == nil {
		err = st.err
	}
	if err != nil {
		in.broken = true
		return nil, err
	}
	return out, nil
}

type callResult struct {
	out []byte
	err error
}

// callAndAbandon runs the call on its own goroutine and answers the caller
// when the call returns or its deadline passes, whichever comes first; the
// caller never waits for its call. It first takes an execution slot, which the
// goroutine returns when the guest returns, abandoned or not, and the deadline
// counts from the moment the slot is held. On the deadline the call is
// abandoned: it keeps the runtime locked and the slot until it ends (its next
// consult of the host sees the deadline by the clock), then the runtime is
// closed and never used again. SPEC-001 amendment 10, A.2, A.4 and D.
func (in *instance) callAndAbandon(ctx context.Context, request []byte, guest expr.Guest) ([]byte, error) {
	if err := in.track.admit(ctx); err != nil {
		return nil, err
	}
	if err := in.track.acquireSlot(ctx); err != nil {
		return nil, err
	}
	in.mu.Lock()
	if in.broken {
		in.mu.Unlock()
		in.track.releaseSlot()
		return nil, fmt.Errorf("%w: the runtime failed earlier and must be reopened", expr.ErrSandbox)
	}
	callCtx, cancel := ctx, context.CancelFunc(func() {})
	if d := expr.TimeoutFrom(ctx); d > 0 {
		callCtx, cancel = context.WithTimeout(ctx, d)
	}
	started := time.Now()
	in.ended = make(chan struct{})
	in.callDone = false
	in.abandoned.Store(false)
	var gave *abandonment // set by the caller, under stateMu, when it gives up
	done := make(chan callResult, 1)
	st := &callState{guest: guest, inst: in, begun: time.Now()}
	st.deadline, st.hasDeadline = callCtx.Deadline()
	go func() {
		out, err := in.callLocked(withCall(callCtx, st), request)
		st.finish()
		in.stretch.Store(int64(st.longest))
		in.tail.Store(int64(st.tail))
		in.first.Store(int64(st.first))
		if err == nil {
			err = st.err
		}
		if err != nil {
			in.broken = true
		}
		in.stateMu.Lock()
		in.callDone = true
		in.endedAt = time.Now()
		given := gave
		in.stateMu.Unlock()
		if given != nil {
			// Nobody is waiting: the runtime is not reused, so close it here.
			in.broken = true
			_ = in.mod.Close(context.WithoutCancel(ctx))
			given.ended()
		}
		in.track.releaseSlot()
		close(in.ended)
		done <- callResult{out, err}
		in.mu.Unlock()
		cancel()
	}()
	select {
	case r := <-done:
		return r.out, r.err
	case <-callCtx.Done():
		in.stateMu.Lock()
		if in.callDone {
			in.stateMu.Unlock()
			r := <-done
			return r.out, r.err
		}
		in.abandoned.Store(true) // from now on the guest is refused, whatever happens next
		in.abandonedAt = time.Now()
		gave = in.track.abandon(callCtx, in.abandonedAt.Sub(started), expired(callCtx))
		in.stateMu.Unlock()
		return nil, stopError(callCtx)
	}
}

// Stretch reports the longest time the guest went without calling the host
// during the last call. It exists for the tests and the report.
func (in *instance) Stretch() time.Duration { return time.Duration(in.stretch.Load()) }

// Tail reports the part of the stretch after the last call of the guest to the
// host: the time from the last consult to the return of the guest.
func (in *instance) Tail() time.Duration { return time.Duration(in.tail.Load()) }

// First reports the time from the start of the last call to its first call of
// the guest to the host.
func (in *instance) First() time.Duration { return time.Duration(in.first.Load()) }

// AbandonedOvershoot waits for an abandoned call to end, or for ctx, and
// reports how long it kept running after the caller stopped waiting. The second result is false
// when the last call was not abandoned. It exists for the isolation tests and
// the measurement report.
func (in *instance) AbandonedOvershoot(ctx context.Context) (time.Duration, bool) {
	in.stateMu.Lock()
	ended, abandoned := in.ended, in.abandoned.Load()
	in.stateMu.Unlock()
	if ended == nil || !abandoned {
		return 0, false
	}
	select {
	case <-ended:
	case <-ctx.Done():
		// The call is still running: report how long it has run on.
		in.stateMu.Lock()
		defer in.stateMu.Unlock()
		return time.Since(in.abandonedAt), true
	}
	in.stateMu.Lock()
	defer in.stateMu.Unlock()
	return in.endedAt.Sub(in.abandonedAt), true
}

func (in *instance) callLocked(ctx context.Context, request []byte) ([]byte, error) {
	if uint32(len(request)) > in.reqCap { //nolint:gosec // requests are bounded by the memory cap.
		if in.reqPtr != 0 {
			_, _ = in.call(ctx, in.free, uint64(in.reqPtr))
		}
		ptr, err := in.put(ctx, make([]byte, len(request)*2))
		if err != nil {
			in.reqPtr, in.reqCap = 0, 0
			return nil, classify(err)
		}
		in.reqPtr, in.reqCap = ptr, uint32(len(request)*2) //nolint:gosec // see above.
	}
	if !in.mem.Write(in.reqPtr, request) {
		return nil, fmt.Errorf("%w: request does not fit guest memory", expr.ErrMemory)
	}
	res, err := in.call(ctx, in.eval, uint64(in.reqPtr), uint64(len(request)))
	if err != nil {
		return nil, classify(err)
	}
	text := in.output(ctx)
	switch code := api.DecodeI32(res); code {
	case 0:
		if ctx.Err() != nil && strings.HasPrefix(text, `{"x":`) {
			// The call ended in an exception after the context was done: the
			// stop is what the caller must see, not the error the stop caused.
			return nil, stopError(ctx)
		}
		return []byte(text), nil
	default:
		if strings.Contains(text, "interrupted") {
			if cause := context.Cause(ctx); cause != nil {
				return nil, stopError(ctx)
			}
			if st := stateOf(ctx); st != nil && st.deadlinePassed() {
				// The handler saw the deadline by the clock before the timer of
				// the context ran.
				return nil, fmt.Errorf("%w: %w", expr.ErrTimeout, context.DeadlineExceeded)
			}
		}
		return nil, classifyScript(text)
	}
}

// MemorySize reports the size of the WebAssembly linear memory of the
// instance in bytes.
func (in *instance) MemorySize() uint32 { return in.mem.Size() }

// MemoryUsage implements [expr.Runtime].
func (in *instance) MemoryUsage(ctx context.Context) int64 {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.broken {
		return 0
	}
	res, err := in.call(withCall(ctx, &callState{guest: noGuest{}, inst: in}), in.memUsed)
	if err != nil {
		return 0
	}
	return int64(res) //nolint:gosec // a byte count below 4 GiB.
}

// Close implements [expr.Runtime].
func (in *instance) Close(ctx context.Context) error {
	if in.abandoned.Load() {
		return nil // the abandoned call closes the runtime when it ends
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.broken = true
	return in.mod.Close(ctx)
}

// Whether the context ended because its deadline passed (expired), as opposed
// to being canceled.
func expired(ctx context.Context) bool { return errors.Is(ctx.Err(), context.DeadlineExceeded) }

// stopError is the error of a call stopped because its context is done: the
// time limit ([expr.ErrTimeout]) when the deadline passed, and the
// cancellation itself when the caller canceled, which is not an error of the
// expression and must not look like one.
func stopError(ctx context.Context) error {
	if expired(ctx) {
		return fmt.Errorf("%w: %w", expr.ErrTimeout, context.Cause(ctx))
	}
	return fmt.Errorf("expr: evaluation canceled: %w", context.Cause(ctx))
}

// classify maps a failure of the WebAssembly runtime to the sandbox errors.
func classify(err error) error {
	var exit *sys.ExitError
	switch {
	case errors.As(err, &exit):
		switch exit.ExitCode() {
		case sys.ExitCodeDeadlineExceeded:
			return fmt.Errorf("%w: %w", expr.ErrTimeout, err)
		case sys.ExitCodeContextCanceled:
			return fmt.Errorf("%w: %w", context.Canceled, err)
		}
		return fmt.Errorf("%w: %w", expr.ErrSandbox, err)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%w: %w", expr.ErrTimeout, err)
	case errors.Is(err, context.Canceled):
		return err
	case errors.Is(err, expr.ErrMemory), errors.Is(err, expr.ErrStack), errors.Is(err, expr.ErrTimeout):
		return err
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "stack overflow"):
		return fmt.Errorf("%w: %w", expr.ErrStack, err)
	case strings.Contains(msg, "out of memory"), strings.Contains(msg, "out of bounds memory"):
		return fmt.Errorf("%w: %w", expr.ErrMemory, err)
	}
	return fmt.Errorf("%w: %w", expr.ErrSandbox, err)
}

// classifyScript maps an exception that escaped the prelude to a sandbox error.
func classifyScript(text string) error {
	switch {
	case strings.Contains(text, "out of memory"):
		return fmt.Errorf("%w: %s", expr.ErrMemory, text)
	case strings.Contains(text, "stack overflow"):
		return fmt.Errorf("%w: %s", expr.ErrStack, text)
	}
	return fmt.Errorf("%w: %s", expr.ErrSandbox, text)
}
