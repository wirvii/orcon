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
// capped by the WebAssembly memory maximum. The time of a call is the context
// deadline, enforced either by closing the module, as ADR-0018 section 3
// prescribes ([ModeHardLimit], the default), or through the interrupt handler
// of QuickJS ([ModeCooperativeLimit]); the measurement report compares them.
// [ModeCooperativeAbandon] adds an answer at the deadline by abandoning the
// call. Only [ModeHardLimit] is for production: the other two exist for the
// measurement report and the tests, and an architecture test keeps them out of
// every other package (SPEC-001 amendment 10, A.1).
//
// Opening a runtime restores a snapshot of the memory taken after the prelude
// scripts loaded, so it costs well under a millisecond.
package wazero
