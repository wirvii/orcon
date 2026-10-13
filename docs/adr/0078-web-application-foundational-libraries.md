<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR 0078: Web application foundational libraries

Status: Accepted (2026-10-13) | Amended: 2026-10-13

## Context
ADR-0074 fixed the visual foundation of the embedded editor (components, tokens, styling, i18n, UI quality gates). The application code around it has no foundation yet: how it reads and caches server data, routes, keeps editor state, builds forms, validates external input, and how it is tested and kept fast. The public API design (SPEC-005, ADR-0054) makes the editor a client of the public API through the generated TypeScript SDK `@orcon/sdk` (openapi-typescript and openapi-fetch) and delivers live updates over server-sent events. The maintainers fixed binding direction for the application code: server state cached by a query library on top of the SDK, typed routes with state in the URL, a store only for the flow editor, schema-validated forms generated from the API contract, feature folders with enforced boundaries, four test levels, performance budgets and the same license policy as the Go code.

## Decision
1. Server state: TanStack Query 5 (MIT) on top of `@orcon/sdk`. Queries and mutations are defined once per feature with query-key factories; screens never call the SDK; server-sent events invalidate or patch only the affected queries.
2. Routing: TanStack Router 1 (MIT), type-checked routes, path parameters and search parameters; route files live in feature folders through virtual file routes (`@tanstack/router-plugin`, `@tanstack/virtual-file-routes`, MIT, build time only). The Go server answers UI routes with `index.html`.
3. Editor state: Zustand 5 (MIT) with Immer 10 (MIT) patches for undo and redo, only for the flow editor. Other UI state is component state or URL state.
4. Forms and validation: React Hook Form 7 (MIT), `@hookform/resolvers` 5 (MIT) and Zod 4 (MIT). Zod also validates every external input at the boundary.
5. Contract-generated code: Kubb 4 (MIT; `plugin-oas`, `plugin-zod`, `plugin-faker`, `plugin-msw`) generates Zod schemas, example data and Mock Service Worker handlers from the OpenAPI document, without a second HTTP client. Output is committed and checked for drift.
6. Lists: `@tanstack/react-virtual` 3 (MIT) inside the design-system data table.
7. Testing: Vitest (MIT) with `@vitest/coverage-v8` (MIT); Testing Library (MIT); Mock Service Worker 2 (MIT); Playwright 1 (Apache-2.0) for end-to-end and performance tests against the real binary; `@axe-core/playwright` (MPL-2.0, development only, not distributed).
8. Static checks: eslint-plugin-boundaries 5 (MIT), `@tanstack/eslint-plugin-query` and `@tanstack/eslint-plugin-router` (MIT), eslint-plugin-react-hooks (MIT), typescript-eslint type-checked rules (MIT), a shared strict `tsconfig.base.json`.
9. Performance and licenses: size-limit with `@size-limit/file` (MIT) for the initial JavaScript budget; rollup-plugin-license 3 (MIT) for the bundled-module license check and the third-party notices file; `pnpm licenses` for the production dependency closure.
10. License policy for shipped JavaScript: Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause or ISC; anything else only with a per-package exception approved by the maintainers and recorded in `tools/licenses/npm-exceptions.txt`, in the same format as the Go exceptions file; development-only tools that are not shipped are exempt but declared. The maintainers approved three exceptions on 2026-10-12: the self-hosted fonts `@fontsource-variable/inter` and `@fontsource-variable/jetbrains-mono` (OFL-1.1) and `tslib` (0BSD), required by react-intl.
11. Major versions above are part of this decision; exact versions are pinned in `pnpm-lock.yaml`. Moving to another major version amends this ADR.
12. The rules that apply these libraries live in the repository skill `orcon-web` (`.agents/skills/orcon-web/references/`); `docs/engineering/web.md` summarizes them.
13. Performance budgets for a mid-range phone on 4G and for 60 frames per second are measured in CI by Chromium CPU and network emulation. This is a reproducible approximation, not a measurement on a physical phone or display.

## Consequences
One definition per query feeds screens, route loaders and live updates; links and URL state are checked by the compiler; editor history is testable without a browser; forms and the server validate against the same contract; screen tests run against handlers generated from that contract. Shipped libraries are permissive except the three recorded exceptions. The build gains a code generation step, a committed route tree and generated contract code, and CI gains end-to-end and performance jobs. Performance results on real low-end phones can be worse than the emulated budget.

## Alternatives considered
SWR (MIT; fewer mutation and cache tools), RTK Query (MIT; brings Redux), Apollo (GraphQL only). React Router 7 (MIT; weaker search-parameter typing). Redux Toolkit (MIT; more ceremony for one editor store), Jotai and Valtio (MIT; less direct undo history), XState (MIT; suited to protocols rather than a document editor). Formik (Apache-2.0; less maintained), TanStack Form (MIT; younger), Conform (MIT; server-action oriented). Valibot, ArkType and Yup (MIT; Zod has the widest generator support). Orval (MIT; mock handlers tied to its own client), Hey API (MIT; client first), openapi-zod-client (MIT; tied to zodios), typed-openapi (MIT; no mock handlers). Cypress (MIT; a second browser tool next to Playwright). Lighthouse CI (Apache-2.0; measures paint metrics rather than the product's own "first useful screen" mark). Measuring on physical devices (not reproducible on every pull request). bundlesize (unmaintained). react-window and react-virtuoso (MIT). dependency-cruiser (MIT; a separate tool, while the boundary rules belong in the lint run). React Compiler (not adopted now; revisited when the editor is measured).

## Amendments

- **SPEC-010 (Amendment 1, 2026-10-13).** Decision 5: `@kubb/plugin-ts` (MIT, Kubb 4) is a required companion of `plugin-zod`, `plugin-faker` and `plugin-msw`, which consume the TypeScript types it generates. It is pinned to the same Kubb 4 version as the other plugins in `packages/js/api-contract`. Its output stays inside `@orcon/api-contract` and adds no HTTP client: requests still go only through `@orcon/sdk` (DT-1).
