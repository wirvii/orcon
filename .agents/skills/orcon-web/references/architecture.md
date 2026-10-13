<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Architecture

Status: Proposed — pending owner approval

This document defines how the web application in `apps/web` and the shared packages in `packages/js` are organized: the composition folder, feature folders, the allowed subfolders, the direction of imports and how shared code is promoted. The foundational libraries are recorded in ADR 0078 (`docs/adr/0078-web-application-foundational-libraries.md`). Rule IDs (`AR-…`) are stable.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` rules for anything visual > this rulebook > reviewer preference.

## Layout

- **AR-1** `apps/web/src` contains only `main.tsx`, `app/`, `features/` (and `i18n/` catalogs). Enforced by `boundaries/no-unknown-files`.
- **AR-2** `app/` holds composition and cross-cutting runtime only: router and route tree, query client, `query-state/`, `live-events/`, `boundary/` (external input), `errors/`, `forms/` (validation messages), `perf/` (marks), `shell/` (root layout). No business rules, no feature screens.
- **AR-3** A feature is `src/features/<area>/` with only these folders: `routes/` (route files, thin), `screens/`, `api/` (`queries.ts`, `mutations.ts`, `live.ts`), `forms/`, `components/` (feature-local, composed from `@orcon/ui`), `model/` (pure TypeScript, no React), and `store/` only in `features/editor/`. Tests sit next to the file (`*.test.ts(x)`).
- **AR-4** Areas are product nouns in kebab-case (`flows`, `editor`, `executions`, `attention`, `credentials`, `settings`, …). A new area is a new folder; no configuration change is needed.

```
apps/web/src/
├── main.tsx
├── app/            router, route tree, query client, query-state, live-events,
│                   boundary, errors, forms, perf, shell
├── features/
│   └── <area>/     routes, screens, api, forms, components, model (store: editor only)
└── i18n/           en.json, es.json
```

## Import direction

- **AR-5** Dependency direction inside a feature: `routes → screens → (forms, components, api, model)`; `api → @orcon/sdk, @orcon/api-contract, model`; `model →` nothing from React or the UI. Features may import from `app/` only `boundary`, `errors`, `forms`, `perf`, `query-state` and the live-events subscription hook. `app/` imports features only through the route tree.
- **AR-6** A feature never imports another feature. Links between features use typed router links (no import). Shared code is promoted: visual → `@orcon/ui` under `orcon-ui` ([components](../../orcon-ui/references/components.md#contract)); server access → `@orcon/sdk` or `@orcon/api-contract`; app-wide runtime → `app/`; pure logic used by two or more features → a new `packages/js` package created by an approved spec.

## Names

- **AR-7** Forbidden folder names: `utils`, `helpers`, `common`, `shared`, `misc`, `lib`, and `types` used as a dumping ground. No barrel `index.ts` in features (they defeat code splitting); import the concrete file.
- **AR-9** Files: React components `PascalCase.tsx`, one exported component per file; other modules `kebab-case.ts`; named exports only (route files export `Route`).

## Visual design

- **AR-8** Anything visual (layout, components, tokens, strings, icons, accessibility, canvas look) follows `orcon-ui`; this rulebook never overrides it. See [components](../../orcon-ui/references/components.md#contract), [content and i18n](../../orcon-ui/references/content-and-i18n.md#strings) and [canvas](../../orcon-ui/references/canvas.md#performance).

## Packages

- **AR-10** `packages/js/*` packages expose an explicit `exports` map; no deep imports into another package (`@orcon/*/src/**`).

## Checks

`boundaries/element-types`, `boundaries/no-unknown-files` and `no-restricted-imports` enforce AR-1 to AR-7 and AR-10 in `pnpm lint`; see `checks.md`.
