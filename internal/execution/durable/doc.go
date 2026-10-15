// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package durable is the durable driver of the execution area (SPEC-001 C4a,
// area package, architecture-layers.md AL-1.1): the worker loop that claims
// tokens under a lease, runs them through the interpreter's session and commits
// each step to the store in one fenced transaction, plus the recovery that
// keeps accepted work from being lost (ADR-0013).
//
// A worker never holds a database transaction while it calls a port. It opens an
// execution from its rows, runs the leased token node by node in memory, and
// commits at a confirmation point: a fork, the end of the token, the budget of
// activations, or a release. The commit locks the execution, checks the fence of
// the token (epoch, owner, status LEASED), and writes the result, the advance of
// the token, the new tokens, the events, the batches and the tracker document
// together or not at all. When another worker committed to the same execution
// since the step was computed, the commit recomputes the step on the newer state
// with the activations the worker already made (bounded recomputation), instead
// of overwriting it.
//
// The package declares the store ports it consumes through the store package
// and the document reader it needs; the composition root wires the adapters.
package durable
