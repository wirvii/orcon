<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# API review

Status: Accepted

This document is the review list for any pull request that changes the public API contract, the checks that enforce the rulebook (with tool, license, command, where it runs and how to fix a failure), the planted cases that prove each lint rule fails when it should, the checks inherited by the designs that build the API, and how API work is tracked. The toolchain is fixed by [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md) §9 and §11. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by ADR-0080, then this rulebook, then reviewer preference.

## Review checklist

The same list is in `assets/pr-api-checklist.md` for pasting into a pull request.

- [ ] The authorizing spec is accepted.
- [ ] Paths follow the grammar and actions use a listed verb ([paths.md](paths.md)).
- [ ] No envelope: resource, `{ items, nextPageToken? }` or problem+json ([responses.md](responses.md)).
- [ ] Fields, enums (including values inside documents returned verbatim), ids and times follow [fields.md](fields.md).
- [ ] Pagination, filters and sort follow [pagination.md](pagination.md).
- [ ] Problem codes are complete for every operation ([errors.md](errors.md)).
- [ ] Idempotency and `If-Match` are declared where required ([idempotency-and-concurrency.md](idempotency-and-concurrency.md)).
- [ ] The rate class is correct ([rate-limits.md](rate-limits.md)).
- [ ] Long-running operations return `202` with an `Operation` ([long-running.md](long-running.md)).
- [ ] Events are in `webhooks` with `x-orcon-event` ([events.md](events.md)).
- [ ] Stability is declared and compatibility is checked (`make api-compat` output pasted) ([versioning.md](versioning.md)).
- [ ] Deprecations carry sunset date and guide.
- [ ] Every operation declares a CLI command or an exemption ([channels.md](channels.md)).
- [ ] The MCP level is correct; a new tool has an accepted spec.
- [ ] Permission and audit action are declared.
- [ ] Summary, description and field documentation are complete ([documentation.md](documentation.md)).
- [ ] Every example has a scenario and ran green on SQLite; the last nightly PostgreSQL run is green.
- [ ] The reference site builds (`make api-reference`).
- [ ] `make generate` output is committed.
- [ ] No secrets in examples.
- [ ] The commands run are listed in the pull request.

## Checks

