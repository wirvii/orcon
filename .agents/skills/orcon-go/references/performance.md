<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Performance

Status: Proposed — pending owner approval

This document defines how Orcon measures performance, how regressions are gated, the initial performance budgets and the reference hardware they are measured on. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`PERF-…`, `PB-…`) are stable. The release gate itself remains D20 (ADR 0019 §4); the budgets below are internal absolute targets.

## Benchmarks

- **PERF-1** Hot paths have Go benchmarks next to the code. The hot paths are: execution start, step transition, journal write, expression evaluation and webhook intake. New hot paths are added to this list by the change that creates them.
- **PERF-1.1** Benchmarks are named `Benchmark<Area><Operation>` (`BenchmarkExecutionStepTransition`, `BenchmarkTriggerWebhookIntake`), use `b.Loop()`, report allocations, and isolate the measured code from setup.
- **PERF-1.2** Micro benchmarks run with `-count=10 -benchtime=2s -benchmem` and `GOMAXPROCS=4`.
- **PERF-1.3** Benchmarks use fakes for storage and network unless the benchmark exists to measure them; macro measurements belong in the `bench/` harness.

## Regression gate

- **PERF-2** The `bench.yml` workflow runs on pull requests on GitHub runners. It runs the base and head benchmarks interleaved, ten times each, and compares them with benchstat through `tools/benchgate`.
- **PERF-2.1** The gate fails when `sec/op`, `B/op` or `allocs/op` gets worse by more than 10 % with p < 0.05.
- **PERF-2.2** A failing gate is overridden only when the pull request has the label `performance-regression-accepted` **and** a non-empty section `### Performance regression justification` in its description, explaining why the regression is acceptable.
- **PERF-2.3** `make bench-compare` runs the same comparison locally; a hot-path change includes its benchstat table in the pull request.
- **PERF-2.4** The gate never provisions the reference hardware.

## Budgets

The budgets apply from the first engine slices (SPEC-001 onward) and are inherited by them as acceptance criteria. Missing one requires an owner decision. They were set before any measurement and are revised only through the procedure in PERF-3.

| ID | Metric | SQLite standalone | PostgreSQL distributed |
|---|---|---|---|
| PB-1 | Webhook to execution start: last request byte received → start record (first step claimed, RUNNING) committed | p50 ≤ 5 ms, p99 ≤ 20 ms at 200 req/s | p50 ≤ 10 ms, p99 ≤ 40 ms at 1,000 req/s |
| PB-2 | Sustained executions per second of F-REF-3 (webhook with immediate response → Set with 3 fast-path expressions → Set), end-to-end p99 ≤ 250 ms, 0 errors over 300 s | ≥ 1,000 exec/s | ≥ 2,500 exec/s |
| PB-3 | Step transition: (a) micro `BenchmarkStepTransition` with a fake store; (b) macro linear flow of 50 no-op nodes | (a) ≤ 10 µs/op, ≤ 25 allocs/op, ≤ 4 KiB/op; (b) p50 ≤ 25 ms | (b) p50 ≤ 50 ms |
| PB-4 | Memory per idle execution: (a) 100,000 paused executions, Δ`heap_inuse` after `runtime.GC()`; (b) 10,000 in-flight executions blocked on I/O against a 10 s mock | (a) ≤ 1 KiB per execution; (b) ≤ 32 KiB per execution | same, per worker process |
| PB-5 | Resident memory (RSS) at rest with no flows | standalone ≤ 64 MiB | api ≤ 64 MiB, worker ≤ 48 MiB |

Rationale: n8n publishes about 220 executions per second per instance; D20 requires at least twice that; PB-2 targets four times to leave margin.

- **PERF-3** A budget changes only through a pull request that edits this table, includes measurements on ORCON-REF-1 from the method below, and is approved by the owner.

## Reference hardware

**ORCON-REF-1** is the fixed hardware profile required by ADR 0019 §4:

- Google Cloud, zone `us-central1-a`.
- Orcon node: `c3-standard-4` (4 vCPU Sapphire Rapids, 16 GiB), Hyperdisk Balanced 100 GiB (3,000 IOPS, 140 MiB/s), Ubuntu 24.04 LTS, Go = the toolchain in `go.mod`.
- PostgreSQL 17 on `c3-standard-8` (32 GiB, Hyperdisk Balanced 200 GiB, 6,000 IOPS), with `shared_buffers=8GB`, `synchronous_commit=on`, `max_connections=200`.
- Load generator on another `c3-standard-4` in the same zone.
- Distributed topology: VM1 runs `api` plus a `scheduler` process; VM2 and VM3 run `worker`.

Execution policy:

- **PERF-4** ORCON-REF-1 runs only on on-demand spot VMs, only for release candidates and engine slices; VMs are torn down after every run; a run interrupted by spot preemption is discarded and repeated; partial results are never published.
- **PERF-4.1** "Engine slices" are slices of SPEC-001 and later that change `internal/execution`, `internal/journal`, `internal/trigger` or the storage adapters. Ordinary pull requests use only the regression gate (PERF-2), never ORCON-REF-1, and no permanent machines are kept.
- **PERF-4.2** Provisioning and teardown are done by a script under `bench/`; teardown runs even when the measurement fails.
- **PERF-4.3** The cost of each run is recorded in `docs/benchmarks/<version>.md`.

## Method

- **PERF-5** Macro measurements use the `bench/` harness with open arrivals at a constant rate (no coordinated omission).
- **PERF-5.1** Latencies are measured on the server with a monotonic clock, through the OpenTelemetry histogram `orcon.ingress.to_start` and HdrHistogram-go.
- **PERF-5.2** Each run has 60 s of warm-up and 300 s of measurement; at least five runs; the report gives the median of p50, p99 and throughput with the minimum and maximum.
- **PERF-5.3** SQLite runs use `synchronous=FULL`.
- **PERF-5.4** Results are published in `docs/benchmarks/<version>.md` together with the commit, toolchain, configuration and cost.
