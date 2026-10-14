<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0026 — Polling and persistent-connection triggers

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-002.

## Context
T4: polling cursor advance and execution creation committed together (no duplicates, no gaps). T6: persistent-connection triggers (queues/streams, IMAP IDLE, SSE…) have exactly one holder across servers using the SPEC-001 lease and fencing mechanism, fail over automatically within seconds, ack to the source only after durable persistence, deduplicate by message id, and are distributed across servers.

## Decision

### 1. Polling
- The scheduler marks polls due; a worker claims the `poll_state` row with a lease and epoch (ADR-0013 §4 semantics, TTL 15 s) through an engine-plane function (ADR-0047).
- The worker calls `Poll(cursor)` outside any transaction, then commits in one transaction: fenced check (`lease_epoch`, `lease_owner`), compare-and-set `cursor_version`, new cursor and seen-id set, and `AcceptEvents` (one execution per poll result batch, matching n8n; per-item splitting is a node option).
- A crash between `Poll` and commit repeats the poll with the same cursor (safe by contract). A fenced or lost CAS discards the result.
- First activation: `InitialCursor` decides baseline-without-emitting or emit.
- Failures: backoff with jitter; after 5 consecutive failures health `DEGRADED`, `poll.failed` event and (rate-limited) error flow.

### 2. Persistent listeners
- `listener_holder` rows are leased with epoch; defaults renewal 3 s, TTL 10 s, database clock; local self-fencing on monotonic clock (ADR-0013 §4). Failover ≤ TTL + 1 s.
- Distribution: a hosting worker holds at most `ceil(total / live hosts) + 1` listeners, using the informational `worker` table; excess is released gracefully (stop consuming, finish acks of committed messages, close).
- Acknowledgement: messages are passed to `AcceptEvents` (group commit); the transaction checks the holder epoch; **only after commit** does the holder `Ack`. A fenced holder never acks; the broker redelivers to the next holder.
- Delivery classes: `AT_LEAST_ONCE` (RabbitMQ/AMQP 0-9-1 manual ack, AMQP 1.0 settlement, MQTT QoS 1/2, Redis Streams consumer groups) → deduplication by message id; `CURSOR_EXACT` (Kafka offsets, IMAP UID with UIDVALIDITY, SSE Last-Event-ID, local-file scan state) → cursor stored in the acceptance transaction, Kafka offsets also committed to the broker after the database commit for visibility; `AT_MOST_ONCE` (Redis pub/sub, PostgreSQL LISTEN, MQTT QoS 0, file-system notifications without scan) → documented possible loss during failover, shown in the editor.
- Flow control: `maxInFlight` (n8n "parallel message processing limit") bounds executions in progress for the trigger; reaching it or the backpressure cap stops consumption (prefetch/credit), never acks unpersisted messages.
- Local-file trigger (owner decision SPEC-002 Q10): disabled by default on the instance; the operator enables it by declaring allowed paths and a host tag; publication fails for a path outside the allowed list (T-V6); only hosting workers with the tag may hold it.
- Hosting: polls, listeners and registration calls run only on workers with `--trigger-hosting=true` (owner decision SPEC-002 Q2); the scheduler never runs them.

### 3. Tests
Fault-injection additions to ADR-0019 (rows of its §3 matrix): FI-52 kill the poll worker between `Poll` and commit; FI-53 kill a listener holder with messages in flight; FI-54 SIGSTOP the holder 60 s; FI-55 broker redelivery storm. Outcomes: no lost acknowledged-source message for `AT_LEAST_ONCE`/`CURSOR_EXACT`; one execution per message id/offset.

## Consequences
The same lease/fencing proofs as tokens; duplicates from redelivery are absorbed by the deduplication index or cursor.

## Alternatives considered
Holding listeners in the scheduler process (rejected by the owner, SPEC-002 Q2). Ack on execution success (n8n option): rejected as default — Orcon's acceptance is durable; failures are handled by retry-from-failure.
