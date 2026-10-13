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
	in := &instance{mod: mod, mem: mod.Memory()}
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
	in := &instance{mod: mod, mem: mod.Memory()}
	if err := in.bind(); err != nil {
		_ = mod.Close(ctx)
		return nil, err
	}
	// The call runs under a state whose guest is a no-op: loading scripts
	// needs the clock and nothing else.
	loadCtx := withCall(ctx, &callState{guest: noGuest{}, inst: in})
	jsLimit := memoryLimit - reserve
	var flags int32
	if e.mode == ModeCooperativeLimit {
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
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.broken {
		return nil, fmt.Errorf("%w: the runtime failed earlier and must be reopened", expr.ErrSandbox)
	}
	st := &callState{guest: guest, inst: in}
	out, err := in.callLocked(withCall(ctx, st), request)
	if err == nil {
		err = st.err
	}
	if err != nil {
		in.broken = true
		return nil, err
	}
	return out, nil
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
		return []byte(text), nil
	default:
		if cause := context.Cause(ctx); cause != nil && strings.Contains(text, "interrupted") {
			return nil, fmt.Errorf("%w: %w", expr.ErrTimeout, cause)
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
	in.mu.Lock()
	defer in.mu.Unlock()
	in.broken = true
	return in.mod.Close(ctx)
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
