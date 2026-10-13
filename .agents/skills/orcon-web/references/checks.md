<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Checks

Status: Accepted (2026-10-13)

This document lists every automated check that enforces the web rulebook: the tool, its license, the command, where it runs, what it verifies and how to fix a failure. `make check` (the Go gate) does not change; the web checks run in the `ui-quality`, `web-e2e` and `web-perf` CI jobs and are reproduced locally by `make web-check`, `make web-e2e` and `make web-perf`.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` rules for anything visual > this rulebook > reviewer preference.

`make web-check` runs, in order: `pnpm install --frozen-lockfile`, `pnpm lint`, `pnpm typecheck`, `node packages/js/lint-config/scripts/check-tsconfig.mjs`, `pnpm --filter @orcon/lint-config test`, `pnpm generate:check`, `pnpm i18n:check`, `pnpm --filter @orcon/api-contract test`, `pnpm --filter @orcon/web test:coverage`, `pnpm --filter @orcon/ui test:virtual`, `pnpm --filter @orcon/web size`, `pnpm licenses:check`.

## Lint

`apps/web/eslint.config.mjs` composes `createConfig({ tailwindEntryPoint })` (the `orcon-ui` layer) with `createWebAppConfig({ appRoot })` from `packages/js/lint-config/web-app.config.mjs`. Packages in `packages/js` use the TypeScript and network parts without the feature rules. Tools: ESLint (MIT), typescript-eslint (MIT), eslint-plugin-boundaries (MIT), `@tanstack/eslint-plugin-query` and `@tanstack/eslint-plugin-router` (MIT), eslint-plugin-react-hooks (MIT). Command: `pnpm lint`. Runs in `ui-quality` and `make web-check`.

| Rule | Tool rule | Rejects | Rulebook | How to fix |
|---|---|---|---|---|
| Features | `boundaries/element-types` (default `disallow`) | `features/a/**` → `features/b/**`; `model/` → React or `@orcon/ui`; `app/` → feature internals except `routes/` | AR-5, AR-6 | Promote the shared code (AR-6) or reverse the import |
| Known files | `boundaries/no-unknown-files` | files outside `main.tsx`, `app/**` or `features/<area>/{routes,screens,api,forms,components,model,store}/**` (`store` only in `editor`); hence `utils/`, `helpers/`, `common/`, `shared/`, `lib/`, barrel `index.ts` in features | AR-1, AR-3, AR-7 | Move the file to an allowed folder |
| Deep imports | `no-restricted-imports` | `@orcon/*/src/**` | AR-10 | Import from the package `exports` |
| Network only through the SDK | `no-restricted-globals`, `no-restricted-properties` | `fetch`, `XMLHttpRequest`, `EventSource`, `WebSocket`, `window.fetch`, `navigator.sendBeacon` outside `packages/js/sdk` | DT-1 | Call the SDK from the feature's `api/` |
| SDK only in the data layer | `boundaries/external` | `@orcon/sdk` outside `features/*/api/**`, `app/live-events/**`, `app/errors/**` | DT-2 | Move the call to `api/` and use its query options |
| Store only in the editor | `boundaries/external` | `zustand`, `immer` outside `features/editor/store/**` | ST-2 | Use component state, URL state or Query |
| External input | `no-restricted-syntax`, `no-restricted-properties` | `JSON.parse`, `localStorage`, `sessionStorage`, `navigator.clipboard`, `window.addEventListener('message')` outside `app/boundary/**` | TS-3 | Use the boundary helper with a Zod schema |
| TypeScript | typescript-eslint `strictTypeChecked` + `stylisticTypeChecked`; `no-explicit-any`, `no-non-null-assertion`, `ban-ts-comment` (`ts-ignore`, `ts-nocheck` forbidden; `ts-expect-error: allow-with-description`, at least 10 characters), `no-floating-promises`, `switch-exhaustiveness-check`, `consistent-type-imports`, `consistent-type-assertions` (`objectLiteralTypeAssertions: never`), `no-restricted-syntax` against `as unknown as` | the constructs of TS-2, floating promises, non-exhaustive switches | TS-2, TS-5, TS-6, TS-7, ER-6 | Type the value, parse it, or handle every case |
| Exports | `no-restricted-exports` (`default`) except configuration files | default exports | AR-9 | Use a named export |
| Query, Router, hooks | `@tanstack/query/exhaustive-deps`, `@tanstack/query/stable-query-client`, `@tanstack/router/create-route-property-order`, `react-hooks/rules-of-hooks`, `react-hooks/exhaustive-deps` | missing key dependencies, unstable clients, misordered route options, hook misuse | DT-3, RT-5 | Add the dependency to the key; follow the rule's message |
| Console | `no-console` except `console.error` in `app/errors/**` | console output | ER-7 | Remove it, or report through `app/errors/` |

### Planted violations

`pnpm --filter @orcon/lint-config test` (Vitest, MIT) runs ESLint over `packages/js/lint-config/test/fixtures/web/bad/<case>/` and requires the expected rule ID for each case, and over a `good/` tree with a complete feature that must produce no findings. A failure means the rule changed: fix the rule, never the fixture. The planted cases, one per violation:

| # | Case | Expected rule |
|---|---|---|
| 1 | cross-feature import | `boundaries/element-types` |
| 2 | `model/` imports React | `boundaries/element-types` |
| 3 | `utils/` folder in a feature | `boundaries/no-unknown-files` |
| 4 | barrel `index.ts` in a feature | `boundaries/no-unknown-files` |
| 5 | deep import `@orcon/ui/src/...` | `no-restricted-imports` |
| 6 | `fetch` in a screen | `no-restricted-globals` |
| 7 | `window.fetch` in a component | `no-restricted-properties` |
| 8 | `@orcon/sdk` imported by a screen | `boundaries/external` |
| 9 | `zustand` outside the editor store | `boundaries/external` |
| 10 | `JSON.parse` outside `app/boundary` | `no-restricted-syntax` |
| 11 | `localStorage` outside `app/boundary` | `no-restricted-properties` |
| 12 | explicit `any` | `@typescript-eslint/no-explicit-any` |
| 13 | non-null assertion `!` | `@typescript-eslint/no-non-null-assertion` |
| 14 | `@ts-ignore` | `@typescript-eslint/ban-ts-comment` |
| 15 | `as unknown as` | `no-restricted-syntax` |
| 16 | floating promise | `@typescript-eslint/no-floating-promises` |
| 17 | non-exhaustive `switch` on a union | `@typescript-eslint/switch-exhaustiveness-check` |
| 18 | default export | `no-restricted-exports` |
| 19 | query key missing a dependency | `@tanstack/query/exhaustive-deps` |

The fixture folder names are the source of truth.

## TypeScript

| Check | Tool (license) | Command | Where | Verifies | How to fix |
|---|---|---|---|---|---|
| Effective flags | TypeScript (Apache-2.0) | `node packages/js/lint-config/scripts/check-tsconfig.mjs` | `ui-quality`, `make web-check` | `tsc --showConfig -p <package>` for every package with a `tsconfig.json` has every TS-1 flag at its required value | Extend `@orcon/lint-config/tsconfig.base.json`; do not turn flags off |
| Type check | TypeScript (Apache-2.0) | `pnpm typecheck` | `ui-quality`, `make web-check` | the workspace compiles under the strict flags | Fix the type error |

## Tests

| Check | Tool (license) | Command | Where | Verifies | How to fix |
|---|---|---|---|---|---|
| Unit and screen tests with coverage | Vitest, `@vitest/coverage-v8`, Testing Library, Mock Service Worker (MIT) | `pnpm --filter @orcon/web test:coverage` | `ui-quality`, `make web-check` | projects `unit` and `screens` pass; coverage ≥ 80 % lines, branches, functions and statements over the logic paths of TE-2 | Add tests for the uncovered logic |
| Catalogs | the `orcon-ui` catalog check | `pnpm i18n:check` | `ui-quality`, `make web-check` | `apps/web/src/i18n/{en,es}.json` are complete and extracted | Follow [content and i18n](../../orcon-ui/references/content-and-i18n.md#catalogs) |

## Contract generation

| Check | Tool (license) | Command | Where | Verifies | How to fix |
|---|---|---|---|---|---|
| Generated code is current | Kubb 4 (MIT), TanStack Router plugin (MIT) | `pnpm generate:check` | `ui-quality`, `make web-check` | the route tree and the contract code equal a fresh generation (`git diff --exit-code`) | Run `pnpm generate` and commit |
| Generator pipeline | Kubb 4 (MIT) | `pnpm --filter @orcon/api-contract test` | `ui-quality`, `make web-check` | from an OpenAPI 3.1 test document, Kubb produces Zod schemas that accept and reject the right input, Mock Service Worker handlers for a list and a problem+json response, and the SPDX header | Fix the generator configuration |

Until the real OpenAPI document exists under `packages/openapi/v1/`, `pnpm generate` prints `SKIP: OpenAPI document not present (SPEC-005 API1)` and exits 0.

## Performance

The phone and frame-rate measurements are emulated in Chromium on the CI runner and are an approximation of real devices, not a measurement on a physical phone or display (`performance.md`).

| Check | Tool (license) | Command | Where | Verifies | How to fix |
|---|---|---|---|---|---|
| Initial JavaScript | size-limit with `@size-limit/file` (MIT), Vite manifest | `pnpm --filter @orcon/web size` | `ui-quality`, `make web-check` | initial chunks ≤ 250 kB gzip; `check-initial-chunks.mjs`: no `@xyflow/react`, `zustand`, `immer` or `src/features/editor/**` in them (PF-1, PF-2) | Lazy-load the route or the heavy module |
| Long lists | Vitest browser mode, `@tanstack/react-virtual` (MIT) | `pnpm --filter @orcon/ui test:virtual` | `ui-quality`, `make web-check` | 10,000 rows: ≤ 60 rows in the DOM, first render < 100 ms, no frame > 50 ms to the end (PF-5) | Use the data table's virtual mode |
| First useful screen | Playwright (Apache-2.0) | `make web-perf` | `web-perf` | median of 5 cold loads to `orcon:first-useful`: < 1500 ms desktop, < 3000 ms emulated phone (PF-3); results in `test-results/perf.json` with `"method": "emulated-approximation"` | Find the slow render or request in the saved trace |
| 500-node editor | Playwright (Apache-2.0) | `make web-perf` | `web-perf` | open < 1000 ms; pan, zoom and drag with ≤ 5 % frames > 25 ms and none > 50 ms; median of 3 (PF-4) | Find the slow render in the saved trace |

## Licenses and notices

| Check | Tool (license) | Command | Where | Verifies | How to fix |
|---|---|---|---|---|---|
| Shipped licenses | `pnpm licenses list --prod --json` (pnpm, MIT), rollup-plugin-license (MIT) | `pnpm licenses:check` | `ui-quality`, `make web-check` | every package of the production closure of `@orcon/web` and every bundled module has an allowed license or an exact line in `tools/licenses/npm-exceptions.txt` (DP-2) | Replace the package or ask for an approved exception |
| Third-party notices | rollup-plugin-license (MIT) | `pnpm --filter @orcon/web build` | `ui-quality` | `dist/third-party-notices.txt` is written and embedded (DP-5) | Fix the plugin configuration |
| Vulnerabilities | pnpm (MIT) | `pnpm audit --prod --audit-level=high` | `ui-quality` | no high or critical advisory in shipped packages (DP-9) | Upgrade or replace the package |
| SPDX headers | `spdxcheck` (repository tool) | `make spdx` | `check` | every file carries the SPDX header (DP-6) | Add the header; `make spdx-fix` |
| Provenance | `provenance` (repository tool) | `make provenance` | `provenance` | no build-assistant names or AI authorship trailers (DP-7) | Reword the text |

## Server routing

| Check | Tool (license) | Command | Where | Verifies | How to fix |
|---|---|---|---|---|---|
| Single-page fallback | Go `testing` | `go test ./internal/adapter/webui/...` (`TestSPAFallback`) | `check` | deep routes get `index.html` with `no-cache`; assets are immutable; missing assets, reserved prefixes and `/api/` answer 404; other methods 405 (RT-8) | Fix the handler under the `orcon-go` rules |

## End-to-end

| Check | Tool (license) | Command | Where | Verifies | How to fix |
|---|---|---|---|---|---|
| Journeys and accessibility | Playwright (Apache-2.0), `@axe-core/playwright` (MPL-2.0, development only) | `make web-e2e` | `web-e2e` | journey J0 on desktop (1440×900) and phone (390×844) against the real binary with a fresh data directory; no WCAG 2.2 A/AA violation from axe on any visited page (TE-1, TE-8) | Open the saved trace and fix the failing step |

## Inherited checks

These checks are owed by specifications that do not exist yet (`testing.md` lists them with their owners): H-API-1, H-API-2, H-API-3 and H-API-4 by the public API contract and live events slices; H-UI-1 (journeys J1, J2 and J3), H-UI-2 (editor performance on the real route), H-UI-3 (virtual executions list), H-UI-4 (first-useful-screen mark on every main route) and H-UI-5 (five-state screen tests) by the slices that build product screens.

## Exceptions

`tools/licenses/npm-exceptions.txt` records shipped packages whose license is outside the allowed list. Each line is `<package>@<version>  <SPDX id>  <reason>  <YYYY-MM-DD>  approved-by:<owner>`; a malformed line or a line without `approved-by:` fails the check, and GPL, AGPL, LGPL, SSPL, BUSL, Elastic, Commons Clause and source-available licenses fail even with a line. The file starts with three lines approved on 2026-10-12: `@fontsource-variable/inter` (OFL-1.1), `@fontsource-variable/jetbrains-mono` (OFL-1.1) and `tslib` (0BSD), at the exact versions in `pnpm-lock.yaml`. Only a pull request the maintainers approve adds a line; upgrading an excepted package renews its line.

## mneme

Specifications, acceptance criteria and QA reports live in mneme, the project's tool for the backlog and specifications. After editing this skill under `.agents/skills/orcon-web/`, run `make skill-sync`, then `mneme skills lint orcon-web` and `mneme skills validate orcon-web`. Never run `mneme skills pin` on this skill.
