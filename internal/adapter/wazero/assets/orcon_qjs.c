/*
 * SPDX-FileCopyrightText: The Orcon Authors
 * SPDX-License-Identifier: Apache-2.0
 *
 * Orcon expression runtime: a reactor-model WebAssembly module that hosts one
 * QuickJS runtime (ADR-0018 section 3). The module has no filesystem, network
 * or clock access of its own: its only imports are the host module orcon_v1
 * (data access and recorded randomness, declared below) and the few WASI
 * functions the C library needs, which the host implements as stubs or from
 * recorded values.
 *
 * Exports:
 *   orcon_alloc / orcon_free        memory management for the host
 *   orcon_init                      creates the runtime and context
 *   orcon_load                      evaluates a script in the global scope
 *   orcon_bind                      looks up the prelude entry point
 *   orcon_eval                      calls __orcon_run(request)
 *   orcon_out_ptr / orcon_out_len   the last result or error text
 *   orcon_memory_used               bytes the QuickJS heap currently holds
 *
 * Built by build.sh against the pinned QuickJS release; see README.md.
 */
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "quickjs.h"

/*
 * host_call asks the host for data. op selects the operation, in/in_len is a
 * UTF-8 payload. The host answers with a buffer it allocated through
 * orcon_alloc, packed as (ptr << 32) | len, or 0 for "no data".
 */
__attribute__((import_module("orcon_v1"), import_name("host_call")))
extern int64_t orcon_host_call(int32_t op, const char *in, int32_t in_len);

/*
 * interrupt asks the host whether the running call must stop (its deadline
 * passed or its context was canceled). QuickJS polls the handler every few
 * thousand bytecode operations, in loops, calls and long built-ins, so a
 * runaway expression stops within about a millisecond of the deadline without
 * the cost of hard preemption (see ADR-0018 section 3 and the measurement
 * report).
 */
__attribute__((import_module("orcon_v1"), import_name("interrupt")))
extern int32_t orcon_host_interrupt(void);

static JSRuntime *rt;
static JSContext *ctx;
static JSValue run_fn;
static char *out_buf;
static size_t out_len;

__attribute__((export_name("orcon_alloc")))
void *orcon_alloc(size_t size) { return malloc(size ? size : 1); }

__attribute__((export_name("orcon_free")))
void orcon_free(void *ptr) { free(ptr); }

static void set_out(const char *s, size_t n) {
    free(out_buf);
    out_buf = malloc(n + 1);
    if (!out_buf) {
        out_len = 0;
        return;
    }
    memcpy(out_buf, s, n);
    out_buf[n] = 0;
    out_len = n;
}

/* __host(op, payload) -> string | undefined */
static JSValue js_host(JSContext *c, JSValueConst this_val, int argc, JSValueConst *argv) {
    int32_t op = 0;
    size_t len = 0;
    const char *payload = "";
    if (argc > 0 && JS_ToInt32(c, &op, argv[0]) < 0) {
        return JS_EXCEPTION;
    }
    if (argc > 1) {
        payload = JS_ToCStringLen2(c, &len, argv[1], 0);
        if (!payload) {
            return JS_EXCEPTION;
        }
    }
    int64_t packed = orcon_host_call(op, payload, (int32_t)len);
    if (argc > 1) {
        JS_FreeCString(c, payload);
    }
    if (packed == 0) {
        return JS_UNDEFINED;
    }
    char *ptr = (char *)(uintptr_t)(uint32_t)((uint64_t)packed >> 32);
    uint32_t n = (uint32_t)(packed & 0xffffffffu);
    JSValue s = JS_NewStringLen(c, ptr, n);
    free(ptr);
    return s;
}

static int interrupt_cb(JSRuntime *r, void *opaque) {
    (void)r;
    (void)opaque;
    return orcon_host_interrupt();
}

