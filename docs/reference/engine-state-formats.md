<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Stored formats of the durable engine

Status: reference for SPEC-001 cut C4a. The normative sources are ADR-0005, ADR-0009,
ADR-0012, ADR-0013, ADR-0014, ADR-0015 and ADR-0019; this page only names the
documents the engine writes so that a reader, a migration or a diagnostic tool can
find them.

All documents are canonical JSON written by `internal/execution` and carried by the
storage ports as opaque bytes. Readers ignore members they do not know within the same
major version; a document with a different major version is refused, never guessed.

## `orcon/engine-state@1`

One document per execution, stored with the execution row and compared and replaced by
revision (compare-and-set). It holds the trackers that cannot be rebuilt from the live
token rows: join arrivals and rounds, loop instances, merge returns, edge liveness,
groups, and the order in which tokens became runnable. Counters (tokens per state, live
tokens) are not stored in it; they are rebuilt from the token rows on every session.

## `orcon/fork-lineage@1`

One document per token, stored with the token row: the forks the token descends from
(split node execution, split node, branch), and the flags that say whether it already
passed a join or was merged. It lets a session rebuild the scope of a token without
reading its siblings.

## `orcon/items@1`

The payload of a batch: an array of items, each with its JSON value (numbers keep their
text) and its lineage references (port and index). Payloads are immutable, named by a
deterministic reference, and written before the commit that mentions them.

## Nondeterminism record

For every activation the engine records the environment it gave the node (the time and
the random seed) with the step that commits the activation. A worker that takes the
token again after a crash, or a commit that has to recompute a step on a newer state of
the execution, reads those records and gives the node the same environment, so a
recomputed step is the same step. Records are kept per token, ordered by sequence.

## Deterministic identifiers

Identifiers of rows created by the engine (tokens, node executions, events, payload
references, attempts) are `id.Derive(fields...)`: the first 128 bits of the SHA-256 of
the length-prefixed fields. The PostgreSQL function `orcon_engine.derive_uuid` computes
the same value. Two workers that run the same step therefore write the same rows, and a
repeated write is a no-op.

## Upgraded-document cache

The engine keeps parsed and upgraded flow documents in memory, keyed by the content
hash of the pinned revision and the format version of the reader, bounded by entries and
by bytes. The stored document never changes; a miss parses and upgrades once, and
concurrent misses of the same key share one load.
