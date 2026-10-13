<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Area make files

The `Makefile` ends with `-include mk/*.mk`. Each area keeps its targets in its
own file here, so parallel changes do not edit the same lines.

## Rules

- One file per area, named as in the table below.
- Each file starts with the SPDX header and declares its own `.PHONY` line.
- Target names are unique across the whole tree. Never redefine the recipe of a
  target that lives in another file.
- An area file adds a prerequisite to `check` only when its spec says so
  expressly. The `check:` line in the `Makefile` does not change.
- Because the include is at the end of the `Makefile`, prerequisites an area file
  adds to an existing target come after the ones in the `Makefile`, in
  alphabetical file order (the expansion order of the GNU make wildcard).
- Whoever adds a new area adds its row to the table, through its spec.

## Registry

| Make file | Own workflows | Owner (spec, cut) |
|---|---|---|
| `mk/engine.mk` (`test-compat-floor`, `test-fault`) | none (run inside `check` or existing jobs, per their spec) | SPEC-001 C2a, C4a |
| `mk/security.mk` (only if SE1a needs targets) | none | SPEC-004 SE1a |
| `mk/web.mk` | `.github/workflows/web.yml` | SPEC-010 X6 |
| `mk/connectors.mk` | `.github/workflows/connectors.yml` (and `connectors-live.yml`, already its own) | SPEC-011 K8 |
| `mk/api.mk` | `.github/workflows/api.yml`, `.github/workflows/api-nightly.yml` | SPEC-012 A4 |
| `mk/contributing.mk` | `.github/workflows/contributing.yml` (job `release-check`), `pr.yml`, `release-please.yml`, `release.yml` | SPEC-013 C4, C5 |
| `mk/ops.mk` | `.github/workflows/ops.yml`, `.github/workflows/ops-nightly.yml`, `install-test.yml` | SPEC-015 O2, O9, O10 |
| `mk/docs.mk` | `.github/workflows/docs.yml` | SPEC-015 O11 |
| `mk/flow.mk` (JSON schema validation of the corpus; its spec decides whether it joins `check`) | none | BL-021 |

## Workflow rules for every area workflow

- Triggers: `pull_request`, `push` to `main` and `merge_group` (except workflows
  that only run on a schedule or a tag).
- No `paths:` filters on required jobs: a required job that does not run blocks
  the pull request forever.
- Actions pinned by SHA; `permissions: contents: read` unless a need is declared.