/* flags: bit 0 installs the cooperative interrupt handler. */
__attribute__((export_name("orcon_init")))
int32_t orcon_init(int32_t memory_limit, int32_t stack_size, int32_t flags) {
    if (rt) {
        return -1;
    }
    rt = JS_NewRuntime();
    if (!rt) {
        return -2;
    }
    if (memory_limit > 0) {
        JS_SetMemoryLimit(rt, (size_t)memory_limit);
    }
    if (stack_size > 0) {
        JS_SetMaxStackSize(rt, (size_t)stack_size);
    }
    if (flags & 1) {
        JS_SetInterruptHandler(rt, interrupt_cb, NULL);
    }
    ctx = JS_NewContextRaw(rt);
    if (!ctx) {
        return -3;
    }
    /* Only the intrinsics an expression may use. No std, os or module loader. */
    JS_AddIntrinsicBaseObjects(ctx);
    JS_AddIntrinsicDate(ctx);
    JS_AddIntrinsicEval(ctx);
    JS_AddIntrinsicRegExpCompiler(ctx);
    JS_AddIntrinsicRegExp(ctx);
    JS_AddIntrinsicJSON(ctx);
    JS_AddIntrinsicProxy(ctx);
    JS_AddIntrinsicMapSet(ctx);
    JS_AddIntrinsicTypedArrays(ctx);
    JS_AddIntrinsicBigInt(ctx);
    JS_AddIntrinsicAToB(ctx);

    JSValue global = JS_GetGlobalObject(ctx);
    JS_SetPropertyStr(ctx, global, "__host", JS_NewCFunction(ctx, js_host, "__host", 2));
    JS_FreeValue(ctx, global);
    run_fn = JS_UNDEFINED;
    return 0;
}

/* orcon_load evaluates src in the global scope. It returns 0, or 1 with the error in the output buffer. */
__attribute__((export_name("orcon_load")))
int32_t orcon_load(const char *src, size_t len, const char *name) {
    JSValue v = JS_Eval(ctx, src, len, name, JS_EVAL_TYPE_GLOBAL);
    if (JS_IsException(v)) {
        JSValue ex = JS_GetException(ctx);
        size_t n;
        const char *s = JS_ToCStringLen2(ctx, &n, ex, 0);
        if (s) {
            set_out(s, n);
            JS_FreeCString(ctx, s);
        }
        JS_FreeValue(ctx, ex);
        return 1;
    }
    JS_FreeValue(ctx, v);
    return 0;
}

/* orcon_bind looks up the prelude entry point after the prelude was loaded. */
__attribute__((export_name("orcon_bind")))
int32_t orcon_bind(void) {
    JSValue global = JS_GetGlobalObject(ctx);
    JSValue fn = JS_GetPropertyStr(ctx, global, "__orcon_run");
    JS_FreeValue(ctx, global);
    if (!JS_IsFunction(ctx, fn)) {
        JS_FreeValue(ctx, fn);
        return 1;
    }
    run_fn = fn;
    return 0;
}

/* orcon_eval calls __orcon_run(request) and leaves the returned string in the output buffer. */
__attribute__((export_name("orcon_eval")))
int32_t orcon_eval(const char *req, size_t len) {
    JSValue arg = JS_NewStringLen(ctx, req, len);
    JSValue res = JS_Call(ctx, run_fn, JS_UNDEFINED, 1, &arg);
    JS_FreeValue(ctx, arg);
    int32_t code = 0;
    if (JS_IsException(res)) {
        JSValue ex = JS_GetException(ctx);
        res = JS_ToString(ctx, ex);
        JS_FreeValue(ctx, ex);
        code = 1;
        if (JS_IsException(res)) {
            JSValue ex2 = JS_GetException(ctx);
            JS_FreeValue(ctx, ex2);
            set_out("internal error", 14);
            return 2;
        }
    }
    size_t n;
    const char *s = JS_ToCStringLen2(ctx, &n, res, 0);
    if (s) {
        set_out(s, n);
        JS_FreeCString(ctx, s);
    } else {
        JSValue ex = JS_GetException(ctx);
        JS_FreeValue(ctx, ex);
        set_out("result is not a string", 22);
        code = 2;
    }
    JS_FreeValue(ctx, res);
    return code;
}

__attribute__((export_name("orcon_out_ptr")))
const char *orcon_out_ptr(void) { return out_buf; }

__attribute__((export_name("orcon_out_len")))
size_t orcon_out_len(void) { return out_len; }

__attribute__((export_name("orcon_memory_used")))
int64_t orcon_memory_used(void) {
    JSMemoryUsage u;
    JS_ComputeMemoryUsage(rt, &u);
    return u.malloc_size;
}
