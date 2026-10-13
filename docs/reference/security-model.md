<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Security model (threat model)

Status: **Accepted** | Source: SPEC-004 §17 and Annex C, with ADR-0038…ADR-0051 (accepted 2026-10-12 via SPEC-004)

Abbreviated STRIDE threat model of the security design. Requirement identifiers (`R-…`) are those of SPEC-004; the mitigations are specified by the ADRs cited. MK is the master key and OK the organization key of the key hierarchy (ADR-0042). Walls 1–4 are the isolation layers of ADR-0046.

## Actors
- **A1** Unauthenticated attacker on the Internet.
- **A2** Malicious member of organization A attacking organization B.
- **A3** Low-privilege user trying to escalate inside their own organization.
- **A4** Flow author trying to extract credentials.
- **A5** Malicious connector (SPEC-003).
- **A6** Compromised or misconfigured identity provider of an organization.
- **A7** Someone who obtains a copy of the database or a backup.
- **A8** Someone who can write to the database.
- **A9** Instance operator (trusted; out of scope except for their access through the user interface).
- **A10** Someone who receives or intercepts a human-attention notification outside Orcon (mailbox, chat channel, webhook receiver).

## Assets
Credentials, execution data, flow definitions, identities, the audit log, MK and OK.

## Out of scope
Full compromise of the server with the MK in memory.

## Threats
| # | Threat | Actor | Mitigation | Residual risk |
|---|---|---|---|---|
| TM-01 | Reading another organization's data through a forgotten filter | A2 | Walls 1–4; uniform 404; no views over organization data | None known on PostgreSQL; on SQLite, no wall 3 |
| TM-02 | Reading another organization's data through an in-process cache | A2 | Cache keys include `org_id`; cases in the isolation suite | — |
| TM-03 | Escalating privileges by creating a role with permissions the actor lacks | A3 | Subset rule (R-AZ-03) | — |
| TM-04 | Keeping access after offboarding | A3 | Revocation ≤ 1 s, authorization epochs, SCIM/LDAP | Window of ≤ 1 s |
| TM-05 | Extracting a use-only credential by sending it to one's own host | A4 | Host binding, no `$credentials` in expressions, Code node without credentials | Approved connectors with raw credential access |
| TM-06 | Extracting a credential through error messages or logs | A4/A5 | Masking of resolved values (R-CRED-04) | Transformations of the value (for example base64) are not detected |
| TM-07 | Copying ciphertext from B to A in the database | A8 | Additional authenticated data with organization and id | — |
| TM-08 | Reading credentials from a database backup | A7 | MK kept outside the database | Backup and MK obtained together |
| TM-09 | Tampering with or deleting the audit log | A8 | Hash chain + MAC with the OK + anchors + hourly verification | Recent truncation without an external anchor (persistent warning, SPEC-004 Q17) |
| TM-10 | Organization A's identity provider asserts the identity of a user of B | A6 | Linking by `(iss, sub)`; email only with a verified domain; accepted sources per organization | A domain verified by A covers A's own users |
| TM-11 | SAML signature wrapping or replay | A1/A6 | Reviewed library + own checks + record of assertion ids; IdP-initiated login off by default | Future library flaws; organizations that enable IdP-initiated login |
| TM-12 | OIDC algorithm confusion (`none`, HS with a public key) | A1 | Algorithm allowlist per identity provider | — |
| TM-13 | LDAP injection | A1 | RFC 4515 escaping | — |
| TM-14 | Brute force of passwords or TOTP | A1 | Attempt limits, argon2id, TOTP reuse rejected | — |
| TM-15 | Session fixation or CSRF | A1 | Token rotation, `__Host-` cookies, SameSite, Origin check + double submit | — |
| TM-16 | SSRF through a SIEM sink, an external secret store or a notification channel | A2 | Anti-SSRF egress client | Internal hosts allowed by the instance administrator |
| TM-17 | Using the server's cloud identity from an organization | A2 | R-XS-04 (instance-level only; explicit, audited authorization per organization) | Organizations authorized by the instance administrator |
| TM-18 | A double OAuth refresh invalidates the token | — (concurrency) | Single refresh holder with epoch | Crash after the provider rotated the token |
| TM-19 | Starting with the wrong MK and mixing ciphertexts | A9 (mistake) | Canaries checked at startup | — |
| TM-20 | Enumerating users or organizations | A1/A2 | Uniform responses; 404 across organizations; generic collisions for routes and project identifiers, with their own case in the isolation suite (ADR-0050 §1); `resolve_*` functions that return only what is needed | Response timing (mitigated with constant work at login) |
| TM-21 | Arbitrary SQL through injection | A1 | Only parameterized queries (checked in CI) | RLS does not protect here |
| TM-22 | Exhausting shared resources (noisy neighbour) | A2 | Level 0 limits, fair share per organization | Approximate across workers |
| TM-23 | Leaking secrets or protected data through a human-attention notification (forwarded email, chat channel, webhook receiver) | A10/A4 | R-AUD-09: payload with identifiers, names, reason and link only; secrets never (masked); internal recipients with access to the project; channel secrets encrypted; anti-SSRF egress; signed webhooks | Flow and step names visible to whoever receives the notification; masked, truncated data only if a project enables it (owner decision SPEC-004 N1) |
| TM-24 | Unauthorized or repeated human action from a leaked notification link or a forged callback | A1/A10 | R-AZ-13: the link does not authorize; authenticated principal with permission in the project; audit event `execution.human_action_taken`; the execution transition happens once (a second action on an already resolved wait is rejected) | No actions from email or chat in V1 (owner decision SPEC-004 N2); Slack buttons only in a later version and with verified chat identities |
