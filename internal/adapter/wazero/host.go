// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero

import (
	"context"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/wirvii/orcon/internal/expr"
)

// WASI error numbers used by the stubs.
const (
	errnoSuccess uint64 = 0
	errnoBadf    uint64 = 8
)

// callState travels in the context of a call so the host functions, which the
// runtime shares across instances, find the instance and guest they serve.
type callState struct {
	guest expr.Guest
	inst  *instance
	// err is the first failure of a host function; Call returns it.
	err error
}

type callKey struct{}

func withCall(ctx context.Context, st *callState) context.Context {
	return context.WithValue(ctx, callKey{}, st)
}

func stateOf(ctx context.Context) *callState {
	st, _ := ctx.Value(callKey{}).(*callState)
	return st
}

// noGuest serves the calls made while a runtime is being opened: scripts load
// with the clock frozen at the epoch and no data.
type noGuest struct{}

func (noGuest) HostCall(context.Context, int32, []byte) ([]byte, error) { return nil, nil }
func (noGuest) Now() time.Time                                          { return time.Unix(0, 0).UTC() }
func (noGuest) Read(p []byte)                                           { clear(p) }

// newModuleConfig returns the configuration of an instance: anonymous, no
// arguments, environment, files, standard streams, random source or clock.
func newModuleConfig() wazero.ModuleConfig { //nolint:ireturn // the wazero API returns its configuration as an interface
	return wazero.NewModuleConfig().WithName("").WithStartFunctions("_initialize")
}

// instantiateHostModules registers the two host modules the QuickJS module
// imports: orcon_v1, the data window of the expression names, and the few WASI
// functions of the C library.
func instantiateHostModules(ctx context.Context, rt wazero.Runtime) error {
	_, err := rt.NewHostModuleBuilder("orcon_v1").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(hostCall),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI64}).
		Export("host_call").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(interrupt), nil, []api.ValueType{api.ValueTypeI32}).
		Export("interrupt").
		Instantiate(ctx)
	if err != nil {
		return err
	}
	_, err = rt.NewHostModuleBuilder("wasi_snapshot_preview1").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(clockTimeGet),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI64, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32}).
		Export("clock_time_get").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(_ context.Context, _ api.Module, stack []uint64) {
			stack[0] = errnoSuccess
		}), []api.ValueType{api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).
		Export("fd_close").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(_ context.Context, _ api.Module, stack []uint64) {
			stack[0] = errnoBadf
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).
		Export("fd_fdstat_get").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(_ context.Context, _ api.Module, stack []uint64) {
			stack[0] = errnoBadf
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI64, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).
		Export("fd_seek").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(fdWrite),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32}).
		Export("fd_write").
		Instantiate(ctx)
	return err
}

// clockTimeGet answers every clock with the recorded activation time, so
// Date.now() and new Date() are constant during an activation (ADR-0018
// section 5). Arguments: clock id, precision, result pointer.
func clockTimeGet(ctx context.Context, mod api.Module, stack []uint64) {
	st := stateOf(ctx)
	now := time.Unix(0, 0)
	if st != nil {
		now = st.guest.Now()
	}
	if !mod.Memory().WriteUint64Le(api.DecodeU32(stack[2]), uint64(now.UnixNano())) {
		stack[0] = errnoBadf
		return
	}
	stack[0] = errnoSuccess
}

// fdWrite discards what the C library writes to a standard stream and reports
// it written, so nothing the code prints leaves the sandbox and nothing
// retries. Arguments: fd, iovs, iovs count, result pointer.
func fdWrite(_ context.Context, mod api.Module, stack []uint64) {
	iovs, count, resultPtr := api.DecodeU32(stack[1]), api.DecodeU32(stack[2]), api.DecodeU32(stack[3])
	var total uint32
	for i := range count {
		n, ok := mod.Memory().ReadUint32Le(iovs + i*8 + 4)
		if !ok {
			stack[0] = errnoBadf
			return
		}
		total += n
	}
	if !mod.Memory().WriteUint32Le(resultPtr, total) {
		stack[0] = errnoBadf
		return
	}
	stack[0] = errnoSuccess
}

// interrupt serves orcon_v1.interrupt() -> i32. QuickJS polls it while code
// runs; a non-zero answer stops the call. The call stops when its context is
// done, which includes its deadline.
func interrupt(ctx context.Context, _ api.Module, stack []uint64) {
	stack[0] = 0
	if st := stateOf(ctx); st != nil && st.inst.refuses(ctx) {
		stack[0] = 1
	}
}

// hostCall serves orcon_v1.host_call(op, ptr, len) -> (ptr << 32 | len).
func hostCall(ctx context.Context, mod api.Module, stack []uint64) {
	op, ptr, n := api.DecodeI32(stack[0]), api.DecodeU32(stack[1]), api.DecodeU32(stack[2])
	st := stateOf(ctx)
	stack[0] = 0
	if st == nil || st.inst.refuses(ctx) {
		return // an abandoned call is refused without reading execution data
	}
	payload, ok := mod.Memory().Read(ptr, n)
	if !ok {
		return
	}
	answer, err := st.guest.HostCall(ctx, op, append([]byte(nil), payload...))
	if err != nil {
		if st.err == nil {
			st.err = err
		}
		return
	}
	if len(answer) == 0 {
		return
	}
	res, err := st.inst.call(ctx, st.inst.alloc, uint64(len(answer)))
	if err != nil {
		if st.err == nil {
			st.err = classify(err)
		}
		return
	}
	dst := api.DecodeU32(res)
	if dst == 0 || !mod.Memory().Write(dst, answer) {
		return
	}
	stack[0] = uint64(dst)<<32 | uint64(len(answer))
}
