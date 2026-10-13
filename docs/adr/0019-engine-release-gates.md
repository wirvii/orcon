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
This matrix is the single registry of fault-injection IDs. An ADR that adds a fault test records its row here and cites the ID; specs, plans and acceptance criteria cite IDs only from this table. A new ID takes the next number after the highest assigned one; gaps are not filled and a retired number is never reused. FI-19..FI-24 are retired (an earlier draft of the trigger rows used them; those rows are now FI-50..FI-57); FI-26..FI-29 and FI-38..FI-39 are unassigned.

| ID | Fault | Required outcome | Defined by |
|---|---|---|---|
| FI-01 | `kill -9` worker: after PREPARED commit, before DISPATCHING | item dispatched once after recovery | ADR-0019 |
| FI-02 | `kill -9` after DISPATCHING commit, before send | recovery per contract; no duplicate at the mock provider | ADR-0019 |
| FI-03 | `kill -9` after provider commit, before outcome commit | resend-with-key or reconcile → SUCCEEDED; no duplicate; non-reconcilable → UNCERTAIN | ADR-0019 |
| FI-04 | `kill -9` during fork / join transaction | all branches exist exactly once; exactly one continuation | ADR-0019 |
| FI-05 | `SIGSTOP` worker 60 s mid-token, then `SIGCONT` | stale worker fenced (no commits); work completed once by another worker | ADR-0019 |
| FI-06 | network cut worker↔provider mid-write (toxiproxy) | attempt timeout → UNCERTAIN or reconcile; never blind retry | ADR-0019 |
| FI-07 | network cut worker↔database | worker self-fences; leases expire; work resumes elsewhere | ADR-0019 |
| FI-08 | PostgreSQL kill/restart; PostgreSQL failover to synchronous replica | no lost accepted executions; progress resumes | ADR-0019 |
| FI-09 | lease-expiry race: two workers attempt commit for the same token | exactly one commit succeeds | ADR-0019 |
| FI-10 | worker wall-clock skew ±1 h | no early/late lease expiry (database clock) | ADR-0019 |
| FI-11 | duplicate delivery: duplicate NOTIFY + poll, duplicate signal webhook, duplicate child completion | no duplicate transitions | ADR-0019 |
| FI-12 | poison item crashing the worker process each time | token SUSPENDED after 3 abandons; other executions unaffected | ADR-0019 |
| FI-13 | one execution exceeds its memory cap among 1,000 concurrent | only it FAILED; 999 succeed | ADR-0019 |
| FI-14 | 1,000 concurrent executions, one panics / infinite-loops in an expression | only it fails; 999 succeed | ADR-0019 |
| FI-15 | SQLite: process kill during write; disk full | no corruption; accepted executions survive restart | ADR-0019 |
| FI-16 | crash between pure nodes with recorded nondeterminism | recomputed outputs identical to uninterrupted run | ADR-0019 |
| FI-17 | `kill -9` of the notification dispatcher after an attention request commits, before any send | every configured channel delivers after recovery with the same deduplication key; no request without notification | ADR-0073 §11 |
| FI-18 | email server and webhook receiver unreachable for 10 min | in-app notification unaffected; email and webhook delivered after recovery; executions unaffected; failures visible to admins | ADR-0073 §11 |
| FI-25 | 1,000 concurrent executions, one of them using a malicious WebAssembly connector module (memory bomb, infinite loop, deep recursion, oversize output, egress to an undeclared domain, private IP or metadata address, DNS rebinding) | only that execution fails, naming the limit it hit; the other 999 succeed without p99 increase beyond the gate margin | ADR-0032 §9 |
| FI-30 | `kill -9` during MK rotation | restart resumes; every OK unwraps; no credential unreadable | ADR-0050 §2 |
| FI-31 | `kill -9` during OK rotation re-wrap | resumes; old version retired only after all DEKs moved | ADR-0050 §2 |
| FI-32 | wrong MK at startup | process exits before any write; clear message | ADR-0050 §2 |
| FI-33 | OAuth refresh holder killed before provider call / after provider response | single refresh at a time; second case yields `needs_reauth` only if the provider rotated the token | ADR-0050 §2 |
| FI-34 | three processes need the same expired token concurrently | exactly one token-endpoint call (mock provider counts) | ADR-0050 §2 |
| FI-35 | audit sealer killed mid-seal | no gaps, no duplicates; verification passes | ADR-0050 §2 |
| FI-36 | direct DB update/delete/reorder of audit rows | verification detects and alerts within the next run | ADR-0050 §2 |
| FI-37 | external secret store unreachable | cached values within max-stale, then transient errors; no persistence | ADR-0050 §2 |
| FI-40 | `kill -9` after model response received, before its confirmation point | model re-queried once; cost recorded `POSSIBLY_BILLED`; run completes | ADR-0072 §1 |
| FI-41 | `kill -9` after the model-step confirmation point | model never re-queried (mock counts calls); same tool calls dispatched | ADR-0072 §1 |
| FI-42 | `kill -9` between effect-tool DISPATCHING and outcome | ADR-0012 recovery; no duplicate at the mock provider; non-reconcilable → run SUSPENDED | ADR-0072 §1 |
| FI-43 | `kill -9` during an approval wait; restart; approve | run resumes at the same step; no process held the run while waiting | ADR-0072 §1 |
| FI-44 | SIGSTOP worker 60 s during a model call | stale worker fenced; one recorded response | ADR-0072 §1 |
| FI-45 | worker crash mid-stream | client receives `stream.reset`; final answer recorded once | ADR-0072 §1 |
| FI-46 | `kill -9` during ingestion batch | no partial document version visible; no chunk embedded twice | ADR-0072 §1 |
| FI-47 | duplicate chat message delivery and concurrent messages in one conversation | one execution per message; FIFO order per conversation | ADR-0072 §1 |
| FI-48 | provider 429/529 storm | fallback used; circuit breaker shared; no budget overshoot beyond the documented bound | ADR-0072 §1 |
| FI-49 | budget exhausted by concurrent runs across workers | overshoot ≤ chunk × workers; later calls refused | ADR-0072 §1 |
| FI-50 | `kill -9` of an api process before / after the ingress commit | before: no 2xx sent and no execution; after: the event and its QUEUED execution exist and complete after restart; no 2xx ever answers an unpersisted event | ADR-0021 §6 |
| FI-51 | duplicate deliveries with the same deduplication key across three api processes | exactly one execution; duplicates answered with the original execution id and `Orcon-Duplicate: true` | ADR-0021 §6 |
| FI-52 | `kill -9` of the poll worker between `Poll` and commit | poll repeated with the same cursor; every source record in exactly one execution (no duplicates, no gaps); a fenced poller commits nothing | ADR-0026 §3 |
| FI-53 | `kill -9` of a listener holder with messages in flight | failover ≤ TTL + 1 s; no lost acknowledged-source message for `AT_LEAST_ONCE`/`CURSOR_EXACT`; one execution per message id/offset | ADR-0026 §3 |
| FI-54 | `SIGSTOP` the listener holder 60 s | stale holder fenced and never acks; one holder acking; same outcomes as FI-53 | ADR-0026 §3 |
| FI-55 | broker redelivery storm | one execution per message id/offset | ADR-0026 §3 |
| FI-56 | `kill -9` of the registration reconciler between `Register` and its commit; flow deactivated while a registration is in flight | after convergence: no provider registration for the deactivated flow, exactly one per address of active flows; an event on an unwanted registration gets `410 Gone` and enqueues a removal; the sweep removes a seeded orphan | ADR-0027 §6 |
| FI-57 | PostgreSQL: three scheduler processes with random `kill -9` while 1,000 schedules fire every second | exactly one `schedule_fire` row and at most one execution per slot; no slot within grace left unfired | ADR-0025 §5 |

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

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-002.** Adds the fault-injection cases FI-50..FI-57 of ADR-0021, ADR-0025, ADR-0026 and ADR-0027, registered as rows of the §3 matrix (the single registry of fault-injection ids); the n8n response-mode compatibility suite (ADR-0023 §3) runs next to the expression suite; benchmark scenario S2 uses both native and imported addresses.
- **SPEC-003.** New gates: the conformance kit over every official connector (ADR-0031 §5); the WASM escape suite (FI-25, ADR-0032 §9); TUF attacks (ADR-0033); unweighted coverage ≥ 90 % (ADR-0037 §4); the Code node measurement reports (ADR-0036); a single notification for an `UNSAFE` operation left uncertain after a crash.
- **SPEC-004.** New gates FI-30..FI-37 and the isolation suite (ADR-0050); §4: D20 is measured with the secure default configuration (RLS with several organizations, audit, encryption).
- **SPEC-006.** Adds the gates of ADR-0072: FI-40..FI-49, benchmark scenario S7, per-step overhead, retrieval, redaction and injection.
