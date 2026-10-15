// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package journal is the external operation journal of Orcon (ADR-0012, SPEC-001
// cut C6): the write-ahead record of every item an effect node sends to an
// external system, and the rules that decide what happens to an item whose
// outcome is not known.
//
// The life of an item is PREPARED, then DISPATCHING, then SUCCEEDED, FAILED or
// UNCERTAIN. The intent is persisted before anything leaves the process
// ([Journal.Prepare]); the DISPATCHING mark is committed in its own transaction
// before the send ([Journal.Dispatch]); the outcome is recorded afterwards in
// batches. No transaction is open while the [Provider] is called. A worker that
// dies with items in DISPATCHING leaves them for [Journal.Recover], which acts
// only after the dispatch deadline plus a grace period. An item whose outcome is
// unknown and that has no safe way to repeat it becomes UNCERTAIN and is never
// sent again by this package.
//
// The package is an area package: it depends on the store ports of
// internal/execution/store and on nothing of the adapters. Transactions are
// grouped: at most 1000 intents per statement group, 1000 DISPATCHING marks per
// transaction and 512 outcomes per transaction (E-EOSC-2).
package journal
