<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# TypeScript

Status: Accepted (2026-10-13)

This document defines the TypeScript compiler settings, the forbidden constructs and how external input becomes typed data in `apps/web` and `packages/js`. Rule IDs (`TS-…`) are stable.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` rules for anything visual > this rulebook > reviewer preference.

## Compiler settings

- **TS-1** Every `tsconfig.json` in `apps/web` and `packages/js` extends `@orcon/lint-config/tsconfig.base.json`: `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`, `noImplicitReturns`, `noFallthroughCasesInSwitch`, `noPropertyAccessFromIndexSignature`, `noUnusedLocals`, `noUnusedParameters`, `useUnknownInCatchVariables`, `allowUnreachableCode: false`, `verbatimModuleSyntax`, `isolatedModules`, `erasableSyntaxOnly`, `forceConsistentCasingInFileNames`. No package turns one off. `check-tsconfig.mjs` verifies the effective configuration of every package.

## Forbidden constructs

- **TS-2** Forbidden: `any` (explicit or implicit), the non-null assertion `!`, `@ts-ignore`, `@ts-nocheck`, `as unknown as`, object literal type assertions. `@ts-expect-error` only with a written reason of at least 10 characters.
- **TS-7** `import type` for types; no `enum` or `namespace` (union literals and `as const` objects instead).

## External input and types

- **TS-3** External input is `unknown` until parsed with Zod through `app/boundary`: API responses, event payloads, storage, clipboard, URL path and search, `postMessage`, uploaded files, any `JSON.parse` result.
- **TS-4** API types come from `@orcon/sdk` and `@orcon/api-contract`; never hand-written copies.
- **TS-5** Discriminated unions with exhaustive `switch`; `satisfies` for configuration objects; `readonly` arrays in props.

## Lint and generated code

- **TS-6** Type-aware lint (`strictTypeChecked`, `stylisticTypeChecked`) everywhere.
- **TS-8** Generated files are excluded from lint and included in type checking.

## Documentation

- **TS-9** Exported functions of packages have TSDoc; comments explain why; wording follows `dependencies.md` DP-7.
- **TS-10** A comment never claims more than the code or its tests guarantee, and it changes in the same commit as the code it describes (same rule as `orcon-go` GS-D7). Incidents: pull requests 168 and 165.
