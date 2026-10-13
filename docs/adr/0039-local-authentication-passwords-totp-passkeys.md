<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0039 — Local authentication: passwords, TOTP and passkeys

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
S6: email + password with TOTP 2FA and passkeys, free.

## Decision
1. **Passwords**: argon2id (`golang.org/x/crypto/argon2`, BSD-3) with defaults m=19 MiB, t=2, p=1, parameters stored with the hash and upgraded on login; minimum length 12; no composition rules; local common-password list; optional k-anonymity breached-password check, disabled by default because no network access is mandatory (owner decision, SPEC-004 Q18). Login does constant work for unknown accounts.
2. **Throttling**: per-account exponential delay after 5 failures (cap 15 min) and per-IP token bucket; no permanent lockout. Uniform responses for unknown email, wrong password and disabled account.
3. **Email flows**: verification and password reset via single-use tokens (hashed, 30 min); without SMTP configured an instance admin can generate reset links (audited).
4. **TOTP**: RFC 6238, SHA-1, 6 digits, 30 s, ±1 step; seed encrypted with the instance key (ADR-0042); last accepted step stored to reject reuse; library `pquerna/otp` (Apache-2.0).
5. **Recovery codes**: 10 single-use codes, 10 characters base32, stored as argon2id hashes; regenerating invalidates the previous set.
6. **Passkeys**: WebAuthn Level 3 via `go-webauthn/webauthn` (BSD-3); RP ID = host of `ORCON_PUBLIC_URL`; discoverable credentials; user verification required for passwordless login; a passkey login is `aal2`; attestation `none` by default, optional AAGUID allowlist per organization; sign-counter regressions flagged.
7. **Assurance**: password only → `aal1`; password + TOTP or recovery code → `aal2`; passkey with UV → `aal2`.
8. **Admin recovery**: `orcon user reset-2fa <email>` (instance admin, audited, revokes sessions).

## Consequences
Strong local authentication with no external service; works offline.

## Alternatives considered
bcrypt (rejected: 72-byte limit, weaker memory hardness); SMS OTP (rejected: SIM-swap risk, needs a paid provider); breached-password check enabled by default (rejected: would make outbound network access mandatory).
