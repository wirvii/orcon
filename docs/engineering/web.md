<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Web application engineering standards

Status: Accepted (2026-10-13)

The editor in `apps/web` is a client of the public API through the generated SDK, organized in feature folders, with server data cached by TanStack Query, typed routes, a store only for the flow editor, forms validated against the API contract, and automated budgets for size and speed. The foundational libraries are recorded in [ADR 0078](../adr/0078-web-application-foundational-libraries.md).

This page is a short orientation for contributors. It contains no rules of its own.

## The web application rulebook

The complete web application rulebook is in `.agents/skills/orcon-web/references/`:

- [Architecture](../../.agents/skills/orcon-web/references/architecture.md) — app and feature folders, allowed subfolders, import direction, promotion of shared code.
- [Data and live updates](../../.agents/skills/orcon-web/references/data-and-live-updates.md) — the SDK, query key factories, mutations, uniform screen states, server-sent events.
- [Routing](../../.agents/skills/orcon-web/references/routing.md) — typed routes, search parameters, loaders, code splitting, the server fallback.
- [State](../../.agents/skills/orcon-web/references/state.md) — where state lives, the editor store, undo and redo, saving.
- [Forms](../../.agents/skills/orcon-web/references/forms.md) — React Hook Form, generated Zod schemas, server errors on fields, node configuration forms.
- [Errors](../../.agents/skills/orcon-web/references/errors.md) — the problem+json adapter, handling by status, error boundaries.
- [TypeScript](../../.agents/skills/orcon-web/references/typescript.md) — strict flags, forbidden constructs, validating external input.
- [Testing](../../.agents/skills/orcon-web/references/testing.md) — unit, screen, end-to-end and accessibility tests, coverage, journeys.
- [Performance](../../.agents/skills/orcon-web/references/performance.md) — initial JavaScript, first useful screen, the 500-node editor, long lists.
- [Dependencies](../../.agents/skills/orcon-web/references/dependencies.md) — licenses, exceptions, versions, notices, SPDX headers.
- [Checks](../../.agents/skills/orcon-web/references/checks.md) — every automated check, where it runs and how to fix it.

Visual design, components, strings and accessibility follow `.agents/skills/orcon-ui/`.

## Before you open a pull request

- Run `make web-check`; CI runs the same steps in the `ui-quality` job.
- Run `make web-e2e` (journeys and accessibility against the real binary) and `make web-perf` (size and speed budgets).
- Phone and frame-rate budgets are measured in CI with CPU and network emulation; they are an approximation of real devices.

## Generated code

The route tree and the code generated from the OpenAPI contract (Zod schemas, example data and mock API handlers) are committed, so reviews show their changes and builds do not depend on a generator run. After changing routes or the API contract, run `pnpm generate` and commit the result; CI fails when the committed files differ from a fresh generation.

## Licenses

Shipped JavaScript is licensed Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause or ISC. Three exceptions were approved on 2026-10-12 and are recorded in `tools/licenses/npm-exceptions.txt`: the self-hosted fonts `@fontsource-variable/inter` and `@fontsource-variable/jetbrains-mono` (OFL-1.1) and `tslib` (0BSD), which react-intl requires. Another exception is requested in a pull request that adds its line to that file; it applies only once the maintainers approve the pull request. The build writes a third-party notices file that ships inside the binary.

## Project tooling — mneme

Web changes start from a specification tracked in [mneme](https://github.com/wirvii/mneme), the project's tool for the backlog, specifications, acceptance criteria and QA reports. The rulebook is packaged as the `orcon-web` mneme skill; after changing it, run `make skill-sync` and validate it with `mneme skills lint orcon-web` and `mneme skills validate orcon-web`.

## Proposing a rule change

Open a pull request that edits the relevant file in `.agents/skills/orcon-web/references/` and runs `make skill-sync`, so the generated copies stay identical. Rule changes are reviewed like code; a change that contradicts an accepted ADR, including [ADR 0078](../adr/0078-web-application-foundational-libraries.md), needs a new ADR first.
