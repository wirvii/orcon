<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Connector authoring

Status: Proposed — pending owner approval

This page is a summary for contributors and connector authors. It holds no rules of its own: when it and the rulebook disagree, the rulebook wins.

## What a connector is

A connector lets flows talk to one external service. It is a package with a manifest (`orcon/connector@1`), English and Spanish catalogs, an icon, documentation and recorded tests. There are three kinds:

- **Declarative** — a JSON description of an HTTP API, interpreted by Orcon. This is the default for every official connector because it needs no code, is easy to review and is safe to run.
- **Native** — Go code shipped inside Orcon, for official connectors only, and only when the declarative format cannot express the service (for example a database protocol). Each native connector cites the gap it needs from [the declarative-gaps registry](../reference/declarative-gaps.md).
- **WebAssembly** — code from third parties, run in a sandbox.

## Where the rules are

The complete connector rulebook is in `.agents/skills/orcon-connector/references/`:

- [kinds.md](../../.agents/skills/orcon-connector/references/kinds.md) — which kind to choose, where it lives, precedence
- [naming.md](../../.agents/skills/orcon-connector/references/naming.md) — ids, resources, the verb vocabulary, parameters, dates, catalogs, value spelling
- [manifest.md](../../.agents/skills/orcon-connector/references/manifest.md) — required fields, network permissions, sensitive fields
- [credentials-and-egress.md](../../.agents/skills/orcon-connector/references/credentials-and-egress.md) — credential types, OAuth, credential tests
- [operation-safety.md](../../.agents/skills/orcon-connector/references/operation-safety.md) — the repeat-safety level of each operation
- [pagination-and-rate-limits.md](../../.agents/skills/orcon-connector/references/pagination-and-rate-limits.md) — paging and provider limits
- [errors.md](../../.agents/skills/orcon-connector/references/errors.md) — error codes and classes
- [testing.md](../../.agents/skills/orcon-connector/references/testing.md) — recorded tests, fault tests, live tests
- [versioning.md](../../.agents/skills/orcon-connector/references/versioning.md) — semantic versions and changelogs
- [documentation.md](../../.agents/skills/orcon-connector/references/documentation.md) — generated pages and connection guides
- [icon.md](../../.agents/skills/orcon-connector/references/icon.md) — the icon file
- [n8n-mapping.md](../../.agents/skills/orcon-connector/references/n8n-mapping.md) — replacing n8n nodes one to one
- [catalog-admission.md](../../.agents/skills/orcon-connector/references/catalog-admission.md) — automated and human review, seals
- [checks.md](../../.agents/skills/orcon-connector/references/checks.md) — every automated check and how to fix it

The design behind them is in ADR [0029](../adr/0029-connector-model-package-and-manifest.md), [0030](../adr/0030-declarative-http-connector-format.md), [0031](../adr/0031-native-go-connector-contract.md), [0032](../adr/0032-wasm-connector-abi-and-sandbox.md), [0033](../adr/0033-connector-trust-signing-and-catalog.md), [0034](../adr/0034-connector-versioning-and-migration.md), [0035](../adr/0035-n8n-community-node-converter.md), [0036](../adr/0036-code-node-languages-and-sandboxes.md), [0037](../adr/0037-v1-catalog-selection-coverage-and-n8n-mapping.md) and [0079](../adr/0079-connector-authoring-conventions-and-toolchain.md).

## Maintainers and external authors

Rules marked `[official]` apply only to Orcon's own connectors (`orcon.*`), which ship in the binary and receive the `OFFICIAL` seal. Every other rule also applies to connectors submitted to the catalog by third parties: they pass the same automated checks, then a person reviews them, and only then do they receive the `VERIFIED` seal. A local package that nobody reviewed is `UNREVIEWED`; the checks show it warnings instead of errors.

## Templates

Starting points for a declarative connector, a native connector, a recorded test and a connection guide are in `.agents/skills/orcon-connector/assets/templates/`. Enumerated values in manifests and package files are written in UPPER_SNAKE (for example `UNSAFE`, `DATE_TIME`); recorded tests keep their recorded values.

## Testing

Real responses are recorded once against a test account and replayed offline on every change. Recording scrubs credentials and personal data automatically, and a separate detector refuses to save a recording that still contains any; a second, independent secret scanner runs in CI.

Fault tests replay each operation with an injected failure (a dropped connection, a 429, a server error) and check that it is retried only when that is safe. For writes without an idempotency key, a server error after sending is treated as uncertain: the connector checks with the service or asks a person, and never repeats the write blindly.

Live tests run nightly against free provider test accounts, never for pull requests, and never block a merge. When a service changes, they open a "Service changed" issue. Test accounts belong to the project, one per provider, with the smallest permissions that work, and their secrets are rotated every 90 days.

Live test account inventory (names only, never values):

| Connector | Test account | Secrets |
|---|---|---|
| — | none yet | — |

## Before opening a pull request

Maintainers run:

```sh
make connectors-check
make connectors-version-check
```

External authors run, on their connector folder:

```sh
orcon connector lint --admission <dir>
orcon connector test --faults <dir>
orcon connector docs --check <dir>
```

These commands are introduced by the code slices of SPEC-011, after the connector model of SPEC-003 is implemented.

## Versioning

Each connector has its own semantic version, checked against an automatic comparison of the manifests:

| Change | Version part |
|---|---|
| nothing visible changes in the manifest (fixes, documentation, tests, icon) | patch |
| something is added (node, operation, optional parameter, credential type, option value) | minor |
| something is removed, renamed or narrowed, or a new required parameter appears | major |

Every version has a changelog entry in English and Spanish. Flows keep the version they were built with until someone upgrades them, and the editor shows the changes first.

## Project tooling — mneme

Connector work in this repository starts from a specification tracked in mneme, the project's tool for the backlog, specifications, acceptance criteria and QA reports. Each connector wave is its own specification. The rulebook is packaged as the `orcon-connector` mneme skill; after changing it, run `make skill-sync` and validate it with `mneme skills lint orcon-connector` and `mneme skills validate orcon-connector`. Third-party authors do not need mneme.

## Changing a rule or adding a gap

A rule changes through a pull request that edits the rulebook and, when the change alters a design decision, a new or amended ADR. A new declarative-format gap is added by pull request to [the declarative-gaps registry](../reference/declarative-gaps.md) with an example of a real connector that needs it.
