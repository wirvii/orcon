// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package bench is the measurement harness of the expression engine
// (ADR-0018 section 4, SPEC-001 criteria E-EXPR-2 and E-EXPR-3). It is a
// business-area package that depends on internal/expr only through its public
// types: the sandbox it measures is injected, so the harness runs unchanged
// against any [expr.Sandbox], and the adapter tests plug in the real one.
//
// It provides:
//
//   - the compatibility measurement: [Compat] checks every expression of a
//     corpus extracted from the public n8n template library and reports the
//     share that runs unchanged, weighted and not weighted by template usage,
//     with the reasons of the rest;
//   - the latency, instance-cost and memory measurements of the fast path and
//     the sandbox ([Measure], [Cases]);
//   - [Report], which renders the measurement report and the verdict on the
//     decision of ADR-0018 section 3 (REPORT.md in this directory).
//
// The corpus itself is never committed: only its digest, counts and scores
// are (SPEC-001 decision P8).
package bench
