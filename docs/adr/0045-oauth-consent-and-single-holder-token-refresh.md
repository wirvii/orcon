<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0045 — OAuth credential consent and single-holder token refresh

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12 (consistency with ADR-0077); 2026-10-13 (stored enumerated values are UPPER_SNAKE, ADR-0015 §2)

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
Given in BL-004: token refresh performed by exactly one holder per credential via the SPEC-001 lease mechanism; others wait — fixing concurrent-refresh invalidation observed in n8n with providers that rotate refresh tokens.

## Decision
1. **Consent**: authorization code (+ PKCE where supported) and client credentials grants per ADR-0030 §4; `state` = MAC'd `{org, credential, session, nonce, exp=10 min}`; single callback `/oauth/callback`; tokens stored as secret fields of a new credential version.
2. **Freshness**: a token is refreshed when it expires within 60 s (skew) or a request returns 401 and the operation is `read` or `retrySafe` (ADR-0029 §6).
3. **Lease**: `credential_refresh_lease(org_id, id, credential_id, lease_owner, lease_epoch, lease_expires_at)`, unique `(org_id, credential_id)`, acquired by conditional update with database-clock expiry (TTL 30 s), as ADR-0013 §4, through `orcon_engine.acquire_refresh_lease(credential_id, lease_owner, lease_duration)` (ADR-0047 §2). The holder re-reads the credential version first (another holder may have finished), then calls the token endpoint with no transaction open, then writes the new version fenced by the epoch and emits `pg_notify('orcon_cred', credentialId)` (SQLite: in-process).
4. **Waiters**: other processes wait for the notification or poll every 500 ms up to 30 s, then read the new version; within one process a single goroutine performs the wait per credential (call coalescing).
5. **Failures**: `invalid_grant` → `NEEDS_REAUTH`, owner notified, dispatches fail with a permanent error naming the credential; network errors → lease released, transient error.
6. **Residual risk**: if the provider rotates the refresh token and the holder crashes after the provider responded but before the fenced write, the new refresh token is lost and re-consent is needed. This window cannot be closed by Orcon and is documented.

## Consequences
No concurrent refreshes per credential across the cluster.

## Alternatives considered
Refresh in every process (n8n behaviour; rejected); advisory locks (rejected: tied to sessions, no fencing).

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-009 K19 (2026-10-13, owner decision).** §5 (`credential.status` `NEEDS_REAUTH`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
