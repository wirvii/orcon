// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package conformance holds the conformance suite of the execution engine
// (ADR-0019 sections 1 and 2): one test per Workflow Control-Flow Pattern
// (WCP 1-43), each marked supported or unsupported with its reason, and the
// tests of the Orcon control-flow semantics that go beyond the patterns:
// completion signals and dead path elimination, the three join modes, item
// routing, sequential branch order and loop limits.
//
// The package has only tests; this file carries its package comment. The suite
// runs against the in-memory interpreter (cut C3). The -store flag selects the
// engine under test; the durable stores join it when the storage cuts land
// and the same flows run unchanged on SQLite and PostgreSQL. A store other
// than "memory" skips the suite with that reason until then.
//
// The pattern matrix lives in patterns_test.go. Patterns whose elements belong
// to a later cut are listed there with the cut that provides them.
package conformance
