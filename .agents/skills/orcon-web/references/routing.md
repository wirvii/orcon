<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Routing

Status: Proposed — pending owner approval

This document defines how the web application declares routes, keeps view state in the URL, prefetches data and splits code, and what it expects from the Go server that serves it. Routing uses TanStack Router with virtual file routes (ADR 0078). Rule IDs (`RT-…`) are stable.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` rules for anything visual > this rulebook > reviewer preference.

## Route tree

- **RT-1** TanStack Router with a generated, committed route tree (`app/routeTree.gen.ts`, SPDX header from `routeTreeFileHeader`). `app/routes.ts` mounts each feature's `routes/` folder with virtual file routes.
- **RT-9** Paths are kebab-case nouns with `$param` segments (`/flows/$flowId`). Paths are public (notifications and events link to them): renaming needs a redirect.

## Links and URL state

- **RT-2** Navigation only through typed `Link`, `useNavigate`, `linkOptions` or `createLink` for `@orcon/ui` link components; no hand-built URLs.
- **RT-3** A route with search parameters declares `validateSearch` with a Zod schema whose fields have defaults or `.catch()`, so a malformed URL falls back instead of failing. Filters, sort, page token, tab and shareable selection live in the search.
- **RT-4** Path parameters are parsed with Zod (`params.parse`); ids are UUID strings.

## Loading and splitting

- **RT-5** `loader` uses `ensureQueryData` with `loaderDeps` from the search; `defaultPreload: 'intent'`, `defaultPreloadStaleTime: 0`.
- **RT-6** `autoCodeSplitting` is on; the editor and other heavy screens are never in the initial chunks (`performance.md` PF-2).

## Errors and sessions

- **RT-7** Every route has an `errorComponent` (`app/errors/RouteError`) and `notFoundComponent`; their look comes from `orcon-ui`.
- **RT-10** The authenticated layout checks the session in `beforeLoad`; when absent it redirects to sign-in with a `redirect` search parameter validated as a same-origin path.

## Server contract

- **RT-8** Server contract: paths under `/api/`, `/healthz`, `/assets/` and the Go-served page prefixes are never UI routes; the server answers every other `GET` without a matching file with `index.html` (`no-cache`); hashed assets are immutable; a missing asset is 404. Deep links and reloads therefore work. The fallback is implemented in Go under the `orcon-go` rules.
