<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Dependencies

Status: Proposed — pending owner approval

This document defines when a JavaScript dependency may be added, which licenses shipped code may carry, how exceptions are recorded, and the SPDX and provenance rules for web code. The policy is the same as the Go rulebook ([dependencies](../../orcon-go/references/dependencies.md)); the foundational libraries and their major versions are recorded in ADR 0078. Rule IDs (`DP-…`) are stable.

Precedence: accepted ADRs > accepted `orcon-go` rules > `orcon-ui` rules for anything visual > this rulebook > reviewer preference.

## Adding a dependency

- **DP-1** Before adding a dependency: platform API, then an existing dependency, then a new one; the pull request states size impact, license and maintenance.
- **DP-4** Major versions are fixed by ADR-0078; `pnpm-lock.yaml` is committed and CI installs with `--frozen-lockfile`; no git, URL or `latest` dependencies; install scripts only for packages listed in `pnpm.onlyBuiltDependencies`.

## Licenses

- **DP-2** Shipped code (production closure of `@orcon/web` and every bundled module) is Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause or ISC. Anything else fails unless its exact `package@version` is listed in `tools/licenses/npm-exceptions.txt` (`<package>@<version>  <SPDX id>  <reason>  <YYYY-MM-DD>  approved-by:<owner>`, the same format as the Go exceptions file), added only in a pull request the maintainers approve; upgrading an excepted package renews its line. Approved exceptions (2026-10-12): `@fontsource-variable/inter` and `@fontsource-variable/jetbrains-mono` (OFL-1.1, self-hosted fonts) and `tslib` (0BSD, required by react-intl). GPL, AGPL, LGPL, SSPL, BUSL, Elastic, Commons Clause and source-available licenses are never allowed. Same policy as the Go rulebook (`../../orcon-go/references/dependencies.md`).
- **DP-3** Development-only tools that are not shipped are exempt from the list but declared in the pull request (for example axe-core, MPL-2.0). Shipped code never imports them.
- **DP-5** The build writes `third-party-notices.txt` into `dist/`; it ships inside the binary.
- **DP-9** `pnpm audit --prod --audit-level=high` passes in CI.

## Headers and provenance

- **DP-6** Every source file starts with `SPDX-FileCopyrightText: The Orcon Authors` and `SPDX-License-Identifier: Apache-2.0` in the file's comment syntax; generators emit the header; `make spdx` checks it. For example, in TypeScript:

  ```ts
  // SPDX-FileCopyrightText: The Orcon Authors
  // SPDX-License-Identifier: Apache-2.0
  ```

- **DP-7** Comments, documentation, commit messages and pull request text never name the assistants or tools used to build Orcon and never carry AI authorship trailers; mneme, the project's specification and memory tool, may be named. `make provenance` and the CI provenance job check it.

## Run time

- **DP-8** No code loaded at run time from a CDN; no analytics or telemetry library without an ADR.
