<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Testing

Status: Accepted (2026-10-13)

This document defines the test levels of the web application, the coverage threshold, how the API is simulated in screen tests and which end-to-end journeys exist or are owed. Component stories and their accessibility and visual tests belong to `orcon-ui` ([accessibility](../../orcon-ui/references/accessibility.md#target)). Rule IDs (`TE-…`) are stable.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` rules for anything visual > this rulebook > reviewer preference.

## Levels

- **TE-1** Levels:
  1. unit — Vitest, node environment: `model/`, `store/`, key factories, live handlers, `app/` logic;
  2. screen — Vitest + Testing Library + Mock Service Worker with handlers from `@orcon/api-contract/msw`; every screen covers loading, empty, error, forbidden and data;
  3. end-to-end — Playwright against the real binary with a fresh SQLite data directory, desktop and phone projects;
  4. accessibility — axe on every page an end-to-end journey visits.

  Component stories and their axe and visual tests belong to `orcon-ui`.
- **TE-2** Coverage (v8): at least 80 % lines, branches, functions and statements over `src/**/{model,store,api}/**` and `src/app/{boundary,live-events,errors,forms,perf}/**`, generated code excluded. Lower fails CI.

## Writing tests

- **TE-3** Mock Service Worker runs with `onUnhandledRequest: 'error'`; responses come from the generated examples, overriding only the fields a test needs; streams use the `sse` handler.
- **TE-4** Query the DOM by role and accessible name; `user-event` for interaction; no class selectors; test ids only when no accessible handle exists.
- **TE-5** No DOM snapshot tests.
- **TE-6** Deterministic: fake timers for time logic, seeded example data, no real network, no sleeps (`expect.poll`, auto-waiting).
- **TE-7** End-to-end data is created through the UI or the SDK, never by writing the database file.
- **TE-9** A bug fix ships a test at the lowest level that reproduces it.
- **TE-10** Screens and journeys with state changes cover retry, cancellation, conflict (`409`/`412`) and stream reconnect where they apply.

## Journeys

- **TE-8** Journeys: J0 shell, deep-link reload and settings round trip (exists now); J1 create, run and inspect a flow; J2 approve a pending item at phone width; J3 resolve an UNCERTAIN operation. J1–J3 are written by the slices that build those screens and run before merge.

## Inherited checks

The screens these checks need do not exist yet. The specifications that build them inherit these checks as acceptance criteria:

| ID | Owed by | Check |
|---|---|---|
| H-API-1 | public API contract slice | `@orcon/api-contract` generates from the real OpenAPI document and `pnpm generate:check` passes |
| H-API-2 | public API contract slice | a type test proves that `z.infer` of every public schema equals the SDK type |
| H-API-3 | public API contract slice | the `@orcon/ui` `ProblemDetails` matches the API error shape and `toProblemDetails` is simplified; the network rule passes with `packages/js/sdk` as its only exception |
| H-API-4 | live events slice | `@orcon/sdk` implements `LiveEventSource`; a screen test with the Mock Service Worker `sse` handler covers reconnection and `stream.reset` |
| H-UI-1 | product screen slices | J1, J2 and J3 in `apps/web/e2e/journeys/` pass in `web-e2e` |
| H-UI-2 | editor slice | `editor-500.spec.ts` points at the real editor route with the same thresholds and the same emulation (`performance.md` PF-4) |
| H-UI-3 | executions slice | the executions list uses the data table virtual mode, with a 10,000-row test against Mock Service Worker |
| H-UI-4 | product screen slices | every main route calls `markFirstUsefulScreen` and joins `first-useful-screen.spec.ts` |
| H-UI-5 | product screen slices | every screen has a screen test for its five states |
