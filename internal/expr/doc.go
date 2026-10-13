// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package expr evaluates the expressions of a flow: the {{ }} segments of a
// parameter value that starts with "=" (ADR-0018). It is a business-area
// package.
//
// An expression is JavaScript that sees the n8n-compatible names ($json,
// $input, $('Node'), $now and the rest of ADR-0018 section 1). Evaluation has
// two engines behind one [Evaluator]:
//
//   - the fast path, a Go evaluator for the simple subset (literals, member
//     access on the item data, operators, ternaries), classified once when an
//     expression is compiled and kept in step with the interpreter by
//     differential tests;
//   - the sandbox, a [Sandbox] port implemented by internal/adapter/wazero,
//     which runs everything else in QuickJS compiled to WebAssembly with
//     hard memory and time limits and no access to the host.
//
// The interpreter calls [Evaluator.Compile] when a flow version is published
// and [Session.Eval] per item. Everything non-deterministic an expression can
// observe ($now, $today, Date.now, Math.random, crypto.randomUUID) comes from
// the recorded [Activation] of the node (ADR-0018 section 5).
//
// [Analyze] parses an expression without running it and lists the names it
// uses; the publication rule V13 (ADR-0010 section 11) and the n8n importer
// (ADR-0018 section 7) read their facts from it.
package expr
