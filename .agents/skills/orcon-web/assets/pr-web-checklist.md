<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
## Web checklist

<!-- Paste this block into the description of every pull request that changes TypeScript or React code in apps/web or packages/js, or their lint, TypeScript, test, bundle or performance configuration. -->

- [ ] The change derives from an accepted ADR or an approved spec (cite it: `SPEC-…`, `BL-…`, `docs/adr/…`).
- [ ] Code lives in `src/app/` or `src/features/<area>/{routes,screens,api,forms,components,model}`; `store/` only in the editor; no `utils`, `helpers` or `shared` folders (AR).
- [ ] No feature imports another feature; shared code was promoted to `@orcon/ui`, `@orcon/sdk`, `@orcon/api-contract` or `app/` (AR-6).
- [ ] Only `@orcon/sdk` talks to the server, and only from `features/<area>/api/`; no `fetch`, `XMLHttpRequest`, `EventSource` or `WebSocket` (DT-1, DT-2).
- [ ] Every query and mutation is defined once with a key factory that starts with the organization id; screens use those options (DT-3, DT-4).
- [ ] Responses, events, storage, clipboard and URL input are parsed with Zod at the boundary before use (DT-5, TS-3).
- [ ] Every query screen renders through `QueryState`: loading, empty, error with retry, forbidden, data (DT-8).
- [ ] Live events patch or invalidate only the affected keys; everything only on `stream.reset` (DT-12).
- [ ] No optimistic updates for runs, approvals, UNCERTAIN resolutions or anything with an external effect (DT-17).
- [ ] Routes are typed; search parameters are validated with Zod with defaults; filters live in the URL; loaders prefetch through Query (RT).
- [ ] Server data is not copied into state; the editor store is changed only by named actions with undo history (ST).
- [ ] Forms use React Hook Form with the generated Zod schema; field `name` is the JSON pointer; server errors land on fields and in the error summary; messages come from catalogs (FM).
- [ ] No `any`, no `!`, no `@ts-ignore`; every `@ts-expect-error` has a reason; the shared strict flags stay on (TS).
- [ ] Every new screen has screen tests for its five states; logic coverage stays at 80 % or more; new user paths have a journey (TE).
- [ ] Initial JavaScript stays at most 250 kB gzip; the editor is not in the initial load; long lists are virtualized (PF).
- [ ] New shipped dependencies are Apache-2.0, MIT, BSD or ISC, or have an owner-approved line in `tools/licenses/npm-exceptions.txt`; size impact, license and maintenance are stated below; every file has the SPDX header (DP).
- [ ] Comments, docs, commit messages and this description do not name build assistants or carry AI authorship trailers (DP-7).
- [ ] Skill changes were made under `.agents/skills/orcon-web/` and projected with `scripts/project.sh`; `mneme skills pin` was not run.

### Commands run

<!-- Paste each command with its result, for example: -->

- `make web-check`:
- `make web-e2e`:
- `make web-perf`:
