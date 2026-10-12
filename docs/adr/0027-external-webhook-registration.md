<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0027 — External webhook registration lifecycle (app-event triggers)

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-002.

## Context
T5: app-event triggers register and deregister external webhooks automatically; operations are retried, failures surfaced, and no orphans are left at providers.

## Decision

### 1. Desired and actual state
`trigger_registration` rows per `(trigger, address)` hold `desired_state ∈ {PRESENT, ABSENT}` and `actual_state ∈ {UNKNOWN, REGISTERING, PRESENT, REMOVING, ABSENT, FAILED}`. Publication, activation, deactivation, address removal and flow deletion change only `desired_state` (same transaction as the lifecycle change).

### 2. Reconciler
Due rows are claimed with lease and epoch by hosting workers (engine-plane function, ADR-0047). Before calling the provider, the intent (`REGISTERING` / `REMOVING`) is committed. The provider call happens outside transactions. The result (`external_reference`, encrypted `secret_reference`, `expires_at`) is committed fenced.

### 3. Orphan avoidance
- The callback URL embeds a per-registration token (only its hash, `token_hash`, is stored).
- After a crash in `REGISTERING`/`REMOVING`, the reconciler calls `List` and matches by callback URL/token before deciding to register again or to mark removed.
- An event arriving on a registration whose `desired_state = ABSENT` or unknown token → `410 Gone` and a removal is enqueued.
- A periodic sweep (default 24 h) lists provider registrations pointing at this instance's ingress URL and removes those without a desired row.

### 4. Retries, renewal, surfacing
Publication and activation never fail because of a registration (owner decision SPEC-002 Q8). Backoff 1 s → 1 h with jitter, unlimited while desired; health `PENDING`/`FAILING` with last error visible in the editor and API; `registration.failed` events; rate-limited error-flow start. Expiring subscriptions renew at two thirds of their lifetime. Provider handshakes (challenge/validation tokens) are answered by `Handshake` on the api role without creating executions.

### 5. Manual-setup providers
Where a provider requires manual configuration (app-level webhooks), the trigger is a plain `WEBHOOK` trigger with a verification preset; the editor shows the address and setup steps.

### 6. Tests
Fault-injection addition to ADR-0019 (a row of its §3 matrix): FI-56 kill the reconciler between `Register` and its commit, and deactivate the flow while a registration is in flight. Outcome after convergence: the provider holds no registration for the deactivated flow and exactly one per address of active flows; an event arriving on an unwanted registration gets `410 Gone` and enqueues a removal; the periodic sweep removes a seeded orphan.

## Consequences
Registrations converge to the desired state despite crashes; orphans are detectable and removed.

## Alternatives considered
Register synchronously in the publication request and fail the publication on error (n8n behaviour; rejected by the owner, SPEC-002 Q8).
