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
// for the expression names and recorded randomness) and five WASI functions
// that this package answers itself, the clock from the recorded activation
// time. Memory is capped by the WebAssembly memory maximum and the time of a
// call by the context deadline, enforced by closing the module.
package wazero
