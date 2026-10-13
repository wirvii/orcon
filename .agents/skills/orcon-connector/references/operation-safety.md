<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Operation safety

Status: Accepted

This file applies the external-operation safety contract (ADR-0005, ADR-0029 §5) to each connector operation. Rule IDs (`SF-…`) are stable. Precedence is in `kinds.md`.

## Rules

- **SF-1** Choose the level per operation (ADR-0029 §5): reads → `READ_ONLY`; writes where the provider honours an idempotency key → `IDEMPOTENCY_KEY` (`location`, `ttl` from provider documentation, `scope`); writes with a unique natural key and a lookup → `RECONCILE` with a verification operation whose purity is `READ`; otherwise `UNSAFE`.
- **SF-2** `upsert` with a provider-side matching key is `IDEMPOTENCY_KEY` (mode `NATURAL_KEY`) or `RECONCILE`, never `UNSAFE`.
- **SF-3** `delete` is `RECONCILE` (`notFoundMeansFailed: false`) when a lookup exists; deleting twice must be harmless.
- **SF-4** Every `UNSAFE` operation declares `manualCheck` with a template that tells a person how to check in the service whether the write happened, with a URL when the service has one.
- **SF-5** `proofLevel` starts `DECLARED`; recorded tests and fault tests raise it to `TESTED`; only catalog review sets `VERIFIED`.
- **SF-6** `batch` only when the provider returns per-item results; `perItemResults: true` is required for partial failures.
- **SF-7** The documentation shows one sentence per level:

  | Level | Sentence |
  |---|---|
  | `READ_ONLY` | Safe to repeat. |
  | `IDEMPOTENCY_KEY` | Safe to repeat: the service ignores duplicates with the same key for <ttl>. |
  | `RECONCILE` | Checked with the service before any repeat. |
  | `UNSAFE` | Not repeated automatically. If the result is unclear, a person is asked to confirm (UNCERTAIN). |

- **SF-8** A weaker level in a new version is a major change (`versioning.md`).
