# ADR-0004 — SQLite local and PostgreSQL distributed

Status: **Accepted (architectural direction)** | Date: 2026-10-10

## Decision
Local standalone mode targets zero external services with SQLite. Distributed deployment uses PostgreSQL as authoritative state, durable tasks and transactional coordination. Redis may be introduced as an optional accelerator, but cannot be required for correctness. No mandatory Temporal or Hatchet server.

## Requirements
PostgreSQL task claiming may use SKIP LOCKED with leases and fencing, subject to a reviewed scheduler ADR. Atomic local state and task creation is required. SQLite is not a distributed shared-file coordinator. The same conceptual execution semantics should apply to both storage adapters, with documented capacity/consistency differences.

## Open
Migrations, SQLite concurrency strategy, PostgreSQL isolation and locking, leader election, partition handling, schema versioning, backups, timers, fair scheduling, quotas, disaster recovery and benchmark thresholds.
