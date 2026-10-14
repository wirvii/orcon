<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Token formats

Status: **Accepted** | Source: ADR-0056 §3 (accepted 2026-10-12 via SPEC-005)

Format of the credentials Orcon issues, published for secret scanners and partner programs. ADR-0056 is normative; if this page and the ADR disagree, the ADR wins.

## API keys and OAuth tokens
`orcon_<type>_<id>_<secret>_<check>`

| Part | Content |
|---|---|
| `type` | `sak` (service-account key), `pak` (personal key), `ocs` (OAuth client secret), `oat` (OAuth access token) |
| `id` | 16 characters, Crockford base32; the lookup key |
| `secret` | 43 characters, base62: 256 random bits from the operating system's cryptographically secure random generator |
| `check` | 6 characters, base62 of CRC32 over the preceding characters |

Detection regex:

```
orcon_(sak|pak|ocs|oat)_[0-9A-HJKMNP-TV-Z]{16}_[0-9A-Za-z]{43}_[0-9A-Za-z]{6}
```

Detection is best-effort: third parties cannot reach self-hosted instances, so revocation stays with the instance owner. The lowercase prefixes are part of the token format and do not change with the API enum form.

## Webhook secrets
Webhook signing secrets use the prefix `whsec_` (ADR-0055 §5).
