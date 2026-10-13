// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package wazero is an adapter: it implements the expression sandbox port
// (internal/expr) with QuickJS compiled to WebAssembly and executed by the
// wazero runtime (ADR-0018 section 3). It is the only package that imports
// wazero (ADR-0076).
//
// The module in assets/quickjs.wasm is built by assets/build.sh from the
// pinned QuickJS release and orcon_qjs.c. It has no filesystem, network or
// clock access: its only imports are the host module orcon_v1 (data access
// for the expression names and recorded randomness, and a poll that asks
// whether the call must stop) and five WASI functions that this package
// answers itself, the clock from the recorded activation time. Memory is
// capped by the WebAssembly memory maximum.
//
// The time of a call is the context deadline. In production
// ([ModeCooperativeAbandon], the default of [New]) the QuickJS interrupt
// handler checks it, also inside the native built-ins that an interrupt-poll
// patch covers, and every call runs on its own goroutine: when the deadline
// passes the caller stops waiting and gets its timeout within the deadline plus
// 5 ms, the abandoned call refuses further requests to the host and closes its
// runtime when it ends, and the process keeps a bounded number of abandoned
// calls (SPEC-001 amendment 10). A saturated process or one holding a stuck
// call reports it through [Sandbox.Degraded] so the composition root can shut
// down for a restart. [ModeHardLimit] (close the module when the context is
// done, wazero's close-on-context-done) and [ModeCooperativeLimit] (the
// cooperative stop without the answer at the deadline) exist for the
// measurement report and the tests; an architecture test keeps them out of
// every other package.
//
// Opening a runtime restores a snapshot of the memory taken after the prelude
// scripts loaded, so it costs well under a millisecond.
package wazero
