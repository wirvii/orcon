<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0050 — Security release gates and adversarial isolation suite

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
S3 requires adversarial cross-organization tests on every subsystem on both stores; principle 12 and ADR-0019 require evidence before claims.

## Decision

### 1. Isolation suite (`test/isolation`)
- A registry enumerates every HTTP route and every store method; a test fails if one lacks an isolation case.
- Fixtures: organizations A and B with projects, flows, versions, credentials (owned and shared), secret stores, executions with data, blobs, webhooks, schedules, audit events, SIEM sinks, human-attention notification channels and rules, groups, roles, SCIM tokens.
- For every role of A (including Owner and a custom role with all permissions) and every route/method: attempts to read, list, count, update, delete, export, reference (`$secrets`, credential ids in flow documents, sub-flow targets, blob ids, webhook paths, notification recipients) B's resources by direct id, filters, pagination cursors and caches; expected: 404/empty/validation error, never B's data, never B-derived counts or timing beyond a fixed tolerance.
- Engine cases: tokens of B processed under A's context fail closed; batched writes mixing A and B land in the right organizations; credential resolution never crosses organizations; `resolve_*` lookups return only the matching organization and never another organization's data.
- Runs on SQLite, PostgreSQL, and PostgreSQL "RLS-only" (wall 1 predicates removed by a test-only build tag) to prove wall 3 alone blocks every case that reaches the database.

### 2. Fault injection (added to ADR-0019 §3)
| ID | Fault | Required outcome |
|---|---|---|
| FI-30 | `kill -9` during MK rotation | restart resumes; every OK unwraps; no credential unreadable |
| FI-31 | `kill -9` during OK rotation re-wrap | resumes; old version retired only after all DEKs moved |
| FI-32 | wrong MK at startup | process exits before any write; clear message |
| FI-33 | OAuth refresh holder killed before provider call / after provider response | single refresh at a time; second case yields `needs_reauth` only if the provider rotated the token |
| FI-34 | three processes need the same expired token concurrently | exactly one token-endpoint call (mock provider counts) |
| FI-35 | audit sealer killed mid-seal | no gaps, no duplicates; verification passes |
| FI-36 | direct DB update/delete/reorder of audit rows | verification detects and alerts within the next run |
| FI-37 | external secret store unreachable | cached values within max-stale, then transient errors; no persistence |

### 3. Other gates
SAML/OIDC attack batteries; RLS configuration check (policies forced, roles without BYPASSRLS, no views over organization data); `bench/rls` gate (ADR-0047 §6); D20 measured with secure defaults (RLS on in multi-organization runs, audit on, encryption on).

### 4. CI placement
Per pull request: unit, store, isolation suite on SQLite and PostgreSQL. Nightly: RLS-only mode, fault cases, attack batteries, `bench/rls` trend. Release: all gates.

## Consequences
Isolation claims are backed by executable evidence on both stores.

## Alternatives considered
Sampling endpoints (rejected: new endpoints would escape coverage).
