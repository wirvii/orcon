---
name: orcon-web
description: Applies the Orcon web application code standards - feature folders and import boundaries, TanStack Query over the generated SDK, live updates over SSE, TanStack Router with typed URL state, Zustand for the flow editor only, React Hook Form with generated Zod schemas, problem+json errors, strict TypeScript, testing levels, performance budgets and dependency licenses. Use when writing, changing or reviewing TypeScript or React code in apps/web or packages/js.
license: Apache-2.0
compatibility: Rules in references/ are self-contained; scripts/ require an Orcon repository checkout with pnpm 9, Node 22, Go, GNU make and uv.
metadata:
  version: 1.1.1
  pinned: true
  owner: wirvii
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Orcon Web

The complete rules are in `references/`. This file is the procedure and the checklist; open the reference named in each step when the step applies.

Anything visual - components, tokens, styles, strings, icons, accessibility, the look of the canvas - follows the `orcon-ui` skill (`../orcon-ui/SKILL.md`). This skill never restates those rules; load both when a change touches a screen.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` for anything visual > `references/` > reviewer preference.

| Reference | Rules |
|---|---|
| `references/architecture.md` | AR - app and feature folders, allowed subfolders, import direction, no cross-feature imports, promotion |
| `references/data-and-live-updates.md` | DT - SDK only, query key factories, parsing, mutations, uniform states, pagination, server-sent events |
| `references/routing.md` | RT - typed routes, search parameters, loaders, code splitting, server fallback |
| `references/state.md` | ST - where state lives, the editor store, undo and redo, clipboard, saving |
| `references/forms.md` | FM - React Hook Form, generated Zod schemas, server errors on fields, node configuration forms |
| `references/errors.md` | ER - problem+json adapter, status handling, boundaries, contract errors |
| `references/typescript.md` | TS - strict flags, forbidden constructs, validating external input |
| `references/testing.md` | TE - unit, screen, end-to-end, accessibility, coverage, journeys |
| `references/performance.md` | PF - initial JavaScript, first useful screen, 500-node editor, long lists, emulated measurement |
| `references/dependencies.md` | DP - licenses, exceptions, versions, notices, SPDX, provenance |
| `references/checks.md` | every automated check, where it runs, inherited checks, mneme |

## When To Use

- You are about to write or change TypeScript or React code in `apps/web` or `packages/js`.
- You add a screen, route, query, mutation, live-update handler, form, store action or dependency.
- You change lint, TypeScript, test, bundle or performance configuration of the web workspace.
- A design or a review asks whether web code follows Orcon conventions.

## Critical Rules

1. Code lives in `src/app/` (composition) or `src/features/<area>/{routes,screens,api,forms,components,model}`; `store/` only in the editor; no `utils`, `helpers` or `shared` folders (AR).
2. A feature never imports another feature; promote shared code to `@orcon/ui`, `@orcon/sdk`, `@orcon/api-contract` or `app/` (AR-6).
3. Only `@orcon/sdk` talks to the server; SDK calls only in `features/<area>/api/`; never `fetch`, `XMLHttpRequest`, `EventSource` or `WebSocket` (DT-1, DT-2).
4. Every query and mutation is defined once with a key factory that starts with the organization id; screens use those options (DT-3, DT-4).
5. Responses, events, storage, clipboard and URL input are parsed with Zod at the boundary before use (DT-5, TS-3).
6. Every query screen renders through `QueryState`: loading, empty, error with retry, forbidden, data (DT-8).
7. Live events patch or invalidate only the affected keys; never everything except on `stream.reset` (DT-12).
8. No optimistic updates for runs, approvals, UNCERTAIN resolutions or anything with an external effect (DT-17).
9. Routes are typed; search parameters are validated with Zod with defaults; filters live in the URL; loaders prefetch through Query (RT).
10. Server data is never copied into state; the editor store is the only store, changed only by named actions with undo history (ST).
11. Forms: React Hook Form + generated Zod schema; field `name` is the JSON pointer; server errors land on fields and in the error summary; messages from catalogs (FM).
12. TypeScript: no `any`, no `!`, no `@ts-ignore`; `@ts-expect-error` only with a reason; the shared strict flags stay on (TS).
13. Every screen has screen tests for its five states; logic coverage stays at 80 % or more; journeys run against the real binary (TE).
14. Initial JavaScript at most 250 kB gzip; the editor is never in the initial load; long lists are virtualized (PF).
15. Shipped dependencies are Apache-2.0, MIT, BSD or ISC unless an approved exception is recorded; every file has the SPDX header (DP).
16. Comments, docs, commit messages and pull request text never name build assistants or carry AI authorship trailers; mneme may be named (DP-7).
17. Edit this skill only under `.agents/skills/orcon-web/` and re-run `scripts/project.sh`; never run `mneme skills pin` on it.

## Automated Checks

| Check | What it verifies | How to fix |
|---|---|---|
| `pnpm lint` boundaries | known files only, no cross-feature imports, allowed import direction | Move the file to an allowed folder or promote the shared code |
| `pnpm lint` network and SDK | no `fetch`/`XMLHttpRequest`/`EventSource`/`WebSocket`; SDK only in `api/` | Call the SDK from the feature's `api/` and use its query options |
| `pnpm lint` store | Zustand and Immer only in the editor store | Use component state, URL state or Query instead |
| `pnpm lint` external input | `JSON.parse`, storage, clipboard and messages only in `app/boundary` | Use the boundary helper with a Zod schema |
| `pnpm lint` TypeScript | no `any`, `!`, `@ts-ignore`, double assertions; reasons on `@ts-expect-error`; no floating promises; exhaustive switches | Type the value, parse it, or handle every case |
| `pnpm --filter @orcon/lint-config test` | every planted violation is detected and the good tree passes | Fix the rule, never the fixture |
| `check-tsconfig.mjs` | every package has the strict flags in its effective configuration | Extend `tsconfig.base.json`; do not turn flags off |
| `pnpm typecheck` | the workspace compiles under the strict flags | Fix the type error |
| `pnpm generate:check` | route tree and contract code are current | Run `pnpm generate` and commit |
| `pnpm --filter @orcon/api-contract test` | the generator produces Zod schemas and mock handlers from OpenAPI 3.1 | Fix the generator configuration |
| `pnpm --filter @orcon/web test:coverage` | unit and screen tests pass; logic coverage at least 80 % | Add tests for the uncovered logic |
| `pnpm --filter @orcon/web size` | initial JavaScript at most 250 kB gzip; editor not in the initial chunks | Lazy-load the route or the heavy module |
| `pnpm --filter @orcon/ui test:virtual` | 10,000 rows stay virtualized and smooth | Use the data table's virtual mode |
| `make web-perf` | first useful screen and 500-node editor budgets, with emulated phone and frame-rate measurement | Find the slow render or request in the saved trace |
| `make web-e2e` | journeys and axe against the real binary on desktop and phone | Open the saved trace and fix the failing step |
| `pnpm licenses:check` | shipped and bundled packages have allowed licenses or a recorded exception | Replace the package or ask for an approved exception |
| `make spdx`, `make provenance` | SPDX headers; no build-assistant names or AI trailers | Add the header; reword the text |
| `make skill-check` | this skill conforms and its generated copy is identical | Fix `SKILL.md`, then run `make skill-sync` |

## Verification

1. `make web-check` exits 0 on your branch; CI runs the same steps in `ui-quality`.
2. `make web-e2e` and `make web-perf` pass (`sh scripts/verify.sh` runs all three).
3. New screens have screen tests for the five states; new routes set the first-useful-screen mark.
4. This skill: `sh scripts/project.sh --home && mneme skills lint orcon-web && mneme skills validate orcon-web` pass.
5. Report the commands you ran, their results and known limitations when you hand back.

## Workflow

1. Read the accepted ADR or spec that authorizes the change (specs live in mneme); stop if it is only Proposed.
2. Pick the area under `src/features/`; copy the files you need from `assets/feature-template/` (`references/architecture.md`).
3. Server data: add key factory entries and `queryOptions` in `api/queries.ts`, parse with the generated schema, add mutations with their invalidations (`references/data-and-live-updates.md`).
4. Live updates: add handlers in `api/live.ts` that patch or invalidate the narrowest keys.
5. Route: add the route file in `routes/` with Zod-validated params and search, a loader that calls `ensureQueryData`, and error and not-found components (`references/routing.md`).
6. Screen: compose `orcon-ui` components (load `orcon-ui`), render queries through `QueryState`, keep UI state local, put shareable state in the URL (`references/state.md`), set the first-useful-screen mark.
7. Form: React Hook Form with the generated request schema, `FormField` names as JSON pointers, server errors mapped, catalog messages (`references/forms.md`, `references/errors.md`).
8. Editor changes: add or change named store actions with undo tests (`references/state.md` ST-5 to ST-7).
9. Tests: unit tests for logic, screen tests with generated mock handlers for the five states, a journey when the change adds a user path (`references/testing.md`).
10. While iterating run `sh scripts/web-change-check.sh`; before handing back run `sh scripts/verify.sh`; paste `assets/pr-web-checklist.md` into the pull request.
11. Write the commit message in Conventional Commits, describing the change only; citing mneme identifiers (`SPEC-…`, `BL-…`) is fine.
