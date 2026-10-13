<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0072 — AI release gates: fault injection, overhead budgets, retrieval, security

Status: **Accepted** | Date: 2026-10-11

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
Principle 12 and ADR-0019; A1 durability must not make Orcon slower than necessary; A3 retrieval must work on both stores; A5 security must be demonstrated.

## Decision

### 1. Fault-injection matrix (real processes, SQLite and PostgreSQL, `mockllm`)
| ID | Fault | Required outcome |
|---|---|---|
| FI-40 | `kill -9` after model response received, before its confirmation point | model re-queried once; cost recorded `POSSIBLY_BILLED`; run completes |
| FI-41 | `kill -9` after the model-step confirmation point | model never re-queried (mock counts calls); same tool calls dispatched |
| FI-42 | `kill -9` between effect-tool DISPATCHING and outcome | ADR-0012 recovery; no duplicate at the mock provider; non-reconcilable → run SUSPENDED |
| FI-43 | `kill -9` during an approval wait; restart; approve | run resumes at the same step; no process held the run while waiting |
| FI-44 | SIGSTOP worker 60 s during a model call | stale worker fenced; one recorded response |
| FI-45 | worker crash mid-stream | client receives `stream.reset`; final answer recorded once |
| FI-46 | `kill -9` during ingestion batch | no partial document version visible; no chunk embedded twice |
| FI-47 | duplicate chat message delivery and concurrent messages in one conversation | one execution per message; FIFO order per conversation |
| FI-48 | provider 429/529 storm | fallback used; circuit breaker shared; no budget overshoot beyond the documented bound |
| FI-49 | budget exhausted by concurrent runs across workers | overshoot ≤ chunk × workers; later calls refused |

### 2. Overhead budgets (mock provider with zero latency)
Per model-step confirmation point: SQLite p50 ≤ 2 ms, p99 ≤ 10 ms; PostgreSQL p50 ≤ 4 ms, p99 ≤ 20 ms. Run start ≤ the same budget. Redaction ≤ 1 ms per 10 KiB of prompt with 1,000 sensitive values. Added time-to-first-delta ≤ 10 ms p95 standalone, ≤ 50 ms p95 distributed.

### 3. Scenario S7 (added to ADR-0019 benchmark)
Chat agent: mock provider 200 ms per call, 3 steps, 2 read tools (HTTP mock 20 ms), window memory, 1,000 concurrent conversations; Orcon standalone vs n8n regular and Orcon distributed vs n8n queue mode with the same mock. Gate (owner decision, SPEC-006 Q5): Orcon throughput ≥ n8n, p99 ≤ n8n, peak memory ≤ 50 % of n8n; the 2× target of ADR-0019 does not apply to S7.

### 4. Retrieval
`bench/retrieval`: hybrid query p95 ≤ 50 ms for 100,000 chunks × 768 dims on SQLite (vector cache warm) and for 1,000,000 chunks × 768 dims on PostgreSQL/pgvector with organization and store filters at 1/100 organizations; HNSW recall@10 ≥ 0.95 against exact search; on a public permissively licensed QA set, hybrid nDCG@10 ≥ max(lexical-only, vector-only) on both stores; ingestion ≥ 2,000 chunks/s on SQLite excluding embedding time.

### 5. Security gates
Injection suite: instructions planted in documents, emails, web pages, MCP outputs and knowledge chunks requesting sensitive tools never dispatch without approval; the mock provider asserts no sensitive value is ever received with redaction on; re-hydration only at dispatch; cross-organization isolation for vectors, conversations, vault, caches, budgets and evaluations (ADR-0050 suite) including RLS-only mode; chat sessions not guessable; approvals by API key principals rejected; public-input floor (ADR-0068 §5): no configuration lets a sensitive tool dispatch without `ORCON`/`LINK` approval in a run with public input; pending approvals emit the ADR-0073 events immediately, escalate after the configured delay and resolve on expiry.

### 6. CI placement
Per pull request: unit, conformance with `mockllm`, isolation cases. Nightly: FI-40..FI-49, `bench/ai`, `bench/retrieval`. Release: all gates.

## Consequences
AI durability, cost control, retrieval quality and security are demonstrated with published evidence before any production claim.

## Alternatives considered
Gating only on end-to-end latency with real providers (rejected: non-reproducible, dominated by provider variance).
