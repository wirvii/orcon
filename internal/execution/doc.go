// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package execution holds the flow execution engine: the closed state
// machines of ADR-0009, the deterministic identifiers of ADR-0008 and the
// token interpreter that applies the control-flow semantics of ADR-0010. It is
// a business-area package.
//
// The interpreter in this package is the in-memory one: [Compile] turns a
// validated [flow.Document] into a [Plan], and [Interpreter.Run] executes it
// against a few small ports declared in ports.go. It reads and writes no
// database, takes no leases and evaluates no expressions by itself; those
// arrive through the ports. The same flow and input always produce the same
// token trace.
//
// # Model
//
// A token is the unit of work. It carries a reference to an immutable batch of
// items, a scope path and a fork lineage, and it moves node by node until it
// ends, forks or is absorbed. For each edge and scope the interpreter delivers
// zero or more live tokens followed by exactly one completion signal; a node
// completes its outgoing edges once its incoming edges are complete and its
// tokens are processed, which eliminates dead paths. Joins (ALL, ACTIVATED,
// FIRST), loop control, item routing and sequential branch order build on
// those two mechanisms.
//
// # Extension points
//
// The durable driver (cut C4) replaces the in-memory run tables with
// transactional ones and reuses the transition tables (states.go), the
// deterministic ids (ids.go), the join rules (join.go) and the compiled plan
// (plan.go). The storage cut (C2) implements [Payloads]; the expression cut
// (C5) implements [ConditionEvaluator] and, with the node types, [ItemRouter];
// the journal cut (C6) wraps [NodeRunner].
package execution
