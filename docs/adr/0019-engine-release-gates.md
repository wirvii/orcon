<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0019 — Engine release gates: conformance, fault injection and benchmark

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
Principles 2 and 12, D7 (Workflow Patterns suite), D15 (mandatory crash tests as a release gate), D19 (same suite on both stores), D20 (benchmark gate), D21 (round-trip), D22 (expression compatibility).

## Decision

### 1. Workflow Patterns conformance
`internal/execution/conformance` contains one test per Workflow Control-Flow Pattern (WCP 1–43, Russell/van der Aalst et al.), each marked `supported` or `unsupported(reason)`; the matrix is published in `docs/reference/workflow-patterns.md`. Minimum supported set required for release: WCP 1–9, 10, 11, 12–14, 16, 19, 20, 21, 22, 24, 25, 26, 28, 29, 33, 34, 35, 37, 41, 42, 43. Every supported pattern passes on SQLite and PostgreSQL.

### 2. Orcon semantic conformance
Additional tests for completion signals/DPE, the three join modes, item routing, sequential branch mode, loop limits and carried state, multi-instance ordering, sub-flow modes and cycle detection, waits and boundary-event races, retries/outcome routing, cancellation summaries and compensation — on both stores.

### 3. Fault-injection matrix (real processes; both stores unless noted)
| ID | Fault | Required outcome |
|---|---|---|
| FI-01 | `kill -9` worker: after PREPARED commit, before DISPATCHING | item dispatched once after recovery |
| FI-02 | `kill -9` after DISPATCHING commit, before send | recovery per contract; no duplicate at the mock provider |
| FI-03 | `kill -9` after provider commit, before outcome commit | resend-with-key or reconcile → SUCCEEDED; no duplicate; non-reconcilable → UNCERTAIN |
| FI-04 | `kill -9` during fork / join transaction | all branches exist exactly once; exactly one continuation |
| FI-05 | `SIGSTOP` worker 60 s mid-token, then `SIGCONT` | stale worker fenced (no commits); work completed once by another worker |
| FI-06 | network cut worker↔provider mid-write (toxiproxy) | attempt timeout → UNCERTAIN or reconcile; never blind retry |
| FI-07 | network cut worker↔database | worker self-fences; leases expire; work resumes elsewhere |
| FI-08 | PostgreSQL kill/restart; PostgreSQL failover to synchronous replica | no lost accepted executions; progress resumes |
| FI-09 | lease-expiry race: two workers attempt commit for the same token | exactly one commit succeeds |
| FI-10 | worker wall-clock skew ±1 h | no early/late lease expiry (database clock) |
| FI-11 | duplicate delivery: duplicate NOTIFY + poll, duplicate signal webhook, duplicate child completion | no duplicate transitions |
| FI-12 | poison item crashing the worker process each time | token SUSPENDED after 3 abandons; other executions unaffected |
| FI-13 | one execution exceeds its memory cap among 1,000 concurrent | only it FAILED; 999 succeed |
| FI-14 | 1,000 concurrent executions, one panics / infinite-loops in an expression | only it fails; 999 succeed |
| FI-15 | SQLite: process kill during write; disk full | no corruption; accepted executions survive restart |
| FI-16 | crash between pure nodes with recorded nondeterminism | recomputed outputs identical to uninterrupted run |
| FI-17 | `kill -9` of the notification dispatcher after an attention request commits, before any send | every configured channel delivers after recovery with the same deduplication key; no request without notification |
| FI-18 | email server and webhook receiver unreachable for 10 min | in-app notification unaffected; email and webhook delivered after recovery; executions unaffected; failures visible to admins |

### 4. Benchmark gate (D20)
- Six fixed scenarios, identical flows in n8n and Orcon: **S1** 10,000-item batch (transform → HTTP write to mock provider, 20 ms latency → transform); **S2** high concurrent webhook rate (open-loop arrivals, 3-node flow; max sustainable throughput and p99 at fixed rate); **S3** fan-out/fan-in (8 parallel HTTP reads, 50 ms, merged; 1,000 executions); **S4** long waits (100,000 executions paused, then resumed; memory while paused, resume throughput); **S5** large payloads (50 MB binary file and 5 MB JSON arrays through 4 nodes); **S6** crash recovery (S1 + S3 load with periodic `kill -9` of workers).
- n8n at a pinned version (the latest stable release when the harness is pinned for an Orcon release; re-pinned explicitly per Orcon release) in its best recommended production configuration, regular mode and queue mode; Orcon standalone vs n8n regular, Orcon distributed vs n8n queue mode, same CPU/memory limits and same PostgreSQL.
- Gate per scenario: Orcon throughput ≥ 2× n8n; Orcon p99 latency ≤ 50 % of n8n; Orcon peak memory ≤ 50 % of n8n (application processes; database excluded on both sides; Redis included for n8n queue mode); S6: zero lost executions and zero duplicate external writes (counted by the mock provider by business key); ≤ 5 % regression vs the previous Orcon release on throughput, p99 and memory.
- Method: pinned hardware profile, ≥ 5 runs, median reported with dispersion; scripts in `bench/`, public and reproducible; results published per release in `docs/benchmarks/<version>.md`.

### 5. Other gates
BPMN round-trip on every conformance flow plus property-generated flows; expression compatibility ≥ 95 %; fast-path parity 100 %; determinism (FI-16); human-attention notification latency, redaction and durability (ADR-0073 §11); compatibility-floor verification (ADR-0015 §4); database rules (ADR-0077 §1: `tools/dbcheck` and `dbrules` with zero findings on SQLite and PostgreSQL 14 and 17, connection settings, hot-query plans and upgrades from a seeded previous version).

### 6. CI placement
Per pull request: unit, storetest, database rules and engine conformance on SQLite and PostgreSQL (testcontainers). Nightly: full fault matrix and benchmark trend. Release: all gates; no release on any failure.

## Consequences
Reliability and performance claims are backed by published evidence (principle 12); no production-readiness claim without these gates.

## Alternatives considered
Synthetic micro-benchmarks only (rejected: not comparable); gating only on throughput (rejected: D20 requires latency, memory, safety and regression).
