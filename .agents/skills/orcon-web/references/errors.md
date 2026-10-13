<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Errors

Status: Proposed — pending owner approval

This document defines how the web application turns failures into what the user sees: one adapter for problem+json responses, a fixed treatment per HTTP status, error boundaries, contract errors and what is never logged. Where and how an error is displayed is defined by `orcon-ui` ([forms and errors](../../orcon-ui/references/components.md#forms-and-errors), [notifications](../../orcon-ui/references/components.md#notifications)). Rule IDs (`ER-…`) are stable.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` rules for anything visual > this rulebook > reviewer preference.

## Problem details

- **ER-1** `app/errors/to-problem-details.ts` converts the SDK typed problem into the `orcon-ui` `ProblemDetails`; no feature parses problem bodies.
- **ER-2** Screen-level errors render with `ProblemAlert`, field errors with `FormField`; decisions are never asked in a toast ([forms and errors](../../orcon-ui/references/components.md#forms-and-errors), [notifications](../../orcon-ui/references/components.md#notifications)).

## By status

- **ER-3** By status:

  | Status | Treatment |
  |---|---|
  | `401` | sign-in, keeping the location |
  | `403` | permission message, no retry |
  | `404` | not-found component |
  | `409`, `412` | show the current state and the conflict choices (an already resolved attention request shows its result) |
  | `428` | a bug; reported |
  | `429`, `503` | after the SDK retries, a retry action that honors `Retry-After` |
  | `5xx` | `ProblemAlert` with request id and retry |

## Boundaries and contract errors

- **ER-4** A root error boundary and a per-route `errorComponent`; no third-party error service without an ADR.
- **ER-5** A response that fails its Zod schema is a contract error: generic message with request id; schema path logged with `console.error`; raw content never shown.
- **ER-6** No floating promises; catch only to translate an error.

## Logging and offline

- **ER-7** Never log request bodies, credential values or execution data; `console` is forbidden except `console.error` in `app/errors/`.
- **ER-8** Offline: Query `onlineManager` pauses queries and the offline banner of `orcon-ui` is shown.
