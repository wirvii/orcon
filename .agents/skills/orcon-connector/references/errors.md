<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Errors

Status: Accepted

This file fixes how a connector classifies provider failures (ADR-0011 §1, ADR-0030 §5, ADR-0079 §4 and §5). Rule IDs (`ER-…`) are stable. Precedence is in `kinds.md`.

## Rules

- **ER-1** Classes are `TRANSIENT`, `PERMANENT` and, on operations whose purity is `EFFECT` only, `UNCERTAIN` (ADR-0011 §1, ADR-0079 §4).
- **ER-2** Codes are one of the following fourteen. Codes are identifiers and keep this form.

  | Code | Meaning |
  |---|---|
  | `AuthInvalid` | the credential is rejected |
  | `AuthExpired` | the credential expired and could not be refreshed |
  | `PermissionDenied` | the credential lacks a permission or scope |
  | `NotFound` | the target record does not exist |
  | `Conflict` | the request conflicts with the current state of the record |
  | `ValidationFailed` | the provider rejected the input |
  | `RateLimited` | the provider asked to slow down; the request was not applied |
  | `QuotaExceeded` | a plan or account quota is exhausted |
  | `PayloadTooLarge` | the request or file is larger than the provider accepts |
  | `ServiceUnavailable` | the provider answered with a server error |
  | `Timeout` | no answer within the deadline |
  | `NetworkError` | the connection failed |
  | `Unsupported` | the provider does not support the requested combination |
  | `ProviderError` | any other provider failure |

  Host-raised codes such as `EgressDenied` and `ConnectorRevoked` stay with the host and are never raised by a connector.
- **ER-3** Default mapping: 400/422 → `ValidationFailed` `PERMANENT`; 401 → `AuthInvalid` `PERMANENT` (after the host refresh of CR-7); 403 → `PermissionDenied` `PERMANENT`; 404 → `NotFound` `PERMANENT`; 409 → `Conflict` `PERMANENT`; 413 → `PayloadTooLarge` `PERMANENT`; 402 or provider quota → `QuotaExceeded` `PERMANENT`; 429 → `RateLimited` `TRANSIENT`; 5xx → `ServiceUnavailable`; timeouts → `Timeout`; connection failures → `NetworkError`.
- **ER-4** Class by safety level for a 5xx, a timeout or a connection failure after the request was sent: `READ_ONLY` and `IDEMPOTENCY_KEY` → `TRANSIENT`; `RECONCILE` and `UNSAFE` → `UNCERTAIN` (reconciliation decides, or a person confirms). A 429 is `TRANSIENT` for every level and means the request was not applied. (Maintainer decision, 2026-10-12.)
- **ER-5** Provider error messages are included after masking (ADR-0017); never include request bodies or credential values.
- **ER-6** The message names the operation and what the user can do (re-authorize, check permission, reduce the batch).