| Check | Tool (license) | Command | Where it runs | What it verifies | How to fix |
|---|---|---|---|---|---|
| Built-in lint | Redocly CLI (MIT) | `make api-lint` | CI job `api-contract`, local | structure, unique and URL-safe operation ids, defined path parameters, no ambiguous paths, valid examples, no unused components, defined security, a declared 4xx response | fix the contract at the reported location; the only built-in rule turned off is `operation-4xx-problem-details-rfc7807` (protocol operations answer errors in their standard's format; `orcon/problem-responses` covers every other operation, ERR-1), and no other rule is disabled |
| Orcon lint plugin | plugin `orcon` (Apache-2.0) on Redocly CLI | `make api-lint` | CI job `api-contract`, local | the 31 `orcon/<rule>` rules named in brackets in this rulebook | read the rule ID in the message, apply the rule in its reference file |
| Plugin self-tests | `node --test` | `make api-lint` (runs `pnpm --filter @orcon/openapi-tools test`) | CI job `api-contract`, local | the good fixture has 0 problems, each planted case fails with its rule, `plugin/verbs.js` equals the PATH-5 table, templates pass | keep `verbs.js`, the PATH-5 table and the fixtures in step in the same pull request |
| Generated drift | git | `make api-lint` | CI job `api-contract`, local | committed bundle, `docs/reference/api/README.md` and `docs/reference/problem-codes.md` match the contract | run `make api-lint` locally and commit the regenerated files |
| Compatibility | oasdiff (Apache-2.0) | `make api-compat` | CI job `api-contract`, local | no breaking change on stable/beta against the latest `v*` tag, event payloads included (VER-2, VER-4) | revert the break, or move it to a new major (VER-5) |
| Compatibility self-test | oasdiff (Apache-2.0) | `make api-compat-selftest` | CI job `api-contract`, local | each fixed breaking pair fails and each additive pair passes, including event pairs and one pair per entry of the severity file | update the pairs when ADR-0054 §3 changes |
| Examples on SQLite | examples runner (Apache-2.0), jsonschema (Apache-2.0, tests only) | `make api-examples` | CI job `api-contract` (required), local | every example runs against the real binary and matches status, schema and values | fix the example, the scenario or the server, never the comparison |
| Examples on PostgreSQL | same | `make api-examples DB=postgres` | nightly workflow (not required) | the same examples on PostgreSQL | reproduce locally with `DB=postgres`; fix before other API work (DOC-10) |
| Examples runner self-test | Go test | `make api-examples` | CI job `api-contract`, local | the runner fails on each planted mismatch | fix the runner |
| Reference site | Scalar API Reference (MIT) | `make api-reference` | CI job `api-contract`, release job `api-release`, local | the site builds, loads no external resource, stays within 3 MiB compressed and ships `LICENSE-scalar.txt` and `third-party-notices.txt` with only allowed licenses | remove the external URL; a larger budget amends ADR-0080 |
| Reference served | Orcon binary | handler tests of the HTTP adapter | CI | `/api/reference/` is served by the `api` and `standalone` roles with the content security policy of ADR-0080 §10, and `/api/reference` answers `301` to `/api/reference/` | fix the handler or its embedded files |
| API changelog | oasdiff (Apache-2.0) | `make api-changelog VERSION=…`, `make api-changelog-check`, `make api-changelog-selftest` | release job `api-release`, CI job `api-contract` | each release has its changelog and it matches the contract | regenerate with `make api-changelog` in the release preparation pull request |
| Skill copy | git, Agent Skills validator | `make skill-check` | CI | the generated copy of this skill is identical to `.agents/skills/orcon-api/` | run `make skill-sync` |
| Provenance | `tools/provenance` | `make provenance` | CI | no assistant names or authorship marks | remove them |

## Planted cases

Each lint rule has one fixture in `packages/js/openapi-tools/testdata/bad/` that must fail with exactly that rule: `path-shape.yaml`, `action-verb.yaml`, `path-param-name.yaml`, `operation-id.yaml`, `header-names.yaml`, `field-camel-case.yaml`, `id-format.yaml`, `enum-upper-snake.yaml`, `time-format.yaml`, `secret-fields.yaml`, `named-schemas.yaml`, `response-envelope.yaml` (rule `response-shape`, a body with `{ data, meta, success }`), `list-pagination.yaml`, `sort-allowlist.yaml`, `problem-responses.yaml`, `problem-codes-declared.yaml`, `idempotency.yaml`, `concurrency.yaml`, `rate-class.yaml`, `long-running.yaml`, `stability.yaml`, `deprecation.yaml`, `permission.yaml`, `audit.yaml`, `cli-declared.yaml`, `mcp-declared.yaml`, `doc-operation.yaml`, `doc-fields.yaml`, `doc-examples.yaml`, `events.yaml`, `codegen-subset.yaml` (31 cases). `testdata/good/openapi.yaml` must produce no problem.

## Inherited checks

The design that builds the API contract (SPEC-005) inherits these checks:

- **H-API1-1** the real contract passes `make api-lint` with no finding and no disabled rule other than the built-in `operation-4xx-problem-details-rfc7807` (ADR-0054 §1); the bundle for the Go generator is produced by `@orcon/openapi-tools`.
- **H-API1-2** `make api-compat` runs in the job `api-contract`.
- **H-API1-3** `/api/reference` and `/api/reference/*` serve the embedded site in the `api` and `standalone` roles with the content security policy of ADR-0080 §10, `/api/reference` redirecting with `301` to `/api/reference/`, no external resource, excluded from the router parity test; `make build` builds the site first.
- **H-API1-4** every registered response follows [responses.md](responses.md); no handler writes a 2xx body outside its declared schema.
- **H-API2-1** every operation has `x-orcon-examples` with a registered scenario.
- **H-API3-1** `make api-examples` runs every example against the real binary on SQLite in the required job and on PostgreSQL nightly, with zero failures.
- **H-API5-1** every event type of ADR-0055 §2 is in `webhooks` with `x-orcon-event`; event payload compatibility is checked by `make api-compat`, which compares events as responses (VER-4).
- **H-API7-1** the CLI coverage test reads `x-orcon-cli` and accepts only the three exemption categories.
- **H-API10-1** the MCP catalog is produced from `x-orcon-mcp.level`; confirmation is required exactly for `destructive`.
- **H-REL-1** the first published release has `docs/reference/api/changelog/<version>.md` and the job `api-release` passed.

## mneme

Every API change starts from a specification tracked in mneme, the project's tool for the backlog, specifications, acceptance criteria and QA reports. After editing this skill run `make skill-sync`, then `mneme skills lint orcon-api` and `mneme skills validate orcon-api`. Never run `mneme skills pin` on it. Commits may cite `SPEC-…` and `BL-…` identifiers.
