<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Testing

Status: Accepted

This file covers recorded tests (cassettes), scrubbing and detection, fault tests, conformance and live tests. Rule IDs (`TE-…`) are stable. Precedence is in `kinds.md`. The commands that run these tests are in `checks.md`.

## Rules

- **TE-1** Every connector has `connector_test.go` with `TestConformance`, `TestFaults` and `TestLive` (templates).
- **TE-2** Cassettes per operation: one success case with `example: true`; for `getMany` a two-page case and a `limit` case; one 4xx case; for `OAUTH2` credentials a 401-then-refresh read case; for `poll` triggers two polls with a cursor.
- **TE-3** Record with `orcon connector record` (or `ORCON_RECORD=1` in Go) against a test account; never edit a cassette by hand except to shorten long arrays, and re-run the detector after. Cassettes keep recorded values as they were (NM-13).
- **TE-4** The scrubber runs before saving; the detector rejects tokens, secrets, real e-mail addresses, personal names that are not pseudonyms, phone numbers, card numbers and IBANs; exceptions go to `tests/scrub-allow.json` with a reason.
- **TE-5** Replay runs offline on every change; unmatched requests fail with a diff.
- **TE-6** Fault matrix, per operation (fake clock, replayed success cassette with an injected fault):

  | Level | Cut after send | 429 with Retry-After | 503 / 500 |
  |---|---|---|---|
  | `READ_ONLY` | retried, succeeds | retried after Retry-After | retried |
  | `IDEMPOTENCY_KEY` | resent with the same key | retried with the same key | resent with the same key |
  | `RECONCILE` | verification called, no resend before it; found → SUCCEEDED, consistent not-found → FAILED, else UNCERTAIN | retried after Retry-After | verification as for a cut |
  | `UNSAFE` | UNCERTAIN, zero resends, exactly one `execution.awaiting_human` | FAILED `RateLimited`, zero resends | UNCERTAIN, zero resends, exactly one `execution.awaiting_human` |

- **TE-7** The common conformance suite (ADR-0031 §5) runs for every connector, including FI-01 to FI-03 against the mock provider.
- **TE-8** Live tests: `tests/live.json` declares whether the provider offers a free test account; when it does, list operations and cleanup; writes only in test mode. Live tests never run for pull requests and never block merges.
- **TE-9** Tests are deterministic: fixed clock and random source, no real network outside `TestLive`.
- **TE-10** Connector tests import `internal/adapter/connectortest` only from `_test.go` files (the declared exception in `../../orcon-go/references/architecture-layers.md`).
