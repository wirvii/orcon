<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0063 — Model output streaming: ephemeral token channel, durable final response

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: `ai_stream_chunk` carries `org_id`, ADR-0077)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
A2 requires streaming for chat and editor testing. ADR-0023 persists response stream chunks at confirmation points, appropriate for respond-node streaming but too heavy for token-level deltas (tens per second per conversation).

## Decision
1. Token deltas are **not** durable state. Standalone/SQLite: in-process channel from the worker goroutine to the HTTP handler. Distributed/PostgreSQL: `ai_stream_chunk(org_id, id, stream_id, sequence_number, chunk_data, created_at)` is an `UNLOGGED` organization table with forced RLS like every other (`stream_id` is an ephemeral identifier listed in `external_ids.txt`); the worker appends coalesced deltas every ≤ 50 ms or ≤ 4 KiB in autocommit inserts within its organization context and issues `pg_notify('orcon_stream', stream_id)`; the api process holding the client connection reads by `sequence_number` within the same organization context and polls every 100 ms as fallback; rows are pruned 10 minutes after stream end by an `orcon_engine` function run by the scheduler.
2. The durable truth is the recorded response (ADR-0060 §3). If a worker dies mid-stream, the api sends `stream.reset {reason}` and the step is re-issued (ADR-0060 §4); clients replace partial text.
3. Only final-answer text is streamed to end users; tool-call deltas and reasoning are never streamed to them; editor test runs receive live-only `ai.delta` execution events (ADR-0055 §3).
4. Every delta passes through the masking scan (SPEC-004 R-CRED-04) and the reveal rule (ADR-0068 §4) with a placeholder-boundary buffer so a placeholder is never split.
5. Latency budget: added time-to-first-delta ≤ 10 ms p95 standalone, ≤ 50 ms p95 distributed (ADR-0072).

## Consequences
Streaming adds no WAL pressure; a crash costs at most a restarted stream; streamed text never crosses organizations even on the shared table.

## Alternatives considered
Persisting deltas in the checkpoint (rejected: write amplification); direct worker→api RPC (rejected in ADR-0023); sticky routing (rejected); an instance-level chunk table without `org_id` (rejected by ADR-0077: organization data always carries `org_id`).
