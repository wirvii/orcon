<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Errors and problem codes

Status: Proposed — pending owner approval

This document defines the error format of the public API, the problem-code catalog and which codes every operation must declare. It refines the problem+json convention of ADR-0053. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`ERR-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md) (while ADR-0080 is Proposed, as design input), then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## Format

- **ERR-1** Every 4xx/5xx response of a non-protocol operation is `application/problem+json` referencing `#/components/schemas/Problem`. [`problem-responses`]
- **ERR-2** `type` is `urn:orcon:problem:<code>`; `code` repeats `<code>`; codes are kebab-case and stable; changing the code for the same condition is breaking ([VER-2](versioning.md)). [`problem-responses`]

## Codes per operation

- **ERR-3** Each operation lists every problem code it can return in `x-orcon-problems`; each listed code's status has a declared response. [`problem-codes-declared`]
- **ERR-4** Minimum codes: all authenticated operations `unauthenticated`, `rate-limited`, `internal-error`; with a permission other than `authenticated` `forbidden`; with an id in the path `not-found`; with parameters or body `validation-failed`; with a body `payload-too-large`; with idempotency `idempotency-key-reused`, `idempotency-in-progress`; with `If-Match` `precondition-failed`, `precondition-required`; lists `invalid-page-token`; experimental `experimental-not-enabled`. [`problem-codes-declared`]
- **ERR-5** Messages never contain secret values or data of another organization; `errors[].pointer` is a JSON Pointer into the request. [review]

## Catalog

- **ERR-6** The catalog source is `components/schemas/ProblemCode` with `x-orcon-problem-catalog`; the page `docs/reference/problem-codes.md` is generated from it. Each entry has: code, status, title, when, retryable (yes/no). Codes:

  | Code | Status | Catalog status |
  |---|---|---|
  | `validation-failed` | 400 | confirmed |
  | `invalid-page-token` | 400 | confirmed |
  | `experimental-not-enabled` | 400 | confirmed |
  | `unauthenticated` | 401 | confirmed |
  | `forbidden` | 403 | confirmed |
  | `step-up-required` | 403 | confirmed |
  | `personal-keys-disabled` | 403 | confirmed |
  | `not-found` | 404 | confirmed |
  | `conflict` | 409 | confirmed |
  | `idempotency-in-progress` | 409 | confirmed |
  | `secret-already-issued` | 409 | confirmed |
  | `attention-already-resolved` | 409 | confirmed |
  | `project-managed-by-git` | 409 | confirmed |
  | `deprecated-removed` | 410 | confirmed |
  | `precondition-failed` | 412 | confirmed |
  | `payload-too-large` | 413 | confirmed |
  | `idempotency-key-reused` | 422 | confirmed |
  | `precondition-required` | 428 | confirmed |
  | `rate-limited` | 429 | confirmed |
  | `internal-error` | 500 | confirmed |
  | `service-unavailable` | 503 | confirmed |
  | `confirmation-required` | to be confirmed | proposed; confirmed by SPEC-005 API1/API3 |
  | `federated-token-rejected` | to be confirmed | proposed; confirmed by SPEC-005 API1/API3; belongs to a protocol endpoint and uses the OAuth error format |

- **ERR-7** New codes are allowed only for new conditions, added to the catalog in the same pull request. [`problem-codes-declared`]
