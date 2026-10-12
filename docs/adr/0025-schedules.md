<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0025 — Schedules: single firing, time zones, DST and missed runs

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-002.

## Context
T2: schedules fire exactly once across servers (claim recorded in the database); per-flow time zone including DST; missed runs skipped by default; optional "catch up if missed recently" with a margin in hours that runs once; every missed run recorded with optional alert.

## Decision

### 1. Rules
n8n-compatible rule kinds: seconds, minutes, hours, days, weeks, months (calendar rules with hour/minute and weekday/day-of-month) and cron (5 or 6 fields). Several rules per node, each with its own `rule_index`. Time zone: IANA name per flow (default instance zone), stored as text next to UTC instants. The binary embeds the tz database (`time/tzdata`) and records its version.

### 2. Firing
The scheduler claims due `schedule_state` rows (`next_slot_at ≤ now()` by database clock) with `FOR UPDATE SKIP LOCKED` in short transactions, through an engine-plane function because due rows span organizations (ADR-0047). In one transaction: insert `schedule_fire(slot_at, outcome = FIRED)` (unique per organization, rule and slot), call `AcceptEvents` (execution with `scheduledFor = slot_at`), advance `next_slot_at`. Two schedulers can never both fire a slot: the unique key decides. This is a local database guarantee, not an external-effect guarantee.

### 3. DST
- Calendar rules are evaluated in local wall-clock time. Non-existent local time (spring forward): fire once at the first valid instant after the gap. Repeated local time (fall back): fire only at the first occurrence (owner decision SPEC-002 Q5).
- Interval rules (every N seconds/minutes/hours) are evaluated as absolute elapsed time in UTC from an anchor and are unaffected by DST.

### 4. Missed slots
- A slot is missed when it is detected after `slot_at + grace` (default 60 s). Late slots within grace fire normally with their real `scheduledFor`.
- On detection (startup, failover or delayed claim), missed slots are enumerated and recorded with `outcome = MISSED` (aggregated as ranges with `range_ended_at` beyond 1,000 slots); `schedule.missed` events and optional alert.
- Policy `SKIP` (default): nothing runs. Policy `CATCH_UP_ONCE` with `marginHours`: if the latest missed slot is within the margin of now, exactly one execution runs for it (`catchUp = true`), others remain missed. Replaying all missed slots is not offered.
- Inactive periods (flow deactivated or deleted) are not missed slots; reactivation starts from the next future slot.
- Backpressure (ADR-0021 §4) records `SKIPPED_BACKPRESSURE`.

## Consequences
Single firing is verifiable by a unique constraint; DST behaviour is explicit and tested.

## Alternatives considered
Leader election for a single scheduler (rejected: the unique key suffices; ADR-0004 needs no leader). Quartz-style skipping of non-existent times (rejected by the owner, SPEC-002 Q5: it loses a day of a daily job).
