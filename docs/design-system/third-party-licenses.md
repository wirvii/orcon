<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Third-party licenses

Dependencies of the web workspaces (`apps/web`, `packages/js/*`, the root) with the version the lockfile pins (`pnpm-lock.yaml`, committed) and the license read from each installed package. The attribution notices that ship with the binary are in [`NOTICE`](../../NOTICE). Go dependencies are checked by `make licenses`.

Every license below is compatible with Apache-2.0. No GPL, AGPL, LGPL, SSPL or BUSL license appears anywhere in the installed tree (`pnpm licenses list`: MIT, ISC, BSD-2-Clause, BSD-3-Clause, Apache-2.0, MIT-0, 0BSD, OFL-1.1, MPL-2.0, BlueOak-1.0.0, CC0-1.0, CC-BY-4.0, Python-2.0 and Public Domain).

## Shipped in the binary

The Vite build bundles these into `internal/adapter/webui/dist/`, which the binary embeds. Tailwind CSS (MIT) is a build-time compiler: only the CSS it emits is shipped. shadcn/ui is copied source, not a dependency; its notice is in `NOTICE`.

| Package | Version | License |
|---|---|---|
| `@dagrejs/dagre` | 3.1.1 | MIT |
| `@dagrejs/graphlib` | 4.0.5 | MIT |
| `@fontsource-variable/inter` | 5.3.0 | OFL-1.1 |
| `@fontsource-variable/jetbrains-mono` | 5.3.0 | OFL-1.1 |
| `@scalar/api-reference` | 1.74.0 | MIT |
| `@xyflow/react` | 12.12.0 | MIT |
| `class-variance-authority` | 0.7.1 | Apache-2.0 |
| `clsx` | 2.1.1 | MIT |
| `cmdk` | 1.1.1 | MIT |
| `immer` | 10.2.0 | MIT |
| `lucide-react` | 0.577.0 | ISC |
| `radix-ui` | 1.8.0 | MIT |
| `react` | 19.3.0 | MIT |
| `react-dom` | 19.3.0 | MIT |
| `react-intl` | 7.1.14 | BSD-3-Clause |
| `sonner` | 2.0.8 | MIT |
| `tailwind-merge` | 3.7.0 | MIT |
| `zustand` | 5.0.15 | MIT |

`@dagrejs/dagre` (with its dependency `@dagrejs/graphlib`) lays out the flow editor when somebody tidies up (SPEC-014 R-ED-CAN-03); it runs in a web worker and loads outside the initial chunks (ADR 0078 decision 14). `zustand` and `immer` hold the editor store and its undo history (ADR 0078 decision 3).

`@scalar/api-reference` is the API reference site (`scalar.standalone.js`, served at `/api/reference`, built by `packages/js/openapi-tools`); the licenses of the packages bundled in that script are in `third-party-notices.txt`, written next to it and checked against the allowed list by `build-reference.mjs`.

Transitive packages bundled with them (for example the d3 modules of `@xyflow/react`, `intl-messageformat`, `hoist-non-react-statics`) are MIT, ISC or BSD-3-Clause.

## Development and build only (not in the binary)

| Package | Version | License |
|---|---|---|
| `@axe-core/playwright` | 4.13.0 | MPL-2.0 |
| `@eslint-community/eslint-plugin-eslint-comments` | 4.8.1 | MIT |
| `@eslint/js` | 9.39.5 | MIT |
| `@formatjs/cli` | 6.16.34 | MIT |
| `@formatjs/icu-messageformat-parser` | 2.11.4 | MIT |
| `@playwright/test` | 1.64.0 | Apache-2.0 |
| `@storybook/addon-a11y` | 9.1.20 | MIT |
| `@storybook/react-vite` | 9.1.20 | MIT |
| `@tailwindcss/vite` | 4.3.3 | MIT |
| `@testing-library/jest-dom` | 6.10.0 | MIT |
| `@testing-library/react` | 16.3.3 | MIT |
| `@testing-library/user-event` | 14.6.7 | MIT |
| `@types/node` | 22.20.5 | MIT |
| `@types/react` | 19.3.0 | MIT |
| `@types/react-dom` | 19.3.0 | MIT |
| `@vitejs/plugin-react` | 4.7.0 | MIT |
| `@vitest/browser` | 3.2.7 | MIT |
| `axe-core` | 4.14.0 | MPL-2.0 |
| `eslint` | 9.39.5 | MIT |
| `eslint-plugin-better-tailwindcss` | 4.9.0 | MIT |
| `eslint-plugin-formatjs` | 5.4.2 | MIT |
| `eslint-plugin-i18next` | 6.1.5 | ISC |
| `globals` | 16.5.0 | MIT |
| `jsdom` | 26.1.0 | MIT |
| `playwright` | 1.64.0 | Apache-2.0 |
| `storybook` | 9.1.20 | MIT |
| `storybook-addon-pseudo-states` | 9.1.20 | MIT |
| `style-dictionary` | 4.4.0 | Apache-2.0 |
| `stylelint` | 16.26.1 | MIT |
| `stylelint-declaration-strict-value` | 1.12.1 | MIT |
| `tailwindcss` | 4.3.3 | MIT |
| `turbo` | 2.11.7 | MIT |
| `typescript` | 5.9.3 | Apache-2.0 |
| `typescript-eslint` | 8.71.2 | MIT |
| `vite` | 6.4.4 | MIT |
| `vitest` | 3.2.7 | MIT |

Notes:

- `axe-core` and `@axe-core/playwright` are MPL-2.0 and run only in tests and CI; they are never bundled. `lightningcss` (MPL-2.0, a Tailwind build tool) is not bundled either.
- Transitive development packages under other licenses (`argparse` Python-2.0, `caniuse-lite` CC-BY-4.0, `mdn-data` CC0-1.0, `jsonify` Public Domain, `jackspeak`, `minimatch`, `minipass`, `path-scurry` and `package-json-from-dist` BlueOak-1.0.0, `tslib` 0BSD) are build and test tooling only. `pnpm licenses list` reports no entry whose license is unknown or the literal word "License".

## Amendment 1 (editor style A)

Amendment 1 added no dependency. Between the commit before it (`25a61b9^`) and the head, the only change in any `package.json` is two scripts of `packages/js/ui/package.json` (`test:canvas`, `test:parity`), and `pnpm-lock.yaml` is unchanged. The canvas uses `@xyflow/react`, `cmdk`, `lucide-react` and Radix, all already present.
